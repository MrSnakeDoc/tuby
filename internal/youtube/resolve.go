package youtube

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const maxChannelPageBytes = 2 << 20

var feedURLPattern = regexp.MustCompile(`https://www\.youtube\.com/feeds/videos\.xml\?channel_id=(UC[A-Za-z0-9_-]{22})`)

type Resolver struct {
	client *http.Client
}

func NewResolver(client *http.Client) *Resolver {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Resolver{client: client}
}

func (r *Resolver) ResolveChannelID(ctx context.Context, rawURL string) (string, error) {
	reference, err := NormalizeChannelReference(rawURL)
	if err != nil {
		return "", err
	}
	if reference.ID != "" {
		return reference.ID, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, reference.SourceURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("create YouTube channel request: %w", err)
	}
	request.Header.Set("Accept", "text/html")
	request.Header.Set("User-Agent", "Tuby/1.0")
	response, err := r.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("fetch YouTube channel page: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch YouTube channel page: unexpected status %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxChannelPageBytes))
	if err != nil {
		return "", fmt.Errorf("read YouTube channel page: %w", err)
	}
	return channelIDFromPage(string(body))
}

func isYouTubeHost(host string) bool {
	host = strings.ToLower(host)
	return host == "youtube.com" || host == "www.youtube.com" || host == "m.youtube.com"
}

func channelIDFromPage(page string) (string, error) {
	matches := feedURLPattern.FindAllStringSubmatch(page, -1)
	if len(matches) == 0 {
		return "", fmt.Errorf("YouTube channel page does not advertise an RSS feed")
	}
	channelID := matches[0][1]
	for _, match := range matches[1:] {
		if match[1] != channelID {
			return "", fmt.Errorf("YouTube channel page advertises multiple RSS feeds")
		}
	}
	return channelID, nil
}
