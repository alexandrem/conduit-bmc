package types

import (
	commonv1 "core/gen/common/v1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// ConvertToProto converts types.DiscoveryMetadata to proto DiscoveryMetadata
func (dm *DiscoveryMetadata) ConvertToProto() *commonv1.DiscoveryMetadata {
	if dm == nil {
		return nil
	}

	proto := &commonv1.DiscoveryMetadata{
		DiscoveryMethod: convertDiscoveryMethodToProto(dm.DiscoveryMethod),
		DiscoverySource: dm.DiscoverySource,
		ConfigSource:    dm.ConfigSource,
		AdditionalInfo:  dm.AdditionalInfo,
	}

	if !dm.DiscoveredAt.IsZero() {
		proto.DiscoveredAt = timestamppb.New(dm.DiscoveredAt)
	}

	if dm.Vendor != nil {
		proto.Vendor = &commonv1.VendorInfo{
			Manufacturer:    dm.Vendor.Manufacturer,
			Model:           dm.Vendor.Model,
			FirmwareVersion: dm.Vendor.FirmwareVersion,
			BmcVersion:      dm.Vendor.BMCVersion,
		}
	}

	if dm.Protocol != nil {
		proto.Protocol = &commonv1.ProtocolConfig{
			PrimaryProtocol:  dm.Protocol.PrimaryProtocol,
			PrimaryVersion:   dm.Protocol.PrimaryVersion,
			FallbackProtocol: dm.Protocol.FallbackProtocol,
			FallbackReason:   dm.Protocol.FallbackReason,
			ConsoleType:      dm.Protocol.ConsoleType,
			ConsolePath:      dm.Protocol.ConsolePath,
			VncTransport:     dm.Protocol.VNCTransport,
		}
	}

	if dm.Endpoints != nil {
		proto.Endpoints = &commonv1.EndpointDetails{
			ControlEndpoint: dm.Endpoints.ControlEndpoint,
			ControlScheme:   dm.Endpoints.ControlScheme,
			ControlPort:     dm.Endpoints.ControlPort,
			ConsoleEndpoint: dm.Endpoints.ConsoleEndpoint,
			VncEndpoint:     dm.Endpoints.VNCEndpoint,
			VncDisplay:      dm.Endpoints.VNCDisplay,
		}
	}

	if dm.Security != nil {
		proto.Security = &commonv1.SecurityConfig{
			TlsEnabled:        dm.Security.TLSEnabled,
			TlsVerify:         dm.Security.TLSVerify,
			AuthMethod:        dm.Security.AuthMethod,
			VncAuthType:       dm.Security.VNCAuthType,
			VncPasswordLength: dm.Security.VNCPasswordLength,
			IpmiCipherSuite:   dm.Security.IPMICipherSuite,
		}
	}

	if dm.Network != nil {
		proto.Network = &commonv1.NetworkInfo{
			IpAddress:      dm.Network.IPAddress,
			MacAddress:     dm.Network.MACAddress,
			NetworkSegment: dm.Network.NetworkSegment,
			VlanId:         dm.Network.VLANId,
			Reachable:      dm.Network.Reachable,
			LatencyMs:      dm.Network.LatencyMs,
		}
	}

	if dm.Capabilities != nil {
		proto.Capabilities = &commonv1.CapabilityInfo{
			SupportedFeatures:   dm.Capabilities.SupportedFeatures,
			UnsupportedFeatures: dm.Capabilities.UnsupportedFeatures,
			DiscoveryErrors:     dm.Capabilities.DiscoveryErrors,
			DiscoveryWarnings:   dm.Capabilities.DiscoveryWarnings,
		}
	}

	return proto
}

// ConvertFromProto converts proto DiscoveryMetadata to types.DiscoveryMetadata
func ConvertDiscoveryMetadataFromProto(proto *commonv1.DiscoveryMetadata) *DiscoveryMetadata {
	if proto == nil {
		return nil
	}

	dm := &DiscoveryMetadata{
		DiscoveryMethod: convertDiscoveryMethodFromProto(proto.DiscoveryMethod),
		DiscoverySource: proto.DiscoverySource,
		ConfigSource:    proto.ConfigSource,
		AdditionalInfo:  proto.AdditionalInfo,
	}

	if proto.DiscoveredAt != nil {
		dm.DiscoveredAt = proto.DiscoveredAt.AsTime()
	}

	if proto.Vendor != nil {
		dm.Vendor = &VendorInfo{
			Manufacturer:    proto.Vendor.Manufacturer,
			Model:           proto.Vendor.Model,
			FirmwareVersion: proto.Vendor.FirmwareVersion,
			BMCVersion:      proto.Vendor.BmcVersion,
		}
	}

	if proto.Protocol != nil {
		dm.Protocol = &ProtocolConfig{
			PrimaryProtocol:  proto.Protocol.PrimaryProtocol,
			PrimaryVersion:   proto.Protocol.PrimaryVersion,
			FallbackProtocol: proto.Protocol.FallbackProtocol,
			FallbackReason:   proto.Protocol.FallbackReason,
			ConsoleType:      proto.Protocol.ConsoleType,
			ConsolePath:      proto.Protocol.ConsolePath,
			VNCTransport:     proto.Protocol.VncTransport,
		}
	}

	if proto.Endpoints != nil {
		dm.Endpoints = &EndpointDetails{
			ControlEndpoint: proto.Endpoints.ControlEndpoint,
			ControlScheme:   proto.Endpoints.ControlScheme,
			ControlPort:     proto.Endpoints.ControlPort,
			ConsoleEndpoint: proto.Endpoints.ConsoleEndpoint,
			VNCEndpoint:     proto.Endpoints.VncEndpoint,
			VNCDisplay:      proto.Endpoints.VncDisplay,
		}
	}

	if proto.Security != nil {
		dm.Security = &SecurityConfig{
			TLSEnabled:        proto.Security.TlsEnabled,
			TLSVerify:         proto.Security.TlsVerify,
			AuthMethod:        proto.Security.AuthMethod,
			VNCAuthType:       proto.Security.VncAuthType,
			VNCPasswordLength: proto.Security.VncPasswordLength,
			IPMICipherSuite:   proto.Security.IpmiCipherSuite,
		}
	}

	if proto.Network != nil {
		dm.Network = &NetworkInfo{
			IPAddress:      proto.Network.IpAddress,
			MACAddress:     proto.Network.MacAddress,
			NetworkSegment: proto.Network.NetworkSegment,
			VLANId:         proto.Network.VlanId,
			Reachable:      proto.Network.Reachable,
			LatencyMs:      proto.Network.LatencyMs,
		}
	}

	if proto.Capabilities != nil {
		dm.Capabilities = &CapabilityInfo{
			SupportedFeatures:   proto.Capabilities.SupportedFeatures,
			UnsupportedFeatures: proto.Capabilities.UnsupportedFeatures,
			DiscoveryErrors:     proto.Capabilities.DiscoveryErrors,
			DiscoveryWarnings:   proto.Capabilities.DiscoveryWarnings,
		}
	}

	return dm
}

// convertDiscoveryMethodToProto converts DiscoveryMethod to proto enum
func convertDiscoveryMethodToProto(method DiscoveryMethod) commonv1.DiscoveryMethod {
	switch method {
	case DiscoveryMethodStaticConfig:
		return commonv1.DiscoveryMethod_DISCOVERY_METHOD_STATIC_CONFIG
	case DiscoveryMethodNetworkScan:
		return commonv1.DiscoveryMethod_DISCOVERY_METHOD_NETWORK_SCAN
	case DiscoveryMethodAPIRegistration:
		return commonv1.DiscoveryMethod_DISCOVERY_METHOD_API_REGISTRATION
	case DiscoveryMethodManual:
		return commonv1.DiscoveryMethod_DISCOVERY_METHOD_MANUAL
	default:
		return commonv1.DiscoveryMethod_DISCOVERY_METHOD_UNSPECIFIED
	}
}

// convertDiscoveryMethodFromProto converts proto DiscoveryMethod enum to DiscoveryMethod
func convertDiscoveryMethodFromProto(method commonv1.DiscoveryMethod) DiscoveryMethod {
	switch method {
	case commonv1.DiscoveryMethod_DISCOVERY_METHOD_STATIC_CONFIG:
		return DiscoveryMethodStaticConfig
	case commonv1.DiscoveryMethod_DISCOVERY_METHOD_NETWORK_SCAN:
		return DiscoveryMethodNetworkScan
	case commonv1.DiscoveryMethod_DISCOVERY_METHOD_API_REGISTRATION:
		return DiscoveryMethodAPIRegistration
	case commonv1.DiscoveryMethod_DISCOVERY_METHOD_MANUAL:
		return DiscoveryMethodManual
	default:
		return DiscoveryMethodUnspecified
	}
}
