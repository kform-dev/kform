package runner

import (
	"context"

	"github.com/henderiw-nephio/kform/kform-plugin/plugin"
	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/syntax/parser"
	"github.com/kform-dev/kform/pkg/syntax/types"
)

type Runner interface {
	Run(ctx context.Context) error
}

type Config struct {
	Input     string // used for none, file or dir
	InputData store.Storer[[]byte]
	Output    string
	Path      string // path of the kform files
}

func NewKformRunner(cfg *Config) Runner {
	return &runner{
		cfg: cfg,
	}
}

type runner struct {
	cfg               *Config
	parser            *parser.KformParser
	providers         store.Storer[types.Provider]
	providerInstances store.Storer[plugin.Provider]
	outputSink        OutputSink
}

func (r *runner) Run(ctx context.Context) error {
	log := log.FromContext(ctx)
	inputVars, err := r.getInputVars(ctx)
	if err != nil {
		return err
	}

	r.outputSink, err = r.getOuputSink(ctx)
	if err != nil {
		return err
	}

	kformRecorder := recorder.New[diag.Diagnostic]()
	ctx = context.WithValue(ctx, types.CtxKeyRecorder, kformRecorder)

	// syntax check config -> build the dag
	log.Debug("parsing packages")
	r.parser, err = parser.NewKformParser(ctx, r.cfg.Path)
	if err != nil {
		return err
	}

	r.parser.Parse(ctx)
	if kformRecorder.Get().HasError() {
		kformRecorder.Print()
		log.Error("failed parsing packages", "error", kformRecorder.Get().Error())
		return kformRecorder.Get().Error()
	}
	kformRecorder.Print()

	// initialize providers which hold the identities of the raw providers
	// that reference the exec/initialization to startup the binaries
	r.providers, err = r.parser.InitProviders(ctx)
	if err != nil {
		log.Error("failed initializing providers", "error", err)
		return err
	}
	// Based on the used provider configs return the providerInstances
	// this is an empty list which will be initialized during the run
	r.providerInstances, err = r.parser.GetEmptyProviderInstances(ctx)
	if err != nil {
		log.Error("failed initializing provider Instances", "error", err)
		return err
	}

	rootPackage, err := r.parser.GetRootPackage(ctx)
	if err != nil {
		return err
	}

	// run the provider DAG
	if err := r.RunProviderDAG(ctx, rootPackage, inputVars); err != nil {
		return err
	}

	defer func() {
		r.providerInstances.List(ctx, func(ctx context.Context, key store.Key, provider plugin.Provider) {
			if provider != nil {
				provider.Close(ctx)
				log.Debug("closing provider", "nsn", key.Name)
			}
			log.Debug("closing provider nil", "nsn", key.Name)
		})
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errCh := make(chan error, 1)
	dataStore := &data.DataStore{Storer: memory.NewStore[*data.BlockData]()}
	// run go routine
	go r.RunKformDAG(ctx, errCh, rootPackage, inputVars, dataStore)
	// wait for kform dag to finish
	err = <-errCh
	if err != nil {
		log.Error("exec failed", "err", err)
	}

	resources, err := r.getResources(ctx, dataStore)
	if err != nil {
		return err
	}

	return r.outputResources(ctx, resources)

}
