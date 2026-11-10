package channels

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api/deps"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/store"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/subscription"
)

func TestDeleteRoute(t *testing.T) {
	service := &stubSubscriptions{}
	router := newRouter(service)
	request := httptest.NewRequest(http.MethodDelete, "/api/channels/UC123", http.NoBody)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || service.deleted != "UC123" {
		t.Fatalf("status %d, deleted %q", response.Code, service.deleted)
	}
}

func TestChannelRouteReturnsNotFound(t *testing.T) {
	service := &stubSubscriptions{deleteErr: store.ErrChannelNotFound}
	router := newRouter(service)
	request := httptest.NewRequest(http.MethodDelete, "/api/channels/missing", http.NoBody)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestFetchChannelRoute(t *testing.T) {
	service := &stubSubscriptions{}
	router := newRouter(service)
	request := httptest.NewRequest(http.MethodPost, "/api/channels/UC123/fetch", http.NoBody)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.refreshed != "UC123" {
		t.Fatalf("status %d, refreshed %q", response.Code, service.refreshed)
	}
}

func TestFetchAllRouteReportsPartialFailures(t *testing.T) {
	service := &stubSubscriptions{report: subscription.RefreshReport{
		Refreshed: 1,
		Failures:  []subscription.RefreshFailure{{ChannelID: "UC456", Err: errors.New("upstream unavailable")}},
	}}
	router := newRouter(service)
	request := httptest.NewRequest(http.MethodPost, "/api/channels/fetch", http.NoBody)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMultiStatus)
	}
	want := `{"refreshed":1,"failed":[{"channel_id":"UC456","error":"upstream unavailable"}]}` + "\n"
	if response.Body.String() != want {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestUpdateSettingsBindsSelectedVideoTypes(t *testing.T) {
	service := &stubSubscriptions{}
	router := newRouter(service)
	request := httptest.NewRequest(
		http.MethodPut,
		"/api/channels/UC123/settings",
		strings.NewReader("include_shorts=on"),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || service.settings.IncludeLong || !service.settings.IncludeShorts {
		t.Fatalf("status %d, settings %#v", response.Code, service.settings)
	}
}

func newRouter(service deps.Subscriptions) http.Handler {
	router := chi.NewRouter()
	Register(router, deps.Deps{Subscriptions: service})
	return router
}

type stubSubscriptions struct {
	deleted    string
	refreshed  string
	deleteErr  error
	refreshErr error
	report     subscription.RefreshReport
	reportErr  error
	settings   subscription.ContentPreferences
}

func (s *stubSubscriptions) Delete(_ context.Context, channelID string) error {
	s.deleted = channelID
	return s.deleteErr
}

func (s *stubSubscriptions) Add(context.Context, string, subscription.ContentPreferences) (store.Channel, error) {
	return store.Channel{}, nil
}

func (s *stubSubscriptions) UpdateSettings(_ context.Context, _ string, settings subscription.ContentPreferences) error {
	s.settings = settings
	return nil
}

func (s *stubSubscriptions) Refresh(_ context.Context, channelID string) error {
	s.refreshed = channelID
	return s.refreshErr
}

func (s *stubSubscriptions) RefreshAll(context.Context) (subscription.RefreshReport, error) {
	return s.report, s.reportErr
}
