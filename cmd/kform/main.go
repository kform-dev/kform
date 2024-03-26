package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/henderiw/logger/log"
	"github.com/kform-dev/kform/cmd/kform/commands"
	"github.com/kform-dev/kform/cmd/kform/globals"
)

func main() {
	os.Exit(runMain())
}

// runMain does the initial setup to setup logging
func runMain() int {
	// init logging
	l := log.NewLogger(&log.HandlerOptions{Name: "kform-logger", AddSource: false, MinLevel: globals.LogLevel})
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
