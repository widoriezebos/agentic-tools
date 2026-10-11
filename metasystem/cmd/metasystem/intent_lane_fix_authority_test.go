package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

func laneAuthorityHuman(b *laneFixBed) {
	prove := func(root string, pid int64, reader humanauthority.Reader, now time.Time) (humanauthority.Proof, error) {
		return fixedFixtureGoalAuthority(root, pid, reader, "", "", now)
	}
	b.owners.dependencies.proveHuman = prove
	b.owners.dependencies.proveTerminal = prove
	b.owners.delivery.now = b.manager.Now
	b.owners.delivery.laneRoot = func(string, time.Time) (string, bool, error) { return "", false, nil }
}

func expireLaneAuthority(file *goal.GoalFile) {
	file.Approved.Authority = goal.ApprovalAuthorityRelayed
	file.Approved.ReviewBy = "2026-08-31"
	for i := range file.History {
		if file.History[i].Opid == file.Approved.Opid {
			file.History[i].AuthorityOutcome = goal.AuthorityOutcomeTemporaryHumanWord
			file.History[i].AuthorityReviewBy = file.Approved.ReviewBy
			file.History[i].AuthorityRuling = humanauthority.TemporaryWordRuling
			file.History[i].TemporaryHumanWord = "Wido authorizes this relayed approval"
		}
	}
}

func TestLaneFixParkedGoalRefusesBuild(t *testing.T) {
	t.Parallel()
	b := newLaneFixBed(t)
	laneAuthorityHuman(b)
	if code, result := b.run("goal", "pause", b.id, "--reason", "Wait for my decision", "--by", "Wido", "--lineage", "m1"); code != 0 {
		t.Fatalf("park: %d %s", code, result.Summary)
	}
	if file := b.goalFile(b.id); file.State != goal.StateParked || file.Parked.By != "human:Wido" {
		t.Fatalf("human park did not stand: %+v", file)
	}
	args := []string{"work", "build", b.id, "--work", b.fixUnit(), "--brief", b.brief("fix.md", "Repair the fixture.\n"), "--lines", "2", "--check", "metasystem test impact"}
	if code, result := b.run(args...); code != 1 || !strings.Contains(result.Summary, "it is parked") {
		t.Fatalf("parked build: %d %s", code, result.Summary)
	}
	records, err := b.manager.Store.List()
	if err != nil || len(records) != 0 {
		t.Fatalf("parked build started jobs: %+v %v", records, err)
	}
	if _, err := os.Stat(filepath.Join(b.root(), "code.go")); !os.IsNotExist(err) {
		t.Fatalf("parked builder wrote code: %v", err)
	}
	if code, result := b.run("goal", "resume", b.id, "--by", "Wido", "--lineage", "m1"); code != 0 {
		t.Fatalf("unpark: %d %s", code, result.Summary)
	}
	if code, result := b.run("goal", "claim", b.id, "--by", "Wido", "--lineage", "m1"); code != 0 {
		t.Fatalf("claim again: %d %s", code, result.Summary)
	}
	if code, result := b.run(args...); code != 0 {
		t.Fatalf("reclaimed build: %d %s", code, result.Summary)
	}
}

func TestLaneFixExpiredApprovalRefusesBuild(t *testing.T) {
	t.Parallel()
	b := newLaneFixBedWith(t, func(file *goal.GoalFile) {
		workApprovedBox(file)
		expireLaneAuthority(file)
	})
	if code, result := b.run("work", "build", b.id, "--work", b.fixUnit(), "--brief", b.brief("fix.md", "Repair the fixture.\n"), "--lines", "2", "--check", "metasystem test impact"); code != 1 || !strings.Contains(result.Summary, "no longer admits new work") {
		t.Fatalf("expired approval build: %d %s", code, result.Summary)
	}
	records, err := b.manager.Store.List()
	if err != nil || len(records) != 0 {
		t.Fatalf("expired approval started jobs: %+v %v", records, err)
	}
}

func TestLaneFixAuthorityWithRealLedgerAndContract(t *testing.T) {
	t.Parallel()
	paths, err := filepath.Glob("../../plans/goals/*.md")
	if err != nil {
		t.Fatal(err)
	}
	var live *goal.GoalFile
	var source string
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		file, problems := goal.ParseFile(raw)
		if len(problems) == 0 && file.Approved != nil && file.Budget != nil && file.Risk != nil {
			live, source = file, path
			break
		}
	}
	if live == nil {
		t.Fatal("the real ledger has no approved goal with risk and budget")
	}
	raw, err := os.ReadFile("../../testing.json")
	if err != nil {
		t.Fatal(err)
	}
	contract, err := testpolicy.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := testpolicy.Select(contract, testpolicy.SelectionRequest{
		ChangedPaths:  []string{"metasystem/cmd/metasystem/intent_work.go", "metasystem/cmd/metasystem/goal_branch.go", "metasystem/cmd/metasystem/intent_lane_fix.go"},
		GoalRisk:      testpolicy.GoalRisk{Severity: int(live.Risk.Severity), Novelty: int(live.Risk.Novelty), Exposure: int(live.Risk.Exposure), Accumulation: int(live.Risk.Accumulation)},
		RequestedMode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDiagnostic,
	})
	if err != nil {
		t.Fatal(err)
	}
	output := testrun.PlanOutputOf(testrun.Preparation{EffectiveContract: contract, Plan: plan})
	if len(output.Groups) == 0 || len(output.Plan.RequiredGroups) == 0 {
		t.Fatalf("real contract emitted no checks: %+v", output)
	}
	b := newLaneFixBedWith(t, func(file *goal.GoalFile) {
		box := *live.Budget
		box.ElapsedLimit, box.ReviewRoundLimit = "1m", 0
		file.Budget, file.Risk, file.Tier = &box, live.Risk, live.Tier
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, box, file.Risk)
		expireLaneAuthority(file)
	})
	if err := os.WriteFile(filepath.Join(b.root(), "testing.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if code, result := b.run("work", "build", b.id, "--work", b.fixUnit(), "--brief", b.brief("fix.md", "Repair the fixture.\n"), "--lines", "2", "--check", "metasystem test impact"); code != 1 || !strings.Contains(result.Summary, "no longer admits new work") {
		t.Fatalf("real ledger budget bypassed expired authority: %d %s", code, result.Summary)
	}
	records, err := b.manager.Store.List()
	if err != nil || len(records) != 0 {
		t.Fatalf("real ledger refusal started jobs: %+v %v", records, err)
	}
	t.Logf("real ledger=%s tier=%d; real plan mode=%s required=%v", source, live.Tier, output.Plan.ExecutedMode, output.Plan.RequiredGroups)
}

// The read owner and its goal-free delegate run in their own process so their
// checkout, ledger projection and authenticated batch custody are real.
func init() {
	testHelperCommands["fixture-lane-authority-read"] = func(args []string) int {
		root, id, commit, path, mode := args[0], args[1], args[2], args[3], args[4]
		delegate := func(_ delegateRequest, stdout, _ io.Writer) int {
			if err := os.WriteFile(path, []byte("started"), 0600); err != nil {
				return 1
			}
			fmt.Fprintln(stdout, `{"outcome":"WON","jobId":"authority-read"}`)
			return 0
		}
		var err error
		if mode == "delegate" {
			_, err = readDelegate(delegate, root, "brief.md", id, commit, "", "")
		} else {
			_, _, err = goalBranchReadRun([]string{"--root", root, "--goal", id, "--unit", commit}, goalBranchReadDependencies{
				Gate: func(string) (string, error) { return "green", nil }, Delegator: delegate,
			})
		}
		if err != nil {
			var neverLaunched *branch.ReadNeverLaunchedError
			if mode == "delegate" && !errors.As(err, &neverLaunched) {
				fmt.Fprintln(os.Stderr, "the refused read was not classified as never started")
				return 1
			}
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
}

// Publish only the fixture's goal page into its accepted Git ref, leaving the
// detached batch commit and its index intact. The command fixture owns the
// human transitions; Git supplies the immutable ledger the read process sees.
func publishLaneAuthorityGoal(t *testing.T, b *laneFixBed) {
	t.Helper()
	index := filepath.Join(t.TempDir(), "index")
	git := func(input []byte, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", b.root()}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+index)
		cmd.Stdin = strings.NewReader(string(input))
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("ledger git %q: %v %s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	git(nil, "read-tree", goal.AcceptedRef)
	blob := git(goal.RenderFile(b.goalFile(b.id)), "hash-object", "-w", "--stdin")
	git(nil, "update-index", "--add", "--cacheinfo", "100644,"+blob+",plans/goals/"+b.id+".md")
	tree := git(nil, "write-tree")
	commit := git(nil, "commit-tree", tree, "-p", goal.AcceptedRef, "-m", "fixture human decision")
	git(nil, "update-ref", goal.AcceptedRef, commit)
	git(nil, "push", "origin", commit+":refs/heads/main")
}

func TestLaneFixGoalAuthorityCommittedReadRealProcess(t *testing.T) {
	t.Parallel()
	for _, decision := range []string{"park", "withdraw", "expire"} {
		t.Run(decision, func(t *testing.T) {
			t.Parallel()
			b := newLaneFixBed(t)
			laneAuthorityHuman(b)
			if code, result := b.run("work", "build", b.id, "--work", b.fixUnit(), "--brief", b.brief("fix.md", "Repair the fixture.\n"), "--lines", "2", "--check", "metasystem test impact"); code != 0 {
				t.Fatalf("build: %d %s", code, result.Summary)
			}
			if code, result := b.run("work", "review", b.id, "--work", b.fixUnit()); code != 0 {
				t.Fatalf("review: %d %s", code, result.Summary)
			}
			switch decision {
			case "park":
				if code, result := b.run("goal", "pause", b.id, "--reason", "Wait for my decision", "--by", "Wido", "--lineage", "m1"); code != 0 {
					t.Fatalf("park: %d %s", code, result.Summary)
				}
			case "withdraw":
				if code, result := b.run("goal", "unapprove", b.id, "--reason", "Withdraw permission", "--by", "Wido", "--lineage", "m1"); code != 0 {
					t.Fatalf("withdraw: %d %s", code, result.Summary)
				}
			case "expire":
				file := b.goalFile(b.id)
				expireLaneAuthority(file)
				b.addGoal(file)
			}
			publishLaneAuthorityGoal(t, b)
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			fixture := pinProofBinaryFixture(t, b.root())
			commit := connectionGit(t, b.root(), "rev-parse", "HEAD")
			marker := filepath.Join(t.TempDir(), "started")
			read := func(mode string) ([]byte, error) {
				cmd := fixture.command(append(os.Environ(), "GO_WANT_BATCH_E2E_COMMAND=1", "METASYSTEM_SUPERVISION_REGISTRY_HOME="+b.registry), executable, "fixture-lane-authority-read", b.root(), b.id, commit, marker, mode)
				cmd.Dir = b.root()
				return cmd.CombinedOutput()
			}
			for _, mode := range []string{"owner", "delegate"} {
				output, err := read(mode)
				want := "the owned claim is unreadable"
				if decision == "expire" {
					want = "no longer admits new work"
				}
				if err == nil || !strings.Contains(string(output), want) {
					t.Fatalf("%s %s read: %v %s", decision, mode, err, output)
				}
				if _, err := os.Stat(marker); !os.IsNotExist(err) {
					t.Fatalf("refused %s read started a job: %v", mode, err)
				}
			}
			if decision == "park" {
				for _, verb := range []string{"resume", "claim"} {
					if code, result := b.run("goal", verb, b.id, "--by", "Wido", "--lineage", "m1"); code != 0 {
						t.Fatalf("%s again: %d %s", verb, code, result.Summary)
					}
				}
				publishLaneAuthorityGoal(t, b)
				for _, mode := range []string{"owner", "delegate"} {
					if output, err := read(mode); err != nil {
						t.Fatalf("reclaimed %s read: %v %s", mode, err, output)
					}
				}
			}
		})
	}
}
