package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractmerge"
)

// The layout goldens of groups G3 (disk and evidence) and G6 (the
// passthroughs): each verb driven in-process on a fixture whose output is
// the same on every run (no measured volume, no random id in the words).
var _ = addLayoutCases(
	layoutCase{name: "disk-show", args: []string{"disk", "show"}, bed: diskLayoutBed(true)},
	layoutCase{name: "disk-show-verbose", args: []string{"disk", "show", "--verbose"}, bed: diskLayoutBed(true)},
	layoutCase{name: "disk-show-empty", args: []string{"disk", "show"}, bed: diskLayoutBed(false)},
	layoutCase{name: "disk-show-refusal", args: []string{"disk", "show"}, bed: outsideLayoutBed},
	layoutCase{name: "disk-clean", args: []string{"disk", "clean"}, bed: diskLayoutBed(false)},
	layoutCase{name: "disk-clean-refusal", args: []string{"disk", "clean", "--preview", "--strays"}, bed: diskLayoutBed(false)},
	layoutCase{name: "evidence-show", args: []string{"evidence", "show"}, bed: evidenceLayoutBed},
	layoutCase{name: "evidence-show-all", args: []string{"evidence", "show", "--all"}, bed: evidenceLayoutBed},
	layoutCase{name: "evidence-export", args: []string{"evidence", "export", "old-chain", "--to", "EXPORTS"}, bed: evidenceLayoutBed},
	layoutCase{name: "evidence-export-refusal", args: []string{"evidence", "export", "old-chain"}, bed: evidenceLayoutBed},
	layoutCase{name: "evidence-dispose-refusal", args: []string{"evidence", "dispose", "old-chain"}, bed: evidenceLayoutBed},

	layoutCase{name: "receipt-add", args: []string{"receipt", "add", "--type", "implement", "--outcome", "shipped", "--goal", "verbs-match-intent",
		"--root", "ROOT", "--file", "LEDGER"}, bed: passthroughLayoutBed(0), noJSON: true},
	layoutCase{name: "receipt-add-refusal", args: []string{"receipt", "add", "--type", "implement", "--root", "ROOT", "--file", "LEDGER"},
		bed: passthroughLayoutBed(0), noJSON: true},
	layoutCase{name: "receipt-status", args: []string{"receipt", "status", "--root", "ROOT", "--file", "LEDGER"}, bed: passthroughLayoutBed(3), noJSON: true},
	layoutCase{name: "receipt-status-due", args: []string{"receipt", "status", "--root", "ROOT", "--file", "LEDGER"}, bed: passthroughLayoutBed(26), noJSON: true},
	layoutCase{name: "receipt-status-uncovered", args: []string{"receipt", "status", "--uncovered", "--root", "ROOT", "--file", "LEDGER"}, bed: passthroughLayoutBed(2)},
	layoutCase{name: "receipt-retro", args: []string{"receipt", "retro", "kept 3, reverted 1", "--root", "ROOT", "--file", "LEDGER"},
		bed: passthroughLayoutBed(3), noJSON: true},
	layoutCase{name: "experiment-record-refusal", args: []string{"experiment", "record", "--file", "FRONTIER"}, bed: passthroughLayoutBed(0), noJSON: true},
	layoutCase{name: "experiment-challenge", args: []string{"experiment", "challenge", "--score", "0.9", "--file", "FRONTIER"}, bed: passthroughLayoutBed(0), noJSON: true},
	layoutCase{name: "experiment-challenge-refusal", args: []string{"experiment", "challenge", "--score", "0.821", "--file", "FRONTIER"},
		bed: passthroughLayoutBed(0), noJSON: true},
	layoutCase{name: "experiment-status", args: []string{"experiment", "status", "--file", "FRONTIER"}, bed: passthroughLayoutBed(0), noJSON: true},
	layoutCase{name: "experiment-check", args: []string{"experiment", "check", "--file", "INVESTIGATION"}, bed: passthroughLayoutBed(0), noJSON: true},
	layoutCase{name: "experiment-check-refusal", args: []string{"experiment", "check", "--file", "STOPPED"}, bed: passthroughLayoutBed(0), noJSON: true},
	layoutCase{name: "session-status", args: []string{"session", "status", "--id", "1", "--root", "ROOT"}, bed: stopReportLayoutBed, noJSON: true},
	layoutCase{name: "session-status-verbose", args: []string{"session", "status", "--id", "1", "--root", "ROOT", "--verbose"}, bed: stopReportLayoutBed, noJSON: true},
	layoutCase{name: "session-status-refusal", args: []string{"session", "status", "--root", "ROOT"}, bed: passthroughLayoutBed(0), noJSON: true},
	layoutCase{name: "session-handoff-refusal", args: []string{"session", "handoff", "--root", "ROOT"}, bed: passthroughLayoutBed(0), noJSON: true},
	layoutCase{name: "session-isolate-refusal", args: []string{"session", "isolate", "--root", "ROOT"}, bed: passthroughLayoutBed(0), noJSON: true},
	layoutCase{name: "test-plan-refusal", args: []string{"test", "plan", "--root", "ROOT"}, bed: passthroughGitLayoutBed},
	layoutCase{name: "test-list", args: []string{"test", "list", "--root", "ROOT"}, bed: passthroughLayoutBed(0)},
	layoutCase{name: "test-add", args: []string{"test", "add", "--file", "CONTRACT", "--group", "app-group", "--inputs", "go.sum"}, bed: passthroughLayoutBed(0), noJSON: true},
	layoutCase{name: "test-add-refusal", args: []string{"test", "add", "--file", "CONTRACT"}, bed: passthroughLayoutBed(0), noJSON: true},
	layoutCase{name: "test-remove", args: []string{"test", "remove", "--file", "CONTRACT", "--group", "app-group", "--inputs", "go.mod"}, bed: passthroughLayoutBed(0), noJSON: true},
	layoutCase{name: "test-baseline-refusal", args: []string{"test", "baseline"}, bed: passthroughLayoutBed(0), noJSON: true},
	layoutCase{name: "test-status-result", args: []string{"test", "status", "--result", "RESULT", "--expensive-ms", "500"}, bed: passthroughLayoutBed(0), noJSON: true},
	layoutCase{name: "test-status-refusal", args: []string{"test", "status", "--root", "ROOT"}, bed: passthroughLayoutBed(0), noJSON: true},
)

// diskLayoutBed is a checkout with one idle stray and one entry that is not
// the engine's in its temporary root; with passed, one pass has run, so
// disk show has both reports to read.
func diskLayoutBed(passed bool) func(t *testing.T) layoutBed {
	return func(t *testing.T) layoutBed {
		bed := newDiskBed(t)
		stray := bed.stray("metasystem-audit.old", 72*time.Hour)
		bed.stray("tmp.foreign", 72*time.Hour)
		// The evidence root is the bed's own, never the test process's home.
		evidenceRoot, err := filepath.EvalSymlinks(t.TempDir())
		helmMust(t, err)
		helmMust(t, os.MkdirAll(evidenceRoot, 0o755),
			os.WriteFile(filepath.Join(bed.inst, "metasystem.conf.local"), []byte("evidence.root="+evidenceRoot+"\n"), 0o644))
		// A stray's file identity differs on every run and every machine.
		var identity syscall.Stat_t
		helmMust(t, syscall.Stat(stray, &identity))
		// No volume is measured: its free space differs on every run.
		bed.owners.disk.pass = func(ctx context.Context, top string, pass steward.DiskPass) (steward.DiskPassResult, error) {
			pass.Home, pass.TempRoots, pass.Volumes, pass.Checkouts = bed.home, []string{bed.tmp}, []string{}, []string{}
			pass.Proofs = map[diskstore.OwnerKind]diskstore.OwnerProof{diskstore.OwnerProcess: diskProof{}}
			pass.Clock = func() time.Time { return diskNow }
			pass.UserHome = filepath.Join(bed.root, "user")
			pass.Facts = func(context.Context, string) (diskstore.CheckoutFacts, error) {
				return diskstore.CheckoutFacts{}, os.ErrNotExist
			}
			return steward.SweepDiskStores(ctx, top, pass)
		}
		if passed {
			if code, out := bed.run("disk", "clean"); code != 0 {
				t.Fatalf("the bed's pass = %d:\n%s", code, out)
			}
		}
		return layoutBed{owners: bed.owners, cwd: bed.root, now: diskNow.Add(time.Hour),
			replace: layoutPaths(bed.root, bed.root, "/Users/wido/GitHub/agentic-tools-m1e",
				fmt.Sprintf(`"device": %d,`, identity.Dev), `"device": 16777233,`, fmt.Sprintf(`"inode": %d,`, identity.Ino), `"inode": 1622845050,`, evidenceRoot, "/Volumes/Evidence/metasystem-evidence")}
	}
}

// evidenceLayoutBed is the evidence verbs' bed: one old, closed chain of a
// concluded goal in this checkout's segment. EXPORTS in a case's words is
// its export directory.
func evidenceLayoutBed(t *testing.T) layoutBed {
	bed := newEvidenceVerbBed(t)
	// The segment is named by a digest of the checkout's temporary path.
	segment := diskstore.Segment(bed.gitRoot)
	return layoutBed{owners: bed.owners, cwd: bed.diskBed.root, now: diskNow.Add(time.Hour),
		replace: layoutPaths(bed.diskBed.root, bed.diskBed.root, "/Users/wido/GitHub/agentic-tools-m1e", bed.export, "/Volumes/Backup/metasystem-exports",
			segment, "0a5c41b756db"),
		words:   map[string]string{"EXPORTS": bed.export}}
}

// passthroughLayoutBed is an installation the passthrough actions name
// explicitly (their words carry ROOT and the files they read): a receipt
// ledger holding a retro and then receipts receipts, a recorded frontier,
// two investigation ledgers, a testing contract and a recorded test result.
func passthroughLayoutBed(receipts int) func(t *testing.T) layoutBed {
	return func(t *testing.T) layoutBed {
		root, err := filepath.EvalSymlinks(t.TempDir())
		helmMust(t, err)
		root = filepath.Join(root, "metasystem")
		helmMust(t, os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o755), os.MkdirAll(filepath.Join(root, "memory"), 0o755),
			os.MkdirAll(filepath.Join(root, "plans"), 0o755),
			os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o644))
		// The ledger: a retro on 28 September, then the receipts, a day apart.
		retro := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC).Unix()
		ledger := fmt.Sprintf("%d|2026-09-28T10:00:00Z|RETRO|summary=kept 2\n", retro)
		outcomes := []string{"shipped", "shipped", "reworked"}
		for index := range receipts {
			at := time.Unix(retro+int64(index+1)*3600, 0).UTC()
			ledger += fmt.Sprintf("%d|%s|RECEIPT|type=implement|outcome=%s|skills=verify|verify=caught|corrections=1|stop_loss=no|goal=verbs-match-intent|critique_waived=none|waiver_stream=none|note=unit %d\n",
				at.Unix(), at.Format(time.RFC3339), outcomes[index%len(outcomes)], index+1)
		}
		frontier := "sha=9498700a9d0c7b1e2f3a4b5c6d7e8f9a0b1c2d3e\nrecorded_epoch=1790589600\nscore=0.82\nmin_delta=0.01\ndirection=max\nmax_age_minutes=\n" +
			"eval=go test ./bench/...\nartifact=runs/1.json\n"
		investigation := "# Investigation\n\n- Cycle budget: 5\n\n### Cycle 1\n\n- Classification: `unresolved`\n\n### Cycle 2\n\n- Classification: `contract-improved`\n"
		stopped := "# Investigation\n\n### Cycle 1\n\n- Classification: `no-progress`\n\n### Cycle 2\n\n- Classification: `no-progress`\n"
		fixture := testingMergeFixture()
		fixture.Groups[0].Inputs = []string{"go.mod", "go.sum"}
		contract, err := contractmerge.Render(fixture)
		helmMust(t, err)
		files := map[string]string{"memory/receipts.log": ledger, "plans/frontier": frontier, "plans/investigation.md": investigation,
			"plans/stopped.md": stopped, "testing.json": string(contract)}
		for rel, data := range files {
			helmMust(t, os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(data), 0o644))
		}
		result := layoutTestResult()
		helmMust(t, writePrivateJSON(filepath.Join(root, "result.json"), result))
		return layoutBed{cwd: root, replace: layoutPaths(root, root, "/Users/wido/GitHub/agentic-tools-m1e/metasystem"),
			words: map[string]string{"ROOT": root, "LEDGER": filepath.Join(root, "memory", "receipts.log"), "FRONTIER": filepath.Join(root, "plans", "frontier"),
				"INVESTIGATION": filepath.Join(root, "plans", "investigation.md"), "STOPPED": filepath.Join(root, "plans", "stopped.md"),
				"CONTRACT": filepath.Join(root, "testing.json"), "RESULT": filepath.Join(root, "result.json")}}
	}
}

// layoutTestResult is a recorded test result of two groups, one of them
// over the expensive threshold.
func layoutTestResult() proofrun.TestResult {
	zero, admissionMaximum := 0, 0
	digest := strings.Repeat("a", 64)
	result := proofrun.TestResult{SchemaVersion: proofrun.TestResultSchemaVersion, AttemptID: "attempt-1",
		WorkerPolicyVersion: proofrun.TestWorkerPolicyVersion, Workers: 1, AdmissionMaximum: &admissionMaximum,
		Purpose: testpolicy.PurposeDiagnostic, RequestedMode: testpolicy.ModeAuto,
		RequiredMode: testpolicy.ModeAuto, ExecutedMode: testpolicy.ModeCanary,
		ProjectRoot: "/project", BaseCommit: strings.Repeat("b", 40), CandidateTree: strings.Repeat("c", 40),
		ContractDigest: digest, BaseContractDigest: digest, PolicyEngineDigest: digest,
		BehaviorPolicyDigest: digest, PlanDigest: digest, SelectedGroups: []string{"unit", "slow"},
		LaunchCounts: proofrun.LaunchCounts{Test: 2, CountsComplete: true},
		Cost:         proofrun.TestCost{DeclaredTargetMS: 2000, ActualDurationMS: 1700, ExecutionDurationMS: 1650},
		Groups: []proofrun.GroupResult{
			{ID: "unit", Kind: "unit", InputManifest: []string{"src/app.go"}, IdentityVersion: proofrun.GroupExecutionIdentityVersion,
				ExecutionIdentity: digest, Status: "passed", NativeLaunched: true, CollectionComplete: true, NativeExitStatus: &zero, DurationMS: 400},
			{ID: "slow", Kind: "unit", InputManifest: []string{"src/slow.go"}, IdentityVersion: proofrun.GroupExecutionIdentityVersion,
				ExecutionIdentity: digest, Status: "passed", NativeLaunched: true, CollectionComplete: true, NativeExitStatus: &zero, DurationMS: 1200}}}
	result.RecomputeDelivery()
	return result
}

// stopReportLayoutBed is an installation with one published Stop report:
// the Stop was allowed with supervision to repair, one seat action, a plan
// with open work touched two days ago and a stale plan untouched for twenty.
func stopReportLayoutBed(t *testing.T) layoutBed {
	bed := passthroughLayoutBed(0)(t)
	root := bed.words["ROOT"]
	for rel, age := range map[string]time.Duration{"plans/work.md": 48 * time.Hour, "plans/old.md": 20 * 24 * time.Hour} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		at := layoutNow.Add(-age)
		helmMust(t, os.WriteFile(path, []byte("# plan\n"), 0o644), os.Chtimes(path, at, at))
	}
	session, observed := "session-safe", "2026-09-30T08:50:00Z"
	control := report.StopControl{ShouldBlock: false, Class: "seat-actionable", JudgmentAvailable: true}
	judgment := &goal.TurnVerdictFacts{
		SchemaVersion: 1,
		Identity:      goal.TurnFactsIdentity{Installation: root, Session: session, MainId: "main-one", ObservedAt: observed},
		Verdict:       goal.Verdict{SchemaVersion: 1, Class: control.Class, LedgerStatus: "ok", Display: "bounded display"},
		Scan: goal.ScanResult{
			Open: []goal.Item{{Kind: "plan", Id: "plans/work.md", Detail: "OPEN-WORK plans/work.md: write the summary view",
				FullDetail: "OPEN-WORK plans/work.md: write the summary view", SourcePath: "plans/work.md", RequestedAction: "write the summary view"}},
			StalePlans: []goal.Item{{Kind: "plan", Id: "STALE-PLAN plans/old.md: claims work in flight while no job is running",
				Detail: "STALE-PLAN plans/old.md: claims work in flight while no job is running", FullDetail: "STALE-PLAN plans/old.md: claims work in flight while no job is running"}},
			TemplateUnfilled: []goal.Item{}, OpenWorkWarnings: []string{}, WaitingOnHuman: []goal.Item{}, Busy: []goal.Item{}, Questions: []goal.Item{},
			Drafts: []goal.Item{}, Unreadable: []string{}, Jobs: []goal.JobFact{}, Runs: []goal.RunFact{}, RunUnreadable: []string{},
		},
		Work: goal.TurnWorkFacts{ReadSucceeded: true, Claimed: []goal.GoalFacts{}, Landing: []goal.GoalFacts{},
			Claimable: []goal.GoalFacts{{Id: "verbs-match-intent", Intent: "verbs a person reads", NextStep: "convert the passthroughs", Revision: "4"}},
			Refused:   []goal.AdmissionRefusal{}, InFlight: []string{}, NonTerminalJobs: []string{}, Queued: 12},
		Ownership: goal.TurnOwnershipFacts{State: "none", Evidence: "no held claim"},
		Actions: []goal.TurnAction{{Kind: "claim-goal", TargetId: "verbs-match-intent", Instruction: "claim the next ready goal",
			Command: "metasystem goal claim verbs-match-intent", Owner: "seat"}},
		Refusal: goal.TurnRefusalFacts{Class: control.Class},
	}
	healthVerdict := steward.HealthVerdict{Schema: 1, ObservedAt: time.Date(2026, 9, 30, 8, 50, 0, 0, time.UTC), Aggregate: "unhealthy", Roles: []steward.RoleVerdict{
		{Role: steward.RoleStewardRunner, Status: steward.HealthDead, Reason: "no steward runner is recorded", Remedy: "metasystem system start"},
		{Role: steward.RoleSupervisionOwner, Status: steward.HealthAlive, Reason: "the owner holds the lock"}}}
	health := steward.NewHookHealthPreview(healthVerdict)
	digestText := "complete digest line\n"
	digestHash := sha256.Sum256([]byte(digestText))
	input := report.StopPresentationInput{
		SchemaVersion: report.StopPresentationSchemaVersion,
		Identity: report.StopIdentity{Installation: root, Runtime: "claude", Session: session, SessionKey: report.StopSessionKey("claude", session),
			Attempt: strings.Repeat("1", 32), MainId: "main-one", Machine: "m1e", Lineage: "lineage-1", ObservedAt: observed, ClaimEpoch: 7},
		Judgment: judgment, Control: control,
		CompletionObservation: &report.StopCompletionObservation{SchemaVersion: report.StopCompletionObservationSchemaVersion,
			Identity:    report.StopCompletionIdentity{Installation: root, Session: session, MainId: "main-one"},
			CollectedAt: observed, Records: []report.StopCompletionRecord{}, Unavailable: []string{}},
		Health:  &health,
		Digest:  &report.StopDigest{Mode: "pending", Text: digestText, SourcePath: "records/narrator-digest.log", CursorPrefix: strings.Repeat("a", 64), SHA256: fmt.Sprintf("%x", digestHash)},
		Receipt: &report.StopCommandResult{ExitCode: 0, Stdout: "receipt current\n"},
		Arming:  &report.StopArmingResult{ExitCode: 0, Stdout: "up healthy\n", Components: []report.StopArmingComponent{}, Aggregate: "up healthy"},
		Notices: []report.StopNotice{}, Unavailable: []report.StopUnavailable{},
	}
	work := t.TempDir()
	data, err := json.Marshal(input)
	helmMust(t, err, os.WriteFile(filepath.Join(work, "input.json"), append(data, '\n'), 0o644))
	if _, err := report.PresentStop(root, filepath.Join(work, "input.json"), filepath.Join(work, "output.json"), layoutNow.Add(-8*time.Minute)); err != nil {
		t.Fatal(err)
	}
	bed.replace = append(bed.replace, input.Identity.SessionKey, "608abd248ae1fd87d6ec15a54165223d7298d4b016cb4a999c1ed0e3715faa8e")
	return bed
}

// passthroughGitLayoutBed is the passthrough bed committed in a repository
// of its own, at a fixed author and date.
func passthroughGitLayoutBed(t *testing.T) layoutBed {
	bed := passthroughLayoutBed(0)(t)
	top := filepath.Dir(bed.words["ROOT"])
	git := func(args ...string) {
		command := exec.Command("git", append([]string{"-C", top}, args...)...)
		command.Env = append(gittree.ScrubbedEnviron(), "GIT_AUTHOR_NAME=bed", "GIT_AUTHOR_EMAIL=bed@example.invalid", "GIT_COMMITTER_NAME=bed",
			"GIT_COMMITTER_EMAIL=bed@example.invalid", "GIT_AUTHOR_DATE=2026-09-30T08:00:00Z", "GIT_COMMITTER_DATE=2026-09-30T08:00:00Z")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	git("init", "-q", "-b", "main")
	git("config", "metasystem.goal.machine", "m1e")
	git("add", "-A")
	git("commit", "-q", "-m", "bed")
	bed.replace = append(bed.replace, top, "/Users/wido/GitHub/agentic-tools-m1e")
	return bed
}

// TestTestStatusResultJSONIsMainsDefault: a recorded result's cost printed
// JSON by default before its text was laid out; --json prints exactly that.
func TestTestStatusResultJSONIsMainsDefault(t *testing.T) {
	t.Parallel()
	c := layoutCase{name: "test-status-result", args: []string{"test", "status", "--result", "RESULT", "--expensive-ms", "500", "--json"}, bed: passthroughLayoutBed(0)}
	code, stdout, stderr := runLayoutCase(t, c, c.bed(t))
	want, err := os.ReadFile(layoutGolden("test-status-result-main.json"))
	if err != nil || code != 0 || stdout != string(want) || stderr != "" {
		t.Fatalf("test status --result --json = %d:\n%s%s\nmain printed:\n%s (%v)", code, stdout, stderr, want, err)
	}
}
