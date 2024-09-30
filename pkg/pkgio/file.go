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
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	"github.com/kform-dev/kform/pkg/fsys"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
)

type filereader struct {
	Checksum bool
	Fsys     fsys.FS
}

func (r *filereader) readFileContent(paths []string) (store.Storer[[]byte], error) {
	data := memory.NewStore[[]byte](nil)
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
				data.Create(store.ToKey(path), d)
				return
			}

			if isYamlMatch(path) {
				f, err := r.Fsys.Open(path)
				if err != nil {
					return
				}
				defer f.Close()
				input := &bytes.Buffer{}
				_, err = io.Copy(input, f)
				if err != nil {
					return
				}
				// Replace the ending \r\n (line ending used in windows) with \n and then split it into multiple YAML documents
				// if it contains document separators (---)
				values, err := SplitDocuments(strings.ReplaceAll(input.String(), "\r\n", "\n"))
				if err != nil {
					return
				}
				for i := range values {
					// the Split used above will eat the tail '\n' from each resource. This may affect the
					// literal string value since '\n' is meaningful in it.
					if i != len(values)-1 {
						values[i] += "\n"
					}
					data.Create(store.KeyFromNSN(
						types.NamespacedName{
							Namespace: strconv.Itoa(i),
							Name:      path,
						}), []byte(values[i]))
				}
			} else {
				d, err = r.Fsys.ReadFile(path)
				if err != nil {
					return
				}
				data.Create(store.ToKey(path), d)
			}

		}()
		if err != nil {
			return nil, err
		}
	}
	wg.Wait()
	return data, nil
}

// splitDocuments returns a slice of all documents contained in a YAML string. Multiple documents can be divided by the
// YAML document separator (---). It allows for white space and comments to be after the separator on the same line,
// but will return an error if anything else is on the line.
func SplitDocuments(s string) ([]string, error) {
	docs := make([]string, 0)
	if len(s) > 0 {
		// The YAML document separator is any line that starts with ---
		yamlSeparatorRegexp := regexp.MustCompile(`\n---.*\n`)

		// Find all separators, check them for invalid content, and append each document to docs
		separatorLocations := yamlSeparatorRegexp.FindAllStringIndex(s, -1)
		prev := 0
		for i := range separatorLocations {
			loc := separatorLocations[i]
			separator := s[loc[0]:loc[1]]

			// If the next non-whitespace character on the line following the separator is not a comment, return an error
			trimmedContentAfterSeparator := strings.TrimSpace(separator[4:])
			if len(trimmedContentAfterSeparator) > 0 && trimmedContentAfterSeparator[0] != '#' {
				return nil, errors.Errorf("invalid document separator: %s", strings.TrimSpace(separator))
			}

			docs = append(docs, s[prev:loc[0]])
			prev = loc[1]
		}
		docs = append(docs, s[prev:])
	}

	return docs, nil
}
