package webui

import (
	"embed"
	"html/template"
)

//go:embed templates/*
var embedFS embed.FS

var templates *template.Template
var vncTemplates *template.Template
var consoleTemplates *template.Template

func init() {
	var err error

	// Parse VNC templates separately (base.html + vnc.html)
	vncTemplates, err = template.ParseFS(embedFS, "templates/base.html", "templates/vnc.html", "templates/bmc_info_sidebar.html", "templates/boot_status_sidebar.html")
	if err != nil {
		panic("Failed to parse VNC templates: " + err.Error())
	}

	// Parse Console templates separately (base.html + console.html)
	consoleTemplates, err = template.ParseFS(embedFS, "templates/base.html", "templates/console.html", "templates/bmc_info_sidebar.html", "templates/boot_status_sidebar.html")
	if err != nil {
		panic("Failed to parse Console templates: " + err.Error())
	}

	// Keep old templates variable for backwards compatibility
	templates = consoleTemplates
}
