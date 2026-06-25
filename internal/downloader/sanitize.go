package downloader

import "strings"

// SanitizeFilename strips characters that are unsafe or meaningless in a
// filename, so a post title can be safely used as one.
func SanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, "..", "")

	name = strings.Map(func(r rune) rune {
		switch {
		case r == '/' || r == '\\' || r == 0:
			return -1
		case r < 0x20:
			return -1
		default:
			return r
		}
	}, name)

	name = strings.TrimSpace(name)
	name = strings.Trim(name, ".")

	if name == "" {
		name = "download"
	}

	const maxLen = 150
	if len(name) > maxLen {
		name = name[:maxLen]
	}

	return name
}
