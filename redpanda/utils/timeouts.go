package utils

import (
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Timeouts is a struct that holds the timeout values for create, update, and delete operations.
type Timeouts struct {
	isCreateNil bool
	isUpdateNil bool
	isDeleteNil bool
	create      time.Duration
	update      time.Duration
	delete      time.Duration
	timeoutObj  types.Object
}

// matches timeout used by the API
const defaultTimeout = time.Minute * 180

// GetCreate returns the create timeout value.
func (t *Timeouts) GetCreate() time.Duration {
	if t == nil || t.isCreateNil {
		return defaultTimeout
	}
	return t.create
}

// GetUpdate returns the update timeout value.
func (t *Timeouts) GetUpdate() time.Duration {
	if t == nil || t.isUpdateNil {
		return defaultTimeout
	}
	return t.update
}

// GetDelete returns the delete timeout value.
func (t *Timeouts) GetDelete() time.Duration {
	if t == nil || t.isDeleteNil {
		return defaultTimeout
	}
	return t.delete
}

// GetModel returns the timeout object.
func (t *Timeouts) GetModel() types.Object {
	if t == nil {
		return types.ObjectNull(map[string]attr.Type{
			"create": types.StringType,
			"update": types.StringType,
			"delete": types.StringType,
		})
	}
	return t.timeoutObj
}

// GetNullModel returns a null timeout object.
func (*Timeouts) GetNullModel() types.Object {
	return types.ObjectNull(map[string]attr.Type{
		"create": types.StringType,
		"update": types.StringType,
		"delete": types.StringType,
	})
}

// GetTimeouts extracts the timeout values from the given object.
func GetTimeouts(ob types.Object) *Timeouts {
	timeoutsAttrTypes := map[string]attr.Type{
		"create": types.StringType,
		"update": types.StringType,
		"delete": types.StringType,
	}

	if ob.IsNull() {
		return &Timeouts{
			isCreateNil: true,
			isUpdateNil: true,
			isDeleteNil: true,
			timeoutObj:  types.ObjectNull(timeoutsAttrTypes),
		}
	}

	out := &Timeouts{}
	att := ob.Attributes()

	create, ok := att["create"].(types.String)
	if !ok || create.ValueString() == "" {
		out.isCreateNil = true
	} else {
		c, err := time.ParseDuration(create.ValueString())
		if err != nil {
			out.isCreateNil = true
		} else {
			out.create = c
		}
	}

	update, ok := att["update"].(types.String)
	if !ok || update.ValueString() == "" {
		out.isUpdateNil = true
	} else {
		u, err := time.ParseDuration(update.ValueString())
		if err != nil {
			out.isUpdateNil = true
		} else {
			out.update = u
		}
	}

	del, ok := att["delete"].(types.String)
	if !ok || del.ValueString() == "" {
		out.isDeleteNil = true
	} else {
		d, err := time.ParseDuration(del.ValueString())
		if err != nil {
			out.isDeleteNil = true
		} else {
			out.delete = d
		}
	}

	timeoutAttrs := map[string]attr.Value{
		"create": types.StringNull(),
		"update": types.StringNull(),
		"delete": types.StringNull(),
	}

	if !out.isCreateNil && out.create > 0 {
		timeoutAttrs["create"] = types.StringValue(create.ValueString())
	}
	if !out.isUpdateNil && out.update > 0 {
		timeoutAttrs["update"] = types.StringValue(update.ValueString())
	}
	if !out.isDeleteNil && out.delete > 0 {
		timeoutAttrs["delete"] = types.StringValue(del.ValueString())
	}

	newObj, diags := types.ObjectValue(timeoutsAttrTypes, timeoutAttrs)
	if diags.HasError() {
		out.timeoutObj = types.ObjectNull(timeoutsAttrTypes)
	} else {
		out.timeoutObj = newObj
	}
	return out
}
