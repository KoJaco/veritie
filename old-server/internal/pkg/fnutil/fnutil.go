package fnutil

import (
	"schma.ai/internal/domain/speech"

	"github.com/google/uuid"
)

func InheritIDs(prev, next []speech.FunctionCall) []speech.FunctionCall {
	idByName := make(map[string]string, len(prev))

	for _, c := range prev {
		if id, ok := c.Args["id"].(string); ok && id != "" {
			idByName[c.Name] = id
		}
	}

	out := make([]speech.FunctionCall, len(next))

	for i, c := range next {
		if _, ok := c.Args["id"]; !ok {
			if id := idByName[c.Name]; id != "" {
				if c.Args == nil {
					c.Args = map[string]any{}
				}
				c.Args["id"] = id
			}
		}
		out[i] = c
	}

	return out
}

// EnsureIDs walks each call, generates ids where missing, and keeps the
// top-level struct.ID in sync with Args["id"].
func EnsureIDs(calls []speech.FunctionCall) []speech.FunctionCall {
	for i := range calls {
		id := calls[i].ID
		if id == "" {
			id = uuid.New().String()
		}
		if calls[i].Args == nil {
			calls[i].Args = map[string]any{}
		}
		calls[i].Args["id"] = id
		calls[i].ID = id
	}
	return calls
}

// MergeUpdate updates-in-place when Name matches, keeps original order
func MergeUpdate(old, delta []speech.FunctionCall) []speech.FunctionCall {
	out := make([]speech.FunctionCall, 0, len(old)+len(delta))
	exists := map[string]int{} // name→index in out

	// copy old into out
	for _, c := range old {
		idx := len(out)
		out = append(out, c)
		exists[c.Name] = idx
	}
	// apply delta
	for _, c := range delta {
		if i, ok := exists[c.Name]; ok {
			out[i] = c // overwrite
		} else {
			out = append(out, c)
		}
	}
	return out
}

// MergePreserveOrder merges delta into current, preserving the original
// order of current.  Match is by "id" (first) or by Name if id missing.
func MergePreserveOrder(current, delta []speech.FunctionCall) []speech.FunctionCall {
	key := func(fc speech.FunctionCall) string {
		if id, ok := fc.Args["id"].(string); ok && id != "" {
			return id
		}
		return fc.Name
	}

	dLookup := map[string]speech.FunctionCall{}
	dOrder := []string{}
	for _, fc := range delta {
		k := key(fc)
		dLookup[k] = fc
		dOrder = append(dOrder, k)
	}

	var merged []speech.FunctionCall
	seen := map[string]bool{}
	for _, cur := range current {
		k := key(cur)
		if upd, ok := dLookup[k]; ok {
			merged = append(merged, upd)
			seen[k] = true
		} else {
			merged = append(merged, cur)
		}
	}

	for _, k := range dOrder {
		if !seen[k] {
			merged = append(merged, dLookup[k])
		}
	}
	return merged
}
