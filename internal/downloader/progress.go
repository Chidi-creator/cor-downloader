package downloader

import "io"

// progressReader wraps an io.Reader, reporting cumulative bytes read via
// onProgress on every Read call. It satisfies io.Reader itself, so it can
// be used anywhere a reader is expected (e.g. as io.Copy's source).
type progressReader struct {
	reader     io.Reader
	total      int64
	downloaded int64
	onProgress ProgressFunc
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.reader.Read(buf)
	if n > 0 {
		p.downloaded += int64(n)
		p.onProgress(p.downloaded, p.total)
	}
	return n, err
}
