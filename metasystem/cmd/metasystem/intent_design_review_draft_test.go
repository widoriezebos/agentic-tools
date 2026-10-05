package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

// frozenCopies reads a brief's frozen-input lines as cited path -> the bytes
// of the copy the critic is handed.
func frozenCopies(t *testing.T, brief string) map[string]string {
	t.Helper()
	copies := map[string]string{}
	for _, line := range strings.Split(brief, "\n") {
		rest, found := strings.CutPrefix(line, "Frozen input: ")
		if !found {
			continue
		}
		name, rest, _ := strings.Cut(rest, " sha256:")
		_, copyPath, _ := strings.Cut(rest, " ")
		copies[name] = string(mustRead(t, copyPath))
	}
	return copies
}

// TestDesignReviewFreezesTheSeatsDraft (item 59): a seat's design page and
// the brief it cites exist only in the seat's checkout, not on HEAD. design
// review admits the critic through the real brief-authority admission and
// hands it exactly the bytes the seat had when it asked, frozen, so a later
// edit in the checkout does not reach the critic; a follow-up whose decisions
// cite a further draft is admitted the same way.
func TestDesignReviewFreezesTheSeatsDraft(t *testing.T) {
	t.Parallel()
	b := newDesignReviewBed(t)
	b.owners.draftPaths = dispatchcore.BriefDraftPaths
	root, err := filepath.EvalSymlinks(b.root())
	if err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		full := append([]string{"-C", root, "-c", "user.name=bed", "-c", "user.email=bed@example.invalid", "-c", "commit.gpgsign=false"}, args...)
		if output, err := exec.Command("git", full...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	// HEAD holds the plans folder but neither draft.
	git("init", "-q")
	designs := filepath.Dir(b.design)
	keep := filepath.Join(designs, ".keep")
	b.writeFile(keep, "")
	git("add", "-f", "--", keep)
	git("commit", "-q", "-m", "base")
	rel := func(path string) string {
		t.Helper()
		resolved, err := filepath.EvalSymlinks(filepath.Dir(path))
		if err != nil {
			t.Fatal(err)
		}
		out, err := filepath.Rel(root, filepath.Join(resolved, filepath.Base(path)))
		if err != nil {
			t.Fatal(err)
		}
		return filepath.ToSlash(out)
	}
	brief := filepath.Join(filepath.Dir(designs), "reader-brief.md")
	b.writeFile(brief, "# Brief for the reader design\n\nThe seat's draft brief.\n")
	page := "# Reader\n\n- Kind: design\n- Id: 01DESIGNREADER\n- Status: draft\n- Goals: standing-validation\n\nFirst version, written from `" + rel(brief) + "`.\n"
	b.writeFile(b.design, page)

	var admitted []error
	var briefs []string
	dispatch := b.handler
	b.handler = func(process intentProcess) intentProcessResult {
		if path := flagValue(process.argv, "--brief"); path != "" {
			_, err := dispatchcore.ReadBriefAdmissionAtRoot(path, b.install, root, root, false)
			admitted = append(admitted, err)
			briefs = append(briefs, string(mustRead(t, path)))
			if err != nil {
				return intentProcessResult{stdout: []byte(`{"outcome":"REFUSED-BRIEF","detail":"brief authority admission refused"}`), code: 1}
			}
		}
		return dispatch(process)
	}
	review := func(extra ...string) intentResult {
		t.Helper()
		_, result := b.do(append([]string{"design", "review", b.design, "--tool-calls", "30"}, extra...)...)
		return result
	}
	if result := review(); result.Outcome != intentInProgress || b.fresh != 1 || len(admitted) != 1 || admitted[0] != nil {
		t.Fatalf("a draft design review: %+v admitted=%v", result, admitted)
	}
	copies := frozenCopies(t, briefs[0])
	if copies[rel(b.design)] != page || copies[rel(brief)] != string(mustRead(t, brief)) {
		t.Fatalf("the critic is not handed the seat's bytes: %v\n%s", copies, briefs[0])
	}
	if !strings.Contains(briefs[0], "Path: ") || strings.Contains(briefs[0], "Path: "+b.design+"\n") {
		t.Fatalf("the prepared copy is the moving checkout, not the frozen page:\n%s", briefs[0])
	}
	// The seat keeps editing: the critic's copy keeps the bytes it was handed.
	b.writeFile(brief, "# Brief, edited after the review was asked\n")
	if again := frozenCopies(t, briefs[0]); again[rel(brief)] != "# Brief for the reader design\n\nThe seat's draft brief.\n" {
		t.Fatalf("a later checkout edit reached the frozen copy: %q", again[rel(brief)])
	}

	// Round 1's finding is answered by a revision whose decisions cite a
	// second draft brief; the follow-up is admitted with it frozen.
	b.finish("rev1", 1, "completed", map[string]any{"id": "F1", "severity": "critical", "material": true})
	review()
	b.writeFile(b.design, strings.Replace(page, "First version", "Second version, F1 addressed", 1))
	template, _ := review().Data.(map[string]any)["template"].(string)
	body := string(mustRead(t, template))
	second := filepath.Join(filepath.Dir(designs), "reader-brief-r2.md")
	b.writeFile(second, "# Revision brief\n")
	decided := filepath.Join(b.root(), "decided.md")
	b.writeFile(decided, strings.Replace(body, "| F1 | DECIDE | | |", "| F1 | accepted | answered in `"+rel(second)+"` | section 2 |", 1))
	if result := review("--dispositions", decided); len(b.followUps) != 1 || len(admitted) != 2 || admitted[1] != nil {
		t.Fatalf("the follow-up citing a draft brief: %+v admitted=%v", result, admitted)
	}
	if copies := frozenCopies(t, briefs[1]); copies[rel(second)] != "# Revision brief\n" {
		t.Fatalf("the follow-up does not hand the critic the draft revision brief: %v\n%s", copies, briefs[1])
	}
}
