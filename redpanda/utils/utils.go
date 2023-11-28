package utils

import (
	"context"
	"fmt"
	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	"strings"
	"time"
)

func IsNotFound(err error) bool {
	if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
		return true
	}
	return false
}

// TODO check more to see if the client handles this

func StringToCloudProvider(p string) cloudv1beta1.CloudProvider {
	switch strings.ToLower(p) {
	case "aws":
		return cloudv1beta1.CloudProvider_CLOUD_PROVIDER_AWS
	case "gcp":
		return cloudv1beta1.CloudProvider_CLOUD_PROVIDER_GCP
	default:
		return cloudv1beta1.CloudProvider_CLOUD_PROVIDER_UNSPECIFIED
		// TODO should we error here?
	}
}

func CloudProviderToString(provider cloudv1beta1.CloudProvider) string {
	switch provider {
	case cloudv1beta1.CloudProvider_CLOUD_PROVIDER_AWS:
		return "aws"
	case cloudv1beta1.CloudProvider_CLOUD_PROVIDER_GCP:
		return "gcp"
	default:
		return "unspecified"
		// TODO should we error here?
	}
}

func StringToClusterType(p string) cloudv1beta1.Cluster_Type {
	switch strings.ToLower(p) {
	case "dedicated":
		return cloudv1beta1.Cluster_TYPE_DEDICATED
	case "cloud":
		return cloudv1beta1.Cluster_TYPE_BYOC
	default:
		return cloudv1beta1.Cluster_TYPE_UNSPECIFIED
		// TODO should we error here?
	}
}

func ClusterTypeToString(provider cloudv1beta1.Cluster_Type) string {
	switch provider {
	case cloudv1beta1.Cluster_TYPE_DEDICATED:
		return "dedicated"
	case cloudv1beta1.Cluster_TYPE_BYOC:
		return "cloud"
	default:
		return "unspecified"
		// TODO should we error here?
	}
}

func AreWeDoneYet(ctx context.Context, op *cloudv1beta1.Operation, timeout time.Duration, client cloudv1beta1.OperationServiceClient) error {
	if CheckOpsState(op) {
		if op.GetError() != nil {
			return fmt.Errorf("operation failed: %s", op.GetError().GetMessage())
		}
		return nil
	}
	startTime := time.Now()
	for {
		o, err := client.GetOperation(ctx, &cloudv1beta1.GetOperationRequest{
			Id: op.GetId(),
		})
		if err != nil {
			return err
		}
		if CheckOpsState(o) {
			if o.GetError() != nil {
				return fmt.Errorf("operation failed: %s", o.GetError().GetMessage())
			}
			return nil
		}
		if time.Since(startTime) > timeout {
			return fmt.Errorf("timeout reached")
		}
		time.Sleep(10 * time.Second)
	}
}

func CheckOpsState(op *cloudv1beta1.Operation) bool {
	switch op.GetState() {
	case cloudv1beta1.Operation_STATE_COMPLETED:
		return true
	case cloudv1beta1.Operation_STATE_FAILED:
		return true
	default:
		return false
	}
}
