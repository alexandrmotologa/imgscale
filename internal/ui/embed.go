package ui

import (
	"embed"
	"io/fs"
)

//go:embed dist/*
var distFS embed.FS

// GetFS returns the sub-filesystem of embedded static assets.
func GetFS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
