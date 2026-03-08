package mappers

import (
	"encoding/json"
	"strings"

	"schma.ai/internal/domain/speech"
)

// convert one OpenAPI / Gemini-style property map → []speech.FunctionParam
func ParamsFromSchema(raw json.RawMessage) ([]speech.FunctionParam, error) {
	// ❶ quick struct for just the bits we need
	var obj struct {
		Type       string                     `json:"type"`
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}

	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}

	reqSet := map[string]bool{}
	for _, n := range obj.Required {
		reqSet[n] = true
	}

	out := make([]speech.FunctionParam, 0, len(obj.Properties))
	for name, blob := range obj.Properties {

		var prop struct {
			Type        string   `json:"type"`
			Description string   `json:"description,omitempty"`
			Enum        []string `json:"enum,omitempty"`
			Format      string   `json:"format,omitempty"`
		}
		if err := json.Unmarshal(blob, &prop); err != nil {
			continue // ignore bad field
		}

		out = append(out, speech.FunctionParam{
			Name:        name,
			Type:        strings.ToLower(prop.Type),
			Description: prop.Description,
			Enum:        prop.Enum,
			Required:    reqSet[name],
		})
	}
	return out, nil
}
