package hash

import (
	"crypto/sha256"
	"fmt"
)

// Compute returns a 12-character hex string from the SHA256 hash of the
// inputs separated by null bytes. It is a pure function with no file I/O.
func Compute(inputs ...string) string {
	h := sha256.New()
	for i, s := range inputs {
		if i > 0 {
			h.Write([]byte{0})
		}
		h.Write([]byte(s))
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:12]
}
