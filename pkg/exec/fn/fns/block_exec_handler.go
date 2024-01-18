package fns

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/henderiw/logger/log"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/recorder"
	"github.com/kform-dev/kform/pkg/recorder/diag"
	"github.com/kform-dev/kform/pkg/render/celrender"
	"github.com/kform-dev/kform/pkg/syntax/types"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func NewExecHandler(ctx context.Context, cfg *Config) *ExecHandler {
	return &ExecHandler{
		RootPackageName: cfg.RootPackageName,
		PackageName:     cfg.PackageName,
		BlockName:       cfg.BlockName,
		DataStore:       cfg.DataStore,
		Recorder:        cfg.Recorder,
		fnsMap: NewMap(ctx, &Config{
			Provider:          cfg.Provider,
			RootPackageName:   cfg.RootPackageName,
			DataStore:         cfg.DataStore,
			Recorder:          cfg.Recorder,
			ProviderInstances: cfg.ProviderInstances,
			ProviderInventory: cfg.ProviderInventory,
		}),
	}
}

type ExecHandler struct {
	RootPackageName string
	PackageName     string
	BlockName       string
	DataStore       *data.DataStore
	Recorder        recorder.Recorder[diag.Diagnostic]
	fnsMap          Map
}

// PostRun records the overall result of the package execHandler
func (r *ExecHandler) PostRun(ctx context.Context, start, stop time.Time, success bool) {
	recordCtx := fmt.Sprintf("total run rootPackageName/PackageName=%s/%s", r.PackageName, r.PackageName)
	recorder := r.Recorder
	if success {
		recorder.Record(diag.Success(recordCtx, start))
	} else {
		recorder.Record(diag.FromErrWithTimeContext(recordCtx, start, fmt.Errorf("failed module execution")))
	}
}

func (r *ExecHandler) BlockRun(ctx context.Context, vertexName string, vctx *types.VertexContext) bool {
	log := log.FromContext(ctx).With("vertexContext", vctx.String())
	log.Debug("run block start...")
	recorder := r.Recorder
	start := time.Now()
	success := true
	if err := r.runInstances(ctx, vctx); err != nil {
		recorder.Record(diag.FromErrWithTimeContext(vctx.String(), start, fmt.Errorf("failed block total run err: %s", err.Error())))
		success = false
	} else {
		recorder.Record(diag.Success(vctx.String(), start, "block total run"))
	}
	log.Debug("run block finished...", "success", success)
	return success
}

func (r *ExecHandler) runInstances(ctx context.Context, vctx *types.VertexContext) error {
	recorder := r.Recorder
	isForEach, items, err := r.getLoopItems(ctx, vctx.Attributes)
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(ctx)
	for idx, item := range items.List() {
		localVars := map[string]any{}
		item := item
		localVars[kformv1alpha1.LoopKeyItemsTotal] = items.Len()
		localVars[kformv1alpha1.LoopKeyItemsIndex] = idx
		if isForEach {
			localVars[kformv1alpha1.LoopKeyForEachKey] = item.key
			localVars[kformv1alpha1.LoopKeyForEachVal] = item.val
		} else {
			// we treat a singleton in the same way as count -> count.index will not be used based on our syntax checks
			localVars[kformv1alpha1.LoopKeyCountIndex] = item.key
		}
		g.Go(func() error {
			start := time.Now()
			// lookup the blockType in the map and run the block instance
			if err := r.fnsMap.Run(ctx, vctx, localVars); err != nil {
				recorder.Record(diag.FromErrWithTimeContext(vctx.String(), start, fmt.Errorf("failed running block instance: %s", err.Error())))
				return err
			}
			recorder.Record(diag.Success(vctx.String(), start, "block instance run"))
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return nil
			}
		})
	}
	return g.Wait()
}

type item struct {
	key any
	val any
}

func (r *ExecHandler) getLoopItems(ctx context.Context, attr *kformv1alpha1.Attributes) (bool, *items, error) {
	log := log.FromContext(ctx)
	log.Debug("getLoopItems", "attr", attr)
	renderer := celrender.New(r.DataStore, map[string]any{})
	isForEach := false
	items := initItems(1)
	// forEach and count cannot be used together
	if attr != nil {
		if attr.ForEach != "" {
			isForEach = true
			v, err := renderer.Render(ctx, attr.ForEach)
			if err != nil {
				if strings.Contains(err.Error(), "no such key") || strings.Contains(err.Error(), "not found") {
					v = nil
				} else {
					return isForEach, items, errors.Wrap(err, "render loop forEach failed")
				}
			}
			log.Debug("getLoopItems forEach render output", "value type", reflect.TypeOf(v), "value", v)
			switch v := v.(type) {
			case []any:
				// in a list we return key = int, val = any
				for k, v := range v {
					log.Debug("getLoopItems forEach insert item", "k", k, "v", v)
					items.Add(k, item{key: k, val: v})
				}
			case map[any]any:
				// in a list we return key = any, val = any
				idx := 0
				for k, v := range v {
					items.Add(idx, item{key: k, val: v})
					idx++
				}
			default:
				// in a regular value we return key = int, val = any
				items.Add(0, item{key: 0, val: v})
			}
			return isForEach, items, nil
		}
		if attr.Count != "" {
			v, err := renderer.Render(ctx, attr.Count)
			if err != nil {
				if strings.Contains(err.Error(), "no such key") || strings.Contains(err.Error(), "not found") {
					v = int64(0)
				} else {
					return isForEach, items, errors.Wrap(err, "render count failed")
				}
			}
			switch v := v.(type) {
			case string:
				c, err := strconv.Atoi(v)
				if err != nil {
					return isForEach, items, fmt.Errorf("render count returned a string that cannot be converted to an int, got: %s", v)
				}
				items = initItems(c)
				return isForEach, items, nil
			case int64:
				items = initItems(int(v))
				return isForEach, items, nil
			default:
				return isForEach, items, errors.Errorf("render count return an unsupported type; support [int64, string], got: %s", reflect.TypeOf(v))
			}

		}
	}
	items = initItems(1)
	return isForEach, items, nil
}

func initItems(i int) *items {
	items := &items{
		items: map[any]item{},
	}
	for idx := 0; idx < i; idx++ {
		items.Add(idx, item{key: idx, val: idx})

	}
	return items
}

type items struct {
	m     sync.RWMutex
	items map[any]item
}

func (r *items) Add(k any, v item) {
	r.m.Lock()
	defer r.m.Unlock()
	r.items[k] = v
}

func (r *items) List() map[any]item {
	r.m.RLock()
	defer r.m.RUnlock()
	x := map[any]item{}
	for k, v := range r.items {
		x[k] = v
	}
	return x
}

func (r *items) Len() int {
	r.m.RLock()
	defer r.m.RUnlock()
	return len(r.items)
}
