// Copyright 2023 Redpanda Data, Inc.
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

// Package utils contains multiple utility functions used across the Redpanda's
// terraform codebase
package utils

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	rpknet "github.com/redpanda-data/redpanda/src/go/rpk/pkg/net"
	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	dataplanev1alpha1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/dataplane/v1alpha1"
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

// StringToCloudProvider returns the cloudv1beta1's CloudProvider code based on
// the input string.
func StringToCloudProvider(p string) (cloudv1beta1.CloudProvider, error) {
	switch strings.ToLower(p) {
	case "aws":
		return cloudv1beta1.CloudProvider_CLOUD_PROVIDER_AWS, nil
	case "gcp":
		return cloudv1beta1.CloudProvider_CLOUD_PROVIDER_GCP, nil
	default:
		return cloudv1beta1.CloudProvider_CLOUD_PROVIDER_UNSPECIFIED, fmt.Errorf("provider %q not supported", p)
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
	}
}

// StringToClusterType returns the cloudv1beta1's Cluster_Type code based on
// the input string.
func StringToClusterType(p string) (cloudv1beta1.Cluster_Type, error) {
	switch strings.ToLower(p) {
	case "dedicated":
		return cloudv1beta1.Cluster_TYPE_DEDICATED, nil
	case "cloud":
		return cloudv1beta1.Cluster_TYPE_BYOC, nil
	default:
		return cloudv1beta1.Cluster_TYPE_UNSPECIFIED, fmt.Errorf("cluster type %q not supported", p)
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
	}
}

// AreWeDoneYet checks an operation's state until one of completion, failure or timeout is reached.
func AreWeDoneYet(ctx context.Context, op *cloudv1beta1.Operation, timeout time.Duration, waitUnit time.Duration, client cloudv1beta1.OperationServiceClient) error {
	startTime := time.Now()
	endTime := startTime.Add(timeout)
	errChan := make(chan error, 1)
	for {
		// Get the latest operation status
		latestOp, err := client.GetOperation(ctx, &cloudv1beta1.GetOperationRequest{
			Id: op.GetId(),
		})
		if err != nil {
			// Send the error to the error channel (non-blocking)
			select {
			case errChan <- fmt.Errorf("error getting operation status: %v", err):
			default:
			}
		} else {
			op = latestOp
		}

		// Check the operation state
		if op.GetState() == cloudv1beta1.Operation_STATE_COMPLETED {
			return nil
		}
		if op.GetState() == cloudv1beta1.Operation_STATE_FAILED {
			return fmt.Errorf("operation failed: %s", op.GetError().GetMessage())
		}

		// Check if the timeout has been reached
		if time.Now().After(endTime) {
			select {
			case err := <-errChan:
				return fmt.Errorf("timeout reached with error: %v", err)
			default:
				return fmt.Errorf("timeout reached")
			}
		}

		// Wait for a certain duration before checking again
		time.Sleep(waitUnit)
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
		return nil, fmt.Errorf("unable to list networks: %v", err)
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

// FindUserByName searches for a user by name using the provided client
func FindUserByName(ctx context.Context, name string, client dataplanev1alpha1.UserServiceClient) (*dataplanev1alpha1.ListUsersResponse_User, error) {
	usrs, err := client.ListUsers(ctx, &dataplanev1alpha1.ListUsersRequest{
		Filter: &dataplanev1alpha1.ListUsersRequest_Filter{
			Name: name,
		},
	})
	if err != nil {
		return nil, err
	}

	for _, v := range usrs.GetUsers() {
		if v.GetName() == name {
			return v, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

// StringToStringPointer converts a string to a pointer to a string
func StringToStringPointer(s string) *string {
	return &s
}

// StringToUserMechanism converts a string to a dataplanev1alpha1.SASLMechanism
func StringToUserMechanism(s string) dataplanev1alpha1.SASLMechanism {
	switch strings.ToLower(s) {
	case "scram-sha-256":
		return dataplanev1alpha1.SASLMechanism_SASL_MECHANISM_SCRAM_SHA_256
	case "scram-sha-512":
		return dataplanev1alpha1.SASLMechanism_SASL_MECHANISM_SCRAM_SHA_512
	default:
		return dataplanev1alpha1.SASLMechanism_SASL_MECHANISM_UNSPECIFIED
	}
}

// UserMechanismToString converts a dataplanev1alpha1.SASLMechanism to a string
func UserMechanismToString(m *dataplanev1alpha1.SASLMechanism) string {
	if m == nil {
		return "unspecified"
	}
	switch *m {
	case dataplanev1alpha1.SASLMechanism_SASL_MECHANISM_SCRAM_SHA_256:
		return "scram-sha-256"
	case dataplanev1alpha1.SASLMechanism_SASL_MECHANISM_SCRAM_SHA_512:
		return "scram-sha-512"
	default:
		return "unspecified"
	}
}

// TopicConfigurationToMap converts a slice of dataplanev1alpha1.Topic_Configuration to a slice of models.TopicConfiguration
func TopicConfigurationToMap(cfg []*dataplanev1alpha1.Topic_Configuration) (types.Map, error) {
	configs := make(map[string]attr.Value, len(cfg))
	for _, v := range cfg {
		configs[v.Name] = types.StringValue(*v.Value)
	}
	cfgMap, diag := types.MapValue(types.StringType, configs)
	if diag.HasError() {
		return types.Map{}, errors.New("unable to parse the configuration map")
	}
	return cfgMap, nil
}

// MapToCreateTopicConfiguration converts a cfg map to a slice of dataplanev1alpha1.CreateTopicRequest_Topic_Config
func MapToCreateTopicConfiguration(cfg types.Map) ([]*dataplanev1alpha1.CreateTopicRequest_Topic_Config, error) {
	var output []*dataplanev1alpha1.CreateTopicRequest_Topic_Config

	for k, v := range cfg.Elements() {
		if v.IsNull() || v.IsUnknown() {
			return nil, fmt.Errorf("topic configuration %q must have a value", k)
		}
		value := strings.Trim(v.String(), `"`)
		output = append(output, &dataplanev1alpha1.CreateTopicRequest_Topic_Config{
			Name:  k,
			Value: &value,
		})
	}
	return output, nil
}

func MapToSetTopicConfiguration(cfg types.Map) ([]*dataplanev1alpha1.SetTopicConfigurationsRequest_SetConfiguration, error) {
	var output []*dataplanev1alpha1.SetTopicConfigurationsRequest_SetConfiguration

	for k, v := range cfg.Elements() {
		if v.IsNull() || v.IsUnknown() {
			return nil, fmt.Errorf("topic configuration %q must have a value", k)
		}
		value := strings.Trim(v.String(), `"`)
		output = append(output, &dataplanev1alpha1.SetTopicConfigurationsRequest_SetConfiguration{
			Name:  k,
			Value: &value,
		})
	}
	return output, nil
}

// NumberToInt32 converts a types.Number to an *int32
func NumberToInt32(n types.Number) *int32 {
	i, _ := n.ValueBigFloat().Int64()
	i32 := int32(i)
	return &i32
}

// Int32ToNumber converts an int32 to a types.Number
func Int32ToNumber(i int32) types.Number {
	return types.NumberValue(big.NewFloat(float64(i)))
}

// TopicConfigurationSourceToString converts a dataplanev1alpha1.Topic_Configuration_Source to a string
func TopicConfigurationSourceToString(e dataplanev1alpha1.ConfigSource) string {
	switch e {
	case dataplanev1alpha1.ConfigSource_CONFIG_SOURCE_UNSPECIFIED:
		return "SOURCE_UNSPECIFIED"
	case dataplanev1alpha1.ConfigSource_CONFIG_SOURCE_DYNAMIC_TOPIC_CONFIG:
		return "DYNAMIC_TOPIC_CONFIG"
	case dataplanev1alpha1.ConfigSource_CONFIG_SOURCE_DYNAMIC_BROKER_CONFIG:
		return "DYNAMIC_BROKER_CONFIG"
	case dataplanev1alpha1.ConfigSource_CONFIG_SOURCE_DYNAMIC_DEFAULT_BROKER_CONFIG:
		return "DYNAMIC_DEFAULT_BROKER_CONFIG"
	case dataplanev1alpha1.ConfigSource_CONFIG_SOURCE_STATIC_BROKER_CONFIG:
		return "STATIC_BROKER_CONFIG"
	case dataplanev1alpha1.ConfigSource_CONFIG_SOURCE_DEFAULT_CONFIG:
		return "DEFAULT_CONFIG"
	case dataplanev1alpha1.ConfigSource_CONFIG_SOURCE_DYNAMIC_BROKER_LOGGER_CONFIG:
		return "DYNAMIC_BROKER_LOGGER_CONFIG"
	default:
		return "UNKNOWN"
	}
}

// FindTopicByName searches for a topic by name using the provided client.
func FindTopicByName(ctx context.Context, topicName string, client dataplanev1alpha1.TopicServiceClient) (*dataplanev1alpha1.ListTopicsResponse_Topic, error) {
	topics, err := client.ListTopics(ctx, &dataplanev1alpha1.ListTopicsRequest{
		Filter: &dataplanev1alpha1.ListTopicsRequest_Filter{
			NameContains: topicName,
		},
	})
	if err != nil {
		return nil, err
	}
	for _, v := range topics.GetTopics() {
		if v.GetName() == topicName {
			return v, nil
		}
	}
	return nil, fmt.Errorf("topic %s not found", topicName)
}

// SplitSchemeDefPort splits the schema from the url and return url+port. If
// there is no port, we use the provided default.
func SplitSchemeDefPort(url, def string) (string, error) {
	_, host, port, err := rpknet.SplitSchemeHostPort(url)
	if err != nil {
		return "", err
	}
	if port == "" {
		port = def
	}
	return host + ":" + port, nil
}

// GetClusterUntilRunningState returns a cluster in the running state or an error
func GetClusterUntilRunningState(ctx context.Context, count, limit int, clusterName string, client cloudv1beta1.ClusterServiceClient) (*cloudv1beta1.Cluster, error) {
	count++
	if count >= limit {
		return nil, fmt.Errorf("cluster %q did not reach the running state after %d attempts", clusterName, count)
	}
	cluster, _ := FindClusterByName(ctx, clusterName, client)

	if cluster.GetState() == cloudv1beta1.Cluster_STATE_READY {
		return cluster, nil
	}

	time.Sleep(1 * time.Minute)
	return GetClusterUntilRunningState(ctx, count, limit, clusterName, client)
}
