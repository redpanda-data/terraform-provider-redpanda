// Copyright 2026 Redpanda Data, Inc.
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

package utils

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"
)

// redacted replaces a removed credential. It carries neither the length nor
// a prefix of what it replaces.
const redacted = "[REDACTED]"

// minRedactLength keeps a short value, such as a credential source name or a
// region, from being masked everywhere it appears in ordinary output.
const minRedactLength = 8

// maxPEMLines bounds the lines a PEM private key's body can take; a 4096-bit
// RSA key is about 50.
const maxPEMLines = 100

// credentialWords names a credential inside a variable, flag, or key name.
const credentialWords = `secret|token|passw(?:or)?d|passphrase|private[_-]?key|credentials?|certificate|keyfile|api[_-]?key|access[_-]?key`

var (
	credentialName = regexp.MustCompile(`(?i)` + credentialWords)
	// A name such as ARM_OIDC_TOKEN_FILE_PATH or --credential-source holds a
	// reference to, or a property of, a credential rather than the credential.
	credentialMetadata = regexp.MustCompile(`(?i)[_-](file|path|url|uri|dir|directory|lifetime|expiration|override|id|name|endpoint|source|enabled)$`)
	// These name credential types or files, not credentials.
	notCredentials = []string{"AZURE_TOKEN_CREDENTIALS", "GOOGLE_APPLICATION_CREDENTIALS"}
)

func isCredentialName(name string) bool {
	return credentialName.MatchString(name) && !credentialMetadata.MatchString(name) && !slices.Contains(notCredentials, name)
}

const credentialKey = `[A-Za-z0-9_-]*(?:` + credentialWords + `)`

var (
	jwtPattern = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]*`)
	// A bearer token has a digit or a token character in it, which keeps an
	// ordinary word after "bearer" readable.
	bearerPattern = regexp.MustCompile(`(?i)(\bbearer\s+)([A-Za-z0-9._~+/=-]*[0-9._~+/=-][A-Za-z0-9._~+/=-]*)()`)
	// A PEM private key whole on one line, as in a JSON string with \n
	// escapes, and the marker of one that continues on the next lines, with
	// any key text after the marker on the same line.
	pemInline = regexp.MustCompile(`-{5}BEGIN [A-Z0-9 ]*PRIVATE KEY-{5}.*?-{5}END [A-Z0-9 ]*PRIVATE KEY-{5}`)
	pemBegin  = regexp.MustCompile(`-{5}BEGIN [A-Z0-9 ]*PRIVATE KEY-{5}((\s|\\n)+[A-Za-z0-9+/=]{40,})*`)
	pemEnd    = regexp.MustCompile(`-{5}END [A-Z0-9 ]*PRIVATE KEY-{5}`)
	// A line of a key's base64 body, optionally behind the - or + of a plan
	// diff.
	pemBody = regexp.MustCompile(`^\s*[-+~]?\s*([A-Za-z0-9+/]{1,76}={0,2})\s*$`)
	// A value assigned to a credential-named key: double-quoted, escaped
	// inside a JSON string, single-quoted, or bare after =, including a
	// Terraform JSON plan's "name": {"value": "..."} under such a key. A bare
	// value never starts with "(", which keeps Terraform's (sensitive value)
	// and (known after apply) placeholders readable.
	assignDouble  = regexp.MustCompile(`(?i)(` + credentialKey + `"?\s*[:=]\s*(?:\{\s*"value"\s*:\s*)?")((?:[^"\\]|\\.)*)(")`)
	assignEscaped = regexp.MustCompile(`(?i)(` + credentialKey + `\\"\s*:\s*(?:\{\s*\\"value\\"\s*:\s*)?\\")((?:[^"\\]|\\[^"])*)(\\")`)
	assignSingle  = regexp.MustCompile(`(?i)(` + credentialKey + `'?\s*[:=]\s*')([^']*)(')`)
	assignBare    = regexp.MustCompile(`(?i)(\b` + credentialKey + `\s*=\s*)([^\s"'&,;#(][^\s"'&,;#]*)()`)
	// The new value of an updated attribute in plan output: "old" -> "new".
	updatedValue = regexp.MustCompile(`(->\s*")((?:[^"\\]|\\.)*)(")`)
)

// redactor removes credentials from text a subprocess prints. It replaces
// every value it holds exactly, then anything shaped like a JWT, a bearer
// token, a PEM private key, a value assigned to a credential-named key, or a
// Terraform plan in a JSON object, which covers some credentials the plugin
// obtains on its own.
type redactor struct {
	values []string
}

// newRedactor returns a redactor for the given credential values. A value
// that spans lines, such as an indented key file, is also held line by line,
// because output is redacted a line at a time. Values shorter than
// minRedactLength are ignored.
func newRedactor(values ...string) *redactor {
	r := &redactor{}
	hold := func(v string) {
		v = strings.TrimSpace(v)
		if len(v) >= minRedactLength && !slices.Contains(r.values, v) {
			r.values = append(r.values, v)
		}
	}
	for _, v := range values {
		hold(v)
		if strings.Contains(v, "\n") {
			for _, l := range strings.Split(v, "\n") {
				hold(strings.TrimRight(l, ","))
			}
		}
	}
	// Longest first, so a value that contains another is replaced whole.
	slices.SortFunc(r.values, func(a, b string) int { return len(b) - len(a) })
	return r
}

// newSubprocessRedactor returns a redactor for the credential values a
// subprocess is handed: the value after each credential-named flag in args,
// the value of each credential-named variable in env, and the values in
// held, such as a key the provider writes to a file for the subprocess.
func newSubprocessRedactor(args, env []string, held ...string) *redactor {
	values := append([]string{}, held...)
	for i, a := range args {
		flag, val, hasVal := strings.Cut(a, "=")
		if !strings.HasPrefix(flag, "-") || !isCredentialName(strings.TrimLeft(flag, "-")) {
			continue
		}
		if hasVal {
			values = append(values, val)
		} else if i+1 < len(args) {
			values = append(values, args[i+1])
		}
	}
	for _, kv := range env {
		if name, val, ok := strings.Cut(kv, "="); ok && isCredentialName(name) {
			values = append(values, val)
		}
	}
	return newRedactor(values...)
}

// String redacts a piece of text such as an error message, line by line.
func (r *redactor) String(s string) string {
	redact := r.Lines()
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = redact(l)
	}
	return strings.Join(lines, "\n")
}

// Lines returns a function that redacts a stream line by line. Every line
// is redacted whole. The one state it carries is a PEM private key printed
// across lines: after a line that ends with the BEGIN marker, up to
// maxPEMLines lines of base64 body are removed, and the END line or any
// other line ends the key. A zap line's message is what is tested for body,
// because the plugin logs each line Terraform prints as one.
func (r *redactor) Lines() func(string) string {
	inPEM, bodyLines := false, 0
	return func(line string) string {
		body := line
		if z := parseZapLog(line); z != nil {
			body = z.Message
		}
		if inPEM {
			switch {
			case pemEnd.MatchString(body):
				inPEM = false
				line = redacted + line[pemEnd.FindStringIndex(line)[1]:]
			case bodyLines < maxPEMLines && isPEMBody(body):
				bodyLines++
				return redacted
			default:
				inPEM = false
			}
		}
		line = planJSON(r.exact(line))
		line = pemInline.ReplaceAllString(line, redacted)
		if loc := pemBegin.FindStringIndex(line); loc != nil {
			inPEM, bodyLines = strings.TrimSpace(line[loc[1]:]) == "", 0
			line = line[:loc[0]] + redacted + line[loc[1]:]
		}
		return patterns(line)
	}
}

// isPEMBody reports whether a line can be a line of a key's base64 body: a
// full 64-character line, or a shorter one with padding, a digit, + or /,
// which an ordinary word lacks.
func isPEMBody(line string) bool {
	m := pemBody.FindStringSubmatch(line)
	if m == nil {
		return false
	}
	return len(m[1]) >= 40 || strings.ContainsAny(m[1], "=+/0123456789")
}

var planKeys = []string{"resource_changes", "planned_values", "prior_state", "raw_plan"}

func isPlan(v any) bool {
	if s, ok := v.(string); ok {
		if json.Unmarshal([]byte(s), &v) != nil {
			return false
		}
	}
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	for _, k := range planKeys {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}

// planJSON replaces a Terraform plan in a line that ends with a JSON object:
// the object itself, as a plan printed alone, or a field of it, as the zap
// fields the plugin logs a plan in. A plan carries every variable and
// resource value in plain text, whatever their names. A line that does not
// parse as such an object is returned as is.
func planJSON(line string) string {
	if !slices.ContainsFunc(planKeys, func(k string) bool { return strings.Contains(line, k) }) {
		return line
	}
	start := strings.LastIndex(line, "\t{") + 1
	if start == 0 && !strings.HasPrefix(line, "{") {
		return line
	}
	var fields map[string]any
	if json.Unmarshal([]byte(line[start:]), &fields) != nil {
		return line
	}
	const replaced = redacted + " (Terraform plan JSON)"
	if isPlan(fields) {
		return line[:start] + replaced
	}
	changed := false
	for k, v := range fields {
		if isPlan(v) {
			fields[k], changed = replaced, true
		}
	}
	if !changed {
		return line
	}
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if enc.Encode(fields) != nil {
		return line[:start] + replaced
	}
	return line[:start] + strings.TrimSuffix(b.String(), "\n")
}

func (r *redactor) exact(s string) string {
	for _, v := range r.values {
		s = strings.ReplaceAll(s, v, redacted)
	}
	return s
}

// replaceValue replaces the second group of every match of re, keeping the
// first and third, unless the value is short or already redacted. It also
// reports whether re matched at all.
func replaceValue(re *regexp.Regexp, s string) (string, bool) {
	matched := false
	out := re.ReplaceAllStringFunc(s, func(m string) string {
		matched = true
		sub := re.FindStringSubmatch(m)
		if strings.HasPrefix(sub[2], redacted) || len(sub[2]) < minRedactLength {
			return m
		}
		return sub[1] + redacted + sub[3]
	})
	return out, matched
}

func patterns(s string) string {
	s = jwtPattern.ReplaceAllString(s, redacted)
	s, _ = replaceValue(bearerPattern, s)
	assigned := false
	for _, re := range []*regexp.Regexp{assignEscaped, assignDouble, assignSingle, assignBare} {
		var matched bool
		s, matched = replaceValue(re, s)
		assigned = assigned || matched
	}
	if assigned {
		s, _ = replaceValue(updatedValue, s)
	}
	return s
}
