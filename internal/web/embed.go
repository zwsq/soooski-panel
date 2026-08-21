package web

import "embed"

//go:embed dist client.html
var FS embed.FS
