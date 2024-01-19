//go:generate $GOBIN/mdtogo ./../../../site/reference/cli/init ./../../../internal/docs/generated/initdocs --license=none --recursive=true --strategy=cmdDocs
//go:generate $GOBIN/mdtogo ./../../../site/reference/cli/apply ./../../../internal/docs/generated/applydocs --license=none --recursive=true --strategy=cmdDocs
//go:generate $GOBIN/mdtogo ./../../../site/reference/cli/pkg ./../../../internal/docs/generated/pkgdocs --license=none --recursive=true --strategy=cmdDocs
//go:generate $GOBIN/mdtogo ./../../../site/reference/cli/README.md ./../../../internal/docs/generated/overview --license=none --strategy=cmdDocs

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/henderiw/logger/log"
	"github.com/kform-dev/kform/cmd/kform/commands"
)

const (
	defaultConfigFileSubDir = "kform"
	defaultConfigFileName   = "kform.yaml"
)

func main() {
	os.Exit(runMain())
}

// runMain does the initial setup to setup logging
func runMain() int {
	// init logging
	l := log.NewLogger(&log.HandlerOptions{Name: "kform-logger", AddSource: false})
	slog.SetDefault(l)

	// init context
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx = log.IntoContext(ctx, l)

	// init cmd context
	cmd := commands.GetMain(ctx)

	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "%s \n", err.Error())
		cancel()
		return 1
	}
	return 0
}
