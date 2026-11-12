package navire

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPublishSendsNavireEventContractWithoutTechnicalIdentifiers(t *testing.T) {
	requests := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/events" || request.Method != http.MethodPost {
			t.Errorf("request = %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", request.Header.Get("Content-Type"))
		}
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		requests <- payload
		writer.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(server.Close)
	client := NewClient(server.URL, time.Second)
	err := client.Publish(context.Background(), Event{
		Variant: "new_video", Priority: "normal", ChannelName: "Veritasium", VideoTitle: "Unicode · video",
	})
	if err != nil {
		t.Fatal(err)
	}
	payload := <-requests
	if payload["key"] != "tuby_video" || payload["variant"] != "new_video" || payload["priority"] != "normal" {
		t.Fatalf("payload = %#v", payload)
	}
	if _, exists := payload["idempotency_key"]; exists {
		t.Fatalf("payload contains idempotency key: %#v", payload)
	}
	data, ok := payload["data"].(map[string]any)
	if !ok || data["channel_name"] != "Veritasium" || data["video_title"] != "Unicode · video" {
		t.Fatalf("data = %#v", payload["data"])
	}
	if _, exists := data["message"]; exists {
		t.Fatalf("message should be absent: %#v", data)
	}
	if _, exists := data["url"]; exists {
		t.Fatalf("URL should be absent: %#v", data)
	}
}

func TestPublishIncludesConfiguredMessage(t *testing.T) {
	var message string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload eventRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		message = payload.Data.Message
		writer.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(server.Close)
	client := NewClient(server.URL, time.Second)
	if err := client.Publish(context.Background(), Event{
		Variant: "new_video_with_message", Priority: "critical", ChannelName: "Channel",
		VideoTitle: "Video", Message: "Preserve punctuation! 🤘",
	}); err != nil {
		t.Fatal(err)
	}
	if message != "Preserve punctuation! 🤘" {
		t.Fatalf("message = %q", message)
	}
}

func TestPublishReturnsHTTPErrorForAnyNonSuccessfulStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = writer.Write([]byte(`{"code":"template_data_invalid"}`))
	}))
	t.Cleanup(server.Close)
	err := NewClient(server.URL, time.Second).Publish(context.Background(), Event{
		Variant: "new_video", Priority: "low", ChannelName: "Channel", VideoTitle: "Video",
	})
	var httpError *HTTPError
	if !errors.As(err, &httpError) || httpError.StatusCode != http.StatusUnprocessableEntity || httpError.Body == "" {
		t.Fatalf("error = %#v", err)
	}
}
