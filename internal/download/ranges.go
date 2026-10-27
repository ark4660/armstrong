package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// ErrNoRanges means the server sent the whole file instead of the part asked
// for; the caller should download it normally.
var ErrNoRanges = errors.New("server doesn't serve byte ranges")

// rangeBlock is how much one request fetches at least: reading a zip touches
// many small spots near each other (headers, the central directory).
const rangeBlock = 64 << 10

// RangeReader reads parts of a remote file with HTTP range requests, so a
// zip's index and one entry can be read without downloading the whole file.
// Fetched blocks are kept for the reader's lifetime.
type RangeReader struct {
	c    *Client
	ctx  context.Context
	url  string
	size int64

	mu       sync.Mutex
	blocks   map[int64][]byte // by block index
	Requests int              // HTTP requests made (for tests and logs)
}

// RangeReader returns a reader for url, whose size must be known.
func (c *Client) RangeReader(ctx context.Context, url string, size int64) *RangeReader {
	return &RangeReader{c: c, ctx: ctx, url: url, size: size, blocks: map[int64][]byte{}}
}

func (r *RangeReader) Size() int64 { return r.size }

// ReadAt implements io.ReaderAt.
func (r *RangeReader) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 || off >= r.size {
		return 0, io.EOF
	}
	end := min(off+int64(len(p)), r.size)
	r.mu.Lock()
	defer r.mu.Unlock()
	first, last := off/rangeBlock, (end-1)/rangeBlock
	// Fetch missing blocks, a run of neighbours in one request.
	for b := first; b <= last; {
		if _, ok := r.blocks[b]; ok {
			b++
			continue
		}
		runEnd := b
		for runEnd+1 <= last {
			if _, ok := r.blocks[runEnd+1]; ok {
				break
			}
			runEnd++
		}
		if err := r.fetch(b, runEnd); err != nil {
			return 0, err
		}
		b = runEnd + 1
	}
	n := 0
	for pos := off; pos < end; {
		blk := r.blocks[pos/rangeBlock]
		n += copy(p[n:], blk[pos%rangeBlock:])
		pos = off + int64(n)
	}
	if end < off+int64(len(p)) {
		return n, io.EOF
	}
	return n, nil
}

// fetch downloads blocks first..last (inclusive) in one request.
func (r *RangeReader) fetch(first, last int64) error {
	from, to := first*rangeBlock, min((last+1)*rangeBlock, r.size)-1
	var err error
	for attempt := range r.c.Retries + 1 {
		if attempt > 0 && r.ctx.Err() != nil {
			return r.ctx.Err()
		}
		var data []byte
		if data, err = r.get(from, to); err == nil {
			for b := first; b <= last; b++ {
				lo := (b - first) * rangeBlock
				r.blocks[b] = data[lo:min(lo+rangeBlock, int64(len(data)))]
			}
			return nil
		}
		if errors.Is(err, ErrNoRanges) || errors.As(err, new(permanentError)) {
			return err
		}
	}
	return err
}

func (r *RangeReader) get(from, to int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(r.ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return nil, permanentError{err}
	}
	req.Header.Set("User-Agent", r.c.UserAgent)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", from, to))
	r.Requests++
	resp, err := r.c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusOK:
		return nil, ErrNoRanges // don't read the whole body
	case resp.StatusCode != http.StatusPartialContent:
		err := fmt.Errorf("HTTP %s", resp.Status)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			return nil, permanentError{err}
		}
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, to-from+2))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) != to-from+1 {
		return nil, fmt.Errorf("range %d-%d: got %d bytes", from, to, len(data))
	}
	return data, nil
}
