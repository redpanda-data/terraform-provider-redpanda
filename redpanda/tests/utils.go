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

package tests

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"math/big"
)

// generateRandomName generates a random name with a given prefix. The name will
// have the form of '<prefix>-<random>' where random is any 4 alphanumeric
// characters.
func generateRandomName(prefix string) string {
	baseChars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	randomLength := 4 // Should be good, this is 62^4 = 14M combinations.

	var randStr bytes.Buffer
	for i := 0; i < randomLength; i++ {
		r, _ := rand.Int(rand.Reader, big.NewInt(int64(len(baseChars))))
		randStr.WriteByte(baseChars[r.Int64()])
	}

	return fmt.Sprintf("%v-%v", prefix, randStr.String())
}
