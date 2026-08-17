// -------------------------------------------------------------------------------
// Flight Fetcher - Server Entrypoint
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Parses flags, installs signal handling, and hands control to
// internal/cli/serve. Everything substantive lives there: this file exists to
// own the two things a library must not do, reading os.Args and calling
// os.Exit.
//
// The work is split so main is a single statement and run is an ordinary
// function returning an exit code. That keeps flag parsing and error reporting
// testable without building and executing the binary.
// -------------------------------------------------------------------------------

// Package main is the flight-fetcher binary entry point. It is a thin shell
// over flag parsing and signal handling; the daemon is implemented in
// internal/cli/serve.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/afreidah/flight-fetcher/internal/cli/serve"
)

// Version is set at build time via -ldflags -X main.Version. It stays in
// package main so the Makefile, the Dockerfile, and goreleaser keep using the
// same symbol, and is passed down rather than read from a package variable in
// the daemon.
var Version = "dev"

// main runs the daemon and exits with its status code.
func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run parses args, derives a signal-cancelled context, and runs the daemon,
// returning the process exit code. Args and the error stream are parameters
// rather than globals so a test can drive it directly.
func run(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("flight-fetcher", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "config.hcl", "path to config file")
	logLevel := fs.String("log-level", "info", "log level (debug, info, warn, error)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ctx, stop := serve.SignalContext(context.Background())
	defer stop()

	if err := serve.Run(ctx, serve.Options{
		ConfigPath: *configPath,
		LogLevel:   *logLevel,
		Version:    Version,
	}); err != nil {
		fmt.Fprintf(stderr, "flight-fetcher: %v\n", err)
		return 1
	}
	return 0
}
