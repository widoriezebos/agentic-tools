package goal

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestAcceptedLedgerTipDistinguishesBootstrapFromBreakage(t *testing.T) {
	_, clone := oneClone(t)
	if tip, exists, err := AcceptedLedgerTip(clone); err != nil || exists || tip != "" {
		t.Fatalf("pre-bootstrap accepted tip: tip=%q exists=%t err=%v", tip, exists, err)
	}
	seedLedger(t, clone)
	tip, exists, err := AcceptedLedgerTip(clone)
	if err != nil || !exists || tip == "" {
		t.Fatalf("migrated accepted tip: tip=%q exists=%t err=%v", tip, exists, err)
	}
	common := mustGit(t, clone, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err := os.WriteFile(filepath.Join(common, filepath.FromSlash(AcceptedRef)), []byte("not-an-oid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := AcceptedLedgerTip(clone); err == nil || !strings.Contains(err.Error(), "accepted ref") {
		t.Fatalf("broken accepted ref was read as bootstrap: %v", err)
	}
}

func TestLedgerChangesFallsBackWhenAcceptedIsASecondParent(t *testing.T) {
	repo := t.TempDir()
	mustGit(t, repo, "init", "-q", "-b", "main")
	write := func(name, body, message string) string {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		mustGit(t, repo, "add", name)
		mustGit(t, repo, "commit", "-qm", message)
		return mustGit(t, repo, "rev-parse", "HEAD")
	}
	base := write("base", "base\n", "base")
	accepted := write("accepted", "accepted\n", "accepted")
	mustGit(t, repo, "checkout", "-qb", "other", base)
	_ = write("other", "other\n", "other")
	mustGit(t, repo, "merge", "--no-ff", "-qm", "merge accepted as second parent", accepted)
	merged := mustGit(t, repo, "rev-parse", "HEAD")
	changes, err := LedgerChanges(repo, accepted, merged)
	if err != nil || len(changes) != 1 || changes[0].Tip != merged || changes[0].Consecutive {
		t.Fatalf("second-parent ancestry invented intermediate canonical states: %+v %v", changes, err)
	}
	next := write("next", "next\n", "next")
	changes, err = LedgerChanges(repo, merged, next)
	if err != nil || len(changes) != 1 || changes[0].Tip != next || !changes[0].Consecutive {
		t.Fatalf("ordinary first-parent movement was not consecutive: %+v %v", changes, err)
	}
}

func TestLedgerChangesDoesNotDisguiseUnreadableHistoryAsARewind(t *testing.T) {
	repo := t.TempDir()
	mustGit(t, repo, "init", "-q", "-b", "main")
	if _, err := LedgerChanges(repo, strings.Repeat("a", 40), strings.Repeat("b", 40)); err == nil || !strings.Contains(err.Error(), "walk accepted ledger changes") {
		t.Fatalf("unreadable object history was accepted as a direct transition: %v", err)
	}
}

func TestCaptureTipBoundedKillsTheWholeTransportGroup(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	groupFile := filepath.Join(dir, "group")
	childFile := filepath.Join(dir, "child")
	wrapper := filepath.Join(dir, "git")
	script := `#!/bin/sh
case " $* " in
  *" fetch "*)
    echo $$ > "$LEDGER_FETCH_GROUP_FILE"
    trap '' TERM
    sh -c 'trap "" TERM; echo $$ > "$LEDGER_FETCH_CHILD_FILE"; while :; do sleep 1; done' &
    wait
    ;;
esac
exec "$LEDGER_REAL_GIT" "$@"
`
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LEDGER_REAL_GIT", realGit)
	t.Setenv("LEDGER_FETCH_GROUP_FILE", groupFile)
	t.Setenv("LEDGER_FETCH_CHILD_FILE", childFile)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	root := t.TempDir()
	mustGit(t, root, "init", "-q")
	_, err = CaptureTipBounded(Endpoint{Root: root, Remote: "blocked", Branch: "refs/heads/main"}, 300*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("blocking transport did not time out: %v", err)
	}
	groupData, groupErr := os.ReadFile(groupFile)
	childData, childErr := os.ReadFile(childFile)
	if groupErr != nil || childErr != nil {
		t.Fatalf("transport did not publish its real process identities: group=%v child=%v", groupErr, childErr)
	}
	groupID, _ := strconv.Atoi(strings.TrimSpace(string(groupData)))
	childID, _ := strconv.Atoi(strings.TrimSpace(string(childData)))
	if groupID < 1 || childID < 1 || groupID == childID {
		t.Fatalf("invalid transport identities: group=%d child=%d", groupID, childID)
	}
	if err := waitForGroupAbsence(groupID, 30*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(childID, 0); err == nil {
		// A killed child may remain briefly as a zombie. Its process state,
		// not kill(0), decides whether any transport work survived.
		stateOut, stateErr := exec.Command("ps", "-o", "stat=", "-p", fmt.Sprint(childID)).CombinedOutput()
		if stateErr == nil && !strings.HasPrefix(strings.TrimSpace(string(stateOut)), "Z") {
			t.Fatalf("transport descendant %d survived group termination with state %q", childID, stateOut)
		}
	}
}

func TestCaptureTipBoundedLetsCooperativeTransportExitDuringGrace(t *testing.T) {
	if boundedCaptureGrace != 5*time.Second {
		t.Fatalf("bounded capture grace=%s, want 5s", boundedCaptureGrace)
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	groupFile := filepath.Join(dir, "group")
	termFile := filepath.Join(dir, "term")
	wrapper := filepath.Join(dir, "git")
	script := `#!/bin/sh
case " $* " in
  *" fetch "*)
    echo $$ > "$LEDGER_GRACE_GROUP_FILE"
    trap 'echo TERM > "$LEDGER_GRACE_TERM_FILE"; exit 0' TERM
    while :; do sleep 1; done
    ;;
esac
exec "$LEDGER_GRACE_REAL_GIT" "$@"
`
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LEDGER_GRACE_REAL_GIT", realGit)
	t.Setenv("LEDGER_GRACE_GROUP_FILE", groupFile)
	t.Setenv("LEDGER_GRACE_TERM_FILE", termFile)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	root := t.TempDir()
	mustGit(t, root, "init", "-q")
	_, err = CaptureTipBounded(Endpoint{Root: root, Remote: "blocked", Branch: "refs/heads/main"}, 300*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("cooperative transport did not time out: %v", err)
	}
	if data, readErr := os.ReadFile(termFile); readErr != nil || strings.TrimSpace(string(data)) != "TERM" {
		t.Fatalf("transport received no graceful TERM opportunity: %q %v", data, readErr)
	}
	groupData, err := os.ReadFile(groupFile)
	if err != nil {
		t.Fatal(err)
	}
	groupID, err := strconv.Atoi(strings.TrimSpace(string(groupData)))
	if err != nil {
		t.Fatal(err)
	}
	if err := waitForGroupAbsence(groupID, 30*time.Second); err != nil {
		t.Fatal(err)
	}
}

func waitForGroupAbsence(pgid int, failsafe time.Duration) error {
	deadline := time.Now().Add(failsafe)
	for {
		err := syscall.Kill(-pgid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if err != nil && !errors.Is(err, syscall.EPERM) {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("transport process group %d still existed after the 30-second hang failsafe", pgid)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestProjectAtReadsACommitWithoutMovingAnyRef(t *testing.T) {
	_, clone := oneClone(t)
	seedLedger(t, clone)
	tip, exists, err := AcceptedLedgerTip(clone)
	if err != nil || !exists {
		t.Fatalf("accepted tip: %q %t %v", tip, exists, err)
	}
	before := mustGit(t, clone, "rev-parse", AcceptedRef)
	projection, err := ProjectAt(clone, tip)
	if err != nil || projection.Tip != tip || projection.Tree == nil {
		t.Fatalf("projection at the accepted tip: %+v err=%v", projection, err)
	}
	if after := mustGit(t, clone, "rev-parse", AcceptedRef); after != before {
		t.Fatalf("a read moved the accepted ref: %q -> %q", before, after)
	}
	if _, err := ProjectAt(clone, "0000000000000000000000000000000000000000"); err == nil {
		t.Fatal("an unreadable commit must refuse, not project")
	}
}

func TestIsAncestorAnswersAllThreeShapes(t *testing.T) {
	_, clone := oneClone(t)
	seedLedger(t, clone)
	tip := strings.TrimSpace(mustGit(t, clone, "rev-parse", AcceptedRef))
	if ok, err := IsAncestor(clone, tip, tip); err != nil || !ok {
		t.Fatalf("a commit is its own ancestor: %t %v", ok, err)
	}
	parent := strings.TrimSpace(mustGit(t, clone, "rev-parse", tip+"^1"))
	if ok, err := IsAncestor(clone, parent, tip); err != nil || !ok {
		t.Fatalf("a first parent precedes its child: %t %v", ok, err)
	}
	if ok, err := IsAncestor(clone, tip, parent); err != nil || ok {
		t.Fatalf("a child does not precede its parent: %t %v", ok, err)
	}
}
