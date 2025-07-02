package network

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func getCMRType() map[string]attr.Type {
	return map[string]attr.Type{
		"aws": types.ObjectType{
			AttrTypes: getAwsType(),
		},
		"gcp": types.ObjectType{
			AttrTypes: getGcpType(),
		},
	}
}

func getCMRNull() types.Object {
	return types.ObjectNull(getCMRType())
}

func getGcpNull() types.Object {
	return types.ObjectNull(getGcpType())
}

func getGcpType() map[string]attr.Type {
	return map[string]attr.Type{
		"network_name":       types.StringType,
		"network_project_id": types.StringType,
		"management_bucket": types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"name": types.StringType,
			},
		},
	}
}

func getAwsNull() types.Object {
	return types.ObjectNull(getAwsType())
}

func getAwsType() map[string]attr.Type {
	return map[string]attr.Type{
		"management_bucket": types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"arn": types.StringType,
			},
		},
		"dynamodb_table": types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"arn": types.StringType,
			},
		},
		"vpc": types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"arn": types.StringType,
			},
		},
		"private_subnets": types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"arns": types.ListType{
					ElemType: types.StringType,
				},
			},
		},
	}
}

func getAwsNullAttributes() map[string]attr.Value {
	return map[string]attr.Value{
		"management_bucket": types.ObjectNull(map[string]attr.Type{
			"arn": types.StringType,
		}),
		"dynamodb_table": types.ObjectNull(map[string]attr.Type{
			"arn": types.StringType,
		}),
		"vpc": types.ObjectNull(map[string]attr.Type{
			"arn": types.StringType,
		}),
		"private_subnets": types.ObjectNull(map[string]attr.Type{
			"arns": types.ListType{
				ElemType: types.StringType,
			},
		}),
	}
}

func getGcpNullAttributes() map[string]attr.Value {
	return map[string]attr.Value{
		"network_name":       types.StringNull(),
		"network_project_id": types.StringNull(),
		"management_bucket": types.ObjectNull(map[string]attr.Type{
			"name": types.StringType,
		}),
	}
}
