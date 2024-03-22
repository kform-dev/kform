package applycmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kform-dev/kform/pkg/kform/runner"
	"github.com/spf13/cobra"
	//docs "github.com/kform-dev/kform/internal/docs/generated/applydocs"
)

func NewCommand(ctx context.Context, version string) *cobra.Command {
	return NewRunner(ctx, version).Command
}

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

type Runner struct {
	Command     *cobra.Command
	rootPath    string
	AutoApprove bool
	Input       string
	Output      string
}

func (r *Runner) runE(c *cobra.Command, args []string) error {
	ctx := c.Context()
	//log := log.FromContext(ctx)

	r.rootPath = args[0]
	// check if the root path exists
	fsi, err := os.Stat(r.rootPath)
	if err != nil {
		return fmt.Errorf("cannot init kform, path does not exist: %s", r.rootPath)
	}
	if !fsi.IsDir() {
		return fmt.Errorf("cannot init kform, path is not a directory: %s", r.rootPath)
	}

	kfrunner := runner.NewKformRunner(&runner.Config{
		PackageName:  filepath.Base(r.rootPath),
		Input:        r.Input,
		Output:       r.Output,
		ResourcePath: r.rootPath,
	})

	return kfrunner.Run(ctx)
}
