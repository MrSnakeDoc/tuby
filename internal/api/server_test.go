package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api/deps"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/config"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/events"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/logger"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/web"
)

func TestRoutesExposeTubyInfrastructure(t *testing.T) {
	server := newServer()
	tests := []struct {
		method string
		path   string
		status int
	}{
		{method: http.MethodGet, path: "/", status: http.StatusOK},
		{method: http.MethodHead, path: "/", status: http.StatusOK},
		{method: http.MethodGet, path: "/healthz", status: http.StatusOK},
		{method: http.MethodPost, path: "/generate", status: http.StatusNotFound},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, http.NoBody)
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d: %s", response.Code, test.status, response.Body.String())
			}
		})
	}
}

func TestHomeRendersTubyPlaceholder(t *testing.T) {
	server := newServer()
	request := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
		t.Fatalf("Content-Type = %q, want HTML", contentType)
	}
	body := response.Body.String()
	if !strings.Contains(body, "Tuby") || strings.Contains(body, "MailClown") {
		t.Fatalf("homepage body = %q", body)
	}
	if !strings.Contains(body, `<meta name="tuby-build" content="test-build">`) ||
		!strings.Contains(body, `/static/js/live-updates.js?v=test-build`) ||
		!strings.Contains(body, `/static/js/channel-thumbnails.js?v=test-build`) ||
		!strings.Contains(body, `/static/js/video-duration.js?v=test-build`) ||
		!strings.Contains(body, `/static/js/keyboard-shortcuts.js?v=test-build`) {
		t.Fatalf("homepage does not expose the current build")
	}
	if strings.Contains(body, "setInterval(pick") {
		t.Fatal("homepage still contains per-thumbnail timers")
	}
	if !strings.Contains(body, `/static/img/home.svg?v=test-build`) ||
		!strings.Contains(body, `/static/img/clock-alert.svg?v=test-build`) ||
		!strings.Contains(body, `/static/img/bookmark.svg?v=test-build`) {
		t.Fatalf("homepage does not render the dock icons")
	}
	if response.Header().Get("Cache-Control") != "no-cache, no-store, must-revalidate" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
}

func TestHomeKeepsZeroCountersHiddenAndTargetable(t *testing.T) {
	server := newServer()
	request := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	body := response.Body.String()
	for _, id := range []string{"missed-count", "watch-later-count"} {
		if !strings.Contains(body, `id="`+id+`"`) || !strings.Contains(body, `id="`+id+`" class="dock-count" hidden`) {
			t.Fatalf("counter %q is not present and hidden", id)
		}
	}
}

func TestCategoryManagementExposesCollapseAllControl(t *testing.T) {
	server := newServer()
	request := httptest.NewRequest(http.MethodGet, "/static/js/category-management.js", http.NoBody)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	body := response.Body.String()
	if !strings.Contains(body, "data-category-collapse") ||
		!strings.Contains(body, "Collapse all categories") || !strings.Contains(body, "Expand all categories") {
		t.Fatal("category management asset does not expose the collapse-all toggle")
	}
}

func TestKeyboardShortcutAssetExposesConfiguredSequences(t *testing.T) {
	server := newServer()
	request := httptest.NewRequest(http.MethodGet, "/static/js/keyboard-shortcuts.js", http.NoBody)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	body := response.Body.String()
	for _, sequence := range []string{"'gh'", "'gm'", "'gw'", "'ac'", "'ag'", "'ca'", "'ea'"} {
		if !strings.Contains(body, sequence) {
			t.Fatalf("keyboard shortcut asset does not contain %s", sequence)
		}
	}
	if !strings.Contains(body, "event.key === '?'") {
		t.Fatal("keyboard shortcut asset does not expose the help shortcut")
	}
}

func TestHomeRendersKeyboardShortcutDialog(t *testing.T) {
	server := newServer()
	request := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	body := response.Body.String()
	for _, expected := range []string{
		`id="shortcuts-dialog"`, `aria-labelledby="shortcuts-title"`,
		`Keyboard shortcuts`, `Search channels`, `Show shortcuts`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("shortcut dialog does not contain %q", expected)
		}
	}
}

func TestHTMXContentIncludesOutOfBandCounters(t *testing.T) {
	server := newServer()
	request := httptest.NewRequest(http.MethodGet, "/missed", http.NoBody)
	request.Header.Set("HX-Request", "true")
	request.Header.Set("X-Tuby-Build", "test-build")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if count := strings.Count(response.Body.String(), `hx-swap-oob="outerHTML"`); count != 2 {
		t.Fatalf("out-of-band counter count = %d, want 2", count)
	}
}

func TestStaleHTMXClientIsForcedToReload(t *testing.T) {
	server := newServer()
	request := httptest.NewRequest(http.MethodPost, "/api/channels/fetch", http.NoBody)
	request.Header.Set("HX-Request", "true")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusConflict || response.Header().Get("HX-Refresh") != "true" {
		t.Fatalf("status = %d, headers = %v", response.Code, response.Header())
	}
}

func TestHealthzReturnsNoStoreJSON(t *testing.T) {
	server := newServer()
	request := httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", response.Header().Get("Cache-Control"))
	}
	var body struct {
		Service string `json:"service"`
		Status  string `json:"status"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ok" || body.Service != "tuby" {
		t.Fatalf("health response = %+v", body)
	}
}

func TestAllowedCIDRSMiddleware(t *testing.T) {
	server := newServerWithAllowedCIDRS([]string{"10.0.0.0/8"})
	request := httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody)
	request.RemoteAddr = "127.0.0.1:8080"
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestAllowedCIDRSMiddlewarePermitsMatchingAddress(t *testing.T) {
	server := newServerWithAllowedCIDRS([]string{"127.0.0.1"})
	request := httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody)
	request.RemoteAddr = "127.0.0.1:8080"
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestStopClosesLiveUpdateHub(t *testing.T) {
	updates := events.New("test-build")
	subscriber, unsubscribe := updates.Subscribe()
	defer unsubscribe()
	dependencies := testDependencies()
	dependencies.Updates = updates
	server := api.New(&config.Config{ListenAddr: "127.0.0.1:0"}, testLogger(), dependencies)
	if err := server.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, open := <-subscriber; open {
		t.Fatal("live update subscriber remained connected after server stop")
	}
}

func newServer() *api.Server {
	return newServerWithAllowedCIDRS(nil)
}

func newServerWithAllowedCIDRS(allowedCIDRS []string) *api.Server {
	return api.New(&config.Config{ListenAddr: ":8080", AllowedCIDRS: allowedCIDRS}, testLogger(), testDependencies())
}

func testLogger() logger.Logger {
	return logger.New("error", false)
}

func testDependencies() deps.Deps {
	templates, err := web.New()
	if err != nil {
		panic(err)
	}
	return deps.Deps{
		Templates: templates,
		StartTime: time.Now(),
		Version:   "test",
		BuildID:   "test-build",
		Commit:    "test",
		BuildDate: "test",
		GoVersion: "test",
	}
}
