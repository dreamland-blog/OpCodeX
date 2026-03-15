package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta"

// GeminiAdapter talks to Google's Gemini API via REST.
type GeminiAdapter struct {
	APIKey string
	Model  string
	client *http.Client

	// SystemInstruction is sent as a separate top-level field, not in contents.
	SystemInstruction string
}

// NewGeminiAdapter creates a ready-to-use adapter.
// model examples: "gemini-2.0-flash", "gemini-2.5-flash-preview-05-20"
func NewGeminiAdapter(apiKey, model string) *GeminiAdapter {
	return &GeminiAdapter{
		APIKey: apiKey,
		Model:  model,
		client: &http.Client{},
	}
}

// ---------------------------------------------------------------------------
// Gemini API request/response types (internal, not exported)
// ---------------------------------------------------------------------------

// -- Request --

type geminiRequest struct {
	Contents          []geminiContent          `json:"contents"`
	Tools             []geminiTool             `json:"tools,omitempty"`
	SystemInstruction *geminiContent           `json:"systemInstruction,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall    `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResp    `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

type geminiFunctionResp struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFuncDecl `json:"functionDeclarations"`
}

type geminiFuncDecl struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// -- Response --

type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
}

type geminiCandidate struct {
	Content geminiContent `json:"content"`
}

// ---------------------------------------------------------------------------
// Chat — the main entry point
// ---------------------------------------------------------------------------

// Chat sends the conversation to the Gemini API and returns its response.
func (g *GeminiAdapter) Chat(ctx context.Context, messages []Message, tools []ToolDef) (Response, error) {
	if g.APIKey == "" {
		return Response{}, fmt.Errorf("gemini: API key not configured")
	}

	// Build the request body.
	req := g.buildRequest(messages, tools)

	body, err := json.Marshal(req)
	if err != nil {
		return Response{}, fmt.Errorf("gemini: marshal request: %w", err)
	}

	// POST to generateContent endpoint.
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", geminiBaseURL, g.Model, g.APIKey)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("gemini: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := g.client.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("gemini: http do: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("gemini: read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("gemini: API returned %d: %s", httpResp.StatusCode, string(respBody))
	}

	return g.parseResponse(respBody)
}

// ---------------------------------------------------------------------------
// Request building
// ---------------------------------------------------------------------------

func (g *GeminiAdapter) buildRequest(messages []Message, tools []ToolDef) geminiRequest {
	var req geminiRequest

	// System instruction (separate from contents).
	if g.SystemInstruction != "" {
		req.SystemInstruction = &geminiContent{
			Role: "user",
			Parts: []geminiPart{
				{Text: g.SystemInstruction},
			},
		}
	}

	// Convert messages → contents.
	//
	// Gemini uses roles "user" and "model".
	// Our Message roles: system, user, assistant, tool.
	//
	// Mapping:
	//   system    → skipped (handled via SystemInstruction)
	//   user      → role:"user"  + text part
	//   assistant → role:"model" + text part (and/or functionCall parts)
	//   tool      → role:"user"  + functionResponse part
	for _, msg := range messages {
		switch msg.Role {
		case RoleSystem:
			// System instructions go to the dedicated field.
			if g.SystemInstruction == "" {
				req.SystemInstruction = &geminiContent{
					Role: "user",
					Parts: []geminiPart{
						{Text: msg.Content},
					},
				}
			}

		case RoleUser:
			req.Contents = append(req.Contents, geminiContent{
				Role:  "user",
				Parts: []geminiPart{{Text: msg.Content}},
			})

		case RoleAssistant:
			req.Contents = append(req.Contents, geminiContent{
				Role:  "model",
				Parts: []geminiPart{{Text: msg.Content}},
			})

		case RoleTool:
			// Tool results serialized as a functionResponse part.
			// We encode the raw text into a simple result map.
			req.Contents = append(req.Contents, geminiContent{
				Role: "user",
				Parts: []geminiPart{
					{
						FunctionResponse: &geminiFunctionResp{
							Name:     "tool_result",
							Response: map[string]any{"output": msg.Content},
						},
					},
				},
			})
		}
	}

	// Convert tools → functionDeclarations.
	if len(tools) > 0 {
		decls := make([]geminiFuncDecl, 0, len(tools))
		for _, t := range tools {
			decls = append(decls, geminiFuncDecl{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			})
		}
		req.Tools = []geminiTool{{FunctionDeclarations: decls}}
	}

	return req
}

// ---------------------------------------------------------------------------
// Response parsing
// ---------------------------------------------------------------------------

func (g *GeminiAdapter) parseResponse(body []byte) (Response, error) {
	var raw geminiResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return Response{}, fmt.Errorf("gemini: unmarshal response: %w", err)
	}

	if len(raw.Candidates) == 0 {
		return Response{}, fmt.Errorf("gemini: empty candidates in response")
	}

	candidate := raw.Candidates[0]
	var resp Response

	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			if resp.Content != "" {
				resp.Content += "\n"
			}
			resp.Content += part.Text
		}
		if part.FunctionCall != nil {
			resp.ToolCalls = append(resp.ToolCalls, ToolCall{
				Name:       part.FunctionCall.Name,
				Parameters: part.FunctionCall.Args,
			})
		}
	}

	return resp, nil
}
