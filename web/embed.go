// Package web embeds the built UI. Run `pnpm build` in this directory first.
package web

import "embed"

//go:embed all:dist
var Dist embed.FS
