// -------------------------------------------------------------------------------
// Migrations - Package Documentation
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Package-level documentation and layering notes for the migrations package.
// -------------------------------------------------------------------------------

// Package migrations embeds the goose migration SQL as a filesystem for the
// store to apply on startup.
//
// Embedding rather than reading from disk means the binary carries its own
// schema: a container image cannot be deployed with migration files that
// disagree with the code compiled beside them. The store applies them on every
// startup, so a fresh database and an upgrade take the same path.
//
// Migrations are numbered and are only ever added, never edited once merged -
// an applied migration is recorded by version, so changing its contents leaves
// existing databases silently out of step. Use `make migration` to create the
// next file with the numbering already resolved.
//
// migrations is a leaf package holding data and no behaviour. It is imported
// only by store.
package migrations
