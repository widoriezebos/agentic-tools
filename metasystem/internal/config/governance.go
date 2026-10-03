package config

import (
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/roots"
)

// CorrelationPolicy reads the deliberately empty policy slot. A, B, or C is
// the complete activation vocabulary; an empty value means no correlation
// policy has authority yet.
func CorrelationPolicy(installation roots.Installation) (string, error) {
	value, code, err := Get(GetParams{Key: "metasystem.governance.correlation-policy",
		ConfPath: installation.Path("metasystem.conf")})
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
