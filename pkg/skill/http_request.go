package skill

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPRequestInput describes the parameters for an HTTP request.
type HTTPRequestInput struct {
	URL     string            `json:"url"     schema:"required" description:"Target URL"`
	Method  string            `json:"method"                     description:"HTTP method: GET, POST, PUT, DELETE (default GET)"`
	Body    string            `json:"body"                       description:"Request body for POST/PUT"`
	Headers map[string]string `json:"headers"                    description:"Custom request headers"`
}

const (
	httpTimeout        = 10 * time.Second
	httpMaxResponseLen = 64 * 1024 // 64 KB
)

// HTTPRequestSkill performs HTTP requests and returns the response.
type HTTPRequestSkill struct {
	client *http.Client
}

// NewHTTPRequestSkill creates the skill with a timeout-bound HTTP client.
func NewHTTPRequestSkill() *HTTPRequestSkill {
	return &HTTPRequestSkill{
		client: &http.Client{Timeout: httpTimeout},
	}
}

func (s *HTTPRequestSkill) Name() string        { return "http_request" }
func (s *HTTPRequestSkill) Description() string {
	return "Send an HTTP request and return the response status and body (max 64KB)."
}

func (s *HTTPRequestSkill) InputSchema() map[string]any {
	return GenerateSchema(HTTPRequestInput{})
}

func (s *HTTPRequestSkill) Execute(ctx context.Context, input SkillInput) (SkillOutput, error) {
	urlStr, _ := input.Parameters["url"].(string)
	if urlStr == "" {
		return SkillOutput{}, fmt.Errorf("http_request: missing required parameter 'url'")
	}

	method, _ := input.Parameters["method"].(string)
	if method == "" {
		method = "GET"
	}
	method = strings.ToUpper(method)

	body, _ := input.Parameters["body"].(string)

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return SkillOutput{}, fmt.Errorf("http_request: create request: %w", err)
	}

	// Apply custom headers.
	if hdrs, ok := input.Parameters["headers"].(map[string]any); ok {
		for k, v := range hdrs {
			if vs, ok := v.(string); ok {
				req.Header.Set(k, vs)
			}
		}
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return SkillOutput{}, fmt.Errorf("http_request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body with limit.
	limited := io.LimitReader(resp.Body, httpMaxResponseLen+1)
	respBody, err := io.ReadAll(limited)
	if err != nil {
		return SkillOutput{}, fmt.Errorf("http_request: read body: %w", err)
	}

	truncated := false
	if len(respBody) > httpMaxResponseLen {
		respBody = respBody[:httpMaxResponseLen]
		truncated = true
	}

	bodyStr := string(respBody)
	if truncated {
		bodyStr += "\n... [truncated at 64KB]"
	}

	summary := fmt.Sprintf("[%d %s] %s %s\n\n%s",
		resp.StatusCode, resp.Status, method, urlStr, bodyStr)

	return SkillOutput{
		Result: map[string]any{
			"status_code": resp.StatusCode,
			"status":      resp.Status,
			"body":        bodyStr,
			"truncated":   truncated,
		},
		RawText: summary,
	}, nil
}
