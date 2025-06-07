package installer

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/moolen/flux-poc/pkg/installer/config/awsmeta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const minVersion = "1.32.0"

type NodeGroupRequirement struct {
	Name         string
	CPU          int64
	MemoryGB     int64
	Architecture string
}

var requiredNodeGroups = []NodeGroupRequirement{
	{"cockroachdb", 4, 16, "amd64"},
	{"nats", 4, 16, "amd64"},
	{"general", 8, 16, "amd64"},
}

var supportedRegions = []string{
	"eu-west-1",
	"eu-west-2",
}

func (i *Installer) CheckPrerequisites() error {
	var validationErrs []error

	// TODO:
	// check supported region
	// correct VPC networking requirements are met?
	// - NAT gateway needed? public internet access needed?
	// check storageclass is configured
	// check metrics server is installed?

	ctx := context.Background()
	clientset, err := getKubeClient()
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes client: %w", err)
	}

	if err := checkKubernetesVersion(clientset); err != nil {
		validationErrs = append(validationErrs, fmt.Errorf("cluster version is not compatible: %w", err))
	}

	if err := checkIRSA(clientset); err != nil {
		validationErrs = append(validationErrs, fmt.Errorf("IRSA (IAM Roles for Service Accounts) is not enabled: %w", err))
	}

	if err := validateNodeGroups(ctx, clientset); err != nil {
		validationErrs = append(validationErrs, fmt.Errorf("node group validation failed: %w", err))
	}
	return errors.Join(validationErrs...)
}

func checkKubernetesVersion(clientset *kubernetes.Clientset) error {
	version, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return err
	}
	return compareVersions(version.GitVersion[1:], minVersion)
}

func compareVersions(current, minimum string) error {
	c := strings.Split(current, ".")
	m := strings.Split(minimum, ".")
	for i := 0; i < len(m); i++ {
		if c[i] > m[i] {
			return nil
		} else if c[i] < m[i] {
			return fmt.Errorf("cluster version %s is less than required %s", current, minimum)
		}
	}
	return nil
}

func checkIRSA(clientset *kubernetes.Clientset) error {
	cm, err := clientset.CoreV1().ConfigMaps("kube-system").Get(context.Background(), "aws-auth", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get aws-auth ConfigMap: %w", err)
	}
	if strings.Contains(cm.Data["mapRoles"], "sts.amazonaws.com") {
		return nil
	}
	return errors.New("IRSA (IAM Roles for Service Accounts) not detected in aws-auth mapRoles")
}

func checkRegion() error {
	region, err := awsmeta.GetRegion()
	if err != nil {
		return fmt.Errorf("failed to get AWS region: %w", err)
	}
	if region == "" {
		return errors.New("region not set or detected")
	}
	for _, r := range supportedRegions {
		if r == region {
			return nil
		}
	}
	return fmt.Errorf("region %q is not supported, supported regions are: %v", region, supportedRegions)
}

func validateNodeGroups(ctx context.Context, clientset *kubernetes.Clientset) error {
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	found := map[string]bool{}

	for _, req := range requiredNodeGroups {
		for _, node := range nodes.Items {
			labels := node.Labels
			if strings.Contains(labels["eks.amazonaws.com/nodegroup"], req.Name) || strings.Contains(labels["alpha.eksctl.io/nodegroup-name"], req.Name) {
				cpu := node.Status.Capacity.Cpu().MilliValue() / 1000
				mem := node.Status.Capacity.Memory().Value() / (1024 * 1024 * 1024)
				arch := node.Status.NodeInfo.Architecture
				if cpu >= req.CPU && mem >= req.MemoryGB && arch == req.Architecture {
					found[req.Name] = true
					break
				}
			}
		}
		if !found[req.Name] {
			return fmt.Errorf("node group %q not found or doesn't meet requirements", req.Name)
		}
	}
	return nil
}
