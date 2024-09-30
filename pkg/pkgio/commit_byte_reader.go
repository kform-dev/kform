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
	"strings"

	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
)

type CommitByteReader struct {
	Commit *object.Commit
	// allows the consumer to specify its own data store
	DataStore store.Storer[[]byte]

	Path           string
	MatchFilesGlob MatchFilesGlob
	SkipDir        bool
}

func (r *CommitByteReader) Read(ctx context.Context) (store.Storer[[]byte], error) {
	if r.Commit == nil {
		return nil, fmt.Errorf("cannot read without a commit")
	}
	datastore := r.DataStore
	if datastore == nil {
		datastore = memory.NewStore[[]byte](nil)
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
		if r.SkipDir && strings.Contains(strings.TrimPrefix(f.Name, r.Path+"/"), "/") {
			return nil
		}
		if match, err := r.MatchFilesGlob.ShouldSkipFile(f.Name); err != nil {
			return err
		} else if match {
			// skip the file
			return nil
		}
		data, err := f.Contents()
		if err != nil {
			return err
		}
		datastore.Create(store.ToKey(f.Name), []byte(data))
		return nil
	})
	if err != nil {
		return datastore, err
	}
	return datastore, nil
}
