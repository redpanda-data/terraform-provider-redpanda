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
	"reflect"
	"strings"
	"time"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1/dataplanev1grpc"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	rpknet "github.com/redpanda-data/redpanda/src/go/rpk/pkg/net"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/resource"
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
	if err == nil {
		return false
	}
	if errors.As(err, &NotFoundError{}) {
		return true
	}
	if e, ok := grpcstatus.FromError(err); ok && e.Code() == grpccodes.NotFound {
		return true
	}
	errStr := err.Error()
	return strings.Contains(strings.ToLower(errStr), "not found") ||
		strings.Contains(errStr, "404") ||
		strings.Contains(strings.ToLower(errStr), "does not exist")
}

// IsPermissionDenied checks if the error indicates a permission/ACL issue
// This includes both gRPC PermissionDenied and HTTP 403/Forbidden errors
func IsPermissionDenied(err error) bool {
	if err == nil {
		return false
	}

	if e, ok := grpcstatus.FromError(err); ok && e.Code() == grpccodes.PermissionDenied {
		return true
	}

	errStr := err.Error()
	return strings.Contains(strings.ToLower(errStr), "forbidden") ||
		strings.Contains(strings.ToLower(errStr), "missing required acls") ||
		strings.Contains(errStr, "403")
}

// IsClusterUnreachable checks if the error indicates the cluster is unreachable
// This typically happens when the cluster is down or DNS cannot resolve the addresses.
//
// IMPORTANT: This should be used in Read operations to determine if a resource should be
// removed from state. However, deletion prevention (allow_deletion) must still be respected:
// - If allow_deletion=false, the resource should remain in state even if unreachable
// - If allow_deletion=true (or not set), the resource can be removed from state
func IsClusterUnreachable(err error) bool {
	if err == nil {
		return false
	}

	if e, ok := grpcstatus.FromError(err); ok && e.Code() == grpccodes.Unavailable {
		if strings.Contains(e.Message(), "name resolver error") &&
			strings.Contains(e.Message(), "produced zero addresses") {
			return true
		}
	}

	errStr := err.Error()
	return strings.Contains(errStr, "name resolver error") &&
		strings.Contains(errStr, "produced zero addresses")
}

// CloudProviderStringAws is the string representation of the CLOUD_PROVIDER_AWS enum
const CloudProviderStringAws = "aws"

// CloudProviderStringAzure is the string representation of the CLOUD_PROVIDER_AZURE enum
const CloudProviderStringAzure = "azure"

// CloudProviderStringGcp is the string representation of the CLOUD_PROVIDER_GCP enum
const CloudProviderStringGcp = "gcp"

// StringToCloudProvider returns the controlplanev1's CloudProvider code based on
// the input string.
func StringToCloudProvider(p string) (controlplanev1.CloudProvider, error) {
	switch strings.ToLower(p) {
	case CloudProviderStringAws:
		return controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS, nil
	case CloudProviderStringGcp:
		return controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP, nil
	case CloudProviderStringAzure:
		return controlplanev1.CloudProvider_CLOUD_PROVIDER_AZURE, nil
	default:
		return controlplanev1.CloudProvider_CLOUD_PROVIDER_UNSPECIFIED, fmt.Errorf("provider %q not supported", p)
	}
}

// StringToCloudProviderBeta returns the controlplanev1's CloudProvider code based on
// the input string.
func StringToCloudProviderBeta(p string) (controlplanev1beta2.CloudProvider, error) {
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
// controlplanev1's CloudProvider code.
func CloudProviderToString(provider controlplanev1.CloudProvider) string {
	switch provider {
	case controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS:
		return CloudProviderStringAws
	case controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP:
		return CloudProviderStringGcp
	case controlplanev1.CloudProvider_CLOUD_PROVIDER_AZURE:
		return CloudProviderStringAzure
	default:
		return providerUnspecified
	}
}

// TODO: remove this when throughput tier is migrated

// CloudProviderToStringBeta returns the cloud provider string based on the
// controlplanev1beta2's CloudProvider code.
func CloudProviderToStringBeta(provider controlplanev1beta2.CloudProvider) string {
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

// StringToClusterType returns the controlplanev1's Cluster_Type code based on
// the input string.
func StringToClusterType(p string) (controlplanev1.Cluster_Type, error) {
	switch strings.ToLower(p) {
	case "dedicated":
		return controlplanev1.Cluster_TYPE_DEDICATED, nil
	case "byoc":
		return controlplanev1.Cluster_TYPE_BYOC, nil
	default:
		return controlplanev1.Cluster_TYPE_UNSPECIFIED, fmt.Errorf("cluster type %q not supported", p)
	}
}

// ClusterTypeToString returns the cloud cluster type string based on the
// controlplanev1's Cluster_Type code.
func ClusterTypeToString(provider controlplanev1.Cluster_Type) string {
	switch provider {
	case controlplanev1.Cluster_TYPE_DEDICATED:
		return "dedicated"
	case controlplanev1.Cluster_TYPE_BYOC:
		return "byoc"
	default:
		return providerUnspecified
	}
}

// AreWeDoneYet checks an operation's state until one of completion, failure or timeout is reached.
func AreWeDoneYet(ctx context.Context, op *controlplanev1.Operation, timeout time.Duration, client controlplanev1grpc.OperationServiceClient) error {
	return Retry(ctx, timeout, func() *RetryError {
		tflog.Info(ctx, "getting operation")
		latestOp, err := client.GetOperation(ctx, &controlplanev1.GetOperationRequest{
			Id: op.GetId(),
		})
		tflog.Info(ctx, "got result of operation")
		if err != nil {
			return NonRetryableError(err)
		}
		op = latestOp.Operation

		if op != nil {
			tflog.Info(ctx, fmt.Sprintf("op %v %s", op, op.GetState()))
		} else {
			tflog.Info(ctx, "op is nil")
		}

		if op != nil && op.GetState() == controlplanev1.Operation_STATE_FAILED {
			return NonRetryableError(fmt.Errorf("operation failed: %s", op.GetError().GetMessage()))
		}
		if op != nil && op.GetState() != controlplanev1.Operation_STATE_COMPLETED {
			return RetryableError(fmt.Errorf("expected operation to be completed but was in state %s", op.GetState()))
		}
		return nil
	})
}

// StringToConnectionType returns the controlplanev1's Cluster_ConnectionType code
// based on the input string.
func StringToConnectionType(s string) controlplanev1.Cluster_ConnectionType {
	switch strings.ToLower(s) {
	case "public":
		return controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC
	case "private":
		return controlplanev1.Cluster_CONNECTION_TYPE_PRIVATE
	default:
		return controlplanev1.Cluster_CONNECTION_TYPE_UNSPECIFIED
	}
}

// ConnectionTypeToString returns the cloud cluster connection type string based
// on the controlplanev1's Cluster_ConnectionType code.
func ConnectionTypeToString(t controlplanev1.Cluster_ConnectionType) string {
	switch t {
	case controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC:
		return "public"
	case controlplanev1.Cluster_CONNECTION_TYPE_PRIVATE:
		return "private"
	default:
		return providerUnspecified
	}
}

// TypeListToStringSlice converts a types.List to a []string, stripping
// surrounding quotes for each element.
func TypeListToStringSlice(t types.List) []string {
	if t.IsNull() {
		return nil
	}
	s := []string{}
	for _, v := range t.Elements() {
		stringval, ok := v.(types.String)
		if ok {
			s = append(s, stringval.ValueString())
		} else {
			// TODO: issue #173 - ensure this is only called on types.List that actually hold strings
			s = append(s, strings.Trim(v.String(), "\"")) // it's easier to strip the quotes than type converting until you hit something that doesn't include them
		}
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
	return types.ListValueMust(types.StringType, values)
}

// TrimmedStringValue returns the string value of a types.String with the quotes removed.
// This is necessary as terraform has a tendency to slap these bad boys in at random which causes the API to fail
func TrimmedStringValue(s string) types.String {
	return basetypes.NewStringValue(strings.Trim(s, "\""))
}

// FindUserByName searches for a user by name using the provided client
func FindUserByName(ctx context.Context, name string, client dataplanev1grpc.UserServiceClient) (*dataplanev1.ListUsersResponse_User, error) {
	usrs, err := client.ListUsers(ctx, &dataplanev1.ListUsersRequest{
		Filter: &dataplanev1.ListUsersRequest_Filter{
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

// StringToUserMechanism converts a string to a dataplanev1.SASLMechanism
func StringToUserMechanism(s string) dataplanev1.SASLMechanism {
	switch strings.ToLower(s) {
	case "scram-sha-256":
		return dataplanev1.SASLMechanism_SASL_MECHANISM_SCRAM_SHA_256
	case "scram-sha-512":
		return dataplanev1.SASLMechanism_SASL_MECHANISM_SCRAM_SHA_512
	default:
		return dataplanev1.SASLMechanism_SASL_MECHANISM_UNSPECIFIED
	}
}

// UserMechanismToString converts a dataplanev1.SASLMechanism to a string
func UserMechanismToString(m *dataplanev1.SASLMechanism) string {
	if m == nil {
		return "unspecified"
	}
	switch *m {
	case dataplanev1.SASLMechanism_SASL_MECHANISM_SCRAM_SHA_256:
		return "scram-sha-256"
	case dataplanev1.SASLMechanism_SASL_MECHANISM_SCRAM_SHA_512:
		return "scram-sha-512"
	default:
		return "unspecified"
	}
}

// GetEffectivePassword returns the password to use from password_wo and password fields.
// Prefers password_wo over password when both are set. This centralizes the logic
// for choosing between write-only and legacy password fields.
func GetEffectivePassword(password, passwordWO types.String) string {
	if !passwordWO.IsNull() && !passwordWO.IsUnknown() {
		return passwordWO.ValueString()
	}
	return password.ValueString()
}

// TopicConfigurationToMap converts a slice of dataplanev1.Topic_Configuration to a slice of
// models.TopicConfiguration
func TopicConfigurationToMap(cfg []*dataplanev1.Topic_Configuration) (types.Map, error) {
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

// MapToCreateTopicConfiguration converts a cfg map to a slice of dataplanev1.CreateTopicRequest_Topic_Config
func MapToCreateTopicConfiguration(cfg types.Map) ([]*dataplanev1.CreateTopicRequest_Topic_Config, error) {
	var output []*dataplanev1.CreateTopicRequest_Topic_Config

	for k, v := range cfg.Elements() {
		if v.IsNull() || v.IsUnknown() {
			return nil, fmt.Errorf("topic configuration %q must have a value", k)
		}
		value := strings.Trim(v.String(), `"`)
		output = append(output, &dataplanev1.CreateTopicRequest_Topic_Config{
			Name:  k,
			Value: &value,
		})
	}
	return output, nil
}

// MapToSetTopicConfiguration converts a cfg map to a slice of
// dataplanev1.SetTopicConfigurationsRequest_SetConfiguration
func MapToSetTopicConfiguration(cfg types.Map) ([]*dataplanev1.SetTopicConfigurationsRequest_SetConfiguration, error) {
	var output []*dataplanev1.SetTopicConfigurationsRequest_SetConfiguration

	for k, v := range cfg.Elements() {
		if v.IsNull() || v.IsUnknown() {
			return nil, fmt.Errorf("topic configuration %q must have a value", k)
		}
		value := strings.Trim(v.String(), `"`)
		output = append(output, &dataplanev1.SetTopicConfigurationsRequest_SetConfiguration{
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
func FindTopicByName(ctx context.Context, topicName string, client dataplanev1grpc.TopicServiceClient) (*dataplanev1.ListTopicsResponse_Topic, error) {
	var pageToken string
	for {
		topics, err := client.ListTopics(ctx, &dataplanev1.ListTopicsRequest{
			Filter: &dataplanev1.ListTopicsRequest_Filter{
				NameContains: topicName,
			},
			PageToken: pageToken,
		})
		if err != nil {
			return nil, err
		}
		for _, v := range topics.GetTopics() {
			if v.GetName() == topicName {
				return v, nil
			}
		}
		pageToken = topics.GetNextPageToken()
		if pageToken == "" {
			break
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

// ConvertToConsoleURL converts a cluster API URL to a console URL for SecurityService operations.
// This transformation is needed because the SecurityService uses console URLs instead of cluster API URLs.
//
// Example:
//
//	https://api-123456.cluster-id.byoc.prd.cloud.redpanda.com
//	-> https://console-123456.cluster-id.byoc.prd.cloud.redpanda.com
func ConvertToConsoleURL(clusterAPIURL string) string {
	return strings.Replace(clusterAPIURL, "://api-", "://console-", 1)
}

// RetryGetCluster will retry a function, passing in the latest state of the given cluster id, until
// it either no longer returns an error or times out
func RetryGetCluster(ctx context.Context, timeout time.Duration, clusterID string, client cloud.CpClientSet, f func(*controlplanev1.Cluster) *RetryError) (*controlplanev1.Cluster, error) {
	var cluster *controlplanev1.Cluster
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

// DeserializeGrpcError err.Error() if it is a standard error. If it is a GRPC error, it returns status and codes
// for sure and details if they're set. Mild formatting to put details on a separate line for readability
func DeserializeGrpcError(err error) string {
	if err == nil {
		return ""
	}
	st, ok := grpcstatus.FromError(err)
	if !ok {
		return err.Error()
	}

	if len(st.Details()) > 0 {
		return fmt.Sprintf("%s : %s\n%s", st.Code(), st.Message(), st.Details())
	}
	return fmt.Sprintf("%s : %s", st.Code(), st.Message())
}

// StringMapToTypesMap converts a map[string]string to a types.Map
func StringMapToTypesMap(m map[string]string) (types.Map, error) {
	mv := make(map[string]attr.Value)
	for k, v := range m {
		mv[k] = types.StringValue(v)
	}
	mvo, diags := types.MapValue(types.StringType, mv)
	if diags.HasError() {
		return types.MapNull(types.StringType), errors.New("unable to convert map to types.Map")
	}
	return mvo, nil
}

// GetObjectFromAttributes is used to pull a Terraform Object out of a attribute map using the name
func GetObjectFromAttributes(ctx context.Context, key string, att map[string]attr.Value) (types.Object, error) {
	attVal, ok := att[key].(basetypes.ObjectValue)
	if !ok {
		return types.ObjectNull(map[string]attr.Type{}), fmt.Errorf("%s not found: object is missing or malformed for network resource", key)
	}
	var keyVal types.Object
	if err := attVal.As(ctx, &keyVal, basetypes.ObjectAsOptions{
		UnhandledNullAsEmpty:    true,
		UnhandledUnknownAsEmpty: true,
	}); err != nil {
		return types.ObjectNull(map[string]attr.Type{}), fmt.Errorf("%s not found: value is missing or malformed for network resource", key)
	}
	return keyVal, nil
}

// GetStringFromAttributes is used to pull a Terraform String out of a Terraform attribute map using the name
func GetStringFromAttributes(key string, attributes map[string]attr.Value) (string, error) {
	if val, ok := attributes[key].(types.String); ok {
		return val.ValueString(), nil
	}
	return "", fmt.Errorf("%s not found: string value is missing or malformed", key)
}

// GetARNListFromAttributes is used to pull a Terraform List out of a Terraform attribute map using the name
func GetARNListFromAttributes(key string, att map[string]attr.Value) ([]string, error) {
	attVal, ok := att[key].(basetypes.ObjectValue)
	if !ok {
		return nil, fmt.Errorf(fmt.Sprintf("%s not found", key), "object is missing or malformed for network resource")
	}
	rt, ok := attVal.Attributes()["arns"].(types.List)
	if !ok {
		return nil, fmt.Errorf(fmt.Sprintf("%s not found", key), "list is missing or malformed for network resource")
	}
	return TypeListToStringSlice(rt), nil
}

// IsNil checks if something is nil
func IsNil[T any](v T) bool {
	rv := reflect.ValueOf(v)

	switch rv.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Map,
		reflect.Slice, reflect.Func, reflect.Chan:
		return rv.IsNil()
	case reflect.Invalid:
		return true
	default:
		return false
	}
}

// ParseCPUToMillicores parses a Kubernetes CPU quantity string and returns millicores.
func ParseCPUToMillicores(value string) (int64, error) {
	qty, err := resource.ParseQuantity(value)
	if err != nil {
		return 0, err
	}
	return qty.MilliValue(), nil
}

// ParseMemoryToBytes parses a Kubernetes memory quantity string and returns bytes.
func ParseMemoryToBytes(value string) (int64, error) {
	qty, err := resource.ParseQuantity(value)
	if err != nil {
		return 0, err
	}
	return qty.Value(), nil
}
