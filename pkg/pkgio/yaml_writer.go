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
	"io"

	"github.com/henderiw/store"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type YAMLWriter struct {
	Writer io.Writer

	Type OutputSink
	// Could be a file or directory
	Path string
}

/*
type fileIndex struct {
	fileName string
	index    string
	entry    string
}
*/

func (r *YAMLWriter) Write(ctx context.Context, datastore store.Storer[*yaml.RNode]) error {
	// preprocess the list
	// remove the specific kform annotations
	//

	/*
		var errm error
		resources := map[string][]string{}
		datastore.List(ctx, func(ctx context.Context, key store.Key, rn *yaml.RNode) {
			// update annotations
			rnAnnotations := rn.GetAnnotations()
			for _, a := range kformv1alpha1.KformAnnotations {
				delete(rnAnnotations, a)
			}
			rn.SetAnnotations(rnAnnotations)

			switch r.Type {
			case OutputSink_Dir: // every entry is an individual file - order does not matter
				os.MkdirAll(filepath.Join(r.Path, filepath.Dir(key.Name)), 0755|os.ModeDir)
				// TBD: do we need to add safety, not to override
				file, err := os.Create(filepath.Join(r.Path, key.Name))
				if err != nil {
					errors.Join(errm, err)
					return
				}
				defer file.Close()
				fmt.Fprintf(file, "%s", rn.MustString())
			case OutputSink_FileRetain: // like a dir - we need to generate the File
			case OutputSink_File: // single file - crd and ns need to be prioritiized
			case OutputSink_StdOut: // like single file - crd and ns need to be prioritiized
			case OutputSink_Memory: // like single file - crd and ns need to be prioritiized
			}

		})
	*/
	return nil
}
