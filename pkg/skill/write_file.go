package skill

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteFileInput describes the parameters for writing a text file.
type WriteFileInput struct {
	Path    string `json:"path"    schema:"required" description:"Absolute path to write the file to"`
	Content string `json:"content" schema:"required" description:"Text content to write"`
	Append  bool   `json:"append"                    description:"If true, append to existing file instead of overwriting"`
}

// DefaultAllowedDir is the directory the WriteTextFileSkill is restricted to.
const DefaultAllowedDir = "/tmp/opcodex"

// WriteTextFileSkill writes text content to a file.
//
// For safety, writes are restricted to AllowedDir (default: /tmp/opcodex/).
// The skill creates any necessary parent directories.
type WriteTextFileSkill struct {
	AllowedDir string
}

// NewWriteTextFileSkill creates the skill with the default allowed directory.
func NewWriteTextFileSkill() *WriteTextFileSkill {
	return &WriteTextFileSkill{AllowedDir: DefaultAllowedDir}
}

func (s *WriteTextFileSkill) Name() string { return "write_file" }
func (s *WriteTextFileSkill) Description() string {
	return fmt.Sprintf(
		"Write text content to a file. Writes are restricted to %s for safety.",
		s.AllowedDir,
	)
}

func (s *WriteTextFileSkill) InputSchema() map[string]any {
	return GenerateSchema(WriteFileInput{})
}

func (s *WriteTextFileSkill) Execute(_ context.Context, input SkillInput) (SkillOutput, error) {
	path, _ := input.Parameters["path"].(string)
	content, _ := input.Parameters["content"].(string)
	appendMode, _ := input.Parameters["append"].(bool)

	if path == "" {
		return SkillOutput{}, fmt.Errorf("write_file: missing required parameter 'path'")
	}
	if content == "" {
		return SkillOutput{}, fmt.Errorf("write_file: missing required parameter 'content'")
	}

	// Security: resolve and check path is within AllowedDir.
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return SkillOutput{}, fmt.Errorf("write_file: path traversal not allowed")
	}

	allowedDir := filepath.Clean(s.AllowedDir)
	if !strings.HasPrefix(cleanPath, allowedDir+"/") && cleanPath != allowedDir {
		return SkillOutput{}, fmt.Errorf("write_file: writes are restricted to %s", allowedDir)
	}

	// Ensure parent directory exists.
	dir := filepath.Dir(cleanPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return SkillOutput{}, fmt.Errorf("write_file: create dirs: %w", err)
	}

	// Write the file.
	flag := os.O_WRONLY | os.O_CREATE
	if appendMode {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}

	f, err := os.OpenFile(cleanPath, flag, 0o644)
	if err != nil {
		return SkillOutput{}, fmt.Errorf("write_file: open: %w", err)
	}
	defer f.Close()

	n, err := f.WriteString(content)
	if err != nil {
		return SkillOutput{}, fmt.Errorf("write_file: write: %w", err)
	}

	mode := "overwrite"
	if appendMode {
		mode = "append"
	}

	summary := fmt.Sprintf("Wrote %d bytes to %s (mode: %s)", n, cleanPath, mode)
	return SkillOutput{
		Result: map[string]any{
			"path":       cleanPath,
			"bytes":      n,
			"mode":       mode,
		},
		RawText: summary,
	}, nil
}
