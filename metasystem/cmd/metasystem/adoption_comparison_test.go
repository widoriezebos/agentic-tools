package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// This is the Go-owned specimen behind adopt-fixtures.sh --comparison. It
// executes the original Bash assertion bodies with actual source-specific
// delivery attempts. No ancestor-shaped terminal fact or inherited witness
// supplies authority to the adopted installations.
func TestAdoptionComparisonSelectedScenarios(t *testing.T) {
	if os.Getenv("METASYSTEM_ADOPTION_COMPARISON") != "1" {
		t.Skip("explicit adopt-fixtures.sh --comparison specimen only")
	}
	providedTarget, providedKind := os.Getenv("METASYSTEM_ADOPTION_TARGET"), os.Getenv("METASYSTEM_ADOPTION_KIND")
	if providedTarget != "" && providedKind != "filled" && providedKind != "copied" {
		t.Fatal("unknown existing adoption target kind")
	}
	selected, err := proofrun.FixtureScenarios("adoption", "comparison")
	want := []string{"filled-target-delivery", "copied-registration-setup", "copied-registration-positive", "copy-drift-source", "copy-drift-registration"}
	if err != nil || !reflect.DeepEqual(selected, want) {
		t.Fatalf("adoption scenario selection: %v, %v", selected, err)
	}
	source, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := proofrun.Freeze(source)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(frozen.Root)) })
	source = frozen.Root
	bed, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if providedTarget != "" {
		bed, err = filepath.EvalSymlinks(os.Getenv("METASYSTEM_ADOPTION_FIXTURE_ROOT"))
		if err != nil {
			t.Fatal(err)
		}
		providedTarget, err = filepath.EvalSymlinks(providedTarget)
		if err != nil || !strings.HasPrefix(providedTarget, bed+string(filepath.Separator)) {
			t.Fatal("adoption target must be inside its fixture root")
		}
	}
	t.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", filepath.Join(bed, "registry"))
	environment := append(receiptCanaryEnvironment(), "METASYSTEM_OWNER_LINEAGE=adoption-comparison", "METASYSTEM_SUPERVISION_REGISTRY_HOME="+filepath.Join(bed, "registry"))
	run := func(cwd string, argv ...string) (string, int) {
		t.Helper()
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir, cmd.Env = cwd, environment
		started := time.Now()
		output, err := cmd.CombinedOutput()
		code := 0
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				code = exit.ExitCode()
			} else {
				t.Fatalf("start %q in %s: %v", argv, cwd, err)
			}
		}
		t.Logf("command=%q cwd=%s status=%d elapsed=%s\n%s", argv, cwd, code, time.Since(started), output)
		return string(output), code
	}
	mustRun := func(cwd string, argv ...string) string {
		t.Helper()
		output, code := run(cwd, argv...)
		if code != 0 {
			// Retain the native failure before the disposable target is removed.
			attempts, _ := proofrun.ReadAttempts(cwd)
			for _, attempt := range attempts {
				if attempt.TestResult == nil {
					continue
				}
				for _, group := range attempt.TestResult.Groups {
					if group.Status != "passed" && group.LogPath != "" {
						log, err := os.ReadFile(group.LogPath)
						t.Logf("native group %s status=%s log=%s read=%v\n%s", group.ID, group.Status, group.LogPath, err, log)
					}
				}
			}
			t.Fatalf("command %q failed with %d:\n%s", argv, code, output)
		}
		return output
	}
	helper := func(function string, args ...string) string {
		t.Helper()
		argv := []string{"bash", "-c", `set -euo pipefail; source "$1"; tmp=$2; shift 2; "$@"`, "adoption-assertion", filepath.Join(source, "scripts", "adopt-fixture-helpers.sh"), bed, function}
		return mustRun(source, append(argv, args...)...)
	}
	runReceiptGit(t, source, "init", "-q", "-b", "main")
	runReceiptGit(t, source, "config", "user.name", "adoption source")
	runReceiptGit(t, source, "config", "user.email", "fixture@example.invalid")
	runReceiptGit(t, source, "config", "metasystem.goal.machine", "fixture-machine")
	runReceiptGit(t, source, "add", ".")
	runReceiptGit(t, source, "commit", "-qm", "exact comparison source")
	if providedTarget == "" {
		commit := runReceiptGit(t, source, "rev-parse", "HEAD")
		mustRun(source, "go", "build", "-buildvcs=false", "-ldflags", "-X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp="+commit, "-o", filepath.Join(source, "bin", "metasystem"), "./cmd/metasystem")
	}

	prepare := func(name, runtimes string, copySkills bool) string {
		t.Helper()
		target := filepath.Join(bed, name)
		if providedTarget != "" {
			target = providedTarget
		}
		if err := os.MkdirAll(target, 0755); err != nil {
			t.Fatal(err)
		}
		runReceiptGit(t, target, "init", "-q", "-b", "main")
		runReceiptGit(t, target, "config", "user.name", "adoption fixture")
		runReceiptGit(t, target, "config", "user.email", "fixture@example.invalid")
		if providedTarget == "" {
			writeReceiptFixture(t, target, "README.md", "application source\n")
			runReceiptGit(t, target, "add", ".")
			runReceiptGit(t, target, "commit", "-qm", "application base")
			argv := []string{"bash", filepath.Join(source, "scripts", "adopt.sh"), target, "--runtimes", runtimes}
			if copySkills {
				argv = append(argv, "--copy-skills")
			}
			mustRun(source, argv...)
			helper("fill_harness_conf", filepath.Join(target, "metasystem.conf"), filepath.Join(bed, name+"-comparison-evidence"))
			helper("fill_harness_testing_contract", filepath.Join(source, "testing.json"), filepath.Join(target, "testing.json"))
			if name == "filled" {
				// Only the filled target carries a covenant: its evidence table lives
				// under docs/, which the copied target's payload digest must match.
				helper("prepare_filled_target_covenant", target)
			}
			rules := filepath.Join(target, "docs", "project-rules.md")
			data, err := os.ReadFile(rules)
			if err != nil {
				t.Fatal(err)
			}
			// This is the ordinary fixture's exact placeholder tailoring.
			for {
				start := strings.IndexByte(string(data), '<')
				if start < 0 {
					break
				}
				end := strings.IndexByte(string(data[start:]), '>')
				if end < 0 {
					break
				}
				data = append(append(append([]byte{}, data[:start]...), []byte("filled")...), data[start+end+1:]...)
			}
			if err := os.WriteFile(rules, data, 0644); err != nil {
				t.Fatal(err)
			}
		} else if name == "filled" {
			helper("prepare_filled_target_covenant", target)
		}
		var contract testpolicy.Contract
		data, err := os.ReadFile(filepath.Join(target, "testing.json"))
		if err != nil || json.Unmarshal(data, &contract) != nil {
			t.Fatalf("read filled contract: %v", err)
		}
		if name == "filled" {
			contract.Always.Standard = append(contract.Always.Standard, "section/covenant-evidence-pre-rebuild", "section/covenant-evidence-post-rebuild")
		}
		data, err = json.MarshalIndent(contract, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		writeReceiptFixture(t, target, "testing.json", string(data)+"\n")
		seedAdoptionComparisonGoal(t, target)
		runReceiptGit(t, target, "add", "-A")
		// Fixture genesis precedes the public proof; subsequent commands keep
		// adoption's installed hooks and use the accepted local goal.
		runReceiptGit(t, target, "-c", "core.hooksPath=/dev/null", "commit", "-qm", "reviewed filled initialization and fixture goal")
		commit := runReceiptGit(t, target, "rev-parse", "HEAD")
		remote := filepath.Join(bed, name+"-origin.git")
		runReceiptGit(t, bed, "init", "-q", "--bare", remote)
		runReceiptGit(t, target, "remote", "add", "origin", remote)
		runReceiptGit(t, target, "push", "-q", "-u", "origin", "main")
		runReceiptGit(t, target, "update-ref", goal.LocalLedgerBranch, commit)
		runReceiptGit(t, target, "update-ref", goal.AcceptedRef, commit)
		runReceiptGit(t, target, "config", "metasystem.steward.landing-ref", "refs/remotes/origin/main")
		// The actual reviewed source is committed before resolving the policy
		// base, and these compiled bytes carry that source's exact commit stamp.
		engine := filepath.Join(target, "bin", "metasystem")
		mustRun(target, "go", "build", "-buildvcs=false", "-ldflags", "-X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp="+commit, "-o", engine, "./cmd/metasystem")
		digest, err := fileSHA256(engine)
		if err != nil {
			t.Fatal(err)
		}
		if err := steward.MintIdentity(steward.RepoIdentityPath(target), steward.InstallIdentity{RepoIdentity: target, Generation: 1, InstallPath: engine, InstallDigest: "sha256:" + digest, MintedAt: time.Now().UTC().Format(time.RFC3339), Enrollment: steward.EnrollmentFixture, EngineBuild: commit, LandedCommit: commit, LandingRef: "refs/remotes/origin/main"}); err != nil {
			t.Fatal(err)
		}
		pid := int64(os.Getpid())
		started, ok := lease.StartedAt(pid, nil)
		if !ok {
			t.Fatal("adoption fixture main identity is unreadable")
		}
		if _, err := lease.Announce(target, "adoption-"+name, pid, started, "adoption-"+name, "codex", "adoption-comparison"); err != nil {
			t.Fatal(err)
		}
		tree := runReceiptGit(t, target, "write-tree")
		first := filepath.Join(bed, name+"-first.json")
		mustRun(target, engine, "test", "run", "--root", target, "--goal", "adoption-goal", "--tree", tree, "--mode", "auto", "--purpose", "delivery", "--cap-min", "10", "--result", first)
		mustRun(target, engine, "test", "verify", "--root", target, "--goal", "adoption-goal", "--tree", tree)
		repeat := filepath.Join(bed, name+"-repeat.json")
		output, code := run(target, engine, "test", "run", "--root", target, "--goal", "adoption-goal", "--tree", tree, "--mode", "auto", "--purpose", "delivery", "--cap-min", "10", "--result", repeat)
		if code != proofrun.ExitReusableSuccess {
			t.Fatalf("completed prerequisite was not reused: status=%d\n%s", code, output)
		}
		var original, reused proofrun.LaunchResult
		firstBytes, e1 := os.ReadFile(first)
		repeatBytes, e2 := os.ReadFile(repeat)
		if e1 != nil || e2 != nil || json.Unmarshal(firstBytes, &original) != nil || json.Unmarshal(repeatBytes, &reused) != nil || original.AttemptID == "" || original.AttemptID != reused.AttemptID {
			t.Fatal("prerequisite reuse lost exact terminal attempt authority")
		}
		return target
	}
	if providedTarget != "" {
		target := prepare(providedKind, "", false)
		if providedKind == "filled" {
			helper("assert_filled_target_delivery", target, "shared-testing")
			helper("assert_filled_target_mutations", target)
		} else {
			helper("assert_copied_registration_positive", source, target)
			helper("assert_copied_registration_drift", target)
			helper("assert_copied_registration_orphan", target)
		}
		return
	}
	filled := prepare("filled", "claude", false)
	helper("assert_filled_target_delivery", filled, "shared-testing")
	output, err := os.ReadFile(filepath.Join(bed, "adopt-filled.out"))
	if err != nil || !strings.Contains(string(output), "completed schema-2 delivery proof verified") {
		t.Fatalf("filled delivery did not consume its own migrated prerequisite: %v\n%s", err, output)
	}
	copied := prepare("copied", "claude,codex", true)
	helper("assert_copied_registration_positive", source, copied)
	helper("assert_copied_registration_drift", copied)
}

func seedAdoptionComparisonGoal(t *testing.T, root string) {
	t.Helper()
	runReceiptGit(t, root, "config", "metasystem.goal.machine", "fixture-machine")
	runReceiptGit(t, root, "config", "goal.sync-remote", "local")
	runReceiptGit(t, root, "config", "goal.sync-branch", goal.LocalLedgerBranch)
	now := time.Now().UTC()
	budget := &goal.Budget{ElapsedLimit: "4h", AttemptLimit: 8, ReservedJobMinutesLimit: 240, ActiveJobLimit: 1, ReviewRoundLimit: 0}
	risk := &goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "The fixture validates one isolated adopted installation."}
	intent := "Validate the actual adopted delivery contract."
	g := &goal.GoalFile{Id: "adoption-goal", State: goal.StateClaimed, Tier: 1, Risk: risk, Intent: intent, Origin: goal.OriginMain, NextStep: "Run delivery.", OpenedAt: now.Add(-2 * time.Minute).Format(time.RFC3339), Revision: 3, Budget: budget,
		Claimed:        &goal.ClaimRecord{Machine: "fixture-machine", Lineage: "adoption-comparison", At: now.Add(-time.Minute).Format(time.RFC3339), Revision: 2, AccountingRevision: 2},
		Approved:       &goal.ApprovalRecord{By: "human:fixture", At: now.Add(-30 * time.Second).Format(time.RFC3339), Revision: 3, Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAZ", "fixture-machine", "adoption-comparison"), Authority: goal.ApprovalAuthorityProven, Digest: goal.ApprovalDigest(intent, 1, *budget, risk)},
		StopCapability: &goal.StopCapability{Generation: 3, Revision: 2, Machine: "fixture-machine", ClaimEpoch: 1}}
	for i, verb := range []string{"open", "claim", "approve"} {
		actor, at := "fixture-machine+adoption-comparison", g.OpenedAt
		opid := goal.Opid(fmt.Sprintf("01ARZ3NDEKTSV4RRFFQ69G5FA%d", i), "fixture-machine", "adoption-comparison")
		if verb == "claim" {
			at = g.Claimed.At
		} else if verb == "approve" {
			actor, at, opid = "human:fixture", g.Approved.At, g.Approved.Opid
		}
		g.History = append(g.History, goal.HistoryLine{At: at, Opid: opid, Verb: verb, Actor: actor, Keep: -1})
	}
	writeReceiptFixture(t, root, "plans/goals/backlog.md", string(goal.RenderRoot(&goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1})))
	writeReceiptFixture(t, root, "plans/goals/adoption-goal.md", string(goal.RenderFile(g)))
}
