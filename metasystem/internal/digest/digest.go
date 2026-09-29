// Package digest owns the lowercase hexadecimal SHA-256 spelling the engine
// binds evidence to: of bytes in hand, and of a file's whole content.
package digest

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
)

// SHA256 is data's SHA-256 in lowercase hexadecimal.
func SHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// FileSHA256 is the SHA-256 of path's whole content in lowercase
// hexadecimal; an unreadable file answers its read error.
func FileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return SHA256(data), nil
}
