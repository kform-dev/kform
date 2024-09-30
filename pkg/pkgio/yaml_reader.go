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
	"strconv"
	"strings"

	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type YAMLReader struct {
	Reader io.Reader

	// Path of the file the user is reading
	Path string

	// Client supplied annotations
	Annotations map[string]string

	// allows the consumer to specify its own data store
	DataStore store.Storer[*yaml.RNode]

	MatchGVKs []schema.GroupVersionKind
}

func (r *YAMLReader) Read(ctx context.Context) (store.Storer[*yaml.RNode], error) {
	datastore := r.DataStore
	if datastore == nil {
		datastore = memory.NewStore[*yaml.RNode](nil)
	}

	// by manually splitting resources -- otherwise the decoder will get the Resource
	// boundaries wrong for header comments.
	input := &bytes.Buffer{}
	_, err := io.Copy(input, r.Reader)
	if err != nil {
		return datastore, err
	}

	// Replace the ending \r\n (line ending used in windows) with \n and then split it into multiple YAML documents
	// if it contains document separators (---)
	values, err := SplitDocuments(strings.ReplaceAll(input.String(), "\r\n", "\n"))
	if err != nil {
		return datastore, err
	}
	for i := range values {
		// the Split used above will eat the tail '\n' from each resource. This may affect the
		// literal string value since '\n' is meaningful in it.
		if i != len(values)-1 {
			values[i] += "\n"
		}
		rn, err := yaml.Parse(values[i])
		if err != nil {
			return datastore, err
		}
		r.updateAnnotations(rn, i)

		filter := true
		if len(r.MatchGVKs) == 0 {
			filter = false
		} else {
			for _, gvk := range r.MatchGVKs {
				if rn.GetApiVersion() == gvk.GroupVersion().Identifier() &&
					rn.GetKind() == gvk.Kind {
					filter = false
				}
			}
		}
		if !filter {
			datastore.Create(store.KeyFromNSN(
				types.NamespacedName{
					Namespace: strconv.Itoa(i),
					Name:      r.Path,
				}), rn)
		}

	}

	return datastore, nil
}

func (r *YAMLReader) updateAnnotations(rn *yaml.RNode, _ int) {
	annotations := rn.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	//annotations[KformInternalPkgioAnnotationInternalKey_Path] = r.Path
	//annotations[KformInternalPkgioAnnotationInternalKey_Index] = strconv.Itoa(i)

	// add user supplied annotations
	for k, v := range r.Annotations {
		annotations[k] = v
	}
	rn.SetAnnotations(annotations)
}
