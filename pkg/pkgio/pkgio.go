package pkgio

import (
	"context"
	"path/filepath"

	"github.com/henderiw/store"
)

var ReadmeFileMatch = []string{"README.md"}
var IgnoreFileMatch = []string{".ignore"}
var MarkdownMatch = []string{"*.md"}
var YAMLMatch = []string{"*.yaml", "*.yml"}
var JSONMatch = []string{"*.json"}
var MatchAll = []string{"*"}

func isYamlMatch(path string) bool {
	for _, g := range YAMLMatch {
		if match, err := filepath.Match(g, filepath.Base(path)); err != nil {
			// if err we return false -> dont process the file
			return false
		} else if match {
			// if match we return true to process the file
			return true
		}
	}
	// if no match we skip
	return false
}

//var PkgMatch = []string{fmt.Sprintf("*.%s", kformOciPkgExt)}

type Reader[T1 any] interface {
	Read(context.Context) (store.Storer[T1], error)
}

type Writer[T1 any] interface {
	Write(context.Context, store.Storer[T1]) error
}

type Process[T1 any] interface {
	Process(context.Context, store.Storer[T1]) (store.Storer[T1], error)
}

type Pipeline[T1 any] struct {
	Inputs     []Reader[T1]  `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Processors []Process[T1] `json:"processors,omitempty" yaml:"processors,omitempty"`
	Outputs    []Writer[T1]  `json:"outputs,omitempty" yaml:"outputs,omitempty"`
}

func (r Pipeline[T1]) Execute(ctx context.Context) error {
	var data store.Storer[T1]
	var err error
	// read from the inputs
	for _, i := range r.Inputs {
		data, err = i.Read(ctx)
		if err != nil {
			return err
		}
		// copy the data

	}
	for _, p := range r.Processors {
		data, err = p.Process(ctx, data)
		if err != nil {
			return err
		}
	}
	// write to the outputs
	for _, o := range r.Outputs {
		if err := o.Write(ctx, data); err != nil {
			return err
		}
	}
	return nil
}
