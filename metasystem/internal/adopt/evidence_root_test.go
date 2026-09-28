package adopt

import (
	"os"
	"path/filepath"
	"testing"
)

func homeLookup(home string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		if name == "HOME" {
			return home, true
		}
		return "", false
	}
}

// Adoption says where evidence goes: the default for a target carrying the
// template's placeholder, the .local root when the target sets one.
func TestAdoptionNotesTheEvidenceRoot(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	target := filepath.Join(t.TempDir(), "adopted-app")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(target, "metasystem.conf")
	if err := os.WriteFile(conf, []byte("evidence.root=<durable evidence root, outside the repository>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := "evidence root: " + filepath.Join(home, "metasystem-evidence", "adopted-app") + " (default; set evidence.root in metasystem.conf.local to change)"
	if got := evidenceRootNote(target, homeLookup(home)); got != want {
		t.Fatalf("placeholder: got %q; want %q", got, want)
	}
	own := t.TempDir()
	if err := os.WriteFile(conf+".local", []byte("evidence.root="+own+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := evidenceRootNote(target, homeLookup(home)); got != "evidence root: "+own+" (metasystem.conf.local)" {
		t.Fatalf(".local: got %q", got)
	}
}
