package webui

import (
	"bytes"
	"io"
)

// TemplateData represents common data for all templates
type TemplateData struct {
	Title         string
	IconText      string
	HeaderTitle   string
	InitialStatus string
}

// VNCData represents data specific to VNC templates
type VNCData struct {
	TemplateData
	SessionID    string
	ServerID     string
	WebSocketURL string
}

// ConsoleData represents data specific to console templates
type ConsoleData struct {
	TemplateData
	SessionID       string
	ServerID        string
	GatewayEndpoint string
	WebSocketURL    string
}

// RenderVNC renders the VNC viewer template
func RenderVNC(data VNCData) (io.Reader, error) {
	var buf bytes.Buffer
	err := templates.ExecuteTemplate(&buf, "vnc.html", data)
	if err != nil {
		return nil, err
	}
	return &buf, nil
}

// RenderConsole renders the console viewer template
func RenderConsole(data ConsoleData) (io.Reader, error) {
	var buf bytes.Buffer
	err := templates.ExecuteTemplate(&buf, "console.html", data)
	if err != nil {
		return nil, err
	}
	return &buf, nil
}
