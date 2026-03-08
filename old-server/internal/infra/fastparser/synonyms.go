package fastparser

import (
	"os"
	"sort"

	"github.com/ynqa/wego/pkg/embedding"
	"schma.ai/internal/pkg/logger"
)


type nnItem struct {
	Word       string
	Similarity float64
}

// const ftVecPath = "models/fasttext/cc.en.300.100k.vec"

// topK returns the k nearest neighbours of `w` in `embs`
func topK(embs embedding.Embeddings, w string, k int) []nnItem {
	target, ok := embs.Find(w)
	if !ok {
		return nil
	}

	items := make([]nnItem, 0, k)
	for _, e := range embs {
		if e.Word == w {
			continue
		}
		sim := CosineSimFloat64(target.Vector, e.Vector)
		items = append(items, nnItem{Word: e.Word, Similarity: sim})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Similarity > items[j].Similarity })
	if len(items) > k {
		return items[:k]
	}
	return items
}

// // Loads model in ftModel
func (a *Adapter) loadFastText() {
	// f, err := os.Open("models/fasttext/cc.en.300.100k.bin")
	// base := paths.ModelDir()
	// ftVecPath := filepath.Join(base, "fasttext", "cc.en.300.100k.vec")

	f, err := os.Open(a.synPath)

	if err != nil {
		logger.Errorf("❌ [NLU] Failed to open FastText model: %v", err)
	}

	defer f.Close()

	m, err := embedding.Load(f)

	logger.Infof("ℹ️ [NLU] FastText loaded %d words (first: %s)", len(m), m[0].Word)

	if err != nil {
		logger.Errorf("❌ [NLU] Failed to load FastText model: %v", err)
	}
	a.ftModel = m

}

// used when building runtime index
func (a *Adapter) NeighbourSynonyms(word string, k int) []string {
	// If synonyms are not enabled, return nil
	if !a.synOn {
		return nil
	}

	// Use the model
	a.ftOnce.Do(a.loadFastText)

	items := topK(a.ftModel, word, k)
	logger.Debugf("ℹ️ [AI] Synonyms for '%s': %v", word, items)

	out := make([]string, 0, k)

	// items already sorted highest-similarity first
	// Filter out synonyms with a similarity score less than 0.55
	for _, it := range items {
		if it.Similarity > 0.55 {
			out = append(out, it.Word)
		}
	}

	return out
}
