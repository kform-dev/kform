package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/data"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type OutputSink int64

const (
	OutputSink_None OutputSink = iota
	OutputSink_File
	OutputSink_Dir
	OutputSink_StdOut
)

func (r *runner) getOuputSink(ctx context.Context) (OutputSink, error) {
	output := OutputSink_StdOut
	if r.cfg.Output != "" {
		//
		fsi, err := os.Stat(r.cfg.Output)
		if err != nil {
			fsi, err := os.Stat(filepath.Dir(r.cfg.Output))
			if err != nil {
				return OutputSink_None, fmt.Errorf("cannot init kform, output path does not exist: %s", r.cfg.Output)
			}
			if fsi.IsDir() {
				output = OutputSink_File
			} else {
				return OutputSink_None, fmt.Errorf("cannot init kform, output path does not exist: %s", r.cfg.Output)
			}
		} else {
			if fsi.IsDir() {
				output = OutputSink_Dir
			}
			if fsi.Mode().IsRegular() {
				output = OutputSink_File
			}
		}
	}
	return output, nil
}

func (r *runner) getResources(ctx context.Context, dataStore *data.DataStore) (map[string]any, error) {
	log := log.FromContext(ctx)
	resources := map[string]any{}
	var errm error
	dataStore.List(ctx, func(ctx context.Context, key store.Key, blockData *data.BlockData) {
		for _, dataInstances := range blockData.Data {
			for idx, dataInstance := range dataInstances {
				b, err := yaml.Marshal(dataInstance)
				if err != nil {
					log.Error("cannot marshal data", "error", err.Error())
					errors.Join(errm, err)
					continue
				}
				u := &unstructured.Unstructured{}
				if err := yaml.Unmarshal(b, u); err != nil {
					log.Error("cannot unmarshal data", "error", err.Error())
					errors.Join(errm, err)
					continue
				}
				apiVersion := strings.ReplaceAll(u.GetAPIVersion(), "/", "_")
				kind := u.GetKind()
				name := u.GetName()
				namespace := u.GetNamespace()

				annotations := u.GetAnnotations()
				for k := range annotations {
					for _, kformKey := range kformv1alpha1.KformAnnotations {
						if k == kformKey {
							delete(annotations, k)
							continue
						}
					}
				}
				if len(annotations) != 0 {
					u.SetAnnotations(annotations)
				} else {
					u.SetAnnotations(nil)
				}

				b, err = yaml.Marshal(u)
				if err != nil {
					log.Error("cannot marshal unstructured", "error", err.Error())
					errors.Join(errm, err)
					continue
				}
				var x any
				if err := yaml.Unmarshal(b, &x); err != nil {
					log.Error("cannot unmarshal unstructured", "error", err.Error())
					errors.Join(errm, err)
					continue
				}

				resources[fmt.Sprintf("%s_%s_%s_%s_%d.yaml", apiVersion, kind, name, namespace, idx)] = x
			}
		}
	})
	return resources, errm

}

func (r *runner) outputResources(ctx context.Context, resources map[string]any) error {
	switch r.outputSink {
	case OutputSink_Dir:
		var errm error
		for resourceName, data := range resources {
			b, err := yaml.Marshal(data)
			if err != nil {
				errors.Join(errm, err)
				continue
			}
			fmt.Println(path.Join(r.cfg.Output, resourceName))
			os.WriteFile(path.Join(r.cfg.Output, resourceName), b, 0644)
		}
		if errm != nil {
			return errm
		}
	case OutputSink_File:
		s, err := r.prepareOutputString(ctx, resources)
		if err != nil {
			return err
		}
		os.WriteFile(r.cfg.Output, []byte(s), 0644)

	case OutputSink_StdOut:
		s, err := r.prepareOutputString(ctx, resources)
		if err != nil {
			return err
		}
		fmt.Println(s)
	default:
		return fmt.Errorf("unexpected output sink: got: %d", r.outputSink)
	}
	return nil
}

func (r *runner) prepareOutputString(ctx context.Context, resources map[string]any) (string, error) {
	ordereredList := []string{
		"Namespace",
		"CustomResourceDefinition",
	}

	priorityOrderedList := []any{}
	for _, kind := range ordereredList {
		for resourceName, data := range resources {
			if d, ok := data.(map[string]any); ok {
				if d["kind"] == kind {
					priorityOrderedList = append(priorityOrderedList, data)
					delete(resources, resourceName)
				}
			}
		}
	}

	// remaining resources
	keys := []string{}
	for resourceName := range resources {
		keys = append(keys, resourceName)
	}
	sort.Strings(keys)

	var sb strings.Builder
	var errm error
	for _, data := range priorityOrderedList {
		b, err := yaml.Marshal(data)
		if err != nil {
			errors.Join(errm, err)
			continue
		}

		sb.WriteString("\n---\n")
		sb.WriteString(string(b))

	}
	for _, key := range keys {
		data, ok := resources[key]
		if ok {
			b, err := yaml.Marshal(data)
			if err != nil {
				errors.Join(errm, err)
				continue
			}

			sb.WriteString("\n---\n")
			sb.WriteString(string(b))
		}
	}
	return sb.String(), errm
}
