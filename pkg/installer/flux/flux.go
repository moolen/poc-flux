package flux

import (
	"fmt"

	"github.com/moolen/flux-poc/pkg/installer/flux/kustomize"
)

type Flux struct {
	renderer *kustomize.Renderer
}

func New() *Flux {
	return &Flux{
		renderer: kustomize.NewRenderer(),
	}
}

// With CACert
func (f *Flux) WithCACert(secretName string) *Flux {
	f.renderer.AddPatch(fmt.Sprintf(`
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

	return f
}

func (f *Flux) Build() ([]byte, error) {
	return f.renderer.Render()
}
