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
	"errors"
	"io/fs"
	"sync"

	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	"github.com/kform-dev/kform/pkg/fsys"
	"github.com/kform-dev/kform/pkg/pkgio/ignore"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// only read yaml
type YAMLDirReader struct {
	FsysPath    string
	RelFsysPath string
	Fsys        fs.FS
	SkipDir     bool
	MatchGVKs   []schema.GroupVersionKind
}

func (r *YAMLDirReader) Read(ctx context.Context) (store.Storer[*yaml.RNode], error) {
	var fs fsys.FS
	if r.Fsys != nil {
		fs = fsys.NewFS(r.Fsys)
	} else {
		fs = fsys.NewDiskFS(r.FsysPath)
	}

	ignoreRules := ignore.Empty(IgnoreFileMatch[0])
	f, err := fs.Open(IgnoreFileMatch[0])
	if err == nil {
		// if an error is return the rules is empty, so we dont have to worry about the error
		ignoreRules, _ = ignore.Parse(f)
	}
	dirReader := &DirReader{
		RelFsysPath:    r.RelFsysPath,
		Fsys:           fs,
		MatchFilesGlob: YAMLMatch,
		IgnoreRules:    ignoreRules,
		SkipDir:        r.SkipDir,
	}
	paths, err := dirReader.getPaths(ctx)
	if err != nil {
		return nil, err
	}
	datastore := memory.NewStore[*yaml.RNode](nil)
	var errm error
	var wg sync.WaitGroup
	for _, path := range paths {
		path := path
		wg.Add(1)

		go func() {
			defer wg.Done()
			annotations := map[string]string{}
			f, err := fs.Open(path)
			if err != nil {
				errors.Join(errm, err)
				return
			}
			defer f.Close()

			reader := YAMLReader{
				Reader:      f,
				Path:        path,
				Annotations: annotations,
				DataStore:   datastore,
				MatchGVKs:   r.MatchGVKs,
			}
			if _, err = reader.Read(ctx); err != nil {
				errors.Join(errm, err)
			}
		}()

	}
	wg.Wait()
	if errm != nil {
		return datastore, errm
	}
	return datastore, nil
}
