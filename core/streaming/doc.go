// Package streaming provides utilities for bidirectional streaming between
// WebSockets and buf Connect RPC.
//
// It is used in the BMC management system where browsers communicate with
// gateways via WebSocket, and gateways communicate with agents via buf Connect streaming.
//
// The package provides:
//   - StreamChunk interface for streaming data
//   - ChunkFactory interface for creating chunks
//   - WebSocketToStreamProxy and StreamToWebSocketProxy for bidirectional translation
//   - HandshakeHelper to manage initial stream handshakes
//
// Example usage (VNC):
//
//	// Gateway side
//	stream := agentClient.StreamVNCData(ctx)
//	helper := streaming.NewHandshakeHelper(&VNCChunkFactory{})
//	helper.SendHandshake(stream, sessionID, serverID)
//	logger := log.With().Str("session_id", sessionID).Str("server_id", serverID).Logger()
//	proxy := streaming.NewWebSocketToStreamProxy(wsConn, sessionID, serverID, logger, &VNCChunkFactory{})
//	proxy.ProxyToStream(ctx, stream)
//
//	// Agent side
//	sessionID, serverID, err := helper.ReceiveHandshake(stream)
//	vncWS, _, err := websocket.DefaultDialer.Dial(server.VNCEndpoint.Endpoint, nil)
//	helper.SendHandshakeAck(stream, sessionID, serverID)
//	logger := log.With().Str("session_id", sessionID).Str("server_id", serverID).Logger()
//	proxy := streaming.NewStreamToWebSocketProxy(sessionID, serverID, logger, &VNCChunkFactory{})
//	proxy.ProxyFromStream(ctx, stream, vncWS)
package streaming
