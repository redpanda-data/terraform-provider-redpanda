// Copyright 2026 Redpanda Data, Inc.
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

package cluster

import (
	"context"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The connections conversions are fully generated (echo_unwrap); these tests
// pin the generated behavior the dual-listener feature depends on: the
// echo-accessor flatten with lowercase enum forms, the identity reorder to
// prev order, and the write expand dropping the computed endpoint. The
// generic reorder semantics are pinned in internal/modelconv.

const (
	typePublic  = "public"
	typePrivate = "private"
)

func connStatus(t controlplanev1.Cluster_ConnectionType, m controlplanev1.AuthMode, endpoint string) *controlplanev1.ConnectionStatus {
	return &controlplanev1.ConnectionStatus{
		Config:   &controlplanev1.ConnectionSpec{Type: t, Auth: &controlplanev1.AuthSpec{Mode: m}},
		Endpoint: endpoint,
	}
}

func connElem(t *testing.T, connType, mode, endpoint string) types.Object {
	t.Helper()
	var ep attr.Value = types.StringValue(endpoint)
	if endpoint == "" {
		ep = types.StringNull()
	}
	return types.ObjectValueMust(KafkaAPIConnectionsAttrTypes(), map[string]attr.Value{
		"type": types.StringValue(connType),
		"auth": types.ObjectValueMust(KafkaAPIConnectionsAuthAttrTypes(), map[string]attr.Value{
			"mode": types.StringValue(mode),
		}),
		"endpoint": ep,
	})
}

func connList(t *testing.T, elems ...types.Object) types.List {
	t.Helper()
	vals := make([]attr.Value, len(elems))
	for i, e := range elems {
		vals[i] = e
	}
	return types.ListValueMust(types.ObjectType{AttrTypes: KafkaAPIConnectionsAttrTypes()}, vals)
}

func elemParts(t *testing.T, list types.List, i int) (connType, mode, endpoint string) {
	t.Helper()
	obj, ok := list.Elements()[i].(types.Object)
	if !ok {
		t.Fatalf("element %d is not an object", i)
	}
	attrs := obj.Attributes()
	auth, ok := attrs["auth"].(types.Object)
	if !ok {
		t.Fatalf("element %d auth is not an object", i)
	}
	return attrs["type"].(types.String).ValueString(),
		auth.Attributes()["mode"].(types.String).ValueString(),
		attrs["endpoint"].(types.String).ValueString()
}

func flattenConnections(t *testing.T, conns []*controlplanev1.ConnectionStatus, prev types.List) types.List {
	t.Helper()
	proto := &controlplanev1.Cluster_KafkaAPI{Connections: conns}
	var prevModel *KafkaAPIModel
	if !prev.IsNull() {
		prevModel = &KafkaAPIModel{Connections: prev}
	}
	m, diags := FlattenKafkaAPI(context.Background(), proto, prevModel)
	if diags.HasError() {
		t.Fatalf("FlattenKafkaAPI: %v", diags)
	}
	return m.Connections
}

func TestConnectionsFlatten_EmptyIsNull(t *testing.T) {
	got := flattenConnections(t, nil, types.ListNull(types.ObjectType{AttrTypes: KafkaAPIConnectionsAttrTypes()}))
	if !got.IsNull() {
		t.Fatalf("expected null list for no connections, got %v", got)
	}
}

func TestConnectionsFlatten_ServerOrderWithoutPrev(t *testing.T) {
	got := flattenConnections(t, []*controlplanev1.ConnectionStatus{
		connStatus(controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC, controlplanev1.AuthMode_AUTH_MODE_SASL, "pub-ep"),
		connStatus(controlplanev1.Cluster_CONNECTION_TYPE_PRIVATE, controlplanev1.AuthMode_AUTH_MODE_MTLS, "prv-ep"),
	}, types.ListNull(types.ObjectType{AttrTypes: KafkaAPIConnectionsAttrTypes()}))

	if n := len(got.Elements()); n != 2 {
		t.Fatalf("expected 2 elements, got %d", n)
	}
	ct, mode, ep := elemParts(t, got, 0)
	if ct != typePublic || mode != "sasl" || ep != "pub-ep" {
		t.Fatalf("element 0 = (%s, %s, %s), want (public, sasl, pub-ep)", ct, mode, ep)
	}
	ct, mode, ep = elemParts(t, got, 1)
	if ct != typePrivate || mode != "mtls" || ep != "prv-ep" {
		t.Fatalf("element 1 = (%s, %s, %s), want (private, mtls, prv-ep)", ct, mode, ep)
	}
}

func TestConnectionsFlatten_ReordersToPrev(t *testing.T) {
	server := []*controlplanev1.ConnectionStatus{
		connStatus(controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC, controlplanev1.AuthMode_AUTH_MODE_SASL, "pub-ep"),
		connStatus(controlplanev1.Cluster_CONNECTION_TYPE_PRIVATE, controlplanev1.AuthMode_AUTH_MODE_SASL, "prv-ep"),
	}
	prev := connList(t,
		connElem(t, typePrivate, "sasl", ""),
		connElem(t, typePublic, "sasl", ""),
	)
	got := flattenConnections(t, server, prev)

	ct, _, ep := elemParts(t, got, 0)
	if ct != typePrivate || ep != "prv-ep" {
		t.Fatalf("element 0 = (%s, %s), want prev-ordered (private, prv-ep)", ct, ep)
	}
	ct, _, ep = elemParts(t, got, 1)
	if ct != typePublic || ep != "pub-ep" {
		t.Fatalf("element 1 = (%s, %s), want prev-ordered (public, pub-ep)", ct, ep)
	}
}

func TestConnectionsFlatten_UnmatchedServerEntriesAppend(t *testing.T) {
	server := []*controlplanev1.ConnectionStatus{
		connStatus(controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC, controlplanev1.AuthMode_AUTH_MODE_MTLS, "pub-mtls-ep"),
		connStatus(controlplanev1.Cluster_CONNECTION_TYPE_PRIVATE, controlplanev1.AuthMode_AUTH_MODE_SASL, "prv-ep"),
	}
	// prev knows only the private entry; the public mTLS entry is new and
	// must append after it in server order.
	prev := connList(t, connElem(t, typePrivate, "sasl", "prv-ep"))
	got := flattenConnections(t, server, prev)

	if n := len(got.Elements()); n != 2 {
		t.Fatalf("expected 2 elements, got %d", n)
	}
	ct, _, _ := elemParts(t, got, 0)
	if ct != typePrivate {
		t.Fatalf("element 0 type = %s, want prev-matched private first", ct)
	}
	ct, mode, _ := elemParts(t, got, 1)
	if ct != typePublic || mode != "mtls" {
		t.Fatalf("element 1 = (%s, %s), want appended (public, mtls)", ct, mode)
	}
}

func TestConnectionsExpandCreate(t *testing.T) {
	m := &KafkaAPIModel{Connections: connList(t,
		connElem(t, typePublic, "sasl", "ignored-endpoint"),
		connElem(t, typePrivate, "mtls", ""),
	)}
	out, diags := ExpandCreateKafkaAPI(context.Background(), m)
	if diags.HasError() {
		t.Fatalf("ExpandCreateKafkaAPI: %v", diags)
	}
	got := out.GetConnections()
	if len(got) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(got))
	}
	if got[0].GetType() != controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC || got[0].GetAuth().GetMode() != controlplanev1.AuthMode_AUTH_MODE_SASL {
		t.Fatalf("spec 0 = %v, want public/sasl", got[0])
	}
	if got[1].GetType() != controlplanev1.Cluster_CONNECTION_TYPE_PRIVATE || got[1].GetAuth().GetMode() != controlplanev1.AuthMode_AUTH_MODE_MTLS {
		t.Fatalf("spec 1 = %v, want private/mtls", got[1])
	}
}
