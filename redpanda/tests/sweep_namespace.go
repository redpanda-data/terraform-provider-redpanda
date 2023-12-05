package tests

import (
	"context"
	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/clients"
)

type sweepNamespace struct {
	AccNamePrepend string
	NamespaceName  string
	ClientId       string
	ClientSecret   string
	Version        string
}

func (s sweepNamespace) SweepNamespaces(r string) error {
	ctx := context.Background()
	client, err := clients.NewNamespaceServiceClient(ctx, s.Version, clients.ClientRequest{
		ClientID:     s.ClientId,
		ClientSecret: s.ClientSecret,
	})
	if err != nil {
		return err
	}

	namespaces, err := client.ListNamespaces(ctx, &cloudv1beta1.ListNamespacesRequest{
		PageSize: 100,
		Filter: &cloudv1beta1.ListNamespacesRequest_Filter{
			Name: s.AccNamePrepend + s.NamespaceName,
		},
	})
	if err != nil {
		return err
	}

	for _, v := range namespaces.GetNamespaces() {
		if v.GetName() == s.AccNamePrepend+s.NamespaceName {
			_, err := client.DeleteNamespace(ctx, &cloudv1beta1.DeleteNamespaceRequest{
				Id: v.GetId(),
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}
