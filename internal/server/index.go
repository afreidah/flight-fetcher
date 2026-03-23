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
	"fmt"
	"strings"
)

//go:embed index.html
var indexHTML string

// renderedHTML returns the index page with placeholders replaced.
func renderedHTML(version string, refreshSec int) []byte {
	r := strings.NewReplacer(
		"{{VERSION}}", version,
		"{{REFRESH}}", fmt.Sprintf("%d", refreshSec),
	)
	return []byte(r.Replace(indexHTML))
}
