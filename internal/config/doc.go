// -------------------------------------------------------------------------------
// Config - Package Documentation
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Package-level documentation and layering notes for the config package.
// -------------------------------------------------------------------------------

// Package config loads and validates the service configuration from HCL:
// receiver location and radius, poll intervals, OpenSky credentials, Postgres
// and Redis connection settings, the optional dump1090 antenna source, route
// enrichment providers, the dashboard server, the squawk monitor, retention
// ages, and notification backends.
//
// Validation belongs here rather than in the constructors of the packages that
// consume the values. A bad configuration then fails at startup with a message
// naming the offending block, instead of surfacing as a confusing runtime error
// from a component several layers down. Optional blocks are represented as
// pointers so their absence is distinguishable from a zero value, which is what
// lets a feature be switched off by omission.
//
// Secrets are templated into the HCL file by Vault in production, so the loader
// treats the file as trusted input and does no decryption of its own.
//
// config is a leaf package. It imports nothing from this module and is used
// only by the composition root in cmd/server.
package config
