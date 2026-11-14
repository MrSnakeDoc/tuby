package youtube

import (
	"strings"
	"testing"
)

func TestParseVideoDurationPrefersCoherentMetadata(t *testing.T) {
	page := []byte(`<html><head><meta content="PT31M14S" itemprop="duration"></head><body><script>
var ytInitialPlayerResponse = {"videoDetails":{"lengthSeconds":"1874"},"streamingData":{"adaptiveFormats":[
{"approxDurationMs":"1873624"},{"approxDurationMs":"1874010"}]}};
</script></body></html>`)
	seconds, err := ParseVideoDuration(page)
	if err != nil {
		t.Fatal(err)
	}
	if seconds != 1874 {
		t.Fatalf("duration = %d, want 1874", seconds)
	}
}

func TestParseVideoDurationFallsBackToPlayerResponse(t *testing.T) {
	page := []byte(`<meta itemprop='duration' content='not-a-duration'>
<script>window["ytInitialPlayerResponse"] = {"videoDetails":{"lengthSeconds":125}};</script>`)
	seconds, err := ParseVideoDuration(page)
	if err != nil {
		t.Fatal(err)
	}
	if seconds != 125 {
		t.Fatalf("duration = %d, want 125", seconds)
	}
}

func TestParseVideoDurationFallsBackToAdaptiveFormats(t *testing.T) {
	page := []byte(`<script>var ytInitialPlayerResponse = {
"videoDetails":{"lengthSeconds":"invalid"},
"streamingData":{"adaptiveFormats":[{"approxDurationMs":"62490"}]}};</script>`)
	seconds, err := ParseVideoDuration(page)
	if err != nil {
		t.Fatal(err)
	}
	if seconds != 62 {
		t.Fatalf("duration = %d, want 62", seconds)
	}
}

func TestParseVideoDurationRejectsInconsistentSources(t *testing.T) {
	page := []byte(`<meta itemprop="duration" content="PT10M">
<script>var ytInitialPlayerResponse = {"videoDetails":{"lengthSeconds":"900"}};</script>`)
	_, err := ParseVideoDuration(page)
	if err == nil || !strings.Contains(err.Error(), "inconsistent duration sources") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseVideoDurationRejectsInconsistentAdaptiveFormats(t *testing.T) {
	page := []byte(`<script>var ytInitialPlayerResponse = {"streamingData":{"adaptiveFormats":[
{"approxDurationMs":"60000"},{"approxDurationMs":"90000"}]}};</script>`)
	_, err := ParseVideoDuration(page)
	if err == nil || !strings.Contains(err.Error(), "inconsistent adaptive format durations") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseVideoDurationSkipsEmptyPlayerResponse(t *testing.T) {
	page := []byte(`<script>var ytInitialPlayerResponse = {};</script>
<script>var ytInitialPlayerResponse = {"videoDetails":{"lengthSeconds":"75"}};</script>`)
	seconds, err := ParseVideoDuration(page)
	if err != nil || seconds != 75 {
		t.Fatalf("duration = %d, error = %v", seconds, err)
	}
}

func TestParseVideoDurationRejectsUnavailableDuration(t *testing.T) {
	for _, page := range [][]byte{
		[]byte(`<html><body>Unavailable video</body></html>`),
		[]byte(`<meta itemprop="duration" content="PT0S">`),
		[]byte(`<script>var ytInitialPlayerResponse = {"videoDetails":{"lengthSeconds":"0"}};</script>`),
	} {
		if _, err := ParseVideoDuration(page); err == nil {
			t.Fatalf("page %q unexpectedly produced a duration", page)
		}
	}
}
