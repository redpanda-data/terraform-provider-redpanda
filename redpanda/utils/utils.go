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
	"math"
	"math/big"
	"strings"
	"time"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1beta2/controlplanev1beta2grpc"
	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1alpha2/dataplanev1alpha2grpc"
	dataplanev1alpha2 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1alpha2"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	rpknet "github.com/redpanda-data/redpanda/src/go/rpk/pkg/net"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

const providerUnspecified = "unspecified"

// NotFoundError represents a resource that couldn't be found
type NotFoundError struct {
	Message string
}

// Error returns the error message
func (e NotFoundError) Error() string {
	return e.Message
}

// IsNotFound checks if the passed error is a NotFoundError or a GRPC NotFound error
func IsNotFound(err error) bool {
	if errors.As(err, &NotFoundError{}) {
		return true
	}
	if e, ok := grpcstatus.FromError(err); ok && e.Code() == grpccodes.NotFound {
		return true
	}
	return false
}

// CloudProviderStringAws is the string representation of the CLOUD_PROVIDER_AWS enum
const CloudProviderStringAws = "aws"

// CloudProviderStringAzure is the string representation of the CLOUD_PROVIDER_AZURE enum
const CloudProviderStringAzure = "azure"

// CloudProviderStringGcp is the string representation of the CLOUD_PROVIDER_GCP enum
const CloudProviderStringGcp = "gcp"

// StringToCloudProvider returns the controlplanev1beta2's CloudProvider code based on
// the input string.
func StringToCloudProvider(p string) (controlplanev1beta2.CloudProvider, error) {
	switch strings.ToLower(p) {
	case CloudProviderStringAws:
		return controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_AWS, nil
	case CloudProviderStringGcp:
		return controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_GCP, nil
	case CloudProviderStringAzure:
		return controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_AZURE, nil
	default:
		return controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_UNSPECIFIED, fmt.Errorf("provider %q not supported", p)
	}
}

// CloudProviderToString returns the cloud provider string based on the
// controlplanev1beta2's CloudProvider code.
func CloudProviderToString(provider controlplanev1beta2.CloudProvider) string {
	switch provider {
	case controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_AWS:
		return CloudProviderStringAws
	case controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_GCP:
		return CloudProviderStringGcp
	case controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_AZURE:
		return CloudProviderStringAzure
	default:
		return providerUnspecified
	}
}

// StringToClusterType returns the controlplanev1beta2's Cluster_Type code based on
// the input string.
func StringToClusterType(p string) (controlplanev1beta2.Cluster_Type, error) {
	switch strings.ToLower(p) {
	case "dedicated":
		return controlplanev1beta2.Cluster_TYPE_DEDICATED, nil
	case "byoc":
		return controlplanev1beta2.Cluster_TYPE_BYOC, nil
	default:
		return controlplanev1beta2.Cluster_TYPE_UNSPECIFIED, fmt.Errorf("cluster type %q not supported", p)
	}
}

// ClusterTypeToString returns the cloud cluster type string based on the
// controlplanev1beta2's Cluster_Type code.
func ClusterTypeToString(provider controlplanev1beta2.Cluster_Type) string {
	switch provider {
	case controlplanev1beta2.Cluster_TYPE_DEDICATED:
		return "dedicated"
	case controlplanev1beta2.Cluster_TYPE_BYOC:
		return "byoc"
	default:
		return providerUnspecified
	}
}

// AreWeDoneYet checks an operation's state until one of completion, failure or timeout is reached.
func AreWeDoneYet(ctx context.Context, op *controlplanev1beta2.Operation, timeout time.Duration, client controlplanev1beta2grpc.OperationServiceClient) error {
	return Retry(ctx, timeout, func() *RetryError {
		// Get the latest operation status
		tflog.Info(ctx, "getting operation")
		latestOp, err := client.GetOperation(ctx, &controlplanev1beta2.GetOperationRequest{
			Id: op.GetId(),
		})
		tflog.Info(ctx, "got result of operation")
		if err != nil {
			return NonRetryableError(err)
		}
		op = latestOp.Operation
		tflog.Info(ctx, fmt.Sprintf("op %v %s", op, op.GetState()))

		// Check the operation state
		if op.GetState() == controlplanev1beta2.Operation_STATE_FAILED {
			return NonRetryableError(fmt.Errorf("operation failed: %s", op.GetError().GetMessage()))
		}
		if op.GetState() != controlplanev1beta2.Operation_STATE_COMPLETED {
			return RetryableError(fmt.Errorf("expected operation to be completed but was in state %s", op.GetState()))
		}
		return nil
	})
}

// StringToConnectionType returns the controlplanev1beta2's Cluster_ConnectionType code
// based on the input string.
func StringToConnectionType(s string) controlplanev1beta2.Cluster_ConnectionType {
	switch strings.ToLower(s) {
	case "public":
		return controlplanev1beta2.Cluster_CONNECTION_TYPE_PUBLIC
	case "private":
		return controlplanev1beta2.Cluster_CONNECTION_TYPE_PRIVATE
	default:
		return controlplanev1beta2.Cluster_CONNECTION_TYPE_UNSPECIFIED
	}
}

// ConnectionTypeToString returns the cloud cluster connection type string based
// on the controlplanev1beta2's Cluster_ConnectionType code.
func ConnectionTypeToString(t controlplanev1beta2.Cluster_ConnectionType) string {
	switch t {
	case controlplanev1beta2.Cluster_CONNECTION_TYPE_PUBLIC:
		return "public"
	case controlplanev1beta2.Cluster_CONNECTION_TYPE_PRIVATE:
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
		s = append(s, strings.Trim(v.String(), "\"")) // it's easier to strip the quotes than type converting until you hit something that doesn't include them
	}
	return s
}

// StringSliceToTypeList safely converts a string slice into a Terraform types.List
func StringSliceToTypeList(elements []string) types.List {
	if elements == nil {
		return types.ListNull(types.StringType)
	}

	values := []attr.Value{}
	for _, e := range elements {
		values = append(values, types.StringValue(e))
	}
	// this is safe because ListValueMust only panics if the values don't match the list
	// type, and we're making sure that all values that go in are strings.
	return types.ListValueMust(types.StringType, values)
}

// TrimmedStringValue returns the string value of a types.String with the quotes removed.
// This is necessary as terraform has a tendency to slap these bad boys in at random which causes the API to fail
func TrimmedStringValue(s string) types.String {
	return basetypes.NewStringValue(strings.Trim(s, "\""))
}

// FindUserByName searches for a user by name using the provided client
func FindUserByName(ctx context.Context, name string, client dataplanev1alpha2grpc.UserServiceClient) (*dataplanev1alpha2.ListUsersResponse_User, error) {
	usrs, err := client.ListUsers(ctx, &dataplanev1alpha2.ListUsersRequest{
		Filter: &dataplanev1alpha2.ListUsersRequest_Filter{
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
	return nil, NotFoundError{fmt.Sprintf("user %q not found", name)}
}

// StringToStringPointer converts a string to a pointer to a string
func StringToStringPointer(s string) *string {
	return &s
}

// StringToUserMechanism converts a string to a dataplanev1alpha2.SASLMechanism
func StringToUserMechanism(s string) dataplanev1alpha2.SASLMechanism {
	switch strings.ToLower(s) {
	case "scram-sha-256":
		return dataplanev1alpha2.SASLMechanism_SASL_MECHANISM_SCRAM_SHA_256
	case "scram-sha-512":
		return dataplanev1alpha2.SASLMechanism_SASL_MECHANISM_SCRAM_SHA_512
	default:
		return dataplanev1alpha2.SASLMechanism_SASL_MECHANISM_UNSPECIFIED
	}
}

// UserMechanismToString converts a dataplanev1alpha2.SASLMechanism to a string
func UserMechanismToString(m *dataplanev1alpha2.SASLMechanism) string {
	if m == nil {
		return "unspecified"
	}
	switch *m {
	case dataplanev1alpha2.SASLMechanism_SASL_MECHANISM_SCRAM_SHA_256:
		return "scram-sha-256"
	case dataplanev1alpha2.SASLMechanism_SASL_MECHANISM_SCRAM_SHA_512:
		return "scram-sha-512"
	default:
		return "unspecified"
	}
}

// TopicConfigurationToMap converts a slice of dataplanev1alpha2.Topic_Configuration to a slice of
// models.TopicConfiguration
func TopicConfigurationToMap(cfg []*dataplanev1alpha2.Topic_Configuration) (types.Map, error) {
	configs := make(map[string]attr.Value, len(cfg))
	for _, v := range cfg {
		if v.Value == nil {
			return types.Map{}, fmt.Errorf("nil value for topic configuration %q", v.Name)
		}
		configs[v.Name] = types.StringValue(*v.Value)
	}
	cfgMap, diag := types.MapValue(types.StringType, configs)
	if diag.HasError() {
		return types.Map{}, errors.New("unable to parse the configuration map")
	}
	return cfgMap, nil
}

// MapToCreateTopicConfiguration converts a cfg map to a slice of dataplanev1alpha2.CreateTopicRequest_Topic_Config
func MapToCreateTopicConfiguration(cfg types.Map) ([]*dataplanev1alpha2.CreateTopicRequest_Topic_Config, error) {
	var output []*dataplanev1alpha2.CreateTopicRequest_Topic_Config

	for k, v := range cfg.Elements() {
		if v.IsNull() || v.IsUnknown() {
			return nil, fmt.Errorf("topic configuration %q must have a value", k)
		}
		value := strings.Trim(v.String(), `"`)
		output = append(output, &dataplanev1alpha2.CreateTopicRequest_Topic_Config{
			Name:  k,
			Value: &value,
		})
	}
	return output, nil
}

// MapToSetTopicConfiguration converts a cfg map to a slice of
// dataplanev1alpha2.SetTopicConfigurationsRequest_SetConfiguration
func MapToSetTopicConfiguration(cfg types.Map) ([]*dataplanev1alpha2.SetTopicConfigurationsRequest_SetConfiguration, error) {
	var output []*dataplanev1alpha2.SetTopicConfigurationsRequest_SetConfiguration

	for k, v := range cfg.Elements() {
		if v.IsNull() || v.IsUnknown() {
			return nil, fmt.Errorf("topic configuration %q must have a value", k)
		}
		value := strings.Trim(v.String(), `"`)
		output = append(output, &dataplanev1alpha2.SetTopicConfigurationsRequest_SetConfiguration{
			Name:  k,
			Value: &value,
		})
	}
	return output, nil
}

// NumberToInt32 converts a types.Number to an *int32
func NumberToInt32(n types.Number) *int32 {
	i, _ := n.ValueBigFloat().Int64()
	var i32 int32
	switch {
	case i > math.MaxInt32:
		i32 = math.MaxInt32
	case i < math.MinInt32:
		i32 = math.MinInt32
	default:
		i32 = int32(i)
	}
	return &i32
}

// Int32ToNumber converts an int32 to a types.Number
func Int32ToNumber(i int32) types.Number {
	return types.NumberValue(big.NewFloat(float64(i)))
}

// FindTopicByName searches for a topic by name using the provided client.
func FindTopicByName(ctx context.Context, topicName string, client dataplanev1alpha2grpc.TopicServiceClient) (*dataplanev1alpha2.ListTopicsResponse_Topic, error) {
	topics, err := client.ListTopics(ctx, &dataplanev1alpha2.ListTopicsRequest{
		Filter: &dataplanev1alpha2.ListTopicsRequest_Filter{
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
	return nil, NotFoundError{fmt.Sprintf("topic %s not found", topicName)}
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

// RetryGetCluster will retry a function, passing in the latest state of the given cluster id, until
// it either no longer returns an error or times out
func RetryGetCluster(ctx context.Context, timeout time.Duration, clusterID string, client *cloud.ControlPlaneClientSet, f func(*controlplanev1beta2.Cluster) *RetryError) (*controlplanev1beta2.Cluster, error) {
	var cluster *controlplanev1beta2.Cluster
	err := Retry(ctx, timeout, func() *RetryError {
		var err error
		cluster, err = client.ClusterForID(ctx, clusterID)
		if err != nil {
			if IsNotFound(err) {
				tflog.Info(ctx, fmt.Sprintf("cluster %q not found", clusterID))
				cluster = nil
				return nil
			}
			return NonRetryableError(err)
		}
		tflog.Info(ctx, fmt.Sprintf("cluster %v : %v", clusterID, cluster.GetState()))
		return f(cluster)
	})
	return cluster, err
}

// TypeMapToStringMap converts a types.Map to a map[string]string
func TypeMapToStringMap(tags types.Map) map[string]string {
	tagsMap := make(map[string]string)
	for k, v := range tags.Elements() {
		tagsMap[k] = strings.ReplaceAll(strings.ReplaceAll(v.String(), "\\", ""), "\"", "")
	}
	if len(tagsMap) == 0 {
		return nil
	}
	return tagsMap
}
