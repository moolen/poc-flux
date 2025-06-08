package kubemeta

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Metadata struct {
	KubeVersion      string
	CACertBase64     string
	Host             string
	ClusterDNSDomain string
}

func Load(ctx context.Context) (*Metadata, error) {
	restConfig, err := getRestConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	serverVersion, err := discoveryClient.ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get server version: %w", err)
	}

	dnsDomain := "cluster.local" // default value, will override if kube-dns configmap is found
	cm, err := clientset.CoreV1().ConfigMaps("kube-system").Get(ctx, "kube-dns", metav1.GetOptions{})
	if err == nil {
		if val, ok := cm.Data["stubDomains"]; ok && val != "" {
			dnsDomain = val // optionally parse JSON if needed
		}
	}

	caData := base64.StdEncoding.EncodeToString(restConfig.CAData)

	return &Metadata{
		KubeVersion:      serverVersion.GitVersion,
		CACertBase64:     caData,
		Host:             restConfig.Host,
		ClusterDNSDomain: dnsDomain,
	}, nil
}

func getRestConfig() (*rest.Config, error) {
	// Try in-cluster config first
	restConfig, err := rest.InClusterConfig()
	if err == nil {
		return restConfig, nil
	}

	// Fallback to kubeconfig
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = clientcmd.RecommendedHomeFile
	}

	restConfig, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build rest config: %w", err)
	}

	return restConfig, nil
}
