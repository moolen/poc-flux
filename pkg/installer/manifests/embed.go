package manifests

import (
	"embed"
	"io/fs"
)

//go:embed manifests/*
var embeddedFiles embed.FS

var manifestsFS fs.FS

// FS returns a read-only filesystem containing the contents of /hack/flux-install.
func FS() fs.FS {
	return manifestsFS
}

func init() {
	var err error
	// Strip the path prefix so it starts at the root of flux-install
	manifestsFS, err = fs.Sub(embeddedFiles, "manifests")
	if err != nil {
		panic("failed to create sub FS: " + err.Error())
	}
}
