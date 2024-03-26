package initcmd

import (
	"context"

	"github.com/kform-dev/kform/pkg/inventory/config"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	//docs "github.com/kform-dev/kform/internal/docs/generated/applydocs"
)

func NewCommand(ctx context.Context, ioStreams genericclioptions.IOStreams) *cobra.Command {
	return NewRunner(ctx, ioStreams).Command
}

// NewRunner returns a command runner.
func NewRunner(ctx context.Context, ioStreams genericclioptions.IOStreams) *Runner {
	r := &Runner{
		InvConfig: config.New(ioStreams),
	}
	cmd := &cobra.Command{
		Use:  "init DIRECTORY [flags]",
		Args: cobra.ExactArgs(1),
		//Short:   docs.InitShort,
		//Long:    docs.InitShort + "\n" + docs.InitLong,
		//Example: docs.InitExamples,
		RunE: r.runE,
	}

	r.Command = cmd
	r.Command.Flags().StringVarP(&r.InvConfig.InventoryID, "inventory-id", "i", "", "iventory-id listing the applied resources, use valid semantics")
	return r
}

type Runner struct {
	Command   *cobra.Command
	InvConfig *config.Config
}

func (r *Runner) runE(c *cobra.Command, args []string) error {
	ctx := c.Context()
	err := r.InvConfig.Complete(ctx, args[0])
	if err != nil {
		return err
	}
	return r.InvConfig.Run(ctx)
}
