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
	"sync"
	"time"

	"github.com/henderiw/logger/log"
)

type execContext[T any] struct {
	execName   string
	vertexName string

	// used to signal the vertex function is done
	// to the main walk entry
	doneFnCh chan bool
	// used to handle the dependencies between the functions
	m sync.RWMutex
	// used to send fn result from the src function
	// to the dependent function
	doneChs map[string]chan bool
	// used by the dependent vertex function to rcv the result
	// of the dependent src function
	depChs map[string]chan bool
	deps   []string
	// identifies the time the vertex got scheduled
	visited time.Time
	// identifies the time the vertex fn finished
	finished time.Time

	//vertexContext *rtdag.VertexContext
	vertexContext T

	// handler
	handler ExecHandler[T]
}

type VertexResult struct {
	Start   time.Time
	Finish  time.Time
	Success bool
	Reason  string
	Input   any
	Output  any
}

func (r *execContext[T]) ListDoneCh() map[string]chan bool {
	r.m.RLock()
	defer r.m.RUnlock()
	doneChs := make(map[string]chan bool, len(r.doneChs))
	for k, v := range r.doneChs {
		doneChs[k] = v
	}
	return doneChs
}

func (r *execContext[T]) AddDoneCh(n string, c chan bool) {
	r.m.Lock()
	defer r.m.Unlock()
	r.doneChs[n] = c
}

func (r *execContext[T]) AddDepCh(n string, c chan bool) {
	r.m.Lock()
	defer r.m.Unlock()
	r.depChs[n] = c
}

func (r *execContext[T]) isFinished() bool {
	r.m.RLock()
	defer r.m.RUnlock()
	return !r.finished.IsZero()
}

func (r *execContext[T]) updateFinished() {
	r.m.Lock()
	defer r.m.Unlock()
	r.finished = time.Now()
}

func (r *execContext[T]) isVisted() bool {
	r.m.RLock()
	defer r.m.RUnlock()
	return !r.visited.IsZero()
}

func (r *execContext[T]) updateVisted() {
	r.m.Lock()
	defer r.m.Unlock()
	r.visited = time.Now()
}

// run is executed in a go routine
func (r *execContext[T]) run(ctx context.Context) {
	log := log.FromContext(ctx).With("vertexName", r.vertexName)
	// execute the handler that runs the function
	success := r.handler.BlockRun(ctx, r.vertexName, r.vertexContext)
	//r.finished = time.Now()
	r.updateFinished()
	doneChs := r.ListDoneCh()
	downVertices := []string{}
	for k := range doneChs {
		downVertices = append(downVertices, k)
	}
	log.Info("block run finished", "downVertices", downVertices)
	// signal to the dependent function the result of the vertex fn execution
	for vertexName, doneCh := range doneChs {
		doneCh <- success
		close(doneCh)
		log.Info("sent done", "from", r.vertexName, "to", vertexName)
		//fmt.Printf("execContext execName %s vertexName: %s -> %s send done\n", r.execName, r.vertexName, vertexName)
	}
	// signal the result of the vertex execution to the main walk
	r.doneFnCh <- success
	close(r.doneFnCh)
	log.Info("done")
	//fmt.Printf("execContext execName %s vertexName: %s -> walk main fn done\n", r.execName, r.vertexName)
}

func (r *execContext[T]) waitDependencies(ctx context.Context) bool {
	// for each dependency wait till a it completed, either through
	// the dependency Channel or cancel or
	log := log.FromContext(ctx).With("vertexName", r.vertexName)
	log.Info("wait dependencies", "deps", r.deps)
	//fmt.Printf("execContext execName %s vertexName: %s wait dependencies: %v\n", r.execName, r.vertexName, r.depChs)
DepSatisfied:
	for depVertexName, depCh := range r.depChs {
		//fmt.Printf("waitDependencies %s -> %s\n", depVertexName, r.vertexName)
		//DepSatisfied:
		for {
			select {
			case d, ok := <-depCh:
				log.Info("rcvd done", "from", depVertexName, "to", r.vertexName, "success", d, "ok", ok)
				//fmt.Printf("execContext execName %s: %s -> %s rcvd done, d: %t, ok: %t\n", r.execName, depVertexName, r.vertexName, d, ok)
				if ok {
					continue DepSatisfied
				}
				if !d {
					// dependency failed
					return false
				}
				continue DepSatisfied
			case <-time.After(time.Second * 5):
				log.Info("rwait timeout, waiting", "for", depVertexName)
				//fmt.Printf("execContext execName %s vertexName: %s wait timeout, is waiting for %s\n", r.execName, r.vertexName, depVertexName)
			}
		}
	}
	log.Info("finished waiting ...")
	//fmt.Printf("execContext execName %s vertexName: %s finished waiting\n", r.execName, r.vertexName)
	return true
}
