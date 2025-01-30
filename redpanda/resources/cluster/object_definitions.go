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
			"permissions_boundary_policy": types.ObjectType{
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
	"permissions_boundary_policy": types.ObjectType{
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
	"redpanda_agent_security_group":          types.ObjectNull(singleElementContainer),
	"connectors_security_group":              types.ObjectNull(singleElementContainer),
	"redpanda_node_group_security_group":     types.ObjectNull(singleElementContainer),
	"utility_security_group":                 types.ObjectNull(singleElementContainer),
	"cluster_security_group":                 types.ObjectNull(singleElementContainer),
	"node_security_group":                    types.ObjectNull(singleElementContainer),
	"cloud_storage_bucket":                   types.ObjectNull(singleElementContainer),
	"permissions_boundary_policy":            types.ObjectNull(singleElementContainer),
}

var mtlsType = map[string]attr.Type{
	"enabled":                 types.BoolType,
	"ca_certificates_pem":     types.ListType{ElemType: types.StringType},
	"principal_mapping_rules": types.ListType{ElemType: types.StringType},
}

var kafkaAPIType = map[string]attr.Type{
	"seed_brokers": types.ListType{ElemType: types.StringType},
	"mtls":         types.ObjectType{AttrTypes: mtlsType},
}

var httpProxyType = map[string]attr.Type{
	"mtls": types.ObjectType{AttrTypes: mtlsType},
	"url":  types.StringType,
}

var redpandaConsole = map[string]attr.Type{
	"url": types.StringType,
}

var schemaRegistryType = map[string]attr.Type{
	"mtls": types.ObjectType{AttrTypes: mtlsType},
	"url":  types.StringType,
}

var prometheusType = map[string]attr.Type{
	"url": types.StringType,
}

var awsPrivateLinkType = map[string]attr.Type{
	"enabled":            types.BoolType,
	"connect_console":    types.BoolType,
	"allowed_principals": types.ListType{ElemType: types.StringType},
}

var gcpPrivateServiceConnectType = map[string]attr.Type{
	"enabled":               types.BoolType,
	"global_access_enabled": types.BoolType,
	"consumer_accept_list":  types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"source": types.StringType}}},
}

var azurePrivateLinkType = map[string]attr.Type{
	"enabled":               types.BoolType,
	"connect_console":       types.BoolType,
	"allowed_subscriptions": types.ListType{ElemType: types.StringType},
}

var dataplaneApi = map[string]attr.Type{
	"url": types.StringType,
}
