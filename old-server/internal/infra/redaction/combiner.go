package redaction

import (
	redactor "schma.ai/internal/domain/redaction"
)

// CombinedRedactor aggregates multiple redactors (e.g., PCI + PHI)
type CombinedRedactor struct {
    redactors []redactor.Redactor
}

func NewCombinedRedactor(redactors ...redactor.Redactor) *CombinedRedactor {
    return &CombinedRedactor{redactors: redactors}
}

func (c *CombinedRedactor) RedactTranscript(text string) (spans []redactor.Span, err error) {
    if len(c.redactors) == 0 || text == "" { return nil, nil }
    var out []redactor.Span
    for _, r := range c.redactors {
        s, _ := r.RedactTranscript(text)
        if len(s) > 0 { out = append(out, s...) }
    }
    return out, nil
}

func (c *CombinedRedactor) RedactFunctionArgs(args map[string]interface{}) (spans []redactor.Span, err error) {
    if len(c.redactors) == 0 || args == nil { return nil, nil }
    var out []redactor.Span
    for _, r := range c.redactors {
        s, _ := r.RedactFunctionArgs(args)
        if len(s) > 0 { out = append(out, s...) }
    }
    return out, nil
}

func (c *CombinedRedactor) RedactStructuredOutput(output map[string]interface{}) (spans []redactor.Span, err error) {
    if len(c.redactors) == 0 || output == nil { return nil, nil }
    var out []redactor.Span
    for _, r := range c.redactors {
        s, _ := r.RedactStructuredOutput(output)
        if len(s) > 0 { out = append(out, s...) }
    }
    return out, nil
}


