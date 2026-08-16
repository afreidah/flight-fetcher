// -------------------------------------------------------------------------------
// Server - Web Dashboard HTTP Server
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Serves a lightweight web dashboard for viewing current flight state from Redis
// and enriched aircraft metadata from Postgres. Provides JSON API endpoints and
// an embedded HTML frontend.
// -------------------------------------------------------------------------------

package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/afreidah/flight-fetcher/internal/aircraft"
	"github.com/afreidah/flight-fetcher/internal/apiclient/opensky"
	"github.com/afreidah/flight-fetcher/internal/route"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// HealthPinger pairs a backend dependency with the name reported for it in
// the /healthz response.
type HealthPinger struct {
	Name   string
	Pinger pinger
}

// Options holds the dependencies and configuration for the dashboard server.
// Routes, Alerts, Images, and Heard are optional: the handlers that use them
// degrade to omitting that part of the response when they are nil, so a
// deployment without route enrichment or a second poller still serves. Sources
// names the pollers the heard-by check considers and must be non-empty for
// Heard to have any effect.
type Options struct {
	Flights    flightLister
	Aircraft   aircraftMetaReader
	Routes     routeReader
	Alerts     squawkAlertReader
	Images     imageFetcher
	Heard      heardChecker
	Sources    []string
	Pingers    []HealthPinger
	Version    string
	RefreshSec int
}

// Server serves the web dashboard and flight data API.
type Server struct {
	opts      Options
	indexPage []byte
	mux       *http.ServeMux
}

// flightDetail combines live state, enriched metadata, and route information.
type flightDetail struct {
	State          *opensky.StateVector  `json:"state"`
	Aircraft       *aircraft.Info        `json:"aircraft,omitempty"`
	Route          *route.Info           `json:"route,omitempty"`
	Classification string                `json:"classification,omitempty"`
	TypeSpec       *aircraft.TypeSpec    `json:"type_spec,omitempty"`
	Airline        *aircraft.AirlineInfo `json:"airline,omitempty"`
	HeardBy        []string              `json:"heard_by,omitempty"`
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// New creates a Server with the given options.
func New(opts *Options) *Server {
	s := &Server{
		opts:      *opts,
		indexPage: renderedHTML(opts.Version, opts.RefreshSec),
		mux:       http.NewServeMux(),
	}
	s.mux.HandleFunc("GET /", s.handleIndex)
	s.mux.HandleFunc("GET /api/flights", s.handleListFlights)
	s.mux.HandleFunc("GET /api/flights/{icao24}", s.handleGetFlight)
	s.mux.HandleFunc("GET /api/squawk-alerts", s.handleSquawkAlerts)
	s.mux.HandleFunc("GET /api/aircraft/{icao24}", s.handleGetAircraft)
	s.mux.HandleFunc("GET /api/routes/{callsign}", s.handleGetRoute)
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.Handle("GET /metrics", promhttp.Handler())
	return s
}

// ListenAndServe starts the HTTP server on the given address. Blocks until
// the context is cancelled, then shuts down gracefully. Returns a non-nil
// error if the server fails to start or shutdown fails.
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           otelhttp.NewHandler(s.mux, "flight-fetcher"),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		slog.InfoContext(ctx, "dashboard shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.ErrorContext(ctx, "dashboard shutdown error", slog.String("error", err.Error()))
		}
	}()

	slog.InfoContext(ctx, "dashboard listening", slog.String("addr", addr))
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// -------------------------------------------------------------------------
// INTERNALS
// -------------------------------------------------------------------------

// handleIndex serves the embedded HTML dashboard page.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write(s.indexPage); err != nil {
		slog.WarnContext(r.Context(), "failed to write index page",
			slog.String("error", err.Error()))
	}
}

// flightListEntry wraps a state vector with classification for the list view.
type flightListEntry struct {
	opensky.StateVector
	Classification string   `json:"classification,omitempty"`
	OperatorCode   string   `json:"operator_code,omitempty"`
	HeardBy        []string `json:"heard_by,omitempty"`
}

// handleListFlights returns all current flights as JSON. Returns an empty
// array instead of 500 when Redis is unavailable so the dashboard degrades
// gracefully rather than failing entirely.
func (s *Server) handleListFlights(w http.ResponseWriter, r *http.Request) {
	flights, err := s.opts.Flights.GetAllFlights(r.Context())
	if err != nil {
		slog.WarnContext(r.Context(), "api: list flights failed, returning empty",
			slog.String("error", err.Error()))
		writeJSON(r.Context(), w, []flightListEntry{})
		return
	}
	var heardMap map[string][]string
	if s.opts.Heard != nil && len(s.opts.Sources) > 0 && len(flights) > 0 {
		icaos := make([]string, len(flights))
		for i := range flights {
			icaos[i] = flights[i].ICAO24
		}
		m, err := s.opts.Heard.HeardByAll(r.Context(), icaos, s.opts.Sources)
		if err != nil {
			slog.WarnContext(r.Context(), "api: heard-by lookup failed",
				slog.String("error", err.Error()))
		} else {
			heardMap = m
		}
	}

	entries := make([]flightListEntry, len(flights))
	for i := range flights {
		f := &flights[i]
		owners := ""
		operator := ""
		if meta, err := s.opts.Aircraft.GetAircraftMeta(r.Context(), f.ICAO24); err == nil && meta != nil && !meta.IsSentinel() {
			owners = meta.RegisteredOwners
			operator = meta.OperatorFlagCode
		}
		entries[i] = flightListEntry{
			StateVector:    *f,
			Classification: aircraft.Classify(f.ICAO24, owners),
			OperatorCode:   operator,
			HeardBy:        heardMap[f.ICAO24],
		}
	}
	writeJSON(r.Context(), w, entries)
}

// handleGetFlight returns live state and enriched metadata for a single aircraft.
func (s *Server) handleGetFlight(w http.ResponseWriter, r *http.Request) {
	icao24 := r.PathValue("icao24")

	sv, err := s.opts.Flights.GetFlight(r.Context(), icao24)
	if err != nil {
		http.Error(w, "failed to get flight", http.StatusInternalServerError)
		slog.WarnContext(r.Context(), "api: get flight failed",
			slog.String("icao24", icao24),
			slog.String("error", err.Error()))
		return
	}
	if sv == nil {
		http.Error(w, "flight not found", http.StatusNotFound)
		return
	}

	detail := flightDetail{State: sv}
	s.attachAircraft(r, icao24, &detail)
	s.attachRoute(r, icao24, sv.Callsign, &detail)
	s.attachHeardBy(r, icao24, &detail)

	writeJSON(r.Context(), w, detail)
}

// attachAircraft fills in the metadata-derived fields of detail. A lookup
// failure and a sentinel record are both treated as "no metadata": the
// aircraft still gets a classification from its ICAO24 alone, so the response
// degrades to live state rather than failing.
func (s *Server) attachAircraft(r *http.Request, icao24 string, detail *flightDetail) {
	meta, err := s.opts.Aircraft.GetAircraftMeta(r.Context(), icao24)
	if err != nil {
		slog.WarnContext(r.Context(), "api: aircraft meta lookup failed",
			slog.String("icao24", icao24),
			slog.String("error", err.Error()))
	}
	if meta != nil && meta.IsSentinel() {
		meta = nil
	}

	detail.Aircraft = meta
	if meta == nil {
		detail.Classification = aircraft.Classify(icao24, "")
		return
	}

	if meta.ImageURL == "" && s.opts.Images != nil {
		meta.ImageURL = s.opts.Images.FetchImageURL(r.Context(), icao24)
	}
	detail.Classification = aircraft.Classify(icao24, meta.RegisteredOwners)
	detail.TypeSpec = aircraft.LookupType(meta.ICAOTypeCode)
	detail.Airline = aircraft.LookupAirline(meta.OperatorFlagCode)
}

// attachRoute fills in the route field when route enrichment is configured and
// the aircraft is broadcasting a callsign to look it up by.
func (s *Server) attachRoute(r *http.Request, icao24, callsign string, detail *flightDetail) {
	if s.opts.Routes == nil || callsign == "" {
		return
	}
	route, err := s.opts.Routes.GetFlightRoute(r.Context(), strings.TrimSpace(callsign))
	if err != nil {
		slog.WarnContext(r.Context(), "api: route lookup failed",
			slog.String("icao24", icao24),
			slog.String("error", err.Error()))
	}
	detail.Route = route
}

// attachHeardBy fills in which configured sources are currently hearing the
// aircraft. Only meaningful when at least one poller source is named.
func (s *Server) attachHeardBy(r *http.Request, icao24 string, detail *flightDetail) {
	if s.opts.Heard == nil || len(s.opts.Sources) == 0 {
		return
	}
	heard, err := s.opts.Heard.HeardBy(r.Context(), icao24, s.opts.Sources)
	if err != nil {
		slog.WarnContext(r.Context(), "api: heard-by lookup failed",
			slog.String("icao24", icao24),
			slog.String("error", err.Error()))
		return
	}
	detail.HeardBy = heard
}

// handleSquawkAlerts returns recent emergency squawk alerts as JSON.
func (s *Server) handleSquawkAlerts(w http.ResponseWriter, r *http.Request) {
	if s.opts.Alerts == nil {
		writeJSON(r.Context(), w, []any{})
		return
	}
	alerts, err := s.opts.Alerts.GetRecentSquawkAlerts(r.Context(), 24*time.Hour)
	if err != nil {
		http.Error(w, "failed to get squawk alerts", http.StatusInternalServerError)
		slog.WarnContext(r.Context(), "api: squawk alerts failed",
			slog.String("error", err.Error()))
		return
	}
	writeJSON(r.Context(), w, alerts)
}

// handleGetAircraft returns cached aircraft metadata by ICAO24.
func (s *Server) handleGetAircraft(w http.ResponseWriter, r *http.Request) {
	icao24 := r.PathValue("icao24")
	meta, err := s.opts.Aircraft.GetAircraftMeta(r.Context(), icao24)
	if err != nil {
		http.Error(w, "failed to get aircraft", http.StatusInternalServerError)
		slog.WarnContext(r.Context(), "api: aircraft lookup failed",
			slog.String("icao24", icao24),
			slog.String("error", err.Error()))
		return
	}
	if meta == nil || meta.IsSentinel() {
		http.Error(w, "aircraft not found", http.StatusNotFound)
		return
	}
	if meta.ImageURL == "" && s.opts.Images != nil {
		meta.ImageURL = s.opts.Images.FetchImageURL(r.Context(), icao24)
	}
	writeJSON(r.Context(), w, meta)
}

// handleGetRoute returns cached flight route information by callsign.
func (s *Server) handleGetRoute(w http.ResponseWriter, r *http.Request) {
	callsign := r.PathValue("callsign")
	if s.opts.Routes == nil {
		http.Error(w, "route not found", http.StatusNotFound)
		return
	}
	route, err := s.opts.Routes.GetFlightRoute(r.Context(), callsign)
	if err != nil {
		http.Error(w, "failed to get route", http.StatusInternalServerError)
		slog.WarnContext(r.Context(), "api: route lookup failed",
			slog.String("callsign", callsign),
			slog.String("error", err.Error()))
		return
	}
	if route == nil {
		http.Error(w, "route not found", http.StatusNotFound)
		return
	}
	writeJSON(r.Context(), w, route)
}

// healthResponse is returned by the /healthz endpoint.
type healthResponse struct {
	Status     string            `json:"status"`
	Components map[string]string `json:"components"`
}

// handleHealthz checks all registered backend dependencies and returns
// JSON with per-component status. Returns 200 for healthy or degraded,
// 503 only if all components are unreachable.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	result := healthResponse{
		Status:     "healthy",
		Components: make(map[string]string, len(s.opts.Pingers)),
	}

	failures := 0
	for _, p := range s.opts.Pingers {
		if err := p.Pinger.Ping(ctx); err != nil {
			result.Components[p.Name] = err.Error()
			failures++
			slog.WarnContext(r.Context(), "health check failed",
				slog.String("component", p.Name),
				slog.String("error", err.Error()))
		} else {
			result.Components[p.Name] = "ok"
		}
	}

	if failures > 0 && failures < len(s.opts.Pingers) {
		result.Status = "degraded"
	} else if failures == len(s.opts.Pingers) && len(s.opts.Pingers) > 0 {
		result.Status = "unhealthy"
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	writeJSON(r.Context(), w, result)
}

// writeJSON encodes v as JSON and writes it to w.
func writeJSON(ctx context.Context, w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.WarnContext(ctx, "failed to write JSON response",
			slog.String("error", err.Error()))
	}
}
