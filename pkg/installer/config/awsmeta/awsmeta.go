package awsmeta

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// MetadataKeys for returned map
const (
	KeyAccountID       = "aws_account_id"
	KeyRegion          = "aws_region"
	KeyClusterName     = "cluster_name"
	KeyOIDCProviderARN = "oidc_provider_arn"

	kubeHostEnv = "KUBERNETES_SERVICE_HOST"
)

type Metadata struct {
	AccountID       string `json:"aws_account_id"`
	Region          string `json:"aws_region"`
	ClusterName     string `json:"cluster_name"`
	OIDCProviderARN string `json:"oidc_provider_arn"`
}

func (m *Metadata) ToMap() map[string]string {
	return map[string]string{
		KeyAccountID:       aws.ToString(&m.AccountID),
		KeyRegion:          aws.ToString(&m.Region),
		KeyClusterName:     aws.ToString(&m.ClusterName),
		KeyOIDCProviderARN: aws.ToString(&m.OIDCProviderARN),
	}
}

// Load returns AWS account ID, region, and EKS cluster name inferred from environment and STS.
func Load() (*Metadata, error) {
	// Load AWS config with default credential chain and region resolution
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Get AWS Account ID via STS
	stsClient := sts.NewFromConfig(cfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to get caller identity: %w", err)
	}

	// Get region from loaded config
	region := cfg.Region
	if region == "" {
		return nil, fmt.Errorf("AWS region not found in config")
	}

	clusterName, err := inferClusterNameFromKubeHost()
	if err != nil {
		return nil, fmt.Errorf("failed to infer EKS cluster name: %w", err)
	}

	oidcProviderArn, err := getOIDCProviderARN(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to get OIDC provider ARN: %w", err)
	}

	return &Metadata{
		AccountID:       aws.ToString(identity.Account),
		Region:          region,
		ClusterName:     clusterName,
		OIDCProviderARN: oidcProviderArn,
	}, nil
}

func GetRegion() (string, error) {
	// Load AWS config with default credential chain and region resolution
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %w", err)
	}
	return cfg.Region, nil
}

// inferClusterNameFromKubeHost tries to parse the EKS cluster name from the in-cluster DNS hostname.
func inferClusterNameFromKubeHost() (string, error) {
	kubeHost := os.Getenv(kubeHostEnv)
	if kubeHost == "" {
		return "", fmt.Errorf("not running inside a Kubernetes cluster (missing %s)", kubeHostEnv)
	}

	// Attempt to match EKS cluster endpoint style
	// e.g. "<cluster-name>.yl4.us-west-2.eks.amazonaws.com"
	hostname := kubeHost
	if !strings.Contains(hostname, ".") {
		hostname += ".default.svc" // fallback DNS suffix
	}

	fullURL := "https://" + hostname
	u, err := url.Parse(fullURL)
	if err != nil {
		return "", fmt.Errorf("unable to parse Kubernetes host: %w", err)
	}

	re := regexp.MustCompile(`^([a-zA-Z0-9-]+)\..*\.eks\.amazonaws\.com$`)
	matches := re.FindStringSubmatch(u.Host)
	if len(matches) != 2 {
		return "", fmt.Errorf("hostname %s doesn't look like an EKS endpoint", u.Host)
	}

	return matches[1], nil
}

func getOIDCProviderARN(ctx context.Context, clusterName string) (string, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return "", err
	}

	eksClient := eks.NewFromConfig(cfg)

	out, err := eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe EKS cluster: %w", err)
	}

	issuer := aws.ToString(out.Cluster.Identity.Oidc.Issuer)
	if issuer == "" {
		return "", fmt.Errorf("OIDC issuer not found in cluster identity")
	}

	// Example: issuer = "https://oidc.eks.us-west-2.amazonaws.com/id/1234567890ABCDEF"
	// ARN format: arn:aws:iam::<account_id>:oidc-provider/oidc.eks.us-west-2.amazonaws.com/id/1234567890ABCDEF
	issuer = strings.TrimPrefix(issuer, "https://")

	// Get AWS Account ID via STS
	stsClient := sts.NewFromConfig(cfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("failed to get caller identity: %w", err)
	}
	oidcARN := fmt.Sprintf("arn:aws:iam::%s:oidc-provider/%s", aws.ToString(identity.Account), issuer)
	return oidcARN, nil
}
