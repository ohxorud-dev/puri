package problem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Frontmatter struct {
	Title       string    `yaml:"title"`
	TimeLimit   string    `yaml:"time_limit"`
	MemoryLimit string    `yaml:"memory_limit"`
	Tier        string    `yaml:"tier"`
	Step        string    `yaml:"step"`
	Examples    []Example `yaml:"examples"`
}

type Example struct {
	Input  string `yaml:"input"`
	Output string `yaml:"output"`
}

type TestCase struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

func LoadMetadata(problemsPath string, problemID int32) (*Frontmatter, error) {
	path := filepath.Join(problemsPath, fmt.Sprintf("%d", problemID), "index.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read problem file: %w", err)
	}

	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("invalid markdown: missing frontmatter")
	}

	parts := strings.SplitN(content[3:], "---", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid markdown: frontmatter not closed")
	}

	var fm Frontmatter
	if err := yaml.Unmarshal([]byte(parts[0]), &fm); err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	return &fm, nil
}

func LoadTestCases(testcasesPath string, problemID int32) ([]TestCase, error) {
	path := filepath.Join(testcasesPath, fmt.Sprintf("%d", problemID), "testcases.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read testcases file: %w", err)
	}

	var cases []TestCase
	if err := yaml.Unmarshal(data, &cases); err != nil {
		return nil, fmt.Errorf("failed to parse testcases: %w", err)
	}

	return cases, nil
}
