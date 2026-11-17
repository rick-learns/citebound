//go:build embedui

// Package web holds the built web UI, embedded when the binary is built with
// the embedui tag (make build). See docs/decisions/0009-embed-the-web-ui.md.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Files returns the built UI, rooted at dist.
func Files() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // "dist" is a valid path, so this cannot happen
	}
	return sub
}
