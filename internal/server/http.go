// Package server provides the HTTP server implementation.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/route-ans/route-ans/internal/cache"
	"github.com/route-ans/route-ans/internal/config"
	"github.com/route-ans/route-ans/internal/registry"
	"github.com/route-ans/route-ans/internal/resolver"
	"github.com/route-ans/route-ans/internal/store"
	"github.com/route-ans/route-ans/internal/telemetry"
	"github.com/route-ans/route-ans/internal/trust"
	"github.com/route-ans/route-ans/pkg/ansname"
	"github.com/rs/zerolog/log"
)

// Server represents the HTTP server
type Server struct {
	cfg        *config.Config
	version    string
	httpServer *http.Server
	metrics    *telemetry.Metrics

	// Core resolver (encapsulates cache, store, registry, trust)
	resolver *resolver.DefaultResolver

	// Keep references for health checks and direct access if needed
	cache    cache.Provider
	store    store.Provider
	registry registry.Adapter
}

// New creates a new server instance
func New(cfg *config.Config, version string) (*Server, error) {
	s := &Server{
		cfg:     cfg,
		version: version,
		metrics: telemetry.NewMetrics(
			cfg.Spec.Telemetry.Metrics.Namespace,
			cfg.Spec.Telemetry.Metrics.Subsystem,
		),
	}

	// Initialize providers
	if err := s.initProviders(); err != nil {
		return nil, fmt.Errorf("failed to initialize providers: %w", err)
	}

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:           fmt.Sprintf("%s:%d", cfg.Spec.Server.HTTP.Host, cfg.Spec.Server.HTTP.Port),
		Handler:        s.routes(),
		ReadTimeout:    cfg.Spec.Server.HTTP.ReadTimeout,
		WriteTimeout:   cfg.Spec.Server.HTTP.WriteTimeout,
		IdleTimeout:    cfg.Spec.Server.HTTP.IdleTimeout,
		MaxHeaderBytes: cfg.Spec.Server.HTTP.MaxHeaderBytes,
	}

	return s, nil
}

func (s *Server) initProviders() error {
	var err error

	// Initialize cache
	cacheOpts := cache.Options{
		DefaultTTL:      s.cfg.Spec.Cache.TTL.Default,
		MaxSize:         s.cfg.Spec.Cache.Memory.MaxSize,
		CleanupInterval: s.cfg.Spec.Cache.Memory.CleanupInterval,
	}
	s.cache, err = cache.New(s.cfg.Spec.Cache.Provider, cacheOpts)
	if err != nil {
		return fmt.Errorf("failed to create cache: %w", err)
	}
	log.Info().Str("provider", s.cfg.Spec.Cache.Provider).Msg("Cache initialized")

	// Initialize store
	storeOpts := store.Options{
		MaxSize: s.cfg.Spec.Store.Memory.MaxSize,
	}
	s.store, err = store.New(s.cfg.Spec.Store.Provider, storeOpts, nil)
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}
	log.Info().Str("provider", s.cfg.Spec.Store.Provider).Msg("Store initialized")

	// Initialize registry adapter (use first enabled one)
	for _, regCfg := range s.cfg.Spec.Registries {
		if !regCfg.Enabled {
			continue
		}
		regOpts := registry.Options{
			Timeout:      regCfg.Timeout,
			Retries:      regCfg.Retries,
			RetryBackoff: regCfg.RetryBackoff,
			Priority:     regCfg.Priority,
		}
		s.registry, err = registry.New(regCfg.Type, regOpts, regCfg.Config)
		if err != nil {
			log.Warn().Err(err).Str("registry", regCfg.Name).Msg("Failed to create registry adapter")
			continue
		}
		log.Info().Str("name", regCfg.Name).Str("type", regCfg.Type).Msg("Registry adapter initialized")
		break
	}

	if s.registry == nil {
		// Create a mock registry if none configured
		s.registry, _ = registry.New("mock", registry.DefaultOptions(), nil)
		log.Warn().Msg("No registry configured, using mock adapter")
	}

	// Initialize trust verifier
	var verifier trust.Verifier
	trustConfig := map[string]interface{}{
		"trustedRootsFile":      s.cfg.Spec.Trust.File.TrustedRootsFile,
		"trustedRegistrarsFile": s.cfg.Spec.Trust.File.TrustedRegistrarsFile,
	}
	trustProvider, err := trust.New(s.cfg.Spec.Trust.Provider, trust.Options{}, trustConfig)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to create trust provider, verification disabled")
	} else {
		verifierConfig := trust.VerifierConfig{
			RequireSignature:   s.cfg.Spec.Trust.Verification.RequireSignature,
			RequireMerkleProof: s.cfg.Spec.Trust.Verification.RequireMerkleProof,
			CheckRevocation:    s.cfg.Spec.Trust.Verification.CheckRevocation,
			OCSPEnabled:        s.cfg.Spec.Trust.Verification.OCSP.Enabled,
			OCSPTimeout:        s.cfg.Spec.Trust.Verification.OCSP.Timeout,
			CRLEnabled:         s.cfg.Spec.Trust.Verification.CRL.Enabled,
			CRLCacheTimeout:    s.cfg.Spec.Trust.Verification.CRL.CacheTimeout,
			GracePeriod:        s.cfg.Spec.Trust.Verification.AllowExpiredGracePeriod,
		}
		verifier = trust.NewVerifier(trustProvider, verifierConfig)
		log.Info().Str("provider", s.cfg.Spec.Trust.Provider).Msg("Trust provider initialized")
	}

	// Determine verification mode
	verificationMode := "disabled"
	if s.cfg.Spec.Trust.Verification.Enabled {
		verificationMode = "strict"
	}

	// Create the core resolver
	lookupTimeout := 10 * time.Second
	if len(s.cfg.Spec.Registries) > 0 {
		lookupTimeout = s.cfg.Spec.Registries[0].Timeout
	}
	resolverCfg := resolver.Config{
		DefaultTTL:       s.cfg.Spec.Cache.TTL.Default,
		VerifiedTTL:      s.cfg.Spec.Cache.TTL.Verified,
		RevokedTTL:       s.cfg.Spec.Cache.TTL.Revoked,
		NotFoundTTL:      s.cfg.Spec.Cache.TTL.NotFound,
		VerificationMode: verificationMode,
		ParallelLookups:  10,
		LookupTimeout:    lookupTimeout,
	}
	s.resolver = resolver.NewResolver(s.cache, s.store, s.registry, verifier, resolverCfg)
	log.Info().Msg("Resolver initialized")

	return nil
}

// routes sets up the HTTP routes
func (s *Server) routes() http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(s.loggingMiddleware)

	// Rate limiting
	if s.cfg.Spec.RateLimit.Enabled {
		limits := s.cfg.Spec.RateLimit.Limits
		if anon, ok := limits["anonymous"]; ok {
			r.Use(httprate.LimitByIP(anon.Requests, anon.Window))
		}
	}

	// Health endpoints
	r.Get("/health", s.handleHealth)
	r.Get("/ready", s.handleReady)

	// Metrics endpoint
	if s.cfg.Spec.Telemetry.Metrics.Enabled {
		r.Handle("/metrics", telemetry.Handler())
	}

	// API v1
	r.Route("/v1", func(r chi.Router) {
		// Resolution endpoints
		r.Get("/resolve", s.handleResolve)
		r.Post("/resolve/batch", s.handleResolveBatch)

		// Agent info endpoints
		r.Get("/agent/{ansName}", s.handleGetAgent)
		r.Get("/agent/{ansName}/verify", s.handleVerifyAgent)
		r.Get("/agent/{ansName}/versions", s.handleListVersions)

		// Search/discovery
		r.Get("/search", s.handleSearch)
		r.Get("/discover", s.handleDiscover)

		// Stats
		r.Get("/stats", s.handleStats)
	})

	return r
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	log.Info().
		Str("addr", s.httpServer.Addr).
		Msg("Starting HTTP server")

	// Start metrics server if configured on different port
	if s.cfg.Spec.Telemetry.Metrics.Enabled &&
		s.cfg.Spec.Telemetry.Metrics.Port != s.cfg.Spec.Server.HTTP.Port {
		go s.startMetricsServer()
	}

	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) startMetricsServer() {
	mux := http.NewServeMux()
	mux.Handle(s.cfg.Spec.Telemetry.Metrics.Path, telemetry.Handler())

	addr := fmt.Sprintf("%s:%d",
		s.cfg.Spec.Telemetry.Metrics.Host,
		s.cfg.Spec.Telemetry.Metrics.Port,
	)

	log.Info().Str("addr", addr).Msg("Starting metrics server")

	srv := &http.Server{Addr: addr, Handler: mux}
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error().Err(err).Msg("Metrics server error")
	}
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	log.Info().Msg("Shutting down server...")

	// Close providers
	if s.cache != nil {
		s.cache.Close()
	}
	if s.store != nil {
		s.store.Close()
	}
	if s.registry != nil {
		s.registry.Close()
	}

	return s.httpServer.Shutdown(ctx)
}

// Middleware
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		duration := time.Since(start)

		log.Debug().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", ww.Status()).
			Dur("duration", duration).
			Int("bytes", ww.BytesWritten()).
			Msg("Request completed")

		// Record metrics
		s.metrics.RecordHTTPRequest(
			r.Method,
			r.URL.Path,
			ww.Status(),
			duration,
			int(r.ContentLength),
			ww.BytesWritten(),
		)
	})
}

// Handlers

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":  "healthy",
		"version": s.version,
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	// Check registry health
	healthy, err := s.registry.Healthy(r.Context())
	if err != nil || !healthy {
		s.jsonResponse(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status": "not ready",
			"error":  "registry unavailable",
		})
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status": "ready",
	})
}

func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()

	// Get ANSName from query
	nameStr := r.URL.Query().Get("name")
	if nameStr == "" {
		s.errorResponse(w, http.StatusBadRequest, "missing 'name' query parameter")
		return
	}

	// Parse ANSName
	name, err := ansname.Parse(nameStr)
	if err != nil {
		s.errorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid ANSName: %v", err))
		return
	}

	// Use the resolver to resolve the ANSName
	record, err := s.resolver.Resolve(ctx, name)
	if err != nil {
		var notFoundErr *resolver.ErrNotFound
		var verifyErr *resolver.ErrVerificationFailed
		if errors.As(err, &notFoundErr) {
			s.metrics.RecordResolution("not_found", name.Protocol, time.Since(start), false)
			s.errorResponse(w, http.StatusNotFound, "agent not found")
			return
		}
		if errors.As(err, &verifyErr) {
			s.metrics.RecordResolution("verification_failed", name.Protocol, time.Since(start), false)
			s.errorResponse(w, http.StatusUnprocessableEntity, fmt.Sprintf("verification failed: %s", verifyErr.Message))
			return
		}
		s.metrics.RecordResolution("error", name.Protocol, time.Since(start), false)
		s.errorResponse(w, http.StatusInternalServerError, fmt.Sprintf("resolution failed: %v", err))
		return
	}

	// Check status
	if record.Status == resolver.StatusRevoked {
		s.metrics.RecordResolution("revoked", name.Protocol, time.Since(start), false)
		s.errorResponse(w, http.StatusUnprocessableEntity, "certificate revoked")
		return
	}

	s.metrics.RecordResolution("success", name.Protocol, time.Since(start), false)

	s.jsonResponse(w, http.StatusOK, ResolutionResponse{
		Status:             record.Status,
		Agent:              record.ANSName,
		Protocol:           record.Protocol,
		Endpoint:           record.Endpoint,
		CertFingerprint:    record.CertFingerprint,
		ExpiresAt:          record.ExpiresAt.Format(time.RFC3339),
		ProtocolExtensions: record.ProtocolExtensions,
		Cached:             false, // The resolver handles caching internally
	})
}

func (s *Server) handleResolveBatch(w http.ResponseWriter, r *http.Request) {
	var req BatchResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	results := make(map[string]*ResolutionResponse)

	for _, nameStr := range req.Names {
		name, err := ansname.Parse(nameStr)
		if err != nil {
			results[nameStr] = &ResolutionResponse{
				Status: "error",
				Error:  fmt.Sprintf("invalid ANSName: %v", err),
			}
			continue
		}

		record, err := s.resolver.Resolve(ctx, name)
		if err != nil {
			status := "error"
			var notFoundErr *resolver.ErrNotFound
			var verifyErr *resolver.ErrVerificationFailed
			if errors.As(err, &notFoundErr) {
				status = "not_found"
			} else if errors.As(err, &verifyErr) {
				status = "verification_failed"
			}
			results[nameStr] = &ResolutionResponse{
				Status: status,
				Error:  err.Error(),
			}
			continue
		}

		results[nameStr] = &ResolutionResponse{
			Status:          "verified",
			Agent:           record.ANSName,
			Protocol:        record.Protocol,
			Endpoint:        record.Endpoint,
			CertFingerprint: record.CertFingerprint,
			ExpiresAt:       record.ExpiresAt.Format(time.RFC3339),
		}
	}

	s.jsonResponse(w, http.StatusOK, BatchResolveResponse{Results: results})
}

func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	ansName := chi.URLParam(r, "ansName")

	record, err := s.resolver.ResolveRaw(r.Context(), ansName)
	if err != nil {
		var notFoundErr *resolver.ErrNotFound
		if errors.As(err, &notFoundErr) {
			s.errorResponse(w, http.StatusNotFound, "agent not found")
			return
		}
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, record)
}

func (s *Server) handleVerifyAgent(w http.ResponseWriter, r *http.Request) {
	ansName := chi.URLParam(r, "ansName")

	name, err := ansname.Parse(ansName)
	if err != nil {
		s.errorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid ANSName: %v", err))
		return
	}

	// Use the resolver to verify the agent
	result, err := s.resolver.Verify(r.Context(), name)
	if err != nil {
		var notFoundErr *resolver.ErrNotFound
		if errors.As(err, &notFoundErr) {
			s.errorResponse(w, http.StatusNotFound, "agent not found")
			return
		}
		s.errorResponse(w, http.StatusInternalServerError, fmt.Sprintf("verification failed: %v", err))
		return
	}

	s.jsonResponse(w, http.StatusOK, VerifyResponse{
		Valid:      result.Valid,
		VerifiedAt: result.VerifiedAt.Format(time.RFC3339),
		Checks:     result.Checks,
	})
}

func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	ansName := chi.URLParam(r, "ansName")

	// Parse to get FQDN
	name, err := ansname.Parse(ansName)
	if err != nil {
		s.errorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid ANSName: %v", err))
		return
	}

	records, err := s.resolver.ListVersions(r.Context(), name.FQDN())
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"fqdn":     name.FQDN(),
		"versions": records,
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := &resolver.SearchQuery{
		Text:       r.URL.Query().Get("q"),
		Protocol:   r.URL.Query().Get("protocol"),
		Capability: r.URL.Query().Get("capability"),
		Status:     r.URL.Query().Get("status"),
		Limit:      100,
	}

	result, err := s.resolver.Search(r.Context(), query)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, result)
}

func (s *Server) handleDiscover(w http.ResponseWriter, r *http.Request) {
	// Discover is similar to search but focused on capabilities
	s.handleSearch(w, r)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.resolver.Stats(r.Context())
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"resolver": stats,
		"version":  s.version,
	})
}

// Response helpers

func (s *Server) jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) errorResponse(w http.ResponseWriter, status int, message string) {
	s.jsonResponse(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
		Code:    status,
	})
}

// Response types

type ResolutionResponse struct {
	Status             string                 `json:"status"`
	Agent              string                 `json:"agent,omitempty"`
	Protocol           string                 `json:"protocol,omitempty"`
	Endpoint           string                 `json:"endpoint,omitempty"`
	CertFingerprint    string                 `json:"certFingerprint,omitempty"`
	ExpiresAt          string                 `json:"expiresAt,omitempty"`
	ProtocolExtensions map[string]interface{} `json:"protocolExtensions,omitempty"`
	Cached             bool                   `json:"cached,omitempty"`
	Error              string                 `json:"error,omitempty"`
}

type BatchResolveRequest struct {
	Names []string `json:"names"`
}

type BatchResolveResponse struct {
	Results map[string]*ResolutionResponse `json:"results"`
}

type VerifyResponse struct {
	Valid      bool                             `json:"valid"`
	VerifiedAt string                           `json:"verifiedAt"`
	Checks     map[string]*resolver.CheckResult `json:"checks"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}
