package youtube

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const maximumVideoDuration = 30 * 24 * 60 * 60

var (
	metaTagPattern     = regexp.MustCompile(`(?is)<meta\b[^>]*>`)
	attributePattern   = regexp.MustCompile(`(?i)([a-z_:][-a-z0-9_:.]*)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'=<>]+))`)
	isoDurationPattern = regexp.MustCompile(`^P(?:(\d+(?:\.\d+)?)D)?(?:T(?:(\d+(?:\.\d+)?)H)?(?:(\d+(?:\.\d+)?)M)?(?:(\d+(?:\.\d+)?)S)?)?$`)
)

type durationCandidate struct {
	name    string
	seconds int64
}

type playerResponse struct {
	VideoDetails struct {
		LengthSeconds json.RawMessage `json:"lengthSeconds"`
	} `json:"videoDetails"`
	StreamingData struct {
		AdaptiveFormats []struct {
			ApproxDurationMS json.RawMessage `json:"approxDurationMs"`
		} `json:"adaptiveFormats"`
	} `json:"streamingData"`
}

func ParseVideoDuration(page []byte) (int64, error) {
	candidates := make([]durationCandidate, 0, 3)
	if seconds, found := metaDuration(page); found {
		candidates = append(candidates, durationCandidate{name: "meta", seconds: seconds})
	}
	response, found := initialPlayerResponse(page)
	if found {
		if seconds, valid := rawSeconds(response.VideoDetails.LengthSeconds); valid {
			candidates = append(candidates, durationCandidate{name: "player", seconds: seconds})
		}
		seconds, valid, err := adaptiveDuration(response)
		if err != nil {
			return 0, err
		}
		if valid {
			candidates = append(candidates, durationCandidate{name: "adaptive", seconds: seconds})
		}
	}
	if len(candidates) == 0 {
		return 0, fmt.Errorf("duration is unavailable")
	}
	if err := coherentDurations(candidates); err != nil {
		return 0, err
	}
	return candidates[0].seconds, nil
}

func metaDuration(page []byte) (int64, bool) {
	for _, tag := range metaTagPattern.FindAll(page, -1) {
		attributes := tagAttributes(tag)
		if !strings.EqualFold(attributes["itemprop"], "duration") {
			continue
		}
		if seconds, valid := parseISODuration(attributes["content"]); valid {
			return seconds, true
		}
	}
	return 0, false
}

func tagAttributes(tag []byte) map[string]string {
	attributes := make(map[string]string)
	for _, match := range attributePattern.FindAllSubmatch(tag, -1) {
		value := firstValue(match[2:])
		attributes[strings.ToLower(string(match[1]))] = string(value)
	}
	return attributes
}

func firstValue(values [][]byte) []byte {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return nil
}

func parseISODuration(value string) (int64, bool) {
	matches := isoDurationPattern.FindStringSubmatch(value)
	if matches == nil {
		return 0, false
	}
	multipliers := []float64{24 * 60 * 60, 60 * 60, 60, 1}
	seconds := float64(0)
	for index, part := range matches[1:] {
		if part == "" {
			continue
		}
		amount, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return 0, false
		}
		seconds += amount * multipliers[index]
	}
	return validDuration(int64(math.Round(seconds)))
}

func initialPlayerResponse(page []byte) (playerResponse, bool) {
	marker := []byte("ytInitialPlayerResponse")
	for offset := 0; offset < len(page); {
		index := bytes.Index(page[offset:], marker)
		if index < 0 {
			return playerResponse{}, false
		}
		start := offset + index + len(marker)
		object, end, found := jsonObjectAfter(page, start)
		if found {
			var response playerResponse
			if json.Unmarshal(object, &response) == nil && hasDurationData(response) {
				return response, true
			}
			offset = end
			continue
		}
		offset = start
	}
	return playerResponse{}, false
}

func hasDurationData(response playerResponse) bool {
	return len(response.VideoDetails.LengthSeconds) > 0 || len(response.StreamingData.AdaptiveFormats) > 0
}

func jsonObjectAfter(page []byte, start int) ([]byte, int, bool) {
	objectStart := bytes.IndexByte(page[start:], '{')
	if objectStart < 0 || objectStart > 512 {
		return nil, start, false
	}
	objectStart += start
	depth := 0
	inString := false
	escaped := false
	for index := objectStart; index < len(page); index++ {
		character := page[index]
		if inString {
			escaped, inString = stringState(character, escaped, inString)
			continue
		}
		if character == '"' {
			inString = true
			continue
		}
		switch character {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return page[objectStart : index+1], index + 1, true
			}
		}
	}
	return nil, len(page), false
}

func stringState(character byte, escaped, inString bool) (bool, bool) {
	if escaped {
		return false, inString
	}
	if character == '\\' {
		return true, inString
	}
	if character == '"' {
		return false, false
	}
	return false, inString
}

func rawSeconds(value json.RawMessage) (int64, bool) {
	if len(value) == 0 {
		return 0, false
	}
	var encoded string
	if value[0] == '"' {
		if json.Unmarshal(value, &encoded) != nil {
			return 0, false
		}
	} else {
		encoded = string(value)
	}
	seconds, err := strconv.ParseInt(encoded, 10, 64)
	if err != nil {
		return 0, false
	}
	return validDuration(seconds)
}

func adaptiveDuration(response playerResponse) (int64, bool, error) {
	values := make([]int64, 0, len(response.StreamingData.AdaptiveFormats))
	for _, format := range response.StreamingData.AdaptiveFormats {
		milliseconds, valid := rawMilliseconds(format.ApproxDurationMS)
		if valid {
			values = append(values, milliseconds)
		}
	}
	if len(values) == 0 {
		return 0, false, nil
	}
	sort.Slice(values, func(left, right int) bool { return values[left] < values[right] })
	median := values[len(values)/2]
	if !coherentAdaptiveValues(values, median) {
		return 0, false, fmt.Errorf("inconsistent adaptive format durations")
	}
	seconds := int64(math.Round(float64(median) / 1000))
	seconds, valid := validDuration(seconds)
	return seconds, valid, nil
}

func coherentAdaptiveValues(values []int64, median int64) bool {
	tolerance := median / 200
	if tolerance < 2000 {
		tolerance = 2000
	}
	return median-values[0] <= tolerance && values[len(values)-1]-median <= tolerance
}

func rawMilliseconds(value json.RawMessage) (int64, bool) {
	if len(value) == 0 {
		return 0, false
	}
	var encoded string
	if value[0] == '"' {
		if json.Unmarshal(value, &encoded) != nil {
			return 0, false
		}
	} else {
		encoded = string(value)
	}
	milliseconds, err := strconv.ParseInt(encoded, 10, 64)
	if err != nil || milliseconds <= 0 {
		return 0, false
	}
	return milliseconds, true
}

func validDuration(seconds int64) (int64, bool) {
	return seconds, seconds > 0 && seconds <= maximumVideoDuration
}

func coherentDurations(candidates []durationCandidate) error {
	preferred := candidates[0]
	for _, candidate := range candidates[1:] {
		difference := preferred.seconds - candidate.seconds
		if difference < 0 {
			difference = -difference
		}
		tolerance := preferred.seconds / 200
		if tolerance < 2 {
			tolerance = 2
		}
		if difference > tolerance {
			return fmt.Errorf("inconsistent duration sources: %s=%d, %s=%d",
				preferred.name, preferred.seconds, candidate.name, candidate.seconds)
		}
	}
	return nil
}
