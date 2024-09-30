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
)

// only read yaml

type MemReader struct {
	Resources map[string]string
}

func (r *MemReader) Read(ctx context.Context) (store.Storer[[]byte], error) {
	datastore := memory.NewStore[[]byte](nil)

	var wg sync.WaitGroup
	var errm error
	for path, data := range r.Resources {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reader := ByteReader{
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
