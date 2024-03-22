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
	"bytes"
	"context"
	"io"

	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
)

type ByteReader struct {
	Reader io.Reader

	// allows the consumer to specify its own data store
	DataStore store.Storer[[]byte]

	// Path of the file the user is reading
	Path string
}

func (r *ByteReader) Read(ctx context.Context) (store.Storer[[]byte], error) {
	datastore := r.DataStore
	if datastore == nil {
		datastore = memory.NewStore[[]byte]()
	}

	input := &bytes.Buffer{}
	_, err := io.Copy(input, r.Reader)
	if err != nil {
		return datastore, err
	}
	datastore.Create(ctx, store.ToKey(r.Path), input.Bytes())

	return datastore, nil
}
