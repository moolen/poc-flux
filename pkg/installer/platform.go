package installer

import (
	"context"
	"fmt"

	"github.com/moolen/flux-poc/pkg/installer/vault"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (i *Installer) ReconcilePlatform() error {

	if err := i.reconcileVault(); err != nil {
		return fmt.Errorf("reconciling vault: %w", err)
	}
	return nil
}

func (i *Installer) reconcileVault() error {
	// we expect vault address to be static and
	// vault root token to be available in a Kubernetes secret
	// TODO: discover vault CA cert
	vaultAddr := "http://vault.vault.svc.cluster.local.:8200"
	token, err := i.getVaultToken()
	if err != nil {
		return fmt.Errorf("getting vault token: %w", err)
	}
	vaultMgt, err := vault.New(vaultAddr, token)
	if err != nil {
		return fmt.Errorf("creating vault manager: %w", err)
	}
	if err = vaultMgt.ReconcilePolicies(context.Background(), []vault.VaultPolicy{
		{
			Name: "flux-system",
			Policy: `
path "secret/*" {
  capabilities = ["read", "list"]
}
`,
		},
	}); err != nil {
		return fmt.Errorf("unable to reconcile roles: %w", err)
	}
	if err := vaultMgt.Reconcile(context.Background(), vault.KubernetesAuthConfig{
		MountPath:     "kubernetes",
		KubeHost:      i.context.KubeMeta.Host,
		KubeCA:        i.context.KubeMeta.CACertBase64,
		TokenReviewer: "vault-token-reviewer",
		Roles:         getKubernetesVaultRoles(),
	}); err != nil {
		return fmt.Errorf("reconciling vault: %w", err)
	}
	return nil
}

func getKubernetesVaultRoles() []vault.VaultKubeRole {
	return []vault.VaultKubeRole{
		{
			Name:                          "flux-system",
			BoundServiceAccountNames:      []string{"flux-system"},
			BoundServiceAccountNamespaces: []string{"flux-system"},
			Policies:                      []string{"flux-system"},
			TTL:                           "1h",
			Period:                        "30m",
		},
	}
}

func (i *Installer) getVaultToken() (string, error) {
	rootToken, err := i.kubeClient.CoreV1().Secrets("vault").Get(context.Background(), "root-token", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("getting vault root token: %w", err)
	}
	if rootToken == nil || rootToken.Data == nil || len(rootToken.Data) == 0 {
		return "", fmt.Errorf("vault root token secret is empty")
	}
	if _, ok := rootToken.Data["token"]; !ok {
		return "", fmt.Errorf("vault root token secret does not contain 'token' key")
	}
	return string(rootToken.Data["token"]), nil
}
