// Copyright 2023 Redpanda Data, Inc.
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package schemagen

import (
	"fmt"

	"github.com/redpanda-data/terraform-provider-redpanda/internal/fileutil"
	"gopkg.in/yaml.v3"
)

// Config is the top-level YAML configuration for generating a schema.
type Config struct {
	Message string `yaml:"message"`

	APISchema string `yaml:"api_schema,omitempty"`

	StripOpenAPIPrefix string `yaml:"strip_openapi_prefix,omitempty"`

	Timeouts []string `yaml:"timeouts,omitempty"`

	ComputedDefault bool `yaml:"computed_default,omitempty"`

	Version int64 `yaml:"version,omitempty"`

	TFName string `yaml:"tf_name,omitempty"`

	Fields map[string]FieldConfig `yaml:"fields,omitempty"`

	API *APIConfig `yaml:"api,omitempty"`

	ExcludeOperations []string `yaml:"exclude_operations,omitempty"`

	sourcePath string

	maskContract *MaskContract
}

// Source returns the path this config was loaded from, or "" if hand-built.
func (c *Config) Source() string {
	if c == nil {
		return ""
	}
	return c.sourcePath
}

// MaskContract mirrors the control plane's update-mask path map for one
// resource: TopLevel names are accepted at object granularity, Leaf names
// only at leaf granularity (the provider expands them). A top-level field in
// neither set cannot be updated in place — it requires replace.
type MaskContract struct {
	TopLevel map[string]bool
	Leaf     map[string]bool
}

// SetMaskContract attaches the resolved update-mask contract; cmd/schemagen
// resolves the yaml `api.update.mask_contract` name against its registry.
func (c *Config) SetMaskContract(mc *MaskContract) { c.maskContract = mc }

// MaskContract returns the attached contract, or nil when the resource
// declared none (derivation is then a no-op).
func (c *Config) MaskContract() *MaskContract {
	if c == nil {
		return nil
	}
	return c.maskContract
}

// APIConfig declares the resource's proto RPC shapes for the
// flatten/expand emission path.
type APIConfig struct {
	Create *RPCConfig `yaml:"create,omitempty"`
	Update *RPCConfig `yaml:"update,omitempty"`
	Delete *RPCConfig `yaml:"delete,omitempty"`

	ResponseInterface *ResponseInterfaceConfig `yaml:"response_interface,omitempty"`

	ResourceType string `yaml:"resource_type,omitempty"`

	PreValidateHook string `yaml:"pre_validate_hook,omitempty"`

	PostExpandHook string `yaml:"post_expand_hook,omitempty"`
}

// RPCConfig describes a single create/update/delete RPC's request shape.
type RPCConfig struct {
	RPC string `yaml:"rpc,omitempty"`

	Request string `yaml:"request,omitempty"`

	PayloadField string `yaml:"payload_field,omitempty"`

	PayloadType string `yaml:"payload_type,omitempty"`

	FlatFieldMap map[string]string `yaml:"flat_field_map,omitempty"`

	ReturnPayload bool `yaml:"return_payload,omitempty"`

	// DiffMask, when set on an update RPC, emits an ExpandUpdateWithMask helper
	// alongside ExpandUpdate. "sparse" returns the diff payload via
	// utils.GenerateProtobufDiffAndUpdateMask (server tolerates a partial
	// message); "full" returns the whole plan payload via
	// utils.PlanPayloadWithUpdateMask (server runs buf.validate on every field).
	// In both cases the returned mask has empty Paths when nothing changed, so
	// callers can skip the update RPC entirely.
	DiffMask string `yaml:"diff_mask,omitempty"`

	// MaskContract names the control-plane update-mask contract registered in
	// cmd/schemagen. Top-level fields absent from the contract derive
	// RequiresReplace; yaml overrides that disagree are warned about.
	MaskContract string `yaml:"mask_contract,omitempty"`
}

// ResponseInterfaceConfig declares a Go interface synthesized over the
// API's response payload types so Flatten can take a single argument.
// The interface is duck-typed — methods are inferred from the schema's
// field conversions, and any proto type with matching getter shapes
// satisfies it. Only the interface name is configurable here.
type ResponseInterfaceConfig struct {
	Name string `yaml:"name,omitempty"`
}

// FieldConfig holds the override configuration for a single field.
type FieldConfig struct {
	Required     bool `yaml:"required,omitempty"`
	ComputedOnly bool `yaml:"computed_only,omitempty"`

	Optional *bool `yaml:"optional,omitempty"`
	Computed *bool `yaml:"computed,omitempty"`

	Validator any `yaml:"validator,omitempty"`
	Default   any `yaml:"default,omitempty"`

	MinimalDefault any      `yaml:"minimal_default,omitempty"`
	PlanModifiers  []string `yaml:"plan_modifiers,omitempty"`

	Rename      string `yaml:"rename,omitempty"`
	Exclude     bool   `yaml:"exclude,omitempty"`
	Todo        bool   `yaml:"todo,omitempty"`
	Deprecated  bool   `yaml:"deprecated,omitempty"`
	Synthetic   bool   `yaml:"synthetic,omitempty"`
	ForceType   string `yaml:"force_type,omitempty"`
	ElementType string `yaml:"element_type,omitempty"`

	Extra bool   `yaml:"extra,omitempty"`
	Type  string `yaml:"type,omitempty"`

	Sensitive          bool   `yaml:"sensitive,omitempty"`
	WriteOnly          bool   `yaml:"write_only,omitempty"`
	DeprecationMessage string `yaml:"deprecation_message,omitempty"`

	SkipProtoValidation bool `yaml:"skip_proto_validation,omitempty"`

	ExpandVia string `yaml:"expand_via,omitempty"`

	FlattenVia string `yaml:"flatten_via,omitempty"`

	FromProto string `yaml:"from_proto,omitempty"`

	FlattenSkip bool `yaml:"flatten_skip,omitempty"`

	ExpandSkip bool `yaml:"expand_skip,omitempty"`

	HasPresence bool `yaml:"has_presence,omitempty"`

	FlattenFromPrev bool `yaml:"flatten_from_prev,omitempty"`

	ProtoOnly bool `yaml:"proto_only,omitempty"`

	Fields map[string]FieldConfig `yaml:"fields,omitempty"`
}

// ValidatorNames returns the validator names as a string slice, handling both
// single string and list forms in YAML. LoadConfig is expected to have
// already rejected any other shape via validateValidatorTypes; the panic
// below guards against constructing a FieldConfig directly with a malformed
// Validator (e.g. in tests) without surfacing the bug loudly.
func (fc FieldConfig) ValidatorNames() []string {
	if fc.Validator == nil {
		return nil
	}
	switch v := fc.Validator.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case []any:
		names := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				names = append(names, s)
			}
		}
		return names
	default:
		panic(fmt.Sprintf("ValidatorNames: unvalidated Validator type %T — LoadConfig should have rejected this", fc.Validator))
	}
}

// LoadConfig reads and parses a YAML schema config file.
func LoadConfig(path string) (*Config, error) {
	data, err := fileutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %s: %w", path, err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse config %s: %w", path, err)
	}

	cfg := &Config{
		Fields:     make(map[string]FieldConfig),
		sourcePath: path,
	}

	assignScalar(&cfg.Message, raw, "message")
	appendStringList(&cfg.Timeouts, raw, "timeouts")
	assignScalar(&cfg.ComputedDefault, raw, "computed_default")
	assignScalar(&cfg.APISchema, raw, "api_schema")
	assignScalar(&cfg.TFName, raw, "tf_name")
	assignScalar(&cfg.StripOpenAPIPrefix, raw, "strip_openapi_prefix")
	appendStringList(&cfg.ExcludeOperations, raw, "exclude_operations")

	if v, ok := raw["version"]; ok {
		switch n := v.(type) {
		case int:
			cfg.Version = int64(n)
		case int64:
			cfg.Version = n
		}
	}

	if v, ok := raw["api"]; ok {
		apiData, err := yaml.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal api block in %s: %w", path, err)
		}
		var api APIConfig
		if err := yaml.Unmarshal(apiData, &api); err != nil {
			return nil, fmt.Errorf("failed to parse api block in %s: %w", path, err)
		}
		cfg.API = &api
	}

	reservedKeys := map[string]bool{
		"message": true, "timeouts": true,
		"computed_default": true, "api_schema": true, "tf_name": true,
		"strip_openapi_prefix": true, "version": true,
		"api": true, "exclude_operations": true,
	}
	for key, val := range raw {
		if reservedKeys[key] {
			continue
		}
		if err := rejectDescriptionOverrides(key, val); err != nil {
			return nil, fmt.Errorf("config %s: %w", path, err)
		}
		fc, err := parseFieldConfig(val)
		if err != nil {
			return nil, fmt.Errorf("failed to parse field %s: %w", key, err)
		}
		cfg.Fields[key] = fc
	}

	if err := validateValidatorTypes(cfg.Fields, ""); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}

	return cfg, nil
}

// assignScalar sets *dst to raw[key] when that key is present and holds a value
// of type T; otherwise *dst is left unchanged.
func assignScalar[T any](dst *T, raw map[string]any, key string) {
	if v, ok := raw[key]; ok {
		if t, ok := v.(T); ok {
			*dst = t
		}
	}
}

// appendStringList appends the string elements of raw[key] (a YAML sequence) to
// *dst, skipping non-string items.
func appendStringList(dst *[]string, raw map[string]any, key string) {
	v, ok := raw[key]
	if !ok {
		return
	}
	list, ok := v.([]any)
	if !ok {
		return
	}
	for _, item := range list {
		if s, ok := item.(string); ok {
			*dst = append(*dst, s)
		}
	}
}

// validateValidatorTypes walks the field tree and rejects any `validator:`
// value that isn't a string, []string, or a list whose items are all
// non-empty strings. Catches silent drops at config-load time so a typoed
// validator never disappears into a downstream nil.
func validateValidatorTypes(fields map[string]FieldConfig, parent string) error {
	for name := range fields {
		fc := fields[name]
		path := joinPath(parent, name)
		if fc.Validator != nil {
			switch v := fc.Validator.(type) {
			case string:
			case []any:
				for i, item := range v {
					if _, ok := item.(string); !ok {
						return fmt.Errorf("field %q: validator[%d] must be string, got %T", path, i, item)
					}
				}
			default:
				return fmt.Errorf("field %q: validator must be string or []string, got %T", path, fc.Validator)
			}
		}
		if len(fc.Fields) > 0 {
			if err := validateValidatorTypes(fc.Fields, path); err != nil {
				return err
			}
		}
	}
	return nil
}

// SupportedOperations resolves the effective CRUD ops for the config given
// its schema type. Defaults: [create, read, update, delete] for resources,
// [read] for datasources. ExcludeOperations is subtracted, with validation:
// `read` cannot be excluded; datasources reject any non-`read` exclusion;
// every api.<op> block must not be in the exclusion list.
func (c *Config) SupportedOperations(schemaType string) (map[string]bool, error) {
	defaults := []string{"create", "read", "update", "delete"}
	if schemaType == SchemaTypeDatasource {
		defaults = []string{"read"}
	}
	supported := make(map[string]bool, len(defaults))
	for _, op := range defaults {
		supported[op] = true
	}
	excluded := make(map[string]bool, len(c.ExcludeOperations))
	for _, op := range c.ExcludeOperations {
		if op == "read" {
			return nil, fmt.Errorf("exclude_operations: %q cannot be excluded — every resource has Read", op)
		}
		if !supported[op] {
			return nil, fmt.Errorf("exclude_operations: %q is not a valid CRUD op for %s schema", op, schemaType)
		}
		excluded[op] = true
		delete(supported, op)
	}
	if c.API != nil {
		apiOps := map[string]bool{}
		if c.API.Create != nil && c.API.Create.Request != "" {
			apiOps["create"] = true
		}
		if c.API.Update != nil && c.API.Update.Request != "" {
			apiOps["update"] = true
		}
		if c.API.Delete != nil && c.API.Delete.Request != "" {
			apiOps["delete"] = true
		}
		for op := range apiOps {
			if excluded[op] {
				return nil, fmt.Errorf("api.%s is declared but %q is in exclude_operations — remove one", op, op)
			}
		}
	}
	return supported, nil
}

// rejectDescriptionOverrides errors on a description: key inside a
// field-config map (recursing through fields:). Yaml description overrides
// were removed; without this tombstone the non-strict parse would silently
// ignore stale keys. Fields NAMED description are fine — they appear as keys
// of a fields: map, never as a config key.
func rejectDescriptionOverrides(path string, val any) error {
	m, ok := val.(map[string]any)
	if !ok {
		return nil
	}
	if _, has := m["description"]; has {
		return fmt.Errorf("field %s sets description: — yaml description overrides were removed; descriptions come from apidescriptions.yaml (proto/OpenAPI) or the curated tables in internal/schemagen/descriptions.go", path)
	}
	if fields, ok := m["fields"].(map[string]any); ok {
		for name, child := range fields {
			if err := rejectDescriptionOverrides(path+"."+name, child); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseFieldConfig(val any) (FieldConfig, error) {
	fieldData, err := yaml.Marshal(val)
	if err != nil {
		return FieldConfig{}, err
	}
	var fc FieldConfig
	if err := yaml.Unmarshal(fieldData, &fc); err != nil {
		return FieldConfig{}, err
	}
	return fc, nil
}
