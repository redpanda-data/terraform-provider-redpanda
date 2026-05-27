//go:build upgrade

package upgrade

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"math/big"
)

// RandomName returns "<prefix>-<6 lowercase alnum chars>". Used by TestUpgrade*
// tests to keep resource names unique across runs.
func RandomName(prefix string) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	const n = 6
	var b bytes.Buffer
	for i := 0; i < n; i++ {
		r, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b.WriteByte(chars[r.Int64()])
	}
	return fmt.Sprintf("%s-%s", prefix, b.String())
}
