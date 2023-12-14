package utils

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
)

const providerUnspecified = "unspecified"

// IsNotFound checks if the passed error is a Not Found error or if it has a
// 404 code in the error message.
func IsNotFound(err error) bool {
	if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "404") {
		return true
	}
	return false
}

// TODO check more to see if the client handles this

// StringToCloudProvider returns the cloudv1beta1's CloudProvider code based on
// the input string.
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

// CloudProviderToString returns the cloud provider string based on the
// cloudv1beta1's CloudProvider code.
func CloudProviderToString(provider cloudv1beta1.CloudProvider) string {
	switch provider {
	case cloudv1beta1.CloudProvider_CLOUD_PROVIDER_AWS:
		return "aws"
	case cloudv1beta1.CloudProvider_CLOUD_PROVIDER_GCP:
		return "gcp"
	default:
		return providerUnspecified
		// TODO should we error here?
	}
}

// StringToClusterType returns the cloudv1beta1's Cluster_Type code based on
// the input string.
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

// ClusterTypeToString returns the cloud cluster type string based on the
// cloudv1beta1's Cluster_Type code.
func ClusterTypeToString(provider cloudv1beta1.Cluster_Type) string {
	switch provider {
	case cloudv1beta1.Cluster_TYPE_DEDICATED:
		return "dedicated"
	case cloudv1beta1.Cluster_TYPE_BYOC:
		return "cloud"
	default:
		return providerUnspecified
		// TODO should we error here?
	}
}

// AreWeDoneYet checks the status of a given operation until it either completes
// successfully, encounters an error, or reaches a timeout.
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
				if !IsNotFound(errors.New(o.GetError().GetMessage())) {
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

// CheckOpsState checks if the op.State is either complete or failed, otherwise
// it returns false.
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

// StringToConnectionType returns the cloudv1beta1's Cluster_ConnectionType code
// based on the input string.
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

// ConnectionTypeToString returns the cloud cluster connection type string based
// on the cloudv1beta1's Cluster_ConnectionType code.
func ConnectionTypeToString(t cloudv1beta1.Cluster_ConnectionType) string {
	switch t {
	case cloudv1beta1.Cluster_CONNECTION_TYPE_PUBLIC:
		return "public"
	case cloudv1beta1.Cluster_CONNECTION_TYPE_PRIVATE:
		return "private"
	default:
		return providerUnspecified
	}
}

// TypeListToStringSlice converts a types.List to a []string, stripping
// surrounding quotes for each element.
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

// FindNamespaceByName searches for a namespace by name using the provided
// client. It queries the namespaces and returns the first match by name or an
// error if not found.
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
	return nil, fmt.Errorf("namespace %s not found", n)
}

// FindNetworkByName searches for a network by name using the provided client.
// It queries the networks and returns the first match by name or an error if
// not found.
func FindNetworkByName(ctx context.Context, n string, client cloudv1beta1.NetworkServiceClient) (*cloudv1beta1.Network, error) {
	ns, err := client.ListNetworks(ctx, &cloudv1beta1.ListNetworksRequest{
		Filter: &cloudv1beta1.ListNetworksRequest_Filter{Name: n},
	})
	if err != nil {
		return nil, err
	}
	for _, v := range ns.GetNetworks() {
		if v.GetName() == n {
			return v, nil
		}
	}
	return nil, fmt.Errorf("network not found")
}

// FindClusterByName searches for a cluster by name using the provided client.
// It queries the clusters and returns the first match by name or an error if
// not found.
func FindClusterByName(ctx context.Context, n string, client cloudv1beta1.ClusterServiceClient) (*cloudv1beta1.Cluster, error) {
	ns, err := client.ListClusters(ctx, &cloudv1beta1.ListClustersRequest{
		Filter: &cloudv1beta1.ListClustersRequest_Filter{Name: n},
	})
	if err != nil {
		return nil, err
	}
	for _, v := range ns.GetClusters() {
		if v.GetName() == n {
			return v, nil
		}
	}
	return nil, fmt.Errorf("cluster not found")
}
