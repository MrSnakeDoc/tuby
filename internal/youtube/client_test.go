package youtube

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestFeedClientUsesConditionalHeaders(t *testing.T) {
	client := NewFeedClient(&http.Client{Transport: roundTripper(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://www.youtube.com/feeds/videos.xml?channel_id=UCHnyfMqiRRG1u-2MsSQLbXA" {
			t.Fatalf("URL = %s", request.URL)
		}
		if request.Header.Get("If-None-Match") != "old-etag" || request.Header.Get("If-Modified-Since") != "yesterday" {
			t.Fatalf("conditional headers = %#v", request.Header)
		}
		return response(http.StatusNotModified, "", map[string]string{"ETag": "new-etag"}), nil
	})})
	result, err := client.Fetch(context.Background(), "UCHnyfMqiRRG1u-2MsSQLbXA", FetchOptions{ETag: "old-etag", LastModified: "yesterday"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.NotModified || result.ETag != "new-etag" {
		t.Fatalf("result = %#v", result)
	}
}

func TestFeedClientClassifiesTemporaryResponses(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusBadGateway} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			client := NewFeedClient(&http.Client{Transport: roundTripper(func(*http.Request) (*http.Response, error) {
				return response(status, "", nil), nil
			})})
			_, err := client.Fetch(context.Background(), "UCHnyfMqiRRG1u-2MsSQLbXA", FetchOptions{})
			if !IsTemporary(err) {
				t.Fatalf("error %v is not temporary", err)
			}
		})
	}
}

func response(status int, body string, headers map[string]string) *http.Response {
	header := make(http.Header)
	for key, value := range headers {
		header.Set(key, value)
	}
	return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body))}
}

type roundTripper func(*http.Request) (*http.Response, error)

func (r roundTripper) RoundTrip(request *http.Request) (*http.Response, error) { return r(request) }
