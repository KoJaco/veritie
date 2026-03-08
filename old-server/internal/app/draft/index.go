package draft

import (
	"strings"

	tok "github.com/sugarme/tokenizer"
	tokpre "github.com/sugarme/tokenizer/pretrained"

	"schma.ai/internal/domain/speech"
	"schma.ai/internal/infra/fastparser"
	"schma.ai/internal/pkg/env"
)

type runtimeFunc struct {
	Name   string
	Vec    []float32
	Params []speech.FunctionParam
	Tokens map[string]struct{}
}

type Index struct {
	funcs      []runtimeFunc
	tokenizer  *tok.Tokenizer
	fp         speech.FastParser // adapter for Embed+Synonyms
	windowSize int
	window     []string
}

const simThreshold = 0.75

func (x *Index) Detect(chunk string) *speech.FunctionCall {
	if chunk == "" {
		return nil
	}

	// ─── 0. update sliding window ─────────────────────────────
	words := strings.Fields(chunk)
	x.window = append(x.window, words...)
	if len(x.window) > x.windowSize {
		x.window = x.window[len(x.window)-x.windowSize:]
	}
	windowStr := strings.Join(x.window, " ")

	// ─── 1. cheap keyword hit ────────────────────────────────
	lower := strings.ToLower(windowStr)
	for _, f := range x.funcs {
		for tok := range f.Tokens {
			if strings.Contains(lower, tok) {
				return &speech.FunctionCall{
					Name:            f.Name,
					Args:            map[string]any{},
					SimilarityScore: 1,
				}
			}
		}
	}

	// ─── 2. embed & cosine sim ───────────────────────────────
	enc, _ := x.tokenizer.Encode(
		tok.NewSingleEncodeInput(tok.NewInputSequence(windowStr)), false)
	ids64 := fastparser.ToInt64(enc.Ids)
	mask := make([]int64, len(ids64))
	for i := range mask {
		mask[i] = 1
	}

	qVec := x.fp.Embed(ids64, mask)

	bestSim, bestIdx := -1.0, -1
	for i, f := range x.funcs {
		sim := fastparser.CosineSim(qVec, f.Vec)
		if sim > bestSim {
			bestSim, bestIdx = sim, i
		}
	}

	if bestIdx != -1 && bestSim >= simThreshold {
		return &speech.FunctionCall{
			Name:            x.funcs[bestIdx].Name,
			Args:            map[string]any{},
			SimilarityScore: bestSim,
		}
	}
	return nil
}

// New builds the per-session index from the function definitions.
func New(defs []speech.FunctionDefinition, fp speech.FastParser, modelDir string) (*Index, error) {
	if modelDir == "" {
		modelDir = env.ModelDir()
	}

	tokPath := modelDir + "/bge/tokenizer.json"
	tokenizer, err := tokpre.FromFile(tokPath)
	if err != nil {
		return nil, err
	}

	stopVerb := map[string]struct{}{
		"update": {}, "create": {}, "set": {}, "add": {}, "remove": {}, "delete": {},
		"the": {}, "of": {}, "a": {}, "an": {}, "to": {}, "for": {}, "in": {},
	}
	keep := func(w string) bool {
		w = strings.ToLower(w)
		if len(w) < 3 {
			return false
		}
		_, bad := stopVerb[w]
		return !bad
	}

	var out []runtimeFunc
	for _, d := range defs {
		desc := d.Description
		if desc == "" {
			desc = d.Name
		}

		enc, _ := tokenizer.Encode(tok.NewSingleEncodeInput(tok.NewInputSequence(desc)), false)
		vec := fp.Embed(fastparser.ToInt64(enc.Ids), make([]int64, len(enc.Ids)))

		// token set
		tokSet := map[string]struct{}{}
		for idx, w := range strings.Fields(strings.ReplaceAll(d.Name, "_", " ")) {
			w = strings.ToLower(w)
			if idx == 0 {
				if _, bad := stopVerb[w]; bad {
					continue
				}
			}
			if keep(w) {
				tokSet[w] = struct{}{}
			}
			for _, nb := range fp.Synonyms(w, 5) {
				nb = strings.ToLower(nb)
				if keep(nb) {
					tokSet[nb] = struct{}{}
				}
			}
		}

		out = append(out, runtimeFunc{
			Name:   d.Name,
			Vec:    vec,
			Params: d.Parameters,
			Tokens: tokSet,
		})
	}

	return &Index{funcs: out, tokenizer: tokenizer, fp: fp, windowSize: 7}, nil
}
