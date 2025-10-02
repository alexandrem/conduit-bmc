package webui

import (
	"embed"
	"html/template"
)

//go:embed templates/*
var embedFS embed.FS

var templates *template.Template

func init() {
	var err error
	templates, err = template.ParseFS(embedFS, "templates/*.html")
	if err != nil {
		panic("Failed to parse embedded templates: " + err.Error())
	}
}
