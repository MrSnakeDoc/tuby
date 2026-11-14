package web

import "testing"

func TestCompactCountCapsThreeDigitValues(t *testing.T) {
	tests := []struct {
		value int
		want  string
	}{
		{value: 0, want: "0"},
		{value: 9, want: "9"},
		{value: 99, want: "99"},
		{value: 100, want: "99+"},
		{value: 999, want: "99+"},
	}
	for _, test := range tests {
		if got := compactCount(test.value); got != test.want {
			t.Fatalf("compactCount(%d) = %q, want %q", test.value, got, test.want)
		}
	}
}

func TestVideoDurationFormatsMinutesAndHours(t *testing.T) {
	minutes := int64(754)
	hours := int64(4354)
	if got := videoDuration(&minutes); got != "12:34" {
		t.Fatalf("minutes = %q, want 12:34", got)
	}
	if got := videoDuration(&hours); got != "1:12:34" {
		t.Fatalf("hours = %q, want 1:12:34", got)
	}
	if got := videoDuration(nil); got != "" {
		t.Fatalf("unknown duration = %q", got)
	}
}

func TestNotificationPresentationUsesExpectedIconsAndLabels(t *testing.T) {
	tests := []struct {
		level string
		icon  string
		label string
	}{
		{level: "off", icon: "cloud-hail", label: "Off"},
		{level: "low", icon: "bell-electric", label: "Wake Me Up Before You Go-Go!"},
		{level: "medium", icon: "disc-3", label: "I Like to Move It!"},
		{level: "critical", icon: "dog", label: "Who Let the Dogs Out?!"},
	}
	for _, test := range tests {
		if icon := notificationIcon(test.level); icon != test.icon {
			t.Fatalf("notificationIcon(%q) = %q, want %q", test.level, icon, test.icon)
		}
		if label := notificationLabel(test.level); label != test.label {
			t.Fatalf("notificationLabel(%q) = %q, want %q", test.level, label, test.label)
		}
	}
}
