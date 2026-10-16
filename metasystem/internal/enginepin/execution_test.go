package enginepin

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin/testdata/pinfixture"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

type probeFunc func(int64) (identity.Exact, identity.Liveness, error)

func (f probeFunc) Probe(pid int64) (identity.Exact, identity.Liveness, error) { return f(pid) }

func TestReadExecutionRetainsVerifiedPinIdentity(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path, digest := pinfixture.Pin(t, root, 147, "main-source")
	exact := pinfixture.Process(41, path)
	got := Read(root, exact.Ref(), pinfixture.Prober{41: exact})
	if got.Digest != digest || got.Generation != 147 || got.Source != "main-source" || got.Cause != "" {
		t.Fatalf("execution = %+v, want digest %s, generation 147 and source main-source", got, digest)
	}
}

func TestReadExecutionUnknownEvidence(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"missing", "uninspectable", "mismatch", "reused", "no-executable", "dead", "changed", "not-pin", "not-executable"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			path, _ := pinfixture.Pin(t, root, 7, "source-seven")
			exact := pinfixture.Process(41, path)
			ref := exact.Ref()
			cause := "identity"
			calls := 0
			prober := probeFunc(func(int64) (identity.Exact, identity.Liveness, error) {
				calls++
				if kind == "uninspectable" {
					return identity.Exact{}, identity.Unknown, errors.New("probe denied")
				}
				if kind == "dead" {
					return identity.Exact{}, identity.Dead, nil
				}
				if kind == "changed" && calls == 2 {
					return identity.Exact{}, identity.Unknown, errors.New("probe lost")
				}
				return exact, identity.Alive, nil
			})
			switch kind {
			case "missing":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				cause = "no such file"
			case "uninspectable":
				cause = "probe denied"
			case "mismatch":
				if err := os.Chmod(path, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := testexec.WriteFile(path, []byte("different bytes"), 0o500); err != nil {
					t.Fatal(err)
				}
				if err := testexec.Locked(func() error { return os.Chmod(path, 0o500) }); err != nil {
					t.Fatal(err)
				}
				cause = "filename/bytes mismatch"
			case "reused":
				exact.Pid++
			case "no-executable":
				exact.ExeKnown = false
			case "changed":
				cause = "during observation"
			case "not-pin":
				exact.Exe = filepath.Join(root, "engine")
				cause = "not an engine pin"
			case "not-executable":
				if err := os.Chmod(path, 0o400); err != nil {
					t.Fatal(err)
				}
				cause = "executable regular file"
			}
			got := Read(root, ref, prober)
			if got.Digest != "" || !strings.Contains(got.Cause, cause) {
				t.Fatalf("%s: execution = %+v, want UNKNOWN with %q", kind, got, cause)
			}
		})
	}
}
