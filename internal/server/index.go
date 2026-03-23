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

import _ "embed"

//go:embed index.html
var indexHTML []byte
