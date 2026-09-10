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

package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const licenseHeaderFmt = `// Copyright %d Redpanda Data, Inc.
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

`

var headerYearRe = regexp.MustCompile(`(?m)^// Copyright (\d{4}) Redpanda Data, Inc\.`)

// LicenseHeader is the canonical header stamped with the current year, for new files only.
func LicenseHeader() string { return LicenseHeaderYear(time.Now().Year()) }

// LicenseHeaderYear renders the canonical header for a fixed year; tests pin it.
func LicenseHeaderYear(year int) string { return fmt.Sprintf(licenseHeaderFmt, year) }

// PreserveHeaderYear keeps the copyright year already on disk at path: the year is
// the file's creation year and regeneration must not bump it.
func PreserveHeaderYear(path string, src []byte) []byte {
	existing, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return src
	}
	m := headerYearRe.FindSubmatch(existing)
	if m == nil {
		return src
	}
	return headerYearRe.ReplaceAllLiteral(src, []byte("// Copyright "+string(m[1])+" Redpanda Data, Inc."))
}
