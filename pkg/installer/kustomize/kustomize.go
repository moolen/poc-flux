package kustomize

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	yaml "sigs.k8s.io/yaml/goyaml.v3"
)

type Patch struct {
	PatchYAML string
}

// Renderer represents a renderer with optional patches and an image registry override.
type Renderer struct {
	imageRegistry string
	patches       []Patch
}

// NewRenderer creates a new Renderer.
// The imageRegistry parameter can be used to override the registry of all images.
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

func (r *Renderer) WithImageRegistry(registry string) *Renderer {
	r.imageRegistry = registry
	return r
}

// Render renders the kustomize manifests with optional patches and image registry overrides.
func (r *Renderer) Render(target fs.FS) ([]byte, error) {
	tmpDir := filepath.Join(os.TempDir(), "fluxkustomizer-"+uuid.New().String())
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := copyFS(target, ".", tmpDir); err != nil {
		return nil, fmt.Errorf("failed to copy embedded FS: %w", err)
	}

	// Find all unique images if an imageRegistry override is provided.
	uniqueImages := make(map[string]struct{})
	if r.imageRegistry != "" {
		if err := findImagesInFS(target, uniqueImages); err != nil {
			return nil, fmt.Errorf("failed to find images in target fs: %w", err)
		}
	}
	logrus.Debugf("found %d unique images in target fs", len(uniqueImages))

	kustomizationPath := filepath.Join(tmpDir, "kustomization.yaml")
	kustomizationData, err := os.ReadFile(kustomizationPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read kustomization.yaml: %w", err)
	}

	kustomization := &types.Kustomization{}
	if err := yaml.Unmarshal(kustomizationData, kustomization); err != nil {
		return nil, fmt.Errorf("failed to decode kustomization.yaml: %w", err)
	}

	r.replaceImageRegistry(kustomization, uniqueImages)

	// Apply patches.
	for i, patch := range r.patches {
		patchFilename := fmt.Sprintf("custom-patch-%d.yaml", i)
		patchPath := filepath.Join(tmpDir, patchFilename)

		if err := os.WriteFile(patchPath, []byte(patch.PatchYAML), 0644); err != nil {
			return nil, fmt.Errorf("failed to write patch file: %w", err)
		}

		kustomization.Patches = append(kustomization.Patches, types.Patch{
			Path: patchFilename,
		})
	}

	// Marshal the updated kustomization struct and write it back to the file.
	updatedKustomizationData, err := yaml.Marshal(kustomization)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal updated kustomization: %w", err)
	}

	if err := os.WriteFile(kustomizationPath, updatedKustomizationData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write updated kustomization.yaml: %w", err)
	}

	// Run kustomize.
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

func (r *Renderer) replaceImageRegistry(kustomization *types.Kustomization, uniqueImages map[string]struct{}) {
	// Apply image registry override.
	if r.imageRegistry != "" {
		// Use a map to avoid adding duplicate image names.
		imageOverrides := make(map[string]types.Image)
		for _, img := range kustomization.Images {
			imageOverrides[img.Name] = img
		}

		for imgStr := range uniqueImages {
			ref, err := name.ParseReference(imgStr)
			if err != nil {
				// Skip invalid image references.
				continue
			}
			originalName := ref.Context().Name()
			if _, exists := imageOverrides[originalName]; exists {
				continue
			}

			repoPath := ref.Context().RepositoryStr()
			// For default Docker Hub images, remove the implicit "library/" path.
			if ref.Context().RegistryStr() == name.DefaultRegistry {
				repoPath = strings.TrimPrefix(repoPath, "library/")
			}

			newName := filepath.Join(r.imageRegistry, repoPath)

			imageOverrides[originalName] = types.Image{
				Name:    originalName,
				NewName: newName,
			}
		}

		// Replace the images slice with the updated list.
		kustomization.Images = make([]types.Image, 0, len(imageOverrides))
		for _, img := range imageOverrides {
			logrus.Debugf("Replacing image %s with %s", img.Name, img.NewName)
			kustomization.Images = append(kustomization.Images, img)
		}
	}
}

// findImagesInFS walks the fs.FS, parses all YAML files, and collects unique image strings.
func findImagesInFS(src fs.FS, images map[string]struct{}) error {
	return fs.WalkDir(src, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !(strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
			return nil
		}

		data, err := fs.ReadFile(src, path)
		if err != nil {
			return err
		}

		decoder := yaml.NewDecoder(strings.NewReader(string(data)))
		for {
			var doc interface{}
			if err := decoder.Decode(&doc); err != nil {
				if err.Error() == "EOF" { // Using string comparison as io.EOF is not directly comparable when wrapped
					break // End of file
				}
				// Ignore files that are not valid YAML documents or other decoding errors.
				// We continue to the next document if possible, or break if it's a fatal error.
				break
			}
			findImageKeys(images, doc)
		}
		return nil
	})
}

// findImageKeys recursively traverses a decoded YAML document and finds all values for the "image" key.
func findImageKeys(images map[string]struct{}, data interface{}) {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, val := range v {
			if key == "image" {
				if imageName, ok := val.(string); ok {
					images[imageName] = struct{}{}
				}
			} else {
				findImageKeys(images, val)
			}
		}
	case []interface{}:
		for _, item := range v {
			findImageKeys(images, item)
		}
	}
}

// copyFS copies embedded fs.FS to disk.
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
