package tests

import (
	"context"
	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

type sweepNamespace struct {
	NamespaceName string
	Client        cloudv1beta1.NamespaceServiceClient
}

func (s sweepNamespace) SweepNamespaces(r string) error {
	ctx := context.Background()
	name, err := utils.FindNamespaceByName(ctx, s.NamespaceName, s.Client)
	if err != nil {
		return err
	}

	if _, err := s.Client.DeleteNamespace(ctx, &cloudv1beta1.DeleteNamespaceRequest{
		Id: name.GetId(),
	}); err != nil {
		return err
	}
	return nil
}
