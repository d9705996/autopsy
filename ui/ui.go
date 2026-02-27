// Package ui embeds the compiled React SPA from ui/dist.
// The dist/ directory is populated by `npm run build --prefix web`.
package ui

import "embed"

// FS holds the embedded compiled React SPA assets.
//
//go:embed dist
var FS embed.FS
