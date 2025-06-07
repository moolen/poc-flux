package installer

import (
	"context"
	"fmt"

	"github.com/moolen/flux-poc/pkg/installer/aws/irsa"
)

func (i *Installer) ReconcileInfrastructure() (bool, error) {
	// reconcile IAM roles for service accounts (IRSA)
	mgr, err := irsa.New(context.Background())
	if err != nil {
		return false, err
	}
	irsaConfig := i.IRSAConfig()
	if err = mgr.Reconcile(context.Background(), irsaConfig); err != nil {
		return false, fmt.Errorf("reconciling IRSA: %w", err)
	}
	if err := mgr.GarbageCollect(context.Background(), irsaConfig); err != nil {
		return false, fmt.Errorf("garbage collecting IRSA: %w", err)
	}

	return false, nil
}

func (i *Installer) IRSAConfig() []irsa.IRSAConfig {
	return []irsa.IRSAConfig{
		{
			RoleName: fmt.Sprintf("%s-flux-source-controller", i.context.AWSMeta.ClusterName),
			PolicyArns: []string{
				"arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
			},
			OIDCProviderArn: i.context.AWSMeta.OIDCProviderARN,
			ServiceAccount:  "flux-system:source-controller",
			Audience:        "sts.amazonaws.com",
		},
	}
}
