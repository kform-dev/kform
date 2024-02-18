package applycmd

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/exec/fn/fns"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	//docs "github.com/kform-dev/kform/internal/docs/generated/applydocs"
	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	"github.com/kform-dev/kform/pkg/pkgio"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/syntax/loader"
	"github.com/kform-dev/kform/pkg/syntax/parser"
	"github.com/kform-dev/kform/pkg/syntax/parser/pkgparser"
	"github.com/kform-dev/kform/pkg/syntax/types"
)

// NewRunner returns a command runner.
func NewRunner(ctx context.Context, version string) *Runner {
	r := &Runner{}
	cmd := &cobra.Command{
		Use:  "apply [flags]",
		Args: cobra.ExactArgs(1),
		//Short:   docs.ApplyShort,
		//Long:    docs.ApplyShort + "\n" + docs.ApplyLong,
		//Example: docs.ApplyExamples,
		RunE: r.runE,
	}

	r.Command = cmd

	r.Command.Flags().BoolVar(&r.AutoApprove, "auto-approve", false, "skip interactive approval of plan before applying")
	r.Command.Flags().StringVarP(&r.Input, "in", "i", "", "a file or directory of KRM resource(s) that act as input rendering the package")
	r.Command.Flags().StringVarP(&r.Output, "out", "o", "", "a file or directory where the result is stored, a filename creates a single yaml doc; a dir creates seperated yaml files")

	return r
}

func NewCommand(ctx context.Context, version string) *cobra.Command {
	return NewRunner(ctx, version).Command
}

type Runner struct {
	Command     *cobra.Command
	rootPath    string
	AutoApprove bool
	Input       string
	Output      string
}

func (r *Runner) runE(c *cobra.Command, args []string) error {
	ctx := c.Context()
	log := log.FromContext(ctx)

	r.rootPath = args[0]
	// check if the root path exists
	fsi, err := os.Stat(r.rootPath)
	if err != nil {
		return fmt.Errorf("cannot init kform, path does not exist: %s", r.rootPath)
	}
	if !fsi.IsDir() {
		return fmt.Errorf("cannot init kform, path is not a directory: %s", r.rootPath)
	}

	outFile := false
	outDir := false
	if r.Output != "" {
		//
		fsi, err = os.Stat(r.Output)
		if err != nil {
			fsi, err := os.Stat(filepath.Dir(r.Output))
			if err != nil {
				return fmt.Errorf("cannot init kform, output path does not exist: %s", r.Output)
			}
			if fsi.IsDir() {
				outFile = true
				outDir = false
			} else {
				return fmt.Errorf("cannot init kform, output path does not exist: %s", r.Output)
			}
		} else {
			if fsi.IsDir() {
				outDir = true
			}
			if fsi.Mode().IsRegular() {
				outFile = true
				outDir = false
			}
		}

	}
	// captures dynamic input
	inputVars := map[string]any{}
	if r.Input != "" {
		// gathers the dynamic input as if it were a package
		fsi, err := os.Stat(r.Input)
		if err != nil {
			return fmt.Errorf("cannot init kform, input path does not exist: %s", r.Input)
		}

		// recorder need to be set before
		inputRecorder := recorder.New[diag.Diagnostic]()
		ctx = context.WithValue(ctx, types.CtxKeyRecorder, inputRecorder)

		var kf *kformv1alpha1.KformFile
		var kforms map[string]*fn.KubeObject
		if !fsi.IsDir() {
			b, err := os.ReadFile(r.Input)
			if err != nil {
				return err
			}
			ko, err := fn.ParseKubeObject(b)
			if err != nil {
				return err
			}
			ko.SetAnnotation(kformv1alpha1.KformAnnotationKey_BLOCK_TYPE, kformv1alpha1.BlockTYPE_INPUT.String())
			kforms = map[string]*fn.KubeObject{
				r.Input: ko,
			}
		} else {
			kf, kforms, err = loader.GetKforms(ctx, r.Input, true)
			if err != nil {
				return err
			}
		}

		inputParser, err := pkgparser.New(ctx, r.Input)
		if err != nil {
			return err
		}
		inputPkg := inputParser.Parse(ctx, kf, kforms)
		if inputRecorder.Get().HasError() {
			inputRecorder.Print()
			log.Error("failed parsing input", "error", inputRecorder.Get().Error())
			return inputRecorder.Get().Error()
		}
		inputRecorder.Print()
		// initialize the input vars
		inputVars = inputPkg.GetBlockdata(ctx)
	}

	// initialize the recorder
	kformRecorder := recorder.New[diag.Diagnostic]()
	ctx = context.WithValue(ctx, types.CtxKeyRecorder, kformRecorder)

	// syntax check config -> build the dag
	log.Info("parsing packages")
	p, err := parser.NewKformParser(ctx, r.rootPath)
	if err != nil {
		return err
	}
	p.Parse(ctx)
	if kformRecorder.Get().HasError() {
		kformRecorder.Print()
		log.Error("failed parsing packages", "error", kformRecorder.Get().Error())
		return kformRecorder.Get().Error()
	}
	kformRecorder.Print()

	/*
		providerInventory, err := p.InitProviderInventory(ctx)
		if err != nil {
			log.Error("failed initializing provider inventory", "error", err)
			return err
		}

		providerInstances := p.InitProviderInstances(ctx)
	*/

	/*
		for nsn := range providerInstances.List() {
			fmt.Println("provider instance", nsn.Name)
		}
	*/

	rootPackage, err := p.GetRootPackage(ctx)
	if err != nil {
		return err
	}
	//rootPackage.DAG.Print("root")
	//rootPackage.ProviderDAG.Print("root")

	/*
		for blockName, block := range types.ListBlocks(ctx, rootPackage.Blocks) {
			fmt.Println("blockData", blockName, block.GetData())
		}
	*/

	/*
		runRecorder := recorder.New[diag.Diagnostic]()
		resultStore :=

			// run the provider DAG
		log.Info("create provider runner")

		rmfn := fns.NewPackageFn(&fns.Config{
			Provider:          true,
			RootModuleName:    rm.NSN.Name,
			Vars:              varsCache,
			Recorder:          runrecorder,
			ProviderInstances: providerInstances,
			ProviderInventory: providerInventory,
		})
		log.Info("executing provider runner DAG")
		if err := rmfn.Run(ctx, &types.VertexContext{
			FileName:     filepath.Join(r.rootPath, pkgio.PkgFileMatch[0]),
			ModuleName:   rm.NSN.Name,
			BlockType:    types.BlockTypeModule,
			BlockName:    rm.NSN.Name,
			DAG:          rm.ProviderDAG, // we supply the provider DAG here
			BlockContext: types.KformBlockContext{},
		}, map[string]any{}); err != nil {
			log.Error("failed running provider DAG", "err", err)
			return err
		}
		log.Info("success executing provider DAG")

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
	*/

	// doneCh := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		defer close(errCh)

		runRecorder := recorder.New[diag.Diagnostic]()
		dataStore := &data.DataStore{Storer: memory.NewStore[*data.BlockData]()}

		cmdPackageFn := fns.NewPackageFn(&fns.Config{
			RootPackageName: rootPackage.Name,
			DataStore:       dataStore,
			Recorder:        runRecorder,
			/*
				TODO update when adding providers
				ProviderInstances: providerInstances,
				ProviderInventory: providerInventory,
			*/
		})

		log.Info("executing package")
		if err := cmdPackageFn.Run(ctx, &types.VertexContext{
			FileName:    filepath.Join(r.rootPath, pkgio.PkgFileMatch[0]),
			PackageName: rootPackage.Name,
			BlockType:   kformv1alpha1.BlockTYPE_PACKAGE,
			BlockName:   rootPackage.Name,
			DAG:         rootPackage.DAG,
		}, inputVars); err != nil {
			log.Error("failed executing package", "err", err)
			errCh <- err
			return
		}

		log.Info("success executing package")

		resources := map[string]any{}
		dataStore.List(ctx, func(ctx context.Context, key store.Key, blockData *data.BlockData) {
			for _, dataInstances := range blockData.Data {
				for idx, dataInstance := range dataInstances {
					b, err := yaml.Marshal(dataInstance)
					if err != nil {
						errCh <- err
						return
					}
					u := &unstructured.Unstructured{}
					if err := yaml.Unmarshal(b, u); err != nil {
						errCh <- err
						return
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
						errCh <- err
						return
					}
					var x any
					if err := yaml.Unmarshal(b, &x); err != nil {
						errCh <- err
						return
					}

					resources[fmt.Sprintf("%s_%s_%s_%s_%d.yaml", apiVersion, kind, name, namespace, idx)] = x
				}
			}
		})

		if !outFile {
			for resourceName, data := range resources {
				b, err := yaml.Marshal(data)
				if err != nil {
					errCh <- err
					return
				}
				fmt.Println(path.Join(r.Output, resourceName))
				os.WriteFile(path.Join(r.Output, resourceName), b, 0644)
			}
		} else {
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

			for _, data := range priorityOrderedList {
				b, err := yaml.Marshal(data)
				if err != nil {
					errCh <- err
					return
				}
				if outDir {
					// write individual files
				} else {
					sb.WriteString("\n---\n")
					sb.WriteString(string(b))
				}
			}
			for _, key := range keys {
				data, ok := resources[key]
				if ok {
					b, err := yaml.Marshal(data)
					if err != nil {
						errCh <- err
						return
					}
					if outDir {
						// write individual files
					} else {
						sb.WriteString("\n---\n")
						sb.WriteString(string(b))
					}
				}
			}
			os.WriteFile(r.Output, []byte(sb.String()), 0644)
		}

		//runRecorder.Print()
		// auto-apply -> depends on the flag if we approve the change or not.
	}()

	err = <-errCh
	if err != nil {
		log.Error("exec failed", "err", err)
	}

	/*
		providersList := providerInstances.List()
		fmt.Println("exec Done", len(providersList))
		for nsn, provider := range providersList {
			if provider != nil {
				provider.Close(ctx)
				log.Info("closing provider", "nsn", nsn)
				continue
			}
			log.Info("closing provider nil", "nsn", nsn)
		}
	*/
	return nil
}
