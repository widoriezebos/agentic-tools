package plain

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// stubGit answers the git calls a proof makes, with no repository: HEAD is
// the commit and tree given, a worktree add makes the worktree's folder (or
// fails with addErr), a remove deletes it, and a diff between two trees
// answers the paths changed between them.
type stubGit struct {
	commit, tree     string
	addErr           error
	changed          map[[2]string]string
	batches          map[string]string
	onMain           map[string]bool
	shows            map[string]string
	diffErr, showErr error
	installPrefix    string
}

func (g stubGit) run(dir string, args ...string) (string, error) {
	switch {
	case len(args) == 3 && args[0] == "rev-parse" && args[2] == "HEAD^{commit}":
		return g.commit, nil
	case len(args) == 3 && args[0] == "rev-parse" && args[2] == "HEAD^{tree}":
		return g.tree, nil
	case len(args) == 5 && args[0] == "worktree" && args[1] == "add":
		if g.addErr != nil {
			return "", g.addErr
		}
		return "", os.MkdirAll(filepath.Join(args[3], g.installPrefix), 0o755)
	case len(args) == 4 && args[0] == "worktree" && args[1] == "remove":
		return "", os.RemoveAll(args[3])
	case len(args) >= 2 && args[0] == "worktree" && (args[1] == "list" || args[1] == "prune"):
		return "", nil
	case len(args) == 5 && strings.Join(args[:3], " ") == "diff --name-only --no-renames":
		return g.changed[[2]string{args[3], args[4]}], g.diffErr
	case len(args) == 4 && args[0] == "rev-list" && args[1] == "--no-merges" && args[3] == "^origin/main":
		return g.batches[args[2]], nil
	case len(args) == 4 && args[0] == "merge-base" && args[1] == "--is-ancestor" && args[3] == "origin/main":
		if g.onMain[args[2]] {
			return "", nil
		}
		return "", errors.New("commit is not on main")
	case len(args) == 2 && args[0] == "show":
		return g.shows[args[1]], g.showErr
	}
	return "", fmt.Errorf("git %s is not stubbed", strings.Join(args, " "))
}

// proveStubbed runs a proof of the stubbed HEAD with command, the lane
// checkout being its installation, and returns the result recorded.
func proveStubbed(t *testing.T, install string, git stubGit, command string) Result {
	t.Helper()
	var output bytes.Buffer
	result, err := Run(install, install, command, "", &output, ProveSeams{Now: func() time.Time { return bedNow }, Git: git.run})
	if err != nil {
		t.Fatalf("prove: %v (%s)", err, output.String())
	}
	last, ok, err := LastResult(install)
	if err != nil || !ok || !reflect.DeepEqual(last, result) {
		t.Fatalf("the result recorded is not the result returned: %+v %v %v, returned %+v", last, ok, err, result)
	}
	return result
}

// A red proof says why it is red: the proof command's own exit, how it
// ended when a signal ended it, or why it could not run at all. Nothing is
// parsed from the log. A green one says nothing.
func TestARedProofRecordsWhyItIsRed(t *testing.T) {
	t.Parallel()
	git := stubGit{commit: "c0ffee1234567890abcdef", tree: "7ee1234567890abcdef"}
	cases := []struct {
		name, command string
		git           stubGit
		result        string
		reason        string
	}{
		{"exit 3", "exit 3", git, Red, "the proving command exited 3"},
		{"killed", "kill -KILL $$", git, Red, "the proving command ended: signal: killed"},
		{"no worktree", "exit 0", stubGit{commit: git.commit, tree: git.tree, addErr: errors.New("fatal: invalid reference")}, Red,
			"the worktree of commit c0ffee123456 could not be made: fatal: invalid reference"},
		{"green", "exit 0", git, Green, ""},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			t.Parallel()
			result := proveStubbed(t, t.TempDir(), each.git, each.command)
			if result.Result != each.result || result.Reason != each.reason {
				t.Fatalf("want %s with reason %q, got %+v", each.result, each.reason, result)
			}
		})
	}
}

// A tree that differs from a green tree only in goal ledger files inherits
// that green, and says so (703ecb830): the red reason is set only where the
// proof command ran and failed, so it never replaces this one, and the
// command does not run.
func TestAnInheritedGreenKeepsItsReason(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	first := stubGit{commit: "c1c1c1c1c1c1c1c1", tree: "a1a1a1a1a1a1a1a1a1"}
	if result := proveStubbed(t, install, first, "exit 0"); result.Result != Green || result.Reason != "" {
		t.Fatalf("the first proof: %+v", result)
	}
	ledgerOnly := stubGit{commit: "c2c2c2c2c2c2c2c2", tree: "b2b2b2b2b2b2b2b2b2",
		changed: map[[2]string]string{{first.tree, "b2b2b2b2b2b2b2b2b2"}: "metasystem/plans/goals/goal-a.md"}}
	result := proveStubbed(t, install, ledgerOnly, "exit 1")
	if result.Result != Green || result.Reason != "inherits green from tree a1a1a1a1a1a1: only goal ledger files changed since" {
		t.Fatalf("the ledger-only tree: %+v", result)
	}
}

// writeProofRecords plants the lane's two proof records: results.jsonl's
// lines, then running.json when running is not empty.
func writeProofRecords(t *testing.T, install string, results []string, running string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(Dir(install), "proofs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resultsPath(install), []byte(strings.Join(results, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if running != "" {
		if err := os.WriteFile(runningPath(install), []byte(running), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// The proof log a lane record names: a result's, else the running proof's,
// only when it lies directly inside the lane's proofs folder. Anything else
// is no log this serves, said in words.
func TestProofLogIsTheLogARecordNamesInTheProofsFolder(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	proofs := filepath.Join(Dir(install), "proofs")
	writeProofRecords(t, install, []string{
		`{"tree":"t1","commit":"c1","result":"red","log":"` + filepath.Join(proofs, "a1.log") + `","at":"2026-10-03T08:00:00Z","attempt":"a1"}`,
		`{"tree":"t2","commit":"c2","result":"red","log":"/tmp/x.log","at":"2026-10-03T08:10:00Z","attempt":"a2"}`,
		`{"tree":"t3","commit":"c3","result":"red","log":"` + proofs + `/../results.jsonl","at":"2026-10-03T08:20:00Z","attempt":"a4"}`,
		`{"tree":"t5","commit":"c5","result":"green","log":"","at":"2026-10-03T08:30:00Z","attempt":"a5"}`,
		`{"tree":"t6","commit":"c6","result":"green","log":"` + filepath.Join(proofs, "old.log") + `","at":"2026-10-03T08:40:00Z"}`,
	}, `{"attempt":"a3","tree":"t3","commit":"c3","log":"`+filepath.Join(proofs, "a3.log")+`","since":"2026-10-03T08:50:00Z","pid":1}`)
	for attempt, want := range map[string]string{"a1": filepath.Join(proofs, "a1.log"), "a3": filepath.Join(proofs, "a3.log")} {
		if got, err := ProofLog(install, attempt); err != nil || got != want {
			t.Fatalf("attempt %s: %q %v, want %q", attempt, got, err, want)
		}
	}
	refused := map[string]string{
		"a2":               "the log of attempt a2 is not in the lane's own log folder",
		"a4":               "the log of attempt a4 is not in the lane's own log folder",
		"a5":               "the log of attempt a5 is not in the lane's own log folder",
		"unknown":          "no lane record names attempt unknown",
		"../results.jsonl": "no lane record names attempt ../results.jsonl",
		"":                 "no attempt was named",
	}
	for attempt, words := range refused {
		got, err := ProofLog(install, attempt)
		if !errors.Is(err, ErrNoProofLog) || !strings.Contains(err.Error(), words) || got != "" {
			t.Fatalf("attempt %q: %q %v, want ErrNoProofLog saying %q", attempt, got, err, words)
		}
	}
}

// A record it cannot read is said as such, never as no record: an
// unreadable results file, an unreadable running proof, and an attempt
// nothing names beside lines that do not decode.
func TestProofLogSaysARecordItCannotRead(t *testing.T) {
	t.Parallel()
	unreadable := t.TempDir()
	if err := os.MkdirAll(resultsPath(unreadable), 0o755); err != nil {
		t.Fatal(err)
	}
	torn := t.TempDir()
	writeProofRecords(t, torn, []string{`{"tree":"t1","commit":"c1","result":"red","log":"x","attempt":"a1"}`, `{"tree":"t2","res`}, "")
	running := t.TempDir()
	writeProofRecords(t, running, nil, "")
	if err := os.MkdirAll(runningPath(running), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, each := range map[string]struct{ install, words string }{
		"results": {unreadable, "the landing results can't be read: "},
		"torn":    {torn, "the landing results have 1 line(s) that can't be read, so whether one names attempt a9 is not known"},
		"running": {running, "the running landing check can't be read: "},
	} {
		got, err := ProofLog(each.install, "a9")
		if err == nil || errors.Is(err, ErrNoProofLog) || !strings.Contains(err.Error(), each.words) || got != "" {
			t.Fatalf("%s: %q %v, want a read failure saying %q", name, got, err, each.words)
		}
	}
}

// A log in the proofs folder that is a link is refused, never followed: a
// link moved there could point anywhere. A missing log is the caller's to
// say (the record names it; the file is gone).
func TestProofLogRefusesALinkInTheProofsFolder(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	proofs := filepath.Join(Dir(install), "proofs")
	if err := os.MkdirAll(proofs, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("not a proof log"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(proofs, "a1.log")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proofs, "a2.log"), []byte("landing prove: the proof command exited 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeProofRecords(t, install, []string{
		`{"tree":"t1","commit":"c1","result":"red","log":"` + filepath.Join(proofs, "a1.log") + `","at":"2026-10-03T08:00:00Z","attempt":"a1"}`,
		`{"tree":"t2","commit":"c2","result":"red","log":"` + filepath.Join(proofs, "a2.log") + `","at":"2026-10-03T08:10:00Z","attempt":"a2"}`,
		`{"tree":"t3","commit":"c3","result":"red","log":"` + filepath.Join(proofs, "a3.log") + `","at":"2026-10-03T08:20:00Z","attempt":"a3"}`,
	}, "")

	if got, err := ProofLog(install, "a1"); !errors.Is(err, ErrNoProofLog) || !strings.Contains(err.Error(), "the log of attempt a1 is not a file in the lane's own log folder") || got != "" {
		t.Fatalf("a link: %q %v, want ErrNoProofLog refusing it", got, err)
	}
	for attempt, want := range map[string]string{"a2": filepath.Join(proofs, "a2.log"), "a3": filepath.Join(proofs, "a3.log")} {
		if got, err := ProofLog(install, attempt); err != nil || got != want {
			t.Fatalf("attempt %s: %q %v, want %q", attempt, got, err, want)
		}
	}
}
