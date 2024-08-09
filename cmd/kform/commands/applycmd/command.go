package applycmd

import (
	"context"
	"path/filepath"

	"github.com/kform-dev/kform/pkg/exec/kform/runner"
	"github.com/kform-dev/kform/pkg/fsys"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/cmd/util"
	//docs "github.com/kform-dev/kform/internal/docs/generated/applydocs"
)

func NewCommand(ctx context.Context, factory util.Factory, ioStreams genericclioptions.IOStreams) *cobra.Command {
	return NewRunner(ctx, factory, ioStreams).Command
}

// NewRunner returns a command runner.
func NewRunner(ctx context.Context, factory util.Factory, ioStreams genericclioptions.IOStreams) *Runner {
	r := &Runner{
		Factory: factory,
	}
	cmd := &cobra.Command{
		Use:  "apply (DIRECTORY | STDIN) [flags]",
		Args: cobra.ExactArgs(1),
		//Short:   docs.ApplyShort,
		//Long:    docs.ApplyShort + "\n" + docs.ApplyLong,
		//Example: docs.ApplyExamples,
		RunE: r.runE,
	}

	r.Command = cmd

	r.Command.Flags().BoolVar(&r.AutoApprove, "auto-approve", false, "skip interactive approval of plan before applying")
	r.Command.Flags().BoolVar(&r.DryRun, "dry-run", false, "executes a speculative execution plan, without applying the resources")
	r.Command.Flags().StringVarP(&r.Input, "in", "i", "", "a file or directory of KRM resource(s) that act as input rendering the package")
	r.Command.Flags().StringVarP(&r.Output, "out", "o", "", "a file or directory where the result is stored, a filename creates a single yaml doc; a dir creates seperated yaml files")
	r.Command.Flags().StringVar(&r.InventoryID, "inventory-id", "", "iventory-id to identify the applied resources, use valid semantics")

	return r
}

type Runner struct {
	Command     *cobra.Command
	Factory     util.Factory
	AutoApprove bool
	DryRun      bool
	Input       string
	Output      string
	InventoryID string
}

func (r *Runner) runE(c *cobra.Command, args []string) error {
	ctx := c.Context()
	//log := log.FromContext(ctx)

	path, err := fsys.NormalizeDir(args[0])
	if err != nil {
		return err
	}

	kfrunner := runner.NewKformRunner(&runner.Config{
		Factory:     r.Factory,
		PackageName: filepath.Base(path),
		Input:       r.Input,
		Output:      r.Output,
		Path:        path,
		DryRun:      r.DryRun,
		AutoApprove: r.AutoApprove,
		InventoryID: r.InventoryID,
	})

	return kfrunner.Run(ctx)
}
