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
	"github.com/henderiw/store/memory"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// only read yaml

type KformStreamReader struct {
	Reader io.Reader
	Input  bool
}

func (r *KformStreamReader) Read(ctx context.Context) (store.Storer[*yaml.RNode], error) {
	annotations := map[string]string{}
	if r.Input {
		annotations[kformv1alpha1.KformAnnotationKey_BLOCK_TYPE] = kformv1alpha1.BlockTYPE_INPUT.String()
	}

	datastore := memory.NewStore[*yaml.RNode]()
	reader := YAMLReader{
		Reader:      r.Reader,
		Path:        "stream",
		Annotations: annotations,
		DataStore:   datastore,
	}
	if _, err := reader.Read(ctx); err != nil {
		return datastore, err
	}

	return datastore, nil
}
