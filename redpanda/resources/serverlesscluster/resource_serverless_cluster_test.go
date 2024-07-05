package serverlesscluster

import (
	"fmt"
	"reflect"
	"testing"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
)

func TestGenerateServerlessClusterRequest(t *testing.T) {
	type args struct {
		model models.ServerlessCluster
	}
	tests := []struct {
		name string
		args args
		want *controlplanev1beta2.ServerlessClusterCreate
	}{
		{
			name: "validate_schema",
			args: args{
				model: models.ServerlessCluster{
					Name:             types.StringValue("testname"),
					ServerlessRegion: types.StringValue("pro-us-east-1"),
					ResourceGroupID:  types.StringValue("testrg"),
				},
			},
			want: &controlplanev1beta2.ServerlessClusterCreate{
				Name:             "testname",
				ServerlessRegion: "pro-us-east-1",
				ResourceGroupId:  "testrg",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := GenerateServerlessClusterRequest(tt.args.model); !reflect.DeepEqual(got, tt.want) {
				fmt.Println("got")
				spew.Dump(got)
				fmt.Println("want")
				spew.Dump(tt.want)
				t.Errorf("GenerateServerlessClusterRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}
