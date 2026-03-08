package sttdeepgram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"net/http"
	"strings"
	"time"

	"schma.ai/internal/domain/speech"
	"schma.ai/internal/pkg/logger"
)

const preRecorderURL = "https://api.deepgram.com/v1/listen?model=nova-3&smart_format=true"

var _ speech.STTBatchClient = (*BatchClient)(nil)

type BatchClient struct {
	apiKey string
	model  string
	HTTPClient *http.Client
	diarization speech.DiarizationConfig
}

func NewBatchClient(apiKey, model string, diarization speech.DiarizationConfig) *BatchClient {
	bc := &BatchClient{
		apiKey: apiKey,
		model: model,
		HTTPClient: &http.Client{Timeout: 120 * time.Second},
		diarization: diarization,
	}

	logger.ServiceDebugf("STT", "BatchClient.New diarization: enable=%v min=%d max=%d", diarization.EnableSpeakerDiarization, diarization.MinSpeakerCount, diarization.MaxSpeakerCount)
	return bc
}

// batchResp is inline with JSON received from deepgram
// https://developers.deepgram.com/docs/pre-recorded-audio
// Strip out metadata
type batchResp struct {
	Results struct {
		Channels []struct {
			Alternatives []struct {
				Transcript string  `json:"transcript"`
				Confidence float32 `json:"confidence"`
				Words      []struct {
					Word           string  `json:"word"`
					Start          float32 `json:"start"`
					End            float32 `json:"end"`
					Confidence     float32 `json:"confidence"`
					PunctuatedWord string  `json:"punctuated_word"`
				} `json:"words"`
			} `json:"alternatives"`
		} `json:"channels"`
	} `json:"results"`
}

// TranscribeFile implements BatchTranscriber
func (bc *BatchClient) TranscribeFile(ctx context.Context, r io.Reader) (speech.Transcript, error) {
	// Build request
	req, _ := http.NewRequestWithContext(ctx, "POST", preRecorderURL, r)

	// Determine Content-Type from context hint when available
	contentType := "application/octet-stream"
	if v := ctx.Value(CtxKeyAudioEncoding); v != nil {
		if enc, ok := v.(string); ok {
			switch strings.ToLower(enc) {
			case "mp3": contentType = "audio/mpeg"
			case "wav": contentType = "audio/wav"
			case "webm": contentType = "audio/webm"
			case "ogg": contentType = "audio/ogg"
			case "m4a": contentType = "audio/mp4"
			case "flac": contentType = "audio/flac"
			case "aac": contentType = "audio/aac"
			}
		}
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Token "+bc.apiKey)
	logger.ServiceDebugf("STT", "POST %s content_type=%s", preRecorderURL, contentType)

	start := time.Now()
	res, err := bc.HTTPClient.Do(req)
	if err != nil {
		return speech.Transcript{}, err
	}
	defer res.Body.Close()
	logger.ServiceDebugf("STT", "status=%d", res.StatusCode)

	respBytes, _ := io.ReadAll(res.Body)
	rawPreview := string(respBytes)
	if len(rawPreview) > 512 { rawPreview = rawPreview[:512] + "…" }
	logger.ServiceDebugf("STT", "raw_len=%d preview=%s", len(respBytes), rawPreview)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var body map[string]any
		_ = json.Unmarshal(respBytes, &body)
		return speech.Transcript{}, fmt.Errorf("deepgram batch error: status=%d body=%v", res.StatusCode, body)
	}

	var br batchResp
	if err := json.Unmarshal(respBytes, &br); err != nil {
		return speech.Transcript{}, err
	}

	// Collapse deepgram structure -> single transcript (prefer first channel/alternative)
	var tr speech.Transcript
	chanCount := len(br.Results.Channels)
	altCount := 0
	if chanCount > 0 { altCount = len(br.Results.Channels[0].Alternatives) }
	logger.ServiceDebugf("STT", "parsed channels=%d alternatives_in_ch0=%d", chanCount, altCount)

	if chanCount > 0 && altCount > 0 {
		alt := br.Results.Channels[0].Alternatives[0]
		tr.Text = strings.TrimSpace(alt.Transcript)
		tr.Confidence = alt.Confidence
		for _, w := range alt.Words {
			tr.Words = append(tr.Words, speech.Word{
				Text:           w.Word,
				Start:          w.Start,
				End:            w.End,
				Confidence:     w.Confidence,
				PunctuatedWord: w.PunctuatedWord,
			})
		}
		// Derive duration from word timings if available
		if len(alt.Words) > 0 {
			first := alt.Words[0]
			last := alt.Words[len(alt.Words)-1]
			tr.ChunkDurSec = float64(last.End - first.Start)
		}
		preview := tr.Text
		if len(preview) > 120 { preview = preview[:120] + "…" }
		logger.ServiceDebugf("STT", "transcript_len=%d confidence=%.3f preview=%s", len(tr.Text), tr.Confidence, preview)
	} else {
		logger.Warnf("⚠️ [DEEPGRAM-BATCH] No transcript alternatives returned")
	}

	tr.IsFinal = true
	// Fallback duration if no words
	if tr.ChunkDurSec == 0 {
		tr.ChunkDurSec = float64(time.Since(start)) / float64(time.Second)
	}

	return tr, nil
}
