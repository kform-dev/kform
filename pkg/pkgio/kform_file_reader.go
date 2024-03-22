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
	"os"

	"github.com/henderiw/store"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// only read yaml

type KformFileReader struct {
	Path  string
	Input bool
}

func (r *KformFileReader) Read(ctx context.Context) (store.Storer[*yaml.RNode], error) {
	if match, err := MatchFilesGlob(YAMLMatch).shouldSkipFile(r.Path); err != nil {
		return nil, fmt.Errorf("not a yaml file, err: %s", err.Error())
	} else if match {
		return nil, fmt.Errorf("not a yaml file")
	}

	f, err := os.Open(r.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	annotations := map[string]string{}
	if r.Input {
		annotations[kformv1alpha1.KformAnnotationKey_BLOCK_TYPE] = kformv1alpha1.BlockTYPE_INPUT.String()
	}

	reader := YAMLReader{
		Reader:      f,
		Path:        r.Path,
		Annotations: annotations,
	}
	return reader.Read(ctx)
}
