/*
Copyright 2024 Nokia.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pkgio

import (
	"context"
	"io/fs"
	"path/filepath"

	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	"github.com/kform-dev/kform/pkg/fsys"
	"github.com/kform-dev/kform/pkg/pkgio/ignore"
	"github.com/henderiw/logger/log"
)

type PkgReader struct {
	PathExists     bool
	Path           string // dir
	Fsys           fsys.FS
	MatchFilesGlob []string
	IgnoreRules    *ignore.Rules
	SkipDir        bool
	Checksum       bool
}

func (r *PkgReader) Read(ctx context.Context) (store.Storer[[]byte], error) {
	if !r.PathExists {
		return memory.NewStore[[]byte](), nil
	}
	paths, err := r.getPaths(ctx)
	if err != nil {
		return memory.NewStore[[]byte](), err
	}
	reader := filereader{
		Checksum:       r.Checksum,
		Fsys:           r.Fsys,
		MatchFilesGlob: r.MatchFilesGlob,
	}
	return reader.readFileContent(ctx, paths)
}

func (r *PkgReader) getPaths(ctx context.Context) ([]string, error) {
	log := log.FromContext(ctx)
	log.Debug("getPatchs")
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
