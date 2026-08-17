// -------------------------------------------------------------------------------
// Flight Fetcher - Entrypoint Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Covers flag parsing and exit-code translation. run is an ordinary function
// returning a status code rather than calling os.Exit, so these drive it
// directly instead of building and executing the binary. main itself is one
// statement, os.Exit(run(...)), and is the only thing here left uncovered.
// -------------------------------------------------------------------------------

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRun_BadFlag verifies an unparseable flag exits 2, the conventional code
// for a usage error, and that the failure is written to the provided stream
// rather than the process's stderr.
func TestRun_BadFlag(t *testing.T) {
	var stderr bytes.Buffer

	if got := run([]string{"-nosuchflag"}, &stderr); got != 2 {
		t.Errorf("run() = %d, want 2 for a usage error", got)
	}
	if stderr.Len() == 0 {
		t.Error("nothing written to the error stream")
	}
}

// TestRun_StartupFailure verifies a daemon that fails to start exits 1 with
// the reason on the error stream. A missing config is the cheapest way to
// reach that path without standing up Redis or Postgres.
func TestRun_StartupFailure(t *testing.T) {
	var stderr bytes.Buffer
	missing := filepath.Join(t.TempDir(), "nope.hcl")

	if got := run([]string{"-config", missing, "-log-level", "error"}, &stderr); got != 1 {
		t.Errorf("run() = %d, want 1 for a startup failure", got)
	}
	out := stderr.String()
	if !strings.Contains(out, "flight-fetcher:") {
		t.Errorf("stderr = %q, want it prefixed with the binary name", out)
	}
	if !strings.Contains(out, "loading config") {
		t.Errorf("stderr = %q, want it to name the failing step", out)
	}
}

// TestRun_InvalidLogLevel verifies the log-level flag is validated by the
// daemon and reported the same way, so a typo exits non-zero rather than
// silently defaulting.
func TestRun_InvalidLogLevel(t *testing.T) {
	var stderr bytes.Buffer
	cfg := filepath.Join(t.TempDir(), "config.hcl")
	if err := os.WriteFile(cfg, []byte("location {\n lat = 1\n lon = 1\n radius_km = 1\n}\n"), 0o600); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	if got := run([]string{"-config", cfg, "-log-level", "chatty"}, &stderr); got != 1 {
		t.Errorf("run() = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "invalid log level") {
		t.Errorf("stderr = %q, want it to name the invalid log level", stderr.String())
	}
}

// TestRun_HelpExitsCleanly verifies -h is a usage request, not a crash. flag
// reports ErrHelp through the same Parse error path as a bad flag, so this
// pins that it still exits 2 and prints usage rather than starting a daemon.
func TestRun_HelpExitsCleanly(t *testing.T) {
	var stderr bytes.Buffer

	if got := run([]string{"-h"}, &stderr); got != 2 {
		t.Errorf("run() = %d, want 2", got)
	}
	usage := stderr.String()
	for _, flagName := range []string{"-config", "-log-level"} {
		if !strings.Contains(usage, flagName) {
			t.Errorf("usage output missing %s:\n%s", flagName, usage)
		}
	}
}
