package schemagen

import (
	"context"
	"strings"
	"testing"

	"github.com/bufbuild/protocompile"
)

// compileInMemory compiles a proto3 source string and extracts a named message.
func compileInMemory(t *testing.T, src, messageName string) *ProtoMessage {
	t.Helper()
	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			Accessor: protocompile.SourceAccessorFromMap(map[string]string{
				"test.proto": src,
			}),
		}),
		SourceInfoMode: protocompile.SourceInfoStandard,
	}
	files, err := compiler.Compile(context.Background(), "test.proto")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	msg, _, err := ExtractMessage(files, messageName, "test.proto")
	if err != nil {
		t.Fatalf("extract %s: %v", messageName, err)
	}
	return msg
}

// TestExtractMessage_OneofName verifies that fields belonging to a real
// (non-synthetic) oneof have their OneofName populated on the ProtoField.
// Fields outside any oneof must have an empty OneofName.
func TestExtractMessage_OneofName(t *testing.T) {
	src := strings.TrimSpace(`
syntax = "proto3";
package test;
message Request {
  string id = 1;
  oneof payload {
    string create_body = 2;
    string update_body = 3;
  }
  bool active = 4;
}
`)
	msg := compileInMemory(t, src, "Request")

	fm := make(map[string]ProtoField)
	for _, f := range msg.Fields {
		fm[f.Name] = f
	}

	cases := []struct {
		field         string
		wantOneofName string
	}{
		{"id", ""},
		{"create_body", "payload"},
		{"update_body", "payload"},
		{"active", ""},
	}
	for _, c := range cases {
		f, ok := fm[c.field]
		if !ok {
			t.Errorf("field %q not found", c.field)
			continue
		}
		if f.OneofName != c.wantOneofName {
			t.Errorf("field %q: OneofName = %q, want %q", c.field, f.OneofName, c.wantOneofName)
		}
	}
}

// TestExtractMessage_SyntheticOneofNotExposed verifies that proto3-optional
// fields (which use synthetic oneofs internally) have IsOptional=true but
// OneofName="" — synthetic oneofs must not leak as real oneof membership.
func TestExtractMessage_SyntheticOneofNotExposed(t *testing.T) {
	src := strings.TrimSpace(`
syntax = "proto3";
package test;
message Msg {
  optional string maybe = 1;
  string always  = 2;
}
`)
	msg := compileInMemory(t, src, "Msg")

	fm := make(map[string]ProtoField)
	for _, f := range msg.Fields {
		fm[f.Name] = f
	}

	maybe, ok := fm["maybe"]
	if !ok {
		t.Fatal("field 'maybe' not found")
	}
	if !maybe.IsOptional {
		t.Error("maybe: expected IsOptional=true")
	}
	if maybe.OneofName != "" {
		t.Errorf("maybe: synthetic oneof leaked as OneofName=%q", maybe.OneofName)
	}
	always, ok := fm["always"]
	if !ok {
		t.Fatal("field 'always' not found")
	}
	if always.IsOptional {
		t.Error("always: expected IsOptional=false")
	}
}
