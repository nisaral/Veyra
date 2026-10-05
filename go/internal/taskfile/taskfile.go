// Package taskfile loads TaskSpec from JSON or YAML.
package taskfile

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	veyrav1 "github.com/nisaral/veyra/gen/veyra/v1"
)

type yamlTask struct {
	ID           string            `json:"id" yaml:"id"`
	Instruction  string            `json:"instruction" yaml:"instruction"`
	Workspace    string            `json:"workspace" yaml:"workspace"`
	Category     []string          `json:"category" yaml:"category"`
	Metadata     map[string]string `json:"metadata" yaml:"metadata"`
	Capabilities []string          `json:"required_capabilities" yaml:"required_capabilities"`
	Budget       *yamlBudget       `json:"budget" yaml:"budget"`
}

type yamlBudget struct {
	MaxActions int64   `json:"max_actions" yaml:"max_actions"`
	MaxUSD     float64 `json:"max_usd" yaml:"max_usd"`
	MaxWallMS  int64   `json:"max_wall_ms" yaml:"max_wall_ms"`
	MaxTokens  int64   `json:"max_tokens" yaml:"max_tokens"`
}

// Load reads a task file. JSON is detected by content, so either extension works.
func Load(path string) (*veyrav1.TaskSpec, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var yt yamlTask
	trimmed := strings.TrimSpace(string(b))
	if strings.HasPrefix(trimmed, "{") {
		if err := json.Unmarshal(b, &yt); err != nil {
			return nil, fmt.Errorf("parse json task: %w", err)
		}
	} else if err := yaml.Unmarshal(b, &yt); err != nil {
		return nil, fmt.Errorf("parse yaml task: %w", err)
	}
	if yt.ID == "" {
		return nil, fmt.Errorf("task file %s has no id", path)
	}
	return FromYAML(yt.ID, yt.Instruction, yt.Workspace, yt.Category, yt.Metadata, yt.Capabilities, yamlBudgetToProto(yt.Budget)), nil
}

// Inline builds a task from CLI arguments.
func Inline(instruction, workspace string, category []string) *veyrav1.TaskSpec {
	id := "inline-" + slug(instruction)
	return FromYAML(id, instruction, workspace, category, map[string]string{}, nil, &veyrav1.Budget{})
}

func FromYAML(id, instruction, workspace string, category []string, metadata map[string]string, caps []string, b *veyrav1.Budget) *veyrav1.TaskSpec {
	if metadata == nil {
		metadata = map[string]string{}
	}
	return &veyrav1.TaskSpec{
		Id:                   id,
		Instruction:          instruction,
		Workspace:            workspace,
		Category:             category,
		Metadata:             metadata,
		RequiredCapabilities: caps,
		Budget:               b,
	}
}

func yamlBudgetToProto(b *yamlBudget) *veyrav1.Budget {
	if b == nil {
		return &veyrav1.Budget{}
	}
	return &veyrav1.Budget{MaxActions: b.MaxActions, MaxUsd: b.MaxUSD, MaxWallMs: b.MaxWallMS, MaxTokens: b.MaxTokens}
}

func slug(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
		if b.Len() > 40 {
			break
		}
	}
	return strings.Trim(b.String(), "-")
}