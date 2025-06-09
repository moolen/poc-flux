package installer

import (
	"context"
	"fmt"

	"github.com/moolen/flux-poc/pkg/installer/config"
	"github.com/moolen/flux-poc/pkg/installer/manifests"
)

func (i *Installer) buildManifests() ([]byte, error) {
	kustomizeManifests, err := i.kustomizeRender.Render(manifests.FS())
	if err != nil {
		return nil, fmt.Errorf("failed to render kustomize manifests: %w", err)
	}
	configManifests, err := config.Render()
	if err != nil {
		return nil, fmt.Errorf("failed to render config manifests: %w", err)
	}
	return mergeManifests(kustomizeManifests, configManifests), nil
}

func (i *Installer) ApplyBootstrapManifests() error {
	manifests, err := i.buildManifests()
	if err != nil {
		return fmt.Errorf("failed to build manifests: %w", err)
	}
	fmt.Printf("%s", string(manifests))
	return applyYAMLManifests(context.TODO(), manifests)
}

func mergeManifests(manifests ...[]byte) []byte {
	var merged []byte
	for _, manifest := range manifests {
		merged = append(merged, manifest...)
		merged = append(merged, []byte("\n---\n")...)
	}
	return merged
}
