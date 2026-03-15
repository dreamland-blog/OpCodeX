package skill

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadFileInput describes the parameters for reading a text file.
type ReadFileInput struct {
	Path     string `json:"path"     schema:"required" description:"Absolute path to the file to read"`
	MaxLines int    `json:"max_lines"                   description:"Maximum number of lines to return (default 200)"`
}

// forbiddenReadPrefixes are path prefixes that should never be read to
// prevent accidental exposure of sensitive kernel/device data.
var forbiddenReadPrefixes = []string{
	"/proc/",
	"/sys/",
	"/dev/",
}

// ReadTextFileSkill reads a text file and returns its content.
type ReadTextFileSkill struct{}

// NewReadTextFileSkill creates the skill.
func NewReadTextFileSkill() *ReadTextFileSkill {
	return &ReadTextFileSkill{}
}

func (s *ReadTextFileSkill) Name() string        { return "read_file" }
func (s *ReadTextFileSkill) Description() string {
	return "Read a text file from the filesystem and return its content (up to max_lines lines)."
}

func (s *ReadTextFileSkill) InputSchema() map[string]any {
	return GenerateSchema(ReadFileInput{})
}

func (s *ReadTextFileSkill) Execute(_ context.Context, input SkillInput) (SkillOutput, error) {
	path, _ := input.Parameters["path"].(string)
	if path == "" {
		return SkillOutput{}, fmt.Errorf("read_file: missing required parameter 'path'")
	}

	// Security: reject traversal and dangerous paths.
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return SkillOutput{}, fmt.Errorf("read_file: path traversal not allowed")
	}
	for _, prefix := range forbiddenReadPrefixes {
		if strings.HasPrefix(cleanPath, prefix) {
			return SkillOutput{}, fmt.Errorf("read_file: access to %s is forbidden", prefix)
		}
	}

	maxLines := 200
	if ml, ok := input.Parameters["max_lines"].(float64); ok && ml > 0 {
		maxLines = int(ml)
	}

	file, err := os.Open(cleanPath)
	if err != nil {
		return SkillOutput{}, fmt.Errorf("read_file: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) >= maxLines {
			lines = append(lines, fmt.Sprintf("... [truncated at %d lines]", maxLines))
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return SkillOutput{}, fmt.Errorf("read_file: scan error: %w", err)
	}

	content := strings.Join(lines, "\n")
	return SkillOutput{
		Result: map[string]any{
			"path":       cleanPath,
			"line_count": len(lines),
			"content":    content,
		},
		RawText: content,
	}, nil
}
