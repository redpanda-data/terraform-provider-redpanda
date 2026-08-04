// Copyright 2025 Redpanda Data, Inc.
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

package utils

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// DataplaneCallError annotates a dataplane RPC failure with the method and
// cluster endpoint that produced it. A bare gRPC status names neither, which
// left operators with "code = Unknown desc = " and no way to tell which call
// against which cluster failed.
//
// It unwraps to the original error, so grpcstatus.FromError and every code-based
// classifier keep working through it.
type DataplaneCallError struct {
	Method   string
	Endpoint string
	err      error
}

// Error renders the annotation ahead of the underlying gRPC error.
func (e *DataplaneCallError) Error() string {
	return fmt.Sprintf("%s: %v", e.annotation(), e.err)
}

// Unwrap exposes the underlying error to errors.As/Is and grpcstatus.FromError.
func (e *DataplaneCallError) Unwrap() error { return e.err }

func (e *DataplaneCallError) annotation() string {
	return fmt.Sprintf("dataplane call %s on %s", e.Method, e.Endpoint)
}

// dataplaneAnnotation returns the method/endpoint prefix for err, or "" when err
// carries no dataplane annotation.
func dataplaneAnnotation(err error) string {
	var dce *DataplaneCallError
	if errors.As(err, &dce) {
		return dce.annotation()
	}
	return ""
}

// unannotated strips a provider annotation, returning the error as the server
// produced it. Only the outermost annotation is removed; nothing else is unwrapped.
func unannotated(err error) error {
	var dce *DataplaneCallError
	if errors.As(err, &dce) {
		return dce.err
	}
	return err
}

// serverMessage returns the text the substring heuristics in the Is* classifiers
// are allowed to inspect: the error as the server produced it, with any
// provider-added annotation stripped.
//
// Those heuristics match bare tokens such as "404" and "unavailable" anywhere in
// the string. Endpoints and method names are provider prose, and cluster IDs are
// alphanumeric — an endpoint containing "404" must not make IsNotFound fire.
func serverMessage(err error) string {
	return unannotated(err).Error()
}

// serverStatus returns the gRPC status as the server produced it.
//
// The annotation must be stripped first. grpcstatus.FromError carries the code
// through a wrapper, but when it locates the status by unwrapping it overwrites
// the message with the wrapper's full text (status.go: p.Message = err.Error()).
// Read straight off an annotated error, a bare UNKNOWN therefore looks like it
// carries a message — which is the difference between retrying and not.
func serverStatus(err error) (*grpcstatus.Status, bool) {
	return grpcstatus.FromError(unannotated(err))
}

// IsTransientDataplaneError reports whether a dataplane RPC failure is a
// warm-up or topology-churn signal rather than a verdict on the request.
//
// A cluster reports Ready before its dataplane serves traffic. Calls made in
// that window come back as Unavailable, as broker-churn errors, or as a bare
// UNKNOWN — code 2 with an empty message, which asserts nothing about the
// request and is the signature observed when the endpoint is reachable but not
// yet serving.
//
// An UNKNOWN that carries a message is left alone: that is a server-side
// handler reporting a real failure, and retrying it only delays the report.
func IsTransientDataplaneError(err error) bool {
	if err == nil {
		return false
	}
	return IsUnavailable(err) || isBareUnknown(err) || isBrokerChurn(err)
}

// isBareUnknown matches gRPC code Unknown with no message.
func isBareUnknown(err error) bool {
	st, ok := serverStatus(err)
	return ok && st.Code() == grpccodes.Unknown && strings.TrimSpace(st.Message()) == ""
}

// isBrokerChurn matches broker-side failures raised while the Kafka API's view
// of the cluster is still settling.
func isBrokerChurn(err error) bool {
	msg := serverMessage(err)
	return strings.Contains(msg, "broker struct chosen to issue this request has died") ||
		strings.Contains(msg, "client closed") ||
		strings.Contains(msg, "context canceled")
}

// annotatingConn decorates a pooled dataplane connection so every RPC failure
// carries its method and endpoint.
type annotatingConn struct {
	grpc.ClientConnInterface
	endpoint string
}

func (c annotatingConn) Invoke(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
	if err := c.ClientConnInterface.Invoke(ctx, method, args, reply, opts...); err != nil {
		return &DataplaneCallError{Method: method, Endpoint: c.endpoint, err: err}
	}
	return nil
}

func (c annotatingConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	stream, err := c.ClientConnInterface.NewStream(ctx, desc, method, opts...)
	if err != nil {
		return nil, &DataplaneCallError{Method: method, Endpoint: c.endpoint, err: err}
	}
	return stream, nil
}
