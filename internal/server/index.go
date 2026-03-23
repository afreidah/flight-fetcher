// -------------------------------------------------------------------------------
// Server - Embedded Dashboard HTML
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Contains the embedded HTML, CSS, and JavaScript for the web dashboard.
// The page fetches current flights from the API and displays detail for a
// selected aircraft.
// -------------------------------------------------------------------------------

package server

import (
	_ "embed"
	"strings"
)

//go:embed index.html
var indexHTML string

// renderedHTML returns the index page with the version placeholder replaced.
func renderedHTML(version string) []byte {
	return []byte(strings.Replace(indexHTML, "{{VERSION}}", version, 1))
}
