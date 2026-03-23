**Project:** Flight Fetcher
**Author:** Alex Freidah

---

## Table of Contents

- [Core Principles](#core-principles)
- [Comment Types and Spacing](#comment-types-and-spacing)
- [File Headers](#file-headers)
- [Go Conventions](#go-conventions)
- [Error Handling](#error-handling)
- [Logging](#logging)
- [Testing](#testing)
- [Nomad Job Structure](#nomad-job-structure)
- [Code Style](#code-style)
- [Branch Naming](#branch-naming)

---

## Core Principles

- **ASCII-only characters** - Never use Unicode em-dashes, en-dashes, or box-drawing characters
- **Dashes, not equals** - Always use `-` for dividers, never `=`
- **Box comment spacing** - ALL box comments (79-char file headers and 73-char sections) ALWAYS have a blank line after
- **Professional tone** - No personal references, no numbered lists, no casual language
- **Self-documenting** - Code explains *why*, not just *what*
- **Context propagation** - Pass `context.Context` through all function chains for cancellation and tracing

---

## Comment Types and Spacing

### File Header (79 characters)

**Format:**
```go
// -------------------------------------------------------------------------------
// Title of File or Component
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// 2-4 sentence description of the file's purpose, scope, and key functionality.
// Include architecture notes, design decisions, or important context that helps
// readers understand the overall purpose.
// -------------------------------------------------------------------------------

package mypackage
```

**Spacing Rules:**
- Blank line after title
- Blank line after metadata
- Blank line before closing divider
- **Blank line after closing divider** - always separate box from code

### Major Section Box (73 characters)

**Format:**
```go
// -------------------------------------------------------------------------
// SECTION NAME
// -------------------------------------------------------------------------

func doSomething() {
    // ...
}
```

**Spacing Rules:**
- Use ALL CAPS for section name
- **Blank line AFTER closing divider** - separates section from code
- Used for major logical divisions (e.g., PUBLIC API, INTERNALS, TYPES)

### Single-Line Comments

Standard Go comments placed directly above the code they describe:

```go
// Parse request path
bucket, key, ok := parsePath(r.URL.Path)
if !ok {
    return errInvalidPath
}
```

- **NO blank line before code** - placed directly above the block
- Use lowercase or sentence case
- Used for minor divisions or labels within functions

### Inline Comments

```go
m.usage.Record(backendName, 2, movedSize, 0) // Get + Delete, egress
```

- Use sparingly
- Explain *why*, not *what*
- Keep concise (< 50 characters)

---

## Comment Type Decision Tree

```
Is this a file header?
  YES -> Use 79-char divider, blank line AFTER

Is this a major section (types, public API, internals)?
  YES -> Use 73-char box, blank line AFTER

Is this a minor division or label within a function?
  YES -> Use a standard single-line comment, NO blank line before code

Is this explaining a specific line?
  YES -> Use inline comment
```

**Key Rule:** ALL box comments (79-char and 73-char) have a blank line after. Single-line comments have no extra spacing.

---

## File Headers

### Go Files

```go
// -------------------------------------------------------------------------------
// Package or File Name
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Description of what this file or package does. Include key types, functions,
// and any important architectural decisions or dependencies.
// -------------------------------------------------------------------------------

package main
```

**Go-Specific Rules:**
- Use `//` comments (not `/* */` blocks)
- File headers use 79-char dividers with `//`
- Major sections use 73-char dividers with `//`
- Single-line markers: `// --- description ---`
- **Full godoc compliance** - every exported AND unexported function, method, type, and constant gets a `//` doc comment placed directly above the declaration
- Doc comments start with the identifier name: `// PutObject uploads...`, `// wrapReader returns...`
- Doc comments describe behavior and purpose, not implementation details
- 1 tab indentation (Go standard)
- Import groups: stdlib, internal packages, external packages (separated by blank lines)

### HCL Files (Config, Nomad)

```hcl
# -------------------------------------------------------------------------------
# Title of File or Component
#
# Project: Flight Fetcher / Author: Alex Freidah
#
# Description of what this file configures. Include dependencies and any
# important operational considerations.
# -------------------------------------------------------------------------------
```

### Dockerfiles

```dockerfile
# -------------------------------------------------------------------------------
# Title
#
# Project: Flight Fetcher / Author: Alex Freidah
#
# Description of what this Dockerfile builds, base images, and any important
# build considerations.
# -------------------------------------------------------------------------------
```

---

## Go Conventions

### Indentation

- **1 tab** - Go standard (`gofmt` enforced)

### Imports

Group imports in three blocks separated by blank lines:

```go
import (
    "context"
    "fmt"
    "time"

    "github.com/afreidah/flight-fetcher/internal/geo"
    "github.com/afreidah/flight-fetcher/internal/store"

    "github.com/redis/go-redis/v9"
)
```

Order: stdlib, internal packages, external packages.

### Naming

- **Exported types** get standard Go doc comments placed directly above the declaration
- **Constants** grouped by concern with `const` blocks, named in `CamelCase`
- **Sentinel errors** use `Err` prefix: `ErrNotFound`, `ErrTimeout`

### Struct Organization

Group related fields with inline comments explaining non-obvious fields:

```go
type Poller struct {
    opensky  *opensky.Client
    redis    *store.RedisStore
    postgres *store.PostgresStore
    enricher *enricher.Enricher
    center   geo.Coord
    radiusKm float64
    interval time.Duration
}
```

### Concurrency Patterns

- **Context-scoped timeouts** for external API calls
- **Graceful shutdown** via `context.WithCancel` + signal handling

---

## Error Handling

- Use `fmt.Errorf("doing thing: %w", err)` to wrap errors with context
- Sentinel errors for known failure modes: `var ErrNotFound = errors.New("not found")`
- Background workers log errors and continue rather than crashing
- Individual item failures are logged and skipped; the batch proceeds with remaining items

---

## Logging

### Structured Logging

All logging uses `log/slog` with JSON output to stdout. The logger is initialized in `main.go`:

```go
slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
})))
```

### Log Levels

| Level | Use |
|-------|-----|
| `slog.Info` | Startup, shutdown, config reload, polling results |
| `slog.Warn` | Recoverable failures (API timeouts, enrichment failures, non-critical errors) |
| `slog.Error` | Unrecoverable failures (startup errors, DB connection loss) |

### Guidelines

- Always pass context: use `slog.InfoContext(ctx, ...)` rather than `slog.Info(...)`
- Include enough attributes to reconstruct the operation without reading other log lines
- Use dotted notation for event names where applicable

---

## Testing

### Unit Tests

- Test files live alongside the code they test: `geo_test.go`, `client_test.go`
- Use table-driven tests for operations with multiple input/output combinations
- Test names follow `TestFunctionName_Scenario` convention
- Test assertions use standard `testing.T` methods, not external assertion libraries

### Test Patterns

```go
func TestHaversineKm_KnownDistances(t *testing.T) {
    tests := []struct {
        name     string
        a, b     geo.Coord
        wantKm   float64
        epsilon  float64
    }{
        {
            name:    "same point",
            a:       geo.Coord{Lat: 40.0, Lon: -74.0},
            b:       geo.Coord{Lat: 40.0, Lon: -74.0},
            wantKm:  0,
            epsilon: 0.01,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := geo.HaversineKm(tt.a, tt.b)
            if diff := got - tt.wantKm; diff > tt.epsilon || diff < -tt.epsilon {
                t.Errorf("HaversineKm() = %f, want %f (+/- %f)", got, tt.wantKm, tt.epsilon)
            }
        })
    }
}
```

---

## Nomad Job Structure

### Indentation

- **2 spaces** for HCL/Nomad files
- **No tabs** - spaces only

### Structural Order

**Job level:**
- Metadata (name, type, datacenters, namespace)
- Update policy
- Constraints

**Group level:**
- Count
- Network
- Constraints
- Storage (volumes)
- Restart policy
- Reschedule policy

**Task level:**
- Driver
- Identity
- Config
- Service
- Environment
- Resources
- Termination (kill_timeout, kill_signal)

### Example

```hcl
# -------------------------------------------------------------------------------
# Flight Fetcher - Aircraft Tracking Poller
#
# Project: Flight Fetcher / Author: Alex Freidah
#
# Polls OpenSky Network for aircraft within a configurable radius, enriches
# metadata via HexDB.io, stores current state in Redis and history in Postgres.
# -------------------------------------------------------------------------------

job "flight-fetcher" {
  datacenters = ["dc1"]
  type        = "service"

  # -------------------------------------------------------------------------
  # SERVICE GROUP
  # -------------------------------------------------------------------------

  group "flight-fetcher" {
    count = 1

    # --- Network configuration ---
    network {
      mode = "bridge"
    }

    task "server" {
      driver = "docker"

      # --- Container configuration ---
      config {
        image = "flight-fetcher:latest"
        args  = ["-config", "/local/config.hcl"]
      }

      # --- Resources ---
      resources {
        cpu    = 100
        memory = 64
      }
    }
  }
}
```

---

## Code Style

### Character Rules

**ALWAYS USE:**
- ASCII dash: `-` (hyphen-minus, U+002D)
- Standard ASCII characters only

**NEVER USE:**
- Unicode em-dash (U+2014)
- Unicode en-dash (U+2013)
- Unicode box-drawing (U+2500)
- Equals signs for dividers

### Professional Tone

Avoid:
- Personal references: "Let me show you...", "We need to..."
- Numbered lists in comments: "1. First do this", "2. Then do that"
- Conversational tone: "Now we're going to..."
- Future tense: "This will create...", "We'll configure..."

Use:
- Present tense: "Creates", "Configures", "Manages"
- Declarative statements: "Service runs on port 9000"
- Technical precision: "Uses OAuth2 for API authentication"
- Impersonal voice: "The poller queries...", "The enricher caches..."

---

## Branch Naming

When a branch corresponds to a GitHub issue, use this format:

```
GH_ISSUE_<issue number>-<description of topic>
```

Examples:
- `GH_ISSUE_5-structured-logging`
- `GH_ISSUE_12-postgres-migrations`

For branches without a linked issue, use a short kebab-case description of the topic.

---

## Quick Reference

| Comment Type | Length | Spacing After | Use Case |
|-------------|--------|---------------|----------|
| File header | 79 chars | 1 blank line | Top of every file |
| Major section | 73 chars | 1 blank line | Major divisions (types, API, internals) |
| Single-line comment | Variable | None | Minor divisions within functions |
| Inline | Brief | N/A | Specific line explanation |

---

**Remember:** Comments should explain *why* decisions were made, not *what* the code does. The code itself should be clear enough to understand *what* it does.
