package download

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRangeReader(t *testing.T) {
	data := make([]byte, 5*rangeBlock+123)
	for i := range data {
		data[i] = byte(i * 7)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "f", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	r := New("test").RangeReader(context.Background(), srv.URL, int64(len(data)))

	// Across a block boundary, then the same spot again (cached), then the tail.
	got := make([]byte, 200)
	if n, err := r.ReadAt(got, rangeBlock-100); n != 200 || err != nil || !bytes.Equal(got, data[rangeBlock-100:rangeBlock+100]) {
		t.Fatalf("read across blocks: n %d, err %v", n, err)
	}
	if _, err := r.ReadAt(got[:50], rangeBlock); err != nil || r.Requests != 1 {
		t.Errorf("cached read: err %v, requests %d (want 1: both blocks came in one request)", err, r.Requests)
	}
	tail := make([]byte, 500)
	n, err := r.ReadAt(tail, int64(len(data))-123)
	if n != 123 || !bytes.Equal(tail[:n], data[len(data)-123:]) || err == nil {
		t.Errorf("tail: n %d, err %v (want 123 bytes and io.EOF)", n, err)
	}

	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data) // ignores Range
	}))
	defer plain.Close()
	if _, err := New("test").RangeReader(context.Background(), plain.URL, int64(len(data))).ReadAt(got, 0); !errors.Is(err, ErrNoRanges) {
		t.Errorf("server without ranges: err %v, want ErrNoRanges", err)
	}
}
