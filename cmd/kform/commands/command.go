package commands

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/henderiw/logger/log"
	"github.com/kform-dev/kform/cmd/kform/commands/applycmd"
	"github.com/kform-dev/kform/cmd/kform/commands/destroycmd"
	"github.com/kform-dev/kform/cmd/kform/commands/initcmd"
	"github.com/kform-dev/kform/cmd/kform/commands/plancmd"
	"github.com/kform-dev/kform/cmd/kform/globals"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/cli-utils/pkg/flowcontrol"
)

var (
	debug bool
)

func GetMain(ctx context.Context) *cobra.Command {
	//showVersion := false
	cmd := &cobra.Command{
		Use:          "kform",
		Short:        "kform is a KRM orchestration tool",
		Long:         "kform is a KRM orchestration tool",
		SilenceUsage: true,
		// We handle all errors in main after return from cobra so we can
		// adjust the error message coming from libraries
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if debug {
				globals.LogLevel.Set(slog.LevelDebug)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := cmd.Flags().GetBool("help")
			if err != nil {
				return err
			}
			if h {
				return cmd.Help()
			}
			return cmd.Usage()
		},
	}

	cmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "debug")
	// kubernetes flags
	flags := cmd.PersistentFlags()
	kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
	kubeConfigFlags.AddFlags(flags)
	matchVersionKubeConfigFlags := util.NewMatchVersionFlags(kubeConfigFlags)
	matchVersionKubeConfigFlags.AddFlags(flags)
	flags.AddGoFlagSet(flag.CommandLine)
	f := util.NewFactory(matchVersionKubeConfigFlags)

	// Update ConfigFlags before subcommands run that talk to the server.
	preRunE := newConfigFilerPreRunE(ctx, f, kubeConfigFlags)

	ioStreams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	subCmds := map[string]*cobra.Command{
		"init":    initcmd.NewCommand(ctx, ioStreams),
		"apply":   applycmd.NewCommand(ctx, f, ioStreams),
		"destroy": destroycmd.NewCommand(ctx, f, ioStreams),
		"plan":    plancmd.NewCommand(ctx, f, ioStreams),
	}

	for _, subCmd := range subCmds {
		subCmd.PreRunE = preRunE
		//updateHelp(names, subCmd)
		cmd.AddCommand(subCmd)
	}
	return cmd
}

type Runner struct {
	Command *cobra.Command
	//Ctx     context.Context
}

// newConfigFilerPreRunE returns a cobra command PreRunE function that
// performs a lookup to determine if server-side throttling is enabled. If so,
// client-side throttling is disabled in the ConfigFlags.
func newConfigFilerPreRunE(ctx context.Context, f util.Factory, configFlags *genericclioptions.ConfigFlags) func(*cobra.Command, []string) error {
	log := log.FromContext(ctx)
	return func(_ *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		restConfig, err := f.ToRESTConfig()
		if err != nil {
			return err
		}
		enabled, err := flowcontrol.IsEnabled(ctx, restConfig)
		if err != nil {
			return fmt.Errorf("checking server-side throttling enablement: %w", err)
		}
		if enabled {
			// Disable client-side throttling.
			log.Debug("client-side throttling disabled")
			// WrapConfigFn will affect future Factory.ToRESTConfig() calls.
			configFlags.WrapConfigFn = func(cfg *rest.Config) *rest.Config {
				cfg.QPS = -1
				cfg.Burst = -1
				return cfg
			}
		}
		return nil
	}
}
