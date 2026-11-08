//go:build integration

package youtube

import (
	"context"
	"testing"
)

func TestResolveKnownChannels(t *testing.T) {
	resolver := NewResolver(nil)
	tests := []struct {
		name string
		url  string
		id   string
	}{
		{name: "Veritasium", url: "https://www.youtube.com/@veritasium", id: "UCHnyfMqiRRG1u-2MsSQLbXA"},
		{name: "Linus Tech Tips", url: "https://www.youtube.com/@LinusTechTips", id: "UCXuqSBlHAE6Xw-yeJA0Tunw"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			channelID, err := resolver.ResolveChannelID(context.Background(), test.url)
			if err != nil {
				t.Fatal(err)
			}
			if channelID != test.id {
				t.Fatalf("channel ID = %q, want %q", channelID, test.id)
			}
		})
	}
}
