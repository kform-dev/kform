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
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// only read yaml

type KformMemReader struct {
	Resources map[string]string
	Data      store.Storer[[]byte]
	Input     bool
}

func (r *KformMemReader) Read(ctx context.Context) (store.Storer[*yaml.RNode], error) {
	annotations := map[string]string{}
	if r.Input {
		annotations[kformv1alpha1.KformAnnotationKey_BLOCK_TYPE] = kformv1alpha1.BlockTYPE_INPUT.String()
	}
	var wg sync.WaitGroup
	var errm error
	datastore := memory.NewStore[*yaml.RNode]()
	if r.Data != nil {
		r.Data.List(ctx, func(ctx context.Context, k store.Key, b []byte) {
			// only look at yaml files
			if match, err := MatchFilesGlob(YAMLMatch).shouldSkipFile(k.Name); err != nil {
				errors.Join(errm, err)
				return
			} else if match {
				return
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				reader := YAMLReader{
					Reader:      strings.NewReader(string(b)),
					Path:        k.Name,
					Annotations: annotations,
					DataStore:   datastore,
				}
				if _, err := reader.Read(ctx); err != nil {
					errors.Join(errm, err)
				}
			}()

		})
	}

	for path, data := range r.Resources {
		data := data
		path := path
		// only look at yaml files
		if match, err := MatchFilesGlob(YAMLMatch).shouldSkipFile(path); err != nil {
			errors.Join(errm, err)
			continue
		} else if match {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			reader := YAMLReader{
				Reader:      strings.NewReader(data),
				Path:        path,
				Annotations: annotations,
				DataStore:   datastore,
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
