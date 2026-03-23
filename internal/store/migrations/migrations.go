// -------------------------------------------------------------------------------
// Store - Embedded Migrations
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Embeds goose migration SQL files for use with the goose migration provider.
// Migrations are run on startup to ensure the database schema is current.
// -------------------------------------------------------------------------------

package migrations

import "embed"

// FS contains the embedded migration SQL files.
//
//go:embed *.sql
var FS embed.FS
