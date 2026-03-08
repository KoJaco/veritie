package speech

import (
	"context"
	"io"
)

/**
* Deepgram Adapter, Google Adapter, Router, Pipeline tests
* -- depend on this contract
*
* Zero imports outside std
 */

// Raw 16-bit PCM, little-endian, 1 chan
type AudioChunk []byte

// Transcript is provider-agnostic; only timing

type Word struct {
	Text           string  `json:"text"`
	Start          float32 `json:"start"` // seconds from start of audio
	End            float32 `json:"end"`   // seconds from end of audio
	Confidence     float32 `json:"confidence,omitempty"`
	PunctuatedWord string  `json:"punctuated_word,omitempty"`
	Speaker           string  `json:"speaker,omitempty"` // Streaming
	SpeakerConfidence float32 `json:"speaker_confidence,omitempty"` // only for batch processes.

}

// Phase is derived from final transcript chunk's words.
type Phrase struct {
	Speaker string `json:"speaker,omitempty"`
	Start float32 `json:"start,omitempty"` // first word start
	End float32 `json:"end,omitempty"` // last word end
	TextNorm string `json:"text_norm,omitempty"` // normalized (unredacted) for UI;
	TextRedacted string `json:"text_redacted,omitempty"` // redacted for LLM; post norm
}

// Storage-firendly
type PhraseLight struct {
	Speaker string `json:"speaker,omitempty"`
	Start float32 `json:"start,omitempty"`
	End float32 `json:"end,omitempty"`
	TextRedacted string `json:"text_redacted,omitempty"`
	// masked text gets appended into the session transcript instead
}

// Maps of turns 
type Turn struct {
	ID string `json:"id"`
	Speaker string `json:"speaker"`
	Start float32 `json:"start"`
	End float32 `json:"end"`
	Confidence float32 `json:"confidence,omitempty"`
	Words []Word `json:"words,omitempty"` // optional: could just use start and end
	Final bool `json:"final,omitempty"`
}


type Transcript struct {
	Text        string  `json:"text"`
	IsFinal     bool    `json:"final"`
	Confidence  float32 `json:"confidence,omitempty"`
	Stability   float32 `json:"stability,omitempty"`
	Words       []Word  `json:"words,omitempty"`
	ChunkDurSec float64 `json:"chunk_dur_sec,omitempty"`
	PhrasesDisplay []Phrase `json:"phrases_display,omitempty"`
	Phrases []PhraseLight `json:"phrases,omitempty"`
	// Diarization (optional)
	Turns []Turn `json:"turns,omitempty"` // optional: can be derived from words in client instead I guess?
	Channel int `json:"channel,omitempty"` // optional: for multi-channel paths
}

type DiarizationConfig struct {
	EnableSpeakerDiarization bool `json:"enable_speaker_diarization"`
	MinSpeakerCount int `json:"min_speaker_count,omitempty"`
	MaxSpeakerCount int `json:"max_speaker_count,omitempty"`
}

// STTClient is the single interface the *app* knows about.
type STTClient interface {
	// Stream starts a bidirectional session.
	// * in - raw PCM from mic (SDK closes when done)
	// * out - incremental transcripts (closed by adapter on ctx cancel)
	Stream(ctx context.Context, in <-chan AudioChunk) (<-chan Transcript, error)
}

type STTBatchClient interface {
	TranscribeFile(ctx context.Context, r io.Reader) (Transcript, error)
}
