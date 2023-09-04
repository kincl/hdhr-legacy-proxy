package web

import (
	"embed"
)

// content holds our static web server content.
//
//go:embed index.html
var Content embed.FS
