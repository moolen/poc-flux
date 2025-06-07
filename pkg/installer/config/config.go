package config

import (
	"bytes"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/moolen/flux-poc/pkg/installer/config/awsmeta"
)

func Render() ([]byte, error) {
	config := make(map[string]string)
	config["hello"] = "world"
	awsMeta, err := awsmeta.GetAWSMetadata()
	if err != nil {
		return nil, fmt.Errorf("failed to get AWS metadata: %w", err)
	}
	mergedMaps := mergeMaps(config, awsMeta.ToMap())

	cm, err := mapToConfigMapYAML("cluster-config", "default", mergedMaps)
	if err != nil {
		return nil, fmt.Errorf("failed to create ConfigMap YAML: %w", err)
	}
	return []byte(cm), nil
}

func mapToConfigMapYAML(name, namespace string, data map[string]string) (string, error) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)

	cm := &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Data:       data,
	}

	serializer := json.NewSerializerWithOptions(
		json.DefaultMetaFactory, scheme, scheme,
		json.SerializerOptions{Yaml: true, Pretty: true, Strict: true},
	)

	var buf bytes.Buffer
	if err := serializer.Encode(cm, &buf); err != nil {
		return "", fmt.Errorf("failed to serialize ConfigMap: %w", err)
	}

	return buf.String(), nil
}

func mergeMaps(maps ...map[string]string) map[string]string {
	merged := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			merged[k] = v
		}
	}
	return merged
}
