package videos

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api/deps"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/store"
)

func TestFetchDurationRoute(t *testing.T) {
	service := &durationServiceStub{}
	router := durationRouter(service)
	request := httptest.NewRequest(http.MethodPost, "/api/videos/dQw4w9WgXcQ/duration", http.NoBody)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || service.videoID != "dQw4w9WgXcQ" {
		t.Fatalf("status = %d, video = %q", response.Code, service.videoID)
	}
}

func TestFetchDurationRouteReportsFailures(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		status int
	}{
		{name: "missing", err: store.ErrVideoNotFound, status: http.StatusNotFound},
		{name: "upstream", err: errors.New("YouTube unavailable"), status: http.StatusBadGateway},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			durationRouter(&durationServiceStub{err: test.err}).ServeHTTP(response,
				httptest.NewRequest(http.MethodPost, "/api/videos/dQw4w9WgXcQ/duration", http.NoBody))
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}

func durationRouter(service deps.VideoDurations) http.Handler {
	router := chi.NewRouter()
	Register(router, deps.Deps{VideoDurations: service})
	return router
}

type durationServiceStub struct {
	videoID string
	err     error
}

func (s *durationServiceStub) Fetch(_ context.Context, videoID string) error {
	s.videoID = videoID
	return s.err
}
