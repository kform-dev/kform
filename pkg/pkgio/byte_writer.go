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
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/henderiw/store"
)

type ByteWriter struct {
	Type OutputSink
	// Could be a file or directory
	Path string
}

func (r *ByteWriter) Write(ctx context.Context, datastore store.Storer[[]byte]) error {

	var errm error
	datastore.List(ctx, func(ctx context.Context, key store.Key, b []byte) {
		switch r.Type {
		case OutputSink_StdOut:
			fmt.Fprintf(os.Stdout, "---\nfile: %s\n---\n%s\n", key.Name, string(b))
		case OutputSink_Memory:
			var buf bytes.Buffer
			fmt.Fprintf(&buf, "---\n%s\n%s", key.Name, string(b))
		case OutputSink_Dir:
			os.MkdirAll(filepath.Join(r.Path, filepath.Dir(key.Name)), 0755|os.ModeDir)
			// TBD: do we need to add safety, not to override
			file, err := os.Create(filepath.Join(r.Path, key.Name))
			if err != nil {
				errors.Join(errm, err)
				return
			}
			defer file.Close()
			fmt.Fprintf(file, "%s", string(b))
		}

	})
	return errm
}
