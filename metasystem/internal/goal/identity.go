package goal

import (
	"crypto/rand"
	"fmt"
)

// NewOperationULID mints the caller-owned 26-character identity from which a
// goal operation identifier is derived. Owning this in the engine package
// makes non-command producers such as dispatch use the same identity seam.
func NewOperationULID() (string, error) {
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	raw := make([]byte, 26)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("mint goal operation ULID: %w", err)
	}
	for i := range raw {
		raw[i] = alphabet[int(raw[i])%len(alphabet)]
	}
	return string(raw), nil
}
