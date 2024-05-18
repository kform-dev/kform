package oci

import (
	"compress/gzip"
	"io"
)

var _ io.ReadCloser = &gzipReadCloser{}

// gzipReadCloser reads compressed contents from a file.
type gzipReadCloser struct {
	rc   io.ReadCloser
	gzip *gzip.Reader
}

// GzipReadCloser constructs a new gzipReadCloser from the passed file.
func GzipReadCloser(rc io.ReadCloser) (io.ReadCloser, error) {
	r, err := gzip.NewReader(rc)
	if err != nil {
		return nil, err
	}
	return &gzipReadCloser{
		rc:   rc,
		gzip: r,
	}, nil
}

// Read calls the underlying gzip reader's Read method.
func (g *gzipReadCloser) Read(p []byte) (n int, err error) {
	return g.gzip.Read(p)
}

// Close first closes the gzip reader, then closes the underlying closer.
func (g *gzipReadCloser) Close() error {
	if err := g.gzip.Close(); err != nil {
		_ = g.rc.Close()
		return err
	}
	return g.rc.Close()
}
