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

	"github.com/henderiw/store"
	"github.com/kform-dev/kform/pkg/fsys"
)

type FileReader struct {
	FileName string
	Fsys     fsys.FS
	Checksum bool
}

func (r *FileReader) Read(ctx context.Context) (store.Storer[[]byte], error) {
	reader := filereader{
		Checksum: r.Checksum,
		Fsys:     r.Fsys,
	}
	return reader.readFileContent([]string{r.FileName})
}
