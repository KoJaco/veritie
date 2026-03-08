package llmgemini

import (
	"encoding/json"

	"github.com/google/generative-ai-go/genai"
	"schma.ai/internal/domain/speech"
	"schma.ai/internal/pkg/logger"
)

// SDKGenaiSchema mirrors the TypeScript GenaiSchema for unmarshalling.
type SDKGenaiSchema struct {
	Type        string                     `json:"type"` // e.g., "object", "string", "number", "boolean"
	Description string                     `json:"description,omitempty"`
	Format      string                     `json:"format,omitempty"`
	Nullable    bool                       `json:"nullable,omitempty"`
	Enum        []string                   `json:"enum,omitempty"`       // THIS IS WHAT WE NEED FOR llm.FunctionParam.Enum
	Items       *SDKGenaiSchema            `json:"items,omitempty"`      // For type "array"
	Properties  map[string]*SDKGenaiSchema `json:"properties,omitempty"` // For type "object", keys are param names
	Required    []string                   `json:"required,omitempty"`   // List of required property names
}

// SDKFunctionCallingSchemaObject mirrors the TypeScript FunctionCallingSchemaObject.
type SDKFunctionCallingSchemaObject struct {
	Name        string         `json:"name"`        // Function name
	Description string         `json:"description"` // Function description
	Parameters  SDKGenaiSchema `json:"parameters"`  // Describes the parameters object for this function
}

func toGenaiType(t string) genai.Type {
	switch t {
	case "string":
		return genai.TypeString
	case "object":
		return genai.TypeObject
	case "array":
		return genai.TypeArray
	case "boolean":
		return genai.TypeBoolean
	case "number":
		return genai.TypeNumber
	case "integer":
		return genai.TypeInteger
	default:
		// Fallback to object if the type is unknown.
		return genai.TypeObject
	}
}



func convertDefs(src []speech.FunctionDefinition) []*genai.FunctionDeclaration {
	out := make([]*genai.FunctionDeclaration, 0, len(src))
	for _, d := range src {
		fd := &genai.FunctionDeclaration{
			Name:        d.Name,
			Description: d.Description,
			Parameters:  &genai.Schema{Type: genai.TypeObject},
		}
		// Build Parameters->Properties
		props := make(map[string]*genai.Schema)
		for _, p := range d.Parameters {
			props[p.Name] = &genai.Schema{
				Type:        toGenaiType(p.Type),
				Description: p.Description,
				Enum:        p.Enum,
			}
		}
		fd.Parameters.Properties = props
		out = append(out, fd)
	}
	return out
}



func buildGenaiSchema(input map[string]interface{}) *genai.Schema {
	schema := &genai.Schema{}

	// Process "type". It might be a simple string or a map with "const", or with "enum" and nested "type".
	if t, ok := input["type"].(string); ok {
		schema.Type = toGenaiType(t)
	} else if tMap, ok := input["type"].(map[string]interface{}); ok {
		if constVal, ok := tMap["const"].(string); ok {
			schema.Type = toGenaiType(constVal)
		} else if typeStr, ok := tMap["type"].(string); ok {
			schema.Type = toGenaiType(typeStr)
		}
		// Process "enum" if present inside the type map.
		if enumArr, ok := tMap["enum"].([]interface{}); ok {
			for _, v := range enumArr {
				if s, ok := v.(string); ok {
					schema.Enum = append(schema.Enum, s)
				}
			}
		}
	}

	// Process "format"
	if f, ok := input["format"].(string); ok {
		schema.Format = f
	}

	// Process "description"
	if d, ok := input["description"].(string); ok {
		schema.Description = d
	}

	// Process "nullable"
	if n, ok := input["nullable"].(bool); ok {
		schema.Nullable = n
	}

	// Process "enum" if present at the top level.
	if enumArr, ok := input["enum"].([]interface{}); ok {
		for _, v := range enumArr {
			if s, ok := v.(string); ok {
				schema.Enum = append(schema.Enum, s)
			}
		}
	}

	// ---- items (robust) ----
    if raw, ok := input["items"]; ok {
        switch v := raw.(type) {
        case map[string]interface{}:
            schema.Items = buildGenaiSchema(v)

        case []interface{}:
            // JSON Schema allows items as an array; take the first schema if present
            if len(v) > 0 {
                if first, ok := v[0].(map[string]interface{}); ok {
                    schema.Items = buildGenaiSchema(first)
                }
            }

        case string:
            // Allow shorthand: items: "string" | "number" | ...
            schema.Items = &genai.Schema{ Type: toGenaiType(v) }
        }
    }

	// Process "properties" if present.
	if props, ok := input["properties"].(map[string]interface{}); ok {
		schema.Properties = make(map[string]*genai.Schema)
		for key, val := range props {
			if propMap, ok := val.(map[string]interface{}); ok {
				schema.Properties[key] = buildGenaiSchema(propMap)
			}
		}
	}

	// Process "required" if present.
	if reqArr, ok := input["required"].([]interface{}); ok {
		for _, r := range reqArr {
			if s, ok := r.(string); ok {
				schema.Required = append(schema.Required, s)
			}
		}
	}

	// ---- safety net: arrays MUST have items ----
    if schema.Type == genai.TypeArray && schema.Items == nil {
        // Choose a sensible default (string works for your emails/phones/tags/photos).
        schema.Items = &genai.Schema{ Type: genai.TypeString }
    }

	return schema
}

func convertToGenaiSchema(v interface{}) *genai.Schema {
	if v == nil {
		logger.Warnf("⚠️ [LLM] Schema value was nil")
		return &genai.Schema{Type: genai.TypeObject}
	}
	if m, ok := v.(map[string]interface{}); ok {
		schema := buildGenaiSchema(m)
		logger.Debugf("✅ [LLM] Schema out: %v", schema)
		return schema
	}
	b, err := json.Marshal(v)
	if err != nil {
		logger.Errorf("❌ [LLM] Error marshaling schema: %v", err)
		return &genai.Schema{Type: genai.TypeObject}
	}
	var schema genai.Schema
	if err := json.Unmarshal(b, &schema); err != nil {
		logger.Errorf("❌ [LLM] Error unmarshaling JSON: %v", err)
		return &genai.Schema{Type: genai.TypeObject}
	}
	logger.ServiceDebugf("LLM", "Schema out: %v", schema)
	return &schema
}
