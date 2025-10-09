package vnc

import (
	"context"
	"fmt"
	"io"

	"github.com/rs/zerolog/log"

	"local-agent/pkg/vnc/rfb"
)

// RFBProxyHandler handles RFB protocol translation between browser client and authenticated VNC server.
//
// Architecture:
//
//	Browser (noVNC) <--RFB handshake--> Agent <--authenticated session--> BMC VNC Server
//
// The agent acts as an RFB protocol terminator:
// 1. Accepts browser's RFB version negotiation
// 2. Tells browser "no authentication required" (agent already authenticated with BMC)
// 3. Accepts browser's ClientInit
// 4. Sends ServerInit from the real VNC server (triggers it by sending ClientInit to BMC)
// 5. Proxies framebuffer data bidirectionally
//
// This architecture keeps BMC credentials on the agent - browser never sees them.
type RFBProxyHandler struct {
	bmcTransport Transport // Authenticated connection to BMC VNC server
}

// NewRFBProxyHandler creates a new RFB proxy handler
//
// bmcTransport must be an already-authenticated VNC transport connected to the BMC.
func NewRFBProxyHandler(bmcTransport Transport) *RFBProxyHandler {
	return &RFBProxyHandler{
		bmcTransport: bmcTransport,
	}
}

// HandleBrowserHandshake performs the RFB handshake with the browser client
//
// This handles the browser-side RFB protocol:
// 1. Version negotiation (tell browser we support RFB 3.8)
// 2. Security negotiation (tell browser "no auth required")
// 3. ClientInit (accept from browser)
// 4. ServerInit (request from BMC and forward to browser)
//
// After this completes, both the browser and BMC are ready for framebuffer data exchange.
// The caller should then start transparent bidirectional proxying.
func (h *RFBProxyHandler) HandleBrowserHandshake(ctx context.Context, browserConn io.ReadWriter) error {
	log.Debug().Msg("Starting RFB proxy handshake with browser")

	// Create RFB protocol handlers for browser connection
	browserWriter := rfb.NewProtocolWriter(browserConn)
	browserReader := rfb.NewProtocolReader(browserConn)

	// Step 1: Send RFB version to browser (we act as VNC server)
	log.Debug().Msg("Sending RFB version to browser")
	if err := browserWriter.WriteString(rfb.ProtocolVersion38); err != nil {
		return fmt.Errorf("failed to send RFB version to browser: %w", err)
	}

	// Step 2: Read browser's version response
	versionBytes, err := browserReader.ReadBytes(rfb.ProtocolVersionLength)
	if err != nil {
		return fmt.Errorf("failed to read browser RFB version: %w", err)
	}

	browserVersion, err := rfb.ParseProtocolVersion(versionBytes)
	if err != nil {
		return fmt.Errorf("invalid browser RFB version: %w", err)
	}

	log.Debug().Str("browser_version", browserVersion.String()).Msg("Browser RFB version received")

	// Step 3: Send security types to browser (offer "None" since we already authenticated with BMC)
	log.Debug().Msg("Sending security types to browser (offering SecurityTypeNone)")

	if browserVersion.Minor >= 7 {
		// RFB 3.7/3.8: Send list of security types
		// Count (1 byte) + SecurityTypeNone (1 byte)
		if err := browserWriter.WriteU8(1); err != nil {
			return fmt.Errorf("failed to send security type count: %w", err)
		}
		if err := browserWriter.WriteU8(uint8(rfb.SecurityTypeNone)); err != nil {
			return fmt.Errorf("failed to send security type: %w", err)
		}

		// Read browser's security type selection
		selectedType, err := browserReader.ReadU8()
		if err != nil {
			return fmt.Errorf("failed to read browser security type selection: %w", err)
		}

		if rfb.SecurityType(selectedType) != rfb.SecurityTypeNone {
			return fmt.Errorf("browser selected unsupported security type: %d", selectedType)
		}

		log.Debug().Msg("Browser selected SecurityTypeNone")

		// Send security result (OK)
		if err := browserWriter.WriteU32(rfb.SecurityResultOK); err != nil {
			return fmt.Errorf("failed to send security result: %w", err)
		}

	} else {
		// RFB 3.3: Send single security type (server chooses)
		if err := browserWriter.WriteU32(uint32(rfb.SecurityTypeNone)); err != nil {
			return fmt.Errorf("failed to send security type: %w", err)
		}
	}

	log.Info().Msg("Browser RFB security negotiation completed (no auth required)")

	// Step 4: Read ClientInit from browser
	clientInitByte, err := browserReader.ReadU8()
	if err != nil {
		return fmt.Errorf("failed to read ClientInit from browser: %w", err)
	}

	sharedFlag := clientInitByte != 0
	log.Debug().Bool("shared", sharedFlag).Msg("Received ClientInit from browser")

	// Step 5: Get cached ServerInit from transport
	// The transport already sent ClientInit during authentication and cached the ServerInit response.
	// We just need to replay it to the browser.
	var serverInitData []byte

	switch t := h.bmcTransport.(type) {
	case *NativeTransport:
		serverInitData = t.GetServerInit()
	case *WebSocketTransport:
		serverInitData = t.GetServerInit()
	default:
		return fmt.Errorf("unsupported transport type for ServerInit retrieval: %T", h.bmcTransport)
	}

	if len(serverInitData) == 0 {
		return fmt.Errorf("ServerInit not cached - authentication may have failed")
	}

	log.Debug().
		Int("server_init_size", len(serverInitData)).
		Msg("Replaying cached ServerInit to browser")

	// Step 6: Forward cached ServerInit to browser
	if err := browserWriter.Write(serverInitData); err != nil {
		return fmt.Errorf("failed to send ServerInit to browser: %w", err)
	}

	log.Info().Msg("RFB proxy handshake completed - browser and BMC ready for framebuffer data")

	return nil
}

// transportWriter adapts Transport to io.Writer for RFB protocol writer
type transportWriter struct {
	transport Transport
	ctx       context.Context
}

func (tw *transportWriter) Write(p []byte) (int, error) {
	if err := tw.transport.Write(tw.ctx, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// transportReader adapts Transport to io.Reader for RFB protocol reader
type transportReader struct {
	transport Transport
	ctx       context.Context
	buffer    []byte
	pos       int
}

func (tr *transportReader) Read(p []byte) (int, error) {
	// Return buffered data first
	if tr.pos < len(tr.buffer) {
		n := copy(p, tr.buffer[tr.pos:])
		tr.pos += n
		if tr.pos >= len(tr.buffer) {
			tr.buffer = nil
			tr.pos = 0
		}
		return n, nil
	}

	// Read new data from transport
	data, err := tr.transport.Read(tr.ctx)
	if err != nil {
		return 0, err
	}

	// Copy to output buffer
	n := copy(p, data)

	// Buffer remaining data
	if n < len(data) {
		tr.buffer = data
		tr.pos = n
	}

	return n, nil
}
