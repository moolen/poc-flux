package irsa

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/sirupsen/logrus"
)

const (
	ClusterTagKey   = "kubernetes.io/cluster/flux-poc"
	ClusterTagValue = "owned"
)

type IRSAConfig struct {
	RoleName         string
	PolicyArns       []string
	InlinePolicyName string
	InlinePolicyDoc  string
	OIDCProviderArn  string
	ServiceAccount   string // format: namespace:serviceaccount
	Audience         string
}

type Manager struct {
	client *iam.Client
}

var tags = []types.Tag{
	{
		Key:   aws.String(ClusterTagKey),
		Value: aws.String(ClusterTagValue),
	},
}

func New(ctx context.Context) (*Manager, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &Manager{client: iam.NewFromConfig(cfg)}, nil
}

func (m *Manager) Reconcile(ctx context.Context, roles []IRSAConfig) error {
	for _, role := range roles {
		if err := m.ensureRole(ctx, role); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) ensureRole(ctx context.Context, cfg IRSAConfig) error {
	assumeRoleDoc, err := generateTrustPolicy(cfg)
	if err != nil {
		return err
	}

	logrus.Debugf("Ensuring IAM role %s with trust policy: %s", cfg.RoleName, assumeRoleDoc)
	getOut, err := m.client.GetRole(ctx, &iam.GetRoleInput{
		RoleName: aws.String(cfg.RoleName),
	})

	if err != nil {
		var notFoundErr *types.NoSuchEntityException
		if errors.As(err, &notFoundErr) {
			logrus.Debugf("Role %s not found, creating it", cfg.RoleName)
			_, err := m.client.CreateRole(ctx, &iam.CreateRoleInput{
				RoleName:                 aws.String(cfg.RoleName),
				AssumeRolePolicyDocument: aws.String(assumeRoleDoc),
				Tags:                     tags,
			})
			if err != nil {
				return fmt.Errorf("failed to create role: %w", err)
			}
		} else {
			return fmt.Errorf("failed to get role: %w", err)
		}
	} else {
		logrus.Debugf("Role %s already exists, checking trust policy", cfg.RoleName)
		if getOut.Role.AssumeRolePolicyDocument == nil || *getOut.Role.AssumeRolePolicyDocument != assumeRoleDoc {
			_, err := m.client.UpdateAssumeRolePolicy(ctx, &iam.UpdateAssumeRolePolicyInput{
				RoleName:       aws.String(cfg.RoleName),
				PolicyDocument: aws.String(assumeRoleDoc),
			})
			if err != nil {
				return fmt.Errorf("failed to update trust policy: %w", err)
			}
			_, err = m.client.TagRole(ctx, &iam.TagRoleInput{
				RoleName: aws.String(cfg.RoleName),
				Tags:     tags,
			})
			if err != nil {
				return fmt.Errorf("failed to tag role: %w", err)
			}
		}
	}

	for _, policyArn := range cfg.PolicyArns {
		logrus.Debugf("Attaching policy %s to role %s", policyArn, cfg.RoleName)
		_, err := m.client.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{
			PolicyArn: aws.String(policyArn),
			RoleName:  aws.String(cfg.RoleName),
		})
		if err != nil {
			return fmt.Errorf("failed to attach policy %s: %w", policyArn, err)
		}
	}

	if cfg.InlinePolicyName != "" && cfg.InlinePolicyDoc != "" {
		logrus.Debugf("Putting inline policy %s for role %s", cfg.InlinePolicyName, cfg.RoleName)
		_, err := m.client.PutRolePolicy(ctx, &iam.PutRolePolicyInput{
			PolicyName:     aws.String(cfg.InlinePolicyName),
			PolicyDocument: aws.String(cfg.InlinePolicyDoc),
			RoleName:       aws.String(cfg.RoleName),
		})
		if err != nil {
			return fmt.Errorf("failed to put inline policy: %w", err)
		}
	}

	return nil
}

func generateTrustPolicy(cfg IRSAConfig) (string, error) {
	saParts := strings.Split(cfg.ServiceAccount, ":")
	if len(saParts) != 2 {
		return "", fmt.Errorf("invalid service account format, expected namespace:serviceaccount")
	}

	sub := fmt.Sprintf("system:serviceaccount:%s", cfg.ServiceAccount)
	aud := cfg.Audience

	trust := map[string]interface{}{
		"Version": "2012-10-17",
		"Statement": []map[string]interface{}{
			{
				"Effect": "Allow",
				"Principal": map[string]string{
					"Federated": cfg.OIDCProviderArn,
				},
				"Action": "sts:AssumeRoleWithWebIdentity",
				"Condition": map[string]map[string]string{
					"StringEquals": {
						fmt.Sprintf("%s:sub", cfg.OIDCProviderArn): sub,
						fmt.Sprintf("%s:aud", cfg.OIDCProviderArn): aud,
					},
				},
			},
		},
	}

	b, err := json.Marshal(trust)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
