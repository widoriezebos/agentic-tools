package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeFrozenCopy writes a frozen copy of content under dir and returns its
// absolute path and SHA-256.
func writeFrozenCopy(t *testing.T, dir, name, content string) (string, string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(content))
	return path, hex.EncodeToString(digest[:])
}

// TestBriefAuthorityAdmitsAFrozenDraft: a path the delegate's base tree does
// not hold is admitted only when the brief names a frozen copy of it that
// exists inside the dispatcher's checkout with exactly the recorded SHA-256;
// a path present nowhere, a copy whose bytes changed, a missing copy and a
// copy outside the checkout all still refuse.
func TestBriefAuthorityAdmitsAFrozenDraft(t *testing.T) {
	t.Parallel()
	repo := newBriefAuthorityRepo(t)
	draft := "records/drafts/page.md"
	copyPath, digest := writeFrozenCopy(t, repo.root, "artifacts/agents/intent-review/frozen/page.md", "the seat's draft\n")
	frozen := FrozenInputLine(draft, digest, copyPath)
	brief := func(lines ...string) string {
		return writeBriefAuthorityFile(t, repo.root, "brief.md", strings.Join(append([]string{"Working Mode: design-critique", "Read `" + draft + "`."}, lines...), "\n")+"\n")
	}
	if err := briefAuthorityError(brief(frozen), repo.root, repo.facts); err != nil {
		t.Fatalf("a draft frozen into the critic's input was refused: %v", err)
	}
	wantMissing := func(name, content string, want []string) {
		t.Helper()
		err := briefAuthorityError(brief(content), repo.root, repo.facts)
		var refusal *BriefAuthorityRefusal
		if !errors.As(err, &refusal) || !reflect.DeepEqual(refusal.MissingPaths, want) {
			t.Fatalf("%s: authority error = %v, want missing %v", name, err, want)
		}
	}
	// Present nowhere: no frozen copy, and a frozen copy of another path.
	wantMissing("not frozen", "", []string{draft})
	wantMissing("present nowhere", frozen+"\nRead `records/drafts/absent.md` too.", []string{"records/drafts/absent.md"})
	// A copy that does not hold the recorded bytes.
	_, otherDigest := writeFrozenCopy(t, t.TempDir(), "x", "other bytes\n")
	wantMissing("changed copy", FrozenInputLine(draft, otherDigest, copyPath), []string{draft})
	wantMissing("missing copy", FrozenInputLine(draft, digest, copyPath+".gone"), []string{draft})
	wantMissing("relative copy", FrozenInputLine(draft, digest, "artifacts/agents/intent-review/frozen/page.md"), []string{draft})
	outside, outsideDigest := writeFrozenCopy(t, t.TempDir(), "page.md", "the seat's draft\n")
	wantMissing("copy outside the checkout", FrozenInputLine(draft, outsideDigest, outside), []string{draft})
	wantMissing("malformed line", "Frozen input: "+draft+" "+digest, []string{draft})
}
