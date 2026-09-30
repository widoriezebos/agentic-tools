package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// supAOperatorLayout builds the ordinary operator layout of the retired
// supervision bed's operator-layout scenario: an application repository whose
// vendored metasystem installation sits one directory below the Git toplevel.
// The installation carries a configuration listing the fake runtime and a
// standing enrollment of the engine that runs this test (whose registry holds
// the runtime signatures), so up reaches the session-identity step.
func supAOperatorLayout(t *testing.T) (scope, installation string) {
	t.Helper()
	scope = t.TempDir()
	installation = filepath.Join(scope, "metasystem")
	if err := os.MkdirAll(installation, 0o755); err != nil {
		t.Fatal(err)
	}
	if marker, err := os.OpenFile(filepath.Join(installation, "metasystem.conf"), os.O_CREATE|os.O_WRONLY, 0o644); err != nil {
		t.Fatal(err)
	} else {
		marker.Close()
	}
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.runtimes=fake\nrole.default.model.fake=fake-model\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	canonicalScope, err := canonicalPath(scope)
	if err != nil {
		t.Fatal(err)
	}
	canonicalInstallation, err := canonicalPath(installation)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	engine, err = canonicalPath(engine)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(engine)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	identity := steward.InstallIdentity{
		RepoIdentity: canonicalInstallation, Generation: 1, InstallPath: engine,
		InstallDigest: fmt.Sprintf("sha256:%x", digest[:]), MintedAt: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
	}
	if err := os.MkdirAll(filepath.Dir(steward.RepoIdentityPath(canonicalInstallation)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := steward.MintIdentity(steward.RepoIdentityPath(canonicalInstallation), identity); err != nil {
		t.Fatal(err)
	}
	return canonicalScope, canonicalInstallation
}

// Ported from supervision-fixtures.sh operator-layout: a plain shell in the
// nested operator layout, with no runtime-signature ancestor, must refuse to
// arm, and the refusal names both supported ways forward. Nothing is written
// at the Git repository scope, and the refusal leaves no announcement or
// supervision state at the installation either.
func TestSupervisionBedAOperatorLayoutRefusesAnUnprovenSession(t *testing.T) {
	t.Setenv("METASYSTEM_AGENT_RUNTIME", "fake")
	t.Setenv("METASYSTEM_FAKE_AGENT_ANCESTOR_PID", "")
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", "")
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", "")
	scope, installation := supAOperatorLayout(t)
	repositoryTop := declaredRepositoryTop(t, scope, map[string]int{installation: 1})

	stdout, stderr, code := captureRelay(t, func(stdout, stderr io.Writer) int {
		return runUpWith([]string{"--metasystem-root", installation, "--repo", installation}, repositoryTop, stdout, stderr)
	})
	if code == 0 {
		t.Fatalf("nested operator identity inference unexpectedly succeeded: stdout=%s stderr=%s", stdout, stderr)
	}
	for _, want := range []string{
		"component=session-identity outcome=failed",
		"could not be traced back to its session",
		"pass --pid <session-pid> and --start-time <epoch-seconds>",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("nested operator identity refusal lacks %q: stdout=%s stderr=%s", want, stdout, stderr)
		}
	}
	for _, forbidden := range []string{"has no signature adapters", "No such file or directory"} {
		if strings.Contains(stdout+stderr, forbidden) {
			t.Errorf("nested operator refusal resolved its adapters from the wrong root (%q): stdout=%s stderr=%s", forbidden, stdout, stderr)
		}
	}
	if _, err := os.Stat(filepath.Join(scope, "artifacts")); !os.IsNotExist(err) {
		t.Errorf("nested operator refusal wrote state at the Git repository scope: %v", err)
	}
	for _, path := range []string{
		filepath.Join(installation, "artifacts", "agents", "mains"),
		filepath.Join(installation, "artifacts", "agents", "supervision", "lock.d"),
		filepath.Join(installation, "artifacts", "agents", "supervision", "last-census.json"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("nested operator refusal left arming state %s: %v", path, err)
		}
	}
}
