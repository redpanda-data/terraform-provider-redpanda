package network

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var cmrType = map[string]attr.Type{
	"aws": types.ObjectType{
		AttrTypes: awsType,
	},
	"gcp": types.ObjectType{
		AttrTypes: gcpType,
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

var gcpType = map[string]attr.Type{
	"network_name":       types.StringType,
	"network_project_id": types.StringType,
	"management_bucket": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"name": types.StringType,
		},
	},
}

var gcpBucketType = map[string]attr.Type{
	"name": types.StringType,
}

// Define null values for GCP fields
var gcpValueDefaults = map[string]attr.Value{
	"network_name":       types.StringNull(),
	"network_project_id": types.StringNull(),
	"management_bucket":  types.ObjectNull(gcpBucketType),
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
