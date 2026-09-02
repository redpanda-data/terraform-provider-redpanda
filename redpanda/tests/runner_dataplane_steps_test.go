//go:build live_test

// Copyright 2025 Redpanda Data, Inc.
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
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc"
)

// dataplaneFixture builds the dataplane lifecycle steps for one cluster shape.
//
// Every group is gated on whether the fixture's main.tf actually declares the
// resource, so a scope that drops a resource drops its steps with it and cannot
// silently assert against something that isn't there. Both the dedicated and the
// serverless runners build from this so the two cannot drift: dataplane depth
// belongs to whichever cluster is cheapest to stand up, not to whichever runner
// happened to grow it.
type dataplaneFixture struct {
	dir  string
	name string
	acc.DataplaneGates
}

// clusterIDFunc resolves the cluster ID that dataplane import IDs embed. It is
// supplied per call rather than stored, because the cluster's *name* changes
// mid-sequence: steps before the rename must resolve by the original name and
// steps after it by the new one. The ID itself is stable across the rename.
type clusterIDFunc func() (string, error)

// newDataplaneFixture sniffs dir's main.tf for the dataplane resources it declares.
func newDataplaneFixture(dir, name string) (*dataplaneFixture, error) {
	gates, err := acc.SniffDataplaneGates(dir)
	if err != nil {
		return nil, err
	}
	return &dataplaneFixture{dir: dir, name: name, DataplaneGates: gates}, nil
}

// withVar returns a copy of base with key set to value.
func withVar(base map[string]config.Variable, key string, value config.Variable) map[string]config.Variable {
	out := make(map[string]config.Variable, len(base)+1)
	maps.Copy(out, base)
	out[key] = value
	return out
}

// step is shorthand for the boilerplate every step repeats.
func (f *dataplaneFixture) step(vars map[string]config.Variable, checks ...resource.TestCheckFunc) resource.TestStep {
	return resource.TestStep{
		ConfigDirectory:          config.StaticDirectory(f.dir),
		ConfigVariables:          vars,
		ProtoV6ProviderFactories: acc.ProtoV6Factories,
		Check:                    resource.ComposeAggregateTestCheckFunc(checks...),
	}
}

// UserImportSteps round-trips the user through import twice: once bare, once with
// the password/mechanism tail that the import ID may carry.
func (f *dataplaneFixture) UserImportSteps(vars map[string]config.Variable, clusterID clusterIDFunc) []resource.TestStep {
	if !f.User {
		return nil
	}
	name := f.name
	return []resource.TestStep{
		{
			ResourceName:    acc.UserResourceName,
			ConfigDirectory: config.StaticDirectory(f.dir),
			ConfigVariables: vars,
			ImportState:     true,
			ImportStateIdFunc: func(_ *terraform.State) (string, error) {
				id, err := clusterID()
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("%v,%v", name, id), nil
			},
			ImportStateCheck: func(state []*terraform.InstanceState) error {
				attr := state[0].Attributes
				if attr["name"] != name {
					return fmt.Errorf("expected user %q; got %q", name, attr["name"])
				}
				if attr["id"] != name {
					return fmt.Errorf("expected ID %q; got %q", name, attr["id"])
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
			ConfigDirectory: config.StaticDirectory(f.dir),
			ConfigVariables: vars,
			ImportState:     true,
			ImportStateIdFunc: func(_ *terraform.State) (string, error) {
				id, err := clusterID()
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("%v,%v,test-password,SCRAM-SHA-256", name, id), nil
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
	}
}

// TopicConfigSteps pins the redpanda.* strip branch: an empty configuration must
// re-plan clean despite the broker injecting redpanda.storage.mode post-v26.1.1.
// The mutation pairs with a flip back so state is restored for later steps.
func (f *dataplaneFixture) TopicConfigSteps(base map[string]config.Variable) []resource.TestStep {
	if !f.Topic || !f.TopicConfigurable {
		return nil
	}
	base = f.topicConfigBase(base)
	return []resource.TestStep{
		f.emptyPlanStep(withVar(base, "topic_configuration", config.MapVariable(map[string]config.Variable{})),
			resource.TestCheckResourceAttr(acc.TopicResourceName, "configuration.%", "0")),
		f.topicConfigRestoreStep(base),
	}
}

// TopicClampRegressionSteps pins the max.compaction.lag.ms clamp (issue #355):
// the broker accepts this noop config, ignores it, and echoes back a clamped
// value, so the provider must report the user's value or apply fails with
// "inconsistent result after apply".
//
// Dedicated/BYOC only, and not by preference. Serverless rejects any topic
// carrying three or more configuration entries — on both create and update —
// with `Internal : unexpected number of resources in alter configs response`
// (REASON_CONSOLE_ERROR, retrieved_results=2). Reproduced with a benign third
// key, so it is a backend limit rather than anything about this config.
func (f *dataplaneFixture) TopicClampRegressionSteps(base map[string]config.Variable) []resource.TestStep {
	if !f.Topic || !f.TopicConfigurable {
		return nil
	}
	base = f.topicConfigBase(base)
	return []resource.TestStep{
		f.emptyPlanStep(withVar(base, "topic_configuration", config.MapVariable(map[string]config.Variable{
			"cleanup.policy":        config.StringVariable("delete"),
			"retention.ms":          config.StringVariable("3600000"),
			"max.compaction.lag.ms": config.StringVariable("9223372036854775807"),
		})), resource.TestCheckResourceAttr(acc.TopicResourceName, "configuration.max.compaction.lag.ms", "9223372036854775807")),
		// The broker reports the unset key with a non-dynamic source, so it is
		// filtered back out of state.
		f.emptyPlanStep(base,
			resource.TestCheckNoResourceAttr(acc.TopicResourceName, "configuration.max.compaction.lag.ms")),
	}
}

// topicConfigBase pins retention to the value the restore assertions check,
// rather than trusting the caller to have set a matching one.
func (f *dataplaneFixture) topicConfigBase(base map[string]config.Variable) map[string]config.Variable {
	return withVar(base, "topic_retention_ms", config.StringVariable("3600000"))
}

func (f *dataplaneFixture) topicConfigRestoreStep(base map[string]config.Variable) resource.TestStep {
	return f.emptyPlanStep(base,
		resource.TestCheckResourceAttr(acc.TopicResourceName, "configuration.cleanup.policy", "delete"),
		resource.TestCheckResourceAttr(acc.TopicResourceName, "configuration.retention.ms", "3600000"),
	)
}

// emptyPlanStep is a step that must leave no diff behind it.
func (f *dataplaneFixture) emptyPlanStep(vars map[string]config.Variable, checks ...resource.TestCheckFunc) resource.TestStep {
	step := f.step(vars, checks...)
	step.ConfigPlanChecks = resource.ConfigPlanChecks{
		PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
	}
	return step
}

// ImportSteps round-trips every remaining dataplane resource through import.
// clusterName is the cluster's current name at this point in the sequence.
func (f *dataplaneFixture) ImportSteps(vars map[string]config.Variable, clusterID clusterIDFunc) []resource.TestStep {
	var steps []resource.TestStep

	if f.SchemaRegistryACL {
		steps = append(steps, resource.TestStep{
			ResourceName:    acc.SchemaRegistryACLReadProductName,
			ConfigDirectory: config.StaticDirectory(f.dir),
			ConfigVariables: vars,
			ImportState:     true,
			ImportStateIdFunc: func(state *terraform.State) (string, error) {
				rs, ok := state.RootModule().Resources[acc.SchemaRegistryACLReadProductName]
				if !ok {
					return "", errors.New("schema registry ACL resource not found in state")
				}
				a := rs.Primary.Attributes
				base := fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s",
					a["cluster_id"], a["principal"], a["resource_type"], a["resource_name"],
					a["pattern_type"], a["host"], a["operation"], a["permission"])
				// Bearer-primary: 8-field form when the fixture didn't set
				// username/password. Basic auth appends ,username,password.
				if a["username"] == "" && a["password"] == "" {
					return base, nil
				}
				return fmt.Sprintf("%s,%s,%s", base, a["username"], a["password"]), nil
			},
			ImportStateCheck: func(state []*terraform.InstanceState) error {
				attr := state[0].Attributes
				for _, k := range []string{"cluster_id", "principal", "host", "id"} {
					if attr[k] == "" {
						return fmt.Errorf("expected non-empty %s", k)
					}
				}
				for k, want := range map[string]string{
					"resource_type": "SUBJECT",
					"resource_name": "product-",
					"pattern_type":  "PREFIXED",
					"operation":     "READ",
					"permission":    "ALLOW",
				} {
					if attr[k] != want {
						return fmt.Errorf("expected %s %q; got %q", k, want, attr[k])
					}
				}
				if allowDeletion := attr["allow_deletion"]; allowDeletion != acc.AllowDeletionFalseValue {
					return fmt.Errorf("expected allow_deletion to default to false; got %q", allowDeletion)
				}
				return nil
			},
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
		})
	}

	if f.Schema {
		name := f.name
		steps = append(steps, resource.TestStep{
			ResourceName:    acc.SchemaResourceName,
			ConfigDirectory: config.StaticDirectory(f.dir),
			ConfigVariables: vars,
			ImportState:     true,
			ImportStateIdFunc: func(state *terraform.State) (string, error) {
				rs, ok := state.RootModule().Resources[acc.SchemaResourceName]
				if !ok {
					return "", errors.New("schema resource not found in state")
				}
				a := rs.Primary.Attributes
				// Bearer-primary: 3-field form when the fixture didn't set
				// username/password. Basic auth appends ,username,password.
				if a["username"] == "" && a["password"] == "" {
					return fmt.Sprintf("%s,%s,%s", a["cluster_id"], a["subject"], a["version"]), nil
				}
				return fmt.Sprintf("%s,%s,%s,%s,%s",
					a["cluster_id"], a["subject"], a["version"], a["username"], a["password"]), nil
			},
			ImportStateCheck: func(state []*terraform.InstanceState) error {
				attr := state[0].Attributes
				if attr["subject"] != name+"-value" {
					return fmt.Errorf("expected subject %q; got %q", name+"-value", attr["subject"])
				}
				if attr["schema_type"] != "AVRO" {
					return fmt.Errorf("expected schema_type AVRO; got %q", attr["schema_type"])
				}
				for _, k := range []string{"version", "id", "cluster_id"} {
					if attr[k] == "" {
						return fmt.Errorf("expected non-empty %s", k)
					}
				}
				if allowDeletion := attr["allow_deletion"]; allowDeletion != acc.AllowDeletionFalseValue {
					return fmt.Errorf("expected allow_deletion to default to false; got %q", allowDeletion)
				}
				return nil
			},
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
		})
	}

	if f.Role {
		steps = append(steps, resource.TestStep{
			ResourceName:    acc.RoleResourceName,
			ConfigDirectory: config.StaticDirectory(f.dir),
			ConfigVariables: vars,
			ImportState:     true,
			ImportStateIdFunc: func(_ *terraform.State) (string, error) {
				id, err := clusterID()
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("developer,%v", id), nil
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

	if f.Topic {
		name := f.name
		steps = append(steps, resource.TestStep{
			ResourceName:    acc.TopicResourceName,
			ConfigDirectory: config.StaticDirectory(f.dir),
			ConfigVariables: vars,
			ImportState:     true,
			ImportStateIdFunc: func(_ *terraform.State) (string, error) {
				id, err := clusterID()
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("%s,%v", name, id), nil
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

	if f.Secret {
		steps = append(steps, resource.TestStep{
			ResourceName:    acc.SecretResourceName,
			ConfigDirectory: config.StaticDirectory(f.dir),
			ConfigVariables: vars,
			ImportState:     true,
			ImportStateIdFunc: func(state *terraform.State) (string, error) {
				rs, ok := state.RootModule().Resources[acc.SecretResourceName]
				if !ok {
					return "", errors.New("secret resource not found in state")
				}
				id, err := clusterID()
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("%s,%s", rs.Primary.Attributes["name"], id), nil
			},
			ImportStateCheck: func(state []*terraform.InstanceState) error {
				attr := state[0].Attributes
				if attr["id"] == "" {
					return errors.New("expected non-empty id")
				}
				if cloudURL := attr["cluster_api_url"]; cloudURL == "" {
					return errors.New("expected cluster_api_url to be set after import")
				}
				// secret_data is write-only; the backend masks it on read.
				if v := attr["secret_data"]; v != "" {
					return fmt.Errorf("expected secret_data to stay masked on import; got %q", v)
				}
				return nil
			},
			ImportStateVerifyIgnore:  []string{"secret_data", "secret_data_wo", "tags"},
			ProtoV6ProviderFactories: acc.ProtoV6Factories,
		})
	}

	return steps
}

// PasswordWoRotationStep rotates the write-only password and proves the new one
// actually reached the API, by authenticating to Schema Registry with it. A
// write-only value is absent from state, so asserting state alone would prove
// nothing about delivery.
//
// verify is injected rather than built here: it needs a control-plane client and
// the cluster ID, which differ per cluster shape.
func (f *dataplaneFixture) PasswordWoRotationStep(base map[string]config.Variable, verify func(password string) error) []resource.TestStep {
	if !f.PasswordWo {
		return nil
	}
	const rotated = "rotated-secret-v1"
	vars := withVar(base, "user_password_wo", config.StringVariable(rotated))
	vars = withVar(vars, "user_password_wo_version", config.IntegerVariable(1))

	step := f.step(vars,
		resource.TestCheckResourceAttr(acc.UserResourceName, "password_wo_version", "1"),
		func(_ *terraform.State) error { return verify(rotated) },
	)
	step.ConfigPlanChecks = resource.ConfigPlanChecks{
		PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
	}
	return []resource.TestStep{step}
}

// PipelineSteps import-round-trips the pipeline, starts it, then enables
// deletion so teardown can proceed (pipeline defaults to allow_deletion=false).
func (f *dataplaneFixture) PipelineSteps(base map[string]config.Variable, clusterID clusterIDFunc) []resource.TestStep {
	if !f.Pipeline {
		return nil
	}
	running := withVar(base, "pipeline_state", config.StringVariable("running"))
	return []resource.TestStep{
		{
			ResourceName:    acc.PipelineResourceName,
			ConfigDirectory: config.StaticDirectory(f.dir),
			ConfigVariables: base,
			ImportState:     true,
			ImportStateIdFunc: func(state *terraform.State) (string, error) {
				rs, ok := state.RootModule().Resources[acc.PipelineResourceName]
				if !ok {
					return "", errors.New("pipeline resource not found in state")
				}
				id, err := clusterID()
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("%s,%v", rs.Primary.Attributes["id"], id), nil
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
		f.step(running,
			resource.TestCheckResourceAttr(acc.PipelineResourceName, "state", "running")),
		// Chained off `running`, not `base`: the pipeline must stay started while
		// deletion is enabled, matching the pre-extraction sequence.
		f.step(withVar(running, "pipeline_allow_deletion", config.BoolVariable(true)),
			resource.TestCheckResourceAttr(acc.PipelineResourceName, "allow_deletion", "true")),
	}
}
