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

package acc

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"strings"
)

// RandomName returns a name of the form "<prefix>-<suffix>". When the env
// var TF_TEST_OBJECT_SUFFIX is set (e.g. to $BUILDKITE_COMMIT) the suffix
// is the first 8 chars of that value lowercased; otherwise the suffix is
// 4 random alphanumerics.
func RandomName(prefix string) string {
	if suffix := os.Getenv("TF_TEST_OBJECT_SUFFIX"); suffix != "" {
		if len(suffix) > 8 {
			suffix = suffix[:8]
		}
		return fmt.Sprintf("%s-%s", prefix, strings.ToLower(suffix))
	}
	const baseChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	const randomLength = 4
	var b bytes.Buffer
	for i := 0; i < randomLength; i++ {
		r, _ := rand.Int(rand.Reader, big.NewInt(int64(len(baseChars))))
		b.WriteByte(baseChars[r.Int64()])
	}
	return fmt.Sprintf("%s-%s", prefix, b.String())
}
