package serverlesscluster

import (
	"fmt"
	"reflect"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/terraform-plugin-framework/types"
	serverlessclustermodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/serverlesscluster"
)

func TestGenerateServerlessClusterRequest(t *testing.T) {
	type args struct {
		model serverlessclustermodel.ResourceModel
	}
	tests := []struct {
		name string
		args args
		want *controlplanev1.ServerlessClusterCreate
	}{
		{
			name: "validate_schema",
			args: args{
				model: serverlessclustermodel.ResourceModel{
					Name:             types.StringValue("testname"),
					ServerlessRegion: types.StringValue("pro-us-east-1"),
					ResourceGroupID:  types.StringValue("testrg"),
				},
			},
			want: &controlplanev1.ServerlessClusterCreate{
				Name:             "testname",
				ServerlessRegion: "pro-us-east-1",
				ResourceGroupId:  "testrg",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := generateServerlessClusterRequest(tt.args.model); !reflect.DeepEqual(got, tt.want) {
				fmt.Println("got")
				spew.Dump(got)
				fmt.Println("want")
				spew.Dump(tt.want)
				t.Errorf("generateServerlessClusterRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}
