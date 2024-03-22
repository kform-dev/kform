package pkgio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/henderiw/store"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/data"
	"k8s.io/apimachinery/pkg/types"
)

type KformWriter struct {
	//Writer io.Writer
	Type OutputSink
	// Could be a file or directory
	Path string
}

func (r *KformWriter) Write(ctx context.Context, datastore store.Storer[data.BlockData]) error {

	//
	prioritizedKeys := []store.Key{}
	keys := []store.Key{}
	files := map[string][]store.Key{}
	datastore.List(ctx, func(ctx context.Context, k store.Key, bd data.BlockData) {
		if len(bd) > 0 { // only the first entry is relevant -> all
			rn := bd[0]
			rnAnnotations := rn.GetAnnotations()
			path := rnAnnotations[kformv1alpha1.KformAnnotationKey_PATH]
			fileIdx := rnAnnotations[kformv1alpha1.KformAnnotationKey_INDEX]

			switch r.Type {
			case OutputSink_Dir:
				keys = append(keys, k)
			case OutputSink_File, OutputSink_StdOut: // prioritize namespace; all the rest is mapped after to a single file
				if rn.GetKind() == "Namespace" {
					prioritizedKeys = append(prioritizedKeys, k)
				} else {
					keys = append(keys, k)
				}
			case OutputSink_FileRetain, OutputSink_Memory:
				if len(files[path]) == 0 {
					files[path] = []store.Key{}
				}

				files[path] = append(files[path], store.KeyFromNSN(types.NamespacedName{
					Namespace: fileIdx,
					Name:      k.Name,
				}))
			}
		}
	})

	sort.SliceStable(keys, func(i, j int) bool {
		return keys[i].Name < keys[j].Name
	})

	for _, keys := range files {
		sort.SliceStable(keys, func(i, j int) bool {
			return keys[i].Namespace < keys[j].Namespace
		})
	}

	switch r.Type {
	case OutputSink_Dir:
		return r.writeDir(ctx, keys, datastore)
	case OutputSink_File:
		file, err := os.Create(r.Path)
		if err != nil {
			return err
		}
		defer file.Close()
		return r.writeSingle(ctx, file, prioritizedKeys, keys, datastore)
	case OutputSink_StdOut: // nothing to do
		return r.writeSingle(ctx, os.Stdout, prioritizedKeys, keys, datastore)
	case OutputSink_FileRetain:
		return r.writeRetainFile(ctx, files, datastore)
	case OutputSink_Memory:
	}
	return nil
}

func (r *KformWriter) writeDir(ctx context.Context, keys []store.Key, datastore store.Storer[data.BlockData]) error {
	var errm error
	for _, key := range keys {
		bd, err := datastore.Get(ctx, store.ToKey(key.Name))
		if err != nil {
			errors.Join(errm, err)
			continue
		}

		for _, rn := range bd {
			rnAnnotations := rn.GetAnnotations()
			for _, a := range kformv1alpha1.KformAnnotations {
				delete(rnAnnotations, a)
			}
			rn.SetAnnotations(rnAnnotations)

			file, err := os.Create(filepath.Join(r.Path, fmt.Sprintf(
				"%s.%s.%s.%s.yaml",
				strings.ReplaceAll(rn.GetApiVersion(), "/", "_"),
				rn.GetKind(),
				rn.GetNamespace(),
				rn.GetName(),
			)))
			if err != nil {
				return err
			}
			fmt.Fprintf(file, "---\n%s\n", rn.MustString())
			file.Close()
		}
	}
	return errm
}

func (r *KformWriter) writeSingle(ctx context.Context, w io.Writer, prioritizedKeys, keys []store.Key, datastore store.Storer[data.BlockData]) error {
	var errm error
	for _, priorityKey := range prioritizedKeys {
		bd, err := datastore.Get(ctx, store.ToKey(priorityKey.Name))
		if err != nil {
			errors.Join(errm, err)
			continue
		}
		for _, rn := range bd {
			rnAnnotations := rn.GetAnnotations()
			for _, a := range kformv1alpha1.KformAnnotations {
				delete(rnAnnotations, a)
			}
			rn.SetAnnotations(rnAnnotations)
			fmt.Fprintf(w, "---\n%s\n", rn.MustString())
		}
	}
	for _, key := range keys {
		bd, err := datastore.Get(ctx, store.ToKey(key.Name))
		if err != nil {
			errors.Join(errm, err)
			continue
		}
		for _, rn := range bd {
			rnAnnotations := rn.GetAnnotations()
			for _, a := range kformv1alpha1.KformAnnotations {
				delete(rnAnnotations, a)
			}
			rn.SetAnnotations(rnAnnotations)
			fmt.Fprintf(w, "---\n%s\n", rn.MustString())
		}
	}
	return errm
}

func (r *KformWriter) writeRetainFile(ctx context.Context, files map[string][]store.Key, datastore store.Storer[data.BlockData]) error {
	var errm error
	for fileName, keys := range files {
		file, err := os.Create(filepath.Join(r.Path, fileName))
		if err != nil {
			return err
		}
		defer file.Close()

		for _, key := range keys {
			bd, err := datastore.Get(ctx, store.ToKey(key.Name))
			if err != nil {
				errors.Join(errm, err)
				continue
			}

			for _, rn := range bd {
				rnAnnotations := rn.GetAnnotations()
				for _, a := range kformv1alpha1.KformAnnotations {
					delete(rnAnnotations, a)
				}
				rn.SetAnnotations(rnAnnotations)
				fmt.Fprintf(file, "---\n%s\n", rn.MustString())
			}
		}
	}
	return errm
}
