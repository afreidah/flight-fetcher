**Project:** Flight Fetcher
**Author:** Alex Freidah

---

## Table of Contents

- [Core Principles](#core-principles)
- [Comment Types and Spacing](#comment-types-and-spacing)
- [Comment Type Decision Tree](#comment-type-decision-tree)
- [File Headers](#file-headers)
- [Package Docs (doc.go)](#package-docs-docgo)
- [Go Conventions](#go-conventions)
- [Project Structure and Layers](#project-structure-and-layers)
- [Error Handling](#error-handling)
- [Logging](#logging)
- [Tracing](#tracing)
- [Metrics](#metrics)
- [Testing](#testing)
- [Lint Suppression](#lint-suppression)
- [Nomad Job Structure](#nomad-job-structure)
- [Code Style](#code-style)
- [Versioning](#versioning)
- [Documentation Updates](#documentation-updates)
- [Branch Naming](#branch-naming)
- [Quick Reference](#quick-reference)
- [Examples](#examples)

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

## Package Docs (doc.go)

Every package carries a `doc.go` holding the package-level godoc. It sits apart
from the implementation files so the package's purpose does not compete with
whichever file happened to be alphabetically first.

```go
// -------------------------------------------------------------------------------
// Route - Package Documentation
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Package-level documentation and layering notes for the route package.
// -------------------------------------------------------------------------------

// Package route holds the flight route domain type: the departure and arrival
// airports resolved for a callsign, along with the airline and flight number
// when the provider supplies them.
//
// route is a domain package. It imports nothing from this module and holds no
// behaviour, so it is exercised through the packages that consume it rather
// than by its own test suite.
package route
```

**What belongs in a package doc:**

- What the package is for, in one opening sentence starting `Package <name>`
- The domain vocabulary a reader needs (why routes key on callsign rather than ICAO24, for instance)
- Its layer position and what it may import, when that is not obvious
- Any standing decision about how the package is tested

That last point matters for packages that carry no statements. `internal/route`
is types only, so it can never report coverage; its `doc.go` records that it is
covered through its consumers, which is the answer to "why is there no test
file here" without anyone having to ask.

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

### Variable Naming

Scope drives length. A name is read where it is used, so the amount of context
it must carry depends on how far it travels.

| Scope | Style | Example |
|---|---|---|
| Loop index, receiver | 1-2 chars | `i`, `p`, `r` |
| Short block (under ~10 lines) | 1 word | `row`, `spec`, `cutoff` |
| Function-wide | descriptive | `redisStore`, `routeSources` |
| Package-level | fully descriptive | `DefaultAircraftTTL`, `flightKeyPrefix` |

- Receivers are the initial of the type and consistent across every method on
  it: `p *PostgresStore`, `r *RedisStore`, `c *Client`.
- Do not repeat the package name in an identifier. `route.Info`, not
  `route.RouteInfo`.
- `err` is the only acceptable name for an error value. The linter forbids `e`
  as a variable name for this reason.

### Typed Constants

Constants that name a member of a closed set should get their own named type,
so the compiler rejects a value from the wrong set and the set is discoverable
from the type:

```go
// SquawkCode is an emergency transponder code the monitor watches for.
type SquawkCode string

const (
    SquawkHijack       SquawkCode = "7500"
    SquawkRadioFailure SquawkCode = "7600"
    SquawkEmergency    SquawkCode = "7700"
)
```

The emergency squawk codes in `internal/squawk` are currently plain untyped
string constants. They predate this rule and are the reference case for it:
because they are untyped, any string reaches a function that takes a squawk,
and the set has to be rediscovered by grep rather than read off a type.

A bare `const` block is correct for values that are not a set - tunables,
prefixes, and defaults:

```go
const (
    // DefaultRouteTTL is the default time before cached routes are considered stale.
    DefaultRouteTTL = 24 * time.Hour
    // DefaultAircraftTTL is the default time before cached aircraft metadata
    // is considered stale and re-enriched.
    DefaultAircraftTTL = 7 * 24 * time.Hour
)
```

Durations are always written as expressions (`7 * 24 * time.Hour`), never as a
pre-multiplied literal, so the intent survives review.

### Struct Organization

**No comments inside struct bodies.** Explain the fields in the doc comment
above the type, and use blank lines within the body to group related fields.
An interleaved comment breaks the visual scan of the field list, and the
grouping usually carries the meaning on its own.

```go
// Options configures a Poller. Name identifies the source in logs and metrics.
// Dedup is shared across every poller so enrichment is not duplicated when
// multiple sources observe the same aircraft. Center and RadiusKm define the
// exact haversine filter applied to every state vector the source returns.
type Options struct {
    Name string

    Source   flightSource
    Cache    flightCache
    Logger   sightingLogger
    Enricher aircraftEnricher

    Dedup *DedupState

    Center   geo.Coord
    RadiusKm float64
    Interval time.Duration
}
```

The groups carry the structure: identity, dependencies, shared state, geometry.

### Interface Design: Consumer-Declared Interfaces

This codebase follows the Go-idiomatic "accept interfaces, return structs"
pattern: **producer packages export concrete `*Type` values with no
producer-side interface**, and **each consumer declares its own narrow
interface** listing only the methods it actually calls. The concrete type
satisfies every consumer's interface because Go interfaces are structurally
typed - the producer never imports the consumer, and never knows the interface
exists.

**Rationale:**

- A consumer's dependency footprint is documented in its own source file.
- Adding a method to a producer type never bloats existing consumer fakes.
- Tests fake at the granularity of what is used, not the full producer surface.
- The producer stays free to grow without coordinating with its consumers.

**Trade-off:** each consumer declares its own small interface, so the same
method signature may appear in two or three places. That duplication is the
price of the decoupling, and it is localized.

**Where the interfaces live.** A package that declares more than one puts them
in `consumer_interfaces.go`, so the whole dependency surface of the package is
readable in one file:

| Location | Declares |
|---|---|
| `internal/server/consumer_interfaces.go` | `flightLister`, `aircraftMetaReader`, `routeReader`, `squawkAlertReader`, `imageFetcher`, `heardChecker`, `pinger` |
| `internal/poller/consumer_interfaces.go` | `flightSource`, `flightCache`, `sightingLogger`, `aircraftEnricher` |
| `internal/squawk/consumer_interfaces.go` | `globalFlightSource`, `alertRecorder`, `aircraftEnricher` |
| `internal/enricher/consumer_interfaces.go` | `aircraftMetaReadWriter`, `routeReadWriter` |
| `internal/retention/consumer_interfaces.go` | `cleaner` |
| `internal/store/postgres.go` | `querier` |
| `cmd/server/wiring.go` | `flightSource` |

Note the last two. A single interface does not need its own file - `querier`
sits next to the store that consumes it, and `flightSource` sits in the
composition root's wiring file.

**Interfaces are unexported by default.** Every interface in the table above is
lowercase. A consumer-declared interface exists for the consumer's benefit; if
it were exported, callers would start passing it around and it would drift back
into being a shared contract. The exception is when the interface is part of
the package's own API surface, as with `notify.Notifier`, which the discord and
telegram backends implement deliberately.

**Naming follows the Go `-er` convention.** A single-method interface is named
after its method in agent-noun form:

| Methods | Good | Bad |
|---|---|---|
| `Ping(ctx) error` | `pinger` | `pingOps` |
| `GetAllFlights(ctx)` | `flightLister` | `flightStore` |
| `LogSighting(ctx, ...)` | `sightingLogger` | `sightingDB` |
| `DeleteOld*(ctx, maxAge)` | `cleaner` | `retentionOps` |

Multi-method interfaces that model a role rather than a single action are
exempt: `querier` and `aircraftMetaReadWriter` describe a surface, not one
verb.

**When NOT to declare a narrow interface.** A consumer-side interface earns its
keep when at least one of these is true:

1. **Multiple implementations actually exist.** `flightSource` in
   `cmd/server/wiring.go` unifies the OpenSky client and the dump1090 antenna
   client so they can sit in one slice - that is a real polymorphism point.
2. **A test fake genuinely benefits from the seam.** `querier` in
   `internal/store` narrows the sqlc-generated API to the ten methods the store
   calls, which is what lets the mapping and error-translation paths run
   against a fake instead of a container.
3. **An import cycle would otherwise form.**
4. **The interface models a real boundary** between subsystems.

If none apply - one implementation, one consumer, no fake, no cycle, no
boundary - pass the concrete `*Type` directly. An interface with a single
implementation and no test fake is indirection that costs a jump-to-definition
and buys nothing.

**Declare the interface where it is consumed, not where it is convenient.**
`flightSource` appears twice, in `internal/poller` and in `cmd/server/wiring.go`,
with the same method. That is deliberate rather than an oversight: the poller
needs it to accept any source, and the composition root needs it because that
is the one place the OpenSky and dump1090 clients are held in the same slice.
Hoisting it into a shared package to remove the duplication would create the
producer-side interface this pattern exists to avoid.

**Producer-side interfaces are an anti-pattern.** `*opensky.Client`,
`*store.PostgresStore`, `*store.RedisStore`, and `*enricher.Enricher` are
exported as concrete pointer types with no sibling interface mirroring their
public surface. A producer-side interface forces every consumer to fake the
full producer API, which is exactly what this pattern is built to avoid.

**A logger is never a behaviour dependency.** Do not put `Log() *slog.Logger`
in a consumer-declared interface. The logger is observability infrastructure:
the consumer depends on no return value from it, and its scope is a property of
the consumer, not the producer. Use the package-level `slog` functions with
context (see [Logging](#logging)).

### Constructor Patterns

**Up to three parameters, take them positionally:**

```go
func NewRedisStore(addr, password string, db int, ttl time.Duration) *RedisStore
func NewPostgresStore(ctx context.Context, dsn string, routeTTL time.Duration) (*PostgresStore, error)
```

**Four or more, take an `Options` struct by pointer:**

```go
func New(opts *Options) *Poller
```

Named fields at the call site beat a positional list nobody can read:

```go
poller.New(&poller.Options{
    Name:     s.name,
    Source:   s.source,
    Cache:    redisStore,
    Logger:   pgStore,
    Enricher: enr,
    Dedup:    dedup,
    Center:   center,
    RadiusKm: cfg.Location.RadiusKm,
    Interval: s.interval,
})
```

`poller.Options`, `server.Options`, and `enricher.Options` all follow this.

**Constructors do not validate configuration.** Validation belongs in
`internal/config`, which parses and checks the HCL before anything is
constructed. By the time a constructor runs, its inputs are already known good.
A constructor that re-validates either duplicates the config package or
disagrees with it, and the second failure mode is worse.

**The composition root is `cmd/server`, and there is no DI framework.**
`main.go` constructs everything in order and passes concrete types into
consumers that accept interfaces. The decisions about *what* to construct live
in `cmd/server/wiring.go` as pure functions of config, which is what makes the
entrypoint's logic testable without standing up Redis, Postgres, or a network.
Keep construction in `main.go` and decisions in `wiring.go`.

**Inject the clock for anything time-dependent.** A type whose behaviour
depends on the current time takes a `now func() time.Time` field, defaulting to
`time.Now` in the constructor:

```go
type PostgresStore struct {
    // ...
    now func() time.Time
}
```

Every TTL, cooldown, and retention cutoff derives from `p.now()`. Tests freeze
it and assert an exact instant instead of allowing a tolerance, which turns a
flaky comparison into an equality check.

### No Empty Cleanup Funcs

A constructor that returns a cleanup function returns a real one or does not
return one at all. An empty `func() {}` is a lie that survives refactors: the
caller defers it, a reader assumes something is torn down, and nothing is.

```go
// Bad - the caller cannot tell this does nothing.
func New() (*Thing, func()) {
    return &Thing{}, func() {}
}

// Good - nothing to clean up, so nothing is returned.
func New() *Thing {
    return &Thing{}
}
```

Where a cleanup genuinely exists, it does one thing and the doc comment says
what. `observe.Setup` returns a shutdown that flushes pending spans, and its
comment states that the caller must defer it rather than the callee, or the
trace provider is torn down before anything has been traced.

### Concurrency Patterns

- **Context-scoped timeouts** for external API calls
- **Graceful shutdown** via `signal.NotifyContext` plus an `errgroup`
- Long-running loops live behind `internal/runloop` so ticker handling and
  context cancellation are implemented once

---

## Project Structure and Layers

```
cmd/server/          Composition root: main.go constructs, wiring.go decides
internal/
  aircraft/          Domain type: aircraft metadata
  route/             Domain type: flight route
  geo/               Domain type: coordinates, bounding boxes, haversine
  config/            HCL parsing and validation
  observe/           OTel tracer and meter setup, trace-aware slog handler
  runloop/           Ticker loop with context cancellation
  apiclient/         Shared HTTP client: retry, backoff, metrics, tracing
    airlabs/         Route lookup
    flightaware/     Route lookup
    hexdb/           Aircraft metadata lookup
    opensky/         State vectors and aircraft metadata
    dump1090/        Local antenna state vectors
  notify/            Notifier interface and fan-out manager
    discord/         Webhook backend
    telegram/        Bot API backend
  enricher/          Ordered fallback chains for metadata and routes
  poller/            Poll a source, filter by radius, cache, log, enrich
  squawk/            Emergency squawk detection and alerting
  retention/         Periodic deletion of aged rows
  store/             Redis current state, Postgres history and cache
    sqlc/            Generated query code - do not edit
    migrations/      Embedded goose migrations
  server/            HTTP dashboard and JSON API
```

### Layer Responsibilities

| Layer | Packages | May import |
|---|---|---|
| **Domain** | `aircraft`, `route`, `geo` | nothing from this module |
| **Infrastructure** | `config`, `observe`, `runloop`, `apiclient` | nothing from this module |
| **Clients** | `apiclient/*`, `notify/*` | domain, infrastructure |
| **Services** | `enricher`, `poller`, `squawk`, `retention`, `store`, `server` | domain, infrastructure, clients |
| **Composition** | `cmd/server` | everything |

The rule that keeps this honest: **domain packages import nothing from this
module.** `aircraft`, `route`, and `geo` are the shared vocabulary every other
layer agrees on, so a dependency pointing out of them would make them a service
in disguise and open the door to a cycle.

Services do not import each other. `poller` does not import `store`; it
declares `flightCache` and `sightingLogger` and lets the composition root pass
the concrete store in. This is what makes the layer table enforceable rather
than aspirational - a service that imported another service would immediately
show up as an extra edge here.

### A Single Poll Cycle, By Layer

1. `cmd/server` constructs a `poller.Poller` with an OpenSky client, a Redis
   store, a Postgres store, and an enricher.
2. `poller` asks its `flightSource` for state vectors in a bounding box
   (`geo`).
3. `apiclient` performs the request with retry and backoff, recording a span
   and metrics.
4. `poller` filters to a circular radius with `geo.HaversineKm`.
5. Each survivor is written to the `flightCache` and the `sightingLogger`, both
   satisfied by the concrete stores.
6. Unseen aircraft are handed to the `aircraftEnricher`, which tries HexDB then
   OpenSky and writes results through to Postgres.

No step in that chain required one service to import another.

---

## Error Handling

- Use `fmt.Errorf("doing thing: %w", err)` to wrap errors with context
- Sentinel errors for known failure modes: `var ErrNotFound = errors.New("not found")`
- Background workers log errors and continue rather than crashing
- Individual item failures are logged and skipped; the batch proceeds with remaining items

**Distinguish "absent" from "failed".** A lookup that finds nothing returns
`(nil, nil)`, not an error. The store translates `pgx.ErrNoRows` and
`redis.Nil` into a nil result at the boundary, so callers can tell "this
aircraft has not been enriched" from "the database is down" without inspecting
driver-specific sentinels:

```go
row, err := p.queries.GetAircraftMeta(ctx, params)
if errors.Is(err, pgx.ErrNoRows) {
    return nil, nil
}
if err != nil {
    return nil, err
}
```

**Degrade where a partial answer is useful, fail where it is not.** The list
endpoint skips an entry it cannot decode and returns the rest, because one
corrupt key should not blank the dashboard. A single-key lookup surfaces the
same decode failure as an error, because there is no partial answer to give.

**Return the zero value alongside an error, never a partial one.** A failed
retention sweep returns `(0, err)`, so a caller that logs the row count cannot
mistake a failure for a sweep that found nothing to delete.

---

## Logging

### Structured Logging

All logging uses `log/slog` with JSON output to stdout. The default logger is
installed in `main.go` and wrapped in a trace-aware handler so every line
carries its trace and span IDs:

```go
slog.SetDefault(slog.New(&observe.TracedHandler{
    Handler: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}),
}))
```

### Log Levels

| Level | Use |
|-------|-----|
| `slog.Info` | Startup, shutdown, config reload, polling results |
| `slog.Warn` | Recoverable failures (API timeouts, enrichment failures, non-critical errors) |
| `slog.Error` | Unrecoverable failures (startup errors, DB connection loss) |

### Guidelines

- Always pass context: use `slog.InfoContext(ctx, ...)` rather than `slog.Info(...)`.
  This is what lets `TracedHandler` attach the trace ID, and it is enforced by
  the `sloglint` linter.
- Use `slog.String`, `slog.Int`, `slog.Duration` attributes rather than
  formatting values into the message. The message stays a constant, which is
  what makes it groupable.
- Include enough attributes to reconstruct the operation without reading other
  log lines
- Never log credentials. Config types carry `json:"-"` on every
  credential-bearing field for the same reason.

---

## Tracing

Tracing is set up once in `observe.Setup` and consumed through the global
provider. A package that traces acquires its tracer once, at construction.

### Tracer Names

One tracer per package, named `flight-fetcher/<package>`:

```go
tracer: otel.Tracer("flight-fetcher/postgres"),
```

### Span Names

`<subsystem>.<operation>`, lowercase subsystem, dot separated:

| Span | Where |
|---|---|
| `poller.poll` | one poll cycle |
| `squawk.scan` | one squawk sweep |
| `postgres.GetAircraftMeta` | one store method |
| `opensky.request` | one outbound HTTP request |

The store derives the operation from the method name, so a new method is traced
correctly without a second edit.

### Starting a Span

Wrap the span in a helper rather than repeating the start-and-end dance. The
store's `startSpan` returns the context and an end function that records any
error:

```go
func (p *PostgresStore) startSpan(ctx context.Context, name string) (context.Context, func(error)) {
    ctx, span := p.tracer.Start(ctx, "postgres."+name)
    return ctx, func(err error) {
        if err != nil {
            span.RecordError(err)
            span.SetStatus(codes.Error, err.Error())
        }
        span.End()
    }
}
```

### Recording Errors

An error on a span gets both `RecordError` and `SetStatus(codes.Error, ...)`.
`RecordError` alone attaches the event but leaves the span green in most UIs,
so a failure would not surface in a trace search.

### Attributes

Attach the identifiers needed to find the span again - `icao24`, `callsign`,
`source` - and nothing derived from them. Do not attach anything unbounded such
as a full response body.

---

## Metrics

### Meter Names

One meter per package, matching the tracer name:

```go
meter := otel.Meter("flight-fetcher/poller")
```

### Instrument Names

`<subsystem>.<noun>` for counters and gauges, `<subsystem>.<noun>.duration` for
histograms. Every instrument carries a description; durations carry a unit:

```go
pollCount, _ := meter.Int64Counter("poller.polls",
    metric.WithDescription("Total poll cycles by result"))
pollDuration, _ := meter.Float64Histogram("poller.poll.duration",
    metric.WithDescription("Poll cycle duration in seconds"),
    metric.WithUnit("s"))
aircraftGauge, _ := meter.Int64Gauge("poller.aircraft.count",
    metric.WithDescription("Aircraft seen in the last poll cycle"))
```

Durations are always seconds, as a float histogram. Mixed units across a
dashboard are a recurring source of wrong conclusions.

### Label Cardinality

Labels must come from a bounded set. Source name, upstream name, HTTP status
class, and result (`ok` / `error`) are all fine. **ICAO24, callsign, and any
other per-aircraft identifier are not** - they are unbounded, and each distinct
value is a new time series that never expires. Per-aircraft detail belongs on a
span, which is sampled, not on a metric, which is not.

### Instruments Are Created Once

Create instruments at construction and store them on the type. Creating one per
call re-registers it on every invocation and allocates in the hot path.

---

## Testing

### Unit Tests

- Test files live alongside the code they test: `geo_test.go`, `client_test.go`
- Use table-driven tests for operations with multiple input/output combinations
- Test names follow `TestFunctionName_Scenario` convention
- Test assertions use standard `testing.T` methods, not external assertion libraries
- Mark helpers with `t.Helper()` so a failure reports the caller's line

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

### Unit Tests and Integration Tests

Both live in the same package, split by what they prove and gated so the unit
half always runs:

| | Unit | Integration |
|---|---|---|
| File | `postgres_test.go` | `postgres_integration_test.go` |
| Proves | mapping, error translation, cutoff arithmetic, key layout | real SQL behaviour, real Redis semantics, migrations |
| Backed by | a hand-rolled fake, or `miniredis` | `testcontainers-go` |
| Needs Docker | no | yes |
| `make test` (`-short`) | runs | skips |

Integration tests skip themselves under `-short`:

```go
if testing.Short() {
    t.Skip("skipping integration test in short mode")
}
```

**Prefer the unit half for anything that is awkward to arrange against a live
backend.** A key holding the wrong type, a value that is not valid JSON, a
driver error, an exact TTL boundary - all are a line of setup against a fake
and a fight against a real database.

**Fake at the interface, not the implementation.** Because consumers declare
narrow interfaces, a fake implements four to ten methods rather than a full
client. Record what the code under test passed down, and assert on that; it is
usually more informative than the return value.

**Use `miniredis` rather than a container where the protocol is the point.** It
speaks real Redis, so the client, the SCAN iterator, and pipelines all still
execute - only the server is substituted, and `FastForward` replaces sleeping
for a TTL.

**Names must not collide across the two files.** They share a package, so a
unit helper called `newTestStore` will clash with the integration one. Give the
unit fixtures distinct names (`newFakeStore`, `newMiniredisStore`).

### What to Pin

Beyond the happy path, cover the behaviours that regress silently:

- A miss returns `(nil, nil)`; a failure returns an error
- Empty results are empty-but-non-nil where the value is marshalled to JSON,
  since nil renders as `null` rather than `[]`
- Optional pointer and slice fields survive a round trip, since those are the
  ones a wrong json tag silently drops
- Order is preserved where a caller depends on it

---

## Lint Suppression

`make lint` must pass with zero issues. When a finding cannot be fixed, the
suppression goes at the site, not in the config:

```go
//nolint:gocritic // signature fixed by the generated querier interface
func (f *fakeQuerier) UpsertAircraftMeta(_ context.Context, arg db.UpsertAircraftMetaParams) error {
```

**Always name the linter and give the reason.** A bare `//nolint` suppresses
every linter at that line, including ones that have not run yet.

**Prefer a narrow directive over a `.golangci.yml` exclusion.** A directive
applies to one line and is visible to the next reader of that line; a config
exclusion applies to every current and future occurrence and is invisible from
the code. A config-level exclusion is appropriate only for a whole generated
tree, such as `internal/store/sqlc`.

**A config exclusion added for a known-bad case is temporary.** When it is
added, it names the issue that will remove it, and it is deleted in the PR that
closes that issue.

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

## Versioning

The repository version lives in `.version` at the root, as a `v`-prefixed
semantic version on a single line:

```
v0.9.32
```

**Every pull request bumps it.** The build stamps it into the binary via
`-ldflags -X main.Version`, the container image is tagged from it, and the
dashboard reports it, so a version that did not move makes a deployed build
impossible to identify.

| Change | Bump |
|---|---|
| Bug fix, refactor, docs, test, CI | patch |
| New capability, new config block, new endpoint | minor |
| Breaking config change, removed endpoint | major |

Pre-1.0, a breaking change takes a minor bump. `CHANGELOG.md` is generated from
commit messages by `git-cliff` (`cliff.toml`), which is why commit subjects are
written to be read by someone scanning a release.

---

## Documentation Updates

Documentation is part of the change, not a follow-up. A pull request updates
whatever it invalidated:

| Changed | Also update |
|---|---|
| A config block | `config.example.hcl`, the README config section |
| An HTTP endpoint | the README API section |
| A package's purpose or layering | that package's `doc.go` |
| A convention this guide records | this guide |
| Anything user-visible | `.version` |

**A package doc that describes the old design is worse than no package doc**,
because it is trusted. When behaviour moves between packages, the `doc.go` on
both sides is part of the diff.

**Record the decision, not just the change.** Where a non-obvious choice is
made, the reason belongs in the code as a comment, next to what it explains.
The comment on `plannedSources` in `cmd/server/wiring.go` explaining why three
separate OpenSky clients are correct is the model: it answers the question a
reader will have, at the place they will have it, and points at the issue that
would change the answer.

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

| Rule | Value |
|---|---|
| Constructor params before an Options struct | 4 |
| Tracer / meter name | `flight-fetcher/<package>` |
| Span name | `<subsystem>.<operation>` |
| Duration metric unit | seconds, float histogram |
| Version file | `.version`, bumped every PR |
| Lint gate | `make lint`, zero issues |

---

## Examples

### Bad

```go
// Store interface for the poller
type Store interface {
    SetFlight(ctx context.Context, sv *opensky.StateVector) error
    GetFlight(ctx context.Context, icao24 string) (*opensky.StateVector, error)
    GetAllFlights(ctx context.Context) ([]opensky.StateVector, error)
    LogSighting(ctx context.Context, icao24 string, lat, lon, dist float64) error
    SaveAircraftMeta(ctx context.Context, info *aircraft.Info) error
    GetAircraftMeta(ctx context.Context, icao24 string) (*aircraft.Info, error)
    Ping(ctx context.Context) error
    Close()
}

type Poller struct {
    store  Store          // needs 2 of these 8 methods
    logger *slog.Logger   // threaded in as a dependency
    ttl    time.Duration  // 3 hours? 3 minutes? unclear at the call site
}

func NewPoller(s Store, l *slog.Logger, ttl time.Duration, n string, i time.Duration, lat, lon, r float64) *Poller {
    if r <= 0 {
        r = 50 // silently corrects bad config
    }
    return &Poller{store: s, logger: l, ttl: ttl}
}

func (p *Poller) poll(ctx context.Context) {
    flights, err := p.fetch(ctx)
    if err != nil {
        p.logger.Error("poll failed: " + err.Error()) // no context, formatted message
        return
    }
    for _, f := range flights {
        p.store.SetFlight(ctx, &f) // error dropped
    }
}
```

Problems: one wide producer-side interface every consumer must fake whole; a
logger threaded as a dependency; eight positional constructor parameters; config
silently corrected in a constructor; a formatted log message that cannot be
grouped, with no context; a dropped error.

### Good

```go
// flightCache is the subset of the flight store the poller writes to.
type flightCache interface {
    SetFlight(ctx context.Context, sv *opensky.StateVector) error
    MarkHeard(ctx context.Context, source, icao24 string, ttl time.Duration) error
}

// Options configures a Poller. Name identifies the source in logs and metrics.
// Center and RadiusKm define the exact haversine filter applied to every state
// vector the source returns.
type Options struct {
    Name string

    Source flightSource
    Cache  flightCache

    Center   geo.Coord
    RadiusKm float64
    Interval time.Duration
}

// New returns a Poller for the configured source. Options are already
// validated by internal/config, so nothing is corrected here.
func New(opts *Options) *Poller {
    return &Poller{opts: *opts}
}

// poll runs one cycle: fetch, filter to the radius, cache. A single aircraft
// failing to cache is logged and skipped so one bad record cannot end the cycle.
func (p *Poller) poll(ctx context.Context) {
    ctx, span := p.tracer.Start(ctx, "poller.poll")
    defer span.End()

    flights, err := p.opts.Source.GetStates(ctx, p.bbox())
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        slog.ErrorContext(ctx, "poll failed",
            slog.String("source", p.opts.Name),
            slog.String("error", err.Error()))
        return
    }

    for i := range flights.States {
        sv := &flights.States[i]
        if geo.HaversineKm(p.opts.Center, geo.Coord{Lat: sv.Latitude, Lon: sv.Longitude}) > p.opts.RadiusKm {
            continue
        }
        if err := p.opts.Cache.SetFlight(ctx, sv); err != nil {
            slog.WarnContext(ctx, "caching flight failed",
                slog.String("icao24", sv.ICAO24),
                slog.String("error", err.Error()))
        }
    }
}
```

The consumer declares the two methods it uses; the concrete store satisfies it
structurally. Options are named at the call site and already validated. The
span records the error and sets the status. Log messages are constants with
attributes. A per-aircraft failure degrades the cycle rather than ending it.

---

**Remember:** Comments should explain *why* decisions were made, not *what* the code does. The code itself should be clear enough to understand *what* it does.
