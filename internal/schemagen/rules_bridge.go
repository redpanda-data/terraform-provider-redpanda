// Copyright 2026 Redpanda Data, Inc.
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
	"log"

	bufvalidate "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"google.golang.org/protobuf/proto"
)

// BridgeWriteShapeRules copies buf.validate rules from the create-payload
// (write shape) onto rule-less fields of the walked read-shape tree, in
// place. Asymmetric APIs (cluster: walks Cluster, creates via ClusterCreate)
// carry their input rules only on the write shape, so the validator/
// description derivations that run over the walked tree would miss them.
// `required` never flows: on the read shape it would flip attrs Required and
// trip the optional-override drift check for fields that are legitimately
// server-populated after create.
func BridgeWriteShapeRules(walked *ProtoMessage, cfg *Config, schemaType string, lookup ProtoLookup) error {
	if cfg == nil || cfg.API == nil || cfg.API.Create == nil || cfg.API.Create.Request == "" {
		return nil
	}
	supported, err := cfg.SupportedOperations(schemaType)
	if err != nil {
		return err
	}
	if !supported["create"] {
		return nil
	}
	rpc := cfg.API.Create
	if err := inferRPCPayload(rpc, lookup); err != nil {
		return fmt.Errorf("bridge rules: %w", err)
	}
	reqMsg, err := lookup(rpc.Request)
	if err != nil {
		return fmt.Errorf("bridge rules: looking up %s: %w", rpc.Request, err)
	}
	writeShape := reqMsg
	if rpc.PayloadField != "" {
		pf := reqMsg.FindField(rpc.PayloadField)
		if pf == nil || pf.Nested == nil {
			log.Printf("[schemagen warning] bridge: %s.%s has no nested payload message — skipping rule bridge", rpc.Request, rpc.PayloadField)
			return nil
		}
		writeShape = pf.Nested
	}
	if writeShape.Name == walked.Name && writeShape.GoName == walked.GoName {
		return nil
	}
	bridgeMessageRules(walked, writeShape, "")
	return nil
}

// bridgeMessageRules recursively copies write-shape rules onto rule-less
// read-shape fields, matching by proto field name. Fields present on only
// one shape are expected divergence (read-only status, write-only inputs)
// and skip silently; same-named fields with incompatible shapes warn and
// skip their subtree.
func bridgeMessageRules(read, write *ProtoMessage, path string) {
	if read == nil || write == nil {
		return
	}
	for i := range read.Fields {
		rf := &read.Fields[i]
		wf := write.FindField(rf.Name)
		if wf == nil {
			continue
		}
		if rf.Cardinality != wf.Cardinality || rf.Kind != wf.Kind || rf.MapValKind != wf.MapValKind {
			log.Printf("[schemagen warning] bridge %s: read shape is %s/%s but write shape is %s/%s — rules not bridged",
				joinPath(path, rf.Name), rf.Cardinality, rf.Kind, wf.Cardinality, wf.Kind)
			continue
		}
		if rf.ValidateRules == nil && wf.ValidateRules != nil {
			cloned, ok := proto.Clone(wf.ValidateRules).(*bufvalidate.FieldRules)
			if !ok {
				log.Printf("[schemagen warning] bridge %s: cloned rules are %T, not *bufvalidate.FieldRules — rules not bridged",
					joinPath(path, rf.Name), proto.Clone(wf.ValidateRules))
				continue
			}
			cloned.Required = nil
			if proto.Size(cloned) > 0 {
				rf.ValidateRules = cloned
			}
		}
		if rf.Nested != nil && wf.Nested != nil {
			bridgeMessageRules(rf.Nested, wf.Nested, joinPath(path, rf.Name))
		}
	}
}
