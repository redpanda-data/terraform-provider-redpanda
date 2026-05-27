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

package fakes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"sync"
	"testing"

	"github.com/redpanda-data/common-go/rpsr"
	"github.com/twmb/franz-go/pkg/sr"
)

const defaultCompatibility = "BACKWARD"

// SchemaRegistryFake is an httptest-backed in-memory Schema Registry. It also
// serves the rpsr ACL endpoints (/security/acls) so a single fake backs both
// redpanda_schema and redpanda_schema_registry_acl. No auth validation — the
// integration tests pass Basic creds in HCL purely to satisfy the provider's
// schemaRegistryAuthOption gate; the fake ignores the headers.
type SchemaRegistryFake struct {
	srv *httptest.Server

	mu            sync.Mutex
	subjects      map[string][]*srEntry
	subjectCompat map[string]string
	globalCompat  string
	acls          []rpsr.ACL
	nextID        int
	httpOverrides map[string]*httpOverride
}

type srEntry struct {
	version int
	id      int
	schema  sr.Schema
}

type httpOverride struct {
	statusCode int
	body       string
}

// NewSchemaRegistryFake starts the httptest server and registers cleanup. The
// httptest server is wrapped with overrideMiddleware so OverrideOnceHTTP can
// intercept matching requests before the mux dispatches them. On t.Cleanup the
// fake fails the test if any registered override was never consumed — a typo
// in the path argument is otherwise silent.
func NewSchemaRegistryFake(t testing.TB) *SchemaRegistryFake {
	t.Helper()
	f := &SchemaRegistryFake{
		subjects:      map[string][]*srEntry{},
		subjectCompat: map[string]string{},
		globalCompat:  defaultCompatibility,
		httpOverrides: map[string]*httpOverride{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /subjects/{subject}/versions", f.handleRegisterSchema)
	mux.HandleFunc("GET /subjects/{subject}/versions", f.handleListVersions)
	mux.HandleFunc("GET /subjects/{subject}/versions/{version}", f.handleGetVersion)
	mux.HandleFunc("DELETE /subjects/{subject}", f.handleDeleteSubject)
	mux.HandleFunc("GET /schemas/ids/{id}/versions", f.handleSchemaIDVersions)
	mux.HandleFunc("GET /config/{subject}", f.handleGetConfig)
	mux.HandleFunc("PUT /config/{subject}", f.handlePutConfig)
	mux.HandleFunc("POST /security/acls", f.handleCreateACLs)
	mux.HandleFunc("DELETE /security/acls", f.handleDeleteACLs)
	mux.HandleFunc("GET /security/acls", f.handleListACLs)
	f.srv = httptest.NewServer(f.overrideMiddleware(mux))
	t.Cleanup(func() {
		f.srv.Close()
		f.mu.Lock()
		leftover := make([]string, 0, len(f.httpOverrides))
		for k := range f.httpOverrides {
			leftover = append(leftover, k)
		}
		f.mu.Unlock()
		if len(leftover) > 0 {
			sort.Strings(leftover)
			t.Errorf("SchemaRegistryFake: %d OverrideOnceHTTP entry(ies) registered but never consumed (likely path typo): %v", len(leftover), leftover)
		}
	})
	return f
}

// OverrideOnceHTTP arranges that the next request whose method and path match
// the given key returns the given status code and body instead of dispatching
// to the route handler. Consumed on first match; subsequent requests fall
// through to the registered handler. Calling OverrideOnceHTTP again for the
// same (method, path) before the first override fires replaces it.
//
// path is the EXPANDED literal URL path that the client will send, e.g.
// "/subjects/my-subject/versions/3", NOT a mux pattern with placeholders like
// "/subjects/{subject}/versions/{version}". A typo or mismatched path is
// flagged at t.Cleanup as an unconsumed override.
//
// body is written verbatim as the response body; the caller is responsible for
// the content (typically the Schema Registry error JSON shape, but the hook is
// payload-agnostic). Content-Type is set to the SR JSON media type.
func (f *SchemaRegistryFake) OverrideOnceHTTP(method, path string, statusCode int, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.httpOverrides == nil {
		f.httpOverrides = map[string]*httpOverride{}
	}
	f.httpOverrides[method+" "+path] = &httpOverride{statusCode: statusCode, body: body}
}

// overrideMiddleware wraps the route mux. Before dispatch it checks for a
// pending OverrideOnceHTTP entry keyed on "METHOD path"; on hit it consumes
// the entry and writes the override response, on miss it falls through to the
// inner handler. Lookup and consumption are guarded by f.mu, the same mutex
// that protects the fake's state maps.
func (f *SchemaRegistryFake) overrideMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		f.mu.Lock()
		ov, ok := f.httpOverrides[key]
		if ok {
			delete(f.httpOverrides, key)
		}
		f.mu.Unlock()
		if ok {
			w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
			w.WriteHeader(ov.statusCode)
			_, _ = w.Write([]byte(ov.body))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// BaseURL returns the httptest server URL. Cluster fakes report this on
// cluster.SchemaRegistry.Url so the provider's SR client dials in-process.
func (f *SchemaRegistryFake) BaseURL() string {
	return f.srv.URL
}

func (f *SchemaRegistryFake) handleRegisterSchema(w http.ResponseWriter, r *http.Request) {
	subject := r.PathValue("subject")
	var body sr.Schema
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	existing := f.subjects[subject]
	for _, e := range existing {
		if e.schema.Schema == body.Schema && e.schema.Type == body.Type {
			writeJSON(w, map[string]int{"id": e.id})
			return
		}
	}
	f.nextID++
	id := f.nextID
	version := len(existing) + 1
	f.subjects[subject] = append(existing, &srEntry{version: version, id: id, schema: body})
	writeJSON(w, map[string]int{"id": id})
}

func (f *SchemaRegistryFake) handleListVersions(w http.ResponseWriter, r *http.Request) {
	subject := r.PathValue("subject")
	f.mu.Lock()
	defer f.mu.Unlock()
	entries, ok := f.subjects[subject]
	if !ok {
		writeError(w, http.StatusNotFound, 40401, "Subject not found")
		return
	}
	versions := make([]int, 0, len(entries))
	for _, e := range entries {
		versions = append(versions, e.version)
	}
	sort.Ints(versions)
	writeJSON(w, versions)
}

func (f *SchemaRegistryFake) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	subject := r.PathValue("subject")
	versionStr := r.PathValue("version")
	f.mu.Lock()
	defer f.mu.Unlock()
	entries, ok := f.subjects[subject]
	if !ok || len(entries) == 0 {
		writeError(w, http.StatusNotFound, 40401, "Subject not found")
		return
	}
	var target *srEntry
	if versionStr == "latest" {
		target = entries[len(entries)-1]
	} else {
		v, err := strconv.Atoi(versionStr)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, 42202, "Invalid version")
			return
		}
		for _, e := range entries {
			if e.version == v {
				target = e
				break
			}
		}
	}
	if target == nil {
		writeError(w, http.StatusNotFound, 40402, "Version not found")
		return
	}
	writeJSON(w, sr.SubjectSchema{
		Subject: subject,
		Version: target.version,
		ID:      target.id,
		Schema:  target.schema,
	})
}

func (f *SchemaRegistryFake) handleDeleteSubject(w http.ResponseWriter, r *http.Request) {
	subject := r.PathValue("subject")
	f.mu.Lock()
	defer f.mu.Unlock()
	entries, ok := f.subjects[subject]
	if !ok {
		writeError(w, http.StatusNotFound, 40401, "Subject not found")
		return
	}
	versions := make([]int, 0, len(entries))
	for _, e := range entries {
		versions = append(versions, e.version)
	}
	delete(f.subjects, subject)
	delete(f.subjectCompat, subject)
	sort.Ints(versions)
	writeJSON(w, versions)
}

func (f *SchemaRegistryFake) handleSchemaIDVersions(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, 42202, "Invalid id")
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	type subjectVersion struct {
		Subject string `json:"subject"`
		Version int    `json:"version"`
	}
	var out []subjectVersion
	for subject, entries := range f.subjects {
		for _, e := range entries {
			if e.id == id {
				out = append(out, subjectVersion{Subject: subject, Version: e.version})
			}
		}
	}
	if out == nil {
		writeError(w, http.StatusNotFound, 40403, "Schema not found")
		return
	}
	writeJSON(w, out)
}

func (f *SchemaRegistryFake) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	subject := r.PathValue("subject")
	defaultToGlobal := r.URL.Query().Get("defaultToGlobal") == "true"
	f.mu.Lock()
	defer f.mu.Unlock()
	level, ok := f.subjectCompat[subject]
	if !ok {
		if !defaultToGlobal {
			writeError(w, http.StatusNotFound, 40401, "Subject not found")
			return
		}
		level = f.globalCompat
	}
	writeJSON(w, map[string]string{"compatibilityLevel": level})
}

func (f *SchemaRegistryFake) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	subject := r.PathValue("subject")
	var body sr.SetCompatibility
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	level := body.Level.String()
	f.mu.Lock()
	f.subjectCompat[subject] = level
	f.mu.Unlock()
	writeJSON(w, map[string]string{"compatibility": level})
}

func (f *SchemaRegistryFake) handleCreateACLs(w http.ResponseWriter, r *http.Request) {
	var body []rpsr.ACL
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, a := range body {
		if !f.aclExists(a) {
			f.acls = append(f.acls, a)
		}
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("[]"))
}

func (f *SchemaRegistryFake) handleDeleteACLs(w http.ResponseWriter, r *http.Request) {
	var body []rpsr.ACL
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	kept := f.acls[:0]
	for _, existing := range f.acls {
		match := false
		for _, a := range body {
			if aclEqual(existing, a) {
				match = true
				break
			}
		}
		if !match {
			kept = append(kept, existing)
		}
	}
	f.acls = kept
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("[]"))
}

func (f *SchemaRegistryFake) handleListACLs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := rpsr.ACL{
		Principal:    q.Get("principal"),
		Resource:     q.Get("resource"),
		ResourceType: rpsr.ResourceType(q.Get("resource_type")),
		PatternType:  rpsr.PatternType(q.Get("pattern_type")),
		Host:         q.Get("host"),
		Operation:    rpsr.Operation(q.Get("operation")),
		Permission:   rpsr.Permission(q.Get("permission")),
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []rpsr.ACL{}
	for _, a := range f.acls {
		if matchesFilter(a, filter) {
			out = append(out, a)
		}
	}
	writeJSON(w, out)
}

func (f *SchemaRegistryFake) aclExists(a rpsr.ACL) bool {
	for _, existing := range f.acls {
		if aclEqual(existing, a) {
			return true
		}
	}
	return false
}

func aclEqual(a, b rpsr.ACL) bool {
	return a.Principal == b.Principal &&
		a.Resource == b.Resource &&
		a.ResourceType == b.ResourceType &&
		a.PatternType == b.PatternType &&
		a.Host == b.Host &&
		a.Operation == b.Operation &&
		a.Permission == b.Permission
}

func matchesFilter(a, filter rpsr.ACL) bool {
	if filter.Principal != "" && filter.Principal != a.Principal {
		return false
	}
	if filter.Resource != "" && filter.Resource != a.Resource {
		return false
	}
	if filter.ResourceType != "" && filter.ResourceType != rpsr.ResourceTypeAny && filter.ResourceType != a.ResourceType {
		return false
	}
	if filter.PatternType != "" && filter.PatternType != rpsr.PatternTypeAny && filter.PatternType != a.PatternType {
		return false
	}
	if filter.Host != "" && filter.Host != a.Host {
		return false
	}
	if filter.Operation != "" && filter.Operation != rpsr.OperationAny && filter.Operation != a.Operation {
		return false
	}
	if filter.Permission != "" && filter.Permission != rpsr.PermissionAny && filter.Permission != a.Permission {
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status, code int, msg string) {
	w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error_code": code, "message": msg})
}
