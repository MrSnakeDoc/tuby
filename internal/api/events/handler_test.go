package events

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domainevents "git.nexacloud.dev/MrSnakeDoc/tuby/internal/events"
)

func TestStreamStartsWithSyncEvent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodGet, "/api/events", http.NoBody).WithContext(ctx)
	response := &flushRecorder{header: make(http.Header), cancel: cancel}

	(&handler{updates: domainevents.New()}).stream(response, request)

	if response.header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("Content-Type = %q", response.header.Get("Content-Type"))
	}
	if body := response.body.String(); !strings.Contains(body, "event: update") || !strings.Contains(body, `"reason":"sync"`) {
		t.Fatalf("body = %q", body)
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
