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

package dag

import (
	"context"
	"fmt"
	"os"
	"sync"
)

const (
	// errors
	NotFound = "not found"
	Root     = "root"
)

type DAG[T1 any] interface {
	AddVertex(ctx context.Context, s string, v T1) error
	UpdateVertex(ctx context.Context, s string, v T1) error
	Connect(ctx context.Context, from, to string)
	AddDownEdge(ctx context.Context, from, to string)
	AddUpEdge(ctx context.Context, from, to string)
	VertexExists(s string) bool
	GetVertex(s string) (T1, error)
	GetVertices() map[string]T1
	GetDownVertexes(from string) []string
	GetUpVertexes(from string) []string
	TransitiveReduction(ctx context.Context)
	Print(name string)
	PrintFrom(name, from string)
}

// used for returning
type Edge struct {
	From string
	To   string
}

type dag[T1 any] struct {
	// vertices first key is the vertexName
	mv       sync.RWMutex
	vertices map[string]T1
	// downEdges/upEdges
	// 1st key is from, 2nd key is to
	mde       sync.RWMutex
	downEdges map[string]map[string]struct{}
	mue       sync.RWMutex
	upEdges   map[string]map[string]struct{}
	// used for transit reduction
	mvd         sync.RWMutex
	vertexDepth map[string]int

	// tempDepMap
	tempDepMap map[string]struct{}
}

func New[T1 any]() DAG[T1] {
	return &dag[T1]{
		//dagCtx:    dagCtx,
		vertices:  make(map[string]T1),
		downEdges: make(map[string]map[string]struct{}),
		upEdges:   make(map[string]map[string]struct{}),
	}
}

func (r *dag[T1]) AddVertex(ctx context.Context, s string, v T1) error {
	r.mv.Lock()
	defer r.mv.Unlock()

	// validate duplicate entry
	if _, ok := r.vertices[s]; ok {
		return fmt.Errorf("duplicate vertex entry: %s", s)
	}
	r.vertices[s] = v
	return nil
}

func (r *dag[T1]) UpdateVertex(ctx context.Context, s string, v T1) error {
	r.mv.Lock()
	defer r.mv.Unlock()

	// validate duplicate entry
	if _, ok := r.vertices[s]; !ok {
		return fmt.Errorf("vertex entry not found: %s", s)
	}
	r.vertices[s] = v
	return nil
}

func (r *dag[T1]) GetVertices() map[string]T1 {
	r.mv.RLock()
	defer r.mv.RUnlock()
	vcs := map[string]T1{}
	for vertexName, v := range r.vertices {
		vcs[vertexName] = v
	}
	return vcs

}

func (r *dag[T1]) VertexExists(s string) bool {
	r.mv.RLock()
	defer r.mv.RUnlock()
	_, ok := r.vertices[s]
	return ok
}

func (r *dag[T1]) GetVertex(s string) (T1, error) {
	r.mv.RLock()
	defer r.mv.RUnlock()
	d, ok := r.vertices[s]
	if !ok {
		return *new(T1), fmt.Errorf("%s, name: %s", NotFound, s)
	}
	return d, nil
}

func (r *dag[T1]) Connect(ctx context.Context, from, to string) {
	//fmt.Printf("connect dag: %s -> %s\n", to, from)
	r.AddDownEdge(ctx, from, to)
	r.AddUpEdge(ctx, to, from)
}

func (r *dag[T1]) Disconnect(ctx context.Context, from, to string) {
	r.DeleteDownEdge(ctx, from, to)
	r.DeleteUpEdge(ctx, to, from)
}

func (r *dag[T1]) AddDownEdge(ctx context.Context, from, to string) {
	r.mde.Lock()
	defer r.mde.Unlock()

	// initialize the from entry if it does not exist
	if _, ok := r.downEdges[from]; !ok {
		r.downEdges[from] = make(map[string]struct{})
	}
	if _, ok := r.downEdges[from][to]; ok {
		//  down edge entry already exists
		return
	}
	// add entry
	r.downEdges[from][to] = struct{}{}
}

func (r *dag[T1]) DeleteDownEdge(ctx context.Context, from, to string) {
	r.mde.Lock()
	defer r.mde.Unlock()

	//fmt.Printf("deleteDownEdge: from: %s, to: %s\n", from, to)
	if de, ok := r.downEdges[from]; ok {
		if _, ok := r.downEdges[from][to]; ok {
			delete(de, to)
		}
	}
}

func (r *dag[T1]) GetDownVertexes(from string) []string {
	r.mde.RLock()
	defer r.mde.RUnlock()

	edges := make([]string, 0)
	if fromVertex, ok := r.downEdges[from]; ok {
		for to := range fromVertex {
			edges = append(edges, to)
		}
	}
	return edges
}

func (r *dag[T1]) AddUpEdge(ctx context.Context, from, to string) {
	r.mue.Lock()
	defer r.mue.Unlock()

	// initialize the from entry if it does not exist
	if _, ok := r.upEdges[from]; !ok {
		r.upEdges[from] = make(map[string]struct{})
	}
	if _, ok := r.upEdges[from][to]; ok {
		//  up edge entry already exists
		return
	}
	// add entry
	r.upEdges[from][to] = struct{}{}
}

func (r *dag[T1]) DeleteUpEdge(ctx context.Context, from, to string) {
	r.mue.Lock()
	defer r.mue.Unlock()

	if ue, ok := r.upEdges[from]; ok {
		if _, ok := r.upEdges[from][to]; ok {
			delete(ue, to)
		}
	}
}

func (r *dag[T1]) GetUpEdges(from string) []Edge {
	r.mue.RLock()
	defer r.mue.RUnlock()

	edges := make([]Edge, 0)
	if fromVertex, ok := r.upEdges[from]; ok {
		for to := range fromVertex {
			edges = append(edges, Edge{From: from, To: to})
		}
	}
	return edges
}

func (r *dag[T1]) GetUpVertexes(from string) []string {
	r.mue.RLock()
	defer r.mue.RUnlock()

	upVerteces := []string{}
	if fromVertex, ok := r.upEdges[from]; ok {
		for to := range fromVertex {
			upVerteces = append(upVerteces, to)
		}
	}
	return upVerteces
}

func (r *dag[T1]) Print(name string) {
	r.printFrom(name, Root)
}

func (r *dag[T1]) PrintFrom(name, from string) {
	r.printFrom(name, from)
}

func (r *dag[T1]) printFrom(name, from string) {
	fmt.Println()
	r.printVertices(name)
	fmt.Printf("######### DAG %s dependency map start ###########\n", name)
	r.tempDepMap = map[string]struct{}{
		from: {},
	}
	r.getDependencyMap(from, 0)
	fmt.Printf("######### DAG %s dependency map end   ###########\n", name)
	fmt.Println()
}

func (r *dag[T1]) printVertices(module string) {
	fmt.Printf("###### DAG %s start #######\n", module)
	for vertexName := range r.GetVertices() {
		fmt.Printf("vertexname: %s upVertices: %v, downVertices: %v\n", vertexName, r.GetUpVertexes(vertexName), r.GetDownVertexes(vertexName))
	}
	fmt.Printf("###### DAG %s output stop #######\n", module)
}

func (r *dag[T1]) getDependencyMap(from string, indent int) {
	fmt.Printf("%s:\n", from)
	for _, upVertex := range r.GetUpVertexes(from) {
		found := r.checkVertex(upVertex)
		if !found {
			fmt.Printf("upVertex %s no found in vertices\n", upVertex)
			os.Exit(1)
		}
		fmt.Printf("-> %s\n", upVertex)
	}
	indent++
	for _, downVertex := range r.GetDownVertexes(from) {
		if _, ok := r.tempDepMap[downVertex]; ok {
			continue
		}
		r.tempDepMap[downVertex] = struct{}{}
		found := r.checkVertex(downVertex)
		if !found {
			fmt.Printf("upVertex %s no found in vertices\n", downVertex)
			os.Exit(1)
		}
		//fmt.Printf("<- %s\n", downVertex)
		r.getDependencyMap(downVertex, indent)
	}
}

func (r *dag[T1]) checkVertex(s string) bool {
	for vertexName := range r.GetVertices() {
		if vertexName == s {
			return true
		}
	}
	return false
}
