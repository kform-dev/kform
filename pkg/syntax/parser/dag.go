package parser

import (
	"context"

	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	"github.com/kform-dev/kform/pkg/recorder/diag"
)

func (r *KformParser) generateProviderDAG(ctx context.Context, unrefed []string) {
	log := log.FromContext(ctx)
	log.Debug("generating provider DAG")
	rootPackage, err := r.GetRootPackage(ctx)
	if err != nil {
		r.recorder.Record(diag.DiagFromErr(err))
		return
	}
	rootPackage.GenerateDAG(ctx, true, unrefed)
	// update the module with the DAG in the cache
	r.packages.Update(ctx, store.ToKey(r.rootPackageName), rootPackage)
}

func (r *KformParser) generateDAG(ctx context.Context) {
	log := log.FromContext(ctx)
	log.Debug("generating DAG")
	for packageName, pkg := range r.ListPackages(ctx) {
		// generate a regular DAG
		pkg.GenerateDAG(ctx, false, []string{})
		// update the module with the DAG in the cache
		r.packages.Update(ctx, store.ToKey(packageName), pkg)
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
					pkg.DAG.UpdateVertex(ctx, vertexName, vCtx)
				}
			}
		}
	}
}
