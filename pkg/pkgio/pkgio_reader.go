package pkgio

import (
	"context"
	"io/fs"
	"path/filepath"
	"sync"

	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	"github.com/kform-dev/kform/pkg/fsys"
	"github.com/kform-dev/kform/pkg/pkgio/ignore"
)

func NewData() store.Storer[[]byte] {
	return memory.NewStore[[]byte]()
}

type PkgReader struct {
	PathExists     bool
	Fsys           fsys.FS
	MatchFilesGlob []string
	IgnoreRules    *ignore.Rules
	SkipDir        bool
	Checksum       bool
}

func (r *PkgReader) Read(ctx context.Context, data store.Storer[[]byte]) (store.Storer[[]byte], error) {
	if !r.PathExists {
		return data, nil
	}
	paths, err := r.getPaths(ctx)
	if err != nil {
		return data, err
	}
	return r.readFileContent(ctx, paths, data)
}

func (r *PkgReader) getPaths(ctx context.Context) ([]string, error) {
	// collect the paths
	paths := []string{}
	if err := r.Fsys.Walk(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Directory-based ignore rules involve skipping the entire
			// contents of that directory.
			if r.IgnoreRules.Ignore(path, d) {
				return filepath.SkipDir
			}
			if r.SkipDir && d.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if r.IgnoreRules.Ignore(path, d) {
			return nil
		}
		// process glob
		if match, err := r.shouldSkipFile(path); err != nil {
			return err
		} else if match {
			// skip the file
			return nil
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		return nil, err
	}
	return paths, nil
}

func (r *PkgReader) readFileContent(ctx context.Context, paths []string, data store.Storer[[]byte]) (store.Storer[[]byte], error) {
	var wg sync.WaitGroup
	for _, path := range paths {
		path := path
		wg.Add(1)
		var err error
		go func() {
			defer wg.Done()
			var d []byte
			if r.Checksum {
				hash, err := r.Fsys.Sha256(path)
				if err != nil {
					return
				}
				d = []byte(hash)
			} else {
				d, err = r.Fsys.ReadFile(path)
				if err != nil {
					return
				}
			}
			data.Create(ctx, store.ToKey(path), d)
		}()
		if err != nil {
			return nil, err
		}
	}
	wg.Wait()

	return data, nil
}

func (r *PkgReader) Write(store.Storer[[]byte]) error {
	return nil
}

func (r *PkgReader) shouldSkipFile(path string) (bool, error) {
	for _, g := range r.MatchFilesGlob {
		if match, err := filepath.Match(g, filepath.Base(path)); err != nil {
			// if err we should skip the file
			return true, err
		} else if match {
			// if matchw e should include the file
			return false, nil
		}
	}
	// if no match we should skip the file
	return true, nil
}
