package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/shellquote"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestLaneFixCheckpointMatchesRegisteredBatch(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		change func(*lane.Record, *plain.Batch, *plain.Running)
		want   bool
	}{
		{"member batch", func(*lane.Record, *plain.Batch, *plain.Running) {}, true},
		{"other checkout", func(r *lane.Record, _ *plain.Batch, _ *plain.Running) { r.Root += "-other" }, false},
		{"new registration", func(r *lane.Record, _ *plain.Batch, _ *plain.Running) { r.CustodyEpoch++ }, false},
		{"closed batch", func(_ *lane.Record, b *plain.Batch, _ *plain.Running) { b.State = plain.BatchClosed }, false},
		{"other proof", func(_ *lane.Record, _ *plain.Batch, r *plain.Running) { r.BatchID = "another" }, false},
		{"other members", func(_ *lane.Record, _ *plain.Batch, r *plain.Running) {
			r.BatchMembers = []plain.GoalSHA{{Goal: "other", SHA: "sha"}}
		}, false},
		{"prepared batch", func(_ *lane.Record, b *plain.Batch, _ *plain.Running) { b.State = plain.BatchPrepared }, false},
		{"no batch commit", func(_ *lane.Record, _ *plain.Batch, r *plain.Running) { r.Commit = "" }, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			registered := lane.Record{Root: root, Install: root, CustodyEpoch: 1}
			members := []plain.GoalSHA{{Goal: "goal-a", SHA: "member"}}
			batch := plain.Batch{ID: "batch-1", Lane: registered, Base: "main", State: plain.BatchRunning, Members: members}
			running := plain.Running{Commit: "batch-commit", Tree: "batch-tree", Attempt: "proof", BatchID: batch.ID, BatchMembers: members}
			red := plain.Result{Commit: running.Commit, Tree: running.Tree, Attempt: running.Attempt, BatchID: running.BatchID, BatchMembers: running.BatchMembers, Result: plain.Red}
			test.change(&registered, &batch, &running)
			dir := plain.Dir(root)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			for name, value := range map[string]any{"batch.json": batch, "running.json": running} {
				data, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			data, _ := json.Marshal(red)
			if err := os.WriteFile(filepath.Join(dir, "results.jsonl"), append(data, '\n'), 0o600); err != nil {
				t.Fatal(err)
			}
			fix := landingFixCheckpoint(root, root, registered)
			if (fix != nil) != test.want {
				t.Fatalf("checkpoint %+v, want admitted=%v", fix, test.want)
			}
			if fix != nil && (fix.Commit != "batch-commit" || len(fix.Members) != 1 || fix.Members[0] != "goal-a") {
				t.Fatalf("wrong checkpoint: %+v", fix)
			}
		})
	}
}

func TestLaneFixMessageReadsGitFileAndRefusesEdits(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	message := "goal goal-a: lane fix of package-a (fix round 1)\n\nGoal-Unit: goal-a/lane-fix-1\n"
	if err := os.WriteFile(filepath.Join(root, "message"), []byte(message), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{"file", []string{"-F", "message"}, message},
		{"literal", []string{"-m", message}, message},
		{"paragraphs", []string{"-m", "subject", "-m", "Goal-Unit: goal-a/lane-fix-1"}, "subject\n\nGoal-Unit: goal-a/lane-fix-1"},
		{"long literal", []string{"--message=" + message}, message},
		{"long file", []string{"--file=message", "--quiet"}, message},
		{"no message", nil, ""},
		{"unreadable", []string{"-F", "missing"}, ""},
		{"stdin", []string{"-F", "-"}, ""},
		{"edited", []string{"-F", "message", "--edit"}, ""},
		{"amended", []string{"-F", "message", "--amend"}, ""},
		{"different message", []string{"-F", "message", "-m", "no trailer"}, ""},
		{"extra trailer", []string{"-F", "message", "--trailer", "Goal-Unit: other/lane-fix-1"}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := landingFixMessage(root, test.args); got != test.want {
				t.Fatalf("message %q, want %q", got, test.want)
			}
		})
	}
}

func TestSkillLandingAgentOneFixRound(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "skills", "landing-agent", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, words := range []string{"ONE fix round", "every red of the batch proof's gate", "fix the goal's code or", "never loosen or delete a test", "--work lane-fix-1", "metasystem work review GOAL", "Goal-Unit: GOAL/lane-fix-1", "material read finding", "the fix job id and read id", "For `main`, hold the batch", "--check 'metasystem test impact'", "followed by `metasystem landing prove`", "`running_fix`", "When the engine refuses the fix build, return as before"} {
		if !strings.Contains(text, words) {
			t.Errorf("skill omits %q", words)
		}
	}
	if strings.Contains(text, "The build commits ONE") || !strings.Contains(text, "The review commits ONE") {
		t.Fatal("skill assigns the repair commit to the build")
	}
	if strings.Contains(text, "Never commit in the lane") {
		t.Fatal("skill still forbids its engine fix commit")
	}
}

func TestLaneFixGuardRealGitCommit(t *testing.T) {
	t.Parallel()
	root := syncedClaimedGoalFixture(t)
	head := goalSyncMutationGit(t, root, "rev-parse", "HEAD")
	goalSyncMutationGit(t, root, "checkout", "--detach", head)
	pid := int64(os.Getpid())
	exact, state, err := (identity.KernelProber{}).Probe(pid)
	if err != nil || state != identity.Alive {
		t.Fatalf("probe %s: %v", state, err)
	}
	if _, err := lease.AnnounceWithPair(root, "lane-fix-session", pid, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "lane-fix-session", "metasystem", lane.AgentLineage); err != nil {
		t.Fatal(err)
	}
	if holder, err := lease.RequireHolder(root, pid, nil); err != nil || !holder.Holder {
		t.Fatalf("holder %+v: %v", holder, err)
	}
	seat := t.TempDir()
	if _, err := lease.AnnounceWithPair(seat, "ordinary", pid, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "ordinary", "metasystem", "ordinary-seat"); err != nil {
		t.Fatal(err)
	}
	if _, err := lease.RequireHolder(seat, pid, nil); err != nil {
		t.Fatal(err)
	}
	if !landingFixActor(root, pid) || landingFixActor(seat, pid) {
		t.Fatal("landing identity did not distinguish its session from another process")
	}
	registry := t.TempDir()
	registered := lane.Record{Root: root, Install: root, CustodyEpoch: 1}
	members := []plain.GoalSHA{{Goal: "standing-validation", SHA: head}}
	records := map[string]any{
		lane.RecordPath(filepath.Join(registry, ".metasystem")): registered,
		filepath.Join(plain.Dir(root), "batch.json"):            plain.Batch{ID: "batch-1", Lane: registered, Base: head, State: plain.BatchRunning, Members: members},
		filepath.Join(plain.Dir(root), "running.json"):          plain.Running{Commit: head, Tree: "batch-tree", Attempt: "attempt-1", BatchID: "batch-1", BatchMembers: members},
	}
	records[filepath.Join(plain.Dir(root), "fixes", "attempt-1.json")] = plain.Fix{Goal: "standing-validation", Units: []string{"fixture"}, Job: "build", State: "building", Round: 1, Attempt: "attempt-1", Parent: head}
	records[filepath.Join(plain.Dir(root), "results.jsonl")] = plain.Result{Attempt: "attempt-1", Commit: head, Tree: "batch-tree", BatchID: "batch-1", BatchMembers: members, Result: plain.Red, Cause: &plain.Cause{Kind: "own", Goal: "standing-validation"}, Failed: []plain.FailedUnit{{Unit: "fixture"}}}
	for path, value := range records {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	red, _ := json.Marshal(records[filepath.Join(plain.Dir(root), "results.jsonl")])
	if err := os.WriteFile(filepath.Join(plain.Dir(root), "results.jsonl"), append(red, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	hook := "#!/bin/sh\nexec " + shellquote.Token(executable) + " internal pre-commit --root " + shellquote.Token(root) + "\n"
	if err := testexec.WriteFile(filepath.Join(root, ".git", "hooks", "pre-commit"), []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "code.go"), []byte("package fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	goalSyncMutationGit(t, root, "add", "code.go")
	messagePath := filepath.Join(t.TempDir(), "message")
	commit := func(message string) error {
		t.Helper()
		if err := os.WriteFile(messagePath, []byte(message), 0o600); err != nil {
			t.Fatal(err)
		}
		args := []string{"-C", root, "commit"}
		for _, paragraph := range strings.Split(strings.TrimSpace(message), "\n\n") {
			args = append(args, "-m", paragraph)
		}
		command := exec.Command("git", args...)
		command.Env = append(os.Environ(), "GO_WANT_BATCH_E2E_COMMAND=1", "METASYSTEM_SUPERVISION_REGISTRY_HOME="+registry)
		output, err := command.CombinedOutput()
		t.Logf("git commit: %v\n%s", err, output)
		return err
	}
	if err := commit("repair without a trailer\n"); err == nil {
		t.Fatal("commit without trailer passed")
	}
	if got := goalSyncMutationGit(t, root, "rev-parse", "HEAD"); got != head {
		t.Fatalf("refused commit moved HEAD: %s", got)
	}
	if err := commit("repair\n\nGoal-Unit: standing-validation/lane-fix-2\n"); err == nil {
		t.Fatal("unrecorded fix round passed")
	}
	message := "goal standing-validation: lane fix of fixture (fix round 1)\n\nGoal-Unit: standing-validation/lane-fix-1\n"
	fix, err := plain.FixForAttempt(root, "attempt-1")
	if err != nil {
		t.Fatal(err)
	}
	stale := *fix
	stale.Attempt = "older"
	writeLaneFixJSON(t, filepath.Join(plain.Dir(root), "fixes", "attempt-1.json"), stale)
	if err := commit(message); err == nil {
		t.Fatal("stale attempt authorized the real hook")
	}
	if err := plain.WriteFix(root, fix); err != nil {
		t.Fatal(err)
	}
	if err := commit(message); err != nil {
		t.Fatal("valid lane fix refused", err)
	}
	fix.Commit = goalSyncMutationGit(t, root, "rev-parse", "HEAD")
	if err := plain.WriteFix(root, fix); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "code.go"), []byte("package fixture\n// second repair\n"), 0600); err != nil {
		t.Fatal(err)
	}
	goalSyncMutationGit(t, root, "add", "code.go")
	if err := commit(message); err == nil {
		t.Fatal("real hook admitted a second repair commit")
	}
	if got := goalSyncMutationGit(t, root, "show", "-s", "--format=%B", "HEAD"); !strings.Contains(got, "Goal-Unit: standing-validation/lane-fix-1") {
		t.Fatalf("commit omitted trailer: %q", got)
	}
}

func TestLaneMergeGuardRealGitCommit(t *testing.T) {
	t.Parallel()
	root := syncedClaimedGoalFixture(t)
	head := goalSyncMutationGit(t, root, "rev-parse", "HEAD")
	goalSyncMutationGit(t, root, "checkout", "--detach", head)
	goalSyncMutationGit(t, root, "checkout", "-b", "merge-member")
	if err := os.WriteFile(filepath.Join(root, "member.go"), []byte("package fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	goalSyncMutationGit(t, root, "add", "member.go")
	goalSyncMutationGit(t, root, "commit", "-m", "member")
	tip := goalSyncMutationGit(t, root, "rev-parse", "HEAD")
	goalSyncMutationGit(t, root, "checkout", "--detach", head)
	goalSyncMutationGit(t, root, "merge", "--no-commit", "--no-ff", tip)
	pid := int64(os.Getpid())
	exact, state, err := (identity.KernelProber{}).Probe(pid)
	if err != nil || state != identity.Alive {
		t.Fatalf("probe %s: %v", state, err)
	}
	if _, err := lease.AnnounceWithPair(root, "lane-fix-session", pid, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "lane-fix-session", "metasystem", lane.AgentLineage); err != nil {
		t.Fatal(err)
	}
	if holder, err := lease.RequireHolder(root, pid, nil); err != nil || !holder.Holder {
		t.Fatalf("holder %+v: %v", holder, err)
	}
	if !landingFixActor(root, pid) || landingFixActor(root, int64(os.Getppid())) {
		t.Fatal("landing identity did not distinguish its session from another process")
	}
	registry := t.TempDir()
	registered := lane.Record{Root: root, Install: root, CustodyEpoch: 1}
	members := []plain.GoalSHA{{Goal: "standing-validation", SHA: tip}}
	records := map[string]any{
		lane.RecordPath(filepath.Join(registry, ".metasystem")): registered,
		filepath.Join(plain.Dir(root), "batch.json"):            plain.Batch{ID: "batch-1", Lane: registered, Base: head, State: plain.BatchRunning, Members: members},
	}
	for path, value := range records {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := plain.WriteFix(root, &plain.Fix{Goal: "standing-validation", Units: []string{"lane-merge-1"}, Commit: head, Tip: tip, State: "resolving", Attempt: "merge-attempt", Paths: []conflict.Path{{Path: "code.go", Class: conflict.Builder}}}); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	hook := "#!/bin/sh\nexec " + shellquote.Token(executable) + " internal pre-commit --root " + shellquote.Token(root) + "\n"
	if err := testexec.WriteFile(filepath.Join(root, ".git", "hooks", "pre-commit"), []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "code.go"), []byte("package fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	goalSyncMutationGit(t, root, "add", "code.go")
	messagePath := filepath.Join(t.TempDir(), "message")
	commit := func(message string) error {
		t.Helper()
		if err := os.WriteFile(messagePath, []byte(message), 0o600); err != nil {
			t.Fatal(err)
		}
		command := exec.Command("git", "-C", root, "commit", "-F", messagePath)
		command.Env = append(os.Environ(), "GO_WANT_BATCH_E2E_COMMAND=1", "METASYSTEM_SUPERVISION_REGISTRY_HOME="+registry)
		output, err := command.CombinedOutput()
		t.Logf("git commit: %v\n%s", err, output)
		return err
	}
	if err := commit("repair without a trailer\n"); err == nil {
		t.Fatal("commit without trailer passed")
	}
	if got := goalSyncMutationGit(t, root, "rev-parse", "HEAD"); got != head {
		t.Fatalf("refused commit moved HEAD: %s", got)
	}
	message := "goal standing-validation: merge resolution of fixture\n\nGoal-Unit: standing-validation/lane-merge-1\n"
	if err := commit(message); err != nil {
		t.Fatal("valid lane merge refused", err)
	}
	if parents := goalSyncMutationGit(t, root, "show", "-s", "--format=%P", "HEAD"); parents != head+" "+tip {
		t.Fatalf("merge parents=%q", parents)
	}
	if got := goalSyncMutationGit(t, root, "show", "-s", "--format=%B", "HEAD"); !strings.Contains(got, "Goal-Unit: standing-validation/lane-merge-1") {
		t.Fatalf("commit omitted trailer: %q", got)
	}
}

func TestLaneFixCheckpointRequiresRecordedRed(t *testing.T) {
	t.Parallel()
	for _, verdict := range []string{"missing", "green", "red"} {
		t.Run(verdict, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			registered := lane.Record{Root: root, Install: root, CustodyEpoch: 1}
			members := []plain.GoalSHA{{Goal: "goal", SHA: "member"}}
			running := plain.Running{Commit: "batch", Tree: "tree", Attempt: "proof", BatchID: "batch", BatchMembers: members}
			if err := os.MkdirAll(plain.Dir(root), 0755); err != nil {
				t.Fatal(err)
			}
			for name, value := range map[string]any{"batch.json": plain.Batch{ID: "batch", Lane: registered, Base: "main", State: plain.BatchRunning, Members: members}, "running.json": running} {
				data, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(plain.Dir(root), name), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if verdict != "missing" {
				data, _ := json.Marshal(plain.Result{Commit: running.Commit, Tree: running.Tree, Attempt: running.Attempt, BatchID: running.BatchID, BatchMembers: members, Result: verdict})
				if err := os.WriteFile(filepath.Join(plain.Dir(root), "results.jsonl"), append(data, '\n'), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if fix := landingFixCheckpoint(root, root, registered); (fix != nil) != (verdict == "red") {
				t.Fatalf("%s proof checkpoint=%+v", verdict, fix)
			}
		})
	}
}
