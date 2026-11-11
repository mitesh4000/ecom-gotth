// Package static exposes the frontend assets that are compiled into the Go
// binary. User-uploaded files must not live here; they belong in UPLOADS_DIR.
package static

import "embed"

// FS contains the production CSS, local HTMX build and bundled images.
//
//go:embed css js images
var FS embed.FS
