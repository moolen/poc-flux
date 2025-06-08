package installer

import (
	"github.com/moolen/flux-poc/pkg/installer/config/awsmeta"
	"github.com/moolen/flux-poc/pkg/installer/config/kubemeta"
	"github.com/moolen/flux-poc/pkg/installer/flux"
	"k8s.io/client-go/kubernetes"
)

type Installer struct {
	kubeClient *kubernetes.Clientset
	flux       *flux.Flux
	context    InstallerContext
}

type InstallerContext struct {
	AWSMeta  *awsmeta.Metadata
	KubeMeta *kubemeta.Metadata
}

func New() *Installer {
	return &Installer{
		flux:    flux.New(),
		context: InstallerContext{},
	}
}

func (i *Installer) WithCACert(secretName string) *Installer {
	i.flux.WithCACert(secretName)
	return i
}
