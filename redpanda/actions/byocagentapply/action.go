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

// Package byocagentapply implements the redpanda_byoc_agent_apply action,
// the Terraform equivalent of `rpk cloud byoc <cloud> apply` on an existing
// cluster.
package byocagentapply

import (
	"context"
	"fmt"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils/enums"
)

const defaultTimeout = 45 * time.Minute

var (
	_ action.Action                   = &Action{}
	_ action.ActionWithConfigure      = &Action{}
	_ action.ActionWithModifyPlan     = &Action{}
	_ action.ActionWithValidateConfig = &Action{}
)

// Action runs the rpk byoc plugin's apply against a READY BYOC cluster.
type Action struct {
	cpCl *cloud.ControlPlaneClientSet
	byoc utils.ByocRunner
}

// Model is the action's config.
type Model struct {
	ClusterID types.String `tfsdk:"cluster_id"`
	Timeout   types.String `tfsdk:"timeout"`
}

// New constructs the action.
func New() action.Action { return &Action{} }

// Metadata implements action.Action.
func (*Action) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_byoc_agent_apply"
}

// Schema implements action.Action.
func (*Action) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Re-runs the Redpanda BYOC agent apply (`rpk cloud byoc <cloud> apply --redpanda-id=<id>`) " +
			"against an existing BYOC or BYOVPC cluster, reconciling the agent VM and its network without changing the cluster. " +
			"The cluster must be `STATE_READY`. Runs on the machine executing Terraform and needs that cloud's credentials " +
			"the same way cluster creation does. Requires Terraform 1.14 or later. " +
			"Invoke it ad hoc with `terraform apply -invoke=action.redpanda_byoc_agent_apply.<name>`, " +
			"or trigger it from a resource lifecycle with `action_trigger`.",
		Attributes: map[string]schema.Attribute{
			"cluster_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the BYOC cluster whose agent to apply.",
			},
			"timeout": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "How long to wait for the plugin, as a duration such as `45m`. Defaults to 45m.",
			},
		},
	}
}

// Configure implements action.ActionWithConfigure.
func (a *Action) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	p, ok := req.ProviderData.(config.Resource)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Action Configure Type",
			fmt.Sprintf("Expected config.Resource, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	a.cpCl = cloud.NewControlPlaneClientSet(p.ControlPlaneConnection)
	a.byoc = p.ByocClient
}

// ValidateConfig implements action.ActionWithValidateConfig.
func (*Action) ValidateConfig(ctx context.Context, req action.ValidateConfigRequest, resp *action.ValidateConfigResponse) {
	var m Model
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := parseTimeout(m.Timeout); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("timeout"), "Invalid timeout", err.Error())
	}
}

// ModifyPlan implements action.ActionWithModifyPlan. It runs the preflight
// at plan time when cluster_id is already known, so a cluster that cannot
// take an agent apply fails before the plugin is downloaded.
func (a *Action) ModifyPlan(ctx context.Context, req action.ModifyPlanRequest, resp *action.ModifyPlanResponse) {
	var m Model
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || m.ClusterID.IsUnknown() || m.ClusterID.IsNull() || a.cpCl == nil {
		return
	}
	_, diags := a.preflight(ctx, m.ClusterID.ValueString())
	resp.Diagnostics.Append(diags...)
}

// Invoke implements action.Action.
func (a *Action) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var m Model
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	timeout, err := parseTimeout(m.Timeout)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("timeout"), "Invalid timeout", err.Error())
		return
	}
	clusterID := m.ClusterID.ValueString()
	cl, diags := a.preflight(ctx, clusterID)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The deadline bounds the plugin run itself, not only Retry's window:
	// Retry checks the clock only between attempts, and RunByoc blocks for
	// as long as the subprocess does.
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	sink := func(line string) {
		resp.SendProgress(action.InvokeProgressEvent{Message: line})
	}
	tflog.Info(ctx, "running byoc agent apply", map[string]any{"cluster_id": clusterID, "cloud_provider": enums.CloudProviderToString(cl.GetCloudProvider())})
	resp.SendProgress(action.InvokeProgressEvent{Message: fmt.Sprintf("running byoc agent apply for cluster %s", clusterID)})
	err = utils.Retry(ctx, timeout, func() *utils.RetryError {
		runErr := a.byoc.RunByoc(ctx, clusterID, "apply", sink)
		if runErr == nil {
			return nil
		}
		if utils.IsRetryableByocError(runErr) {
			tflog.Debug(ctx, fmt.Sprintf("retryable byoc error during agent apply: %v", runErr))
			resp.SendProgress(action.InvokeProgressEvent{Message: "byoc plugin asked for a retry; retrying"})
			return utils.RetryableError(runErr)
		}
		return utils.NonRetryableError(runErr)
	})
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("byoc agent apply failed for cluster %s", clusterID), err.Error())
		return
	}
	resp.SendProgress(action.InvokeProgressEvent{Message: fmt.Sprintf("byoc agent apply completed for cluster %s", clusterID)})
}

// preflight reads the cluster and refuses anything the plugin run cannot
// help: a missing cluster, a non-BYOC cluster, or a cluster that is not
// READY. FAILED is called out separately because the control plane treats it
// as terminal for the create workflow, so a healthy agent would not move it.
func (a *Action) preflight(ctx context.Context, clusterID string) (*controlplanev1.Cluster, diag.Diagnostics) {
	var diags diag.Diagnostics
	cl, err := a.cpCl.ClusterForID(ctx, clusterID)
	if err != nil {
		if utils.IsNotFound(err) {
			diags.AddAttributeError(path.Root("cluster_id"), "Cluster not found", fmt.Sprintf("cluster %q does not exist", clusterID))
			return nil, diags
		}
		diags.AddError(fmt.Sprintf("failed to read cluster %s", clusterID), utils.DeserializeGrpcError(err))
		return nil, diags
	}
	if cl.GetType() != controlplanev1.Cluster_TYPE_BYOC {
		diags.AddAttributeError(path.Root("cluster_id"), "Cluster is not a BYOC cluster",
			fmt.Sprintf("cluster %q is not a BYOC cluster (type %s); only BYOC and BYOVPC clusters have an agent to apply", clusterID, enums.ClusterTypeToString(cl.GetType())))
		return nil, diags
	}
	switch cl.GetState() {
	case controlplanev1.Cluster_STATE_READY:
	case controlplanev1.Cluster_STATE_FAILED:
		detail := fmt.Sprintf("cluster %q is in state STATE_FAILED", clusterID)
		if desc := cl.GetStateDescription().GetMessage(); desc != "" {
			detail += ": " + desc
		}
		diags.AddAttributeError(path.Root("cluster_id"), "Cluster has failed",
			detail+". The control plane does not resume a failed cluster after an agent apply; replace the cluster or contact Redpanda support.")
		return nil, diags
	default:
		diags.AddAttributeError(path.Root("cluster_id"), "Cluster is not ready",
			fmt.Sprintf("cluster %q is in state %s; the agent can only be applied on a STATE_READY cluster", clusterID, cl.GetState()))
		return nil, diags
	}
	if checker, ok := a.byoc.(utils.ByocCloudConfigChecker); ok {
		if err := checker.CheckCloudConfig(enums.CloudProviderToString(cl.GetCloudProvider())); err != nil {
			diags.AddError("Provider is missing the cloud configuration the byoc plugin needs", err.Error())
			return nil, diags
		}
	}
	return cl, diags
}

func parseTimeout(v types.String) (time.Duration, error) {
	if v.IsNull() || v.IsUnknown() {
		return defaultTimeout, nil
	}
	d, err := time.ParseDuration(v.ValueString())
	if err != nil {
		return 0, fmt.Errorf("timeout %q is not a duration such as \"45m\": %w", v.ValueString(), err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("timeout %q must be positive", v.ValueString())
	}
	return d, nil
}
