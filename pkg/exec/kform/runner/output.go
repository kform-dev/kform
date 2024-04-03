package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/henderiw/logger/log"
	"github.com/kform-dev/kform/pkg/pkgio"
)

func (r *runner) getOuputSink(ctx context.Context) (pkgio.OutputSink, error) {
	log := log.FromContext(ctx)
	log.Debug("getOuputSink")
	output := pkgio.OutputSink_StdOut
	if r.cfg.OutputData != nil { // if memory outpur is specified it gets priority
		return pkgio.OutputSink_Memory, nil
	}
	if r.cfg.Output != "" {
		//
		fsi, err := os.Stat(r.cfg.Output)
		if err != nil {
			fsi, err := os.Stat(filepath.Dir(r.cfg.Output))
			if err != nil {
				return pkgio.OutputSink_None, fmt.Errorf("cannot init kform, output path does not exist: %s", r.cfg.Output)
			}
			if fsi.IsDir() {
				output = pkgio.OutputSink_File
			} else {
				return pkgio.OutputSink_None, fmt.Errorf("cannot init kform, output path does not exist: %s", r.cfg.Output)
			}
		} else {
			if fsi.IsDir() {
				output = pkgio.OutputSink_Dir
			}
			if fsi.Mode().IsRegular() {
				output = pkgio.OutputSink_File
			}
		}
	}
	return output, nil
}
