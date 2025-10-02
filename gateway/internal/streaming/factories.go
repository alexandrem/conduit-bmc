package streaming

import (
	"core/streaming"
	gatewayv1 "gateway/gen/gateway/v1"
)

// VNCChunkFactory creates VNC data chunks for streaming
type VNCChunkFactory struct{}

func (f *VNCChunkFactory) NewChunk(sessionID, serverID string, data []byte, isHandshake, closeStream bool) *gatewayv1.VNCDataChunk {
	return &gatewayv1.VNCDataChunk{
		SessionId:   sessionID,
		ServerId:    serverID,
		Data:        data,
		IsHandshake: isHandshake,
		CloseStream: closeStream,
	}
}

// Ensure VNCDataChunk implements StreamChunk interface
var _ streaming.StreamChunk = (*gatewayv1.VNCDataChunk)(nil)

// ConsoleChunkFactory creates console data chunks for streaming
type ConsoleChunkFactory struct{}

func (f *ConsoleChunkFactory) NewChunk(sessionID, serverID string, data []byte, isHandshake, closeStream bool) *gatewayv1.ConsoleDataChunk {
	return &gatewayv1.ConsoleDataChunk{
		SessionId:   sessionID,
		ServerId:    serverID,
		Data:        data,
		IsHandshake: isHandshake,
		CloseStream: closeStream,
	}
}

// Ensure ConsoleDataChunk implements StreamChunk interface
var _ streaming.StreamChunk = (*gatewayv1.ConsoleDataChunk)(nil)
