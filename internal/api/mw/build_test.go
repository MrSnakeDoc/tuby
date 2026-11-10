package mw

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireCurrentBuildRefreshesStaleHTMXClients(t *testing.T) {
	tests := []struct {
		name        string
		htmx        bool
		buildID     string
		wantRefresh bool
	}{
		{name: "matching build", htmx: true, buildID: "v2-abc"},
		{name: "different build", htmx: true, buildID: "v1-old", wantRefresh: true},
		{name: "pre-versioning client", htmx: true, wantRefresh: true},
		{name: "stale fetch", buildID: "v1-old", wantRefresh: true},
		{name: "regular request without versioning"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			handler := RequireCurrentBuild("v2-abc")(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				called = true
				writer.WriteHeader(http.StatusNoContent)
			}))
			request := httptest.NewRequest(http.MethodPost, "/api/action", http.NoBody)
			if test.htmx {
				request.Header.Set("HX-Request", "true")
			}
			if test.buildID != "" {
				request.Header.Set(BuildHeader, test.buildID)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if test.wantRefresh {
				if called || response.Code != http.StatusConflict || response.Header().Get("HX-Refresh") != "true" {
					t.Fatalf("called = %t, status = %d, headers = %v", called, response.Code, response.Header())
				}
				return
			}
			if !called || response.Code != http.StatusNoContent || response.Header().Get("HX-Refresh") != "" {
				t.Fatalf("called = %t, status = %d, headers = %v", called, response.Code, response.Header())
			}
		})
	}
}
