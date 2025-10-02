package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	baseconf "core/config"
	"local-agent/internal/agent"
	"local-agent/internal/discovery"
	"local-agent/pkg/bmc"
	"local-agent/pkg/config"
	"local-agent/pkg/ipmi"
	"local-agent/pkg/redfish"
)

func init() {
	// Configure zerolog for human-friendly console output
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.Kitchen})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

func main() {
	// Parse command line flags
	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to configuration file")
	flag.Parse()

	// Load configuration using standardized discovery
	var configFile, envFile string
	if configPath != "" {
		// If config path was explicitly provided, verify it exists
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			log.Fatal().
				Str("config_path", configPath).
				Msg("Configuration file does not exist")
		} else if err != nil {
			log.Fatal().
				Err(err).
				Str("config_path", configPath).
				Msg("Failed to access configuration file")
		}
		configFile = configPath
	} else {
		configFile = baseconf.FindConfigFile("agent")
	}
	envFile = baseconf.FindEnvironmentFile("agent")

	cfg, err := config.Load(configFile, envFile)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Configure logging based on config
	cfg.Log.ConfigureZerolog()

	log.Info().Msg("Starting BMC Local Agent Service")
	log.Info().Str("config_file", configFile).Msg("Configuration loaded")
	log.Info().Str("env_file", envFile).Msg("Environment loaded")
	log.Info().
		Str("log_level", cfg.Log.Level).
		Bool("debug", cfg.Log.Debug).
		Msg("Log level configured")
	log.Info().
		Str("agent_id", cfg.Agent.ID).
		Str("datacenter_id", cfg.Agent.DatacenterID).
		Str("gateway_endpoint", cfg.Agent.GatewayEndpoint).
		Str("agent_endpoint", cfg.Agent.Endpoint).
		Bool("discovery_enabled", cfg.Agent.BMCDiscovery.Enabled).
		Int("static_hosts", len(cfg.Static.Hosts)).
		Msg("Agent configuration")

	// Initialize BMC clients
	ipmiClient := ipmi.NewClient()
	redfishClient := redfish.NewClient()

	// Initialize BMC client wrapper for power operations
	bmcClient := bmc.NewClient(ipmiClient, redfishClient)

	// Initialize discovery service with configuration
	discoveryService := discovery.NewService(ipmiClient, redfishClient, cfg)

	// Initialize agent
	localAgent := agent.NewLocalAgent(cfg, discoveryService, bmcClient)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start agent in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := localAgent.Start(ctx); err != nil {
			errChan <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		log.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
		cancel()
	case err := <-errChan:
		log.Error().Err(err).Msg("Agent encountered an error")
		cancel()
	}

	// Give some time for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := localAgent.Stop(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error during shutdown")
	}

	log.Info().Msg("Local Agent stopped")
}
