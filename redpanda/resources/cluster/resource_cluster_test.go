package cluster

import (
	"fmt"
	"reflect"
	"testing"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

func TestGenerateClusterRequest(t *testing.T) {
	type args struct {
		model models.Cluster
	}
	rpVersion := "v24.1.1"
	tests := []struct {
		name string
		args args
		want *controlplanev1beta2.ClusterCreate
	}{
		{
			name: "validate_schema",
			args: args{
				model: models.Cluster{
					Name:            types.StringValue("testname"),
					RedpandaVersion: types.StringValue(rpVersion),
					ConnectionType:  types.StringValue("public"),
					CloudProvider:   types.StringValue("gcp"),
					ClusterType:     types.StringValue("dedicated"),
					ThroughputTier:  types.StringValue("tier-2-gcp-um50"),
					Region:          types.StringValue("us-west1"),
					Zones:           utils.TestingOnlyStringSliceToTypeList([]string{"us-west1-a", "us-west1-b"}),
					AllowDeletion:   types.BoolValue(true),
					ResourceGroupID: types.StringValue("testrg"),
					NetworkID:       types.StringValue("testnetwork"),
				},
			},
			want: &controlplanev1beta2.ClusterCreate{
				Name:            "testname",
				RedpandaVersion: &rpVersion,
				ConnectionType:  controlplanev1beta2.Cluster_CONNECTION_TYPE_PUBLIC,
				CloudProvider:   controlplanev1beta2.CloudProvider_CLOUD_PROVIDER_GCP,
				Type:            controlplanev1beta2.Cluster_TYPE_DEDICATED,
				ThroughputTier:  "tier-2-gcp-um50",
				Region:          "us-west1",
				Zones:           []string{"us-west1-a", "us-west1-b"},
				ResourceGroupId: "testrg",
				NetworkId:       "testnetwork",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := GenerateClusterRequest(tt.args.model); !reflect.DeepEqual(got, tt.want) {
				fmt.Println("got")
				spew.Dump(got)
				fmt.Println("want")
				spew.Dump(tt.want)
				t.Errorf("GenerateClusterRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}
