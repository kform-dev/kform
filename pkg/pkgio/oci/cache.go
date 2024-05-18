package oci

import (
	"compress/gzip"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

const (
	errGetNopCache = "cannot get content from a NopCache"
)

const cacheContentExt = ".gz"

// A Cache caches OCI package content.
type Cache interface {
	Has(id string) bool
	Get(id string) (io.ReadCloser, error)
	Store(id string, content io.ReadCloser) error
	Delete(id string) error
}

// fsCache stores and retrieves package content in a filesystem-backed
// cache in a thread-safe manner.
type fsCache struct {
	dir string
	fs  fs.FS
	m   sync.RWMutex
}

// NewFsCache creates a pkg Cache.
func NewFsCache(dir string) Cache {
	return &fsCache{
		dir: dir,
		fs:  os.DirFS(dir),
	}
}

// Has indicates whether an item with the given id is in the cache.
func (r *fsCache) Has(id string) bool {
	if fi, err := os.Stat(filepath.Join(r.dir, id, cacheContentExt)); err == nil && !fi.IsDir() {
		return true
	}
	return false
}

// Get retrieves package contents from the cache.
func (r *fsCache) Get(id string) (io.ReadCloser, error) {
	r.m.RLock()
	defer r.m.RUnlock()
	f, err := r.fs.Open(filepath.Join(r.dir, id, cacheContentExt))
	if err != nil {
		return nil, err
	}
	return GzipReadCloser(f)
}

// Get retrieves package contents from the cache.
func (r *fsCache) Store(id string, content io.ReadCloser) error {
	r.m.Lock()
	defer r.m.Unlock()
	cf, err := os.Create(filepath.Join(r.dir, id, cacheContentExt))
	if err != nil {
		return err
	}
	defer cf.Close() //nolint:errcheck // Error is checked in the happy path.
	w, err := gzip.NewWriterLevel(cf, gzip.BestSpeed)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, content)
	if err != nil {
		return err
	}
	// gzip writer must be closed to ensure all data is flushed to file.
	if err := w.Close(); err != nil {
		return err
	}
	return cf.Close()
}

// Get retrieves package contents from the cache.
func (r *fsCache) Delete(id string) error {
	r.m.Lock()
	defer r.m.Unlock()
	err := os.Remove(filepath.Join(r.dir, id, cacheContentExt))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// NopCache is a cache implementation that does not store anything and always
// returns an error on get.
type NopCache struct{}

// NewNopCache creates a new NopCache.
func NewNopCache() Cache {
	return &NopCache{}
}

// Has indicates whether content is in the NopCache.
func (c *NopCache) Has(string) bool {
	return false
}

// Get retrieves content from the NopCache.
func (c *NopCache) Get(string) (io.ReadCloser, error) {
	return nil, errors.New(errGetNopCache)
}

// Store saves content to the NopCache.
func (c *NopCache) Store(string, io.ReadCloser) error {
	return nil
}

// Delete removes content from the NopCache.
func (c *NopCache) Delete(string) error {
	return nil
}
