package discovery

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"core/types"
	"local-agent/pkg/config"
	"local-agent/pkg/ipmi"
	"local-agent/pkg/redfish"
)

// Server represents a discovered BMC server with separate endpoint types
type Server struct {
	ID                string                   `json:"id"`
	CustomerID        string                   `json:"customer_id"`
	ControlEndpoints  []*BMCControlEndpoint    `json:"control_endpoints"`  // Multiple BMC control APIs (RFD 006)
	PrimaryProtocol   types.BMCType            `json:"primary_protocol"`   // Preferred protocol for operations (RFD 006)
	SOLEndpoint       *SOLEndpoint             `json:"sol_endpoint"`       // Serial-over-LAN (optional)
	VNCEndpoint       *VNCEndpoint             `json:"vnc_endpoint"`       // VNC/KVM access (optional)
	Features          []string                 `json:"features"`           // High-level features
	Status            string                   `json:"status"`             // "active", "inactive", etc.
	Metadata          map[string]string        `json:"metadata"`           // Additional metadata
	DiscoveryMetadata *types.DiscoveryMetadata `json:"discovery_metadata"` // Discovery metadata (RFD 017)
}

// GetPrimaryControlEndpoint returns the control endpoint matching PrimaryProtocol.
// If PrimaryProtocol is set and found, returns that endpoint.
// Otherwise, falls back to the first endpoint in the array.
// Returns nil if no endpoints are available.
func (s *Server) GetPrimaryControlEndpoint() *BMCControlEndpoint {
	if len(s.ControlEndpoints) == 0 {
		return nil
	}

	// If PrimaryProtocol is set, try to find matching endpoint
	if s.PrimaryProtocol != "" {
		for _, endpoint := range s.ControlEndpoints {
			if endpoint.Type == s.PrimaryProtocol {
				return endpoint
			}
		}
	}

	// Fallback to first endpoint
	return s.ControlEndpoints[0]
}

// BMCControlEndpoint represents BMC control API
type BMCControlEndpoint struct {
	Endpoint     string        `json:"endpoint"`
	Type         types.BMCType `json:"type"` // ipmi or redfish
	Username     string        `json:"username"`
	Password     string        `json:"password"`
	Capabilities []string      `json:"capabilities"`
	TLS          *TLSConfig    `json:"tls"`
}

type TLSConfig struct {
	Enabled            bool `json:"enabled"`
	InsecureSkipVerify bool `json:"insecure_skip_verify"`
}

// SOLEndpoint represents Serial-over-LAN access
type SOLEndpoint struct {
	Type     types.SOLType `json:"type"` // ipmi or redfish_serial
	Endpoint string        `json:"endpoint"`
	Username string        `json:"username"`
	Password string        `json:"password"`
}

// VNCEndpoint represents VNC/KVM access
type VNCEndpoint struct {
	Type     types.VNCType `json:"type"`     // native or websocket
	Endpoint string        `json:"endpoint"` // Full connection URL (e.g., "ws://novnc:6080/websockify")
	Username string        `json:"username"`
	Password string        `json:"password"`
	TLS      *TLSConfig    `json:"tls"` // Optional TLS configuration for VeNCrypt/RFB-over-TLS
}

// Service handles BMC discovery in the local datacenter
type Service struct {
	ipmiClient    *ipmi.Client
	redfishClient *redfish.Client
	config        *config.Config
}

func NewService(ipmiClient *ipmi.Client, redfishClient *redfish.Client, cfg *config.Config) *Service {
	return &Service{
		ipmiClient:    ipmiClient,
		redfishClient: redfishClient,
		config:        cfg,
	}
}

// DiscoverServers discovers all BMC endpoints combining static config and auto-discovery
func (s *Service) DiscoverServers(ctx context.Context) ([]*Server, error) {
	log.Info().Msg("Starting BMC discovery")

	var allServers []*Server

	// First, add statically configured servers
	staticServers := s.loadStaticServers()
	allServers = append(allServers, staticServers...)
	log.Info().Int("count", len(staticServers)).Msg("Loaded static BMC hosts")

	// Then perform auto-discovery if enabled
	if s.config.Agent.BMCDiscovery.Enabled {
		discoveredServers, err := s.performAutoDiscovery(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("Auto-discovery failed")
		} else {
			// Filter out any duplicates (static hosts that were also discovered)
			discoveredServers = s.filterDuplicates(allServers, discoveredServers)
			allServers = append(allServers, discoveredServers...)
			log.Info().Int("count", len(discoveredServers)).Msg("Auto-discovery found additional servers")
		}
	} else {
		log.Info().Msg("Auto-discovery disabled")
	}

	log.Info().
		Int("total", len(allServers)).
		Int("static", len(staticServers)).
		Int("discovered", len(allServers)-len(staticServers)).
		Msg("Discovery completed")
	return allServers, nil
}

// loadStaticServers converts configured static hosts to Server structs
func (s *Service) loadStaticServers() []*Server {
	var servers []*Server

	for _, host := range s.config.Static.Hosts {
		// Initialize metadata map if not present
		metadata := host.Metadata
		if metadata == nil {
			metadata = make(map[string]string)
		}

		server := &Server{
			ID:         host.ID,
			CustomerID: host.CustomerID,
			Features:   host.Features,
			Status:     "configured", // Mark as configured vs discovered
			Metadata:   metadata,
		}

		// Convert control endpoints
		if len(host.ControlEndpoints) > 0 {
			server.ControlEndpoints = make([]*BMCControlEndpoint, len(host.ControlEndpoints))
			for i, endpoint := range host.ControlEndpoints {
				server.ControlEndpoints[i] = &BMCControlEndpoint{
					Endpoint:     endpoint.Endpoint,
					Type:         endpoint.InferType(), // Infer from endpoint if not specified
					Username:     endpoint.Username,
					Password:     endpoint.Password,
					Capabilities: endpoint.Capabilities,
				}
			}
			// Set primary protocol to first endpoint's type
			if len(server.ControlEndpoints) > 0 {
				server.PrimaryProtocol = server.GetPrimaryControlEndpoint().Type
			}
		}

		// Convert SOL endpoint
		if host.SOLEndpoint != nil {
			server.SOLEndpoint = &SOLEndpoint{
				Type:     host.SOLEndpoint.InferType(), // Infer from endpoint if not specified
				Endpoint: host.SOLEndpoint.Endpoint,
				Username: host.SOLEndpoint.Username,
				Password: host.SOLEndpoint.Password,
			}
		}

		// Convert VNC endpoint
		if host.VNCEndpoint != nil {
			vncEndpoint := &VNCEndpoint{
				Type:     host.VNCEndpoint.InferType(), // Infer from endpoint scheme if not specified
				Endpoint: host.VNCEndpoint.Endpoint,
				Username: host.VNCEndpoint.Username,
				Password: host.VNCEndpoint.Password,
			}

			// Copy TLS configuration if present
			if host.VNCEndpoint.TLS != nil {
				vncEndpoint.TLS = &TLSConfig{
					Enabled:            host.VNCEndpoint.TLS.Enabled,
					InsecureSkipVerify: host.VNCEndpoint.TLS.InsecureSkipVerify,
				}
			}

			server.VNCEndpoint = vncEndpoint
		}

		// If Redfish, perform API discovery if enabled
		// Check primary endpoint (first in list) for Redfish protocol
		if len(server.ControlEndpoints) > 0 && server.GetPrimaryControlEndpoint().Type == "redfish" {
			endpoint := server.GetPrimaryControlEndpoint().Endpoint
			info, err := s.redfishClient.DiscoverSerialConsole(context.Background(), endpoint, server.GetPrimaryControlEndpoint().Username, server.GetPrimaryControlEndpoint().Password)
			if err != nil {
				log.Warn().Err(err).Str("endpoint", endpoint).Msg("Failed to discover SerialConsole for static server")
				server.Metadata["discovery_error"] = err.Error()
			} else {
				// Store vendor information
				server.Metadata["vendor"] = string(info.Vendor)

				// Log discovery results for debugging
				log.Debug().
					Str("endpoint", endpoint).
					Str("vendor", string(info.Vendor)).
					Bool("supported", info.Supported).
					Bool("fallbackToIPMI", info.FallbackToIPMI).
					Str("serialPath", info.SerialPath).
					Msg("Serial console discovery results")

				// Configure SOL endpoint based on discovery
				// Always override inferred/configured SOL endpoints with actual discovery results
				// This ensures vendor-specific behavior (like iDRAC requiring IPMI fallback) is respected
				if info.Supported && info.SerialPath != "" {
					// Use Redfish serial console if supported
					server.SOLEndpoint = &SOLEndpoint{
						Type:     "redfish_serial",
						Endpoint: endpoint + info.SerialPath,
						Username: server.GetPrimaryControlEndpoint().Username,
						Password: server.GetPrimaryControlEndpoint().Password,
					}
					log.Info().Str("endpoint", endpoint).Str("vendor", string(info.Vendor)).Msg("Using Redfish serial console")
				} else if info.FallbackToIPMI {
					// Fallback to IPMI SOL
					log.Debug().Str("endpoint", endpoint).Msg("Attempting to build IPMI endpoint for fallback")
					ipmiEndpoint, err := s.buildIPMIEndpoint(endpoint)
					if err != nil {
						log.Warn().Err(err).Str("endpoint", endpoint).Msg("Failed to build IPMI endpoint")
					} else {
						log.Debug().Str("ipmiEndpoint", ipmiEndpoint).Msg("Built IPMI endpoint successfully")
						server.SOLEndpoint = &SOLEndpoint{
							Type:     "ipmi",
							Endpoint: ipmiEndpoint,
							Username: server.GetPrimaryControlEndpoint().Username,
							Password: server.GetPrimaryControlEndpoint().Password,
						}
						server.Metadata["sol_fallback"] = "ipmi"
						log.Info().
							Str("endpoint", endpoint).
							Str("ipmiEndpoint", ipmiEndpoint).
							Str("vendor", string(info.Vendor)).
							Msg("Using IPMI SOL fallback")
					}
				} else {
					// No console support detected, clear any inferred SOL endpoint
					server.SOLEndpoint = nil
					log.Warn().Str("endpoint", endpoint).Str("vendor", string(info.Vendor)).Msg("No serial console support detected")
				}

				// Ensure FeatureConsole is included if supported or fallback
				if info.Supported || info.FallbackToIPMI {
					hasConsole := false
					for _, f := range server.Features {
						if f == string(types.FeatureConsole) {
							hasConsole = true
							break
						}
					}
					if !hasConsole {
						server.Features = append(server.Features, string(types.FeatureConsole))
					}
				}
			}
		}

		// Build discovery metadata for static configuration
		discoveryMetadata := s.buildDiscoveryMetadata(server, types.DiscoveryMethodStaticConfig, "config.yaml")
		discoveryMetadata.DiscoveredAt = time.Now()
		server.DiscoveryMetadata = discoveryMetadata

		servers = append(servers, server)

		vncEndpoint := "none"
		if server.VNCEndpoint != nil {
			vncEndpoint = server.VNCEndpoint.Endpoint
		}

		log.Debug().
			Str("host_id", host.ID).
			Str("control", host.GetControlEndpoint()).
			Str("sol", host.GetSOLEndpoint()).
			Str("vnc", vncEndpoint).
			Msg("Loaded static BMC host")
	}

	return servers
}

// performAutoDiscovery runs the original auto-discovery logic
func (s *Service) performAutoDiscovery(ctx context.Context) ([]*Server, error) {
	var allServers []*Server

	// Get list of network interfaces and subnets to scan
	subnets, err := s.getLocalSubnets()
	if err != nil {
		return nil, fmt.Errorf("failed to get local subnets: %w", err)
	}

	for _, subnet := range subnets {
		log.Info().Str("subnet", subnet).Msg("Scanning subnet")

		// Discover IPMI BMCs
		ipmiServers, err := s.discoverIPMI(ctx, subnet)
		if err != nil {
			log.Warn().Str("subnet", subnet).Err(err).Msg("IPMI discovery failed")
		} else {
			allServers = append(allServers, ipmiServers...)
		}

		// Discover Redfish BMCs
		redfishServers, err := s.discoverRedfish(ctx, subnet)
		if err != nil {
			log.Warn().Str("subnet", subnet).Err(err).Msg("Redfish discovery failed")
		} else {
			allServers = append(allServers, redfishServers...)
		}
	}

	return allServers, nil
}

// filterDuplicates removes discovered servers that match static configuration
func (s *Service) filterDuplicates(staticServers, discoveredServers []*Server) []*Server {
	var filtered []*Server

	// Create a map of static control endpoints for quick lookup
	staticEndpoints := make(map[string]bool)
	for _, server := range staticServers {
		if len(server.ControlEndpoints) > 0 {
			staticEndpoints[server.GetPrimaryControlEndpoint().Endpoint] = true
		}
	}

	// Only include discovered servers that don't match static config
	for _, server := range discoveredServers {
		controlEndpoint := ""
		if len(server.ControlEndpoints) > 0 {
			controlEndpoint = server.GetPrimaryControlEndpoint().Endpoint
		}
		if !staticEndpoints[controlEndpoint] {
			filtered = append(filtered, server)
		} else {
			log.Debug().Str("endpoint", controlEndpoint).Msg("Skipping discovered server (already configured statically)")
		}
	}

	return filtered
}

// discoverIPMI discovers IPMI-enabled BMCs in a subnet
func (s *Service) discoverIPMI(ctx context.Context, subnet string) ([]*Server, error) {
	log.Debug().Str("subnet", subnet).Msg("Discovering IPMI BMCs")

	var servers []*Server

	// Parse subnet to get IP range
	_, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		return nil, fmt.Errorf("invalid subnet: %w", err)
	}

	// Scan common IPMI ports (623/udp is standard)
	ips := s.generateIPsFromSubnet(ipnet)
	for _, ip := range ips {
		select {
		case <-ctx.Done():
			return servers, ctx.Err()
		default:
		}

		// Test for IPMI on port 623
		endpoint := fmt.Sprintf("%s:623", ip.String())
		if s.ipmiClient.IsAccessible(ctx, endpoint) {
			server := &Server{
				ID:         fmt.Sprintf("server-%s", strings.ReplaceAll(ip.String(), ".", "-")),
				CustomerID: "customer-1", // TODO: Determine customer ownership
				ControlEndpoints: []*BMCControlEndpoint{
					{
						Endpoint:     endpoint,
						Type:         "ipmi",
						Username:     "admin",    // Default credentials
						Password:     "password", // Default credentials
						Capabilities: types.CapabilitiesToStrings(types.IPMICapabilities()),
					},
				},
				PrimaryProtocol: "ipmi",
				Features: types.FeaturesToStrings([]types.Feature{
					types.FeaturePower,
					types.FeatureConsole,
					types.FeatureVNC,
					types.FeatureSensors,
				}),
				Status:   "active",
				Metadata: make(map[string]string),
			}
			servers = append(servers, server)
			log.Info().Str("endpoint", endpoint).Msg("Found IPMI BMC")
		}
	}

	return servers, nil
}

// discoverRedfish discovers Redfish-enabled BMCs in a subnet
func (s *Service) discoverRedfish(ctx context.Context, subnet string) ([]*Server, error) {
	log.Debug().Str("subnet", subnet).Msg("Discovering Redfish BMCs")

	var servers []*Server

	// Parse subnet to get IP range
	_, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		return nil, fmt.Errorf("invalid subnet: %w", err)
	}

	// Scan common Redfish ports (443/tcp, 8443/tcp)
	ips := s.generateIPsFromSubnet(ipnet)
	redfishPorts := []int{443, 8443, 8080}

	for _, ip := range ips {
		select {
		case <-ctx.Done():
			return servers, ctx.Err()
		default:
		}

		for _, port := range redfishPorts {
			endpoint := fmt.Sprintf("https://%s:%d", ip.String(), port)
			if s.redfishClient.IsAccessible(ctx, endpoint) {
				server := &Server{
					ID:         fmt.Sprintf("server-%s-%d", strings.ReplaceAll(ip.String(), ".", "-"), port),
					CustomerID: "customer-1", // TODO: Determine customer ownership
					ControlEndpoints: []*BMCControlEndpoint{
						{
							Endpoint:     endpoint,
							Type:         "redfish",
							Username:     "admin",    // Default credentials
							Password:     "password", // Default credentials
							Capabilities: types.CapabilitiesToStrings(types.RedfishCapabilities()),
						},
					},
					PrimaryProtocol: "redfish",
					Features: types.FeaturesToStrings([]types.Feature{
						types.FeaturePower,
						types.FeatureConsole,
						types.FeatureVNC,
						types.FeatureSensors,
					}),
					Status:   "active",
					Metadata: make(map[string]string),
				}

				// Perform API discovery
				info, err := s.redfishClient.DiscoverSerialConsole(ctx, endpoint, server.GetPrimaryControlEndpoint().Username, server.GetPrimaryControlEndpoint().Password)
				if err != nil {
					log.Warn().Err(err).Str("endpoint", endpoint).Msg("Failed to discover SerialConsole")
					server.Metadata["discovery_error"] = err.Error()
				} else if info.Supported {
					server.SOLEndpoint = &SOLEndpoint{
						Type:     "redfish_serial",
						Endpoint: endpoint + "/redfish/v1/Managers/1/SerialConsole", // Adjust based on actual path
						Username: server.GetPrimaryControlEndpoint().Username,
						Password: server.GetPrimaryControlEndpoint().Password,
					}
					// Ensure FeatureConsole is included
					hasConsole := false
					for _, f := range server.Features {
						if f == string(types.FeatureConsole) {
							hasConsole = true
							break
						}
					}
					if !hasConsole {
						server.Features = append(server.Features, string(types.FeatureConsole))
					}
				}

				servers = append(servers, server)
				log.Info().Str("endpoint", endpoint).Msg("Found Redfish BMC")
				break // Found Redfish on this IP, no need to check other ports
			}
		}
	}

	return servers, nil
}

// getLocalSubnets returns a list of local subnets to scan for BMCs
func (s *Service) getLocalSubnets() ([]string, error) {
	// If subnets are explicitly configured, use those
	if len(s.config.Agent.BMCDiscovery.NetworkRanges) > 0 {
		log.Info().Strs("subnets", s.config.Agent.BMCDiscovery.NetworkRanges).Msg("Using configured subnets")
		return s.config.Agent.BMCDiscovery.NetworkRanges, nil
	}

	// Otherwise, auto-detect local subnets
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get network interfaces: %w", err)
	}

	var subnets []string

	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			log.Debug().Str("interface", iface.Name).Err(err).Msg("Failed to get interface addresses")
			continue
		}

		for _, addr := range addrs {
			var ipnet *net.IPNet
			switch v := addr.(type) {
			case *net.IPNet:
				ipnet = v
			case *net.IPAddr:
				ipnet = &net.IPNet{IP: v.IP, Mask: v.IP.DefaultMask()}
			}

			// Only consider IPv4 addresses
			if ipnet != nil && ipnet.IP.To4() != nil {
				// Skip local/private management subnets commonly used for BMCs
				// Look for typical BMC subnets (e.g., 192.168.x.x/24, 10.x.x.x/24)
				if s.isBMCSubnet(ipnet) {
					subnets = append(subnets, ipnet.String())
				}
			}
		}
	}

	// If no suitable subnets found, add some common BMC subnets
	if len(subnets) == 0 {
		log.Info().Msg("No suitable subnets found, using defaults")
		subnets = []string{
			"192.168.1.0/24",  // Common home/small office
			"192.168.10.0/24", // Common BMC subnet
			"10.0.1.0/24",     // Common enterprise BMC
		}
	}

	return subnets, nil
}

// isBMCSubnet checks if a subnet is likely to contain BMCs
func (s *Service) isBMCSubnet(ipnet *net.IPNet) bool {
	ip := ipnet.IP.To4()
	if ip == nil {
		return false
	}

	// Look for private IP ranges that might contain BMCs
	return (ip[0] == 192 && ip[1] == 168) || // 192.168.x.x
		(ip[0] == 10) || // 10.x.x.x
		(ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31) // 172.16-31.x.x
}

// buildIPMIEndpoint converts a Redfish endpoint URL to an IPMI endpoint
func (s *Service) buildIPMIEndpoint(redfishEndpoint string) (string, error) {
	u, err := url.Parse(redfishEndpoint)
	if err != nil {
		return "", fmt.Errorf("failed to parse endpoint: %w", err)
	}

	// Extract host without port
	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		// No port specified, use the host as-is
		host = u.Host
	}

	// Standard IPMI port is 623
	return host + ":623", nil
}

// generateIPsFromSubnet generates a list of IPs to scan in a subnet
func (s *Service) generateIPsFromSubnet(ipnet *net.IPNet) []net.IP {
	var ips []net.IP

	// For performance, limit scanning to first 254 IPs
	// In production, this would be more sophisticated
	ip := ipnet.IP.To4()
	if ip == nil {
		return ips
	}

	// Create base IP for iteration
	base := make(net.IP, 4)
	copy(base, ip)

	// Scan the last octet (simplified approach)
	for i := 1; i <= 254; i++ {
		scanIP := make(net.IP, 4)
		copy(scanIP, base)
		scanIP[3] = byte(i)

		if ipnet.Contains(scanIP) {
			ips = append(ips, scanIP)
		}

		// Limit to prevent excessive scanning
		if len(ips) >= 100 {
			break
		}
	}

	return ips
}

// buildDiscoveryMetadata constructs discovery metadata for a server
func (s *Service) buildDiscoveryMetadata(server *Server, discoveryMethod types.DiscoveryMethod, configSource string) *types.DiscoveryMetadata {
	metadata := &types.DiscoveryMetadata{
		DiscoveryMethod: discoveryMethod,
		DiscoveredAt:    time.Time{}, // Will be set by caller with time.Now()
		DiscoverySource: s.config.Agent.ID,
		ConfigSource:    configSource,
		AdditionalInfo:  make(map[string]string),
	}

	// Build protocol configuration
	if len(server.ControlEndpoints) > 0 {
		protocol := &types.ProtocolConfig{
			PrimaryProtocol: string(server.GetPrimaryControlEndpoint().Type),
		}

		// Add protocol version if known
		if server.GetPrimaryControlEndpoint().Type == "ipmi" {
			protocol.PrimaryVersion = "2.0" // Assume IPMI 2.0
		}

		// Check for fallback configuration
		if val, ok := server.Metadata["sol_fallback"]; ok && val == "ipmi" {
			protocol.FallbackProtocol = "ipmi"
			protocol.FallbackReason = fmt.Sprintf("Vendor %s requires IPMI fallback for console", server.Metadata["vendor"])
		}

		// Set console type
		if server.SOLEndpoint != nil {
			protocol.ConsoleType = string(server.SOLEndpoint.Type)
			// If it's a Redfish serial console, try to extract the path
			if server.SOLEndpoint.Type == "redfish_serial" && strings.Contains(server.SOLEndpoint.Endpoint, "/redfish/") {
				u, err := url.Parse(server.SOLEndpoint.Endpoint)
				if err == nil {
					protocol.ConsolePath = u.Path
				}
			}
		}

		// Set VNC transport
		if server.VNCEndpoint != nil {
			protocol.VNCTransport = string(server.VNCEndpoint.Type)
		}

		metadata.Protocol = protocol
	}

	// Build endpoint details
	if len(server.ControlEndpoints) > 0 {
		endpoints := &types.EndpointDetails{
			ControlEndpoint: server.GetPrimaryControlEndpoint().Endpoint,
		}

		// Parse control endpoint URL to extract scheme and port
		if u, err := url.Parse(server.GetPrimaryControlEndpoint().Endpoint); err == nil {
			endpoints.ControlScheme = u.Scheme
			if portStr := u.Port(); portStr != "" {
				if port, err := fmt.Sscanf(portStr, "%d", &endpoints.ControlPort); err == nil && port == 1 {
					// Port extracted successfully
				}
			}
		}

		if server.SOLEndpoint != nil {
			endpoints.ConsoleEndpoint = server.SOLEndpoint.Endpoint
		}

		if server.VNCEndpoint != nil {
			endpoints.VNCEndpoint = server.VNCEndpoint.Endpoint
		}

		metadata.Endpoints = endpoints
	}

	// Build security configuration
	security := &types.SecurityConfig{
		AuthMethod: "basic", // Default to basic auth
	}

	if len(server.ControlEndpoints) > 0 && server.GetPrimaryControlEndpoint().TLS != nil {
		security.TLSEnabled = server.GetPrimaryControlEndpoint().TLS.Enabled
		security.TLSVerify = !server.GetPrimaryControlEndpoint().TLS.InsecureSkipVerify
	}

	if server.VNCEndpoint != nil && server.VNCEndpoint.Password != "" {
		security.VNCAuthType = "password"
		security.VNCPasswordLength = int32(len(server.VNCEndpoint.Password))
	}

	metadata.Security = security

	// Build network information
	if len(server.ControlEndpoints) > 0 {
		network := &types.NetworkInfo{
			Reachable: true, // If we got here, it's reachable
		}

		// Try to extract IP address from endpoint
		if u, err := url.Parse(server.GetPrimaryControlEndpoint().Endpoint); err == nil {
			host := u.Hostname()
			if host != "" {
				network.IPAddress = host
			}
		} else if strings.Contains(server.GetPrimaryControlEndpoint().Endpoint, ":") {
			// Might be IP:port format
			host, _, err := net.SplitHostPort(server.GetPrimaryControlEndpoint().Endpoint)
			if err == nil {
				network.IPAddress = host
			}
		}

		metadata.Network = network
	}

	// Build capability information
	capabilities := &types.CapabilityInfo{
		SupportedFeatures: server.Features,
	}

	if discoveryError, ok := server.Metadata["discovery_error"]; ok {
		capabilities.DiscoveryErrors = []string{discoveryError}
	}

	metadata.Capabilities = capabilities

	// Build vendor information from metadata if available
	if vendor, ok := server.Metadata["vendor"]; ok && vendor != "" {
		metadata.Vendor = &types.VendorInfo{
			Manufacturer: vendor,
		}
	}

	return metadata
}
