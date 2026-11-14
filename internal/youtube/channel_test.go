package youtube

import "testing"

func TestNormalizeChannelReference(t *testing.T) {
	channelID := "UCHnyfMqiRRG1u-2MsSQLbXA"
	tests := []struct {
		name  string
		input string
		want  ChannelReference
	}{
		{name: "channel ID", input: channelID, want: ChannelReference{ID: channelID, SourceURL: canonicalChannelURL(channelID)}},
		{name: "channel URL", input: "https://m.youtube.com/channel/" + channelID + "?feature=share#about", want: ChannelReference{ID: channelID, SourceURL: canonicalChannelURL(channelID)}},
		{name: "handle URL", input: " https://youtube.com/@veritasium?feature=shared ", want: ChannelReference{SourceURL: "https://www.youtube.com/@veritasium"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizeChannelReference(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("reference = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestNormalizeChannelReferenceRejectsInvalidInput(t *testing.T) {
	inputs := []string{
		"", "UCtoo-short", "http://youtube.com/@example", "https://youtube.com/",
		"https://youtube.com/watch?v=video", "https://youtube.com/channel/not-an-id",
		"https://example.com/@channel", "https://evil.youtube.com/@channel",
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			if _, err := NormalizeChannelReference(input); err == nil {
				t.Fatalf("NormalizeChannelReference(%q) succeeded", input)
			}
		})
	}
}
