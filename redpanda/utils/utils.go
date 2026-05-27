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

// IsAlreadyExists matches gRPC AlreadyExists; used by Create paths to recover from a prior retry's lost response.
func IsAlreadyExists(err error) bool {
	if err == nil {
		return false
	}

	if e, ok := grpcstatus.FromError(err); ok && e.Code() == grpccodes.AlreadyExists {
		return true
	}

	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "already exists") ||
		strings.Contains(errStr, "alreadyexists")
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

// IsUnavailable checks if the error indicates a service unavailable or transient error
// that should be retried at the application level. This includes gRPC Unavailable errors
// and HTTP 503 responses that may come from load balancers or gateways.
func IsUnavailable(err error) bool {
	if err == nil {
		return false
	}

	if e, ok := grpcstatus.FromError(err); ok && e.Code() == grpccodes.Unavailable {
		return true
	}

	errStr := err.Error()
	return strings.Contains(errStr, "503") ||
		strings.Contains(strings.ToLower(errStr), "service unavailable") ||
		strings.Contains(strings.ToLower(errStr), "unavailable")
}

// IsTransientServerError returns true for gRPC error codes that we consider
// safe to retry on the read side (GetOperation polling). Covers Unavailable
// (load balancer / restart) plus Internal (observed three times during
// serverless tag-mutation polling — the underlying mutation succeeded each
// time; only the GetOperation read transiently glitched). Callers must cap
// the retry count to avoid masking real bugs surfaced as Internal.
func IsTransientServerError(err error) bool {
	if err == nil {
		return false
	}
	if IsUnavailable(err) {
		return true
	}
	if e, ok := grpcstatus.FromError(err); ok && e.Code() == grpccodes.Internal {
		return true
	}
	return false
}

// isGRPCInternal reports whether err's underlying gRPC status code is Internal.
// Used to differentiate the log level on transient-error retry (Warn for
// Internal so it shows up in CI, Info for the routine Unavailable case).
func isGRPCInternal(err error) bool {
	if err == nil {
		return false
	}
	e, ok := grpcstatus.FromError(err)
	return ok && e.Code() == grpccodes.Internal
}

// IsRetryableByocError checks if an error from rpk byoc should be retried.
// Some byoc errors are transient and explicitly ask to retry.
func IsRetryableByocError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Check for explicit retry instructions
	if strings.Contains(errStr, "please retry later") {
		return true
	}

	return false
}

// AreWeDoneYet checks an operation's state until one of completion, failure
// or timeout is reached. Transient errors (Unavailable/Internal) on
// GetOperation are tolerated within a stuck-cap window derived from the
// caller's `timeout` — the cap fires when there has been no successful
// poll for stuckCap = min(5min, timeout/6) of wall-clock time. The window
// resets on any successful poll, so an op with intermittent transients
// keeps retrying within the overall timeout; only a sustained burst with
// no progress trips it. Scales correctly across resource timeouts:
// long-running cluster Create (90m) gets the 5-minute ceiling; a 1-minute
// test op bails after ~10s of solid transients.
func AreWeDoneYet(ctx context.Context, op *controlplanev1.Operation, timeout time.Duration, client controlplanev1grpc.OperationServiceClient) error {
	stuckCap := timeout / 6
	if stuckCap > 5*time.Minute {
		stuckCap = 5 * time.Minute
	}
	lastSuccessfulPoll := time.Now()

	return Retry(ctx, timeout, func() *RetryError {
		tflog.Info(ctx, "getting operation")
		latestOp, err := client.GetOperation(ctx, &controlplanev1.GetOperationRequest{
			Id: op.GetId(),
		})
		tflog.Info(ctx, "got result of operation")
		if err != nil {
			if IsTransientServerError(err) {
				stuckFor := time.Since(lastSuccessfulPoll)
				if stuckFor > stuckCap {
					tflog.Warn(ctx, fmt.Sprintf("server unresponsive for %s (stuck cap %s) on operation %q", stuckFor, stuckCap, op.GetId()))
					return NonRetryableError(fmt.Errorf("server unresponsive for %s (stuck cap %s): %w", stuckFor, stuckCap, err))
				}
				// Internal can mask real bugs — surface at Warn so a recurring
				// pattern is visible in CI; Unavailable is routine, stays Info.
				logf := tflog.Info
				if isGRPCInternal(err) {
					logf = tflog.Warn
				}
				logf(ctx, fmt.Sprintf("transient error for operation %q (stuck %s / cap %s): %v", op.GetId(), stuckFor, stuckCap, err))
				return RetryableError(err)
			}
			return NonRetryableError(err)
		}
		lastSuccessfulPoll = time.Now() // reset stuck window on any successful poll
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

// StringValueOrNull returns StringNull for "" and StringValue otherwise.
func StringValueOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

// StringSliceToTypeListOrNull is StringSliceToTypeList but also maps empty slices to ListNull.
func StringSliceToTypeListOrNull(elements []string) types.List {
	if len(elements) == 0 {
		return types.ListNull(types.StringType)
	}
	return StringSliceToTypeList(elements)
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

// TypesNullable is the subset of attr.Value used by PointerOrNil — every
// types.String / types.Bool / types.Int32 / types.Int64 / types.Float64
// satisfies it.
type TypesNullable interface {
	IsNull() bool
	IsUnknown() bool
}

// PointerOrNil returns nil when v is null or unknown; otherwise returns a
// pointer to extract(v). Used by the schemagen Expand emission for
// proto3-optional / wrapper / oneof scalar fields so "unset" round-trips as
// proto's nil rather than the type's zero value. Callers pass an unbound
// method expression such as types.String.ValueString or types.Bool.ValueBool.
func PointerOrNil[V TypesNullable, T any](v V, extract func(V) T) *T {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	out := extract(v)
	return &out
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

// NumberToInt32OrNil converts a types.Number to *int32, returning nil for
// null or unknown values. Used by generated Expand for NumberAttribute
// fields backed by proto3-optional int32 fields, where nil tells the
// server to use its default.
func NumberToInt32OrNil(n types.Number) *int32 {
	if n.IsNull() || n.IsUnknown() {
		return nil
	}
	return NumberToInt32(n)
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

// RetryGetCluster retries f against the latest cluster snapshot until f
// succeeds, ctx expires, or maxTransientRetries Unavailable errors have been
// observed. Uses a count cap on transient errors rather than the stuck-cap
// timing AreWeDoneYet uses, because cluster lifecycle calls are interactive
// and should fail fast on persistent server faults.
func RetryGetCluster(ctx context.Context, timeout time.Duration, clusterID string, client cloud.CpClientSet, f func(*controlplanev1.Cluster) *RetryError) (*controlplanev1.Cluster, error) {
	var cluster *controlplanev1.Cluster
	const maxTransientRetries = 10
	transientRetryCount := 0

	err := Retry(ctx, timeout, func() *RetryError {
		var err error
		cluster, err = client.ClusterForID(ctx, clusterID)
		if err != nil {
			if IsNotFound(err) {
				tflog.Info(ctx, fmt.Sprintf("cluster %q not found", clusterID))
				cluster = nil
				return nil
			}
			if IsPermissionDenied(err) {
				tflog.Warn(ctx, fmt.Sprintf("cluster %q returned PermissionDenied — cluster may have been deleted", clusterID))
				cluster = nil
				return nil
			}
			if IsUnavailable(err) {
				transientRetryCount++
				if transientRetryCount >= maxTransientRetries {
					tflog.Warn(ctx, fmt.Sprintf("max transient retries (%d) exceeded for cluster %q", maxTransientRetries, clusterID))
					return NonRetryableError(fmt.Errorf("max transient retries exceeded: %w", err))
				}
				tflog.Info(ctx, fmt.Sprintf("transient error for cluster %q (attempt %d/%d): %v", clusterID, transientRetryCount, maxTransientRetries, err))
				return RetryableError(err)
			}
			return NonRetryableError(err)
		}
		transientRetryCount = 0 // Reset on success
		tflog.Info(ctx, fmt.Sprintf("cluster %v : %v", clusterID, cluster.GetState()))
		return f(cluster)
	})
	return cluster, err
}

// DeserializeGrpcError returns a formatted error string with gRPC status code, message, and details.
// Falls back to raw error string when the gRPC message is empty.
func DeserializeGrpcError(err error) string {
	if err == nil {
		return ""
	}
	st, ok := grpcstatus.FromError(err)
	if !ok {
		return err.Error()
	}

	code := st.Code().String()
	msg := st.Message()
	rawErr := err.Error()

	var result string
	switch {
	case msg != "":
		result = fmt.Sprintf("%s : %s", code, msg)
	case rawErr != "":
		result = fmt.Sprintf("%s (raw: %s)", code, rawErr)
	default:
		result = code
	}

	if len(st.Details()) > 0 {
		result = fmt.Sprintf("%s\n%v", result, st.Details())
	}

	return result
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
