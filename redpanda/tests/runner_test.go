//go:build live_test && (all || cluster_aws || cluster_gcp || byoc_aws || byoc_gcp)

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

package tests

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc/sweep"
)

func testRunner(ctx context.Context, name, rename, version, testFile string, customVars map[string]config.Variable, t *testing.T) {
	origTestCaseVars := make(map[string]config.Variable)
	maps.Copy(origTestCaseVars, acc.ProviderCfgIDSecretVars)
	origTestCaseVars["resource_group_name"] = config.StringVariable(name)
	origTestCaseVars["network_name"] = config.StringVariable(name)
	origTestCaseVars["cluster_name"] = config.StringVariable(name)
	origTestCaseVars["user_name"] = config.StringVariable(name)
	origTestCaseVars["topic_name"] = config.StringVariable(name)
	if acc.ThroughputTier != "" {
		origTestCaseVars["throughput_tier"] = config.StringVariable(acc.ThroughputTier)
	}

	if len(customVars) > 0 {
		for k, v := range customVars {
			origTestCaseVars[k] = v
		}
	}
	if version != "" {
		origTestCaseVars["version"] = config.StringVariable(version)
	}

	updateTestCaseVars := make(map[string]config.Variable)
	maps.Copy(updateTestCaseVars, origTestCaseVars)
	updateTestCaseVars["cluster_name"] = config.StringVariable(rename)
	updateTestCaseVars["cluster_allow_deletion"] = config.BoolVariable(true)
	updateTestCaseVars["user_allow_deletion"] = config.BoolVariable(true)
	updateTestCaseVars["acl_allow_deletion"] = config.BoolVariable(true)

	compatibilityUpdateVars := make(map[string]config.Variable)
	maps.Copy(compatibilityUpdateVars, updateTestCaseVars)
	compatibilityUpdateVars["compatibility_level"] = config.StringVariable("FORWARD")

	userAllowDeletionFalseVars := make(map[string]config.Variable)
	maps.Copy(userAllowDeletionFalseVars, updateTestCaseVars)
	userAllowDeletionFalseVars["user_allow_deletion"] = config.BoolVariable(false)

	userAllowDeletionTrueVars := make(map[string]config.Variable)
	maps.Copy(userAllowDeletionTrueVars, updateTestCaseVars)
	userAllowDeletionTrueVars["user_allow_deletion"] = config.BoolVariable(true)

	aclAllowDeletionFalseVars := make(map[string]config.Variable)
	maps.Copy(aclAllowDeletionFalseVars, updateTestCaseVars)
	aclAllowDeletionFalseVars["acl_allow_deletion"] = config.BoolVariable(false)

	aclAllowDeletionTrueVars := make(map[string]config.Variable)
	maps.Copy(aclAllowDeletionTrueVars, updateTestCaseVars)
	aclAllowDeletionTrueVars["acl_allow_deletion"] = config.BoolVariable(true)

	srACLAllowDeletionFalseVars := make(map[string]config.Variable)
	maps.Copy(srACLAllowDeletionFalseVars, updateTestCaseVars)
	srACLAllowDeletionFalseVars["sr_acl_allow_deletion"] = config.BoolVariable(false)

	srACLAllowDeletionTrueVars := make(map[string]config.Variable)
	maps.Copy(srACLAllowDeletionTrueVars, updateTestCaseVars)
	srACLAllowDeletionTrueVars["sr_acl_allow_deletion"] = config.BoolVariable(true)

	pipelineRunningVars := make(map[string]config.Variable)
	maps.Copy(pipelineRunningVars, updateTestCaseVars)
	pipelineRunningVars["pipeline_state"] = config.StringVariable("running")

	pipelineAllowDeletionTrueVars := make(map[string]config.Variable)
	maps.Copy(pipelineAllowDeletionTrueVars, pipelineRunningVars)
	pipelineAllowDeletionTrueVars["pipeline_allow_deletion"] = config.BoolVariable(true)

	c, err := acc.NewTestClients(ctx, acc.ClientID, acc.ClientSecret, acc.CloudEnv)
	if err != nil {
		t.Fatal(err)
	}

	acc.Register(acc.KindCluster, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.Cluster{ClusterName: name, Client: c}.SweepCluster("")
	}))
	acc.Register(acc.KindCluster, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.Cluster{ClusterName: rename, Client: c}.SweepCluster("")
	}))
	acc.Register(acc.KindNetwork, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.Network{NetworkName: name, Client: c}.SweepNetworks("")
	}))
	acc.Register(acc.KindResourceGroup, acc.CleanupFunc(func(_ context.Context) error {
		return sweep.ResourceGroup{ResourceGroupName: name, Client: c}.SweepResourceGroup("")
	}))

	checkFuncs, err := acc.BuildTestCheckFuncs(testFile, name)
	if err != nil {
		t.Fatal(err)
	}

	testFileContent, err := os.ReadFile(testFile + "/main.tf") // #nosec G304 -- testFile is controlled by test constants
	if err != nil {
		t.Fatal(fmt.Errorf("failed to read test file: %w", err))
	}
	hasSchemaRegistryACL := strings.Contains(string(testFileContent), `resource "redpanda_schema_registry_acl" "read_product"`)
	hasSchema := strings.Contains(string(testFileContent), `resource "redpanda_schema" "user_schema"`)
	hasRole := strings.Contains(string(testFileContent), `resource "redpanda_role" "developer"`)
	hasTopic := strings.Contains(string(testFileContent), `resource "redpanda_topic" "test"`)
	hasPipeline := strings.Contains(string(testFileContent), `resource "redpanda_pipeline" "test"`)
	hasPasswordWo := strings.Contains(string(testFileContent), "var.user_password_wo")
	hasMaintenanceWindow := strings.Contains(string(testFileContent), "maintenance_window_config")

	if hasMaintenanceWindow {
		checkFuncs = append(checkFuncs,
			resource.TestCheckResourceAttr(acc.ClusterResourceName, "maintenance_window_config.day_hour.hour_of_day", "0"))
	}

	steps := []resource.TestStep{
		{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          origTestCaseVars,
			Check:                    resource.ComposeAggregateTestCheckFunc(checkFuncs...),
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			},
		},
		{
			ResourceName:    acc.UserResourceName,
			ConfigDirectory: config.StaticDirectory(testFile),
			ConfigVariables: origTestCaseVars,
			ImportState:     true,
			ImportStateIdFunc: func(_ *terraform.State) (string, error) {
				i, err := c.ClusterForName(ctx, name)
				if err != nil {
					return "", errors.New("test error: unable to get cluster by name")
				}
				importID := fmt.Sprintf("%v,%v", name, i.GetId())
				return importID, nil
			},
			ImportStateCheck: func(state []*terraform.InstanceState) error {
				attr := state[0].Attributes
				id, user := attr["id"], attr["name"]
				if user != name {
					return fmt.Errorf("expected user %q; got %q", name, user)
				}
				if id != name {
					return fmt.Errorf("expected ID %q; got %q", name, id)
				}
				if cloudURL := attr["cluster_api_url"]; cloudURL == "" {
					return errors.New("unexpected empty cloud URL")
				}
				if pw, ok := attr["password"]; ok {
					return fmt.Errorf("expected empty password; got %q", pw)
				}
				if allowDeletion := attr["allow_deletion"]; allowDeletion != acc.AllowDeletionFalseValue {
					return fmt.Errorf("expected allow_deletion to default to false; got %q", allowDeletion)
				}
				return nil
			},
			ImportStateVerifyIgnore:  []string{"tags"},
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
		},
		{
			ResourceName:    acc.UserResourceName,
			ConfigDirectory: config.StaticDirectory(testFile),
			ConfigVariables: origTestCaseVars,
			ImportState:     true,
			ImportStateIdFunc: func(_ *terraform.State) (string, error) {
				i, err := c.ClusterForName(ctx, name)
				if err != nil {
					return "", errors.New("test error: unable to get cluster by name")
				}
				importID := fmt.Sprintf("%v,%v,test-password,SCRAM-SHA-256", name, i.GetId())
				return importID, nil
			},
			ImportStateCheck: func(state []*terraform.InstanceState) error {
				attr := state[0].Attributes
				if attr["name"] != name {
					return fmt.Errorf("expected user name %q; got %q", name, attr["name"])
				}
				if attr["id"] != name {
					return fmt.Errorf("expected ID %q; got %q", name, attr["id"])
				}
				if attr["password"] != "test-password" {
					return fmt.Errorf("expected password 'test-password'; got %q", attr["password"])
				}
				if !strings.EqualFold(attr["mechanism"], "SCRAM-SHA-256") {
					return fmt.Errorf("expected mechanism 'scram-sha-256' (case-insensitive); got %q", attr["mechanism"])
				}
				if cloudURL := attr["cluster_api_url"]; cloudURL == "" {
					return errors.New("unexpected empty cloud URL")
				}
				if allowDeletion := attr["allow_deletion"]; allowDeletion != acc.AllowDeletionFalseValue {
					return fmt.Errorf("expected allow_deletion to default to false; got %q", allowDeletion)
				}
				return nil
			},
			ImportStateVerifyIgnore:  []string{"tags"},
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
		},
		{
			ResourceName:             acc.ClusterResourceName,
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          origTestCaseVars,
			ImportState:              true,
			ImportStateVerify:        true,
			ImportStateVerifyIgnore:  []string{"tags"},
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
		},
		{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          updateTestCaseVars,
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(acc.ResourceGroupName, "name", name),
				resource.TestCheckResourceAttr(acc.NetworkResourceName, "name", name),
				resource.TestCheckResourceAttr(acc.ClusterResourceName, "name", rename),
			),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			},
		},
		{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          userAllowDeletionFalseVars,
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(acc.UserResourceName, "allow_deletion", "false"),
				resource.TestCheckResourceAttr(acc.ClusterResourceName, "name", rename),
			),
		},
		{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          userAllowDeletionTrueVars,
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(acc.UserResourceName, "allow_deletion", "true"),
				resource.TestCheckResourceAttr(acc.ClusterResourceName, "name", rename),
			),
		},
		{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          aclAllowDeletionFalseVars,
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
			Check: resource.ComposeAggregateTestCheckFunc(
				func() resource.TestCheckFunc {
					testFileContent, err := os.ReadFile(testFile + "/main.tf") // #nosec G304 -- testFile is controlled by test constants
					if err != nil {
						return func(_ *terraform.State) error {
							return fmt.Errorf("failed to read test file: %w", err)
						}
					}
					aclResourceName := acc.ClusterAdminACLResourceName
					if strings.Contains(string(testFileContent), `resource "redpanda_acl" "topic_access"`) {
						aclResourceName = acc.TopicAccessACLResourceName
					}
					return resource.TestCheckResourceAttr(aclResourceName, "allow_deletion", "false")
				}(),
				resource.TestCheckResourceAttr(acc.UserResourceName, "allow_deletion", "true"),
			),
		},
		{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          aclAllowDeletionTrueVars,
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
			Check: resource.ComposeAggregateTestCheckFunc(
				func() resource.TestCheckFunc {
					testFileContent, err := os.ReadFile(testFile + "/main.tf") // #nosec G304 -- testFile is controlled by test constants
					if err != nil {
						return func(_ *terraform.State) error {
							return fmt.Errorf("failed to read test file: %w", err)
						}
					}
					aclResourceName := acc.ClusterAdminACLResourceName
					if strings.Contains(string(testFileContent), `resource "redpanda_acl" "topic_access"`) {
						aclResourceName = acc.TopicAccessACLResourceName
					}
					return resource.TestCheckResourceAttr(aclResourceName, "allow_deletion", "true")
				}(),
				resource.TestCheckResourceAttr(acc.UserResourceName, "allow_deletion", "true"),
			),
		},
		{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          srACLAllowDeletionFalseVars,
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
			Check: resource.ComposeAggregateTestCheckFunc(
				func() resource.TestCheckFunc {
					testFileContent, err := os.ReadFile(testFile + "/main.tf") // #nosec G304 -- testFile is controlled by test constants
					if err != nil {
						return func(_ *terraform.State) error {
							return fmt.Errorf("failed to read test file: %w", err)
						}
					}
					if strings.Contains(string(testFileContent), `resource "redpanda_schema_registry_acl" "read_product"`) {
						return resource.TestCheckResourceAttr(acc.SchemaRegistryACLReadProductName, "allow_deletion", "false")
					}
					return func(_ *terraform.State) error {
						return nil
					}
				}(),
			),
		},
		{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          srACLAllowDeletionTrueVars,
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
			Check: resource.ComposeAggregateTestCheckFunc(
				func() resource.TestCheckFunc {
					testFileContent, err := os.ReadFile(testFile + "/main.tf") // #nosec G304 -- testFile is controlled by test constants
					if err != nil {
						return func(_ *terraform.State) error {
							return fmt.Errorf("failed to read test file: %w", err)
						}
					}
					if strings.Contains(string(testFileContent), `resource "redpanda_schema_registry_acl" "read_product"`) {
						return resource.TestCheckResourceAttr(acc.SchemaRegistryACLReadProductName, "allow_deletion", "true")
					}
					return func(_ *terraform.State) error {
						return nil
					}
				}(),
			),
		},
		{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          compatibilityUpdateVars,
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(acc.ResourceGroupName, "name", name),
				resource.TestCheckResourceAttr(acc.NetworkResourceName, "name", name),
				resource.TestCheckResourceAttr(acc.ClusterResourceName, "name", rename),
				func() resource.TestCheckFunc {
					testFileContent, err := os.ReadFile(testFile + "/main.tf") // #nosec G304 -- testFile is controlled by test constants
					if err != nil {
						return func(_ *terraform.State) error {
							return fmt.Errorf("failed to read test file: %w", err)
						}
					}
					if strings.Contains(string(testFileContent), `resource "redpanda_schema" "product_schema"`) {
						return resource.TestCheckResourceAttr(acc.SchemaProductResourceName, "compatibility", "FORWARD")
					}
					return func(_ *terraform.State) error {
						return nil
					}
				}(),
			),
		},
	}

	fieldMutationVars := make(map[string]config.Variable)
	maps.Copy(fieldMutationVars, updateTestCaseVars)
	fieldMutationVars["cluster_tags"] = config.MapVariable(map[string]config.Variable{
		"env":  config.StringVariable("acc"),
		"team": config.StringVariable("platform"),
	})
	fieldMutationChecks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr(acc.ClusterResourceName, "tags.env", "acc"),
		resource.TestCheckResourceAttr(acc.ClusterResourceName, "tags.team", "platform"),
	}
	if hasMaintenanceWindow {
		fieldMutationVars["maintenance_hour_of_day"] = config.IntegerVariable(3)
		fieldMutationChecks = append(fieldMutationChecks,
			resource.TestCheckResourceAttr(acc.ClusterResourceName, "maintenance_window_config.day_hour.hour_of_day", "3"))
	}
	if hasTopic {
		fieldMutationVars["partition_count"] = config.IntegerVariable(6)
		fieldMutationVars["topic_retention_ms"] = config.StringVariable("3600000")
		fieldMutationChecks = append(fieldMutationChecks,
			resource.TestCheckResourceAttr(acc.TopicResourceName, "partition_count", "6"),
			resource.TestCheckResourceAttr(acc.TopicResourceName, "configuration.retention.ms", "3600000"),
		)
	}
	steps = append(steps, resource.TestStep{
		ConfigDirectory:          config.StaticDirectory(testFile),
		ConfigVariables:          fieldMutationVars,
		ProtoV6ProviderFactories: acc.ProtoV6Factories,
		Check:                    resource.ComposeAggregateTestCheckFunc(fieldMutationChecks...),
	})

	// Topic-configuration empty-plan-guard pair: flip topic_configuration to {}
	// then back to the basic config. Both steps assert ExpectEmptyPlan to pin
	// the redpanda.* strip branch in mergeWithPlannedConfig — without it, the
	// post-v26.1.1 broker injection of redpanda.storage.mode would force a
	// perpetual plan diff.
	if hasTopic {
		topicNoConfigVars := make(map[string]config.Variable)
		maps.Copy(topicNoConfigVars, fieldMutationVars)
		topicNoConfigVars["topic_configuration"] = config.MapVariable(map[string]config.Variable{})

		steps = append(steps,
			resource.TestStep{
				ConfigDirectory:          config.StaticDirectory(testFile),
				ConfigVariables:          topicNoConfigVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(acc.TopicResourceName, "configuration.%", "0"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			resource.TestStep{
				ConfigDirectory:          config.StaticDirectory(testFile),
				ConfigVariables:          fieldMutationVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(acc.TopicResourceName, "configuration.cleanup.policy", "delete"),
					resource.TestCheckResourceAttr(acc.TopicResourceName, "configuration.retention.ms", "3600000"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		)
	}

	zonesSentinelVars := make(map[string]config.Variable)
	maps.Copy(zonesSentinelVars, fieldMutationVars)
	zonesSentinelVars["zones"] = config.ListVariable(
		config.StringVariable("zzz9-az9"),
		config.StringVariable("zzz9-az8"),
	)
	steps = append(steps, resource.TestStep{
		ConfigDirectory:          config.StaticDirectory(testFile),
		ConfigVariables:          zonesSentinelVars,
		PlanOnly:                 true,
		ExpectNonEmptyPlan:       true,
		ProtoV6ProviderFactories: acc.ProtoV6Factories,
	})

	regionSentinelVars := make(map[string]config.Variable)
	maps.Copy(regionSentinelVars, fieldMutationVars)
	regionSentinelVars["region"] = config.StringVariable("region-sentinel-zzz")
	steps = append(steps, resource.TestStep{
		ConfigDirectory:          config.StaticDirectory(testFile),
		ConfigVariables:          regionSentinelVars,
		PlanOnly:                 true,
		ExpectNonEmptyPlan:       true,
		ProtoV6ProviderFactories: acc.ProtoV6Factories,
	})

	if hasTopic {
		rfSentinelVars := make(map[string]config.Variable)
		maps.Copy(rfSentinelVars, fieldMutationVars)
		rfSentinelVars["replication_factor"] = config.IntegerVariable(1)
		steps = append(steps, resource.TestStep{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          rfSentinelVars,
			PlanOnly:                 true,
			ExpectNonEmptyPlan:       true,
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
		})
	}

	if hasSchemaRegistryACL {
		steps = append(steps, resource.TestStep{
			ResourceName:    acc.SchemaRegistryACLReadProductName,
			ConfigDirectory: config.StaticDirectory(testFile),
			ConfigVariables: updateTestCaseVars,
			ImportState:     true,
			ImportStateIdFunc: func(state *terraform.State) (string, error) {
				rs, ok := state.RootModule().Resources[acc.SchemaRegistryACLReadProductName]
				if !ok {
					return "", errors.New("schema registry ACL resource not found in state")
				}

				clusterID := rs.Primary.Attributes["cluster_id"]
				principal := rs.Primary.Attributes["principal"]
				resourceType := rs.Primary.Attributes["resource_type"]
				resourceName := rs.Primary.Attributes["resource_name"]
				patternType := rs.Primary.Attributes["pattern_type"]
				host := rs.Primary.Attributes["host"]
				operation := rs.Primary.Attributes["operation"]
				permission := rs.Primary.Attributes["permission"]
				username := rs.Primary.Attributes["username"]
				password := rs.Primary.Attributes["password"]

				// Bearer-primary: 8-field form when the fixture didn't set
				// username/password. Basic auth appends ,username,password.
				if username == "" && password == "" {
					return fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s",
						clusterID, principal, resourceType, resourceName, patternType, host, operation, permission), nil
				}
				return fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s,%s,%s",
					clusterID, principal, resourceType, resourceName, patternType, host, operation, permission, username, password), nil
			},
			ImportStateCheck: func(state []*terraform.InstanceState) error {
				attr := state[0].Attributes
				if attr["cluster_id"] == "" {
					return errors.New("expected non-empty cluster_id")
				}
				if attr["principal"] == "" {
					return errors.New("expected non-empty principal")
				}
				if attr["resource_type"] != "SUBJECT" {
					return fmt.Errorf("expected resource_type SUBJECT; got %q", attr["resource_type"])
				}
				if attr["resource_name"] != "product-" {
					return fmt.Errorf("expected resource_name 'product-'; got %q", attr["resource_name"])
				}
				if attr["pattern_type"] != "PREFIXED" {
					return fmt.Errorf("expected pattern_type PREFIXED; got %q", attr["pattern_type"])
				}
				if attr["host"] == "" {
					return errors.New("expected non-empty host")
				}
				if attr["operation"] != "READ" {
					return fmt.Errorf("expected operation READ; got %q", attr["operation"])
				}
				if attr["permission"] != "ALLOW" {
					return fmt.Errorf("expected permission ALLOW; got %q", attr["permission"])
				}
				if attr["id"] == "" {
					return errors.New("expected non-empty id")
				}
				if allowDeletion := attr["allow_deletion"]; allowDeletion != acc.AllowDeletionFalseValue {
					return fmt.Errorf("expected allow_deletion to default to false; got %q", allowDeletion)
				}
				return nil
			},
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
		})
	}

	if hasSchema {
		steps = append(steps, resource.TestStep{
			ResourceName:    acc.SchemaResourceName,
			ConfigDirectory: config.StaticDirectory(testFile),
			ConfigVariables: updateTestCaseVars,
			ImportState:     true,
			ImportStateIdFunc: func(state *terraform.State) (string, error) {
				rs, ok := state.RootModule().Resources[acc.SchemaResourceName]
				if !ok {
					return "", errors.New("schema resource not found in state")
				}

				clusterID := rs.Primary.Attributes["cluster_id"]
				subject := rs.Primary.Attributes["subject"]
				version := rs.Primary.Attributes["version"]
				username := rs.Primary.Attributes["username"]
				password := rs.Primary.Attributes["password"]

				// Bearer-primary: 3-field form when the fixture didn't set
				// username/password. Basic auth appends ,username,password.
				if username == "" && password == "" {
					return fmt.Sprintf("%s,%s,%s", clusterID, subject, version), nil
				}
				return fmt.Sprintf("%s,%s,%s,%s,%s",
					clusterID, subject, version, username, password), nil
			},
			ImportStateCheck: func(state []*terraform.InstanceState) error {
				attr := state[0].Attributes
				if attr["subject"] != name+"-value" {
					return fmt.Errorf("expected subject %q; got %q", name+"-value", attr["subject"])
				}
				if attr["schema_type"] != "AVRO" {
					return fmt.Errorf("expected schema_type AVRO; got %q", attr["schema_type"])
				}
				if attr["version"] == "" {
					return errors.New("expected non-empty version")
				}
				if attr["id"] == "" {
					return errors.New("expected non-empty id")
				}
				if attr["cluster_id"] == "" {
					return errors.New("expected non-empty cluster_id")
				}
				if allowDeletion := attr["allow_deletion"]; allowDeletion != acc.AllowDeletionFalseValue {
					return fmt.Errorf("expected allow_deletion to default to false; got %q", allowDeletion)
				}
				return nil
			},
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
		})
	}

	if hasRole {
		steps = append(steps, resource.TestStep{
			ResourceName:    acc.RoleResourceName,
			ConfigDirectory: config.StaticDirectory(testFile),
			ConfigVariables: updateTestCaseVars,
			ImportState:     true,
			ImportStateIdFunc: func(_ *terraform.State) (string, error) {
				cluster, err := c.ClusterForName(ctx, rename)
				if err != nil {
					return "", errors.New("test error: unable to get cluster by name")
				}
				importID := fmt.Sprintf("developer,%v", cluster.GetId())
				return importID, nil
			},
			ImportStateCheck: func(state []*terraform.InstanceState) error {
				attr := state[0].Attributes
				if attr["name"] != "developer" {
					return fmt.Errorf("expected role name 'developer'; got %q", attr["name"])
				}
				if attr["id"] != "developer" {
					return fmt.Errorf("expected ID 'developer'; got %q", attr["id"])
				}
				if cloudURL := attr["cluster_api_url"]; cloudURL == "" {
					return errors.New("expected cluster_api_url to be set after import")
				}
				if allowDeletion := attr["allow_deletion"]; allowDeletion != acc.AllowDeletionFalseValue {
					return fmt.Errorf("expected allow_deletion to default to false; got %q", allowDeletion)
				}
				return nil
			},
			ImportStateVerifyIgnore:  []string{"tags"},
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
		})
	}

	if hasTopic {
		steps = append(steps, resource.TestStep{
			ResourceName:    acc.TopicResourceName,
			ConfigDirectory: config.StaticDirectory(testFile),
			ConfigVariables: updateTestCaseVars,
			ImportState:     true,
			ImportStateIdFunc: func(_ *terraform.State) (string, error) {
				cluster, err := c.ClusterForName(ctx, rename)
				if err != nil {
					return "", errors.New("test error: unable to get cluster by name")
				}
				importID := fmt.Sprintf("%s,%v", name, cluster.GetId())
				return importID, nil
			},
			ImportStateCheck: func(state []*terraform.InstanceState) error {
				attr := state[0].Attributes
				if attr["name"] != name {
					return fmt.Errorf("expected topic name %q; got %q", name, attr["name"])
				}
				if attr["id"] != name {
					return fmt.Errorf("expected ID %q; got %q", name, attr["id"])
				}
				if cloudURL := attr["cluster_api_url"]; cloudURL == "" {
					return errors.New("expected cluster_api_url to be set after import")
				}
				if allowDeletion := attr["allow_deletion"]; allowDeletion != acc.AllowDeletionFalseValue {
					return fmt.Errorf("expected allow_deletion to default to false; got %q", allowDeletion)
				}
				return nil
			},
			ImportStateVerifyIgnore:  []string{"tags"},
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
		})
	}

	if hasPasswordWo {
		passwordWoRotationVars := make(map[string]config.Variable)
		maps.Copy(passwordWoRotationVars, fieldMutationVars)
		passwordWoRotationVars["user_password_wo"] = config.StringVariable("rotated-secret-v1")
		passwordWoRotationVars["user_password_wo_version"] = config.IntegerVariable(1)
		steps = append(steps, resource.TestStep{
			ConfigDirectory:          config.StaticDirectory(testFile),
			ConfigVariables:          passwordWoRotationVars,
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(acc.UserResourceName, "password_wo_version", "1"),
				func(_ *terraform.State) error {
					return acc.VerifySRAuth(ctx, c, rename, name, "rotated-secret-v1")
				},
			),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			},
		})
	}

	if hasPipeline {
		steps = append(steps,
			resource.TestStep{
				ResourceName:    acc.PipelineResourceName,
				ConfigDirectory: config.StaticDirectory(testFile),
				ConfigVariables: updateTestCaseVars,
				ImportState:     true,
				ImportStateIdFunc: func(state *terraform.State) (string, error) {
					rs, ok := state.RootModule().Resources[acc.PipelineResourceName]
					if !ok {
						return "", errors.New("pipeline resource not found in state")
					}
					pipelineID := rs.Primary.Attributes["id"]
					cluster, err := c.ClusterForName(ctx, rename)
					if err != nil {
						return "", errors.New("test error: unable to get cluster by name")
					}
					importID := fmt.Sprintf("%s,%v", pipelineID, cluster.GetId())
					return importID, nil
				},
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					attr := state[0].Attributes
					if attr["id"] == "" {
						return errors.New("expected non-empty id")
					}
					if attr["display_name"] != "test-pipeline" {
						return fmt.Errorf("expected display_name 'test-pipeline'; got %q", attr["display_name"])
					}
					if cloudURL := attr["cluster_api_url"]; cloudURL == "" {
						return errors.New("expected cluster_api_url to be set after import")
					}
					if attr["state"] == "" {
						return errors.New("expected non-empty state")
					}
					return nil
				},
				ImportStateVerifyIgnore:  []string{"config_yaml", "description", "resources", "tags"},
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
			},
			resource.TestStep{
				ConfigDirectory:          config.StaticDirectory(testFile),
				ConfigVariables:          pipelineRunningVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(acc.PipelineResourceName, "state", "running"),
				),
			},
			// Enable deletion for cleanup - pipeline defaults to allow_deletion=false
			resource.TestStep{
				ConfigDirectory:          config.StaticDirectory(testFile),
				ConfigVariables:          pipelineAllowDeletionTrueVars,
				ProtoV6ProviderFactories: acc.ProtoV6Factories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(acc.PipelineResourceName, "allow_deletion", "true"),
				),
			},
		)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { acc.PreCheck(t) },
		Steps:    steps,
	},
	)
}
