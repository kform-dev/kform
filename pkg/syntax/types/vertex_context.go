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

package types

import (
	"fmt"

	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/dag"
	"github.com/kform-dev/kform/pkg/data"
	"k8s.io/apimachinery/pkg/util/sets"
)

type VertexContext struct {
	// FileName and PackageName provide context in which this vertex is handled
	FileName    string `json:"fileName"`
	Index       string `json:"index"`
	PackageName string `json:"packageName"`
	// BlockType determines which function we need to execute
	BlockType kformv1alpha1.BlockType `json:"blockType"`
	// BlockName has syntx <namespace>.<name>
	BlockName string `json:"blockName"`
	// provides the contextual data
	Data            data.BlockData
	Attributes      *kformv1alpha1.Attributes
	Dependencies    sets.Set[string]
	PkgDependencies sets.Set[string]
	// only relevant for blocktype resource, data and list
	//Provider string
	// only relevant for blocktype package/mixin
	// can be either a regular DAG or a provider DAG
	DAG dag.DAG[*VertexContext]
}

func (r *VertexContext) String() string {
	return fmt.Sprintf(
		"fileName: %s, packageName: %s, blockType: %s, blockName: %s",
		r.FileName,
		r.PackageName,
		r.BlockType.String(),
		r.BlockName,
	)
}

func (r *VertexContext) AddDAG(d dag.DAG[*VertexContext]) {
	r.DAG = d
}

func (r *VertexContext) GetDependencies() sets.Set[string] {
	return r.Dependencies
}

func (r *VertexContext) GetBlockDependencies() sets.Set[string] {
	blockDeps := sets.New[string]()
	for k := range r.Dependencies {
		// filter out the dependencies that refer to loop variables
		if _, ok := kformv1alpha1.LocalVars[k]; !ok {
			blockDeps.Insert(k)
		}
	}
	return blockDeps
}
