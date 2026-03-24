# Contributing to Flight Fetcher

## Prerequisites

- Go 1.26+
- Docker and Docker Compose
- [sqlc](https://sqlc.dev/) for query code generation
- [golangci-lint](https://golangci-lint.run/) v2.10+ for linting
- [git-cliff](https://git-cliff.org/) for changelog generation (optional)

## Development Setup

```bash
# Clone and configure
git clone https://github.com/afreidah/flight-fetcher.git
cd flight-fetcher
cp config.example.hcl config.hcl
# Edit config.hcl with your API credentials

# Run the full stack locally
docker compose up --build

# Or build and run the binary directly
make build
./flight-fetcher -config config.hcl
```

## Running Tests

```bash
make test          # unit tests with race detector and coverage
make lint          # golangci-lint
make vet           # Go vet static analysis
make govulncheck   # vulnerability scanner
```

All tests must pass before submitting a PR.

## Code Generation

If you modify SQL queries or interfaces with `//go:generate` directives:

```bash
make generate      # runs sqlc generate and go generate
```

## Branch Naming

When a branch corresponds to a GitHub issue:

```
GH_ISSUE_<issue number>-<description>
```

For branches without a linked issue, use a short kebab-case description.

## Pull Request Process

1. Create a GitHub issue describing the change (unless trivial)
2. Create a branch from `main` using the naming convention above
3. Bump `.version` (patch for fixes, minor for features, major for breaking changes)
4. Make your changes with tests
5. Ensure `make test` and `make lint` pass with zero issues
6. Commit with a clear message describing the change
7. Push and open a PR referencing the issue (`Closes #N`)

## Code Style

See [STYLE_GUIDE.md](STYLE_GUIDE.md) for detailed conventions including:

- File header format (79-char box comments)
- Section dividers (73-char box comments)
- Import grouping (stdlib, internal, external)
- Doc comments on all functions
- Structured logging with `slog` and context propagation
- Error wrapping with `fmt.Errorf("doing thing: %w", err)`

## Project Layout

All application code lives under `internal/`. Each package defines its own
interfaces at the consumer boundary and has co-located tests. See the README
for the full project structure.
