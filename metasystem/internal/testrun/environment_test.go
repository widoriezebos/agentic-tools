package testrun

import (
	"errors"
	"strings"
	"testing"
)

func TestSharedEnvironmentRefusesUnsupportedInputsBeforeLaunch(t *testing.T) {
	for _, name := range []string{"GOPROXY", "GOFLAGS", "GOAUTH"} {
		value := "https://user:synthetic-unsupported@host"
		if name == "GOAUTH" {
			value = "synthetic-unsupported"
		}
		environment := []string{name + "=" + value}
		if _, err := Environment(environment); err == nil || !strings.Contains(err.Error(), name) || strings.Contains(err.Error(), "synthetic") {
			t.Fatal("shared input was accepted or disclosed")
		}
		_, err := RequireWorkerCapabilities(t.Context(), "/unavailable-worker", environment)
		if err == nil || errors.Is(err, ErrWorkerPolicyUnsupported) || !strings.Contains(err.Error(), name) || strings.Contains(err.Error(), "synthetic") {
			t.Fatal("worker launch preceded input admission")
		}
	}
}
