package events

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	domainevents "git.nexacloud.dev/MrSnakeDoc/tuby/internal/events"
)

func TestStreamStartsWithSyncEvent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodGet, "/api/events", http.NoBody).WithContext(ctx)
	response := &flushRecorder{header: make(http.Header), cancel: cancel}

	(&handler{updates: domainevents.New("v2-test")}).stream(response, request)

	if response.header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("Content-Type = %q", response.header.Get("Content-Type"))
	}
	if body := response.body.String(); !strings.Contains(body, "event: update") ||
		!strings.Contains(body, `"build_id":"v2-test"`) || !strings.Contains(body, `"reason":"sync"`) {
		t.Fatalf("body = %q", body)
	}
}

func TestStreamStopsWhenUpdateHubCloses(t *testing.T) {
	hub := domainevents.New("v2-test")
	response := newShutdownRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/events", http.NoBody)
	done := make(chan struct{})
	go func() {
		(&handler{updates: hub}).stream(response, request)
		close(done)
	}()
	select {
	case <-response.flushed:
	case <-time.After(time.Second):
		t.Fatal("SSE stream did not start")
	}
	hub.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SSE stream did not stop when hub closed")
	}
}

type flushRecorder struct {
	header http.Header
	body   bytes.Buffer
	cancel context.CancelFunc
}

func (r *flushRecorder) Header() http.Header             { return r.header }
func (r *flushRecorder) WriteHeader(int)                 {}
func (r *flushRecorder) Write(value []byte) (int, error) { return r.body.Write(value) }
func (r *flushRecorder) Flush()                          { r.cancel() }

type shutdownRecorder struct {
	header  http.Header
	body    bytes.Buffer
	flushed chan struct{}
	once    sync.Once
}

func newShutdownRecorder() *shutdownRecorder {
	return &shutdownRecorder{header: make(http.Header), flushed: make(chan struct{})}
}

func (r *shutdownRecorder) Header() http.Header             { return r.header }
func (r *shutdownRecorder) WriteHeader(int)                 {}
func (r *shutdownRecorder) Write(value []byte) (int, error) { return r.body.Write(value) }
func (r *shutdownRecorder) Flush()                          { r.once.Do(func() { close(r.flushed) }) }
