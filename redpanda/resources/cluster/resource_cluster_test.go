package cluster

import (
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/terraform-plugin-framework/types"
	cloudv1beta1 "github.com/redpanda-data/terraform-provider-redpanda/proto/gen/go/redpanda/api/controlplane/v1beta1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"reflect"
	"testing"
)

func TestGenerateClusterRequest(t *testing.T) {
	type args struct {
		model models.Cluster
	}
	tests := []struct {
		name string
		args args
		want *cloudv1beta1.Cluster
	}{
		{
			name: "validate_schema",
			args: args{
				model: models.Cluster{
					Name:           types.StringValue("testname"),
					ConnectionType: types.StringValue("public"),
					CloudProvider:  types.StringValue("gcp"),
					ClusterType:    types.StringValue("dedicated"),
					ThroughputTier: types.StringValue("tier-2-gcp-um50"),
					Region:         types.StringValue("us-west1"),
					Zones:          utils.TestingOnlyStringSliceToTypeList([]string{"us-west1-a", "us-west1-b"}),
					AllowDeletion:  types.BoolValue(true),
					NamespaceId:    types.StringValue("testnamespace"),
					NetworkId:      types.StringValue("testnetwork"),
				},
			},
			want: &cloudv1beta1.Cluster{
				Name:           "testname",
				ConnectionType: cloudv1beta1.Cluster_CONNECTION_TYPE_PUBLIC,
				CloudProvider:  cloudv1beta1.CloudProvider_CLOUD_PROVIDER_GCP,
				Type:           cloudv1beta1.Cluster_TYPE_DEDICATED,
				ThroughputTier: "tier-2-gcp-um50",
				Region:         "us-west1",
				Zones:          []string{"us-west1-a", "us-west1-b"},
				NamespaceId:    "testnamespace",
				NetworkId:      "testnetwork",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateClusterRequest(tt.args.model); !reflect.DeepEqual(got, tt.want) {
				fmt.Println("got")
				spew.Dump(got)
				fmt.Println("want")
				spew.Dump(tt.want)
				t.Errorf("GenerateClusterRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}
