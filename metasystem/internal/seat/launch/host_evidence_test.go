package launch

// The host's evidence-root side, on a real filesystem and without Git: the
// copy that blanks the key, the resolver's scrubbed view, and the two
// isolation fixtures of the failsafe round driven through the sequencer's own
// judgement.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

func homeOnlyEnv(home string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		if name == "HOME" {
			return home, true
		}
		return "", false
	}
}

func writeFixture(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCopyLocalConfKeepsEveryOtherLineAndBlanksTheEvidenceRoot(t *testing.T) {
	t.Parallel()
	bed := t.TempDir()
	source := filepath.Join(bed, "from", "metasystem.conf.local")
	destination := filepath.Join(bed, "to", "metasystem.conf.local")
	writeFixture(t, source, "fleet.channel.secret=s3\n"+config.EvidenceRootKey+"=/w/evidence/m1u\nrole.default.runtime=fake\n")
	if err := (OSHost{}).CopyLocalConf(source, destination); err != nil {
		t.Fatal(err)
	}
	copied, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	text := string(copied)
	if !strings.Contains(text, "fleet.channel.secret=s3\n") || !strings.Contains(text, "role.default.runtime=fake\n") ||
		strings.Contains(text, "/w/evidence/m1u") || !strings.Contains(text, config.EvidenceRootKey+"=\n") {
		t.Fatalf("copied conf =\n%s", text)
	}
}

func TestOSHostEvidenceRootNamesTheDefaultUnderHome(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	checkout := filepath.Join(t.TempDir(), "agentic-tools-m1f")
	writeFixture(t, filepath.Join(checkout, ".git"), "gitdir: /elsewhere\n")
	writeFixture(t, filepath.Join(checkout, "metasystem", "metasystem.conf"), "x=1\n")
	got, err := (OSHost{Env: homeOnlyEnv(home)}).EvidenceRoot(filepath.Join(checkout, "metasystem"))
	if err != nil || got.Path != filepath.Join(home, "metasystem-evidence", "agentic-tools-m1f") || !got.Default() {
		t.Fatalf("got %+v, %v", got, err)
	}
}

// evidenceBed is two installations, a source seat and its clone, each a
// checkout with a .git file and a metasystem/ installation.
func evidenceBed(t *testing.T, home, sourceLocal, cloneCommitted string) *Sequencer {
	t.Helper()
	bed := t.TempDir()
	from, to := filepath.Join(bed, "agentic-tools"), filepath.Join(bed, "agentic-tools-m1f")
	for _, checkout := range []string{from, to} {
		writeFixture(t, filepath.Join(checkout, ".git"), "gitdir: /elsewhere\n")
		writeFixture(t, filepath.Join(checkout, "metasystem", "metasystem.conf"), "x=1\n")
	}
	writeFixture(t, filepath.Join(from, "metasystem", "metasystem.conf.local"), sourceLocal)
	writeFixture(t, filepath.Join(to, "metasystem", "metasystem.conf"), cloneCommitted)
	return &Sequencer{
		Request:      Request{Machine: machineName, From: from, Destination: to},
		Installation: install, Host: OSHost{Env: homeOnlyEnv(home)},
	}
}

// ERD-02: a source root under a linked parent and a clone root under its
// target are one directory even while neither leaf exists.
func TestLaunchRefusesMissingSourceRootAlias(t *testing.T) {
	t.Parallel()
	bed, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(bed, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(bed, "alias")); err != nil {
		t.Fatal(err)
	}
	s := evidenceBed(t, t.TempDir(),
		config.EvidenceRootKey+"="+filepath.Join(bed, "alias", "new")+"\n",
		config.EvidenceRootKey+"="+filepath.Join(real, "new")+"\n")
	record := fresh()
	record.Destination = s.Request.Destination
	_, err = s.evidenceRoot(&record)
	if refusal, named := err.(*Refusal); !named || refusal.Code != CodeEvidenceRootUnsafe || !strings.Contains(refusal.Message, "this seat's own root") {
		t.Fatalf("error = %v, want %s", err, CodeEvidenceRootUnsafe)
	}
	if _, err := os.Lstat(filepath.Join(real, "new")); !os.IsNotExist(err) {
		t.Fatalf("the aliased root was created: %v", err)
	}
}

// ERD-06: the first launch on a host with no metasystem-evidence directory
// makes the clone's default root, parents included; an occupied leaf this
// launch did not make is still refused.
func TestLaunchCreatesDefaultRootWithoutEvidenceParent(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	s := evidenceBed(t, home, config.EvidenceRootKey+"="+filepath.Join(t.TempDir(), "m1u")+"\n", "x=1\n")
	record := fresh()
	record.Destination = s.Request.Destination
	root, err := s.evidenceRoot(&record)
	want := filepath.Join(home, "metasystem-evidence", "agentic-tools-m1f")
	if err != nil || root.Path != want || !record.Created.EvidenceRoot {
		t.Fatalf("root = %+v, %v, created %+v", root, err, record.Created)
	}
	if info, err := os.Stat(want); err != nil || !info.IsDir() {
		t.Fatalf("the default root was not created: %v", err)
	}
	again := fresh()
	again.Destination = s.Request.Destination
	_, err = s.evidenceRoot(&again)
	if refusal, named := err.(*Refusal); !named || refusal.Code != CodeEvidenceRootUnsafe || !strings.Contains(refusal.Message, "already there and this launch did not create it") {
		t.Fatalf("an occupied leaf: error = %v", err)
	}
}
