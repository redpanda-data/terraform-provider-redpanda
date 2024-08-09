package validators

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

func TestThroughputTierValidator_ValidateString(t *testing.T) {
	v := ThroughputTierValidator{}

	tests := []struct {
		name           string
		cloudProvider  string
		region         string
		zones          []string
		throughputTier string
		expectError    bool
	}{
		{
			name:           "Valid AWS tier",
			cloudProvider:  "aws",
			region:         "us-east-1",
			zones:          []string{"use1-az2"},
			throughputTier: "tier-1-aws-v2-arm",
			expectError:    false,
		},
		{
			name:           "Invalid AWS tier",
			cloudProvider:  "aws",
			region:         "us-east-1",
			zones:          []string{"use1-az2"},
			throughputTier: "tier-1-aws-v2-x86",
			expectError:    true,
		},
		{
			name:           "Valid GCP tier",
			cloudProvider:  "gcp",
			region:         "us-central1",
			zones:          []string{"us-central1-a"},
			throughputTier: "tier-3-gcp-v2-x86",
			expectError:    false,
		},
		{
			name:           "Invalid GCP tier",
			cloudProvider:  "gcp",
			region:         "us-central1",
			zones:          []string{"us-central1-a"},
			throughputTier: "tier-6-gcp-v2-x86",
			expectError:    true,
		},
		{
			name:           "Valid Azure tier",
			cloudProvider:  "azure",
			region:         "eastus",
			zones:          []string{"eastus-az1"},
			throughputTier: "tier-5-azure",
			expectError:    false,
		},
		{
			name:           "Invalid Azure tier",
			cloudProvider:  "azure",
			region:         "eastus",
			zones:          []string{"eastus-az1"},
			throughputTier: "tier-1-azure-beta",
			expectError:    true,
		},
		{
			name:           "Invalid cloud provider",
			cloudProvider:  "invalid",
			region:         "us-east-1",
			zones:          []string{"use1-az2"},
			throughputTier: "tier-1-aws-v2-arm",
			expectError:    true,
		},
		{
			name:           "Invalid region",
			cloudProvider:  "aws",
			region:         "invalid-region",
			zones:          []string{"use1-az2"},
			throughputTier: "tier-1-aws-v2-arm",
			expectError:    true,
		},
		{
			name:           "Invalid zone",
			cloudProvider:  "aws",
			region:         "us-east-1",
			zones:          []string{"invalid-zone"},
			throughputTier: "tier-1-aws-v2-arm",
			expectError:    true,
		},
		{
			name:           "Multiple valid zones",
			cloudProvider:  "aws",
			region:         "us-east-1",
			zones:          []string{"use1-az2", "use1-az4", "use1-az6"},
			throughputTier: "tier-1-aws-v2-arm",
			expectError:    false,
		},
		{
			name:           "Mix of valid and invalid zones",
			cloudProvider:  "aws",
			region:         "us-east-1",
			zones:          []string{"use1-az2", "invalid-zone"},
			throughputTier: "tier-1-aws-v2-arm",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &validator.StringResponse{}
			req := validator.StringRequest{
				Path:           path.Root("throughput_tier"),
				PathExpression: path.MatchRoot("throughput_tier"),
				ConfigValue:    types.StringValue(tt.throughputTier),
				Config: tfsdk.Config{
					Raw: tftypes.NewValue(tftypes.Object{
						AttributeTypes: map[string]tftypes.Type{
							"region":          tftypes.String,
							"cloud_provider":  tftypes.String,
							"zones":           tftypes.List{ElementType: tftypes.String},
							"throughput_tier": tftypes.String,
						},
					}, map[string]tftypes.Value{
						"cloud_provider":  tftypes.NewValue(tftypes.String, tt.cloudProvider),
						"region":          tftypes.NewValue(tftypes.String, tt.region),
						"zones":           tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, utils.StringSliceToSliceValues(tt.zones)),
						"throughput_tier": tftypes.NewValue(tftypes.String, tt.throughputTier),
					}),
					Schema: schema.Schema{
						Attributes: map[string]schema.Attribute{
							"throughput_tier": schema.StringAttribute{
								Required: true,
							},
							"cloud_provider": schema.StringAttribute{
								Optional: true,
							},
							"region": schema.StringAttribute{
								Optional: true,
							},
							"zones": schema.ListAttribute{
								Optional:    true,
								ElementType: types.StringType,
							},
						},
					},
				},
			}
			v.ValidateString(context.Background(), req, resp)

			if tt.expectError && !resp.Diagnostics.HasError() {
				t.Errorf("expected error, but got none")
			}
			if !tt.expectError && resp.Diagnostics.HasError() {
				t.Errorf("unexpected error: %v", resp.Diagnostics)
			}
		})
	}
}
