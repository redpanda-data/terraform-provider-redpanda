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
