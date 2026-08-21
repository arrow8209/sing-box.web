package webui

import (
	"embed"
)

// Files contains the Vue build output and the version-controlled fallback page.
//
//go:embed all:assets all:fallback
var Files embed.FS
