package lane

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// laneCheckout makes dir a git checkout holding a MetaSystem installation:
// nested (dir/metasystem/metasystem.conf, checkout root ≠ installation
// root) or flat (dir/metasystem.conf).
func laneCheckout(t *testing.T, dir string, nested bool) {
	t.Helper()
	command := exec.Command("git", "init", "-q", dir)
	command.Env = gittree.ScrubbedEnviron()
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v: %s", dir, err, out)
	}
	install := dir
	if nested {
		install = filepath.Join(dir, "metasystem")
	}
	if err := os.MkdirAll(install, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

// register makes root the host's lane at a person's word.
func register(t *testing.T, home, root string) Record {
	t.Helper()
	layout, err := NewLayout(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Register(home, layout, "m1e", laneNow); err != nil {
		t.Fatal(err)
	}
	record, ok, err := Read(home)
	if err != nil || !ok {
		t.Fatalf("record after registering %s = %v %v", root, ok, err)
	}
	return record
}

// One layout value (r6 U1, kept by r10 K-a): landing set resolves the
// checkout and the installation once, in the nested and the flat shape, and
// a candidate's detached worktree finds its installation at the same place
// relative to its checkout. Every other shape is refused and registers
// nothing.
func TestLayoutNestedFlatAndDetached(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	nested, flat := filepath.Join(resolved(base), "nested"), filepath.Join(resolved(base), "flat")
	laneCheckout(t, nested, true)
	laneCheckout(t, flat, false)
	layout, err := NewLayout(nested)
	if err != nil || string(layout.Checkout) != nested || string(layout.Install) != filepath.Join(nested, "metasystem") {
		t.Fatalf("nested layout = %+v %v", layout, err)
	}
	if got := layout.Execution("/work/candidate"); got != "/work/candidate/metasystem" {
		t.Fatalf("nested execution root = %s", got)
	}
	layout, err = NewLayout(flat)
	if err != nil || string(layout.Checkout) != flat || string(layout.Install) != flat {
		t.Fatalf("flat layout = %+v %v", layout, err)
	}
	if got := layout.Execution("/work/candidate"); got != "/work/candidate" {
		t.Fatalf("flat execution root = %s", got)
	}

	both := filepath.Join(resolved(base), "both")
	laneCheckout(t, both, true)
	if err := os.WriteFile(filepath.Join(both, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	neither := filepath.Join(resolved(base), "neither")
	laneCheckout(t, neither, false)
	if err := os.Remove(filepath.Join(neither, "metasystem.conf")); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(resolved(base), "plain")
	if err := os.MkdirAll(filepath.Join(plain, "metasystem"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plain, "metasystem", "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{
		"a folder inside a checkout": filepath.Join(nested, "metasystem"),
		"both shapes at once":        both,
		"no installation":            neither,
		"not a git checkout":         plain,
		"a relative path":            "nested",
		"a missing folder":           filepath.Join(base, "missing"),
	} {
		_, err := NewLayout(path)
		var refusal *Refusal
		if !errors.As(err, &refusal) || refusal.Code != CodeRegisterInvalid {
			t.Errorf("%s (%s) = %v; want %s", name, path, err, CodeRegisterInvalid)
		}
	}

	home := filepath.Join(base, "home")
	record := register(t, home, nested)
	if record.Root != nested || record.Install != filepath.Join(nested, "metasystem") {
		t.Fatalf("record = %+v; want the layout recorded", record)
	}
	recorded, err := record.Layout()
	if err != nil || recorded != (Layout{Checkout: CheckoutRoot(nested), Install: InstallRoot(filepath.Join(nested, "metasystem")), rel: "metasystem"}) {
		t.Fatalf("recorded layout = %+v %v", recorded, err)
	}
}

// A lane record an older engine wrote names no installation and no custody
// epoch: it is refused wherever it is read, and the refusal names the one
// command a person runs.
func TestRecordWithoutInstallIsRefused(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	if err := os.MkdirAll(HostDir(home), 0o700); err != nil {
		t.Fatal(err)
	}
	old := `{"root":"` + resolved(root) + `","registeredBy":"m1e","at":"2026-09-29T18:00:00Z"}`
	if err := os.WriteFile(RecordPath(home), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := Read(home)
	var refusal *Refusal
	if !errors.As(err, &refusal) || refusal.Code != CodeRecordIncomplete || len(refusal.Argv) != 4 || refusal.Argv[1] != "landing" || refusal.Argv[2] != "set" || refusal.Argv[3] != resolved(root) {
		t.Fatalf("read of an old record = %v; want %s naming metasystem landing set %s", err, CodeRecordIncomplete, root)
	}
	if _, err := Resolve(home, ""); !errors.As(err, &refusal) || refusal.Code != CodeRecordIncomplete {
		t.Fatalf("a seat resolving an old record = %v", err)
	}
}

// landing set bumps the lane's custody epoch (design r10 §1): every new
// registration takes a greater one, across a move and across unset, and a
// repeat of the same registration changes nothing.
func TestSetBumpsTheCustodyEpoch(t *testing.T) {
	t.Parallel()
	home, first, second := laneDirs(t)
	if record := register(t, home, first); record.CustodyEpoch != 1 {
		t.Fatalf("first registration epoch = %d", record.CustodyEpoch)
	}
	if record := register(t, home, first); record.CustodyEpoch != 1 {
		t.Fatalf("repeat epoch = %d; want unchanged", record.CustodyEpoch)
	}
	if record := register(t, home, second); record.CustodyEpoch != 2 {
		t.Fatalf("move epoch = %d", record.CustodyEpoch)
	}
	report, err := Unset(home, "Wido", laneNow, false, emptyUnsetSeams())
	if err != nil || !report.Unregistered {
		t.Fatalf("unset = %+v %v", report, err)
	}
	if record := register(t, home, first); record.CustodyEpoch != 3 {
		t.Fatalf("epoch after unset and set = %d; want 3, never an epoch used before", record.CustodyEpoch)
	}
}
