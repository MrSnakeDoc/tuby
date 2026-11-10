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
