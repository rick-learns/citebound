//go:build !embedui

// Package web holds the built web UI, embedded when the binary is built with
// the embedui tag (make build). See docs/decisions/0009-embed-the-web-ui.md.
package web

import "io/fs"

// Files returns nil: this binary was built without the UI.
func Files() fs.FS { return nil }
