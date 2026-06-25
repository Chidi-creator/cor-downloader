package worker

import (
	"testing"
	"time"
)

func TestShouldSendProgressUpdate(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name       string
		now        time.Time
		last       time.Time
		downloaded int64
		total      int64
		want       bool
	}{
		{"first call ever, zero-value last", base, time.Time{}, 1000, 10000, true},
		{"too soon after last update", base.Add(500 * time.Millisecond), base, 2000, 10000, false},
		{"exactly at the interval", base.Add(1 * time.Second), base, 3000, 10000, true},
		{"final chunk, even though recent", base.Add(10 * time.Millisecond), base, 10000, 10000, true},
		{"unknown total, not yet due", base.Add(100 * time.Millisecond), base, 5000, -1, false},
	}

	for _, c := range cases {
		got := shouldSendProgressUpdate(c.now, c.last, c.downloaded, c.total)
		if got != c.want {
			t.Errorf("%s: shouldSendProgressUpdate(...) = %v, want %v", c.name, got, c.want)
		}
	}
}
