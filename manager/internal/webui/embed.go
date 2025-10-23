package webui

import (
	"embed"
	"html/template"
)

//go:embed templates/*
var embedFS embed.FS

var adminTemplates *template.Template
var loginTemplates *template.Template

func init() {
	var err error

	// Parse admin dashboard templates
	adminTemplates, err = template.ParseFS(embedFS, "templates/base.html", "templates/dashboard.html")
	if err != nil {
		panic("Failed to parse admin templates: " + err.Error())
	}

	// Parse login templates
	loginTemplates, err = template.ParseFS(embedFS, "templates/base.html", "templates/login.html")
	if err != nil {
		panic("Failed to parse login templates: " + err.Error())
	}
}

// GetAdminTemplates returns the compiled admin templates
func GetAdminTemplates() *template.Template {
	return adminTemplates
}

// GetLoginTemplates returns the compiled login templates
func GetLoginTemplates() *template.Template {
	return loginTemplates
}
