package commands

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/kform-dev/kform/cmd/kform/commands/applycmd"
	"github.com/kform-dev/kform/cmd/kform/globals"
	"github.com/kform-dev/kform/pkg/fsys"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	defaultConfigFileSubDir  = "kform"
	defaultConfigFileName    = "kform"
	defaultConfigFileNameExt = "yaml"
	defaultConfigEnvPrefix   = "KFORM"
)

var (
	configFile string
	debug      bool
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
			// initialize viper
			// ensure the viper config directory exists
			cobra.CheckErr(fsys.EnsureDir(ctx, xdg.ConfigHome, defaultConfigFileSubDir))
			// initialize viper settings
			initConfig()
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

	defaultConfigFile := fmt.Sprintf("%s/%s/%s.%s", xdg.ConfigHome, defaultConfigFileSubDir, defaultConfigFileName, defaultConfigFileNameExt)
	cmd.PersistentFlags().StringVarP(&configFile, "config", "c", defaultConfigFile, "config file to store config information for kform")
	cmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "debug")
	cmd.AddCommand(applycmd.NewCommand(ctx, version))
	return cmd
}

type Runner struct {
	Command *cobra.Command
	//Ctx     context.Context
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if configFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(configFile)
	} else {

		viper.AddConfigPath(filepath.Join(xdg.ConfigHome, defaultConfigFileName, defaultConfigFileNameExt))
		viper.SetConfigType(defaultConfigFileNameExt)
		viper.SetConfigName(defaultConfigFileSubDir)

		_ = viper.SafeWriteConfig()
	}

	//viper.Set("kubecontext", kubecontext)
	//viper.Set("kubeconfig", kubeconfig)

	viper.SetEnvPrefix(defaultConfigEnvPrefix)
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		_ = 1
	}
}
