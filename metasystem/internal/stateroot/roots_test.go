package stateroot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/roots"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
)

// The resolver's constructors answer with the root type they own, so a caller
// cannot hand a state root to code that expects the installation.
var (
	_ func(roots.Installation) (roots.State, error) = RootForInstallation
	_ func(Kind) (roots.State, error)               = StateRoot
	_ func() (roots.Installation, error)            = ExecutableInstallation
	_ func(string) (roots.Installation, error)      = RootForCandidate
	_ roots.Installation                            = Layout{}.InstallationRoot
)

func TestParseInstallationAcceptsOnlyADirectoryHoldingTheConfiguration(t *testing.T) {
	t.Parallel()
	installation, _ := installFixture(t, false)
	state := filepath.Join(installation, "plans")
	if err := os.MkdirAll(state, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := ParseInstallation(state); err == nil || !strings.Contains(err.Error(), "is not a metasystem installation") {
		t.Fatalf("ParseInstallation(%q) = %q, %v; want a refusal of a directory without metasystem.conf", state, got, err)
	}
	unclean := state + string(filepath.Separator) + ".." + string(filepath.Separator)
	if got, err := ParseInstallation(unclean); err != nil || got.Path() != installation {
		t.Fatalf("ParseInstallation(%q) = %q, %v; want %q", unclean, got, err, installation)
	}
}

func TestParseStateReturnsAnAbsoluteCleanPath(t *testing.T) {
	t.Parallel()
	working, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for input, want := range map[string]string{
		"plans/../memory": filepath.Join(working, "memory"),
		filepath.Join(working, "records") + "/./../plans/": filepath.Join(working, "plans"),
	} {
		if got, err := ParseState(input); err != nil || got.Path() != want {
			t.Fatalf("ParseState(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
}

func TestStateRootTypesJoinSegmentsBeneathTheirOwnRoot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	installation, state := stateroottest.Installation(t, dir+"/./"), stateroottest.State(t, filepath.Join(dir, "x", ".."))
	if installation.Path() != dir || state.Path() != dir {
		t.Fatalf("fixture roots = %q, %q; want the clean directory %q without requiring metasystem.conf", installation, state, dir)
	}
	if got, want := installation.Path("bin", "metasystem"), filepath.Join(dir, "bin", "metasystem"); got != want {
		t.Fatalf("Installation.Path = %q, want %q", got, want)
	}
	if got, want := state.Path("plans", "goals"), filepath.Join(dir, "plans", "goals"); got != want {
		t.Fatalf("State.Path = %q, want %q", got, want)
	}
}

func TestStateRootResolverFindsTheExecutableInstallation(t *testing.T) {
	t.Parallel()
	installation, _ := installFixture(t, false)
	executable := func() (string, error) { return filepath.Join(installation, "bin", "metasystem"), nil }
	resolver := NewResolver(func(string) (string, error) { t.Fatal("unexpected Git lookup"); return "", nil }, executable)
	if got, err := resolver.ExecutableInstallation(); err != nil || got.Path() != installation {
		t.Fatalf("ExecutableInstallation() = %q, %v; want %q", got, err, installation)
	}
	// The running test binary is an engine only when it sits in an
	// installation's bin directory; any answer must hold metasystem.conf.
	if got, err := ExecutableInstallation(); err == nil && !installationShape(got.Path()) {
		t.Fatalf("ExecutableInstallation() = %q, which holds no metasystem.conf", got)
	}
}
