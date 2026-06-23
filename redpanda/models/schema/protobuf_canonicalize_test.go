// Copyright 2025 Redpanda Data, Inc.
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

package schema

import "testing"

func TestProtobufBodiesEquivalent(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{
			name: "identical",
			a:    `syntax = "proto3"; package p; message M { string a = 1; }`,
			b:    `syntax = "proto3"; package p; message M { string a = 1; }`,
			want: true,
		},
		{
			name: "registry canonicalization: enum reorder + FQN type ref",
			a: `syntax = "proto3";
package fulfillment.activity_feed.v1;

message ActivityFeedEvent {
  ActivityType type = 1;
}

enum ActivityType {
  ACTIVITY_TYPE_UNSPECIFIED = 0;
  ACTIVITY_TYPE_CREATED = 1;
}
`,
			b: `syntax = "proto3";
package fulfillment.activity_feed.v1;

enum ActivityType {
  ACTIVITY_TYPE_UNSPECIFIED = 0;
  ACTIVITY_TYPE_CREATED = 1;
}
message ActivityFeedEvent {
  .fulfillment.activity_feed.v1.ActivityType type = 1;
}
`,
			want: true,
		},
		{
			name: "whitespace and comments only",
			a:    "syntax = \"proto3\";\npackage p;\n// a comment\nmessage M {\n  int32 id = 1;\n}\n",
			b:    `syntax="proto3";package p;message M{int32 id=1;}`,
			want: true,
		},
		{
			name: "field reorder within message",
			a:    `syntax = "proto3"; package p; message M { string a = 1; int32 b = 2; }`,
			b:    `syntax = "proto3"; package p; message M { int32 b = 2; string a = 1; }`,
			want: true,
		},
		{
			name: "different field number is not equivalent",
			a:    `syntax = "proto3"; package p; message M { string a = 1; }`,
			b:    `syntax = "proto3"; package p; message M { string a = 2; }`,
			want: false,
		},
		{
			name: "different field type is not equivalent",
			a:    `syntax = "proto3"; package p; message M { string a = 1; }`,
			b:    `syntax = "proto3"; package p; message M { int32 a = 1; }`,
			want: false,
		},
		{
			name: "added field is not equivalent",
			a:    `syntax = "proto3"; package p; message M { string a = 1; }`,
			b:    `syntax = "proto3"; package p; message M { string a = 1; string b = 2; }`,
			want: false,
		},
		{
			name: "different enum value is not equivalent",
			a:    `syntax = "proto3"; package p; enum E { X = 0; Y = 1; }`,
			b:    `syntax = "proto3"; package p; enum E { X = 0; Y = 2; }`,
			want: false,
		},
		{
			name: "parse failure falls back to not-equivalent",
			a:    `this is not protobuf`,
			b:    `syntax = "proto3"; package p; message M { string a = 1; }`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProtobufBodiesEquivalent(tt.a, tt.b); got != tt.want {
				t.Errorf("ProtobufBodiesEquivalent() = %v, want %v", got, tt.want)
			}
		})
	}
}
