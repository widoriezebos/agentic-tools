package testrun

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/digest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginecause"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// joinPlanningBed is the layout a lane join plans in: a lane checkout whose
// installation is nested (checkout/metasystem), enrolled with an engine
// stamped at the lane's tip; a planning worktree pinned (config --worktree)
// to the open batch's base; and the receipt worktree git adds from it, which
// inherits that pin and borrows the checkout's enrollment.
type joinPlanningBed struct {
	checkout, installation, receipt  string
	batchBase, engineBuild, goalOnly string
}

func newJoinPlanningBed(t *testing.T) joinPlanningBed {
	t.Helper()
	top, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	checkout := filepath.Join(top, "lane")
	installation := filepath.Join(checkout, "metasystem")
	git := func(dir string, args ...string) string {
		return strings.TrimSpace(testingFixtureGit(t, dir, append([]string{"-c", "user.name=Test", "-c", "user.email=test@example.invalid"}, args...)...))
	}
	write := func(path, body string) {
		writeTestingFixtureFile(t, filepath.Join(installation, path), []byte(body), 0o644)
	}
	write("metasystem.conf", "metasystem.runtimes=fake\n")
	write("internal/engine.go", "package engine // base\n")
	write(".gitignore", "artifacts/\nbin/\n")
	git(checkout, "init", "-q")
	git(checkout, "add", ".")
	git(checkout, "commit", "-q", "-m", "batch base")
	batchBase := git(checkout, "rev-parse", "HEAD")
	write("internal/engine.go", "package engine // changed after the batch opened\n")
	git(checkout, "commit", "-q", "-am", "engine change")
	engineBuild := git(checkout, "rev-parse", "HEAD")
	write("plans/goals/a-goal.md", "claimed\n")
	git(checkout, "add", ".")
	git(checkout, "commit", "-q", "-m", "goal claim a-goal")
	goalOnly := git(checkout, "rev-parse", "HEAD")

	engine := filepath.Join(installation, "bin", "metasystem")
	writeTestingFixtureFile(t, engine, []byte("#!/bin/sh\nexit 0\n"+enginebuild.StampRecord(engineBuild)+"\n"), 0o755)
	digest := fileDigestForEnrollment(t, engine)
	if err := steward.MintIdentity(steward.RepoIdentityPath(installation), steward.InstallIdentity{
		RepoIdentity: installation, Generation: 1, InstallPath: engine, InstallDigest: digest,
		MintedAt: time.Now().UTC().Format(time.RFC3339), MintedBy: "human-terminal",
		Enrollment: steward.EnrollmentHumanTerminal, EngineBuild: engineBuild,
	}); err != nil {
		t.Fatal(err)
	}

	planning := filepath.Join(top, "planning")
	git(checkout, "config", "extensions.worktreeConfig", "true")
	git(checkout, "worktree", "add", "-q", "--detach", planning, batchBase)
	git(checkout, "update-ref", "refs/remotes/metasystem-batch/"+batchBase, batchBase)
	git(planning, "config", "--worktree", "metasystem.steward.landing-ref", "refs/remotes/metasystem-batch/"+batchBase)
	receipt := filepath.Join(top, "metasystem-landing-receipt.1", "worktree-1")
	git(planning, "worktree", "add", "-q", "--detach", receipt, "HEAD")
	return joinPlanningBed{checkout: checkout, installation: installation, receipt: filepath.Join(receipt, "metasystem"),
		batchBase: batchBase, engineBuild: engineBuild, goalOnly: goalOnly}
}

func fileDigestForEnrollment(t *testing.T, path string) string {
	t.Helper()
	sum, err := digest.FileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	return "sha256:" + sum
}

// TestJoinPlanningOnAnOlderBatchBaseNamesTheCauseAndTheRealCheckout is the
// 2026-10-01 refusal: a join planned on an open batch whose base predates the
// lane's engine. The receipt worktree inherits the batch-base pin, so the
// policy base is that older commit and the enrolled engine is not landed on
// it. The refusal keeps its plain line, carries the underlying judgment in
// its detail, and its command names the lane checkout, never the temporary
// receipt worktree. A goal-only commit after the engine's build is accepted.
func TestJoinPlanningOnAnOlderBatchBaseNamesTheCauseAndTheRealCheckout(t *testing.T) {
	t.Parallel()
	bed := newJoinPlanningBed(t)
	base, err := TrustedPolicyBase(filepath.Dir(bed.receipt), gittree.Workspace{Dir: filepath.Dir(bed.receipt)})
	if err != nil {
		t.Fatal(err)
	}
	if base != bed.batchBase {
		t.Fatalf("the receipt worktree's policy base = %s, want the batch base %s it inherits", base, bed.batchBase)
	}
	if _, _, _, err := TrustedPolicyEngine(bed.receipt, bed.goalOnly, false); err != nil {
		t.Fatalf("a goal-only commit after the engine's build refused the pinned engine: %v\n%s", err, enginecause.Detail(err))
	}
	_, _, _, err = TrustedPolicyEngine(bed.receipt, base, false)
	if err == nil {
		t.Fatal("an engine newer than the batch base was trusted to plan on it")
	}
	t.Logf("refusal:\n%s\ndetail: %s", err, enginecause.Detail(err))
	lines := strings.Split(err.Error(), "\n")
	if len(lines) != 2 || lines[0] != "the pinned engine was not built from the landing branch this run tests against" || !strings.HasPrefix(lines[1], "run: ") {
		t.Fatalf("the refusal is not its two plain lines: %q", err.Error())
	}
	if strings.Contains(err.Error(), "metasystem-landing-receipt") || strings.Contains(err.Error(), bed.receipt) {
		t.Fatalf("the command names the temporary receipt worktree: %q", lines[1])
	}
	if !strings.Contains(lines[1], bed.installation) {
		t.Fatalf("the command does not name the lane checkout %s: %q", bed.installation, lines[1])
	}
	detail := enginecause.Detail(err)
	if !strings.Contains(detail, "which is not landed on "+bed.batchBase) || !strings.Contains(detail, bed.engineBuild) {
		t.Fatalf("the detail lacks the underlying judgment: %q", detail)
	}
}
