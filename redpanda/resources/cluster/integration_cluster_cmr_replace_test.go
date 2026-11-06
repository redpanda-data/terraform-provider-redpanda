//go:build integration

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

package cluster_test

import (
	"regexp"
	"strings"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/integration"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock"
)

const (
	replaceConnectSG      = "arn:aws:ec2:us-east-1:123456789012:security-group/sg-rp-connect"
	replaceConnectProfile = "arn:aws:iam::123456789012:instance-profile/tfrp-rp-connect-ng"
	replaceClusterSG      = "arn:aws:ec2:us-east-1:123456789012:security-group/sg-cluster-b"
	replaceLoneSG         = "arn:aws:ec2:us-east-1:123456789012:security-group/sg-rp-connect-lone"
)

var awsConnectPath = tfjsonpath.New("customer_managed_resources").AtMapKey("aws")

// TestIntegration_Cluster_CMR_EmptyConnectBlockCarriesNothing pins that an AWS
// Connect block the control plane holds with an empty value, the shape a
// leaf-only clear leaves behind, reads back null and neither counts as a
// setting the config omits nor as a sent half of the pair. The other half,
// kept in config alone, is refused at plan like any lone block on a create,
// and the cluster stands; left out, the replace plans and the cluster comes
// up without either.
func TestIntegration_Cluster_CMR_EmptyConnectBlockCarriesNothing(t *testing.T) {
	leafClear := func(t *testing.T, srv *mock.Server, id *string, leaves ...string) func() {
		return updateCMROutOfBand(t, srv, id, &controlplanev1.CustomerManagedResourcesUpdate{
			CloudProvider: &controlplanev1.CustomerManagedResourcesUpdate_Aws{Aws: &controlplanev1.CustomerManagedResourcesUpdate_AWS{}},
		}, leaves...)
	}
	const (
		sgLeaf      = "customer_managed_resources.aws.redpanda_connect_security_group.arn"
		profileLeaf = "customer_managed_resources.aws.redpanda_connect_node_group_instance_profile.arn"
	)
	noSG := statecheck.ExpectKnownValue(clusterAddr, awsConnectPath.AtMapKey("redpanda_connect_security_group"), knownvalue.Null())
	noProfile := statecheck.ExpectKnownValue(clusterAddr, awsConnectPath.AtMapKey("redpanda_connect_node_group_instance_profile"), knownvalue.Null())

	t.Run("one block emptied, the other kept in config", func(t *testing.T) {
		srv, factories := clusterSetup(t)
		const name = "tfrp-mock-cl-empty-sg"
		var id string
		create := integration.CreateStep(clusterAddr, awsBYOVPCConfig(awsBYOVPCOpts{name: name}), nil)
		create.Check = captureClusterID(&id)
		cleared := integration.NoopReapplyStep(clusterAddr, awsBYOVPCConfig(awsBYOVPCOpts{name: name, omitConnectSG: true}), []statecheck.StateCheck{noSG})
		cleared.PreConfig = leafClear(t, srv, &id, sgLeaf)
		resource.UnitTest(t, resource.TestCase{
			ProtoV6ProviderFactories: factories,
			Steps: []resource.TestStep{
				create, cleared,
				{
					Config:      awsBYOVPCConfig(awsBYOVPCOpts{name: name, omitConnectSG: true, clusterSGARN: replaceClusterSG}),
					ExpectError: regexp.MustCompile(`leave\s+the\s+lone\s+one\s+out`),
				},
				integration.NoopReapplyStep(clusterAddr, awsBYOVPCConfig(awsBYOVPCOpts{name: name, omitConnectSG: true}), []statecheck.StateCheck{noSG}),
				integration.RequiresReplaceStep(clusterAddr, awsBYOVPCConfig(awsBYOVPCOpts{name: name, omitConnectSG: true, omitConnectProfile: true, clusterSGARN: replaceClusterSG}),
					[]statecheck.StateCheck{noSG, noProfile}),
			},
		})
	})

	t.Run("both blocks emptied", func(t *testing.T) {
		srv, factories := clusterSetup(t)
		const name = "tfrp-mock-cl-empty-both"
		var id string
		omitBoth := awsBYOVPCOpts{name: name, omitConnectSG: true, omitConnectProfile: true}
		create := integration.CreateStep(clusterAddr, awsBYOVPCConfig(awsBYOVPCOpts{name: name}), nil)
		create.Check = captureClusterID(&id)
		cleared := integration.NoopReapplyStep(clusterAddr, awsBYOVPCConfig(omitBoth), []statecheck.StateCheck{noSG, noProfile})
		cleared.PreConfig = leafClear(t, srv, &id, sgLeaf, profileLeaf)
		replaced := omitBoth
		replaced.clusterSGARN = replaceClusterSG
		resource.UnitTest(t, resource.TestCase{
			ProtoV6ProviderFactories: factories,
			Steps: []resource.TestStep{
				create, cleared,
				integration.RequiresReplaceStep(clusterAddr, awsBYOVPCConfig(replaced), []statecheck.StateCheck{noSG, noProfile}),
			},
		})
	})
}

// TestIntegration_Cluster_CMR_LoneConnectBlockReplaces pins that a cluster
// holding one half of a Connect pair, which never turned Connect on, can be
// replaced once the half is left out of config: kept, the replace is refused
// at plan, as any create with a lone half is, and the cluster stands.
func TestIntegration_Cluster_CMR_LoneConnectBlockReplaces(t *testing.T) {
	srv, factories := clusterSetup(t)
	const name = "tfrp-mock-cl-lone"
	var id string
	create := integration.CreateStep(clusterAddr, awsBYOVPCConfig(awsBYOVPCOpts{name: name, omitConnectSG: true, omitConnectProfile: true}), nil)
	create.Check = captureClusterID(&id)
	named := awsBYOVPCConfig(awsBYOVPCOpts{name: name, connectSGARN: replaceLoneSG, omitConnectProfile: true})
	lone := integration.NoopReapplyStep(clusterAddr, named, nil)
	lone.PreConfig = updateCMROutOfBand(t, srv, &id, &controlplanev1.CustomerManagedResourcesUpdate{
		CloudProvider: &controlplanev1.CustomerManagedResourcesUpdate_Aws{Aws: &controlplanev1.CustomerManagedResourcesUpdate_AWS{
			RedpandaConnectSecurityGroup: &controlplanev1.AWSSecurityGroup{Arn: replaceLoneSG},
		}},
	}, "customer_managed_resources.aws.redpanda_connect_security_group")
	sgARN := awsConnectPath.AtMapKey("redpanda_connect_security_group").AtMapKey("arn")
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			create, lone,
			{
				Config:      awsBYOVPCConfig(awsBYOVPCOpts{name: name, connectSGARN: replaceLoneSG, omitConnectProfile: true, clusterSGARN: replaceClusterSG}),
				ExpectError: regexp.MustCompile(`leave\s+the\s+lone\s+one\s+out`),
			},
			integration.NoopReapplyStep(clusterAddr, named, []statecheck.StateCheck{statecheck.ExpectKnownValue(clusterAddr, sgARN, knownvalue.StringExact(replaceLoneSG))}),
			integration.RequiresReplaceStep(clusterAddr, awsBYOVPCConfig(awsBYOVPCOpts{name: name, omitConnectSG: true, omitConnectProfile: true, clusterSGARN: replaceClusterSG}),
				[]statecheck.StateCheck{statecheck.ExpectKnownValue(clusterAddr, awsConnectPath.AtMapKey("redpanda_connect_security_group"), knownvalue.Null())}),
		},
	})
}

// TestIntegration_Cluster_CMR_ServerEnabledRpsqlReplaces models a cluster
// whose Redpanda SQL the control plane turned on itself, as it does for an
// Agentic Data Plane cluster without customer-managed resources: the config
// never names rpsql, so a replace is planned from a config without it. The
// control plane enables it again on the replacement and rejects a declared
// block, so the plan may only warn, never refuse.
func TestIntegration_Cluster_CMR_ServerEnabledRpsqlReplaces(t *testing.T) {
	srv, factories := clusterSetup(t)
	srv.Cluster.CreateMutator = func(cl *controlplanev1.Cluster) {
		cl.SetRpsql(&controlplanev1.RPSql{Enabled: true, Replicas: 1, Zones: cl.GetZones(), Url: "https://mock.rpsql.redpanda.cloud", Version: "mock-rpsql-v1"})
	}
	const name = "tfrp-mock-cl-rpsql-srv"
	base := awsBYOCConfig(name)
	moved := strings.Replace(base, `zones             = ["use1-az1"]`, `zones             = ["use1-az2"]`, 1)
	if moved == base {
		t.Fatal("zones substitution did not apply")
	}
	rpsqlOn := statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("rpsql").AtMapKey("enabled"), knownvalue.Bool(true))
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, base, []statecheck.StateCheck{rpsqlOn}),
			integration.RequiresReplaceStep(clusterAddr, moved, []statecheck.StateCheck{rpsqlOn}),
		},
	})
}

// TestIntegration_Cluster_CMR_NetworkMoveMismatchRefused pins that a move to
// a known network is refused at plan when the planned customer_managed_resources
// block disagrees with the network, in either direction, and when the network
// cannot be read. The control plane refuses such a create only after the
// current cluster is destroyed.
func TestIntegration_Cluster_CMR_NetworkMoveMismatchRefused(t *testing.T) {
	t.Run("block kept, network without them", func(t *testing.T) {
		_, factories := clusterSetup(t)
		const name = "tfrp-mock-cl-move-keep"
		resource.UnitTest(t, resource.TestCase{
			ProtoV6ProviderFactories: factories,
			Steps: []resource.TestStep{
				integration.CreateStep(clusterAddr, gcpBYOVPCConfig(gcpBYOVPCOpts{name: name, withPlainNetwork: true}), nil),
				{
					Config:      gcpBYOVPCConfig(gcpBYOVPCOpts{name: name, onPlainNetwork: true}),
					ExpectError: regexp.MustCompile(`has\s+no\s+customer-managed\s+resources`),
				},
			},
		})
	})

	t.Run("no block, network with them", func(t *testing.T) {
		_, factories := clusterSetup(t)
		const name = "tfrp-mock-cl-move-gain"
		resource.UnitTest(t, resource.TestCase{
			ProtoV6ProviderFactories: factories,
			Steps: []resource.TestStep{
				integration.CreateStep(clusterAddr, gcpBYOVPCConfig(gcpBYOVPCOpts{name: name, onPlainNetwork: true, omitCMR: true}), nil),
				{
					Config:      gcpBYOVPCConfig(gcpBYOVPCOpts{name: name, withPlainNetwork: true, omitCMR: true}),
					ExpectError: regexp.MustCompile(`has\s+customer-managed\s+resources`),
				},
			},
		})
	})

	t.Run("network cannot be read", func(t *testing.T) {
		_, factories := clusterSetup(t)
		const name = "tfrp-mock-cl-move-unread"
		// A network id the control plane does not know, as one in another
		// organization reads: the lookup fails and the plan must not go on
		// to a destroy the create would not follow.
		moved := strings.Replace(gcpBYOVPCConfig(gcpBYOVPCOpts{name: name, omitCMR: true}),
			"network_id        = redpanda_network.test.id", `network_id        = "0000000000000000unkn"`, 1)
		resource.UnitTest(t, resource.TestCase{
			ProtoV6ProviderFactories: factories,
			Steps: []resource.TestStep{
				integration.CreateStep(clusterAddr, gcpBYOVPCConfig(gcpBYOVPCOpts{name: name}), nil),
				{Config: moved, ExpectError: regexp.MustCompile(`could\s+not\s+be\s+read`)},
			},
		})
	})
}

// TestIntegration_Cluster_CMR_EmptyConnectBlockClearedWithOtherLeaf pins that
// a Connect block the control plane holds with an empty value is cleared by
// the next apply whose config leaves it out, in the same update as any other
// leaf change: the mask names the message, so the control plane removes it
// rather than refusing the empty required leaf.
func TestIntegration_Cluster_CMR_EmptyConnectBlockClearedWithOtherLeaf(t *testing.T) {
	t.Run("aws, both blocks emptied, rpsql enabled", func(t *testing.T) {
		srv, factories := clusterSetup(t)
		const name = "tfrp-mock-cl-empty-upd"
		var id string
		create := integration.CreateStep(clusterAddr, awsBYOVPCConfig(awsBYOVPCOpts{name: name}), nil)
		create.Check = captureClusterID(&id)
		enabled := integration.UpdateLeafStep(clusterAddr, awsBYOVPCConfig(awsBYOVPCOpts{name: name, omitConnectSG: true, omitConnectProfile: true, rpsql: true}), []statecheck.StateCheck{
			statecheck.ExpectKnownValue(clusterAddr, awsConnectPath.AtMapKey("redpanda_connect_security_group"), knownvalue.Null()),
			statecheck.ExpectKnownValue(clusterAddr, awsConnectPath.AtMapKey("redpanda_connect_node_group_instance_profile"), knownvalue.Null()),
			statecheck.ExpectKnownValue(clusterAddr, tfjsonpath.New("rpsql").AtMapKey("enabled"), knownvalue.Bool(true)),
		})
		enabled.PreConfig = updateCMROutOfBand(t, srv, &id, &controlplanev1.CustomerManagedResourcesUpdate{
			CloudProvider: &controlplanev1.CustomerManagedResourcesUpdate_Aws{Aws: &controlplanev1.CustomerManagedResourcesUpdate_AWS{}},
		}, "customer_managed_resources.aws.redpanda_connect_security_group.arn", "customer_managed_resources.aws.redpanda_connect_node_group_instance_profile.arn")
		resource.UnitTest(t, resource.TestCase{
			ProtoV6ProviderFactories: factories,
			Steps:                    []resource.TestStep{create, enabled},
		})
	})

	t.Run("gcp, one account emptied, nat subnet changed", func(t *testing.T) {
		srv, factories := clusterSetup(t)
		const (
			name       = "tfrp-mock-cl-empty-upd-gcp"
			connectAPI = "rpcn-api@tfrp-proj.iam.gserviceaccount.com"
		)
		gcpPath := tfjsonpath.New("customer_managed_resources").AtMapKey("gcp")
		var id string
		create := integration.CreateStep(clusterAddr, gcpBYOVPCConfig(gcpBYOVPCOpts{name: name, connectSAEmail: "rpcn@tfrp-proj.iam.gserviceaccount.com", connectAPISAEmail: connectAPI}), nil)
		create.Check = captureClusterID(&id)
		nat := integration.UpdateLeafStep(clusterAddr, gcpBYOVPCConfig(gcpBYOVPCOpts{name: name, connectAPISAEmail: connectAPI, pscNat: "tfrp-nat"}), []statecheck.StateCheck{
			statecheck.ExpectKnownValue(clusterAddr, gcpPath.AtMapKey("redpanda_connect_service_account"), knownvalue.Null()),
			statecheck.ExpectKnownValue(clusterAddr, gcpPath.AtMapKey("redpanda_connect_api_service_account").AtMapKey("email"), knownvalue.StringExact(connectAPI)),
			statecheck.ExpectKnownValue(clusterAddr, gcpPath.AtMapKey("psc_nat_subnet_name"), knownvalue.StringExact("tfrp-nat")),
		})
		nat.PreConfig = updateCMROutOfBand(t, srv, &id, &controlplanev1.CustomerManagedResourcesUpdate{
			CloudProvider: &controlplanev1.CustomerManagedResourcesUpdate_Gcp{Gcp: &controlplanev1.CustomerManagedResourcesUpdate_GCP{}},
		}, "customer_managed_resources.gcp.redpanda_connect_service_account.email")
		resource.UnitTest(t, resource.TestCase{
			ProtoV6ProviderFactories: factories,
			Steps:                    []resource.TestStep{create, nat},
		})
	})
}

// TestIntegration_Cluster_CMR_TaintedReplaceWithLoneBlock pins what the lone-block
// error says on a tainted cluster, which core plans as a create with no prior
// private state: the state-only refusal cannot run, so the error itself must
// tell a user whose cluster holds both blocks to declare both rather than
// drop the lone one.
func TestIntegration_Cluster_CMR_TaintedReplaceWithLoneBlock(t *testing.T) {
	const (
		connect    = "rpcn@tfrp-proj.iam.gserviceaccount.com"
		connectAPI = "rpcn-api@tfrp-proj.iam.gserviceaccount.com"
	)
	_, factories := clusterSetup(t)
	const name = "tfrp-mock-cl-taint"
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			integration.CreateStep(clusterAddr, gcpBYOVPCConfig(gcpBYOVPCOpts{name: name, connectSAEmail: connect, connectAPISAEmail: connectAPI}), nil),
			{
				Taint:       []string{clusterAddr},
				Config:      gcpBYOVPCConfig(gcpBYOVPCOpts{name: name, connectSAEmail: connect}),
				ExpectError: regexp.MustCompile(`replaces\s+a\s+cluster\s+that\s+has\s+both,\s+declare\s+both`),
			},
		},
	})
}
