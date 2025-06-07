package kustomize

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	fluxinstall "github.com/moolen/flux-poc/pkg/installer/flux/installer"
)

type Patch struct {
	PatchYAML string
}

// Renderer represents a renderer with optional patches.
type Renderer struct {
	patches []Patch
}

// NewRenderer creates a new Renderer.
func NewRenderer() *Renderer {
	return &Renderer{
		patches: []Patch{},
	}
}

// AddPatch adds a patch targeting a specific object kind and name.
func (r *Renderer) AddPatch(yaml string) {
	r.patches = append(r.patches, Patch{
		PatchYAML: yaml,
	})
}

// Render renders the kustomize manifests with optional patches.
func (r *Renderer) Render() ([]byte, error) {
	tmpDir := filepath.Join(os.TempDir(), "fluxkustomizer-"+uuid.New().String())
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := copyFS(fluxinstall.FS(), ".", tmpDir); err != nil {
		return nil, fmt.Errorf("failed to copy embedded FS: %w", err)
	}

	kustomizationPath := filepath.Join(tmpDir, "kustomization.yaml")

	// Append structured patches
	var patchDefs bytes.Buffer
	if len(r.patches) > 0 {
		patchDefs.WriteString("\npatches:\n")
	}

	for i, patch := range r.patches {
		patchFilename := fmt.Sprintf("custom-patch-%d.yaml", i)
		patchPath := filepath.Join(tmpDir, patchFilename)

		if err := os.WriteFile(patchPath, []byte(patch.PatchYAML), 0644); err != nil {
			return nil, fmt.Errorf("failed to write patch file: %w", err)
		}

		patchDefs.WriteString(fmt.Sprintf(
			"  - path: %s",
			patchFilename,
		))
	}

	// Append to the existing kustomization.yaml
	f, err := os.OpenFile(kustomizationPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open kustomization.yaml: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(patchDefs.Bytes()); err != nil {
		return nil, fmt.Errorf("failed to write patches to kustomization.yaml: %w", err)
	}

	// Run kustomize
	k := krusty.MakeKustomizer(krusty.MakeDefaultOptions())
	fsys := filesys.MakeFsOnDisk()
	resMap, err := k.Run(fsys, tmpDir)
	if err != nil {
		return nil, fmt.Errorf("kustomize build failed: %w", err)
	}

	yml, err := resMap.AsYaml()
	if err != nil {
		return nil, fmt.Errorf("failed to convert to YAML: %w", err)
	}

	return yml, nil
}

// copyFS copies embedded fs.FS to disk
func copyFS(src fs.FS, basePath, dst string) error {
	return fs.WalkDir(src, basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dst, path)
		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		data, err := fs.ReadFile(src, path)
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, 0644)
	})
}
