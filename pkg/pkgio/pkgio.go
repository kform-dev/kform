package pkgio

import (
	"context"

	"github.com/henderiw/store"
)

var ReadmeFileMatch = []string{"README.md"}
var IgnoreFileMatch = []string{".kformignore"}
var PkgFileMatch = []string{"KformFile.yaml"}
var MarkdownMatch = []string{"*.md"}
var YAMLMatch = []string{"*.yaml", "*.yml"}
var JSONMatch = []string{"*.json"}
var MatchAll = []string{"*"}

func isYamlMatch(matches []string) bool {
	if len(matches) != 2 {
		return false
	}
	for i, match := range matches {
		if match != YAMLMatch[i] {
			return false
		}
	}
	return true
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

/*
type Pipeline[T1 any] struct {
	Inputs     []Reader[T1]  `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Processors []Process[T1] `json:"processors,omitempty" yaml:"processors,omitempty"`
	Outputs    []Writer[T1]  `json:"outputs,omitempty" yaml:"outputs,omitempty"`
}

func (r Pipeline[T1]) Execute(ctx context.Context) error {
	data := memory.NewStore[T1]()
	var err error
	// read from the inputs
	for _, i := range r.Inputs {
		newData, err = i.Read(ctx)
		if err != nil {
			return err
		}
		// copy the data

	}
	//data.Print()
	for _, p := range r.Processors {
		data, err = p.Process(ctx)
		if err != nil {
			return err
		}
	}
	//data.Print()
	// write to the outputs
	for _, o := range r.Outputs {
		if err := o.Write(ctx); err != nil {
			return err
		}
	}
	return nil
}
*/
