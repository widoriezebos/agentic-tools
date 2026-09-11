package proofrun

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveAndFrozenContextIdentity(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), []byte("module example.invalid/context\n\ngo 1.27.0\n"), 0o644)
	writeTestFile(t, filepath.Join(root, "metasystem.conf"), []byte("suite.progress-silence-min=2\n"), 0o644)
	writeTestFile(t, filepath.Join(root, "internal", "sample.go"), []byte("package internal\n"), 0o644)
	frozen, err := Freeze(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(frozen.Root)) })

	readonly := replaceContextEnvironment(os.Environ(), map[string]string{
		"GOFLAGS": "-mod=readonly", "METASYSTEM_GATE_FROZEN_TOOLCHAIN": "1",
	})
	source, err := CaptureExecutionContext(root, filepath.Join(root, "metasystem.conf"), readonly)
	if err != nil {
		t.Fatal(err)
	}
	export, err := CaptureExecutionContext(frozen.Root, filepath.Join(frozen.Root, "metasystem.conf"), readonly)
	if err != nil {
		t.Fatal(err)
	}
	sourceIdentity := BuildProofIdentityForContext(source, "full", "context-fixture", []string{"gate"}, 2)
	exportIdentity := BuildProofIdentityForContext(export, "full", "context-fixture", []string{"gate"}, 2)
	if sourceIdentity.IdentityDigest != exportIdentity.IdentityDigest {
		t.Fatalf("frozen export identity %s differs from admitted source %s", exportIdentity.IdentityDigest, sourceIdentity.IdentityDigest)
	}

	ambient := replaceContextEnvironment(readonly, map[string]string{"GOFLAGS": ""})
	differentEnvironment, err := CaptureExecutionContext(frozen.Root, filepath.Join(frozen.Root, "metasystem.conf"), ambient)
	if err != nil {
		t.Fatal(err)
	}
	differentIdentity := BuildProofIdentityForContext(differentEnvironment, "full", "context-fixture", []string{"gate"}, 2)
	if differentIdentity.IdentityDigest == sourceIdentity.IdentityDigest {
		t.Fatal("same source under a different effective Go environment retained proof identity")
	}
}

func TestPreparedEnvironmentSelectsTheMeasuredGoExecutable(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	goPath := filepath.Join(bin, "go")
	body := "#!/bin/sh\ncase \"$1\" in version) printf 'prepared-go\\n';; env) printf 'prepared-env:%s\\n' \"$*\";; *) exit 9;; esac\n"
	if err := os.WriteFile(goPath, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	environment := []string{"PATH=" + bin, "GOFLAGS=-mod=readonly"}
	got, err := CompleteToolchainIdentityAtWithEnvironment(root, environment)
	if err != nil {
		t.Fatal(err)
	}
	wantBytes := []byte("prepared-go\nprepared-env:env GOOS GOARCH GOFLAGS GOWORK GOEXPERIMENT CGO_ENABLED GOTOOLCHAIN\n")
	wantHash := sha256.Sum256(wantBytes)
	if want := hex.EncodeToString(wantHash[:]); got != want {
		t.Fatalf("toolchain identity used bytes outside prepared PATH: got %s want %s", got, want)
	}
}

func TestToolchainClosureIdentityBindsExecutableBytesAndGoRootVersion(t *testing.T) {
	root := t.TempDir()
	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	baseEnvironment := replaceContextEnvironment(os.Environ(), map[string]string{"PATH": filepath.Dir(goPath) + string(os.PathListSeparator) + os.Getenv("PATH")})
	realIdentity, err := ToolchainClosureIdentity(root, baseEnvironment)
	if err != nil {
		t.Fatal(err)
	}
	versionOutput, err := exec.Command(goPath, "version").Output()
	if err != nil {
		t.Fatal(err)
	}
	environmentOutput, err := exec.Command(goPath, "env", "GOOS", "GOARCH", "GOFLAGS", "GOWORK", "GOEXPERIMENT", "CGO_ENABLED", "GOTOOLCHAIN").Output()
	if err != nil {
		t.Fatal(err)
	}
	realGoRoot, err := exec.Command(goPath, "env", "GOROOT").Output()
	if err != nil {
		t.Fatal(err)
	}
	realVersion, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(realGoRoot)), "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	fixtureRoot := t.TempDir()
	goRoot := filepath.Join(fixtureRoot, "goroot")
	writeTestFile(t, filepath.Join(goRoot, "VERSION"), realVersion, 0o644)
	versionPath := filepath.Join(fixtureRoot, "version.out")
	environmentPath := filepath.Join(fixtureRoot, "environment.out")
	writeTestFile(t, versionPath, versionOutput, 0o644)
	writeTestFile(t, environmentPath, environmentOutput, 0o644)
	script := []byte("#!/bin/sh\ncase \"$*\" in\n  version) /bin/cat \"$FIXTURE_VERSION_OUTPUT\" ;;\n  \"env GOOS GOARCH GOFLAGS GOWORK GOEXPERIMENT CGO_ENABLED GOTOOLCHAIN\") /bin/cat \"$FIXTURE_ENV_OUTPUT\" ;;\n  \"env GOROOT\") printf '%s\\n' \"$FIXTURE_GOROOT\" ;;\n  *) exit 9 ;;\nesac\n")
	fakeIdentity := func(bin string) (string, error) {
		if err := os.MkdirAll(bin, 0o755); err != nil {
			return "", err
		}
		writeTestFile(t, filepath.Join(bin, "go"), script, 0o755)
		environment := replaceContextEnvironment(baseEnvironment, map[string]string{
			"PATH":                   bin + string(os.PathListSeparator) + os.Getenv("PATH"),
			"FIXTURE_VERSION_OUTPUT": versionPath,
			"FIXTURE_ENV_OUTPUT":     environmentPath,
			"FIXTURE_GOROOT":         goRoot,
		})
		return ToolchainClosureIdentity(root, environment)
	}
	first, err := fakeIdentity(filepath.Join(fixtureRoot, "first-bin"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := fakeIdentity(filepath.Join(fixtureRoot, "second-bin"))
	if err != nil {
		t.Fatal(err)
	}
	if first == realIdentity {
		t.Fatal("different selected Go executable bytes retained the real toolchain closure identity")
	}
	if first != second {
		t.Fatalf("identical selected Go bytes depended on their path: first=%s second=%s", first, second)
	}
	if err := os.Remove(filepath.Join(goRoot, "VERSION")); err != nil {
		t.Fatal(err)
	}
	if _, err := fakeIdentity(filepath.Join(fixtureRoot, "third-bin")); err == nil {
		t.Fatal("toolchain closure accepted a missing Go root VERSION file")
	}
}

func TestFreezeBindsEveryExportedNonRuntimeInput(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "internal", "engine.go"), []byte("engine"), 0o644)
	writeTestFile(t, filepath.Join(root, "application", "fixture.txt"), []byte("application"), 0o644)
	frozen, err := Freeze(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(frozen.Root)) })
	full, err := FullDigest(frozen.Root)
	if err != nil {
		t.Fatal(err)
	}
	if frozen.Digest != full {
		t.Fatalf("frozen digest %s does not bind the complete exported input set %s", frozen.Digest, full)
	}
	legacy, err := Digest(frozen.Root)
	if err != nil {
		t.Fatal(err)
	}
	if legacy == frozen.Digest {
		t.Fatal("fixture did not distinguish the legacy engine projection from the full exported source")
	}
}

func replaceContextEnvironment(source []string, replacements map[string]string) []string {
	result := make([]string, 0, len(source)+len(replacements))
	seen := make(map[string]bool, len(replacements))
	for _, entry := range source {
		name := entry
		for index, char := range entry {
			if char == '=' {
				name = entry[:index]
				break
			}
		}
		if value, replace := replacements[name]; replace {
			if !seen[name] {
				result = append(result, name+"="+value)
				seen[name] = true
			}
			continue
		}
		result = append(result, entry)
	}
	for name, value := range replacements {
		if !seen[name] {
			result = append(result, name+"="+value)
		}
	}
	return result
}
