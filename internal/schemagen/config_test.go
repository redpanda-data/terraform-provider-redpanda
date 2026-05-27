package schemagen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig_RejectsMalformedValidator(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		wantSub string
	}{
		{
			name:    "scalar non-string",
			yaml:    "message: T\ntf_name: T\nsome_field:\n  type: string\n  validator: 42\n",
			wantSub: `field "some_field": validator must be string or []string, got int`,
		},
		{
			name:    "list with non-string item",
			yaml:    "message: T\ntf_name: T\nsome_field:\n  type: string\n  validator:\n    - foo\n    - 42\n",
			wantSub: `field "some_field": validator[1] must be string, got int`,
		},
		{
			name:    "nested field malformed",
			yaml:    "message: T\ntf_name: T\nouter:\n  type: single_nested\n  fields:\n    inner:\n      type: string\n      validator: {a: b}\n",
			wantSub: `field "outer.inner": validator must be string or []string`,
		},
	}
	dir := t.TempDir()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(dir, tc.name+".yaml")
			if err := os.WriteFile(p, []byte(tc.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := LoadConfig(p)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}

func TestLoadConfig_AcceptsValidValidatorShapes(t *testing.T) {
	yaml := `message: T
tf_name: T
a:
  type: string
  validator: oneof_aws_gcp
b:
  type: string
  validator:
    - foo
    - bar
c:
  type: string
`
	p := filepath.Join(t.TempDir(), "ok.yaml")
	if err := os.WriteFile(p, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cfg.Fields["a"].ValidatorNames(); len(got) != 1 || got[0] != "oneof_aws_gcp" {
		t.Errorf("a.validator: got %v", got)
	}
	if got := cfg.Fields["b"].ValidatorNames(); len(got) != 2 || got[0] != "foo" || got[1] != "bar" {
		t.Errorf("b.validator: got %v", got)
	}
	if got := cfg.Fields["c"].ValidatorNames(); got != nil {
		t.Errorf("c.validator: got %v, want nil", got)
	}
}
