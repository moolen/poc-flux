package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	vault "github.com/hashicorp/vault/api"
)

type KubernetesAuthConfig struct {
	MountPath     string
	KubeHost      string
	KubeCA        string
	TokenReviewer string
	Roles         []VaultKubeRole
}

type VaultKubeRole struct {
	Name                          string
	BoundServiceAccountNames      []string
	BoundServiceAccountNamespaces []string
	Policies                      []string
	TTL                           string
	Period                        string
}

type VaultPolicy struct {
	Name   string
	Policy string
}

type Manager struct {
	client *vault.Client
	mount  string
}

func New(vaultAddr, token string) (*Manager, error) {
	cfg := vault.DefaultConfig()
	cfg.Address = vaultAddr
	client, err := vault.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	client.SetToken(token)
	return &Manager{client: client}, nil
}

func (m *Manager) Reconcile(ctx context.Context, cfg KubernetesAuthConfig) error {
	m.mount = cfg.MountPath

	// Enable Kubernetes auth method if not enabled
	auths, err := m.client.Sys().ListAuth()
	if err != nil {
		return fmt.Errorf("failed to list auth methods: %w", err)
	}
	if _, ok := auths[cfg.MountPath+"/"]; !ok {
		err := m.client.Sys().EnableAuthWithOptions(cfg.MountPath, &vault.EnableAuthOptions{Type: "kubernetes"})
		if err != nil {
			return fmt.Errorf("failed to enable kubernetes auth: %w", err)
		}
	}

	// Write config to /auth/kubernetes/config
	confPath := fmt.Sprintf("auth/%s/config", cfg.MountPath)
	existing, err := m.client.Logical().Read(confPath)
	if err != nil && !isNotFound(err) {
		return err
	}
	input := map[string]interface{}{
		"kubernetes_host":    cfg.KubeHost,
		"kubernetes_ca_cert": cfg.KubeCA,
		"token_reviewer_jwt": cfg.TokenReviewer,
	}
	if existing == nil || !equalMaps(input, existing.Data) {
		_, err := m.client.Logical().Write(confPath, input)
		if err != nil {
			return fmt.Errorf("failed to write kubernetes config: %w", err)
		}
	}

	// Configure roles
	for _, role := range cfg.Roles {
		rolePath := fmt.Sprintf("auth/%s/role/%s", cfg.MountPath, role.Name)
		desired := map[string]interface{}{
			"bound_service_account_names":      role.BoundServiceAccountNames,
			"bound_service_account_namespaces": role.BoundServiceAccountNamespaces,
			"policies":                         strings.Join(role.Policies, ","),
			"ttl":                              role.TTL,
			"period":                           role.Period,
		}
		existing, err := m.client.Logical().Read(rolePath)
		if err != nil && !isNotFound(err) {
			return fmt.Errorf("failed to read existing role: %w", err)
		}
		if existing == nil || !equalMaps(desired, existing.Data) {
			_, err := m.client.Logical().Write(rolePath, desired)
			if err != nil {
				return fmt.Errorf("failed to write role %s: %w", role.Name, err)
			}
		}
	}
	return nil
}

func (m *Manager) ReconcilePolicies(ctx context.Context, policies []VaultPolicy) error {
	for _, policy := range policies {
		existing, err := m.client.Sys().GetPolicy(policy.Name)
		if err != nil {
			return fmt.Errorf("failed to get policy %s: %w", policy.Name, err)
		}
		if existing != policy.Policy {
			err := m.client.Sys().PutPolicy(policy.Name, policy.Policy)
			if err != nil {
				return fmt.Errorf("failed to write policy %s: %w", policy.Name, err)
			}
		}
	}
	return nil
}

func (m *Manager) ReconcileSecretEngine(ctx context.Context) error {
	mountPath := "secrets/"

	mounts, err := m.client.Sys().ListMounts()
	if err != nil {
		return fmt.Errorf("failed to list mounts: %w", err)
	}

	// Check if already mounted with kv v2
	if mount, ok := mounts[mountPath]; ok {
		if mount.Type == "kv" && mount.Options["version"] == "2" {
			return nil // already configured correctly
		}
		// If wrong version or type, disable it before re-mounting
		err := m.client.Sys().Unmount(mountPath)
		if err != nil {
			return fmt.Errorf("failed to unmount existing secret engine: %w", err)
		}
	}

	// Mount KV v2
	opts := &vault.MountInput{
		Type: "kv",
		Options: map[string]string{
			"version": "2",
		},
		Description: "KV v2 secrets engine for cluster",
	}

	if err := m.client.Sys().Mount(mountPath, opts); err != nil {
		return fmt.Errorf("failed to mount kv v2 secret engine: %w", err)
	}
	return nil
}

func isNotFound(err error) bool {
	if respErr, ok := err.(*vault.ResponseError); ok {
		return respErr.StatusCode == http.StatusNotFound
	}
	return false
}

func equalMaps(a, b map[string]interface{}) bool {
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return string(aJSON) == string(bJSON)
}
