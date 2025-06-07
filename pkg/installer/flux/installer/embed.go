package installer

import (
	"embed"
	"io/fs"
)

//go:embed manifests/*
var embeddedFiles embed.FS

var fluxFS fs.FS

func init() {
	var err error
	// Strip the path prefix so it starts at the root of flux-install
	fluxFS, err = fs.Sub(embeddedFiles, "manifests")
	if err != nil {
		panic("failed to create sub FS: " + err.Error())
	}
}
