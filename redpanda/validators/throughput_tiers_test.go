package validators

import (
	"context"
	"fmt"
	"testing"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/golang/mock/gomock"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/mocks"
)

func TestThroughputTierValidator_ValidateString(t *testing.T) {
	tests := []struct {
		name           string
		cloudProvider  string
		region         string
		throughputTier string
		mockSetup      func(m *mocks.MockThroughputTierClient)
		expectError    bool
		expectedErrMsg string
	}{
		{
			name:           "Valid AWS tier",
			cloudProvider:  "aws",
			region:         "us-east-1",
			throughputTier: "tier-1-aws-v2-arm",
			mockSetup: func(m *mocks.MockThroughputTierClient) {
				m.EXPECT().ListThroughputTiers(gomock.Any(), gomock.Any()).Return(&controlplanev1beta2.ListThroughputTiersResponse{
					ThroughputTiers: []*controlplanev1beta2.ThroughputTier{
						{Name: "tier-1-aws-v2-arm"},
					},
				}, nil)
			},
			expectError: false,
		},
		{
			name:           "Invalid AWS tier",
			cloudProvider:  "aws",
			region:         "us-east-1",
			throughputTier: "tier-1-aws-v2-x86",
			mockSetup: func(m *mocks.MockThroughputTierClient) {
				m.EXPECT().ListThroughputTiers(gomock.Any(), gomock.Any()).Return(&controlplanev1beta2.ListThroughputTiersResponse{
					ThroughputTiers: []*controlplanev1beta2.ThroughputTier{
						{Name: "tier-1-aws-v2-arm"},
					},
				}, nil)
			},
			expectError:    true,
			expectedErrMsg: "invalid throughput tier tier-1-aws-v2-x86, please select a valid throughput tier",
		},
		{
			name:           "Valid GCP tier",
			cloudProvider:  "gcp",
			region:         "us-central1",
			throughputTier: "tier-3-gcp-v2-x86",
			mockSetup: func(m *mocks.MockThroughputTierClient) {
				m.EXPECT().ListThroughputTiers(gomock.Any(), gomock.Any()).Return(&controlplanev1beta2.ListThroughputTiersResponse{
					ThroughputTiers: []*controlplanev1beta2.ThroughputTier{
						{Name: "tier-3-gcp-v2-x86"},
					},
				}, nil)
			},
			expectError: false,
		},
		{
			name:           "Invalid GCP tier",
			cloudProvider:  "gcp",
			region:         "us-central1",
			throughputTier: "tier-6-gcp-v2-x86",
			mockSetup: func(m *mocks.MockThroughputTierClient) {
				m.EXPECT().ListThroughputTiers(gomock.Any(), gomock.Any()).Return(&controlplanev1beta2.ListThroughputTiersResponse{
					ThroughputTiers: []*controlplanev1beta2.ThroughputTier{
						{Name: "tier-3-gcp-v2-x86"},
					},
				}, nil)
			},
			expectError:    true,
			expectedErrMsg: "invalid throughput tier tier-6-gcp-v2-x86, please select a valid throughput tier",
		},
		{
			name:           "Valid Azure tier",
			cloudProvider:  "azure",
			region:         "eastus",
			throughputTier: "tier-5-azure",
			mockSetup: func(m *mocks.MockThroughputTierClient) {
				m.EXPECT().ListThroughputTiers(gomock.Any(), gomock.Any()).Return(&controlplanev1beta2.ListThroughputTiersResponse{
					ThroughputTiers: []*controlplanev1beta2.ThroughputTier{
						{Name: "tier-5-azure"},
					},
				}, nil)
			},
			expectError: false,
		},
		{
			name:           "Invalid Azure tier",
			cloudProvider:  "azure",
			region:         "eastus",
			throughputTier: "tier-1-azure-beta",
			mockSetup: func(m *mocks.MockThroughputTierClient) {
				m.EXPECT().ListThroughputTiers(gomock.Any(), gomock.Any()).Return(&controlplanev1beta2.ListThroughputTiersResponse{
					ThroughputTiers: []*controlplanev1beta2.ThroughputTier{
						{Name: "tier-5-azure"},
					},
				}, nil)
			},
			expectError:    true,
			expectedErrMsg: "invalid throughput tier tier-1-azure-beta, please select a valid throughput tier",
		},
		{
			name:           "Invalid region",
			cloudProvider:  "aws",
			region:         "invalid-region",
			throughputTier: "tier-1-aws-v2-arm",
			mockSetup: func(m *mocks.MockThroughputTierClient) {
				m.EXPECT().ListThroughputTiers(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("invalid region"))
			},
			expectError:    true,
			expectedErrMsg: "failed to get throughput tiers from api: invalid region",
		},
		{
			name:           "API error",
			cloudProvider:  "aws",
			region:         "us-east-1",
			throughputTier: "tier-1-aws-v2-arm",
			mockSetup: func(m *mocks.MockThroughputTierClient) {
				m.EXPECT().ListThroughputTiers(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("API error"))
			},
			expectError:    true,
			expectedErrMsg: "failed to get throughput tiers from api: API error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewMockThroughputTierClient(ctrl)
			if tt.mockSetup != nil {
				tt.mockSetup(mockClient)
			}

			v := ThroughputTierValidator{
				ThroughputTierClient: mockClient,
				ClusterType:          "dedicated",
			}

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
							"throughput_tier": tftypes.String,
						},
					}, map[string]tftypes.Value{
						"cloud_provider":  tftypes.NewValue(tftypes.String, tt.cloudProvider),
						"region":          tftypes.NewValue(tftypes.String, tt.region),
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
						},
					},
				},
			}
			v.ValidateString(context.Background(), req, resp)

			if tt.expectError {
				if !resp.Diagnostics.HasError() {
					t.Errorf("expected error, but got none")
				} else {
					errMsg := resp.Diagnostics.Errors()[0].Detail()
					if errMsg != tt.expectedErrMsg {
						t.Errorf("expected error message '%s', but got '%s'", tt.expectedErrMsg, errMsg)
					}
				}
			} else if resp.Diagnostics.HasError() {
				t.Errorf("unexpected error: %v", resp.Diagnostics)
			}
		})
	}
}
