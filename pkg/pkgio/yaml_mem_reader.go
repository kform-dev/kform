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
	"strings"
	"sync"

	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// only read yaml

type YAMLMemReader struct {
	Resources map[string]string
}

func (r *YAMLMemReader) Read(ctx context.Context) (store.Storer[*yaml.RNode], error) {
	datastore := memory.NewStore[*yaml.RNode](nil)

	var wg sync.WaitGroup
	var errm error
	for path, data := range r.Resources {
		data := data
		path := path
		// only look at yaml files
		if match, err := MatchFilesGlob(YAMLMatch).ShouldSkipFile(path); err != nil {
			errors.Join(errm, err)
			continue
		} else if match {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			reader := YAMLReader{
				Reader:    strings.NewReader(data),
				Path:      path,
				DataStore: datastore,
			}
			if _, err := reader.Read(ctx); err != nil {
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
