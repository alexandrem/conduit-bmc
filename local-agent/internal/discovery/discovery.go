package discovery

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/rs/zerolog/log"

	"core/types"
	"local-agent/pkg/config"
	"local-agent/pkg/ipmi"
	"local-agent/pkg/redfish"
)

// Server represents a discovered BMC server with separate endpoint types
type Server struct {
	ID              string              `json:"id"`
	CustomerID      string              `json:"customer_id"`
	ControlEndpoint *BMCControlEndpoint `json:"control_endpoint"` // BMC control API
	SOLEndpoint     *SOLEndpoint        `json:"sol_endpoint"`     // Serial-over-LAN (optional)
	VNCEndpoint     *VNCEndpoint        `json:"vnc_endpoint"`     // VNC/KVM access (optional)
	Features        []string            `json:"features"`         // High-level features
	Status          string              `json:"status"`           // "active", "inactive", etc.
	Metadata        map[string]string   `json:"metadata"`         // Additional metadata
}

// BMCControlEndpoint represents BMC control API
type BMCControlEndpoint struct {
	Endpoint     string   `json:"endpoint"`
	Type         string   `json:"type"` // "ipmi" or "redfish"
	Username     string   `json:"username"`
	Password     string   `json:"password"`
	Capabilities []string `json:"capabilities"`
}

// SOLEndpoint represents Serial-over-LAN access
type SOLEndpoint struct {
	Type     string `json:"type"` // "ipmi" or "redfish_serial"
	Endpoint string `json:"endpoint"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// VNCEndpoint represents VNC/KVM access
type VNCEndpoint struct {
	Type     string `json:"type"`     // "bmc_native", "novnc_proxy", "external_kvm"
	Endpoint string `json:"endpoint"` // Full connection URL (e.g., "ws://novnc:6080/websockify")
	Username string `json:"username"`
	Password string `json:"password"`
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
		server := &Server{
			ID:         host.ID,
			CustomerID: host.CustomerID,
			Features:   host.Features,
			Status:     "configured", // Mark as configured vs discovered
			Metadata:   host.Metadata,
		}

		// Convert control endpoint
		if host.ControlEndpoint != nil {
			server.ControlEndpoint = &BMCControlEndpoint{
				Endpoint:     host.ControlEndpoint.Endpoint,
				Type:         host.ControlEndpoint.Type,
				Username:     host.ControlEndpoint.Username,
				Password:     host.ControlEndpoint.Password,
				Capabilities: host.ControlEndpoint.Capabilities,
			}
		}

		// Convert SOL endpoint
		if host.SOLEndpoint != nil {
			server.SOLEndpoint = &SOLEndpoint{
				Type:     host.SOLEndpoint.Type,
				Endpoint: host.SOLEndpoint.Endpoint,
				Username: host.SOLEndpoint.Username,
				Password: host.SOLEndpoint.Password,
			}
		}

		// Convert VNC endpoint
		if host.VNCEndpoint != nil {
			server.VNCEndpoint = &VNCEndpoint{
				Type:     host.VNCEndpoint.Type,
				Endpoint: host.VNCEndpoint.Endpoint,
				Username: host.VNCEndpoint.Username,
				Password: host.VNCEndpoint.Password,
			}
		}

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
		if server.ControlEndpoint != nil {
			staticEndpoints[server.ControlEndpoint.Endpoint] = true
		}
	}

	// Only include discovered servers that don't match static config
	for _, server := range discoveredServers {
		controlEndpoint := ""
		if server.ControlEndpoint != nil {
			controlEndpoint = server.ControlEndpoint.Endpoint
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
				ControlEndpoint: &BMCControlEndpoint{
					Endpoint:     endpoint,
					Type:         "ipmi",
					Username:     "admin",    // Default credentials
					Password:     "password", // Default credentials
					Capabilities: types.CapabilitiesToStrings(types.IPMICapabilities()),
				},
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
					ControlEndpoint: &BMCControlEndpoint{
						Endpoint:     endpoint,
						Type:         "redfish",
						Username:     "admin",    // Default credentials
						Password:     "password", // Default credentials
						Capabilities: types.CapabilitiesToStrings(types.RedfishCapabilities()),
					},
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
