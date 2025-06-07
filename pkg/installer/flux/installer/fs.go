package installer

import "io/fs"

// FS returns a read-only filesystem containing the contents of /hack/flux-install.
func FS() fs.FS {
	return fluxFS
}
