package webui

import (
	"html/template"
	"io"
	"strings"
	"testing"
)

// TestVNCTemplateRendering ensures that all templates required by VNC can be rendered
func TestVNCTemplateRendering(t *testing.T) {
	data := VNCData{
		TemplateData: TemplateData{
			Title:         "Test VNC",
			IconText:      "🖥️",
			HeaderTitle:   "VNC Console",
			InitialStatus: "Connecting",
		},
		SessionID:       "test-session-123",
		ServerID:        "test-server-456",
		GatewayEndpoint: "gateway.example.com",
		WebSocketURL:    "ws://gateway.example.com/vnc",
	}

	reader, err := RenderVNC(data)
	if err != nil {
		t.Fatalf("Failed to render VNC template: %v", err)
	}

	if reader == nil {
		t.Fatal("RenderVNC returned nil reader")
	}

	// Read the rendered content to ensure it contains expected elements
	contentBytes, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read rendered VNC template: %v", err)
	}

	content := string(contentBytes)

	// Verify that key elements are present
	expectedElements := []string{
		"test-session-123",
		"test-server-456",
		"gateway.example.com",
		"VNC Console Session",
	}

	for _, element := range expectedElements {
		if !strings.Contains(content, element) {
			t.Errorf("Rendered VNC template missing expected element: %s", element)
		}
	}
}

// TestConsoleTemplateRendering ensures that all templates required by Console can be rendered
func TestConsoleTemplateRendering(t *testing.T) {
	data := ConsoleData{
		TemplateData: TemplateData{
			Title:         "Test Console",
			IconText:      "📟",
			HeaderTitle:   "SOL Console",
			InitialStatus: "Connecting",
		},
		SessionID:       "test-session-789",
		ServerID:        "test-server-012",
		GatewayEndpoint: "gateway.example.com",
		WebSocketURL:    "ws://gateway.example.com/console",
	}

	reader, err := RenderConsole(data)
	if err != nil {
		t.Fatalf("Failed to render Console template: %v", err)
	}

	if reader == nil {
		t.Fatal("RenderConsole returned nil reader")
	}

	// Read the rendered content to ensure it contains expected elements
	contentBytes, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read rendered Console template: %v", err)
	}

	content := string(contentBytes)

	// Verify that key elements are present
	expectedElements := []string{
		"test-session-789",
		"test-server-012",
		"gateway.example.com",
		"SOL Console Session",
	}

	for _, element := range expectedElements {
		if !strings.Contains(content, element) {
			t.Errorf("Rendered Console template missing expected element: %s", element)
		}
	}
}

// TestTemplateIntegrity ensures all required template files are parsed
func TestTemplateIntegrity(t *testing.T) {
	tests := []struct {
		name      string
		templates *template.Template
		expected  []string
	}{
		{
			name:      "VNC templates",
			templates: vncTemplates,
			expected: []string{
				"vnc.html",
				"base.html",
				"bmc_info_sidebar",
				"boot_status_sidebar",
			},
		},
		{
			name:      "Console templates",
			templates: consoleTemplates,
			expected: []string{
				"console.html",
				"base.html",
				"bmc_info_sidebar",
				"boot_status_sidebar",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.templates == nil {
				t.Fatal("templates is nil")
			}

			for _, expectedTemplate := range tt.expected {
				tmpl := tt.templates.Lookup(expectedTemplate)
				if tmpl == nil {
					t.Errorf("Required template %q not found in %s", expectedTemplate, tt.name)
				}
			}
		})
	}
}

// TestTemplateConsistency ensures both VNC and Console templates include the same shared components
func TestTemplateConsistency(t *testing.T) {
	sharedTemplates := []string{
		"base.html",
		"bmc_info_sidebar",
		"boot_status_sidebar",
	}

	for _, tmplName := range sharedTemplates {
		t.Run(tmplName, func(t *testing.T) {
			vncTmpl := vncTemplates.Lookup(tmplName)
			if vncTmpl == nil {
				t.Errorf("VNC templates missing shared template: %s", tmplName)
			}

			consoleTmpl := consoleTemplates.Lookup(tmplName)
			if consoleTmpl == nil {
				t.Errorf("Console templates missing shared template: %s", tmplName)
			}
		})
	}
}
