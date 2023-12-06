package clients

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"os"
	"reflect"
	"testing"
)

func TestNewCloudV2Client(t *testing.T) {
	type args struct {
		ctx     context.Context
		version string
		model   models.Redpanda
	}
	tests := []struct {
		name string
		args args
		want CloudV2
		err  bool
	}{
		{
			name: "test_success",
			args: args{
				ctx:     context.Background(),
				version: "dev",
				model: models.Redpanda{
					ClientID:     types.StringValue(os.Getenv("CLIENT_ID")),
					ClientSecret: types.StringValue(os.Getenv("CLIENT_SECRET")),
				},
			},
			want: CloudV2{},
			err:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewCloudV2Client(tt.args.ctx, tt.args.version, tt.args.model)
			if (err != nil) != tt.err {
				t.Errorf("NewCloudV2Client() error = %v, errExpected %v", err, tt.err)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewCloudV2Client() = %v, want %v", got, tt.want)
			}
		})
	}
}
