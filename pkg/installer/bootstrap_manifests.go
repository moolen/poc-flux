package installer

import (
	"context"
	"fmt"

	"github.com/moolen/flux-poc/pkg/installer/config"
)

func (i *Installer) buildManifests() ([]byte, error) {
	fluxManifests, err := i.flux.Build()
	if err != nil {
		return nil, err
	}
	configManifests, err := config.Render()
	if err != nil {
		return nil, err
	}
	return mergeManifests(fluxManifests, configManifests), nil
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
