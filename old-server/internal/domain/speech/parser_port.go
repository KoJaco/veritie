package speech

type Entity struct {
	Token string
	Score float64
}

// FastParse exposes the single capability the pipeline needs
type FastParser interface {
	// Embed returns a dense vector for a sentence (k-NM)
	Embed(ids, mask []int64) []float32

	// Synonyms return up to k neighbours for a word or nil if disabled
	Synonyms(word string, k int) []string
}
