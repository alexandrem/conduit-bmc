package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	baseconf "core/config"
	managerv1 "manager/gen/manager/v1"
	"manager/gen/manager/v1/managerv1connect"
	"manager/internal/database"
	"manager/internal/manager"
	"manager/internal/metrics"
	"manager/internal/webui"
	"manager/pkg/auth"
	"manager/pkg/config"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/protobuf/encoding/protojson"
)

func init() {
	// Configure zerolog for human-friendly console output
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.Kitchen})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

func main() {
	// Load configuration
	configFile := baseconf.FindConfigFile("manager")
	envFile := baseconf.FindEnvironmentFile("manager")

	cfg, err := config.Load(configFile, envFile)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Configure logging based on config
	cfg.Log.ConfigureZerolog()

	log.Info().Msg("Starting BMC Manager Service")
	log.Info().Str("config_file", configFile).Msg("Configuration loaded")
	log.Info().Str("env_file", envFile).Msg("Environment loaded")
	log.Info().
		Str("log_level", cfg.Log.Level).
		Bool("debug", cfg.Log.Debug).
		Msg("Log level configured")

	// Initialize database with debug option from config
	db, err := database.New(cfg.Database.DSN, database.WithDebug(cfg.Log.Debug))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer func(db *database.BunDB) {
		err := db.Close()
		if err != nil {
			log.Error().Err(err).Msg("Failed to close database connection")
		}
	}(db)

	// Initialize JWT manager
	jwtManager := auth.NewJWTManager(cfg.Auth.JWTSecretKey)

	// Log admin users
	if len(cfg.Auth.AdminEmails) > 0 {
		log.Info().Strs("admins", cfg.Auth.AdminEmails).Msg("Admin users configured")
	} else {
		log.Warn().Msg("No admin users configured - admin dashboard will be inaccessible")
	}

	// Initialize Connect handler
	managerHandler := manager.NewBMCManagerServiceHandler(db, jwtManager, cfg.Auth.AdminEmails)

	// Initialize Admin service handler
	adminHandler := manager.NewAdminServiceHandler(db, jwtManager)

	// Create interceptors
	interceptors := connect.WithInterceptors(managerHandler.AuthInterceptor())

	// Create admin interceptor (requires admin privileges)
	adminAuthInterceptor := auth.NewAdminAuthInterceptor(jwtManager)
	adminInterceptors := connect.WithInterceptors(adminAuthInterceptor)

	// Create the Connect service handler
	path, handler := managerv1connect.NewBMCManagerServiceHandler(
		managerHandler,
		interceptors,
	)

	// Create the Admin service handler
	adminPath, adminHandlerConnect := managerv1connect.NewAdminServiceHandler(
		adminHandler,
		adminInterceptors,
	)

	// Create HTTP mux
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	mux.Handle(adminPath, adminHandlerConnect)

	// Add health check endpoint (non-Connect)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "healthy"}`))
	})

	// Add status endpoint (wraps GetSystemStatus RPC)
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Call GetSystemStatus RPC handler
		rpcReq := connect.NewRequest(&managerv1.GetSystemStatusRequest{})
		rpcResp, err := managerHandler.GetSystemStatus(ctx, rpcReq)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get system status")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "Failed to get system status"}`))
			return
		}

		// Convert protobuf response to JSON
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Use protobuf's JSON marshaler for proper formatting
		jsonBytes, err := protojson.Marshal(rpcResp.Msg)
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal status response")
			w.Write([]byte(`{"error": "Failed to marshal response"}`))
			return
		}
		w.Write(jsonBytes)
	})

	// Add Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Add login page
	loginHandler := webui.NewLoginHandler()
	mux.Handle("/login", loginHandler)

	// Add logout handler
	logoutHandler := webui.NewLogoutHandler()
	mux.Handle("/logout", logoutHandler)

	// Add admin dashboard web UI
	adminDashboardHandler := webui.NewAdminDashboardHandler(jwtManager)
	mux.Handle("/admin", adminDashboardHandler)

	// Add CORS and metrics middleware for web clients
	corsHandler := addCORS(metrics.HTTPMetricsMiddleware(mux))

	// Start metrics collector for gauge metrics
	metricsCollector := metrics.NewCollector(db, 30*time.Second)
	ctx := context.Background()
	go metricsCollector.Start(ctx)
	defer metricsCollector.Stop()

	// Create server with HTTP/2 support
	server := &http.Server{
		Addr:           cfg.GetListenAddress(),
		Handler:        h2c.NewHandler(corsHandler, &http2.Server{}),
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	log.Info().
		Str("address", cfg.GetListenAddress()).
		Str("rpc_path", path).
		Str("admin_rpc_path", adminPath).
		Str("database", cfg.Database.Driver).
		Bool("rate_limiting", cfg.Manager.RateLimit.Enabled).
		Msg("Starting manager server")
	log.Info().Msgf("Health check: http://%s/health", cfg.GetListenAddress())
	log.Info().Msgf("System status: http://%s/status", cfg.GetListenAddress())
	log.Info().Msgf("Metrics: http://%s/metrics", cfg.GetListenAddress())
	log.Info().Msgf("Login page: http://%s/login", cfg.GetListenAddress())
	log.Info().Msgf("Admin dashboard: http://%s/admin", cfg.GetListenAddress())

	if err := server.ListenAndServe(); err != nil {
		log.Fatal().Err(err).Msg("Server failed to start")
	}
}

// CORS middleware to handle web clients
func addCORS(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, Connect-Protocol-Version, Connect-Timeout-Ms")
		w.Header().Set("Access-Control-Expose-Headers", "Connect-Protocol-Version, Connect-Timeout-Ms")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		handler.ServeHTTP(w, r)
	})
}
