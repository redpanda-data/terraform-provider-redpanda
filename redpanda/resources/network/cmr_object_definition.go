package network

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var cmrType = map[string]attr.Type{
	"aws": types.ObjectType{
		AttrTypes: map[string]attr.Type{
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
		},
	},
}

var awsType = map[string]attr.Type{
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

var singleElementContainer = map[string]attr.Type{
	"arn": types.StringType,
}

var multiElementContainer = map[string]attr.Type{
	"arns": types.ListType{
		ElemType: types.StringType,
	},
}

var awsValueDefaults = map[string]attr.Value{
	"management_bucket": types.ObjectNull(singleElementContainer),
	"dynamodb_table":    types.ObjectNull(singleElementContainer),
	"vpc":               types.ObjectNull(singleElementContainer),
	"private_subnets":   types.ObjectNull(multiElementContainer),
}
