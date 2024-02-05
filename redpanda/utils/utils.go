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

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	rpknet "github.com/redpanda-data/redpanda/src/go/rpk/pkg/net"
	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	dataplanev1alpha1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/dataplane/v1alpha1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
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
			return fmt.Errorf("unable to get operation status: %v", err)
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

// StringToACLResourceType converts a string to a dataplanev1alpha1.ACL_ResourceType
func StringToACLResourceType(s string) (dataplanev1alpha1.ACL_ResourceType, error) {
	switch strings.ToUpper(s) {
	case "UNSPECIFIED":
		return dataplanev1alpha1.ACL_RESOURCE_TYPE_UNSPECIFIED, nil
	case "ANY":
		return dataplanev1alpha1.ACL_RESOURCE_TYPE_ANY, nil
	case "TOPIC":
		return dataplanev1alpha1.ACL_RESOURCE_TYPE_TOPIC, nil
	case "GROUP":
		return dataplanev1alpha1.ACL_RESOURCE_TYPE_GROUP, nil
	case "CLUSTER":
		return dataplanev1alpha1.ACL_RESOURCE_TYPE_CLUSTER, nil
	case "TRANSACTIONAL_ID":
		return dataplanev1alpha1.ACL_RESOURCE_TYPE_TRANSACTIONAL_ID, nil
	case "DELEGATION_TOKEN":
		return dataplanev1alpha1.ACL_RESOURCE_TYPE_DELEGATION_TOKEN, nil
	case "USER":
		return dataplanev1alpha1.ACL_RESOURCE_TYPE_USER, nil
	default:
		return -1, fmt.Errorf("unknown ACL_ResourceType: %s", s)
	}
}

// ACLResourceTypeToString converts a dataplanev1alpha1.ACL_ResourceType to a string
func ACLResourceTypeToString(e dataplanev1alpha1.ACL_ResourceType) string {
	switch e {
	case dataplanev1alpha1.ACL_RESOURCE_TYPE_UNSPECIFIED:
		return "UNSPECIFIED"
	case dataplanev1alpha1.ACL_RESOURCE_TYPE_ANY:
		return "ANY"
	case dataplanev1alpha1.ACL_RESOURCE_TYPE_TOPIC:
		return "TOPIC"
	case dataplanev1alpha1.ACL_RESOURCE_TYPE_GROUP:
		return "GROUP"
	case dataplanev1alpha1.ACL_RESOURCE_TYPE_CLUSTER:
		return "CLUSTER"
	case dataplanev1alpha1.ACL_RESOURCE_TYPE_TRANSACTIONAL_ID:
		return "TRANSACTIONAL_ID"
	case dataplanev1alpha1.ACL_RESOURCE_TYPE_DELEGATION_TOKEN:
		return "DELEGATION_TOKEN"
	case dataplanev1alpha1.ACL_RESOURCE_TYPE_USER:
		return "USER"
	default:
		return "UNKNOWN"
	}
}

// StringToACLResourcePatternType converts a string to a dataplanev1alpha1.ACL_ResourcePatternType
func StringToACLResourcePatternType(s string) (dataplanev1alpha1.ACL_ResourcePatternType, error) {
	switch strings.ToUpper(s) {
	case "UNSPECIFIED":
		return dataplanev1alpha1.ACL_RESOURCE_PATTERN_TYPE_UNSPECIFIED, nil
	case "ANY":
		return dataplanev1alpha1.ACL_RESOURCE_PATTERN_TYPE_ANY, nil
	case "MATCH":
		return dataplanev1alpha1.ACL_RESOURCE_PATTERN_TYPE_MATCH, nil
	case "LITERAL":
		return dataplanev1alpha1.ACL_RESOURCE_PATTERN_TYPE_LITERAL, nil
	case "PREFIXED":
		return dataplanev1alpha1.ACL_RESOURCE_PATTERN_TYPE_PREFIXED, nil
	default:
		return -1, fmt.Errorf("unknown ACL_ResourcePatternType: %s", s)
	}
}

// ACLResourcePatternTypeToString converts a dataplanev1alpha1.ACL_ResourcePatternType to a string
func ACLResourcePatternTypeToString(e dataplanev1alpha1.ACL_ResourcePatternType) string {
	switch e {
	case dataplanev1alpha1.ACL_RESOURCE_PATTERN_TYPE_UNSPECIFIED:
		return "UNSPECIFIED"
	case dataplanev1alpha1.ACL_RESOURCE_PATTERN_TYPE_ANY:
		return "ANY"
	case dataplanev1alpha1.ACL_RESOURCE_PATTERN_TYPE_MATCH:
		return "MATCH"
	case dataplanev1alpha1.ACL_RESOURCE_PATTERN_TYPE_LITERAL:
		return "LITERAL"
	case dataplanev1alpha1.ACL_RESOURCE_PATTERN_TYPE_PREFIXED:
		return "PREFIXED"
	default:
		return "UNKNOWN"
	}
}

// StringToACLOperation converts a string to a dataplanev1alpha1.ACL_Operation
func StringToACLOperation(s string) (dataplanev1alpha1.ACL_Operation, error) {
	switch strings.ToUpper(s) {
	case "UNSPECIFIED":
		return dataplanev1alpha1.ACL_OPERATION_UNSPECIFIED, nil
	case "ANY":
		return dataplanev1alpha1.ACL_OPERATION_ANY, nil
	case "ALL":
		return dataplanev1alpha1.ACL_OPERATION_ALL, nil
	case "READ":
		return dataplanev1alpha1.ACL_OPERATION_READ, nil
	case "WRITE":
		return dataplanev1alpha1.ACL_OPERATION_WRITE, nil
	case "CREATE":
		return dataplanev1alpha1.ACL_OPERATION_CREATE, nil
	case "DELETE":
		return dataplanev1alpha1.ACL_OPERATION_DELETE, nil
	case "ALTER":
		return dataplanev1alpha1.ACL_OPERATION_ALTER, nil
	case "DESCRIBE":
		return dataplanev1alpha1.ACL_OPERATION_DESCRIBE, nil
	case "CLUSTER_ACTION":
		return dataplanev1alpha1.ACL_OPERATION_CLUSTER_ACTION, nil
	case "DESCRIBE_CONFIGS":
		return dataplanev1alpha1.ACL_OPERATION_DESCRIBE_CONFIGS, nil
	case "ALTER_CONFIGS":
		return dataplanev1alpha1.ACL_OPERATION_ALTER_CONFIGS, nil
	case "IDEMPOTENT_WRITE":
		return dataplanev1alpha1.ACL_OPERATION_IDEMPOTENT_WRITE, nil
	case "CREATE_TOKENS":
		return dataplanev1alpha1.ACL_OPERATION_CREATE_TOKENS, nil
	case "DESCRIBE_TOKENS":
		return dataplanev1alpha1.ACL_OPERATION_DESCRIBE_TOKENS, nil
	default:
		return -1, fmt.Errorf("unknown ACL_Operation: %s", s)
	}
}

// ACLOperationToString converts a dataplanev1alpha1.ACL_Operation to a string
func ACLOperationToString(e dataplanev1alpha1.ACL_Operation) string {
	switch e {
	case dataplanev1alpha1.ACL_OPERATION_UNSPECIFIED:
		return "UNSPECIFIED"
	case dataplanev1alpha1.ACL_OPERATION_ANY:
		return "ANY"
	case dataplanev1alpha1.ACL_OPERATION_ALL:
		return "ALL"
	case dataplanev1alpha1.ACL_OPERATION_READ:
		return "READ"
	case dataplanev1alpha1.ACL_OPERATION_WRITE:
		return "WRITE"
	case dataplanev1alpha1.ACL_OPERATION_CREATE:
		return "CREATE"
	case dataplanev1alpha1.ACL_OPERATION_DELETE:
		return "DELETE"
	case dataplanev1alpha1.ACL_OPERATION_ALTER:
		return "ALTER"
	case dataplanev1alpha1.ACL_OPERATION_DESCRIBE:
		return "DESCRIBE"
	case dataplanev1alpha1.ACL_OPERATION_CLUSTER_ACTION:
		return "CLUSTER_ACTION"
	case dataplanev1alpha1.ACL_OPERATION_DESCRIBE_CONFIGS:
		return "DESCRIBE_CONFIGS"
	case dataplanev1alpha1.ACL_OPERATION_ALTER_CONFIGS:
		return "ALTER_CONFIGS"
	case dataplanev1alpha1.ACL_OPERATION_IDEMPOTENT_WRITE:
		return "IDEMPOTENT_WRITE"
	case dataplanev1alpha1.ACL_OPERATION_CREATE_TOKENS:
		return "CREATE_TOKENS"
	case dataplanev1alpha1.ACL_OPERATION_DESCRIBE_TOKENS:
		return "DESCRIBE_TOKENS"
	default:
		return "UNKNOWN"
	}
}

// StringToACLPermissionType converts a string to a dataplanev1alpha1.ACL_PermissionType
func StringToACLPermissionType(s string) (dataplanev1alpha1.ACL_PermissionType, error) {
	switch strings.ToUpper(s) {
	case "UNSPECIFIED":
		return dataplanev1alpha1.ACL_PERMISSION_TYPE_UNSPECIFIED, nil
	case "ANY":
		return dataplanev1alpha1.ACL_PERMISSION_TYPE_ANY, nil
	case "DENY":
		return dataplanev1alpha1.ACL_PERMISSION_TYPE_DENY, nil
	case "ALLOW":
		return dataplanev1alpha1.ACL_PERMISSION_TYPE_ALLOW, nil
	default:
		return -1, fmt.Errorf("unknown ACL_PermissionType: %s", s)
	}
}

// ACLPermissionTypeToString converts a dataplanev1alpha1.ACL_PermissionType to a string
func ACLPermissionTypeToString(e dataplanev1alpha1.ACL_PermissionType) string {
	switch e {
	case dataplanev1alpha1.ACL_PERMISSION_TYPE_UNSPECIFIED:
		return "UNSPECIFIED"
	case dataplanev1alpha1.ACL_PERMISSION_TYPE_ANY:
		return "ANY"
	case dataplanev1alpha1.ACL_PERMISSION_TYPE_DENY:
		return "DENY"
	case dataplanev1alpha1.ACL_PERMISSION_TYPE_ALLOW:
		return "ALLOW"
	default:
		return "UNKNOWN"
	}
}

// TopicConfigurationToSlice converts a slice of dataplanev1alpha1.Topic_Configuration to a slice of models.TopicConfiguration
func TopicConfigurationToSlice(cfg []*dataplanev1alpha1.Topic_Configuration) ([]*models.TopicConfiguration, error) {
	output := make([]*models.TopicConfiguration, len(cfg))
	for _, v := range cfg {
		cfgType, err := TopicConfigurationTypeToString(v.Type)
		if err != nil {
			return nil, err
		}
		output = append(output, &models.TopicConfiguration{
			Name:           types.StringValue(v.Name),
			Type:           types.StringValue(cfgType),
			Value:          types.StringValue(*v.Value),
			Source:         types.StringValue(TopicConfigurationSourceToString(v.Source)),
			IsReadOnly:     types.BoolValue(v.IsReadOnly),
			IsSensitive:    types.BoolValue(v.IsSensitive),
			ConfigSynonyms: ConfigSynonymsToSlice(v.ConfigSynonyms),
			Documentation:  types.StringValue(*v.Documentation),
		})
	}
	return output, nil
}

// ConfigSynonymsToSlice converts a slice of dataplanev1alpha1.Topic_Configuration_ConfigSynonym to a slice of models.TopicConfigSynonym
func ConfigSynonymsToSlice(synonyms []*dataplanev1alpha1.ConfigSynonym) []*models.TopicConfigSynonym {
	output := make([]*models.TopicConfigSynonym, len(synonyms))
	for _, v := range synonyms {
		output = append(output, &models.TopicConfigSynonym{
			Name:   types.StringValue(v.Name),
			Value:  types.StringValue(*v.Value),
			Source: types.StringValue(TopicConfigurationSourceToString(v.Source)),
		})
	}
	return output
}

// SliceToTopicConfiguration converts a slice of models.TopicConfiguration to a slice of dataplanev1alpha1.Topic_Configuration
func SliceToTopicConfiguration(cfg []*models.TopicConfiguration) ([]*dataplanev1alpha1.CreateTopicRequest_Topic_Config, error) {
	output := make([]*dataplanev1alpha1.CreateTopicRequest_Topic_Config, len(cfg))
	for _, v := range cfg {
		value := v.Value.ValueString()
		output = append(output, &dataplanev1alpha1.CreateTopicRequest_Topic_Config{
			Name:  v.Name.ValueString(),
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

// TopicConfigurationTypeToString converts a dataplanev1alpha1.ConfigType to a string
func TopicConfigurationTypeToString(t dataplanev1alpha1.ConfigType) (string, error) {
	v, ok := dataplanev1alpha1.ConfigType_name[int32(t)]
	if !ok {
		return "", fmt.Errorf("unknown configuration type: %v", t)
	}
	return v, nil
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
