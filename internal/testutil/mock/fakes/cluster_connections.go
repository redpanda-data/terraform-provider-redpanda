// Copyright 2026 Redpanda Data, Inc.
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

package fakes

import (
	"fmt"
	"slices"
	"strings"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Dual listener mode ("connections") fidelity layer. Mirrors cloudv2
// apps/public-api-go/internal/services/cluster/v1/dual_mode_connections.go:
// the create-path guard order (feature flag → Azure → cross-service
// semantics → mTLS coupling), the always-on read projection (every cluster
// reports connections[], legacy included), and the mask-aware update path
// (unmasked body connections cleared, empty-leaf clear rejected on dual,
// legacy sasl/mtls mutations rejected on dual, rename-in-place endpoint
// preservation, stored-CA fallback).

const (
	connTypePublic  = controlplanev1.Cluster_CONNECTION_TYPE_PUBLIC
	connTypePrivate = controlplanev1.Cluster_CONNECTION_TYPE_PRIVATE

	svcKafkaAPI       = "kafka_api"
	svcHTTPProxy      = "http_proxy"
	svcSchemaRegistry = "schema_registry"
)

func connInvalidf(format string, args ...any) error {
	return status.Errorf(codes.InvalidArgument, format, args...)
}

// checkConnectionsFlag mirrors checkPublicPrivateListenersFeatureFlagInUse:
// PermissionDenied when connections are in use but the org lacks the
// enable-public-private-listeners flag. Checked before every other guard so an
// org without the feature is not told the feature exists.
func (f *ClusterFake) checkConnectionsFlag(inUse bool) error {
	if inUse && f.PublicPrivateListenersDisabled {
		return status.Error(codes.PermissionDenied,
			"dual listener mode (connections) is not enabled for this organization")
	}
	return nil
}

func connectionsEnvelopeErr() error {
	return connInvalidf("dual listener mode (connections) is supported only on AWS BYOC clusters")
}

// hasPublicConnection reports whether any spec is a public listener.
func hasPublicConnection(conns []*controlplanev1.ConnectionSpec) bool {
	for _, c := range conns {
		if c.GetType() == connTypePublic {
			return true
		}
	}
	return false
}

// storedPrivateOnly reports whether the stored cluster has listeners and none
// of them is public, across all three services.
func storedPrivateOnly(cl *controlplanev1.Cluster) bool {
	hasAny := false
	for _, stored := range [][]*controlplanev1.ConnectionStatus{
		cl.GetKafkaApi().GetConnections(),
		cl.GetHttpProxy().GetConnections(),
		cl.GetSchemaRegistry().GetConnections(),
	} {
		for _, e := range stored {
			hasAny = true
			if e.GetConfig().GetType() == connTypePublic {
				return false
			}
		}
	}
	return hasAny
}

// validateConnectionSpecs mirrors the shared per-service spec check:
// UNSPECIFIED enums and duplicate (type, auth) pairs are rejected.
func validateConnectionSpecs(service string, conns []*controlplanev1.ConnectionSpec) error {
	seen := map[string]bool{}
	for _, c := range conns {
		if c.GetType() == controlplanev1.Cluster_CONNECTION_TYPE_UNSPECIFIED {
			return connInvalidf("%s has a connection with an unspecified type; set it to public or private", service)
		}
		if c.GetAuth().GetMode() == controlplanev1.AuthMode_AUTH_MODE_UNSPECIFIED {
			return connInvalidf("%s has a connection with an unspecified auth mode; set it to sasl or mtls", service)
		}
		key := c.GetType().String() + "/" + c.GetAuth().GetMode().String()
		if seen[key] {
			return connInvalidf("%s has duplicate connections; each network type and auth mode pair may appear at most once", service)
		}
		seen[key] = true
	}
	return nil
}

func hasSASLConnection(conns []*controlplanev1.ConnectionSpec) bool {
	for _, c := range conns {
		if c.GetAuth().GetMode() == controlplanev1.AuthMode_AUTH_MODE_SASL {
			return true
		}
	}
	return false
}

func hasMTLSConnection(conns []*controlplanev1.ConnectionSpec) bool {
	for _, c := range conns {
		if c.GetAuth().GetMode() == controlplanev1.AuthMode_AUTH_MODE_MTLS {
			return true
		}
	}
	return false
}

func connectionPresence(conns []*controlplanev1.ConnectionSpec) (hasPublic, hasPrivate bool) {
	for _, c := range conns {
		switch c.GetType() {
		case connTypePublic:
			hasPublic = true
		case connTypePrivate:
			hasPrivate = true
		default:
		}
	}
	return hasPublic, hasPrivate
}

func describeConnectionTopology(hasPublic, hasPrivate bool) string {
	switch {
	case hasPublic && hasPrivate:
		return "public+private"
	case hasPrivate:
		return "private-only"
	default:
		return "public-only"
	}
}

// createConnService is one service's create-request view.
type createConnService struct {
	name  string
	conns []*controlplanev1.ConnectionSpec
	sasl  *controlplanev1.SASLSpec
	mtls  *controlplanev1.MTLSSpec
}

// validateCreateConnections mirrors the create path: feature flag → Azure →
// validateConnections cross-service semantics → validateConnectionsMTLS.
func (f *ClusterFake) validateCreateConnections(in *controlplanev1.ClusterCreate) error {
	services := []createConnService{
		{svcKafkaAPI, in.GetKafkaApi().GetConnections(), in.GetKafkaApi().GetSasl(), in.GetKafkaApi().GetMtls()},
		{svcHTTPProxy, in.GetHttpProxy().GetConnections(), in.GetHttpProxy().GetSasl(), in.GetHttpProxy().GetMtls()},
		{svcSchemaRegistry, in.GetSchemaRegistry().GetConnections(), in.GetSchemaRegistry().GetSasl(), in.GetSchemaRegistry().GetMtls()},
	}

	inUse := false
	for _, s := range services {
		if len(s.conns) > 0 {
			inUse = true
		}
	}
	if err := f.checkConnectionsFlag(inUse); err != nil {
		return err
	}
	if !inUse {
		return nil
	}
	if in.GetCloudProvider() != controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS || in.GetType() != controlplanev1.Cluster_TYPE_BYOC {
		return connectionsEnvelopeErr()
	}

	for _, s := range services {
		if err := validateConnectionSpecs(s.name, s.conns); err != nil {
			return err
		}
	}
	if in.GetConnectionType() != controlplanev1.Cluster_CONNECTION_TYPE_UNSPECIFIED {
		return connInvalidf("connection_type cannot be set together with connections; connections define the cluster's network topology")
	}
	for _, s := range services {
		if len(s.conns) > 0 && s.sasl != nil {
			return connInvalidf("%s.sasl cannot be set together with %s.connections; connections define per-listener auth", s.name, s.name)
		}
	}
	var missing []string
	for _, s := range services {
		if len(s.conns) == 0 {
			missing = append(missing, s.name)
		}
	}
	if len(missing) > 0 {
		return connInvalidf("when connections are set they must be set on all services; missing on %s", strings.Join(missing, ", "))
	}
	refPub, refPriv := connectionPresence(services[0].conns)
	for _, s := range services[1:] {
		hasPub, hasPriv := connectionPresence(s.conns)
		if hasPub != refPub || hasPriv != refPriv {
			return connInvalidf("all services must have the same connection network types; %s is %s but %s is %s",
				services[0].name, describeConnectionTopology(refPub, refPriv), s.name, describeConnectionTopology(hasPub, hasPriv))
		}
	}
	for _, s := range services {
		if err := validateServiceConnectionsMTLS(s.name, s.conns, s.mtls); err != nil {
			return err
		}
	}
	return nil
}

// validateServiceConnectionsMTLS mirrors the create-path mTLS coupling: an
// mTLS connection requires a service mtls block with a CA and enabled=true; a
// meaningful mtls block with no mTLS connection has no listener to apply to.
func validateServiceConnectionsMTLS(service string, conns []*controlplanev1.ConnectionSpec, mtls *controlplanev1.MTLSSpec) error {
	hasMTLSConn := hasMTLSConnection(conns)
	if len(conns) > 0 && !hasMTLSConn && mtls != nil && (mtls.GetEnabled() || len(mtls.GetCaCertificatesPem()) > 0) {
		return connInvalidf("%s.mtls cannot be set when no connection uses mTLS auth; add an mTLS connection or remove the mtls block", service)
	}
	if !hasMTLSConn {
		return nil
	}
	if len(mtls.GetCaCertificatesPem()) == 0 {
		return connInvalidf("%s declares an mTLS connection but %s.mtls.ca_certificates_pem is empty; provide the trusted client CA bundle", service, service)
	}
	if !mtls.GetEnabled() {
		return connInvalidf("%s declares an mTLS connection but %s.mtls.enabled is false; set it to true instead of relying on an implicit override", service, service)
	}
	return nil
}

// connectionEndpoint synthesizes the deterministic per-listener endpoint a
// dual cluster reports, patterned on the -pub/-prv listener DNS names.
func connectionEndpoint(service string, c *controlplanev1.ConnectionSpec) string {
	nt := "pub"
	if c.GetType() == connTypePrivate {
		nt = "prv"
	}
	auth := "sasl"
	if c.GetAuth().GetMode() == controlplanev1.AuthMode_AUTH_MODE_MTLS {
		auth = "mtls"
	}
	switch service {
	case svcKafkaAPI:
		return fmt.Sprintf("mock-broker-0-%s-%s.mock.redpanda.cloud:9092", nt, auth)
	case svcHTTPProxy:
		return fmt.Sprintf("https://mock-%s-%s.http-proxy.redpanda.cloud", nt, auth)
	default:
		return fmt.Sprintf("https://mock-%s-%s.schema-registry.redpanda.cloud", nt, auth)
	}
}

func cloneConnectionSpec(c *controlplanev1.ConnectionSpec) *controlplanev1.ConnectionSpec {
	return &controlplanev1.ConnectionSpec{
		Type: c.GetType(),
		Auth: &controlplanev1.AuthSpec{Mode: c.GetAuth().GetMode()},
	}
}

// connectionStatusesFromSpec builds the read-shape entries for a dual create,
// in request order (the control plane appends listeners in connection order on
// create).
func connectionStatusesFromSpec(service string, conns []*controlplanev1.ConnectionSpec) []*controlplanev1.ConnectionStatus {
	out := make([]*controlplanev1.ConnectionStatus, 0, len(conns))
	for _, c := range conns {
		out = append(out, &controlplanev1.ConnectionStatus{
			Config:   cloneConnectionSpec(c),
			Endpoint: connectionEndpoint(service, c),
		})
	}
	return out
}

// legacyConnectionProjection mirrors the always-on read projection for a
// legacy cluster: one entry per implied listener (SASL always; mTLS when the
// service mtls block is enabled), typed by the cluster's connection_type. The
// service fallback endpoint (the deprecated url / first seed broker) is
// borrowed only in the single-listener case, mirroring
// connectionStatusesForListeners' legacy fallback gate.
func legacyConnectionProjection(connType controlplanev1.Cluster_ConnectionType, mtlsEnabled bool, fallbackEndpoint string) []*controlplanev1.ConnectionStatus {
	t := connTypePublic
	if connType == connTypePrivate {
		t = connTypePrivate
	}
	entries := []*controlplanev1.ConnectionStatus{{
		Config: &controlplanev1.ConnectionSpec{
			Type: t,
			Auth: &controlplanev1.AuthSpec{Mode: controlplanev1.AuthMode_AUTH_MODE_SASL},
		},
	}}
	if mtlsEnabled {
		entries = append(entries, &controlplanev1.ConnectionStatus{
			Config: &controlplanev1.ConnectionSpec{
				Type: t,
				Auth: &controlplanev1.AuthSpec{Mode: controlplanev1.AuthMode_AUTH_MODE_MTLS},
			},
		})
	}
	if len(entries) == 1 {
		entries[0].Endpoint = fallbackEndpoint
	}
	return entries
}

// reconcileConnections mirrors apiConnectionsToListenersForUpdateCluster's
// order semantics: retained and renamed-in-place listeners keep their stored
// position (and their endpoint — renaming preserves DNS), genuinely new
// listeners append in request order, undesired listeners are removed. The
// resulting order deliberately DIVERGES from request order after a
// rename/removal cycle, exactly like the control plane.
func reconcileConnections(service string, stored []*controlplanev1.ConnectionStatus, desired []*controlplanev1.ConnectionSpec) []*controlplanev1.ConnectionStatus {
	specKey := func(c *controlplanev1.ConnectionSpec) string {
		return c.GetType().String() + "/" + c.GetAuth().GetMode().String()
	}
	desiredKeys := make([]string, len(desired))
	for i, d := range desired {
		desiredKeys[i] = specKey(d)
	}

	consumedDesired := map[int]bool{}
	out := make([]*controlplanev1.ConnectionStatus, 0, len(desired))
	for _, e := range stored {
		key := specKey(e.GetConfig())
		if i := slices.Index(desiredKeys, key); i >= 0 && !consumedDesired[i] {
			// Exact match: kept in place, endpoint preserved.
			consumedDesired[i] = true
			out = append(out, &controlplanev1.ConnectionStatus{
				Config:   cloneConnectionSpec(desired[i]),
				Endpoint: e.GetEndpoint(),
			})
			continue
		}
		// Auth switch: a same-network-type other-auth stored listener that is
		// not itself desired is renamed in place — endpoint preserved.
		if !slices.Contains(desiredKeys, key) {
			switched := -1
			for i, d := range desired {
				if !consumedDesired[i] && d.GetType() == e.GetConfig().GetType() &&
					d.GetAuth().GetMode() != e.GetConfig().GetAuth().GetMode() {
					switched = i
					break
				}
			}
			if switched >= 0 {
				consumedDesired[switched] = true
				out = append(out, &controlplanev1.ConnectionStatus{
					Config:   cloneConnectionSpec(desired[switched]),
					Endpoint: e.GetEndpoint(),
				})
				continue
			}
		}
		// Undesired: removed.
	}
	for i, d := range desired {
		if !consumedDesired[i] {
			out = append(out, &controlplanev1.ConnectionStatus{
				Config:   cloneConnectionSpec(d),
				Endpoint: connectionEndpoint(service, d),
			})
		}
	}
	return out
}

// connectionsClusterType mirrors getConnectionType: any private kafka listener
// makes the cluster read back "private".
func connectionsClusterType(kafkaConns []*controlplanev1.ConnectionStatus, fallback controlplanev1.Cluster_ConnectionType) controlplanev1.Cluster_ConnectionType {
	for _, e := range kafkaConns {
		if e.GetConfig().GetType() == connTypePrivate {
			return connTypePrivate
		}
	}
	if len(kafkaConns) > 0 {
		return connTypePublic
	}
	return fallback
}

// --- update path ---

// updateConnMasks reports the mask state per service, mirroring
// serviceConnectionsInMask / serviceConnectionsLeafInMask /
// maskHasLegacyListenerPath / serviceSASLInMask / serviceMTLSCAInMask /
// serviceMTLSEnabledInMask.
func maskHasService(paths []string, service string) bool {
	return slices.Contains(paths, service) || slices.Contains(paths, service+".connections")
}

func maskHasConnectionsLeaf(paths []string, service string) bool {
	return slices.Contains(paths, service+".connections")
}

func maskHasLegacyListenerPath(paths []string, service string) bool {
	for _, p := range paths {
		if p == service || strings.HasPrefix(p, service+".sasl") || strings.HasPrefix(p, service+".mtls") {
			return true
		}
	}
	return false
}

func maskHasSASL(paths []string, service string) bool {
	for _, p := range paths {
		switch p {
		case service, service + ".sasl", service + ".sasl.enabled":
			return true
		default:
		}
	}
	return false
}

func maskHasMTLSEnabled(paths []string, service string) bool {
	for _, p := range paths {
		switch p {
		case service, service + ".mtls", service + ".mtls.enabled":
			return true
		default:
		}
	}
	return false
}

func maskHasMTLSCA(paths []string, service string) bool {
	for _, p := range paths {
		switch p {
		case service, service + ".mtls", service + ".mtls.ca_certificates_pem":
			return true
		default:
		}
	}
	return false
}

func maskHasMTLSRules(paths []string, service string) bool {
	for _, p := range paths {
		switch p {
		case service, service + ".mtls", service + ".mtls.principal_mapping_rules":
			return true
		default:
		}
	}
	return false
}

// updateConnSvc is one service's update-request view plus its stored state.
type updateConnSvc struct {
	name       string
	conns      []*controlplanev1.ConnectionSpec
	sasl       *controlplanev1.SASLSpec
	mtls       *controlplanev1.MTLSSpec
	masked     bool
	leafMasked bool
	stored     []*controlplanev1.ConnectionStatus
	storedMTLS *controlplanev1.MTLSSpec
}

// applyConnectionsUpdate runs the connections update guards and, when the
// request uses connections, applies them to the stored cluster. It returns the
// set of services it owned; the caller's mask loop must skip those services'
// paths. Mirrors validateConnectionsUpdate + resolveConnectionsMTLSForUpdate +
// the reconcile mapper.
func (f *ClusterFake) applyConnectionsUpdate(cl *controlplanev1.Cluster, upd *controlplanev1.ClusterUpdate, paths []string) (owned map[string]bool, err error) {
	dual := f.dualModel[cl.GetId()]

	svcs := []*updateConnSvc{
		{name: svcKafkaAPI, conns: upd.GetKafkaApi().GetConnections(), sasl: upd.GetKafkaApi().GetSasl(), mtls: upd.GetKafkaApi().GetMtls(), stored: cl.GetKafkaApi().GetConnections(), storedMTLS: cl.GetKafkaApi().GetMtls()},
		{name: svcHTTPProxy, conns: upd.GetHttpProxy().GetConnections(), sasl: upd.GetHttpProxy().GetSasl(), mtls: upd.GetHttpProxy().GetMtls(), stored: cl.GetHttpProxy().GetConnections(), storedMTLS: cl.GetHttpProxy().GetMtls()},
		{name: svcSchemaRegistry, conns: upd.GetSchemaRegistry().GetConnections(), sasl: upd.GetSchemaRegistry().GetSasl(), mtls: upd.GetSchemaRegistry().GetMtls(), stored: cl.GetSchemaRegistry().GetConnections(), storedMTLS: cl.GetSchemaRegistry().GetMtls()},
	}
	for _, s := range svcs {
		s.masked = maskHasService(paths, s.name)
		s.leafMasked = maskHasConnectionsLeaf(paths, s.name)
		// Unmasked connections in the body are cleared before mapping — the
		// mask is authoritative (mirrors validateConnectionsUpdate).
		if !s.masked {
			s.conns = nil
		}
	}

	inPlay := func(s *updateConnSvc) bool { return s.masked && len(s.conns) > 0 }
	emptyLeaf := func(s *updateConnSvc) bool { return dual && s.leafMasked && len(s.conns) == 0 }

	touched := false
	for _, s := range svcs {
		if inPlay(s) || emptyLeaf(s) {
			touched = true
		}
	}
	if err := f.checkConnectionsFlag(touched); err != nil {
		return nil, err
	}
	if !touched {
		// A legacy sasl/mtls mutation on a dual-model cluster cannot ride the
		// legacy path — its bare listener names no longer exist.
		if dual {
			for _, s := range svcs {
				if maskHasLegacyListenerPath(paths, s.name) && len(s.conns) == 0 {
					return nil, connInvalidf("cannot update legacy sasl/mtls on %s for a cluster with public/private listeners; use the connections field instead", s.name)
				}
			}
		}
		return nil, nil
	}

	if cl.GetCloudProvider() != controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS || cl.GetType() != controlplanev1.Cluster_TYPE_BYOC {
		return nil, connectionsEnvelopeErr()
	}
	// A private-only cluster's network has no public infrastructure; adding
	// public listeners in place is rejected (the CP enforces this in
	// provisioning, past the public-API mapper).
	if storedPrivateOnly(cl) {
		for _, s := range svcs {
			if inPlay(s) && hasPublicConnection(s.conns) {
				return nil, connInvalidf("a private-only cluster cannot gain public listeners; recreate the cluster with the desired topology")
			}
		}
	}
	for _, s := range svcs {
		if emptyLeaf(s) {
			return nil, connInvalidf("%s.connections cannot be cleared; a cluster that uses connections must keep at least one connection", s.name)
		}
	}
	if dual {
		for _, s := range svcs {
			if maskHasLegacyListenerPath(paths, s.name) && len(s.conns) == 0 {
				return nil, connInvalidf("cannot update legacy sasl/mtls on %s for a cluster with public/private listeners; use the connections field instead", s.name)
			}
		}
	}

	// Cross-service semantics on the EFFECTIVE post-update state: in-play
	// services contribute the request topology, others their stored topology.
	for _, s := range svcs {
		if !inPlay(s) {
			continue
		}
		if err := validateConnectionSpecs(s.name, s.conns); err != nil {
			return nil, err
		}
		if s.sasl != nil && maskHasSASL(paths, s.name) {
			return nil, connInvalidf("%s.sasl cannot be set together with %s.connections; connections define per-listener auth", s.name, s.name)
		}
		if !hasMTLSConnection(s.conns) && s.mtls != nil &&
			((maskHasMTLSEnabled(paths, s.name) && s.mtls.GetEnabled()) ||
				(maskHasMTLSCA(paths, s.name) && len(s.mtls.GetCaCertificatesPem()) > 0)) {
			return nil, connInvalidf("%s.mtls cannot be set when no connection uses mTLS auth; add an mTLS connection or remove the mtls block", s.name)
		}
	}
	type effTop struct {
		name                  string
		hasPublic, hasPrivate bool
	}
	var effective []effTop
	for _, s := range svcs {
		var hasPub, hasPriv bool
		if inPlay(s) {
			hasPub, hasPriv = connectionPresence(s.conns)
		} else {
			for _, e := range s.stored {
				if e.GetConfig().GetType() == connTypePrivate {
					hasPriv = true
				} else {
					hasPub = true
				}
			}
			if !hasPub && !hasPriv {
				continue
			}
		}
		effective = append(effective, effTop{s.name, hasPub, hasPriv})
	}
	if len(effective) >= 2 {
		ref := effective[0]
		for _, e := range effective[1:] {
			if e.hasPublic != ref.hasPublic || e.hasPrivate != ref.hasPrivate {
				return nil, connInvalidf("all services must have the same connection network types; %s is %s but %s is %s",
					ref.name, describeConnectionTopology(ref.hasPublic, ref.hasPrivate), e.name, describeConnectionTopology(e.hasPublic, e.hasPrivate))
			}
		}
	}

	// mTLS CA rules, mask-aware (mirrors validateServiceConnectionsMTLSForUpdate).
	for _, s := range svcs {
		if !inPlay(s) || !hasMTLSConnection(s.conns) {
			continue
		}
		if maskHasMTLSEnabled(paths, s.name) && s.mtls != nil && !s.mtls.GetEnabled() {
			return nil, connInvalidf("%s declares an mTLS connection but %s.mtls.enabled is false; set it to true instead of relying on an implicit override", s.name, s.name)
		}
		if maskHasMTLSCA(paths, s.name) {
			if len(s.mtls.GetCaCertificatesPem()) == 0 {
				return nil, connInvalidf("%s declares an mTLS connection but %s.mtls.ca_certificates_pem is empty; provide the trusted client CA bundle", s.name, s.name)
			}
		} else if len(s.storedMTLS.GetCaCertificatesPem()) == 0 {
			return nil, connInvalidf("%s declares an mTLS connection but requires ca_certificates_pem; none provided and none stored on the cluster", s.name)
		}
	}

	// Apply: reconcile each in-play service and resolve its mtls block via the
	// mask-driven merge (mergeConnectionMTLS mirror).
	owned = map[string]bool{}
	for _, s := range svcs {
		if !inPlay(s) {
			continue
		}
		owned[s.name] = true
		newConns := reconcileConnections(s.name, s.stored, s.conns)
		var newMTLS *controlplanev1.MTLSSpec
		if hasMTLSConnection(s.conns) {
			newMTLS = &controlplanev1.MTLSSpec{Enabled: true}
			if maskHasMTLSCA(paths, s.name) {
				newMTLS.CaCertificatesPem = s.mtls.GetCaCertificatesPem()
			} else {
				newMTLS.CaCertificatesPem = s.storedMTLS.GetCaCertificatesPem()
			}
			if maskHasMTLSRules(paths, s.name) {
				newMTLS.PrincipalMappingRules = s.mtls.GetPrincipalMappingRules()
			} else {
				newMTLS.PrincipalMappingRules = s.storedMTLS.GetPrincipalMappingRules()
			}
		}
		// GET always projects a non-nil sasl block ({enabled:false} for an
		// mTLS-only service, {enabled:true} otherwise) — the projection that
		// forces read-modify-write clients onto leaf-granular masks.
		newSASL := &controlplanev1.SASLSpec{Enabled: hasSASLConnection(s.conns)}
		switch s.name {
		case svcKafkaAPI:
			cl.GetKafkaApi().Connections = newConns
			cl.GetKafkaApi().Mtls = newMTLS
			cl.GetKafkaApi().Sasl = newSASL
		case svcHTTPProxy:
			cl.GetHttpProxy().Connections = newConns
			cl.GetHttpProxy().Mtls = newMTLS
			cl.GetHttpProxy().Sasl = newSASL
		default:
			cl.GetSchemaRegistry().Connections = newConns
			cl.GetSchemaRegistry().Mtls = newMTLS
			cl.GetSchemaRegistry().Sasl = newSASL
		}
	}
	f.dualModel[cl.GetId()] = true
	cl.ConnectionType = connectionsClusterType(cl.GetKafkaApi().GetConnections(), cl.GetConnectionType())
	return owned, nil
}

// connectionsOwnedPath reports whether a mask path belongs to a service the
// connections update already applied, so the generic mask loop must skip it.
func connectionsOwnedPath(owned map[string]bool, path string) bool {
	for svc := range owned {
		if path == svc || strings.HasPrefix(path, svc+".") {
			return true
		}
	}
	return false
}

// firstOrEmpty returns the first element of a string slice, or "".
func firstOrEmpty(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}
