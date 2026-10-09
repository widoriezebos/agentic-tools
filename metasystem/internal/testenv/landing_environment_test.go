package testenv

import (
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestMainClearsInheritedLandingEnvironment(t *testing.T) {
	t.Parallel()
	const helper = "TESTENV_LANDING_ENVIRONMENT_HELPER"
	if os.Getenv(helper) == "1" {
		for _, entry := range os.Environ() {
			if strings.HasPrefix(entry, "LANDING_") {
				t.Errorf("outer landing environment survived Main: %s", entry)
			}
		}
		if got := os.Getenv("TESTENV_LANDING_UNRELATED"); got != "kept" {
			t.Errorf("unrelated environment = %q, want kept", got)
		}
		return
	}
	command := exec.Command(os.Args[0], "-test.run=^TestMainClearsInheritedLandingEnvironment$", "-test.timeout=30m")
	command.Env = append(WithoutInheritedControls(os.Environ()), helper+"=1", "TESTENV_LANDING_UNRELATED=kept",
		"LANDING_ONLY=", "LANDING_TREE=outer-tree", "LANDING_COMMIT=outer-commit",
		"LANDING_PROOF_SCOPE=full", "LANDING_PROOF_BASE=outer-base", "LANDING_PROOF_GROUPS=",
		"LANDING_PROOF_PACKAGES=outer-package", "LANDING_FUTURE_CONTROL=outer-value")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("test process inherited outer landing environment: %v\n%s", err, output)
	}
}

func TestWithoutInheritedControlsRemovesLandingEnvironment(t *testing.T) {
	t.Parallel()
	environment := []string{"KEEP=first", "LANDING_ONLY=", "LANDING_COMMIT=outer", "LANDING_FUTURE_CONTROL=value", "KEEP_TOO=last"}
	if got, want := WithoutInheritedControls(environment), []string{"KEEP=first", "KEEP_TOO=last"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("child environment = %v, want %v", got, want)
	}
}
