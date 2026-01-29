// Package web provides embedded frontend assets.
package web

import "embed"

//go:embed dist
var BuildFS embed.FS

//go:embed dist/index.html
var IndexPage []byte
