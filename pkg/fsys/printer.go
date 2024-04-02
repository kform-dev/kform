package fsys

import (
	"io"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/runtime"
)

// Printer is used to print an object.
type Printer struct{}

// Print the object inside the writer w.
func (p *Printer) Print(w io.Writer, obj runtime.Object) error {
	if obj == nil {
		return nil
	}
	data, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err

}
