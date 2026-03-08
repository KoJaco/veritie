package redaction

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	onnx "github.com/yalue/onnxruntime_go"
	redactor "schma.ai/internal/domain/redaction"
)

var _ redactor.Redactor = (*PHIRedactor)(nil)

// Regex patterns for rule-based detection
var (
	reEmail = regexp.MustCompile(`(?i)\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b`)
	reURL   = regexp.MustCompile(`(?i)\bhttps?://\S+\b`)
	rePhoneLand   = regexp.MustCompile(`(?i)(?:\+61\s?[2378]|\(?0[2378]\)?)\s?\d{4}\s?\d{4}`)
	rePhoneMobile = regexp.MustCompile(`(?i)(?:\+61\s?4\d{2}|04\d{2})\s?\d{3}\s?\d{3}`)
	reMRN       = regexp.MustCompile(`\b(?:MRN[:\s]*)?(\d{7,10})\b`)
	reMedicare  = regexp.MustCompile(`\b\d{4}\s?\d{5}\s?\d\b`)
)

var precedence = map[string]int{
	"EMAIL":  90,
	"ID":     88,
	"PHONE":  85,
	"OTHERPHI": 80,
	"DATE":   60, "PATIENT": 55, "STAFF": 55, "HOSP": 55, "PATORG": 50, "LOC": 45,
}

type PHIRedactor struct {
	modelPath string
	configPath string
	tokenizerSock string
	session *onnx.DynamicSession[int64, float32]
	id2label map[int]string
	numLabels int64
}

type encodeResp struct {
	InputIDs      [][]int64 `json:"input_ids"`
	AttentionMask [][]int64 `json:"attention_mask"`
	Offsets       [][][]int `json:"offsets"`
}

type tokClient struct {
	h   *http.Client
	url string
}

func newUDSClient(sock string) *tokClient {
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			d := net.Dialer{Timeout: 2 * time.Second}
			return d.DialContext(ctx, "unix", sock)
		},
		MaxIdleConns:       100,
		IdleConnTimeout:    60 * time.Second,
		DisableCompression: true,
	}
	return &tokClient{
		h:   &http.Client{Transport: tr, Timeout: 5 * time.Second},
		url: "http://unix",
	}
}

func (c *tokClient) Encode(text string, maxLen int) (ids, mask []int64, offsets [][]int, err error) {
	body := []byte(fmt.Sprintf(`{"text":%q,"max_length":%d}`, text, maxLen))
	req, _ := http.NewRequest("POST", c.url+"/encode", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := c.h.Do(req)
	if err != nil { return }
	defer res.Body.Close()
	var r encodeResp
	if err = json.NewDecoder(res.Body).Decode(&r); err != nil { return }
	return r.InputIDs[0], r.AttentionMask[0], r.Offsets[0], nil
}

func NewPHIRedactor(modelPath, configPath, tokenizerSock string) (*PHIRedactor, error) {
	// Load id2label mapping
	id2label, err := loadID2Label(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	fi, err := os.Stat(modelPath)
	
	if err != nil {
    	return nil, fmt.Errorf("phi model not found at %q: %w", modelPath, err)
	}

	log.Printf("[PHI] modelPath=%q size=%d", modelPath, fi.Size())
	
	// Create ONNX session (ONNX runtime already initialized by fastparser)
	session, err := onnx.NewDynamicSession[int64, float32](
		modelPath,
		[]string{"input_ids", "attention_mask"},
		[]string{"logits"},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ONNX session: %w", err)
	}

	return &PHIRedactor{
		modelPath: modelPath,
		configPath: configPath,
		tokenizerSock: tokenizerSock,
		session: session,
		id2label: id2label,
		numLabels: int64(len(id2label)),
	}, nil
}

func (r *PHIRedactor) RedactTranscript(text string) (spans []redactor.Span, err error) {
	// Tokenize via sidecar
	tc := newUDSClient(r.tokenizerSock)
	inIDs, attn, offsets, err := tc.Encode(text, 512)
	if err != nil {
		return nil, fmt.Errorf("tokenization failed: %w", err)
	}

	// Create tensors
	shape := onnx.NewShape(1, int64(len(inIDs)))
	idT, err := onnx.NewTensor[int64](shape, inIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to create input tensor: %w", err)
	}
	defer idT.Destroy()

	mskT, err := onnx.NewTensor[int64](shape, attn)
	if err != nil {
		return nil, fmt.Errorf("failed to create mask tensor: %w", err)
	}
	defer mskT.Destroy()

	outT, err := onnx.NewEmptyTensor[float32](onnx.NewShape(1, int64(len(inIDs)), r.numLabels))
	if err != nil {
		return nil, fmt.Errorf("failed to create output tensor: %w", err)
	}
	defer outT.Destroy()

	// Run inference
	if err := r.session.Run([]*onnx.Tensor[int64]{idT, mskT}, []*onnx.Tensor[float32]{outT}); err != nil {
		return nil, fmt.Errorf("inference failed: %w", err)
	}

	// Process logits
	logits := outT.GetData()
	seq := len(inIDs)
	nl := int(r.numLabels)
	preds := make([]int, seq)
	for i := 0; i < seq; i++ {
		base := i * nl
		maxI := 0
		maxV := logits[base]
		for j := 1; j < nl; j++ {
			if v := logits[base+j]; v > maxV {
				maxV, maxI = v, j
			}
		}
		preds[i] = maxI
	}

	// Decode BIO/BILOU → spans
	var modelSpans []redactor.Span
	var cur *redactor.Span
	for i := 0; i < seq && i < len(offsets); i++ {
		start, end := offsets[i][0], offsets[i][1]
		if end <= start { // special token
			if cur != nil {
				modelSpans = append(modelSpans, *cur)
				cur = nil
			}
			continue
		}
		lab := r.id2label[preds[i]]
		switch {
		case strings.HasPrefix(lab, "B-") || strings.HasPrefix(lab, "U-"):
			if cur != nil {
				modelSpans = append(modelSpans, *cur)
				cur = nil
			}
			typ := lab[2:]
			if strings.HasPrefix(lab, "U-") {
				modelSpans = append(modelSpans, redactor.Span{
					Start: start,
					End:   end,
					Kind:  redactor.KindPHI,
					Confidence: 0.9, // TODO: use actual confidence
					RuleID: "model:" + typ,
				})
			} else {
				cur = &redactor.Span{
					Start: start,
					End:   end,
					Kind:  redactor.KindPHI,
					Confidence: 0.9,
					RuleID: "model:" + typ,
				}
			}
		case strings.HasPrefix(lab, "I-") || strings.HasPrefix(lab, "L-"):
			if cur != nil {
				cur.End = end
				if strings.HasPrefix(lab, "L-") {
					modelSpans = append(modelSpans, *cur)
					cur = nil
				}
			}
		default:
			if cur != nil {
				modelSpans = append(modelSpans, *cur)
				cur = nil
			}
		}
	}
	if cur != nil {
		modelSpans = append(modelSpans, *cur)
	}

	// Add rule-based detection
	ruleSpans := detectRulePHI(text)
	
	// Merge and deduplicate
	allSpans := append(modelSpans, ruleSpans...)
	allSpans = mergeSame(allSpans)
	allSpans = dedupeExact(allSpans)
	allSpans = unionWithPrecedence(modelSpans, ruleSpans)
	allSpans = growStaffAroundTitles(text, allSpans)
	allSpans = mergeClose(text, allSpans)

	return allSpans, nil
}

func (r *PHIRedactor) RedactFunctionArgs(args map[string]interface{}) (spans []redactor.Span, err error) {
	if args == nil {
		return nil, nil
	}
	
	// Create a comprehensive text representation that includes both keys and values
	flattened := FlattenMap(args)
	
	// Apply PHI redaction to the combined text
	detectedSpans, err := r.RedactTranscript(flattened)
	if err != nil {
		return nil, fmt.Errorf("failed to redact function args: %w", err)
	}
	
	return detectedSpans, nil
}

func (r *PHIRedactor) RedactStructuredOutput(output map[string]interface{}) (spans []redactor.Span, err error) {
	if output == nil {
		return nil, nil
	}

	// flatten
	flattened := FlattenMap(output)
	
	// Apply PHI redaction to the combined text
	detectedSpans, err := r.RedactTranscript(flattened)
	if err != nil {
		return nil, fmt.Errorf("failed to redact structured output: %w", err)
	}
	
	return detectedSpans, nil
}

func (r *PHIRedactor) Close() error {
	if r.session != nil {
		r.session.Destroy()
	}
	return nil
}

// Helper functions
func FlattenMap(m map[string]interface{}) string {
	parts := make([]string, 0, len(m)*2) // Pre-allocate for key-value pairs

	for k, v := range m {
		// Add key and value as separate parts
		parts = append(parts, k)
		switch v := v.(type) {
		case string:
			parts = append(parts, v)
		case map[string]interface{}:
			// For nested maps, flatten recursively
			nestedParts := strings.Fields(FlattenMap(v))
			parts = append(parts, nestedParts...)
		default:
			// Convert other types to string
			parts = append(parts, fmt.Sprintf("%v", v))
		}
	}
	return strings.Join(parts, " ")
}

// FlattenMapWithSeparator is used when we need to preserve key-value relationships
// and reconstruct the map later. Uses a more robust separator.
func FlattenMapWithSeparator(m map[string]interface{}) string {
	parts := make([]string, 0, len(m))
	
	for k, v := range m {
		switch v := v.(type) {
		case string:
			parts = append(parts, fmt.Sprintf("%s:%s", k, v))
		case map[string]interface{}:
			// For nested maps, use JSON-like format
			nestedJSON, _ := json.Marshal(v)
			parts = append(parts, fmt.Sprintf("%s:%s", k, string(nestedJSON)))
		default:
			parts = append(parts, fmt.Sprintf("%s:%v", k, v))
		}
	}
	return strings.Join(parts, " | ")
}

// RevertFlattenWithSeparator reconstructs a map from the separator-based format
func RevertFlattenWithSeparator(text string) map[string]interface{} {
	parts := strings.Split(text, " | ")
	m := make(map[string]interface{}, len(parts))
	
	for _, part := range parts {
		if idx := strings.Index(part, ":"); idx > 0 {
			key := part[:idx]
			value := part[idx+1:]
			
			// Try to parse as JSON first (for nested objects)
			var jsonValue interface{}
			if err := json.Unmarshal([]byte(value), &jsonValue); err == nil {
				m[key] = jsonValue
			} else {
				// Treat as string
				m[key] = value
			}
		}
	}
	return m
}

// PlaceholderFormat defines how redaction placeholders should be formatted
type PlaceholderFormat string

const (
	// Verbose format: [PHI:model:PATIENT#1]
	PlaceholderVerbose PlaceholderFormat = "verbose"
	// Compact format: [PATIENT#1]
	PlaceholderCompact PlaceholderFormat = "compact"
	// Minimal format: [DATE#1]
	PlaceholderMinimal PlaceholderFormat = "minimal"
	// Custom format: [CUSTOM#1]
	PlaceholderCustom PlaceholderFormat = "custom"
)

// applyMasking applies redaction spans to text and returns the masked version
func ApplyMasking(text string, spans []redactor.Span) string {
	return ApplyMaskingWithFormat(text, spans, PlaceholderVerbose)
}

// ApplyMaskingWithFormat applies redaction spans to text with a specific placeholder format
func ApplyMaskingWithFormat(text string, spans []redactor.Span, format PlaceholderFormat) string {
	if len(spans) == 0 {
		return text
	}
	
	// Sort spans by start position
	sort.Slice(spans, func(i, j int) bool {
		return spans[i].Start < spans[j].Start
	})
	
	var result strings.Builder
	lastEnd := 0
	
	for i, span := range spans {
		// Add text before the span
		if span.Start > lastEnd {
			result.WriteString(text[lastEnd:span.Start])
		}
		
		// Add placeholder based on format
		placeholder := formatPlaceholder(span, i+1, format)
		result.WriteString(placeholder)
		
		lastEnd = span.End
	}
	
	// Add remaining text
	if lastEnd < len(text) {
		result.WriteString(text[lastEnd:])
	}
	
	return result.String()
}

// formatPlaceholder creates a placeholder string based on the specified format
func formatPlaceholder(span redactor.Span, ordinal int, format PlaceholderFormat) string {
	switch format {
	case PlaceholderVerbose:
		// [PHI:model:PATIENT#1]
		return fmt.Sprintf("[%s:%s#%d]", strings.ToUpper(string(span.Kind)), span.RuleID, ordinal)
	case PlaceholderCompact:
		// [PATIENT#1] - just the rule ID
		return fmt.Sprintf("[%s#%d]", span.RuleID, ordinal)
	case PlaceholderMinimal:
		// [DATE#1] - extract just the type from rule ID
		ruleType := extractRuleType(span.RuleID)
		return fmt.Sprintf("[%s#%d]", ruleType, ordinal)
	case PlaceholderCustom:
		// [CUSTOM#1] - for future custom formats
		return fmt.Sprintf("[CUSTOM#%d]", ordinal)
	default:
		// Default to verbose
		return fmt.Sprintf("[%s:%s#%d]", strings.ToUpper(string(span.Kind)), span.RuleID, ordinal)
	}
}

// extractRuleType extracts the main type from a rule ID
// e.g., "model:PATIENT" -> "PATIENT", "rule:DATE" -> "DATE"
func extractRuleType(ruleID string) string {
	if idx := strings.LastIndex(ruleID, ":"); idx > 0 {
		return strings.ToUpper(ruleID[idx+1:])
	}
	return strings.ToUpper(ruleID)
}


func detectRulePHI(text string) []redactor.Span {
	spans := make([]redactor.Span, 0, 12)
	add := func(label string, idxs [][]int) {
		for _, m := range idxs {
			spans = append(spans, redactor.Span{
				Start: m[0],
				End:   m[1],
				Kind:  redactor.KindPHI,
				Confidence: 1.0,
				RuleID: "rule:" + label,
			})
		}
	}
	add("EMAIL", reEmail.FindAllStringIndex(text, -1))
	add("OTHERPHI", reURL.FindAllStringIndex(text, -1))
	add("PHONE", rePhoneLand.FindAllStringIndex(text, -1))
	add("PHONE", rePhoneMobile.FindAllStringIndex(text, -1))
	
	for _, m := range reMRN.FindAllStringSubmatchIndex(text, -1) {
		if len(m) >= 4 {
			spans = append(spans, redactor.Span{
				Start: m[2],
				End:   m[3],
				Kind:  redactor.KindPHI,
				Confidence: 1.0,
				RuleID: "rule:ID",
			})
		}
	}
	add("ID", reMedicare.FindAllStringIndex(text, -1))
	add("ID", regexp.MustCompile(`\b[A-Z]{2,10}-[A-Z0-9]{2,10}-[A-Z]{2,10}\b`).FindAllStringIndex(text, -1))
	return spans
}

func overlap(a, b redactor.Span) bool {
	return a.Start < b.End && b.Start < a.End
}

func unionWithPrecedence(ner, rules []redactor.Span) []redactor.Span {
	out := make([]redactor.Span, 0, len(ner)+len(rules))
	out = append(out, ner...)
	for _, r := range rules {
		kept := out[:0]
		for _, n := range out {
			if overlap(n, r) && precedence[strings.TrimPrefix(r.RuleID, "rule:")] >= precedence[strings.TrimPrefix(n.RuleID, "rule:")] {
				continue
			}
			kept = append(kept, n)
		}
		out = kept
		out = append(out, r)
	}
	out = mergeSame(out)
	out = dedupeExact(out)
	return out
}

func growStaffAroundTitles(text string, spans []redactor.Span) []redactor.Span {
	for i := range spans {
		if !strings.Contains(spans[i].RuleID, "STAFF") {
			continue
		}
		for _, t := range []string{"Dr ", "Doctor ", "Nurse ", "Prof ", "Professor "} {
			start := spans[i].Start - len(t)
			if start >= 0 && strings.HasPrefix(text[start:spans[i].Start], t) {
				spans[i].Start = start
				break
			}
		}
		j := spans[i].End
		if j < len(text) && text[j] == '.' {
			j++
			if j < len(text) && text[j] == ' ' {
				j++
			}
		}
		k := j
		for k < len(text) && text[k] == ' ' {
			k++
		}
		w := k
		for w < len(text) {
			c := text[w]
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '-' || c == '\'' {
				w++
			} else {
				break
			}
		}
		if k < w && text[k] >= 'A' && text[k] <= 'Z' {
			spans[i].End = w
		}
	}
	return spans
}

func mergeSame(sp []redactor.Span) []redactor.Span {
	if len(sp) == 0 {
		return sp
	}
	sort.Slice(sp, func(i, j int) bool {
		return sp[i].Start < sp[j].Start
	})
	out := []redactor.Span{sp[0]}
	for _, s := range sp[1:] {
		last := &out[len(out)-1]
		if s.Kind == last.Kind && s.Start <= last.End {
			if s.End > last.End {
				last.End = s.End
			}
		} else {
			out = append(out, s)
		}
	}
	return out
}

func dedupeExact(spans []redactor.Span) []redactor.Span {
	seen := make(map[string]struct{}, len(spans))
	out := make([]redactor.Span, 0, len(spans))
	for _, s := range spans {
		key := fmt.Sprintf("%s:%d:%d", s.Kind, s.Start, s.End)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Start != out[j].Start {
			return out[i].Start < out[j].Start
		}
		if out[i].End != out[j].End {
			return out[i].End < out[j].End
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

func mergeClose(text string, in []redactor.Span) []redactor.Span {
	if len(in) == 0 {
		return in
	}
	sort.Slice(in, func(i, j int) bool {
		return in[i].Start < in[j].Start
	})
	isLight := func(s string) bool {
		s = strings.TrimSpace(s)
		if s == "" {
			return true
		}
		for _, r := range s {
			if !strings.ContainsRune(",/ -", r) {
				return false
			}
		}
		return true
	}
	out := []redactor.Span{in[0]}
	for _, s := range in[1:] {
		last := &out[len(out)-1]
		if s.Kind == last.Kind {
			if s.Start <= last.End {
				if s.End > last.End {
					last.End = s.End
				}
				continue
			}
			if s.Start > last.End && isLight(text[last.End:s.Start]) {
				last.End = s.End
				continue
			}
		}
		out = append(out, s)
	}
	return out
}

func loadID2Label(cfgPath string) (map[int]string, error) {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	var cfg struct {
		Id2Label map[string]string `json:"id2label"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}
	m := make(map[int]string, len(cfg.Id2Label))
	for k, v := range cfg.Id2Label {
		var i int
		fmt.Sscanf(k, "%d", &i)
		m[i] = v
	}
	return m, nil
}