package tarfile

import (
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/urfave/cli"
)

// Explicit interface checks
var _ io.ReadCloser = &zstdReadCloser{}
var _ io.ReadCloser = &multiReadCloser{}
var _ io.ReadCloser = &splitReadCloser{}

// ZstdReadCloser implements the ReadCloser interface for zstd. The zstd decompressor's Close()
// method doesn't have a return value and therefore doesn't match the ReadCloser interface, so we
// have to wrap it in our own ReadCloser that doesn't expect a return value. We also need to close
// the underlying filehandle.
func ZstdReadCloser(r *zstd.Decoder, c io.Closer) io.ReadCloser {
	return zstdReadCloser{r, c}
}

type zstdReadCloser struct {
	r *zstd.Decoder
	c io.Closer
}

func (z zstdReadCloser) Read(p []byte) (int, error) {
	return z.r.Read(p)
}

func (z zstdReadCloser) Close() error {
	z.r.Close()
	return z.c.Close()
}

// MultiReadCloser implements the ReadCloser interface for decompressors that need to be closed.
// Some decompressors implement a Close function that needs to be called to clean up resources or
// verify checksums, but we also need to ensure that the underlying file gets closed as well.
func MultiReadCloser(r io.ReadCloser, c io.Closer) io.ReadCloser {
	return multiReadCloser{r, c}
}

type multiReadCloser struct {
	r io.ReadCloser
	c io.Closer
}

func (m multiReadCloser) Read(p []byte) (int, error) {
	return m.r.Read(p)
}

func (m multiReadCloser) Close() error {
	var errs []error
	if err := m.r.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := m.c.Close(); err != nil {
		errs = append(errs, err)
	}
	return cli.NewMultiError(errs...)
}

// SplitReadCloser implements the ReadCloser interface for decompressors that don't need to be
// closed.  Some decompressors don't implement a Close function, so we just need to ensure that the
// underlying file gets closed.
func SplitReadCloser(r io.Reader, c io.Closer) io.ReadCloser {
	return splitReadCloser{r, c}
}

type splitReadCloser struct {
	r io.Reader
	c io.Closer
}

func (s splitReadCloser) Read(p []byte) (int, error) {
	return s.r.Read(p)
}

func (s splitReadCloser) Close() error {
	return s.c.Close()
}
