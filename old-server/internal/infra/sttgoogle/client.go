package sttgoogle

import (
	"context"
	"io"
	"time"

	gstt "cloud.google.com/go/speech/apiv1"
	speechpb "cloud.google.com/go/speech/apiv1/speechpb"
	"schma.ai/internal/domain/speech"
	"schma.ai/internal/pkg/logger"
)

// TODO: move to domain package, could be a shared utility.
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
            ID:        "",
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
            appendTurn()
            currentSpeaker = w.Speaker
            currentStart = w.Start
            currentWords = currentWords[:0]
        }
        currentWords = append(currentWords, w)
        currentEnd = w.End
    }
    appendTurn()

    return turns
}

// TODO: usage in main.go
/**
googleCfg := sttgoogle.Config{
	Encoding:        speechpb.RecognitionConfig_WEBM_OPUS,
	SampleRateHertz: 16000,
	LanguageCode:    "en-US",
	Punctuate:       true,
}

sttPrimary, _ := sttgoogle.New(ctx, googleCfg)

deps := pipeline.Deps{
	STT: sttPrimary, // satisfies speech.STTClient
	// … FP, LLM, etc.
}

*/

// Config for google stt.
// TODO: This should be extended properly.
type Config struct {
	Encoding        speechpb.RecognitionConfig_AudioEncoding
	SampleRateHertz int32
	LanguageCode    string
	Punctuate       bool
	Diarization     speech.DiarizationConfig
}

type Client struct {
	cfg Config
	svc *gstt.Client
}

// New returns an adapter that satisfies speech.STTClient
func New(ctx context.Context, cfg Config, diarization speech.DiarizationConfig) (*Client, error) {
	svc, err := gstt.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return &Client{cfg: Config{
		Encoding:        cfg.Encoding,
		SampleRateHertz: cfg.SampleRateHertz,
		LanguageCode:    cfg.LanguageCode,
		Punctuate:       cfg.Punctuate,
		Diarization:     diarization,
	}, svc: svc}, nil
}

// Stream implements speech.STTClient (domain port).
// uplink - reads AudioChunk's from domain pipeline and sends them.
// downlink - reads StreamingRecognizeResponses, translates them into speech.Transcript, and pushes to outbound channel.
func (c *Client) Stream(
	ctx context.Context,
	in <-chan speech.AudioChunk,
) (<-chan speech.Transcript, error) {
    // Create a span to buffer STT stream debug logs
    span := logger.NewSpan("stt.google.stream", map[string]any{
        "provider":     "google",
        "language":     c.cfg.LanguageCode,
        "sample_rate_hz": c.cfg.SampleRateHertz,
        "punctuate":    c.cfg.Punctuate,
		"diarization": c.cfg.Diarization,
    })
    ctx = logger.WithSpan(ctx, span)

    stream, err := c.svc.StreamingRecognize(ctx)

    if err != nil {
        span.Debug("streamingRecognize init error", map[string]any{"err": err})
        span.Finish("error")
        return nil, err
    }

	// init config message
	err = stream.Send(&speechpb.StreamingRecognizeRequest{
		StreamingRequest: &speechpb.StreamingRecognizeRequest_StreamingConfig{
			StreamingConfig: &speechpb.StreamingRecognitionConfig{
				Config: &speechpb.RecognitionConfig{
					Encoding:                   c.cfg.Encoding,
					SampleRateHertz:            c.cfg.SampleRateHertz,
					LanguageCode:               c.cfg.LanguageCode,
					EnableAutomaticPunctuation: c.cfg.Punctuate,
					EnableWordTimeOffsets:      true,
					DiarizationConfig: &speechpb.SpeakerDiarizationConfig{
						EnableSpeakerDiarization: c.cfg.Diarization.EnableSpeakerDiarization,
						// Note that these are defaults
						MinSpeakerCount:          int32(c.cfg.Diarization.MinSpeakerCount),
						MaxSpeakerCount:          int32(c.cfg.Diarization.MaxSpeakerCount) ,
					},
				},
				InterimResults:  true,
				SingleUtterance: false,
			},
		},
	})

    if err != nil {
        span.Debug("send streaming config failed", map[string]any{"err": err})
        span.Finish("error")
        return nil, err
    }
    span.Debug("sent streaming config", map[string]any{
        "encoding":       c.cfg.Encoding,
        "sample_rate_hz": c.cfg.SampleRateHertz,
        "lang":           c.cfg.LanguageCode,
        "punctuate":      c.cfg.Punctuate,
    })

	out := make(chan speech.Transcript, 64)

    // Uplink goroutine
	go func() {
		defer stream.CloseSend()
        sent := 0
		for chunk := range in {
			_ = stream.Send(&speechpb.StreamingRecognizeRequest{
				StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
					AudioContent: chunk,
				},
			})
            sent++
            if s := logger.SpanFrom(ctx); s != nil {
                s.Debug("uplink chunk sent", map[string]any{"seq": sent, "bytes": len(chunk)})
            } else {
                logger.Debugf("[stt.google.stream] uplink chunk sent seq=%d bytes=%d", sent, len(chunk))
            }
		}
        if s := logger.SpanFrom(ctx); s != nil {
            s.Debug("uplink closed", map[string]any{"sent": sent})
        }
	}()

    // Downlink goroutine
	go func() {
		defer close(out)
        recv := 0
		for {
			resp, err := stream.Recv()
            if err == io.EOF || err != nil {
                if s := logger.SpanFrom(ctx); s != nil {
                    s.Debug("downlink closed", map[string]any{"err": err})
                    s.Finish("ok")
                }
                return
			}

			for _, r := range resp.Results {
				for _, alt := range r.Alternatives {
					googleReceiveTime := time.Now()

					tr := speech.Transcript{
						Text:       alt.Transcript,
						IsFinal:    r.IsFinal,
						Confidence: alt.Confidence, // Google gives a float32 0-1
						Stability:  r.Stability,    // Interim only
						// TODO: add ChunkDurSec
						ChunkDurSec: 0,
					}

                    logger.ServiceDebugf("GOOGLE", "TIMING: Transcript received from Google at %s (final=%t) %q",
                        googleReceiveTime.Format("15:04:05.000"), tr.IsFinal, tr.Text)

					if len(alt.Words) > 0 {
						// last word's end minus first word's start
						dur := alt.Words[len(alt.Words)-1].EndTime.AsDuration() -
							alt.Words[0].StartTime.AsDuration()
						tr.ChunkDurSec = dur.Seconds()
					}

					// Copy word-offsets
					for _, w := range alt.Words {
						tr.Words = append(tr.Words, speech.Word{
							Text:       w.Word,
							Start:      float32(w.StartTime.AsDuration().Seconds()),
							End:        float32(w.EndTime.AsDuration().Seconds()),
                            Confidence: w.Confidence, // per token confidence
                            Speaker:    w.GetSpeakerLabel(), 
						})
					}

                    // If diarization is enabled, generate turns grouped by speaker
                    if c.cfg.Diarization.EnableSpeakerDiarization && len(tr.Words) > 0 {
                        tr.Turns = generateTurns(tr.Words, tr.IsFinal)
                    }

                    // per-transcript span debug
                    recv++
                    preview := tr.Text
                    if len(preview) > 80 {
                        preview = preview[:80] + "…"
                    }
                    if s := logger.SpanFrom(ctx); s != nil {
                        s.Debug("downlink transcript", map[string]any{
                            "seq":        recv,
                            "final":      tr.IsFinal,
                            "dur_sec":    tr.ChunkDurSec,
                            "words":      len(tr.Words),
                            "confidence": tr.Confidence,
                            "text":       preview,
                        })
                    }

                    googleSendTime := time.Now()
                    out <- tr
                    logger.ServiceDebugf("GOOGLE", "TIMING: Transcript sent to pipeline at %s (processing delay: %v)",
                        googleSendTime.Format("15:04:05.000"), googleSendTime.Sub(googleReceiveTime))
				}
			}
		}
	}()

	return out, nil
}

// Close gracefully shuts down underlying gRPC client
func (c *Client) Close() error {
	if c.svc != nil {
		return c.Close()
	}
	return nil
}
