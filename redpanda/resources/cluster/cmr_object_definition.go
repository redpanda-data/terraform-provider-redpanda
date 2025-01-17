package cluster

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var cmrType = map[string]attr.Type{
	"aws": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"agent_instance_profile": types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"arn": types.StringType,
				},
			},
			"connectors_node_group_instance_profile": types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"arn": types.StringType,
				},
			},
			"utility_node_group_instance_profile": types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"arn": types.StringType,
				},
			},
			"redpanda_node_group_instance_profile": types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"arn": types.StringType,
				},
			},
			"k8s_cluster_role": types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"arn": types.StringType,
				},
			},
			"console_secrets_manager_role": types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"arn": types.StringType,
				},
			},
			"redpanda_cloud_storage_manager_role": types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"arn": types.StringType,
				},
			},
			"connectors_secrets_manager_role": types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"arn": types.StringType,
				},
			},
			"redpanda_agent_security_group": types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"arn": types.StringType,
				},
			},
			"connectors_security_group": types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"arn": types.StringType,
				},
			},
			"redpanda_node_group_security_group": types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"arn": types.StringType,
				},
			},
			"utility_security_group": types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"arn": types.StringType,
				},
			},
			"cluster_security_group": types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"arn": types.StringType,
				},
			},
			"node_security_group": types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"arn": types.StringType,
				},
			},
			"cloud_storage_bucket": types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"arn": types.StringType,
				},
			},
		},
	},
}

var crmVal = map[string]attr.Value{
	"aws": types.ObjectNull(awsType),
}

var awsType = map[string]attr.Type{
	"agent_instance_profile": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"connectors_node_group_instance_profile": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"utility_node_group_instance_profile": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"redpanda_node_group_instance_profile": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"k8s_cluster_role": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"console_secrets_manager_role": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"redpanda_cloud_storage_manager_role": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"connectors_secrets_manager_role": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"redpanda_agent_security_group": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"connectors_security_group": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"redpanda_node_group_security_group": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"utility_security_group": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"cluster_security_group": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"node_security_group": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
	"cloud_storage_bucket": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"arn": types.StringType,
		},
	},
}

var singleElementContainer = map[string]attr.Type{
	"arn": types.StringType,
}

var awsValue = map[string]attr.Value{
	"agent_instance_profile":                 types.ObjectNull(singleElementContainer),
	"connectors_node_group_instance_profile": types.ObjectNull(singleElementContainer),
	"utility_node_group_instance_profile":    types.ObjectNull(singleElementContainer),
	"redpanda_node_group_instance_profile":   types.ObjectNull(singleElementContainer),
	"k8s_cluster_role":                       types.ObjectNull(singleElementContainer),
	"console_secrets_manager_role":           types.ObjectNull(singleElementContainer),
	"redpanda_cloud_storage_manager_role":    types.ObjectNull(singleElementContainer),
	"connectors_secrets_manager_role":        types.ObjectNull(singleElementContainer),
	"redpanda_agent_security_group":          types.ObjectNull(singleElementContainer),
	"connectors_security_group":              types.ObjectNull(singleElementContainer),
	"redpanda_node_group_security_group":     types.ObjectNull(singleElementContainer),
	"utility_security_group":                 types.ObjectNull(singleElementContainer),
	"cluster_security_group":                 types.ObjectNull(singleElementContainer),
	"node_security_group":                    types.ObjectNull(singleElementContainer),
	"cloud_storage_bucket":                   types.ObjectNull(singleElementContainer),
}
