package sttnormalizer

import (
	"context"
	"time"

	"schma.ai/internal/domain/normalizer"
)




type req struct {
	text string
	out chan string
	err chan error
}

// Batcher wraps Client and coalesces calls for 2-5ms to amortize overhead
type Batcher struct {
	c *Client
	in chan req
	maxBatch int
	window time.Duration
}

type BatcherConfig struct {
	Client *Client
	MaxBatch int
	Window time.Duration
}

func NewBatcher(cfg BatcherConfig) *Batcher {
	if cfg.MaxBatch <= 0 { cfg.MaxBatch = 16 } // defaults
	if cfg.Window <= 0 { cfg.Window = 3 * time.Millisecond}

	b := &Batcher{
		c: cfg.Client,
		in: make(chan req, 1024),
		maxBatch: cfg.MaxBatch,
		window: cfg.Window,
	}

	go b.loop()
	return b
}

func (b *Batcher) loop() {
	t := time.NewTicker(b.window)
	defer t.Stop()

	var buf []req

	flush := func() {
		if len(buf) == 0 { return}
		texts := make([]string, len(buf))
		// quick set all req text
		for i, r := range buf { texts[i] = r.text}

		ctx, cancel := context.WithTimeout(context.Background(), 2 * time.Second)
		out, err := b.c.NormalizeBatch(ctx, texts)
		cancel()

		for i, r := range buf {
			if err != nil {
				r.err <- err
			} else {
				r.out <- out[i]
				close(r.out); close(r.err)
			}
		}

		buf = buf[:0]
	}

	for {
		select {
		case r := <- b.in:
			buf = append(buf, r)
			if len(buf) >= b.maxBatch { flush()}
		case <- t.C:
			flush()
		}
	}

}


func (b *Batcher) Normalize(ctx context.Context, text string) (string, error) {

	r := req{text: text, out: make(chan string, 1), err: make(chan error, 1)}

	select {
	case b.in <- r:
	case <-ctx.Done():
		return "", ctx.Err()
	}

	select {
	case s := <-r.out:
		return s, nil
	case e := <-r.err:
		return "", e
	case <-ctx.Done():
		return "", ctx.Err()
	}
}


func (b *Batcher) NormalizeBatch(ctx context.Context, texts []string) ([]string, error) {
	return b.c.NormalizeBatch(ctx, texts)
}

func (b *Batcher) Healthy(ctx context.Context) bool { return b.c.Healthy(ctx)}

var _ normalizer.Normalizer = (*Batcher)(nil)