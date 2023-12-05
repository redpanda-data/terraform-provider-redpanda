package utils

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	"strings"
	"time"
)

func IsNotFound(err error) bool {
	if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "404") {
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
				if !IsNotFound(fmt.Errorf(o.GetError().GetMessage())) {
					return nil
				}
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

func StringToConnectionType(s string) cloudv1beta1.Cluster_ConnectionType {
	switch strings.ToLower(s) {
	case "public":
		return cloudv1beta1.Cluster_CONNECTION_TYPE_PUBLIC
	case "private":
		return cloudv1beta1.Cluster_CONNECTION_TYPE_PRIVATE
	default:
		return cloudv1beta1.Cluster_CONNECTION_TYPE_UNSPECIFIED
	}
}

func ConnectionTypeToString(t cloudv1beta1.Cluster_ConnectionType) string {
	switch t {
	case cloudv1beta1.Cluster_CONNECTION_TYPE_PUBLIC:
		return "public"
	case cloudv1beta1.Cluster_CONNECTION_TYPE_PRIVATE:
		return "private"
	default:
		return "unspecified"
	}
}

func TypeListToStringSlice(t types.List) []string {
	var s []string
	for _, v := range t.Elements() {
		s = append(s, strings.Trim(v.String(), "\"")) // it's easier to strip the quotes than type coverting until you hit something that doesn't include them
	}
	return s
}

// TestingOnlyStringSliceToTypeList converts a string slice to a types.List. Only use for testing as it swallows the diag
func TestingOnlyStringSliceToTypeList(s []string) types.List {
	o, _ := types.ListValueFrom(context.TODO(), types.StringType, s)
	return o
}

// TrimmedStringValue returns the string value of a types.String with the quotes removed.
// This is necessary as terraform has a tendency to slap these bad boys in at random which causes the API to fail
func TrimmedStringValue(s string) types.String {
	return basetypes.NewStringValue(strings.Trim(s, "\""))
}

// TrimmedString returns the string value of a types.String with the quotes removed.
func TrimmedString(s types.String) string {
	return strings.Trim(s.String(), "\"")
}

func FindNamespaceByName(ctx context.Context, n string, client cloudv1beta1.NamespaceServiceClient) (*cloudv1beta1.Namespace, error) {
	ns, err := client.ListNamespaces(ctx, &cloudv1beta1.ListNamespacesRequest{
		Filter: &cloudv1beta1.ListNamespacesRequest_Filter{Name: n},
	})
	if err != nil {
		return nil, err
	}
	for _, v := range ns.GetNamespaces() {
		if v.GetName() == n {
			return v, nil
		}
	}
	return nil, fmt.Errorf("namespace not found")
}
