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
	"fmt"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type CommitYAMLReader struct {
	Commit *object.Commit
	// allows the consumer to specify its own data store
	DataStore store.Storer[*yaml.RNode]

	Path      string
	SkipDir   bool
	MatchGVKs []schema.GroupVersionKind
}

func (r *CommitYAMLReader) Read(ctx context.Context) (store.Storer[*yaml.RNode], error) {
	if r.Commit == nil {
		return nil, fmt.Errorf("cannot read without a commit")
	}
	datastore := r.DataStore
	if datastore == nil {
		datastore = memory.NewStore[*yaml.RNode](nil)
	}

	// Get the tree from the commit
	tree, err := r.Commit.Tree()
	if err != nil {
		return datastore, err
	}

	// Get the subtree for the specific directory
	subtree, err := tree.Tree(r.Path)
	if err != nil {
		return datastore, nil
	}

	// List files in the subtree
	err = subtree.Files().ForEach(func(f *object.File) error {
		if r.SkipDir && strings.Contains(f.Name, "/") {
			return nil
		}
		if match, err := MatchFilesGlob(YAMLMatch).ShouldSkipFile(f.Name); err != nil {
			return err
		} else if match {
			// skip the file
			return nil
		}
		data, err := f.Contents()
		if err != nil {
			return err
		}
		// Replace the ending \r\n (line ending used in windows) with \n and then split it into multiple YAML documents
		// if it contains document separators (---)
		values, err := SplitDocuments(strings.ReplaceAll(data, "\r\n", "\n"))
		if err != nil {
			return err
		}
		for i := range values {
			// the Split used above will eat the tail '\n' from each resource. This may affect the
			// literal string value since '\n' is meaningful in it.
			if i != len(values)-1 {
				values[i] += "\n"
			}
			rn, err := yaml.Parse(values[i])
			if err != nil {
				return err
			}

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
						Name:      f.Name,
					}), rn)
			}

		}
		return nil
	})
	if err != nil {
		return datastore, err
	}
	return datastore, nil
}
