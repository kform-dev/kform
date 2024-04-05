/*
Copyright 2023 Nokia.

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

package executor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/henderiw/logger/log"
	"github.com/kform-dev/kform/pkg/dag"
)

type DAGExecutor interface {
	Run(ctx context.Context) bool
}

type ExecHandler[T any] interface {
	BlockRun(ctx context.Context, vertexName string, vertexContext T) bool
	PostRun(ctx context.Context, start, finish time.Time, success bool)
}

//type FuntionRunFn func(ctx context.Context, vertexName string, vertexContext any) bool
//type ExecPostRunFn func(start, finish time.Time, success bool)

type Config[T any] struct {
	Name    string
	From    string
	Handler ExecHandler[T]
}

func NewDAGExecutor[T any](ctx context.Context, d dag.DAG[T], cfg *Config[T]) (DAGExecutor, error) {
	log := log.FromContext(ctx).With("name", cfg.Name)
	if d == nil {
		log.Error("cannot create executor w/o a DAG")
		return nil, fmt.Errorf("cannot create executor w/o a DAG")
	}
	if cfg.Handler == nil {
		log.Error("cannot create executor w/o a Handler")
		return nil, fmt.Errorf("cannot create executor w/o a Handler")
	}
	if cfg.From == "" {
		log.Error("cannot create executor w/o a defined From")
		return nil, fmt.Errorf("cannot create executor w/o a defined From")
	}

	r := &dagExecutor[T]{
		cfg:       *cfg,
		d:         d,
		m:         sync.RWMutex{},
		execMap:   map[string]*execContext[T]{},
		fnDoneMap: map[string]chan bool{},
	}

	// initialize the initial data in the executor
	r.init(ctx)
	return r, nil
}

type dagExecutor[T any] struct {
	d   dag.DAG[T]
	cfg Config[T]

	// cancelFn
	cancelFn context.CancelFunc

	// used during the Walk func
	m         sync.RWMutex
	execMap   map[string]*execContext[T]
	fnDoneMap map[string]chan bool
}

// init initializes the executor with channels and cancel context
// so it is prepaared to execute the dependency map
func (r *dagExecutor[T]) init(ctx context.Context) {
	log := log.FromContext(ctx)
	if r.d == nil {
		log.Error("init failed, no DAG supplied")
		return
	}
	//r.execMap = map[string]*execContext{}
	for vertexName, v := range r.d.GetVertices() {
		log.Debug("init", "vertexName", vertexName)
		r.execMap[vertexName] = &execContext[T]{
			execName:      r.cfg.Name,
			vertexName:    vertexName,
			vertexContext: v,
			doneChs:       make(map[string]chan bool), //snd
			depChs:        make(map[string]chan bool), //rcv
			deps:          make([]string, 0),
			// handler instance of ExecHandler to execute the
			// specific implementation of the vertex
			handler: r.cfg.Handler,
		}
	}
	// build the channel matrix to signal dependencies through channels
	// for every dependency (upstream relationship between vertices)
	// create a channel
	// add the channel to the upstreamm vertex doneCh map ->
	// usedto signal/send the vertex finished the function/job
	// add the channel to the downstream vertex depCh map ->
	// used to wait for the upstream vertex to signal the fn/job is done
	for vertexName, execCtx := range r.execMap {
		// only run these channels when we want to add dependency validation
		for _, depVertexName := range r.d.GetUpVertexes(vertexName) {
			depCh := make(chan bool)
			r.execMap[depVertexName].AddDoneCh(vertexName, depCh) // send when done
			execCtx.AddDepCh(depVertexName, depCh)                // rcvr when done
		}
		execCtx.deps = r.d.GetUpVertexes(vertexName)
		doneFnCh := make(chan bool)
		execCtx.doneFnCh = doneFnCh
		r.fnDoneMap[vertexName] = doneFnCh
	}
}

// Run
func (r *dagExecutor[T]) Run(ctx context.Context) bool {
	from := r.cfg.From
	start := time.Now()
	ctx, cancelFn := context.WithCancel(ctx)
	r.cancelFn = cancelFn
	success := r.execute(ctx, from, true)
	finish := time.Now()

	// handler to execute a final action e.g. recording the overall result
	if r.cfg.Handler != nil {
		r.cfg.Handler.PostRun(ctx, start, finish, success)
	}
	return success
}

func (r *dagExecutor[T]) execute(ctx context.Context, from string, init bool) bool {
	log := log.FromContext(ctx).With("from", from, "init", init)
	log.Debug("execute")
	execCtx := r.getExecContext(from)
	// avoid scheduling a vertex that is already visted
	if !execCtx.isVisted() {
		// updated the exec context with the visited time
		execCtx.updateVisted()
		// execute the vertex function
		log.Debug("execute scheduled vertex", "vertexname", execCtx.vertexName)
		go func() {
			if !r.dependenciesFinished(execCtx.depChs) {
				log.Debug("not finished", "vertexname", from)
			}
			if !execCtx.waitDependencies(ctx) {
				// TODO gather info why the failure occured
				return
			}
			// execute the vertex function
			execCtx.run(ctx)
		}()
	}
	// continue walking the graph
	for _, downEdge := range r.d.GetDownVertexes(from) {
		go func(downEdge string) {
			r.execute(ctx, downEdge, false)
		}(downEdge)
	}
	if init {
		return r.waitFunctionCompletion(ctx)
	}
	return true
}

func (r *dagExecutor[T]) getExecContext(s string) *execContext[T] {
	r.m.RLock()
	defer r.m.RUnlock()
	return r.execMap[s]
}

func (r *dagExecutor[T]) dependenciesFinished(dep map[string]chan bool) bool {
	for vertexName := range dep {
		if !r.getExecContext(vertexName).isFinished() {
			return false
		}
	}
	return true
}

func (r *dagExecutor[T]) waitFunctionCompletion(ctx context.Context) bool {
	log := log.FromContext(ctx)
	log.Debug("main walk wait waiting for function completion...")
DepSatisfied:
	for vertexName, doneFnCh := range r.fnDoneMap {
		for {
			select {
			case d, ok := <-doneFnCh:
				log.Debug("main walk wait rcvd fn done", "from", vertexName, "success", d, "ok", ok)
				if !d {
					r.cancelFn()
					return false
				}
				continue DepSatisfied
			case <-ctx.Done():
				// called when the controller gets cancelled
				return false
			case <-time.After(time.Second * 5):
				log.Debug("main walk wait timeout, waiting", "for", vertexName)
			}
		}
	}
	log.Debug("main walk wait function completion waiting finished - bye !")
	return true
}
