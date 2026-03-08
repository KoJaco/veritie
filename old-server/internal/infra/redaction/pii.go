package redaction

import (
	redactor "schma.ai/internal/domain/redaction"
)

var _ redactor.Redactor = (*PIIRedactor)(nil)

type PIIRedactor struct {


}

func (r *PIIRedactor) RedactTranscript(text string) (spans []redactor.Span, err error) {
	return nil, nil
}

func (r *PIIRedactor) RedactFunctionArgs(args map[string]interface{}) (spans []redactor.Span, err error) {
	return nil, nil
}

func (r *PIIRedactor) RedactStructuredOutput(output map[string]interface{}) (spans []redactor.Span, err error) {
	return nil, nil
}