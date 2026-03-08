package redaction

import (
	"regexp"

	redactor "schma.ai/internal/domain/redaction"
	"schma.ai/internal/pkg/logger"
)

var _ redactor.Redactor = (*PCIRedactor)(nil)

type PCIRedactor struct {
	
}	

func (r *PCIRedactor) RedactTranscript(text string) (spans []redactor.Span, err error) {
	if text == "" { return nil, nil }
	spans = DetectPCI(text)
	if len(spans) > 0 {
		logger.ServiceDebugf("PCI", "Transcript detection: %d spans", len(spans))
	}
	return spans, nil
}

func (r *PCIRedactor) RedactFunctionArgs(args map[string]interface{}) (spans []redactor.Span, err error) {
	if args == nil { return nil, nil }
	flattened := FlattenMap(args)
	spans = DetectPCI(flattened)
	if len(spans) > 0 {
		logger.ServiceDebugf("PCI", "Function args detection: %d spans", len(spans))
	}
	return spans, nil
}

func (r *PCIRedactor) RedactStructuredOutput(output map[string]interface{}) (spans []redactor.Span, err error) {
	if output == nil { return nil, nil }
	flattened := FlattenMap(output)
	spans = DetectPCI(flattened)
	if len(spans) > 0 {
		logger.ServiceDebugf("PCI", "Structured output detection: %d spans", len(spans))
	}
	return spans, nil
}


// helpers
func luhnValid(num string) bool {
	sum, alt := 0, false
	for i := len(num) - 1; i >= 0; i-- {
		c := num[i]
		if c < '0' || c > '9' {
			continue
		}
		n := int(c - '0')
		if alt {
			n *= 2
			if n > 9 { n -= 9 }
		}
		sum += n
		alt = !alt
	}
	return sum%10 == 0
}

func digitsOnly(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			b = append(b, s[i])
		}
	}
	return string(b)
}


// Regexes
var (
	// 13–19 digits with optional spaces/dashes
	rePAN = regexp.MustCompile(`(?i)(?:\b|^)(\d[ -]*?){13,19}\b`)

	// Track data (forbid entirely)
	reTrack1 = regexp.MustCompile(`%B[0-9]{1,19}\^[^^]{2,26}\^\d{4}`)
	reTrack2 = regexp.MustCompile(`;[0-9]{1,19}=\d{4}`)

	// Explicit CVV markers near 3–4 digits
	reCVV = regexp.MustCompile(`(?i)\b(CVV|CVC|CID|security\s*code)\D{0,6}(\d{3,4})\b`)

	// Expiry with context
	reEXP = regexp.MustCompile(`(?i)\b(exp|expires|expiry|valid\s*thru|good\s*thru)\s*[:\-]?\s*(0[1-9]|1[0-2])[\/-](\d{2}|\d{4})\b`)
)


// Find all PCI spans
func DetectPCI(text string) []redactor.Span {
	spans := make([]redactor.Span, 0, 4)

	// Track data (forbid)
	for _, m := range reTrack1.FindAllStringIndex(text, -1) {
		spans = append(spans, redactor.Span{Kind: redactor.KindPCI, Start: m[0], End: m[1], RuleID: "pci:track1"})
	}
	for _, m := range reTrack2.FindAllStringIndex(text, -1) {
		spans = append(spans, redactor.Span{Kind: redactor.KindPCI, Start: m[0], End: m[1], RuleID: "pci:track2"})
	}

	// CVV by keyword proximity
	for _, m := range reCVV.FindAllStringSubmatchIndex(text, -1) {
		spans = append(spans, redactor.Span{Kind: redactor.KindPCI, Start: m[0], End: m[1], RuleID: "pci:cvv"})
	}

	// Expiry by keyword
	for _, m := range reEXP.FindAllStringIndex(text, -1) {
		spans = append(spans, redactor.Span{Kind: redactor.KindPCI, Start: m[0], End: m[1], RuleID: "pci:exp"})
	}

	// PAN: filter with Luhn, and sanity rules to reduce FPs
	for _, m := range rePAN.FindAllStringIndex(text, -1) {
		raw := text[m[0]:m[1]]
		// Avoid obvious non-card sequences (e.g., long IDs of repeated same digit)
		only := digitsOnly(raw)
		if len(only) < 13 || len(only) > 19 {
			continue
		}
		// Reject if >70% same digit
		counts := [10]int{}
		for i := 0; i < len(only); i++ { counts[only[i]-'0']++ }
		max := 0
		for _, c := range counts { if c > max { max = c } }
		if float64(max)/float64(len(only)) > 0.7 {
			continue
		}
		// Luhn
		if luhnValid(only) {
			spans = append(spans, redactor.Span{Kind: redactor.KindPCI, Start: m[0], End: m[1], RuleID: "pci:pan:luhn"})
		}
	}

	if len(spans) > 0 {
		logger.ServiceDebugf("PCI", "DetectPCI found %d spans", len(spans))
	}
	return spans
}

// Mask a PAN to **** **** **** 1234
func maskPAN(raw string) string {
	d := digitsOnly(raw)
	if len(d) < 4 {
		return "[PCI:PAN]"
	}
	last4 := d[len(d)-4:]
	return "**** **** **** " + last4
}