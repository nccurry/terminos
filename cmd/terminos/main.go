// SPDX-License-Identifier: AGPL-3.0-or-later

// Package main wires the terminos command-line entrypoint.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/nccurry/terminos/internal/apperror"
	terminoscli "github.com/nccurry/terminos/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	app := terminoscli.NewApp(os.Stdout, os.Stderr)
	err := app.Run(ctx, os.Args)
	if err == nil {
		stop()
		return
	}

	if errors.Is(err, context.Canceled) {
		stop()
		os.Exit(apperror.ExitInterrupted)
	}

	fmt.Fprintln(os.Stderr, err)
	stop()
	os.Exit(apperror.ExitCode(err))
}
