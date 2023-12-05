package tests

import (
	"context"
	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
)

type sweepNamespace struct {
	NamespaceName string
	Client        cloudv1beta1.NamespaceServiceClient
}

func (s sweepNamespace) SweepNamespaces(r string) error {
	ctx := context.Background()

	namespaces, err := s.Client.ListNamespaces(ctx, &cloudv1beta1.ListNamespacesRequest{
		PageSize: 100,
		Filter: &cloudv1beta1.ListNamespacesRequest_Filter{
			Name: s.NamespaceName,
		},
	})
	if err != nil {
		return err
	}

	for _, v := range namespaces.GetNamespaces() {
		if v.GetName() == s.NamespaceName {
			_, err := s.Client.DeleteNamespace(ctx, &cloudv1beta1.DeleteNamespaceRequest{
				Id: v.GetId(),
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}
