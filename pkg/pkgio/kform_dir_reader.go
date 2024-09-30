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
	"sync"

	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/fsys"
	"github.com/kform-dev/kform/pkg/pkgio/ignore"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// only read yaml

type KformDirReader struct {
	Path  string
	Input bool
}

func (r *KformDirReader) Read(ctx context.Context) (store.Storer[*yaml.RNode], error) {
	fsys := fsys.NewDiskFS(r.Path)

	ignoreRules := ignore.Empty(IgnoreFileMatch[0])
	f, err := fsys.Open(IgnoreFileMatch[0])
	if err == nil {
		// if an error is return the rules is empty, so we dont have to worry about the error
		ignoreRules, _ = ignore.Parse(f)
	}
	dirReader := &DirReader{
		RelFsysPath:    ".",
		Fsys:           fsys,
		MatchFilesGlob: YAMLMatch,
		IgnoreRules:    ignoreRules,
		SkipDir:        true, // a package is contained within a single directory, recursion is not needed
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
			if r.Input {
				annotations[kformv1alpha1.KformAnnotationKey_BLOCK_TYPE] = kformv1alpha1.BlockTYPE_INPUT.String()
			}
			f, err := fsys.Open(path)
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
