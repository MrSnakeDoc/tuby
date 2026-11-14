package youtube

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var channelIDPattern = regexp.MustCompile(`^UC[A-Za-z0-9_-]{22}$`)

type ChannelReference struct {
	ID        string
	SourceURL string
}

func NormalizeChannelReference(raw string) (ChannelReference, error) {
	value := strings.TrimSpace(raw)
	if validChannelID(value) {
		return ChannelReference{ID: value, SourceURL: canonicalChannelURL(value)}, nil
	}
	return normalizeChannelURL(value)
}

func normalizeChannelURL(rawURL string) (ChannelReference, error) {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || parsed.Scheme != "https" || !isYouTubeHost(parsed.Hostname()) {
		return ChannelReference{}, fmt.Errorf("channel URL must be an HTTPS YouTube URL")
	}
	if parsed.User != nil || parsed.Port() != "" || !validChannelPath(parsed.Path) {
		return ChannelReference{}, fmt.Errorf("channel URL must identify a YouTube channel")
	}
	parsed.Scheme = "https"
	parsed.Host = "www.youtube.com"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.RawFragment = ""
	parsed.ForceQuery = false
	reference := ChannelReference{SourceURL: parsed.String()}
	if id := channelIDFromPath(parsed.Path); id != "" {
		reference.ID = id
	}
	return reference, nil
}

func channelIDFromPath(path string) string {
	const channelPrefix = "/channel/"
	if !strings.HasPrefix(path, channelPrefix) {
		return ""
	}
	id := strings.TrimPrefix(path, channelPrefix)
	if strings.Contains(id, "/") || !validChannelID(id) {
		return ""
	}
	return id
}

func validChannelPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 1 {
		return strings.HasPrefix(parts[0], "@") && len(parts[0]) > 1
	}
	if len(parts) != 2 || parts[1] == "" {
		return false
	}
	if parts[0] == "channel" {
		return validChannelID(parts[1])
	}
	return parts[0] == "c" || parts[0] == "user"
}

func canonicalChannelURL(channelID string) string {
	return "https://www.youtube.com/channel/" + channelID
}

func validChannelID(value string) bool {
	return channelIDPattern.MatchString(value)
}
