package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestHarnessPrologueMatchesShellPrologue(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate prologue source test")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "..", ".."))
	if override := os.Getenv("FIXTURE_SOURCE_ROOT"); override != "" {
		root = override
	}
	budget := filepath.Join(root, "scripts", "agents", "fixture-budget.sh")
	command := exec.Command("bash", "-c", `source "$1"; harness_fixture_prologue`, "bash", budget)
	printed, err := command.Output()
	if err != nil || string(printed) != ShellPrologue {
		t.Fatalf("harness_fixture_prologue = %q, %v; want %q", printed, err, ShellPrologue)
	}

	for _, fixture := range []struct {
		name string
		path string
	}{
		{name: "hang", path: filepath.Join(root, "scripts", "agents", "fixture-bed-scenarios-fixtures.sh")},
		{name: "stopped", path: filepath.Join(root, "scripts", "agents", "suite-progress-fixtures.sh")},
		{name: "detached", path: filepath.Join(root, "scripts", "agents", "suite-progress-fixtures.sh")},
		{name: "fake host", path: filepath.Join(root, "scripts", "agents", "hosts", "fake.sh")},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			contents, readErr := os.ReadFile(fixture.path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			expected := ShellPrologue
			if fixture.name == "fake host" {
				expected = strings.ReplaceAll(ShellPrologue, "exec /bin/sh", "exec /bin/bash")
			}
			if !strings.Contains(string(contents), expected) {
				t.Fatalf("%s prologue differs from ShellPrologue", fixture.path)
			}
		})
	}

	hang := readFixtureSource(t, filepath.Join(root, "scripts", "agents", "fixture-bed-scenarios-fixtures.sh"))
	if !strings.Contains(hang, "exec 3<\"$METASYSTEM_FIXTURE_LEASH\"\n    read -r _ <&3") {
		t.Fatal("hang fixture does not block on its leash")
	}
	suite := readFixtureSource(t, filepath.Join(root, "scripts", "agents", "suite-progress-fixtures.sh"))
	cleanupStart, cleanupEnd := strings.Index(suite, "cleanup() {"), strings.Index(suite, "trap cleanup EXIT")
	if cleanupStart < 0 || cleanupEnd <= cleanupStart {
		t.Fatal("suite-progress cleanup boundaries were not found")
	}
	cleanup := suite[cleanupStart:cleanupEnd]
	if !strings.Contains(cleanup, "harness_fixture_reap") || strings.Contains(cleanup, "kill \"$pid\"") {
		t.Fatal("suite-progress cleanup does not delegate fixture reaping")
	}
	if !strings.Contains(suite, "exec 3<\"$METASYSTEM_FIXTURE_LEASH\"\n      read -r _ <&3") {
		t.Fatal("suite-progress detached fixture does not block on its leash")
	}
	fake := readFixtureSource(t, filepath.Join(root, "scripts", "agents", "hosts", "fake.sh"))
	if !strings.Contains(fake, `exec "$ms" util hold --tag "$instance_tag"`) {
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
