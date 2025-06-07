package irsa

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/sirupsen/logrus"
)

func (m *Manager) GarbageCollect(ctx context.Context, desired []IRSAConfig) error {
	desiredSet := make(map[string]struct{})
	for _, cfg := range desired {
		desiredSet[cfg.RoleName] = struct{}{}
	}

	roles, err := m.listTaggedRoles(ctx)
	if err != nil {
		return err
	}

	logrus.Debugf("Found %d roles with tag %s=%s", len(roles), ClusterTagKey, ClusterTagValue)
	for _, role := range roles {
		if _, exists := desiredSet[*role.RoleName]; !exists {
			logrus.Debugf("Deleting role %s as it is not in the desired set", *role.RoleName)
			_, err := m.client.DeleteRole(ctx, &iam.DeleteRoleInput{
				RoleName: role.RoleName,
			})
			if err != nil {
				return fmt.Errorf("failed to delete role %s: %w", *role.RoleName, err)
			}
		}
	}

	return nil
}

func (m *Manager) listTaggedRoles(ctx context.Context) ([]types.Role, error) {
	var roles []types.Role
	var marker *string
	for {
		out, err := m.client.ListRoles(ctx, &iam.ListRolesInput{
			Marker: marker,
		})
		if err != nil {
			return nil, err
		}
		for _, role := range out.Roles {
			tagsOut, err := m.client.ListRoleTags(ctx, &iam.ListRoleTagsInput{
				RoleName: role.RoleName,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to list tags for role %s: %w", *role.RoleName, err)
			}
			for _, tag := range tagsOut.Tags {
				if *tag.Key == ClusterTagKey && *tag.Value == ClusterTagValue {
					roles = append(roles, role)
					break
				}
			}
		}
		if out.IsTruncated {
			marker = out.Marker
		} else {
			break
		}
	}
	return roles, nil
}
