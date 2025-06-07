package installer

import (
	"context"
	"fmt"

	"github.com/moolen/flux-poc/pkg/installer/config"
	"github.com/moolen/flux-poc/pkg/installer/config/awsmeta"
	"github.com/moolen/flux-poc/pkg/installer/flux"
)

type Installer struct {
	flux    *flux.Flux
	context InstallerContext
}

type InstallerContext struct {
	AWSMeta *awsmeta.Metadata
}

func New() *Installer {
	return &Installer{
		flux:    flux.New(),
		context: InstallerContext{},
	}
}

func (i *Installer) Prepare() error {
	var err error
	i.context.AWSMeta, err = awsmeta.GetAWSMetadata()
	if err != nil {
		return fmt.Errorf("failed to get AWS metadata: %w", err)
	}
	return nil
}

func (i *Installer) WithCACert(secretName string) *Installer {
	i.flux.WithCACert(secretName)
	return i
}

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

func (i *Installer) ApplyManifests() error {
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
