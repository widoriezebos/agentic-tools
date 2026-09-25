package config

import (
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

// TestMain clears the seat presence keys the seat tests own, once, before
// any parallel test starts, so an operator's shell cannot decide their input.
func TestMain(m *testing.M) {
	for _, key := range []string{SeatPresenceStaleMinutesKey, SeatPresenceNamespaceKey} {
		_ = os.Unsetenv(EnvName(key))
	}
	os.Exit(testenv.Main(m))
}
