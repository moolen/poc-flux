package installer

import (
	"fmt"

	"github.com/moolen/flux-poc/pkg/installer/config/awsmeta"
	"github.com/moolen/flux-poc/pkg/installer/config/kubemeta"
	"github.com/moolen/flux-poc/pkg/installer/kustomize"
	"k8s.io/client-go/kubernetes"
)

type Installer struct {
	kubeClient      *kubernetes.Clientset
	kustomizeRender *kustomize.Renderer
	context         InstallerContext
}

type InstallerContext struct {
	AWSMeta  *awsmeta.Metadata
	KubeMeta *kubemeta.Metadata
}

func New() *Installer {
	return &Installer{
		kustomizeRender: kustomize.NewRenderer(),
		context:         InstallerContext{},
	}
}

func (i *Installer) WithCACert(secretName string) *Installer {
	i.kustomizeRender.AddPatch(fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: source-controller
  namespace: flux-system
spec:
  template:
    spec:
      volumes:
      - name: ca-cert
        secret:
          secretName: %s
      containers:
      - name: manager
        volumeMounts:
        - name: ca-cert
          mountPath: /etc/ssl/certs/ca.crt
          subPath: ca.crt
`, secretName))

	return i
}
