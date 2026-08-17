// -------------------------------------------------------------------------------
// Flight Fetcher - Server Entrypoint
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Parses flags, installs signal handling, and hands control to
// internal/cli/serve. Everything substantive lives there: this file exists to
// own the two things a library must not do, reading os.Args and calling
// os.Exit, so the daemon itself can return errors and be tested.
// -------------------------------------------------------------------------------

// Package main is the flight-fetcher binary entry point. It is a thin shell
// over flag parsing and signal handling; the daemon is implemented in
// internal/cli/serve.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/afreidah/flight-fetcher/internal/cli/serve"
)

// Version is set at build time via -ldflags -X main.Version. It stays in
// package main so the Makefile, the Dockerfile, and goreleaser keep using the
// same symbol, and is passed down rather than read from a package variable in
// the daemon.
var Version = "dev"

// main parses flags, derives a signal-cancelled context, and exits non-zero if
// the daemon returns an error.
func main() {
	configPath := flag.String("config", "config.hcl", "path to config file")
	logLevel := flag.String("log-level", "info", "log level (debug, info, warn, error)")
	flag.Parse()

	ctx, stop := serve.SignalContext(context.Background())
	defer stop()

	if err := serve.Run(ctx, serve.Options{
		ConfigPath: *configPath,
		LogLevel:   *logLevel,
		Version:    Version,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "flight-fetcher: %v\n", err)
		os.Exit(1)
	}
}
