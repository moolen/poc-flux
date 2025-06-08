package installer

import (
	"context"
	"fmt"

	"github.com/moolen/flux-poc/pkg/installer/config/awsmeta"
	"github.com/moolen/flux-poc/pkg/installer/config/kubemeta"
)

func (i *Installer) Prepare() error {
	var err error

	cl, err := getKubeClient()
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes client: %w", err)
	}
	i.kubeClient = cl

	i.context.AWSMeta, err = awsmeta.Load()
	if err != nil {
		return fmt.Errorf("failed to get AWS metadata: %w", err)
	}
	i.context.KubeMeta, err = kubemeta.Load(context.Background())
	if err != nil {
		return fmt.Errorf("failed to load Kubernetes metadata: %w", err)
	}
	return nil
}
