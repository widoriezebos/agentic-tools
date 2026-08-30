package config

import (
	"fmt"
	"path/filepath"
)

// CorrelationPolicy reads the deliberately empty policy slot. A, B, or C is
// the complete activation vocabulary; an empty value means no correlation
// policy has authority yet.
func CorrelationPolicy(repoRoot string) (string, error) {
	value, code, err := Get(GetParams{Key: "metasystem.governance.correlation-policy",
		ConfPath: filepath.Join(repoRoot, "metasystem.conf")})
	if err != nil || code != 0 {
		return "", err
	}
	switch value {
	case "", "A", "B", "C":
		return value, nil
	default:
		return "", fmt.Errorf("metasystem.governance.correlation-policy must be empty, A, B, or C")
	}
}
