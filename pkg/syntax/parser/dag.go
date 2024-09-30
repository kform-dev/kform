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

package parser

import (
	"context"

	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"k8s.io/apimachinery/pkg/util/sets"
)

func (r *KformParser) generateProviderDAG(ctx context.Context, usedProviderConfigs sets.Set[string]) {
	log := log.FromContext(ctx)
	log.Debug("generating provider DAG")
	rootPackage, err := r.GetRootPackage(ctx)
	if err != nil {
		r.recorder.Record(diag.DiagFromErr(err))
		return
	}
	rootPackage.GenerateDAG(ctx, true, usedProviderConfigs)
	// update the module with the DAG in the cache
	r.packages.Update(store.ToKey(r.rootPackageName), rootPackage)
}

func (r *KformParser) generateDAG(ctx context.Context) {
	log := log.FromContext(ctx)
	log.Debug("generating DAG")
	for packageName, pkg := range r.ListPackages(ctx) {
		// generate a regular DAG, for a regular dag the provider configs don't matter
		if err := pkg.GenerateDAG(ctx, false, nil); err != nil {
			r.recorder.Record(diag.DiagFromErr(err))
			return
		}
		// update the module with the DAG in the cache
		if err := r.packages.Update(store.ToKey(packageName), pkg); err != nil {
			r.recorder.Record(diag.DiagFromErr(err))
			return
		}
	}
	// since we call a DAG in hierarchy we need to update the DAGs with the calling DAG
	// This is done after all the DAG(s) are generated
	// We walk over all the packages -> they all should have a DAG now
	// We walk over the DAG vertices of each package and walk over the packages again since they use mixins
	// so the DAG(s) need to be updated in the calling module vertex (an adajacent module)
	// for each vertex where the name matches with the module name we update the vertexCtx
	// with the DAG
	for _, pkg := range r.ListPackages(ctx) {
		for vertexName, vCtx := range pkg.DAG.GetVertices() {
			for packageName, pkg := range r.ListPackages(ctx) {
				if vertexName == packageName {
					vCtx.DAG = pkg.DAG
					if err := pkg.DAG.UpdateVertex(ctx, vertexName, vCtx); err != nil {
						r.recorder.Record(diag.DiagFromErr(err))
						return
					}
				}
			}
		}
	}
}
