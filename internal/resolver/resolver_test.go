package resolver

import "testing"

func TestPickBestFormatPicksTallestProgressive(t *testing.T) {
	formats := []Format{
		{Protocol: "m3u8_native", VCodec: "h264", ACodec: "aac", Height: 1080}, // HLS, excluded
		{Protocol: "https", VCodec: "none", ACodec: "aac", Height: 720},        // audio-only, excluded
		{Protocol: "https", VCodec: "h264", ACodec: "aac", Height: 360},
		{Protocol: "https", VCodec: "h264", ACodec: "aac", Height: 720}, // should win
	}

	best, ok := pickBestFormat(formats)
	if !ok {
		t.Fatal("expected a usable format to be found")
	}
	if best.Height != 720 {
		t.Fatalf("expected height 720, got %d", best.Height)
	}
}

func TestPickBestFormatTreatsUnspecifiedCodecAsPresent(t *testing.T) {
	// Mirrors real X/Twitter data: direct https formats report vcodec/acodec
	// as empty (unspecified) rather than "none", even though they're
	// complete progressive files with both tracks. Only an explicit "none"
	// should disqualify a format.
	formats := []Format{
		{Protocol: "m3u8_native", VCodec: "avc1.4D401E", ACodec: "none", Height: 360}, // HLS video-only, excluded
		{Protocol: "https", VCodec: "", ACodec: "", Height: 360},
		{Protocol: "https", VCodec: "", ACodec: "", Height: 720}, // should win
	}

	best, ok := pickBestFormat(formats)
	if !ok {
		t.Fatal("expected a usable format to be found")
	}
	if best.Height != 720 {
		t.Fatalf("expected height 720, got %d", best.Height)
	}
}

func TestPickBestFormatNoneAvailable(t *testing.T) {
	formats := []Format{
		{Protocol: "m3u8_native", VCodec: "h264", ACodec: "aac", Height: 1080},
	}

	_, ok := pickBestFormat(formats)
	if ok {
		t.Fatal("expected no progressive format to be found")
	}
}
