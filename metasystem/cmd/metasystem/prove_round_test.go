package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

type proveRoundFixture struct {
	installation, worktree, tree string
}

func proveRoundGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir, "-c", "core.hooksPath=/dev/null", "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid"}, args...)...)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeProveRoundRecord(t *testing.T, installation, id string, fields map[string]any) {
	t.Helper()
	record := map[string]any{"jobId": id, "status": "completed"}
	for key, value := range fields {
		record[key] = value
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(installation, "artifacts", "agents", "jobs", id+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// newProveRoundFixture is a project with its installation under metasystem/,
// one job worktree of the dispatcher's shape under artifacts/agents/worktrees
// holding an uncommitted round, and a two-round chain of records.
func newProveRoundFixture(t *testing.T) proveRoundFixture {
	t.Helper()
	projectRoot, err := canonicalPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	installation := filepath.Join(projectRoot, "metasystem")
	if err := os.MkdirAll(installation, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installation, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".gitignore"), []byte("metasystem/artifacts/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	proveRoundGit(t, projectRoot, "init", "-q", "-b", "main")
	proveRoundGit(t, projectRoot, "add", ".")
	proveRoundGit(t, projectRoot, "commit", "-qm", "seed")
	worktree := filepath.Join(installation, "artifacts", "agents", "worktrees", "implementer-1")
	proveRoundGit(t, projectRoot, "worktree", "add", "-q", "-b", "agent/implementer-1", worktree, "HEAD")
	if err := os.WriteFile(filepath.Join(worktree, "metasystem", "round.txt"), []byte("round 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Delegates never commit: the round's work is the working tree.
	tree, err := (gittree.Workspace{Dir: worktree}).Snapshot("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if head := proveRoundGit(t, worktree, "rev-parse", "HEAD^{tree}"); head == tree {
		t.Fatalf("the fixture's round left HEAD unchanged: %s", head)
	}
	writeProveRoundRecord(t, installation, "implementer-1", map[string]any{"launchMode": "worktree", "workspaceRoot": worktree, "goalId": "goal-a", "round": 1})
	writeProveRoundRecord(t, installation, "implementer-1-r2", map[string]any{"parentJob": "implementer-1", "launchMode": "worktree", "workspaceRoot": worktree, "goalId": "goal-a", "round": 2})
	return proveRoundFixture{installation: installation, worktree: worktree, tree: tree}
}

func stubProveRoundRun(t *testing.T, exit int) *[]string {
	t.Helper()
	var captured []string
	previous := proveRoundTestRun
	proveRoundTestRun = func(args []string) int {
		captured = append([]string(nil), args...)
		return exit
	}
	t.Cleanup(func() { proveRoundTestRun = previous })
	return &captured
}

func readProofRoundRecord(t *testing.T, installation, rootJob string, round int) ProofRoundRecord {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(installation, "artifacts", "agents", rootJob, "rounds", itoa(round), "proof.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record ProofRoundRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	return record
}

func itoa(value int) string { return strconv.Itoa(value) }

func TestProveRoundProvesTheChainWorktreesCommittedTreeOnTheInstallation(t *testing.T) {
	fixture := newProveRoundFixture(t)
	captured := stubProveRoundRun(t, 7)
	// Any member of the chain names it; the tree is the worktree's snapshot
	// (HEAD plus the working tree), the whole project, and the proof runs on
	// the installation, not in the worktree.
	if status := runDispatchProveRound([]string{"--root", fixture.installation, "--job", "implementer-1-r2"}); status != 7 {
		t.Fatalf("prove-round exit = %d; want the test run's own exit 7", status)
	}
	want := []string{"--root", fixture.installation, "--tree", fixture.tree, "--goal", "goal-a", "--mode", "auto", "--purpose", "diagnostic"}
	if strings.Join(*captured, " ") != strings.Join(want, " ") {
		t.Fatalf("test run arguments = %v; want %v", *captured, want)
	}
	record := readProofRoundRecord(t, fixture.installation, "implementer-1", 2)
	if record.CandidateTree != fixture.tree || record.RootJob != "implementer-1" || record.Round != 2 || record.GoalID != "goal-a" ||
		record.Purpose != "diagnostic" || record.ExitStatus != 7 || record.AttemptID != "" || record.Sufficient {
		t.Fatalf("proof record = %+v", record)
	}
	if _, err := time.Parse(time.RFC3339Nano, record.ProvedAt); err != nil {
		t.Fatalf("proof record time: %v", err)
	}
}

func TestProveRoundRefusesWhatItCannotProveAsARound(t *testing.T) {
	fixture := newProveRoundFixture(t)
	captured := stubProveRoundRun(t, 0)
	refuse := func(name string, args ...string) {
		t.Helper()
		*captured = nil
		if status := runDispatchProveRound(args); status != 2 {
			t.Fatalf("%s: exit = %d; want refusal 2", name, status)
		}
		if *captured != nil {
			t.Fatalf("%s: the test run was started anyway with %v", name, *captured)
		}
	}
	// Every change in the worktree is the round's work, an untracked file
	// included: the tree proved follows it.
	if err := os.WriteFile(filepath.Join(fixture.worktree, "metasystem", "extra.txt"), []byte("more\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if status := runDispatchProveRound([]string{"--root", fixture.installation, "--job", "implementer-1"}); status != 0 || len(*captured) < 4 || (*captured)[3] == fixture.tree {
		t.Fatalf("an untracked change did not reach the proved tree: exit=%d args=%v", status, *captured)
	}
	if err := os.Remove(filepath.Join(fixture.worktree, "metasystem", "extra.txt")); err != nil {
		t.Fatal(err)
	}
	// A shared-checkout job has no tree of its own to prove.
	writeProveRoundRecord(t, fixture.installation, "shared-1", map[string]any{"launchMode": "shared-checkout", "workspaceRoot": filepath.Dir(fixture.installation), "goalId": "goal-a", "round": 1})
	refuse("shared checkout", "--root", fixture.installation, "--job", "shared-1")
	// A delivery proof binds a goal.
	writeProveRoundRecord(t, fixture.installation, "goalless-1", map[string]any{"launchMode": "worktree", "workspaceRoot": fixture.worktree, "round": 1})
	refuse("no goal", "--root", fixture.installation, "--job", "goalless-1")
	// A round still running is proved after it has returned.
	writeProveRoundRecord(t, fixture.installation, "implementer-1-r3", map[string]any{"parentJob": "implementer-1", "launchMode": "worktree", "workspaceRoot": fixture.worktree, "goalId": "goal-a", "round": 3, "status": "running"})
	refuse("live round", "--root", fixture.installation, "--job", "implementer-1")
	if err := os.Remove(filepath.Join(fixture.installation, "artifacts", "agents", "jobs", "implementer-1-r3.json")); err != nil {
		t.Fatal(err)
	}
	// A record without a launch mode is placed by where its workspace lives.
	writeProveRoundRecord(t, fixture.installation, "older-1", map[string]any{"workspaceRoot": fixture.worktree, "goalId": "goal-a", "round": 1})
	if status := runDispatchProveRound([]string{"--root", fixture.installation, "--job", "older-1"}); status != 0 || len(*captured) == 0 {
		t.Fatalf("a record without a launch mode was not placed by its worktree: exit=%d args=%v", status, *captured)
	}
}

func TestNewestAttemptForTreeIsTheNewestMatchingOne(t *testing.T) {
	tree, other := strings.Repeat("a", 40), strings.Repeat("b", 40)
	at := func(minutes int) string {
		return time.Date(2026, 9, 12, 20, minutes, 0, 0, time.UTC).Format(time.RFC3339Nano)
	}
	attempts := []proofrun.Attempt{
		{AttemptID: "no-result", GoalID: "g", StartedAt: at(9)},
		{AttemptID: "other-tree", GoalID: "g", StartedAt: at(8), TestResult: &proofrun.TestResult{CandidateTree: other}},
		{AttemptID: "other-goal", GoalID: "h", StartedAt: at(7), TestResult: &proofrun.TestResult{CandidateTree: tree}},
		{AttemptID: "older", GoalID: "g", StartedAt: at(1), TestResult: &proofrun.TestResult{CandidateTree: tree}},
		{AttemptID: "newer", GoalID: "g", StartedAt: at(5), TestResult: &proofrun.TestResult{CandidateTree: tree}},
	}
	got, found := newestAttemptForTree(attempts, tree, "g", nil)
	if !found || got.AttemptID != "newer" {
		t.Fatalf("newest attempt = %q found=%v; want newer", got.AttemptID, found)
	}
	// An attempt that existed before the run is not the run's proof, however
	// new it looks.
	if got, found := newestAttemptForTree(attempts, tree, "g", map[string]bool{"newer": true}); !found || got.AttemptID != "older" {
		t.Fatalf("a known attempt was linked as the run's own: %q found=%v", got.AttemptID, found)
	}
	if _, found := newestAttemptForTree(attempts, strings.Repeat("c", 40), "g", nil); found {
		t.Fatal("an attempt was found for a tree nothing proved")
	}
}
