package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	onnx "github.com/yalue/onnxruntime_go"
)

var (
	// Emails/URLs (simple + robust)
	reEmail = regexp.MustCompile(`(?i)\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b`)
	reURL   = regexp.MustCompile(`(?i)\bhttps?://\S+\b`)

    // +61 2|3|7|8 … or 02|03|07|08 …, with optional parentheses: (02) 9123 4567
    rePhoneLand   = regexp.MustCompile(`(?i)(?:\+61\s?[2378]|\(?0[2378]\)?)\s?\d{4}\s?\d{4}`)
    // +61 4xx … or 04xx …
    rePhoneMobile = regexp.MustCompile(`(?i)(?:\+61\s?4\d{2}|04\d{2})\s?\d{3}\s?\d{3}`)
	// IDs: MRN-like digits, Medicare pattern, and generic “RPT-2025-000123” shape
	reMRN       = regexp.MustCompile(`\b(?:MRN[:\s]*)?(\d{7,10})\b`)
	reMedicare  = regexp.MustCompile(`\b\d{4}\s?\d{5}\s?\d\b`)
)

var precedence = map[string]int{
    "EMAIL":  90,
    "ID":     88,  // ↑ higher than PHONE so "ACME-778-XY" wins as ID
    "PHONE":  85,
    "OTHERPHI": 80,
    "DATE":   60, "PATIENT": 55, "STAFF": 55, "HOSP": 55, "PATORG": 50, "LOC": 45,
}

type Span struct{ Label string; Start, End int }

func detectRulePHI(text string) []Span {
    spans := make([]Span, 0, 12)
    add := func(label string, idxs [][]int) {
        for _, m := range idxs { spans = append(spans, Span{Label: label, Start: m[0], End: m[1]}) }
    }
    add("EMAIL",    reEmail.FindAllStringIndex(text, -1))
    add("OTHERPHI", reURL.FindAllStringIndex(text, -1))
    add("PHONE",    rePhoneLand.FindAllStringIndex(text, -1))
    add("PHONE",    rePhoneMobile.FindAllStringIndex(text, -1))
    // MRN / Medicare / generic IDs
    for _, m := range reMRN.FindAllStringSubmatchIndex(text, -1) {
        if len(m) >= 4 { spans = append(spans, Span{Label: "ID", Start: m[2], End: m[3]}) }
    }
    add("ID", reMedicare.FindAllStringIndex(text, -1))
	add("ID", regexp.MustCompile(`\b[A-Z]{2,10}-[A-Z0-9]{2,10}-[A-Z]{2,10}\b`).FindAllStringIndex(text, -1))
    return spans
}


func overlap(a, b Span) bool { return a.Start < b.End && b.Start < a.End }

func unionWithPrecedence(ner, rules []Span) []Span {
	out := make([]Span, 0, len(ner)+len(rules))
	out = append(out, ner...)
	for _, r := range rules {
		kept := out[:0]
		for _, n := range out {
			if overlap(n, r) && precedence[r.Label] >= precedence[n.Label] {
				// drop n (rule wins)
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

func growStaffAroundTitles(text string, spans []Span) []Span {
	for i := range spans {
		if spans[i].Label != "STAFF" { continue }
		// extend left to include "Dr ", "Nurse ", "Prof "
		for _, t := range []string{"Dr ", "Doctor ", "Nurse ", "Prof ", "Professor "} {
			start := spans[i].Start - len(t)
			if start >= 0 && strings.HasPrefix(text[start:spans[i].Start], t) {
				spans[i].Start = start
				break
			}
		}
		// extend right to include a following capitalized surname (simple heuristic)
		j := spans[i].End
		// skip optional ". "
		if j < len(text) && text[j] == '.' {
			j++
			if j < len(text) && text[j] == ' ' { j++ }
		}
		// include next capitalized word
		k := j
		for k < len(text) && text[k] == ' ' { k++ }
		w := k
		for w < len(text) {
			c := text[w]
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '-' || c == '\'' {
				w++
			} else { break }
		}
		if k < w && text[k] >= 'A' && text[k] <= 'Z' {
			spans[i].End = w
		}
	}
	return spans
}

func must[T any](v T, err error) T { if err != nil { panic(err) }; return v }

func loadID2Label(cfgPath string) map[int]string {
	b := must(os.ReadFile(cfgPath))
	var cfg struct{ Id2Label map[string]string `json:"id2label"` }
	must(0, json.Unmarshal(b, &cfg))
	m := make(map[int]string, len(cfg.Id2Label))
	for k, v := range cfg.Id2Label {
		var i int
		fmt.Sscanf(k, "%d", &i)
		m[i] = v
	}
	return m
}

func mergeClose(text string, in []Span) []Span {
    if len(in) == 0 { return in }
    sort.Slice(in, func(i, j int) bool { return in[i].Start < in[j].Start })
    isLight := func(s string) bool {
        s = strings.TrimSpace(s)
        if s == "" { return true }
        for _, r := range s {
            if !strings.ContainsRune(",/ -", r) { return false }
        }
        return true
    }
    out := []Span{in[0]}
    for _, s := range in[1:] {
        last := &out[len(out)-1]
        if s.Label == last.Label {
            if s.Start <= last.End { // overlap
                if s.End > last.End { last.End = s.End }
                continue
            }
            // only light separators between spans?
            if s.Start > last.End && isLight(text[last.End:s.Start]) {
                last.End = s.End
                continue
            }
        }
        out = append(out, s)
    }
    return out
}

func dedupeExact(spans []Span) []Span {
	seen := make(map[string]struct{}, len(spans))
	out := make([]Span, 0, len(spans))
	for _, s := range spans {
		key := s.Label + ":" + fmt.Sprint(s.Start) + ":" + fmt.Sprint(s.End)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	// keep order stable: sort by start, then label
	sort.Slice(out, func(i, j int) bool {
		if out[i].Start != out[j].Start { return out[i].Start < out[j].Start }
		if out[i].End != out[j].End { return out[i].End < out[j].End }
		return out[i].Label < out[j].Label
	})
	return out
}

func mergeSame(sp []Span) []Span {
	if len(sp) == 0 { return sp }
	sort.Slice(sp, func(i, j int) bool { return sp[i].Start < sp[j].Start })
	out := []Span{sp[0]}
	for _, s := range sp[1:] {
		last := &out[len(out)-1]
		if s.Label == last.Label && s.Start <= last.End {
			if s.End > last.End { last.End = s.End }
		} else { out = append(out, s) }
	}
	return out
}
func expandWords(text string, sp []Span) []Span {
	isB := func(i int) bool {
		if i <= 0 || i >= len(text) { return true }
		ch := text[i]
		return ch == ' ' || strings.ContainsRune(",.;:!?)](", rune(ch))
	}
	for i := range sp {
		for sp[i].Start > 0 && !isB(sp[i].Start-1) { sp[i].Start-- }
		for sp[i].End < len(text) && !isB(sp[i].End) { sp[i].End++ }
	}
	return sp
}
func mask(text string, sp []Span) string {
	if len(sp) == 0 { return text }
    var b strings.Builder
    cur := 0
    prevTag := ""
    for _, s := range sp {
        if s.Start > cur {
            b.WriteString(text[cur:s.Start])
            prevTag = "" // reset when we write plain text
        }
        tag := "[" + s.Label + "]"
        if tag != prevTag {
            b.WriteString(tag)
            prevTag = tag
        }
        cur = s.End
    }
    if cur < len(text) { b.WriteString(text[cur:]) }
    return b.String()
}

/************ UDS tokenizer client ************/
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
			// ignore network/addr; always dial the unix socket
			d := net.Dialer{Timeout: 2 * time.Second}
			return d.DialContext(ctx, "unix", sock)
		},
		MaxIdleConns:       100,
		IdleConnTimeout:    60 * time.Second,
		DisableCompression: true,
	}
	return &tokClient{
		h:   &http.Client{Transport: tr, Timeout: 5 * time.Second},
		url: "http://unix", // host doesn't matter over UDS
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

/************ MAIN ************/
func main() {
	// --- paths: set these for your environment ---
	modelDir := "/home/kori/dev/business/memonic/server/models" // or "./models/phi_roberta_onnx"
	modelPath := filepath.Join(modelDir, "phi_roberta_onnx_int8", "model_quantized.onnx")
	cfgPath := filepath.Join(modelDir, "phi_roberta_onnx_int8", "config.json")

	onnx.SetSharedLibraryPath("/home/kori/dev/business/memonic/server/runtime/onnxruntime-linux-x64-1.17.0/lib/libonnxruntime.so.1.17.0") 

	// 1) id2label / numLabels
	id2label := loadID2Label(cfgPath)
	numLabels := int64(len(id2label))

	// 2) Tokenize via sidecar over /tmp/tok.sock
	tc := newUDSClient("/tmp/tok.sock")
	
	// 3) ONNX env + dynamic session
	must(0, onnx.InitializeEnvironment())
	sess, err := onnx.NewDynamicSession[int64, float32](
		modelPath,
		[]string{"input_ids", "attention_mask"},
		[]string{"logits"},
	)
	if err != nil { panic(err) }
	defer sess.Destroy()

		// --- the 9 extra examples (add your original one if you like) ---
	examples := []string{
		"Michael Nguyen (DOB: 2001-07-14, age 23) presented to Royal North Shore Hospital, Sydney NSW. Contact: 0401 234 567; email michael.nguyen@example.com. MRN 12345678.",
		"Seen by Dr A. Patel in Outpatient Clinic 3B, Alfred Hospital, 55 Commercial Rd Melbourne VIC, on 14/02/2024.",
		"Next of kin: Sarah O'Connor (wife) — phone +61 2 9012 3456, alt +61 412 345 678; address 12 King St, Newtown NSW 2042.",
		"Insurance: Medicare Australia number 5950 12345 1; policy ACME-778-XY; employer Acme Home Care Pty Ltd.",
		"Patient username johnsmith1980; portal https://portal.northernbeacheshospital/login; support email help@hospital.example.",
		"8-year-old male referred by Nurse Kelly from St Vincent's Hospital; appointment 03-Nov-2023.",
		"Radiology report for Jane Doe reviewed by Dr Chen at 15:15 on 05/06/2025; internal ID RPT-2025-000123; pager (02) 5550 1999.",
		"Discharged to 22A/7 Harbour View Apartments, Darling Harbour NSW 2000; follow-up at Prince of Wales Hospital.",
		"Contact GP Dr Emily Wright at emily.wright@gpclinic.au; fax (02) 9311 2233; clinic Greenwood Family Practice.",
	}

	for i, text := range examples {

		inIDs, attn, offsets, err := tc.Encode(text, 512)
		if err != nil { fmt.Printf("example %d encode error: %v\n", i+1, err); continue }




		// 4) Tensors
		shape := onnx.NewShape(1, int64(len(inIDs)))
		idT := must(onnx.NewTensor[int64](shape, inIDs))
		mskT := must(onnx.NewTensor[int64](shape, attn))
		outT := must(onnx.NewEmptyTensor[float32](onnx.NewShape(1, int64(len(inIDs)), numLabels)))
		defer idT.Destroy(); defer mskT.Destroy(); defer outT.Destroy()
	
		// Warm + latency
		_ = sess.Run([]*onnx.Tensor[int64]{idT, mskT}, []*onnx.Tensor[float32]{outT})
		const N = 10; t0 := time.Now()
		for i := 0; i < N; i++ {
			_ = sess.Run([]*onnx.Tensor[int64]{idT, mskT}, []*onnx.Tensor[float32]{outT})
		}
		avgMs := time.Since(t0).Seconds()*1000/float64(N)
	
		// 5) logits → argmax
		logits := outT.GetData() // flat [1*seq*labels]
		seq := len(inIDs); nl := int(numLabels)
		preds := make([]int, seq)
		for i := 0; i < seq; i++ {
			base := i * nl
			maxI := 0; maxV := logits[base]
			for j := 1; j < nl; j++ {
				if v := logits[base+j]; v > maxV { maxV, maxI = v, j }
			}
			preds[i] = maxI
		}
	
		// 6) Decode BIO/BILOU → spans (skip specials with zero-length offsets)
		var spans []Span
		var cur *Span
		for i := 0; i < seq && i < len(offsets); i++ {
			start, end := offsets[i][0], offsets[i][1]
			if end <= start { // special token
				if cur != nil { spans = append(spans, *cur); cur = nil }
				continue
			}
			lab := id2label[preds[i]] // e.g., "B-STAFF", "L-DATE", "O"
			switch {
			case strings.HasPrefix(lab, "B-") || strings.HasPrefix(lab, "U-"):
				if cur != nil { spans = append(spans, *cur); cur = nil }
				typ := lab[2:]
				if strings.HasPrefix(lab, "U-") {
					spans = append(spans, Span{typ, start, end})
				} else { cur = &Span{typ, start, end} }
			case strings.HasPrefix(lab, "I-") || strings.HasPrefix(lab, "L-"):
				if cur != nil {
					cur.End = end
					if strings.HasPrefix(lab, "L-") { spans = append(spans, *cur); cur = nil }
				}
			default:
				if cur != nil { spans = append(spans, *cur); cur = nil }
			}
		}
		if cur != nil { spans = append(spans, *cur) }
	
		// 7) Merge / expand / mask
		// 1) merge/decode spans from the model as you already do...
		spans = mergeSame(spans)
		spans = dedupeExact(spans)

		// 2) add rule hits with precedence
		ruleSpans := detectRulePHI(text)
		spans = unionWithPrecedence(spans, ruleSpans)

		// 3) (optional) tidy STAFF spans that are mid-name
		spans = growStaffAroundTitles(text, spans)
		
		spans = mergeClose(text, spans)

		// 4) final expand + mask
		spans = expandWords(text, spans)
		masked := mask(text, spans)

		fmt.Printf("\n---- example %d ----\n", i+1)
		fmt.Printf("avg latency: %.2f ms\n", avgMs)
		fmt.Println("spans:", spans)
		fmt.Println("masked:", masked)

		idT.Destroy(); mskT.Destroy(); outT.Destroy()

	}



}
