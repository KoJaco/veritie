package llmgemini

import (
	"encoding/json"
	"strings"

	"github.com/google/generative-ai-go/genai"
	g2 "google.golang.org/genai"
	"schma.ai/internal/pkg/logger"
)

func LogTools(tools []*genai.Tool, nameOnly bool) {
	if tools == nil {
		logger.Debugf("🔧 [LLM] Tools logged: nil")
		return
	}

	if nameOnly {
		for _, tool := range tools {
			for _, decl := range tool.FunctionDeclarations {
				logger.Debugf("🔧 [LLM] Tool: %s", decl.Name)
			}
		}
		return
	}

	jsonBytes, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		logger.Errorf("❌ [LLM] Error marshaling tools to JSON: %v", err)
		return
	}
	logger.Debugf("🔧 [LLM] Logging tools:\n%s", string(jsonBytes))
}

func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	// Remove markdown fences if present.
	s = strings.ReplaceAll(s, "```json", "")
	s = strings.ReplaceAll(s, "```", "")
	return strings.TrimSpace(s)
}

// buildSchemaFromJSONV2 parses raw JSON into genai.Schema (v2)
func buildSchemaFromJSONV2(raw []byte) *g2.Schema {
    if len(raw) == 0 {
        return &g2.Schema{Type: g2.Type("OBJECT")}
    }
    var anyMap map[string]any
    if err := json.Unmarshal(raw, &anyMap); err != nil {
        return &g2.Schema{Type: g2.Type("OBJECT")}
    }
    return buildSchemaV2(anyMap)
}

// buildSchemaV2 recursively constructs a genai.Schema from a generic map
func buildSchemaV2(m map[string]any) *g2.Schema {
    s := &g2.Schema{}
    if t, ok := m["type"].(string); ok {
        s.Type = g2.Type(t)
    }
    if props, ok := m["properties"].(map[string]any); ok {
        s.Properties = map[string]*g2.Schema{}
        for k, v := range props {
            if child, ok := v.(map[string]any); ok {
                s.Properties[k] = buildSchemaV2(child)
            }
        }
    }
    if items, ok := m["items"].(map[string]any); ok {
        s.Items = buildSchemaV2(items)
    }
    if req, ok := m["required"].([]any); ok {
        for _, r := range req {
            if name, ok := r.(string); ok {
                s.Required = append(s.Required, name)
            }
        }
    }
    return s
}

func ptr[T any](v T) *T { return &v }
