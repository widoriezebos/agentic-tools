package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHarnessPrologueMatchesShellPrologue(t *testing.T) {
	root := MustSourceRoot(t)
	budget := filepath.Join(root, "scripts", "agents", "fixture-budget.sh")
	command := exec.Command("bash", "-c", `source "$1"; harness_fixture_prologue`, "bash", budget)
	printed, err := command.Output()
	if err != nil || string(printed) != ShellPrologue {
		t.Fatalf("harness_fixture_prologue = %q, %v; want %q", printed, err, ShellPrologue)
	}

	// The hang scenario's bed is the Go-held Bash source the fixture bed
	// tests run; it re-executes through the Bash form of the prologue.
	if !strings.Contains(fixtureBedInnerBed, strings.ReplaceAll(ShellPrologue, "exec /bin/sh", "exec /bin/bash")) {
		t.Fatal("fixture bed inner bed prologue differs from ShellPrologue")
	}
	if !strings.Contains(fixtureBedInnerBed, "exec 3<\"$METASYSTEM_FIXTURE_LEASH\"\n    read -r _ <&3") {
		t.Fatal("hang fixture does not block on its leash")
	}
	// The fake host's hold (formerly hosts/fake.sh's `exec "$ms" util hold`)
	// replaces the host process with the engine's leash-bound hold.
	fake := readFixtureSource(t, filepath.Join(root, "internal", "adapter", "supervisor", "fake_host.go"))
	if !strings.Contains(fake, `syscall.Exec(d.Engine, append([]string{d.Engine, "util", "hold"}, flags...)`) || !strings.Contains(fake, `"--tag", t.Tag`) {
		t.Fatal("fake host does not delegate its leash-bound lifetime to util hold")
	}
}

func readFixtureSource(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}
