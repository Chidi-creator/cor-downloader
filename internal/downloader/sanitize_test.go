package downloader

import (
	"strings"
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"Easily our best CB when he joins", "Easily our best CB when he joins"},
		{"../../etc/passwd", "etcpasswd"},
		{"a/b\\c", "abc"},
		{"...", "download"},
		{"  spaced out  ", "spaced out"},
		{"", "download"},
	}

	for _, c := range cases {
		got := SanitizeFilename(c.name)
		if got != c.want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestSanitizeFilenameTruncatesLongNames(t *testing.T) {
	long := strings.Repeat("a", 300)
	got := SanitizeFilename(long)
	if len(got) != 150 {
		t.Fatalf("expected truncation to 150 chars, got %d", len(got))
	}
}
