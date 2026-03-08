package sttnormalizer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"time"

	"schma.ai/internal/domain/normalizer"
)

var _ normalizer.Normalizer = (*Client)(nil)

type Client struct {
	http *http.Client
	url string
}

type Config struct {
	UDSPath string // "/tmp/norm.sock"
	HTTPTimeout time.Duration // default 5s  
}

func New(cfg Config) *Client {
	if cfg.HTTPTimeout == 0 { cfg.HTTPTimeout = 5 * time.Second}

	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: cfg.HTTPTimeout}
			return d.DialContext(ctx, "unix", cfg.UDSPath)
		}, 
		MaxIdleConns: 128,
		IdleConnTimeout: 60 * time.Second,
	}

	return &Client{
		http: &http.Client{
			Transport: tr, Timeout: cfg.HTTPTimeout,
		
		}, url: "http://unix",

	}
}

// Health probe
func (c *Client) Healthy(ctx context.Context) bool {
	if _, err := os.Stat(getUDSPathFromTransport(c.http.Transport)); err != nil { return false }
	req, _ := http.NewRequestWithContext(ctx, "GET", c.url+"/healthz", nil)
	resp, err := c.http.Do(req)

	if err != nil { return false}

	_ = resp.Body.Close()
	return resp.StatusCode == 200
}

// Normalize a single string, utlise batch norm
func (c *Client) Normalize(ctx context.Context, text string) (normalized string, err error) {
	out, err := c.NormalizeBatch(ctx, []string{text})

	if err != nil { return "", err }
	return out[0], nil
}

// TODO: Add support for norm parameters (day_first, more to be added later) from config.
// Main normalisation function
func (c *Client) NormalizeBatch(ctx context.Context, texts []string) (normalized []string, err error) {
	if len(texts) == 0 { return nil, errors.New("empty input")}

	body, _ := json.Marshal(map[string]any{"texts": texts, "day_first": true})
	req, _ := http.NewRequestWithContext(ctx, "POST", c.url+"/norm_batch", bytes.NewReader(body))
	req.Header.Set("Content-type", "application/json")

	res, err := c.http.Do(req)

	if err != nil { return nil, err }

	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, errors.New("norm sidecar responded non-200")
	}

	var resp struct{Texts []string `json:"texts"`}

	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil { return nil, err}

	if len(resp.Texts) != len(texts) {
		return nil, errors.New("norm sidecar returned mismatched length of texts")
	}

	return resp.Texts, nil
}


// Extract UDS path from the transport (we know our dialer uses it)
func getUDSPathFromTransport(Tr http.RoundTripper) string {
	return "/tmp/norm.sock"
}
