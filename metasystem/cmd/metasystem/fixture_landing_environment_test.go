package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestFixtureCommandEnvironmentOwnsLandingInputs(t *testing.T) {
	t.Parallel()
	const helper = "GO_WANT_FIXTURE_LANDING_ENVIRONMENT_CHILD"
	if os.Getenv(helper) == "1" {
		if value, present := os.LookupEnv("LANDING_ONLY"); !present || value != "" || os.Getenv("LANDING_COMMIT") != "outer-commit" {
			t.Fatal("helper did not receive the deliberately declared outer proof environment")
		}
		for _, entry := range fixtureCommandEnvironment(t) {
			if strings.HasPrefix(entry, "LANDING_") {
				t.Errorf("fixture inherited outer proof environment: %s", entry)
			}
		}
		seen := map[string]string{}
		for _, entry := range fixtureCommandEnvironment(t, "LANDING_ONLY=fixture-unit", "LANDING_PROOF_BASE=fixture-base", "LANDING_FUTURE_CONTROL=fixture-value") {
			name, value, _ := strings.Cut(entry, "=")
			if _, duplicate := seen[name]; duplicate && strings.HasPrefix(name, "LANDING_") {
				t.Errorf("duplicate fixture control: %s", name)
			}
			seen[name] = value
		}
		for name, want := range map[string]string{
			"LANDING_ONLY": "fixture-unit", "LANDING_PROOF_BASE": "fixture-base", "LANDING_FUTURE_CONTROL": "fixture-value",
			"GO_WANT_BATCH_E2E_COMMAND": "1", "FIXTURE_LANDING_UNRELATED": "kept",
		} {
			if got := seen[name]; got != want {
				t.Errorf("fixture %s = %q, want %q", name, got, want)
			}
		}
		return
	}
	command := exec.Command(commandTestExecutable(t), "-test.run=^TestFixtureCommandEnvironmentOwnsLandingInputs$", "-test.timeout=30m")
	command.Env = append(testenv.WithoutInheritedControls(os.Environ()), helper+"=1", "FIXTURE_LANDING_UNRELATED=kept",
		"LANDING_ONLY=", "LANDING_TREE=outer-tree", "LANDING_COMMIT=outer-commit", "LANDING_PROOF_SCOPE=full",
		"LANDING_PROOF_BASE=outer-base", "LANDING_PROOF_GROUPS=", "LANDING_PROOF_PACKAGES=outer-package", "LANDING_FUTURE_CONTROL=outer-value")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("fixture command environment inherited outer proof controls: %v\n%s", err, output)
	}
}
