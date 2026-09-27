package delegation_test

// Ports of dispatch-fixtures.sh lines 3868-4400 (cluster d, the worktree
// follow-up rebase, and the start of cluster e): the rebase scenarios run on
// the dispatch integration bed because the follow-up rebase plan reads the
// chain worktree's Git inside internal/dispatch and the rebase itself
// stashes, fast-forwards and reapplies in a real worktree; no stub can prove
// what a real stash apply leaves behind.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

// p6Git runs git in dir (a chain worktree or the bed repository).
func (b *bed) p6Git(dir string, args ...string) string {
	b.t.Helper()
	command := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=bed", "-c", "user.email=bed@example.invalid", "-c", "core.hooksPath=/dev/null"}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		b.t.Fatalf("git -C %s %v: %v: %s", dir, args, err, output)
	}
	return strings.TrimSpace(string(output))
}

// p6TaggedLaunches makes the bed's adapter start its inert supervisor under
// the fake adapter's real argv shape (fake.sh VERB --instance-tag TAG), so a
// repeated wrapper's claim preflight can prove the standing process is
// ours. The shell keeps a child so it is not replaced by exec.
func (b *bed) p6TaggedLaunches() {
	b.doubles.Adapter.LaunchFunc = func(request delegation.AdapterLaunch) (int64, error) {
		command := exec.Command("/bin/sh", "-c", "sleep 120; :", "fake.sh", request.Verb, "--instance-tag", request.InstanceTag)
		command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := command.Start(); err != nil {
			return 0, err
		}
		pid := int64(command.Process.Pid)
		b.t.Cleanup(func() {
			_ = syscall.Kill(-int(pid), syscall.SIGKILL)
			_, _ = command.Process.Wait()
		})
		b.doubles.Process.Tags[pid] = request.InstanceTag
		b.launches = append(b.launches, request)
		return pid, nil
	}
}

// p6TrunkCommit writes files (an empty content deletes the path) and
// commits them on trunk; it returns the new trunk commit.
func (b *bed) p6TrunkCommit(message string, files map[string]string) string {
	b.t.Helper()
	for path, content := range files {
		if content == "" {
			b.git("rm", "-q", path)
			continue
		}
		b.writeFile(path, content)
		b.git("add", path)
	}
	b.git("commit", "-qm", message)
	return b.git("rev-parse", "HEAD")
}

// p6WorktreeRound dispatches an implementer round into its own job
// worktree and returns the worktree.
func (b *bed) p6WorktreeRound(job string) string {
	b.t.Helper()
	brief := b.brief(job+"-brief.md", "implement", "Do the thing.")
	result := b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "implementer", "--brief", brief, "--job-id", job, "--worktree")
	requireExit(b.t, result, 0, b.stderr.String())
	workspace, _ := b.record(job)["workspaceRoot"].(string)
	if workspace == "" || !exists(workspace) {
		b.t.Fatalf("%s has no job worktree: %v", job, b.record(job))
	}
	return workspace
}

// p6Complete concludes a running round as completed with a return
// declaring boundary under the chain root's round directory.
func (b *bed) p6Complete(root, job string, round int, boundary ...string) {
	b.t.Helper()
	patch := b.writeFile("complete-"+job+".json", `{}`)
	if _, err := dispatch.RecordCAS(b.root, job, "running", "completed", patch); err != nil {
		b.t.Fatalf("complete %s: %v", job, err)
	}
	encoded, _ := json.Marshal(map[string]any{"diffBoundary": append([]string{}, boundary...)})
	b.writeFile(filepath.Join("artifacts", "agents", root, "rounds", itoa(round), "return.json"), string(encoded)+"\n")
}

// p6CompleteReturn concludes a running round as completed with the given
// return under the chain root's round directory.
func (b *bed) p6CompleteReturn(root, job string, round int, value map[string]any) {
	b.t.Helper()
	patch := b.writeFile("complete-"+job+".json", `{}`)
	if _, err := dispatch.RecordCAS(b.root, job, "running", "completed", patch); err != nil {
		b.t.Fatalf("complete %s: %v", job, err)
	}
	encoded, _ := json.Marshal(value)
	b.writeFile(filepath.Join("artifacts", "agents", root, "rounds", itoa(round), "return.json"), string(encoded)+"\n")
}

// p6DropField removes a record field (the fixture's json_remove_field): a
// record that predates launchMode infers it from its workspace.
func (b *bed) p6DropField(job, field string) {
	b.t.Helper()
	record := b.record(job)
	delete(record, field)
	encoded, _ := json.Marshal(record)
	if err := os.WriteFile(b.recordPath(job), append(encoded, '\n'), 0o644); err != nil {
		b.t.Fatal(err)
	}
}

// p6FollowUp runs a follow-up through the delegate boundary.
func (b *bed) p6FollowUp(job, message string) delegation.Result {
	b.t.Helper()
	return b.runEnv(b.dispatchEnv("follow-up"), "follow-up", "--job", job, "--message", message)
}

func (b *bed) p6Message(name string, extra ...string) string {
	return b.writeFile(name, "Working Mode: implement\n\nContinue the work.\n"+strings.Join(extra, "\n"))
}

func (b *bed) p6Read(path string) string {
	b.t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		b.t.Fatal(err)
	}
	return string(content)
}

// p6DirectionFirst is the first non-empty line after "# Task Direction" in
// a round's delivered prompt.
func (b *bed) p6DirectionFirst(root string, round int) string {
	b.t.Helper()
	inside := false
	for _, line := range strings.Split(b.p6Read(filepath.Join(b.root, "artifacts", "agents", root, "rounds", itoa(round), "prompt.md")), "\n") {
		if line == "# Task Direction" {
			inside = true
			continue
		}
		if inside && strings.TrimSpace(line) != "" {
			return line
		}
	}
	b.t.Fatalf("%s round %d prompt has no task direction", root, round)
	return ""
}

func itoa(value int) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func requireRebaseFields(t *testing.T, record map[string]any, from, to any, conflicted []string) {
	t.Helper()
	paths := []string{}
	if raw, ok := record["conflictedPaths"].([]any); ok {
		for _, path := range raw {
			paths = append(paths, path.(string))
		}
	} else {
		t.Fatalf("record %s carries no conflictedPaths list: %v", record["jobId"], record["conflictedPaths"])
	}
	if record["rebasedFrom"] != from || record["rebasedTo"] != to || !reflect.DeepEqual(paths, conflicted) {
		t.Fatalf("record %s rebase fields from=%v to=%v conflicted=%v; want %v %v %v",
			record["jobId"], record["rebasedFrom"], record["rebasedTo"], paths, from, to, conflicted)
	}
}

// Lines 3869-3907: a worktree behind trunk on none of the chain's files is
// not moved, and the follow-up says why. A follow-up whose
// Authority cites a path only trunk has fast-forwards the worktree onto it,
// keeping the delegate's uncommitted change, and records the clean rebase
// (lines 3909-3929).
func TestFollowUpIntegrationBehindWorktreeStaysUnlessTheBriefCitesATrunkPath(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.p6TrunkCommit("rebase fixture base", map[string]string{"metasystem/stale-chain.txt": "base\n"})
	workspace := b.p6WorktreeRound("stale-wt")
	b.p6DropField("stale-wt", "launchMode")
	staleHead := b.p6Git(workspace, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(workspace, "metasystem", "stale-chain.txt"), []byte("delegate change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b.p6Complete("stale-wt", "stale-wt", 1, "metasystem/stale-chain.txt")
	b.p6TrunkCommit("trunk advance", map[string]string{"trunk-advance.txt": "advance\n"})

	result := b.p6FollowUp("stale-wt", b.p6Message("follow.md"))
	requireExit(t, result, 0, b.stderr.String())
	if !strings.Contains(b.stderr.String(), "WORKTREE-BEHIND: the chain worktree is behind 1 commit, none on this chain's files") {
		t.Fatalf("the non-overlapping follow-up did not name why its worktree stayed behind: %s", b.stderr.String())
	}
	if head := b.p6Git(workspace, "rev-parse", "HEAD"); head != staleHead {
		t.Fatalf("the non-overlapping follow-up moved its worktree to %s", head)
	}
	requireRebaseFields(t, b.record("stale-wt-r2"), nil, nil, []string{})
	b.p6Complete("stale-wt", "stale-wt-r2", 2, "metasystem/stale-chain.txt")

	guidance := "metasystem/scripts/agents/fresh-trunk-guidance.md"
	freshTrunk := b.p6TrunkCommit("fresh trunk guidance", map[string]string{guidance: "fresh trunk guidance\n"})
	result = b.p6FollowUp("stale-wt", b.p6Message("fresh-trunk.md", "", "Authority: "+guidance))
	requireExit(t, result, 0, b.stderr.String())
	if head := b.p6Git(workspace, "rev-parse", "HEAD"); head != freshTrunk {
		t.Fatalf("the cited-path follow-up left the worktree at %s, want trunk %s", head, freshTrunk)
	}
	if got := b.p6Read(filepath.Join(workspace, guidance)); got != "fresh trunk guidance\n" {
		t.Fatalf("trunk guidance not delivered: %q", got)
	}
	if got := b.p6Read(filepath.Join(workspace, "metasystem", "stale-chain.txt")); got != "delegate change\n" {
		t.Fatalf("delegate work not preserved: %q", got)
	}
	requireRebaseFields(t, b.record("stale-wt-r3"), staleHead, freshTrunk, []string{})
	if stashes := b.git("stash", "list"); stashes != "" {
		t.Fatalf("the clean rebase left a stash: %s", stashes)
	}
}

// Lines 3931-3982: trunk changed a file the chain changed. The follow-up
// fast-forwards, leaves the conflict for its builder, records the conflict,
// prepends the resolution paragraph to the task direction, and leaves no
// shared stash. Once the builder stages the resolution the next follow-up
// carries no conflict provenance (the plan sees no unmerged path).
func TestFollowUpIntegrationOverlappingTrunkChangeLeavesTheConflictForTheBuilder(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	target := "metasystem/rebase-target.txt"
	b.p6TrunkCommit("rebase fixture base", map[string]string{target: "base\n"})
	workspace := b.p6WorktreeRound("rebase-wt")
	b.p6DropField("rebase-wt", "launchMode")
	from := b.p6Git(workspace, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(workspace, target), []byte("delegate behaviour\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b.p6Complete("rebase-wt", "rebase-wt", 1, target)
	to := b.p6TrunkCommit("rebase fixture conflict", map[string]string{target: "trunk behaviour\n"})

	result := b.p6FollowUp("rebase-wt", b.p6Message("follow.md"))
	requireExit(t, result, 0, b.stderr.String())
	if head := b.p6Git(workspace, "rev-parse", "HEAD"); head != to {
		t.Fatalf("worktree at %s, want trunk %s", head, to)
	}
	requireRebaseFields(t, b.record("rebase-wt-r2"), from, to, []string{target})
	first := b.p6DirectionFirst("rebase-wt", 2)
	for _, want := range []string{"trunk commit " + to, target, "resolve first, keeping both sides' behaviour", "stage each resolved path with `git add`"} {
		if !strings.Contains(first, want) {
			t.Fatalf("the conflict paragraph lacks %q: %q", want, first)
		}
	}
	if !strings.Contains(b.p6Read(filepath.Join(workspace, target)), "<<<<<<<") {
		t.Fatal("the conflict was not left for the builder")
	}
	if stashes := b.git("stash", "list"); stashes != "" {
		t.Fatalf("the overlapping follow-up left a shared stash entry: %s", stashes)
	}

	if err := os.WriteFile(filepath.Join(workspace, target), []byte("resolved delegate and trunk behaviour\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b.p6Git(workspace, "add", target)
	b.p6Complete("rebase-wt", "rebase-wt-r2", 2, target)
	result = b.p6FollowUp("rebase-wt", b.p6Message("follow-staged.md"))
	requireExit(t, result, 0, b.stderr.String())
	requireRebaseFields(t, b.record("rebase-wt-r3"), nil, nil, []string{})
	if first := b.p6DirectionFirst("rebase-wt", 3); strings.Contains(first, "fast-forwarded") {
		t.Fatalf("a staged resolution still delivered a conflict paragraph: %q", first)
	}
}

// Lines 4020-4051: a tracked conflict does not make a simultaneous
// untracked-file collision safe. The follow-up refuses by path, restores
// the original worktree (head and both delegate files), creates no child
// record and leaves no tagged stash.
func TestFollowUpIntegrationUntrackedCollisionRefusesAndRestoresTheWorktree(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	tracked, untracked := "metasystem/collision-target.txt", "metasystem/collision-new.txt"
	b.p6TrunkCommit("rebase fixture base", map[string]string{tracked: "base\n"})
	workspace := b.p6WorktreeRound("collision-wt")
	// Lines 4266-4271 and 4325-4327: an ordinary worktree dispatch records
	// its launch mode, defaults its product root to the worktree, and keeps
	// the worktree inside the watcher's scope.
	root := b.record("collision-wt")
	if roots, _ := root["productRoots"].([]any); root["launchMode"] != "worktree" || len(roots) != 1 || roots[0] != workspace ||
		!strings.HasPrefix(workspace, b.root+"/") {
		t.Fatalf("worktree dispatch launchMode=%v productRoots=%v workspace=%s", root["launchMode"], root["productRoots"], workspace)
	}
	from := b.p6Git(workspace, "rev-parse", "HEAD")
	b.writeFile(strings.TrimPrefix(filepath.Join(workspace, tracked), b.root+"/"), "delegate tracked behaviour\n")
	b.writeFile(strings.TrimPrefix(filepath.Join(workspace, untracked), b.root+"/"), "delegate untracked behaviour\n")
	b.p6Complete("collision-wt", "collision-wt", 1, untracked, tracked)
	b.p6TrunkCommit("untracked collision", map[string]string{tracked: "trunk tracked behaviour\n", untracked: "trunk collision behaviour\n"})

	result := b.p6FollowUp("collision-wt", b.p6Message("follow.md"))
	if result.ExitCode == 0 || !strings.Contains(b.stderr.String(), "collided with untracked paths: "+untracked) {
		t.Fatalf("the untracked collision did not refuse by path: exit %d stderr %s", result.ExitCode, b.stderr.String())
	}
	if head := b.p6Git(workspace, "rev-parse", "HEAD"); head != from {
		t.Fatalf("the refusal left the worktree at %s, want %s", head, from)
	}
	if got := b.p6Read(filepath.Join(workspace, tracked)); got != "delegate tracked behaviour\n" {
		t.Fatalf("tracked delegate file not restored: %q", got)
	}
	if got := b.p6Read(filepath.Join(workspace, untracked)); got != "delegate untracked behaviour\n" {
		t.Fatalf("untracked delegate file not restored: %q", got)
	}
	if exists(b.recordPath("collision-wt-r2")) {
		t.Fatal("the refused follow-up created its child record")
	}
	if strings.Contains(b.git("stash", "list", "--format=%gs"), "collision-wt-2-rebase") {
		t.Fatal("the refusal left its tagged stash behind")
	}
}

// Lines 4053-4089: a second wrapper that finds the rebased follow-up still
// running reconstructs the conflict paragraph from the standing record, so
// it binds to the same operation instead of refusing a mismatch, and the
// standing record keeps its rebase fields.
func TestFollowUpIntegrationRepeatedWrapperRebindsARebasedFollowUp(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.p6TaggedLaunches()
	target := "metasystem/repeat-rebase-target.txt"
	b.p6TrunkCommit("rebase fixture base", map[string]string{target: "base\n"})
	workspace := b.p6WorktreeRound("repeat-rebase-wt")
	from := b.p6Git(workspace, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(workspace, target), []byte("delegate repeated behaviour\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b.p6Complete("repeat-rebase-wt", "repeat-rebase-wt", 1, target)
	to := b.p6TrunkCommit("repeated wrapper", map[string]string{target: "trunk repeated behaviour\n"})
	message := b.p6Message("follow.md")

	requireExit(t, b.p6FollowUp("repeat-rebase-wt", message), 0, b.stderr.String())
	if status := b.record("repeat-rebase-wt-r2")["status"]; status != "running" {
		t.Fatalf("the first wrapper's round is %v, want running", status)
	}
	second := b.p6FollowUp("repeat-rebase-wt", message)
	combined := string(second.Stdout) + string(second.Outcome) + b.stderr.String()
	if (second.ExitCode != 0 && second.ExitCode != 3) || strings.Contains(combined, "REFUSED-OPID-MISMATCH") ||
		!(strings.Contains(combined, `"outcome":"BOUND"`) || strings.Contains(combined, `"outcome":"IN-PROGRESS"`)) {
		t.Fatalf("the repeated rebased follow-up did not bind to its standing operation: exit %d output %s", second.ExitCode, combined)
	}
	requireRebaseFields(t, b.record("repeat-rebase-wt-r2"), from, to, []string{target})
	if exists(b.recordPath("repeat-rebase-wt-r3")) {
		t.Fatal("the repeated wrapper reserved another round")
	}
}

// Lines 4091-4138: a path trunk deleted while the chain modified it
// conflicts (modify/delete). The paragraph names it, yet authority
// admission reads only the caller's message, so neither the first wrapper
// nor its repeat is refused as a new authority citation.
func TestFollowUpIntegrationModifyDeleteConflictIsNotAnAuthorityCitation(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.p6TaggedLaunches()
	target := "metasystem/delete-target.txt"
	b.p6TrunkCommit("rebase fixture base", map[string]string{target: "base\n"})
	workspace := b.p6WorktreeRound("delete-rebase-wt")
	from := b.p6Git(workspace, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(workspace, target), []byte("delegate preserved behaviour\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b.p6Complete("delete-rebase-wt", "delete-rebase-wt", 1, target)
	to := b.p6TrunkCommit("delete conflict", map[string]string{target: ""})
	message := b.p6Message("follow.md")

	first := b.p6FollowUp("delete-rebase-wt", message)
	firstStderr := b.stderr.String()
	requireExit(t, first, 0, firstStderr)
	second := b.p6FollowUp("delete-rebase-wt", message)
	combined := string(second.Stdout) + string(second.Outcome) + b.stderr.String()
	if (second.ExitCode != 0 && second.ExitCode != 3) ||
		!(strings.Contains(combined, `"outcome":"BOUND"`) || strings.Contains(combined, `"outcome":"IN-PROGRESS"`)) ||
		strings.Contains(firstStderr+combined, "follow-up brief authority admission refused") {
		t.Fatalf("the modify-delete follow-up or its retry was refused: first %s second exit %d %s", firstStderr, second.ExitCode, combined)
	}
	requireRebaseFields(t, b.record("delete-rebase-wt-r2"), from, to, []string{target})
	if out, err := exec.Command("git", "-C", workspace, "cat-file", "-e", "HEAD:"+target).CombinedOutput(); err == nil {
		t.Fatalf("worktree HEAD still has the deleted path: %s", out)
	}
	firstLine := b.p6DirectionFirst("delete-rebase-wt", 2)
	if !strings.Contains(firstLine, target) || !strings.Contains(firstLine, "stage each resolved path with `git add`") {
		t.Fatalf("the modify-delete paragraph: %q", firstLine)
	}
}

// Lines 4140-4171: authority admission refuses after a conflicting
// fast-forward already happened; the conflict stays in the worktree. The
// corrected wrapper recovers the conflict from the worktree index: no
// rebasedFrom, rebasedTo the worktree head, the unmerged path, and the
// "earlier dispatcher" paragraph.
func TestFollowUpIntegrationRecoversConflictProvenanceAfterAnAuthorityRefusal(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	target := "metasystem/recover-target.txt"
	b.p6TrunkCommit("rebase fixture base", map[string]string{target: "base\n"})
	workspace := b.p6WorktreeRound("recover-wt")
	if err := os.WriteFile(filepath.Join(workspace, target), []byte("delegate recovered behaviour\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b.p6Complete("recover-wt", "recover-wt", 1, target)
	to := b.p6TrunkCommit("authority retry", map[string]string{target: "trunk recovered behaviour\n"})

	refused := b.p6FollowUp("recover-wt", b.p6Message("missing.md", "", "Authority: scripts/agents/missing-after-rebase.md"))
	if refused.ExitCode == 0 || !strings.Contains(b.stderr.String(), "follow-up brief authority admission refused") {
		t.Fatalf("a missing authority was admitted: exit %d stderr %s", refused.ExitCode, b.stderr.String())
	}
	if !strings.Contains(b.p6Read(filepath.Join(workspace, target)), "<<<<<<<") || b.p6Git(workspace, "rev-parse", "HEAD") != to {
		t.Fatal("the authority refusal did not leave the rebased conflict in place")
	}
	if exists(b.recordPath("recover-wt-r2")) {
		t.Fatal("the refused follow-up reserved its round")
	}

	requireExit(t, b.p6FollowUp("recover-wt", b.p6Message("follow.md")), 0, b.stderr.String())
	requireRebaseFields(t, b.record("recover-wt-r2"), nil, to, []string{target})
	first := b.p6DirectionFirst("recover-wt", 2)
	for _, want := range []string{"trunk commit " + to, target, "resolve first, keeping both sides' behaviour"} {
		if !strings.Contains(first, want) {
			t.Fatalf("the recovered paragraph lacks %q: %q", want, first)
		}
	}
}

// Lines 3984-4018: an implementer round cut off at its cap in a worktree
// that is behind trunk on unrelated files is continued in a fresh-context
// round whose prior-worktree paragraph lists the chain's path alone, never
// trunk's.
func TestFollowUpIntegrationCapContinuationListsTheChainsPathsNotTrunks(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	workspace := b.p6WorktreeRound("capped-behind")
	b.writeFile(strings.TrimPrefix(filepath.Join(workspace, "metasystem", "behind-marker.txt"), b.root+"/"), "capped round work\n")
	patch := b.writeFile("capped.json", `{"error":"budget-cap","groupDeathProvenAt":"2026-09-27T12:00:00Z"}`)
	if _, err := dispatch.RecordCAS(b.root, "capped-behind", "running", "timeout", patch); err != nil {
		t.Fatal(err)
	}
	b.p6TrunkCommit("capped behind trunk move", map[string]string{"metasystem/behind-trunk-only.txt": "trunk moved on\n"})

	result := b.p6FollowUp("capped-behind", b.p6Message("follow.md"))
	requireExit(t, result, 0, b.stderr.String())
	paragraph := b.p6Read(filepath.Join(b.root, "artifacts", "agents", "capped-behind", "rounds", "2", "prior-worktree.md"))
	if !strings.Contains(paragraph, "metasystem/behind-marker.txt") || !strings.Contains(paragraph, "1 path(s) changed") {
		t.Fatalf("the continuation paragraph did not list the chain's path alone:\n%s", paragraph)
	}
	if strings.Contains(paragraph, "behind-trunk-only.txt") {
		t.Fatalf("the continuation paragraph named a trunk change as the predecessor's work:\n%s", paragraph)
	}
	child := b.record("capped-behind-r2")
	if child["continuation"] != "after-cap" || child["resumeMode"] != "fresh-context" {
		t.Fatalf("the continuation round is %v/%v, want after-cap/fresh-context", child["continuation"], child["resumeMode"])
	}
	if last := b.launches[len(b.launches)-1]; last.Job != "capped-behind-r2" || last.Verb != "dispatch" {
		t.Fatalf("the continuation launched %s as %s, want a fresh dispatch", last.Job, last.Verb)
	}
}

// Lines 4185-4199 (the subject half), 4208-4260: a design critic's round
// subject is persisted from its reviewed workspace (kind, page, content
// digest, reviewed commit); the follow-up after the page changed reads the
// changed page; the chain closes after its register folds; a host close
// is not stamped runner-closed; and the closed chain refuses a follow-up.
func TestFollowUpIntegrationDesignCriticReadsTheChangedPageAndHostCloseIsNotRunnerClosed(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.writeFile("metasystem.conf", dispatchBedConfig+"evidence.root="+t.TempDir()+"\n")
	page := "metasystem/fixture-admission/close-race.md"
	commit := b.p6TrunkCommit("add close race fixture design", map[string]string{page: "# Close/follow-up race design\n"})
	outputs := b.writeFile("declared-outputs.txt", "metasystem/internal/dispatch/build.go\n")
	brief := b.brief("design-brief.md", "design", "Review the design.")
	result := b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "design-critic", "--outputs", outputs, "--design", page, "--brief", brief, "--job-id", "close-race")
	requireExit(t, result, 0, b.stderr.String())
	subject := func(round int) map[string]any {
		t.Helper()
		var value map[string]any
		if err := json.Unmarshal([]byte(b.p6Read(filepath.Join(b.root, "artifacts", "agents", "close-race", "rounds", itoa(round), "subject.json"))), &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	first := subject(1)
	if first["kind"] != "design" || first["designPath"] != page || first["reviewedCommit"] != commit ||
		first["contentDigest"] != p6SHA256Hex("# Close/follow-up race design\n") {
		t.Fatalf("the design critic subject was not persisted from its reviewed workspace: %v", first)
	}
	b.p6CompleteReturn("close-race", "close-race", 1, map[string]any{"schemaVersion": 3, "jobId": "close-race", "round": 1, "findings": []any{}, "rigor": []any{}, "reviewedCommit": commit})

	changed := "# Close/follow-up race design\n\nFixture design revision for close-race.\n"
	b.writeFile(page, changed)
	requireExit(t, b.p6FollowUp("close-race", b.writeFile("follow.md", "Working Mode: design\n\nReview again.\n")), 0, b.stderr.String())
	second := subject(2)
	if second["designPath"] != page || second["reviewedCommit"] != commit || second["contentDigest"] != p6SHA256Hex(changed) ||
		second["contentDigest"] == first["contentDigest"] {
		t.Fatalf("the follow-up did not read the changed design page: %v", second)
	}
	b.p6CompleteReturn("close-race", "close-race-r2", 2, map[string]any{"schemaVersion": 3, "jobId": "close-race-r2", "round": 2, "findings": []any{}, "rigor": []any{}, "reviewedCommit": commit})
	requireExit(t, b.run("__critique-register-advance", "--root-job", "close-race", "--round-job", "close-race-r2"), 0, b.stderr.String())

	requireExit(t, b.run("close", "--job", "close-race"), 0, b.stderr.String())
	root := b.record("close-race")
	if root["chainClosed"] != true || root["runnerClosed"] != false {
		t.Fatalf("a host close: chainClosed=%v runnerClosed=%v, want true/false", root["chainClosed"], root["runnerClosed"])
	}
	refused := b.p6FollowUp("close-race", b.writeFile("closed.md", "Working Mode: design\n\nOnce more.\n"))
	if refused.ExitCode != 1 || !strings.Contains(b.stderr.String(), "job chain is closed") || exists(b.recordPath("close-race-r3")) {
		t.Fatalf("a closed chain admitted a follow-up: exit %d stderr %s", refused.ExitCode, b.stderr.String())
	}
}

// Lines 4329-4355: a snapshot miss costs one adapter probe, absent or
// stale, and the dispatch proceeds; a probe that cannot heal the miss
// refuses by naming the failed probe.
func TestDispatchIntegrationCapabilitySnapshotMissSelfHealsOnceOrRefuses(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	snapshots := filepath.Join(b.root, "artifacts", "agents", "capabilities")
	clear := func() {
		t.Helper()
		entries, _ := filepath.Glob(filepath.Join(snapshots, "*.json"))
		for _, entry := range entries {
			if err := os.Remove(entry); err != nil {
				t.Fatal(err)
			}
		}
	}
	probes := 0
	b.doubles.Adapter.ProbeFunc = func(string) error {
		probes++
		_, err := adapter.WriteFakeCapabilitySnapshot(snapshots, "current", 0, 20)
		return err
	}
	dispatchJob := func(job string) delegation.Result {
		brief := b.brief(job+".md", "implement", "Do the thing.")
		return b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "implementer", "--brief", brief, "--job-id", job, "--worktree")
	}

	clear()
	requireExit(t, dispatchJob("no-snapshot"), 0, b.stderr.String())
	if entries, _ := filepath.Glob(filepath.Join(snapshots, "*.json")); probes != 1 || len(entries) == 0 {
		t.Fatalf("an absent snapshot was not re-probed once: probes=%d snapshots=%v", probes, entries)
	}

	clear()
	if _, err := adapter.WriteFakeCapabilitySnapshot(snapshots, "current", 31, 20); err != nil {
		t.Fatal(err)
	}
	requireExit(t, dispatchJob("stale-snapshot"), 0, b.stderr.String())
	if probes != 2 {
		t.Fatalf("a stale snapshot was not re-probed: probes=%d", probes)
	}

	clear()
	b.doubles.Adapter.ProbeFunc = func(string) error { return errors.New("probe verb broken") }
	result := dispatchJob("unhealable-snapshot")
	if result.ExitCode == 0 || !strings.Contains(b.stderr.String(), "adapter probe failed") {
		t.Fatalf("an unhealable snapshot miss was admitted: exit %d stderr %s", result.ExitCode, b.stderr.String())
	}
	for _, launch := range b.launches {
		if launch.Job == "unhealable-snapshot" {
			t.Fatal("an unhealable snapshot miss launched")
		}
	}
}

// Lines 4357-4392: under the old capability profile a dispatch records no
// session-established signal and the resume fallback; its follow-up falls
// back to a fresh-context round (a fresh adapter dispatch, not a resume)
// whose prompt embeds the prior brief and return with their provenance.
func TestFollowUpIntegrationOldCapabilitiesFallBackToFreshContext(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	snapshots := filepath.Join(b.root, "artifacts", "agents", "capabilities")
	entries, _ := filepath.Glob(filepath.Join(snapshots, "*.json"))
	for _, entry := range entries {
		if err := os.Remove(entry); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := adapter.WriteFakeCapabilitySnapshot(snapshots, "old", 0, 20); err != nil {
		t.Fatal(err)
	}
	workspace := b.p6WorktreeRound("old-capabilities")
	_ = workspace
	record := b.record("old-capabilities")
	if record["sessionEstablishedSignal"] != false {
		t.Fatalf("the old-capability dispatch claimed a session-established signal: %v", record["sessionEstablishedSignal"])
	}
	resumeFallback := false
	fallbacks, _ := record["capabilityFallbacks"].([]any)
	for _, item := range fallbacks {
		if entry, ok := item.(map[string]any); ok && entry["capability"] == "resume" {
			resumeFallback = true
		}
	}
	if !resumeFallback {
		t.Fatalf("the old-capability dispatch did not record the resume fallback: %v", record["capabilityFallbacks"])
	}
	if record["status"] == "pending" {
		patch := b.writeFile("session.json", `{"sessionId":"fake-session-old-capabilities"}`)
		if _, err := dispatch.RecordCAS(b.root, "old-capabilities", "pending", "running", patch); err != nil {
			t.Fatal(err)
		}
	}
	b.p6Complete("old-capabilities", "old-capabilities", 1)

	requireExit(t, b.p6FollowUp("old-capabilities", b.p6Message("follow.md")), 0, b.stderr.String())
	child := b.record("old-capabilities-r2")
	if child["resumeMode"] != "fresh-context" {
		t.Fatalf("the no-resume follow-up did not fall back to fresh context: %v", child["resumeMode"])
	}
	if last := b.launches[len(b.launches)-1]; last.Job != "old-capabilities-r2" || last.Verb != "dispatch" {
		t.Fatalf("the fresh-context follow-up launched %s as %s, want a fresh dispatch that resumes no session", last.Job, last.Verb)
	}
	prompt := b.p6Read(filepath.Join(b.root, "artifacts", "agents", "old-capabilities", "rounds", "2", "prompt.md"))
	for _, section := range []string{"# Prior Brief", "# Prior Return", "# Task Direction"} {
		if !strings.Contains(prompt, section) {
			t.Fatalf("the fresh-context prompt lost %q", section)
		}
	}
	sources, _ := json.Marshal(child["composition"].(map[string]any)["sources"])
	for _, source := range []string{`"source":"engine:prior-brief"`, `"source":"engine:prior-return"`} {
		if !strings.Contains(string(sources), source) {
			t.Fatalf("the fresh-context composition lost %s: %s", source, sources)
		}
	}
}

func p6SHA256Hex(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
