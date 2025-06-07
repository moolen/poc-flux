package installer

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

// getKubeConfig returns a Kubernetes REST config by checking in-cluster config first,
// then falling back to the kubeconfig file (~/.kube/config).
func getKubeConfig() (*rest.Config, error) {
	// Try in-cluster config
	if config, err := rest.InClusterConfig(); err == nil {
		return config, nil
	}

	// Fallback to kubeconfig file
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("cannot determine home directory: %w", err)
		}
		kubeconfig = filepath.Join(homeDir, ".kube", "config")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}
	return config, nil
}

func getKubeClient() (*kubernetes.Clientset, error) {
	config, err := getKubeConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}
	return clientset, nil
}

func applyYAMLManifests(ctx context.Context, yamlData []byte) error {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = apiextensionsv1.AddToScheme(scheme)

	cl, err := client.New(config.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	decoder := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlData), 4096)
	universal := serializer.NewCodecFactory(scheme).UniversalDeserializer()

	for {
		raw := map[string]interface{}{}
		if err := decoder.Decode(&raw); err != nil {
			if err.Error() == "EOF" {
				break
			}
			return fmt.Errorf("failed to decode YAML: %w", err)
		}
		if len(raw) == 0 {
			continue // skip empty docs
		}

		objYAML, err := yaml.Marshal(raw)
		if err != nil {
			return fmt.Errorf("failed to marshal raw object: %w", err)
		}

		obj, gvk, err := universal.Decode(objYAML, nil, nil)
		if err != nil {
			return fmt.Errorf("failed to decode typed object: %w", err)
		}

		// assert client.Object
		cObj, ok := obj.(client.Object)
		if !ok {
			return fmt.Errorf("decoded object is not a client.Object: %T", obj)
		}

		logrus.Debugf("Applying %s %s/%s", gvk.Kind, cObj.GetNamespace(), cObj.GetName())
		err = cl.Patch(ctx, cObj, client.Apply, &client.PatchOptions{
			Force:        ptr.To(true),
			FieldManager: "custom-applier",
		})
		if err != nil {
			return fmt.Errorf("failed to apply %s %s/%s: %w",
				gvk.Kind, cObj.GetNamespace(), cObj.GetName(), err)
		}
	}

	return nil
}
