// internal/infra/sttdeepgram/client.go
package sttdeepgram

import (
	"context"
	"encoding/json"
	"strconv"

	"net/http"
	"net/url"

	"time"

	"github.com/gorilla/websocket"
	"schma.ai/internal/domain/speech"
	"schma.ai/internal/pkg/logger"
)

// generateTurns groups consecutive words with the same speaker into turns.
// It assumes words are in chronological order.
func generateTurns(words []speech.Word, isFinal bool) []speech.Turn {
    if len(words) == 0 {
        return nil
    }

    turns := make([]speech.Turn, 0, 4)
    currentSpeaker := words[0].Speaker
    currentStart := words[0].Start
    currentWords := make([]speech.Word, 0, 8)
    var currentEnd float32 = words[0].End

    appendTurn := func() {
        if len(currentWords) == 0 {
            return
        }
        turns = append(turns, speech.Turn{
            ID:        "", // optional for streaming; clients can generate if needed
            Speaker:   currentSpeaker,
            Start:     currentStart,
            End:       currentEnd,
            Words:     append([]speech.Word(nil), currentWords...),
            Confidence: 0,
            Final:     isFinal,
        })
    }

    currentWords = append(currentWords, words[0])
    for i := 1; i < len(words); i++ {
        w := words[i]
        if w.Speaker != currentSpeaker {
            // close previous turn
            appendTurn()
            // start new turn
            currentSpeaker = w.Speaker
            currentStart = w.Start
            currentWords = currentWords[:0]
        }
        currentWords = append(currentWords, w)
        currentEnd = w.End
    }
    // flush last turn
    appendTurn()

    return turns
}

const (
	deepgramWS = "wss://api.deepgram.com/v1/listen"
	sampleRate = 16000
)


// compile-time check
var _ speech.STTClient = (*Client)(nil)

type Client struct {
	apiKey     string
	model      string
	dialer     *websocket.Dialer
	HTTPClient *http.Client
	diarization speech.DiarizationConfig
}

// CtxKey is used to pass optional audio hints to the Deepgram client (batch mode)
type CtxKey string

const (
	CtxKeyAudioEncoding CtxKey = "deepgram_audio_encoding" // e.g., mp3, wav, linear16
	CtxKeySampleRate    CtxKey = "deepgram_sample_rate"    // integer Hz for PCM
)

func New(apiKey, model string, diarization speech.DiarizationConfig) *Client {
    c := &Client{
		apiKey: apiKey,
		model:  model,
		dialer: &websocket.Dialer{
			Proxy:            http.ProxyFromEnvironment,
			HandshakeTimeout: 10 * time.Second,
		},
		HTTPClient: &http.Client{Timeout: 120 * time.Second},
		diarization: diarization,
    }
    logger.ServiceDebugf("DEEPGRAM", "Client.New diarization: enable=%v min=%d max=%d", diarization.EnableSpeakerDiarization, diarization.MinSpeakerCount, diarization.MaxSpeakerCount)
    return c
}

// Stream implements speech.STTClient
func (c *Client) Stream(
	ctx context.Context,
	audio <-chan speech.AudioChunk,
) (<-chan speech.Transcript, error) {
    // Span to buffer deepgram stream logs
    span := logger.NewSpan("stt.deepgram.stream", map[string]any{
        "provider":       "deepgram",
        "model":          c.model,
        "sample_rate_hz": sampleRate,
        "interim":        true,
        "punctuate":      true,
		"paragraphs":     true,
        "endpointing_ms": 500,
        "language":       "en-US",
		"diarization":    c.diarization,
    })
    ctx = logger.WithSpan(ctx, span)
    span.Debug("connect", nil)

	logger.ServiceDebugf("STT", "Starting STT stream")

	params := url.Values{
		"model":           []string{c.model},
		"interim_results": []string{"true"},
		"punctuate":       []string{"true"},
		"paragraphs":      []string{"true"},
		"endpointing":     []string{"500"},   // Reduced from 1000ms - faster finals
		"language":        []string{"en-US"}, // Explicit language for faster processing
		"smart_format":    []string{"true"},  // Better formatting
		"diarize":         []string{strconv.FormatBool(c.diarization.EnableSpeakerDiarization)},
		"numerals":        []string{"false"},
	}
	// Optional batch hints: encoding and sample rate
	if v := ctx.Value(CtxKeyAudioEncoding); v != nil {
		if enc, ok := v.(string); ok && enc != "" { params.Set("encoding", enc) }
	}
	if v := ctx.Value(CtxKeySampleRate); v != nil {
		if sr, ok := v.(int); ok && sr > 0 { params.Set("sample_rate", strconv.Itoa(sr)) }
	}
    logger.ServiceDebugf("DEEPGRAM", "WS params: diarize=%s", params.Get("diarize"))

	wslURL := deepgramWS + "?" + params.Encode()
	h := http.Header{"Authorization": []string{"Token " + c.apiKey}}

    ws, _, err := c.dialer.DialContext(ctx, wslURL, h)
    if err != nil {
        span.Debug("ws dial error", map[string]any{"err": err})
        span.Finish("error")
        logger.Errorf("❌ [DEEPGRAM] WebSocket dial error: %v", err)
        return nil, err
    }
    span.Debug("ws connected", map[string]any{"url": wslURL, "diarize": c.diarization.EnableSpeakerDiarization})
    logger.ServiceDebugf("STT", "WebSocket connection established successfully")

	out := make(chan speech.Transcript, 64)
	done := make(chan struct{})

    logger.ServiceDebugf("STT", "Starting uplink and downlink goroutines")
    go uplink(ctx, ws, audio, done)
    go downlink(ctx, ws, out, done, c.diarization.EnableSpeakerDiarization)

	logger.ServiceDebugf("STT", "STT stream started, returning output channel")
	// TODO: Return keepAlive channel for silence service to use
	return out, nil
}

// uplink sends audio chunks to Deepgram
func uplink(
	ctx context.Context,
	ws *websocket.Conn,
	audio <-chan speech.AudioChunk,
	done chan struct{},
) {
	defer func() {
		logger.ServiceDebugf("STT", "Uplink closing WebSocket")
		ws.Close()
	}()

    var totalBytes int
    var sent int
    logger.ServiceDebugf("STT", "Uplink started, waiting for audio chunks")
	for {
		select {
		case <-ctx.Done():
			logger.ServiceDebugf("STT", "Uplink: context cancelled")
			return
		case <-done:
			logger.ServiceDebugf("STT", "Uplink: done signal received, closing uplink")
			return

		case chunk, ok := <-audio:
			if !ok {	
				logger.ServiceDebugf("STT", "Uplink: audio channel closed, sending stop message to Deepgram")
				_ = ws.WriteMessage(websocket.TextMessage, mustJSON(map[string]any{"type": "stop"}))
				logger.ServiceDebugf("STT", "Uplink: stop message sent, closing uplink")
				return
			}
			
			// Check if this is a keep-alive marker
			if string(chunk) == "KEEP_ALIVE_MARKER" {
				logger.ServiceDebugf("STT", "Uplink: Detected keep-alive marker, sending KeepAlive message")
				keepAliveMsg := map[string]string{"type": "KeepAlive"}
				if err := ws.WriteJSON(keepAliveMsg); err != nil {
					logger.ServiceDebugf("STT", "Uplink: Failed to send KeepAlive message: %v", err)
					close(done)
					return
				}
				logger.ServiceDebugf("STT", "Uplink: Successfully sent KeepAlive message to Deepgram")
				continue
			}
			
			if err := ws.WriteMessage(websocket.BinaryMessage, chunk); err != nil {
				logger.ServiceDebugf("STT", "Uplink: Deepgram write error: %v", err)
				close(done)
				return
			}
			totalBytes += len(chunk)
            sent++
            if s := logger.SpanFrom(ctx); s != nil {
                s.Debug("uplink chunk sent", map[string]any{"seq": sent, "bytes": len(chunk), "total_bytes": totalBytes})
            }
		}
	}
}

// downlink receives transcripts from Deepgram
func downlink(
	ctx context.Context,
	ws *websocket.Conn,
	out chan<- speech.Transcript,
    done chan struct{},
    diarize bool,
) {
	defer func() {
		logger.ServiceDebugf("STT", "Downlink closing output and WebSocket")
		close(out)
		logger.ServiceDebugf("STT", "Downlink: closing done channel")
		close(done)
		logger.ServiceDebugf("STT", "Downlink: closing WebSocket")
		ws.Close()
		logger.ServiceDebugf("STT", "Downlink: cleanup completed")
	}()

    var events int
    var transcripts int
    logger.ServiceDebugf("STT", "Downlink started, waiting for messages from Deepgram")
    for {
		_, data, err := ws.ReadMessage()
		if err != nil {
            if s := logger.SpanFrom(ctx); s != nil { s.Debug("downlink read error", map[string]any{"err": err}); s.Finish("error") }
            logger.ServiceDebugf("STT", "Downlink: WebSocket read error: %v", err)
			// Send CloseStream message to Deepgram
			closeMsg := map[string]string{"type": "CloseStream"}
			if err := ws.WriteJSON(closeMsg); err != nil {
				logger.ServiceDebugf("STT", "Downlink: failed to send CloseStream message: %v", err)
			} else {
				logger.ServiceDebugf("STT", "Downlink: sent CloseStream message to Deepgram")
			}
			return
		}

		// TODO: handle diarization
		// TODO: handle turns

		logger.ServiceDebugf("STT", "Diarization: %+v", diarize)
		
        var ev struct {
			Type    string `json:"type"`
			Channel struct {
				Alternatives []struct {
					Transcript string  `json:"transcript"`
					Confidence float32 `json:"confidence"`
                    Words      []struct {
                        Word               string  `json:"word"`
                        Start              float32 `json:"start"`
                        End                float32 `json:"end"`
                        Confidence         float32 `json:"confidence"`
                        Speaker            int     `json:"speaker,omitempty"`
                        SpeakerConfidence  float32 `json:"speaker_confidence,omitempty"`
                        PunctuatedWord     string  `json:"punctuated_word,omitempty"`
                    } `json:"words"`
				} `json:"alternatives"`
			} `json:"channel"`
			IsFinal bool `json:"is_final"`
		}
        if err := json.Unmarshal(data, &ev); err != nil {
            if s := logger.SpanFrom(ctx); s != nil { s.Debug("downlink unmarshal error", map[string]any{"err": err}) }
            logger.Errorf("❌ [STT] Downlink: JSON unmarshal error: %v", err)
			continue
		}
        events++
        if s := logger.SpanFrom(ctx); s != nil { s.Debug("event", map[string]any{"seq": events, "type": ev.Type}) }
        logger.ServiceDebugf("DEEPGRAM", "Downlink: received event: %s", ev.Type)
		
		if ev.Type != "Results" {
			logger.ServiceDebugf("DEEPGRAM", "Downlink: skipping non-Results event: %s", ev.Type)
			continue
		}

		for _, alt := range ev.Channel.Alternatives {
			// Skip empty transcripts
			if alt.Transcript == "" && alt.Confidence == 0 {
				logger.ServiceDebugf("DEEPGRAM", "TIMING: Skipping empty transcript.")
				continue
			}

			deepgramReceiveTime := time.Now()
            tr := speech.Transcript{
				Text:       alt.Transcript,
				IsFinal:    ev.IsFinal,
				Confidence: alt.Confidence,
			}

			logger.ServiceDebugf("DEEPGRAM", "TIMING: Transcript received from Deepgram at %s (final=%t) %q",
				deepgramReceiveTime.Format("15:04:05.000"), tr.IsFinal, tr.Text)


			// Calculate audio duration from word timings (similar to Google STT)
			if len(alt.Words) > 0 {
				// last word's end minus first word's start
				firstWord := alt.Words[0]
				lastWord := alt.Words[len(alt.Words)-1]
				tr.ChunkDurSec = float64(lastWord.End - firstWord.Start)
			}

            for _, w := range alt.Words {
                tr.Words = append(tr.Words, speech.Word{
                    Text:              w.Word,
                    Start:             w.Start,
                    End:               w.End,
                    Confidence:        w.Confidence,
                    PunctuatedWord:    w.PunctuatedWord,
                    Speaker:           strconv.Itoa(w.Speaker),
                    SpeakerConfidence: w.SpeakerConfidence,
                })
            }

            // If diarization is enabled, generate turns grouped by speaker
            if diarize && len(tr.Words) > 0 {
                tr.Turns = generateTurns(tr.Words, tr.IsFinal)
            }

            transcripts++
            preview := tr.Text
            if len(preview) > 80 { preview = preview[:80] + "…" }
            if s := logger.SpanFrom(ctx); s != nil {
                s.Debug("downlink transcript", map[string]any{
                    "seq":        transcripts,
                    "final":      tr.IsFinal,
                    "dur_sec":    tr.ChunkDurSec,
                    "words":      len(tr.Words),
                    "confidence": tr.Confidence,
                    "text":       preview,
                })
            }

            deepgramSendTime := time.Now()
            logger.ServiceDebugf("DEEPGRAM", "Downlink: received transcript: %+v", tr)
            out <- tr
            logger.ServiceDebugf("DEEPGRAM", "TIMING: Transcript sent to pipeline at %s (processing delay: %v)",
                deepgramSendTime.Format("15:04:05.000"), deepgramSendTime.Sub(deepgramReceiveTime))
		}
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
