/*
Copyright 2024 Nokia.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package commands

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var (
	version = "0.0.0"
	commit  = "none"
	date    = "unknown"
)

const (
	repoUrl     = "https://github.com/kform-dev/kform"
	downloadURL = "https://github.com/kform-dev/kform/raw/main/install.sh"
)

func GetVersionCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "show kform version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("    version: %s\n", version)
			fmt.Printf("     commit: %s\n", commit)
			fmt.Printf("       date: %s\n", date)
			fmt.Printf("     source: %s\n", repoUrl)
			fmt.Printf(" rel. notes: https://docs.kform.dev/rn/%s\n", version)

			return nil
		},
	}

	cmd.AddCommand(GetVersionUpgradeCommand(ctx))
	return cmd
}

func GetVersionUpgradeCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "upgrade kform to latest available version",
		//PreRunE: sudoCheck,
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := os.CreateTemp("", "kform")
			defer os.Remove(f.Name())
			if err != nil {
				return fmt.Errorf("failed to create temp file: %w", err)
			}
			_ = downloadFile(downloadURL, f)

			c := exec.Command("bash", f.Name())
			// pass the environment variables to the upgrade script
			// so that GITHUB_TOKEN is available
			c.Env = os.Environ()

			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			err = c.Run()
			if err != nil {
				return fmt.Errorf("upgrade failed: %w", err)
			}

			return nil
		},
	}

	cmd.AddCommand()
	return cmd
}

// downloadFile will download a file from a URL and write its content to a file.
func downloadFile(url string, file *os.File) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	return nil
}
