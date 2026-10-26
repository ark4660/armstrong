// Package download fetches files in parallel, verifies them by sha1 and size,
// retries with backoff and writes through a .part file so a file on disk is
// always either complete or absent.
package download

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// Item is one file to download. SHA1 and Size are optional; when set, an
// existing file that matches is skipped and a download that doesn't match fails.
type Item struct {
	URL        string `json:"url"`
	Path       string `json:"-"`
	SHA1       string `json:"sha1"`
	Size       int64  `json:"size"`
	Executable bool   `json:"-"`
}

// Progress is reported after each finished item.
type Progress struct {
	Done, Total int
	Bytes       int64 // bytes downloaded so far (skipped files don't count)
}

type Client struct {
	HTTP      *http.Client
	UserAgent string
	Workers   int
	Retries   int
}

func New(userAgent string) *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 5 * time.Minute},
		UserAgent: userAgent,
		Workers:   16,
		Retries:   4,
	}
}

// permanentError marks failures that retrying won't fix (404, hash mismatch).
type permanentError struct{ error }

func (e permanentError) Unwrap() error { return e.error }

// Fetch downloads every item that isn't already present and valid.
// onProgress may be nil; it's called from worker goroutines, one call at a time.
func (c *Client) Fetch(ctx context.Context, items []Item, onProgress func(Progress)) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		done     atomic.Int64
		bytes    atomic.Int64
		mu       sync.Mutex
		firstErr error
		wg       sync.WaitGroup
	)
	jobs := make(chan Item)
	report := func() {
		if onProgress == nil {
			return
		}
		mu.Lock()
		onProgress(Progress{Done: int(done.Load()), Total: len(items), Bytes: bytes.Load()})
		mu.Unlock()
	}

	workers := max(c.Workers, 1)
	for range workers {
		wg.Go(func() {
			for it := range jobs {
				n, err := c.fetchOne(ctx, it, false)
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("%s: %w", it.URL, err)
					}
					mu.Unlock()
					cancel()
					continue
				}
				bytes.Add(n)
				done.Add(1)
				report()
			}
		})
	}

feed:
	for _, it := range items {
		select {
		case jobs <- it:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()

	if firstErr != nil {
		return firstErr
	}
	return ctx.Err()
}

// fetchOne downloads it unless it's already valid; force re-downloads anyway.
func (c *Client) fetchOne(ctx context.Context, it Item, force bool) (int64, error) {
	if !force && Valid(it.Path, it.SHA1, it.Size) {
		return 0, nil
	}
	var err error
	for attempt := range c.Retries + 1 {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(1<<(attempt-1)) * time.Second):
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		}
		var n int64
		n, err = c.downloadTo(ctx, it)
		if err == nil {
			return n, nil
		}
		if errors.As(err, new(permanentError)) || ctx.Err() != nil {
			break
		}
	}
	return 0, err
}

func (c *Client) downloadTo(ctx context.Context, it Item) (int64, error) {
	body, err := c.open(ctx, it.URL)
	if err != nil {
		return 0, err
	}
	defer body.Close()

	if err := os.MkdirAll(filepath.Dir(it.Path), 0o755); err != nil {
		return 0, permanentError{err}
	}
	part := it.Path + ".part"
	mode := os.FileMode(0o644)
	if it.Executable {
		mode = 0o755
	}
	f, err := os.OpenFile(part, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return 0, permanentError{err}
	}
	h := sha1.New()
	n, copyErr := io.Copy(io.MultiWriter(f, h), body)
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil {
		os.Remove(part)
		return 0, errors.Join(copyErr, closeErr)
	}
	if it.Size > 0 && n != it.Size {
		os.Remove(part)
		return 0, fmt.Errorf("size mismatch: got %d, want %d", n, it.Size)
	}
	if it.SHA1 != "" {
		if got := hex.EncodeToString(h.Sum(nil)); got != it.SHA1 {
			os.Remove(part)
			return 0, fmt.Errorf("sha1 mismatch: got %s, want %s", got, it.SHA1)
		}
	}
	if err := os.Rename(part, it.Path); err != nil {
		os.Remove(part)
		return 0, permanentError{err}
	}
	return n, nil
}

func (c *Client) open(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, permanentError{err}
	}
	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		err := fmt.Errorf("HTTP %s", resp.Status)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			return nil, permanentError{err}
		}
		return nil, err
	}
	return resp.Body, nil
}

// GetJSON fetches url (with retries) and decodes it into v.
func (c *Client) GetJSON(ctx context.Context, url string, v any) error {
	var err error
	for attempt := range c.Retries + 1 {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(1<<(attempt-1)) * time.Second):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		var body io.ReadCloser
		body, err = c.open(ctx, url)
		if err == nil {
			err = json.NewDecoder(body).Decode(v)
			body.Close()
			if err == nil {
				return nil
			}
		}
		if errors.As(err, new(permanentError)) || ctx.Err() != nil {
			break
		}
	}
	return fmt.Errorf("%s: %w", url, err)
}

// Get downloads a whole (small) response body, with retries.
func (c *Client) Get(ctx context.Context, url string, maxBytes int64) ([]byte, error) {
	var err error
	for attempt := range c.Retries + 1 {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(1<<(attempt-1)) * time.Second):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		var body io.ReadCloser
		body, err = c.open(ctx, url)
		if err == nil {
			var data []byte
			data, err = io.ReadAll(io.LimitReader(body, maxBytes+1))
			body.Close()
			if err == nil {
				if int64(len(data)) > maxBytes {
					return nil, fmt.Errorf("%s: larger than %d bytes", url, maxBytes)
				}
				return data, nil
			}
		}
		if errors.As(err, new(permanentError)) || ctx.Err() != nil {
			break
		}
	}
	return nil, fmt.Errorf("%s: %w", url, err)
}

// PostJSON sends body as JSON (with retries) and decodes the response into out.
func (c *Client) PostJSON(ctx context.Context, url string, body, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	for attempt := range c.Retries + 1 {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(1<<(attempt-1)) * time.Second):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", c.UserAgent)
		req.Header.Set("Content-Type", "application/json")
		var resp *http.Response
		resp, err = c.HTTP.Do(req)
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				err = json.NewDecoder(resp.Body).Decode(out)
				resp.Body.Close()
				if err == nil {
					return nil
				}
			} else {
				resp.Body.Close()
				err = fmt.Errorf("HTTP %s", resp.Status)
				if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
					break
				}
			}
		}
		if ctx.Err() != nil {
			break
		}
	}
	return fmt.Errorf("%s: %w", url, err)
}

// CachedJSON fetches url into path (verified by sha1 when given) and decodes it.
// If the network fails but a cached copy exists, the cached copy is used, so an
// installed game can still launch offline.
func (c *Client) CachedJSON(ctx context.Context, url, path, sha string, v any) error {
	if sha != "" && Valid(path, sha, 0) {
		return readJSON(path, v)
	}
	// Without a sha1 the cached copy may be stale, so always re-download.
	_, err := c.fetchOne(ctx, Item{URL: url, Path: path, SHA1: sha}, sha == "")
	if err != nil {
		if _, statErr := os.Stat(path); statErr == nil {
			return readJSON(path, v)
		}
		return err
	}
	return readJSON(path, v)
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// Valid reports whether path exists and matches size and sha1 (each checked only if set).
func Valid(path, sha string, size int64) bool {
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() {
		return false
	}
	if size > 0 && st.Size() != size {
		return false
	}
	if sha == "" {
		return true
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return false
	}
	return hex.EncodeToString(h.Sum(nil)) == sha
}
