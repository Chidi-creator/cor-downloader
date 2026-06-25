package downloader

import "testing"

func TestIsAllowedContentType(t *testing.T) {
	cases := []struct {
		contentType string
		want        bool
	}{
		{"video/mp4", true},
		{"video/mp4; charset=binary", true},
		{"audio/mpeg", true},
		{"application/octet-stream", true},
		{"text/html", false},
		{"application/json", false},
		{"", false},
	}

	for _, c := range cases {
		got := isAllowedContentType(c.contentType)
		if got != c.want {
			t.Errorf("isAllowedContentType(%q) = %v, want %v", c.contentType, got, c.want)
		}
	}
}
