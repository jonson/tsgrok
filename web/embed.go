package web

import "embed"

//go:embed templates/*
var TemplatesFS embed.FS

//go:embed all:static
var StaticFS embed.FS
