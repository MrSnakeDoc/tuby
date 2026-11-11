package youtube

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestDurationClientFetchesCanonicalVideoPage(t *testing.T) {
	client := NewDurationClient(&http.Client{Transport: roundTripper(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "www.youtube.com" || request.URL.Path != "/watch" ||
			request.URL.Query().Get("v") != "dQw4w9WgXcQ" {
			t.Fatalf("URL = %s", request.URL)
		}
		if request.Header.Get("Accept-Language") == "" || request.Header.Get("User-Agent") == "" {
			t.Fatalf("headers = %#v", request.Header)
		}
		return response(http.StatusOK, `<meta itemprop="duration" content="PT3M33S">`, nil), nil
	})})
	seconds, err := client.FetchDuration(context.Background(), "dQw4w9WgXcQ")
	if err != nil {
		t.Fatal(err)
	}
	if seconds != 213 {
		t.Fatalf("duration = %d, want 213", seconds)
	}
}

func TestDurationClientRejectsInvalidIDWithoutRequest(t *testing.T) {
	called := false
	client := NewDurationClient(&http.Client{Transport: roundTripper(func(*http.Request) (*http.Response, error) {
		called = true
		return response(http.StatusOK, "", nil), nil
	})})
	_, err := client.FetchDuration(context.Background(), "not-valid")
	if err == nil || called {
		t.Fatalf("error = %v, called = %t", err, called)
	}
}

func TestDurationClientReportsHTTPFailure(t *testing.T) {
	client := NewDurationClient(&http.Client{Transport: roundTripper(func(*http.Request) (*http.Response, error) {
		return response(http.StatusNotFound, "", nil), nil
	})})
	_, err := client.FetchDuration(context.Background(), "dQw4w9WgXcQ")
	if err == nil || !strings.Contains(err.Error(), "unexpected status 404") {
		t.Fatalf("error = %v", err)
	}
}
