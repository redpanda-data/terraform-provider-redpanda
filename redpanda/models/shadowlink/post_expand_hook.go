// Copyright 2025 Redpanda Data, Inc.
//
//	Licensed under the Apache License, Version 2.0 (the "License");
//	you may not use this file except in compliance with the License.
//	You may obtain a copy of the License at
//
//	  http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS,
//	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//	See the License for the specific language governing permissions and
//	limitations under the License.

package shadowlink

import (
	"context"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// ThreadCreateExtras lifts TF-only fields onto the CreateShadowLinkRequest
// payload after ExpandCreate has run. Plugged into the schemagen
// post_expand_hook so the same code path runs in both
// resource_shadowlink.go::Create() and the generated proto-validator —
// the validator's payload then matches what the API actually receives,
// no spurious "either source_redpanda_id or bootstrap_servers" diagnostic
// at plan time.
//
// source_redpanda_id is `extra: true` in schema.yaml because the proto
// only has it on the Create payload (not on the read ShadowLink). The
// generator omits it from ExpandCreate; this hook threads it through.
func ThreadCreateExtras(_ context.Context, m *ResourceModel, req *controlplanev1.CreateShadowLinkRequest) diag.Diagnostics {
	var diags diag.Diagnostics
	if !m.SourceRedpandaID.IsNull() && !m.SourceRedpandaID.IsUnknown() {
		req.GetShadowLink().SetSourceRedpandaId(m.SourceRedpandaID.ValueString())
	}
	return diags
}
