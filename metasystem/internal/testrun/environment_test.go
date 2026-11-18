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

func TestSharedEnvironmentDropsExportedFunctions(t *testing.T) {
	t.Parallel()
	environment, err := Environment([]string{"BASH_FUNC_which%%=synthetic-function-body"})
	if err != nil || len(environment) != 0 {
		t.Fatal("shared environment refused an unrelated ambient name")
	}
	_, err = RequireWorkerCapabilities(t.Context(), "/nonexistent-e1-worker", []string{"BASH_FUNC_which%%=synthetic-function-body"})
	if !errors.Is(err, ErrWorkerPolicyUnsupported) {
		t.Fatal("ambient name blocked worker admission before handshake")
	}
}
