package channels

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/MrSnakeDoc/tuby/internal/api/deps"
	"github.com/MrSnakeDoc/tuby/internal/events"
	"github.com/MrSnakeDoc/tuby/internal/store"
	"github.com/MrSnakeDoc/tuby/internal/subscription"
	"github.com/MrSnakeDoc/tuby/internal/youtube"
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
		strings.NewReader("include_shorts=on&category_id=42"),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || service.settings.IncludeLong || !service.settings.IncludeShorts ||
		service.settings.CategoryID != 42 {
		t.Fatalf("status %d, settings %#v", response.Code, service.settings)
	}
}

func TestAddBindsSelectedCategory(t *testing.T) {
	service := &stubSubscriptions{}
	router := newRouter(service)
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/channels",
		strings.NewReader("url=UC123&include_long=on&category_id=42"),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || service.added.CategoryID != 42 || !service.added.IncludeLong {
		t.Fatalf("status %d, preferences %#v", response.Code, service.added)
	}
}

func TestAddReturnsUsefulErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{
			name: "invalid channel reference", err: subscription.ErrInvalidChannelReference,
			status: http.StatusBadRequest, message: "Enter a valid YouTube channel URL or UC… channel ID.",
		},
		{
			name: "missing category", err: store.ErrCategoryNotFound,
			status: http.StatusBadRequest, message: "That category no longer exists. Refresh the page and try again.",
		},
		{
			name: "channel resolution", err: subscription.ErrChannelResolution,
			status:  http.StatusBadGateway,
			message: "YouTube could not resolve this channel. Check the URL or try again later.",
		},
		{
			name: "RSS not found", err: &youtube.FetchError{StatusCode: http.StatusNotFound},
			status:  http.StatusBadGateway,
			message: "YouTube returned 404 for this channel's RSS feed. This is often temporary; try again later.",
		},
		{
			name: "rate limited", err: &youtube.FetchError{StatusCode: http.StatusTooManyRequests, Temporary: true},
			status:  http.StatusServiceUnavailable,
			message: "YouTube is rate-limiting requests. Try again later.",
		},
		{
			name: "temporary upstream failure", err: &youtube.FetchError{StatusCode: http.StatusBadGateway, Temporary: true},
			status: http.StatusServiceUnavailable, message: "YouTube is temporarily unavailable. Try again later.",
		},
		{
			name: "invalid feed", err: subscription.ErrChannelFeedUnavailable,
			status: http.StatusBadGateway, message: "YouTube returned an unreadable RSS feed. Try again later.",
		},
		{
			name: "internal failure", err: errors.New("database unavailable"),
			status: http.StatusInternalServerError, message: "Tuby could not add this channel. Try again later.",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &stubSubscriptions{addErr: test.err}
			request := httptest.NewRequest(
				http.MethodPost,
				"/api/channels",
				strings.NewReader("url=https%3A%2F%2Fyoutube.com%2F%40example&include_long=on"),
			)
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			response := httptest.NewRecorder()
			newRouter(service).ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
			want := `{"error":` + strconv.Quote(test.message) + "}\n"
			if response.Body.String() != want {
				t.Fatalf("body = %q, want %q", response.Body.String(), want)
			}
		})
	}
}

func TestUpdateSettingsRejectsInvalidCategoryID(t *testing.T) {
	service := &stubSubscriptions{}
	router := newRouter(service)
	request := httptest.NewRequest(
		http.MethodPut,
		"/api/channels/UC123/settings",
		strings.NewReader("include_long=on&category_id=invalid"),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestUpdateSettingsReturnsUsefulErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{
			name: "video types", err: subscription.ErrVideoTypesRequired,
			status: http.StatusBadRequest, message: "Select Long, Shorts, or both.",
		},
		{
			name: "category", err: store.ErrCategoryNotFound,
			status:  http.StatusBadRequest,
			message: "That category no longer exists. Refresh the page and try again.",
		},
		{
			name: "channel", err: store.ErrChannelNotFound,
			status: http.StatusNotFound, message: "This channel no longer exists.",
		},
		{
			name: "internal", err: errors.New("database unavailable"),
			status: http.StatusInternalServerError, message: "Tuby could not save these settings. Try again.",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &stubSubscriptions{settingsErr: test.err}
			request := httptest.NewRequest(
				http.MethodPut,
				"/api/channels/UC123/settings",
				strings.NewReader("include_long=on&category_id=1"),
			)
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			response := httptest.NewRecorder()
			newRouter(service).ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
			want := `{"error":` + strconv.Quote(test.message) + "}\n"
			if response.Body.String() != want {
				t.Fatalf("body = %q, want %q", response.Body.String(), want)
			}
		})
	}
}

func TestGetSettingsReturnsCurrentChannelPreferences(t *testing.T) {
	database, err := store.Open(filepath.Join(t.TempDir(), "tuby.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.SaveChannel(context.Background(), store.Channel{
		YouTubeID: "UC123", SourceURL: "https://youtube.com/channel/UC123", Title: "Example",
		IncludeLong: false, IncludeShorts: true,
	}); err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	Register(router, deps.Deps{Subscriptions: &stubSubscriptions{}, Store: database})
	request := httptest.NewRequest(http.MethodGet, "/api/channels/UC123/settings", http.NoBody)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	want := `{"channel_id":"UC123","title":"Example","category_id":1,"include_long":false,"include_shorts":true}` + "\n"
	if response.Body.String() != want {
		t.Fatalf("body = %s, want %s", response.Body.String(), want)
	}
}

func TestUpdateNotificationPersistsLevelAndPublishesUpdate(t *testing.T) {
	database := notificationTestStore(t)
	hub := events.New("test-build")
	updates, unsubscribe := hub.Subscribe()
	t.Cleanup(unsubscribe)
	router := chi.NewRouter()
	Register(router, deps.Deps{
		Subscriptions: &stubSubscriptions{notificationStore: database}, Store: database,
		Updates: hub, NotificationsEnabled: true,
	})
	request := httptest.NewRequest(
		http.MethodPut,
		"/api/channels/UC123/notification",
		strings.NewReader(`{"level":"medium"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	want := `{"channel_id":"UC123","level":"medium"}` + "\n"
	if response.Body.String() != want {
		t.Fatalf("body = %q, want %q", response.Body.String(), want)
	}
	channel, err := database.Channel(context.Background(), "UC123")
	if err != nil {
		t.Fatal(err)
	}
	if channel.NotificationLevel != store.NotificationMedium || channel.NotificationEnabledAt == nil {
		t.Fatalf("notification state = %q, %v", channel.NotificationLevel, channel.NotificationEnabledAt)
	}
	select {
	case update := <-updates:
		if update.Reason != "channel.notification.updated" || len(update.ChannelIDs) != 1 ||
			update.ChannelIDs[0] != "UC123" {
			t.Fatalf("update = %#v", update)
		}
	default:
		t.Fatal("notification update was not published")
	}
}

func TestUpdateNotificationRejectsInvalidLevel(t *testing.T) {
	database := notificationTestStore(t)
	router := chi.NewRouter()
	Register(router, deps.Deps{
		Subscriptions: &stubSubscriptions{}, Store: database, NotificationsEnabled: true,
	})
	request := httptest.NewRequest(
		http.MethodPut,
		"/api/channels/UC123/notification",
		strings.NewReader(`{"level":"loud"}`),
	)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestNotificationRouteIsAbsentWhenNavireIsDisabled(t *testing.T) {
	router := newRouter(&stubSubscriptions{})
	request := httptest.NewRequest(
		http.MethodPut,
		"/api/channels/UC123/notification",
		strings.NewReader(`{"level":"low"}`),
	)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func notificationTestStore(t *testing.T) *store.Store {
	t.Helper()
	database, err := store.Open(filepath.Join(t.TempDir(), "tuby.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.SaveChannel(context.Background(), store.Channel{
		YouTubeID: "UC123", SourceURL: "https://youtube.com/channel/UC123", Title: "Example",
	}); err != nil {
		t.Fatal(err)
	}
	return database
}

func newRouter(service deps.Subscriptions) http.Handler {
	router := chi.NewRouter()
	Register(router, deps.Deps{Subscriptions: service})
	return router
}

type stubSubscriptions struct {
	deleted           string
	refreshed         string
	deleteErr         error
	refreshErr        error
	report            subscription.RefreshReport
	reportErr         error
	settings          subscription.ContentPreferences
	settingsErr       error
	added             subscription.ContentPreferences
	addErr            error
	notificationStore *store.Store
}

func (s *stubSubscriptions) Delete(_ context.Context, channelID string) error {
	s.deleted = channelID
	return s.deleteErr
}

func (s *stubSubscriptions) Add(
	_ context.Context,
	_ string,
	preferences subscription.ContentPreferences,
) (store.Channel, error) {
	s.added = preferences
	return store.Channel{}, s.addErr
}

func (s *stubSubscriptions) UpdateSettings(_ context.Context, _ string, settings subscription.ContentPreferences) error {
	s.settings = settings
	return s.settingsErr
}

func (s *stubSubscriptions) UpdateNotificationLevel(
	ctx context.Context,
	channelID string,
	level store.NotificationLevel,
) error {
	if s.notificationStore == nil {
		return nil
	}
	return s.notificationStore.UpdateChannelNotificationLevel(ctx, channelID, level, time.Now())
}

func (s *stubSubscriptions) Refresh(_ context.Context, channelID string) error {
	s.refreshed = channelID
	return s.refreshErr
}

func (s *stubSubscriptions) RefreshAll(context.Context) (subscription.RefreshReport, error) {
	return s.report, s.reportErr
}
