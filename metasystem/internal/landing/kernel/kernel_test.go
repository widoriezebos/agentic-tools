package kernel

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

const kernelActor = "m1e+landing-m1l"

var kernelAt = time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)

// kernelBed is a registered lane with a nested checkout (checkout root ≠
// installation root), a bare origin, and a batch of one goal of two
// dependent builds and one change, composed by the agent as plain replays.
type kernelBed struct {
	t                     *testing.T
	home, dir             string
	layout                lane.Layout
	base, goalTip, change string
	batchID               string
	goalID, changeID      string
	head                  string
	ids                   int
}

func newKernelBed(t *testing.T) *kernelBed {
	t.Helper()
	bed := &kernelBed{t: t, dir: t.TempDir(), batchID: "01j5x00000000000000000kb01", goalID: "kernel-goal"}
	bed.home = filepath.Join(bed.dir, "home")
	checkout := filepath.Join(bed.dir, "lane")
	origin := filepath.Join(bed.dir, "origin.git")
	run(t, "", "git", "init", "-q", "--bare", "-b", "main", origin)
	run(t, "", "git", "init", "-q", "-b", "main", checkout)
	bed.write(checkout, ".gitignore", "/artifacts/\n/metasystem/artifacts/\n")
	bed.write(checkout, "metasystem/metasystem.conf", "metasystem.template=true\n")
	bed.write(checkout, "metasystem/app/base.txt", "base\n")
	machine, lineage, _ := strings.Cut(kernelActor, "+")
	ledger := goal.RenderFile(&goal.GoalFile{Id: bed.goalID, State: goal.StateClaimed, Intent: "Fixture goal.", Origin: goal.OriginMain,
		NextStep: "Land it.", OpenedAt: "2026-09-30T08:00:00Z", Revision: 1,
		Claimed: &goal.ClaimRecord{Machine: machine, Lineage: lineage, At: "2026-09-30T08:00:00Z", Revision: 1, AccountingRevision: 1},
		History: []goal.HistoryLine{{At: "2026-09-30T08:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAW-m1e-00000001", Verb: "claim", Actor: kernelActor,
			Targets: []string{bed.goalID}, Keep: -1}}})
	if _, problems := goal.ParseFile(ledger); len(problems) != 0 {
		t.Fatalf("invalid goal: %v", problems)
	}
	bed.write(checkout, "metasystem/plans/goals/"+bed.goalID+".md", string(ledger))
	bed.base = bed.commit(checkout, "base")
	bed.git(checkout, "remote", "add", "origin", origin)
	bed.git(checkout, "push", "-q", "origin", "main")
	bed.git(checkout, "fetch", "-q", "origin")
	layout, err := lane.NewLayout(checkout)
	if err != nil {
		t.Fatal(err)
	}
	if string(layout.Install) == string(layout.Checkout) {
		t.Fatalf("the bed's layout is flat; it must nest the installation")
	}
	bed.layout = layout
	if _, _, err := lane.Register(bed.home, layout, "Wido", kernelAt); err != nil {
		t.Fatal(err)
	}

	// A goal of two dependent builds (the second on a fold of the first).
	bed.git(checkout, "checkout", "-q", "-b", "goal/"+bed.goalID, bed.base)
	bed.write(checkout, "metasystem/app/feature.txt", "alpha\nbeta\ngamma\n")
	one := bed.commit(checkout, "build one")
	bed.write(checkout, "metasystem/app/feature.txt", "alpha\nBETA\ngamma\n")
	fold := bed.commit(checkout, "read fix")
	bed.write(checkout, "metasystem/app/feature.txt", "alpha\nBETA\nGAMMA\ndelta\n")
	two := bed.commit(checkout, "build two")
	bed.goalTip = two
	digestOne, err := goalbranch.UnitDigest(checkout, one)
	if err != nil {
		t.Fatal(err)
	}
	digestTwo, err := goalbranch.UnitDigest(checkout, two)
	if err != nil {
		t.Fatal(err)
	}
	bed.git(checkout, "checkout", "-q", "--detach", bed.base)
	bed.write(checkout, "metasystem/app/change.txt", "a seat's change\n")
	bed.change = bed.commit(checkout, "record: a seat's change\n\nMachine: m1e+human")
	bed.git(checkout, "checkout", "-q", "main")

	goalUnit := batch.BindBranchMember(batch.Unit{GoalID: bed.goalID, Chain: two, SeatRoot: "/seat", State: batch.UnitJoined, Approver: "Wido",
		AuthorName: "Wido Riezebos", AuthorEmail: "wido@example.invalid", SelectedGroups: []string{"feature-standard"},
		Claim: batch.Claim{Machine: "m1e", Lineage: "seat", Epoch: 1, Revision: 1, AccountingRevision: 1}},
		batch.BranchMember{GoalID: bed.goalID, Tip: two, Last: true, Builds: []batch.BranchBuild{
			{Units: []string{"u1"}, Commit: one, Digest: digestOne},
			{Units: []string{"u2"}, Commit: two, Digest: digestTwo, Folds: []goalbranch.Commit{{ID: fold, Kind: goalbranch.Read, Unit: "u1"}}}}})
	changeUnit := batch.NewChangeUnit(batch.ChangeMember{Commit: bed.change, Parent: bed.base, AskedBy: "m1e+human", Subject: "record: a seat's change"},
		"/seat", "m1e", "human", []string{"metasystem/app/change.txt"}, nil)
	changeUnit.State, changeUnit.SelectedGroups = batch.UnitJoined, []string{"change-standard"}
	bed.changeID = changeUnit.GoalID
	baseTree := bed.git(checkout, "rev-parse", bed.base+"^{tree}")
	if err := bed.store().Create(batch.Record{Schema: 1, BatchID: bed.batchID, BaseTree: baseTree, TipTree: baseTree, State: batch.StateOpen,
		Units: []batch.Unit{goalUnit, changeUnit}}); err != nil {
		t.Fatal(err)
	}
	bed.git(checkout, "checkout", "-q", "-B", "lane/"+bed.batchID, bed.base)
	bed.git(checkout, "cherry-pick", one)
	bed.git(checkout, "cherry-pick", fold)
	bed.git(checkout, "cherry-pick", two)
	bed.git(checkout, "reset", "-q", "--soft", "HEAD~2")
	bed.git(checkout, "commit", "-q", "-m", "build two with its fold")
	bed.git(checkout, "cherry-pick", bed.change)
	bed.head = bed.git(checkout, "rev-parse", "HEAD")
	bed.git(checkout, "checkout", "-q", "main")
	return bed
}

func run(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	command := exec.Command(name, args...)
	command.Dir = dir
	command.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2026-10-01T09:00:00Z", "GIT_COMMITTER_DATE=2026-10-01T09:00:00Z")
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func (bed *kernelBed) git(checkout string, args ...string) string {
	bed.t.Helper()
	return run(bed.t, "", "git", append([]string{"-C", checkout, "-c", "user.name=Lane Agent", "-c", "user.email=lane@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
}

func (bed *kernelBed) write(checkout, path, content string) {
	bed.t.Helper()
	full := filepath.Join(checkout, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		bed.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		bed.t.Fatal(err)
	}
}

func (bed *kernelBed) commit(checkout, message string) string {
	bed.t.Helper()
	bed.git(checkout, "add", "-A")
	bed.git(checkout, "commit", "-q", "-m", message)
	return bed.git(checkout, "rev-parse", "HEAD")
}

func (bed *kernelBed) store() batch.Store { return batch.NewStore(string(bed.layout.Checkout), nil) }

func (bed *kernelBed) tree(rev string) string {
	return bed.git(string(bed.layout.Checkout), "rev-parse", rev+"^{tree}")
}

func (bed *kernelBed) newID() (string, error) {
	bed.ids++
	return fmt.Sprintf("01j5x00000000000000000k%03d", bed.ids), nil
}

func (bed *kernelBed) begin() (BeginOutcome, error) {
	bed.t.Helper()
	return Begin(BeginRequest{Home: bed.home, Layout: bed.layout, BatchID: bed.batchID, Members: []string{bed.goalID, bed.changeID},
		Base: bed.base, Head: bed.head, Actor: kernelActor}, BeginSeams{Now: func() time.Time { return kernelAt }, NewID: bed.newID})
}

// fakeChild is the test run child as a program: it records its argv, writes
// result as its --result, and prints envelope; exit is its status.
func (bed *kernelBed) fakeChild(name string, result *proofrun.TestResult, envelope verbresult.Result, exit int) (executable, argvFile string) {
	bed.t.Helper()
	dir := filepath.Join(bed.dir, "children", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		bed.t.Fatal(err)
	}
	argvFile, resultFile, envelopeFile := filepath.Join(dir, "argv"), filepath.Join(dir, "result.json"), filepath.Join(dir, "envelope.json")
	envelope.SchemaVersion, envelope.Verb, envelope.Targets = verbresult.SchemaVersion, testRunVerb, []verbresult.Target{}
	data, err := json.Marshal(envelope)
	if err != nil {
		bed.t.Fatal(err)
	}
	bed.write(dir, "envelope.json", string(data)+"\n")
	copyResult := ":"
	if result != nil {
		data, err := json.Marshal(result)
		if err != nil {
			bed.t.Fatal(err)
		}
		bed.write(dir, "result.json", string(data)+"\n")
		copyResult = `cp '` + resultFile + `' "$out"`
	}
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > '" + argvFile + "'\nout=\nprev=\nfor arg in \"$@\"; do\n  if [ \"$prev\" = --result ]; then out=\"$arg\"; fi\n  prev=\"$arg\"\ndone\n" +
		copyResult + "\ncat '" + envelopeFile + "'\nexit " + fmt.Sprint(exit) + "\n"
	executable = filepath.Join(dir, "metasystem")
	if err := testexec.WriteFile(executable, []byte(script), 0o755); err != nil {
		bed.t.Fatal(err)
	}
	return executable, argvFile
}

func (bed *kernelBed) prove(subject, executable string) (batch.ProofAttempt, error) {
	bed.t.Helper()
	return Prove(ProveRequest{Home: bed.home, Layout: bed.layout, BatchID: bed.batchID, Subject: subject, Actor: kernelActor},
		ProveSeams{Executable: func() (string, error) { return executable, nil }, Prober: identity.KernelProber{},
			Now: func() time.Time { return kernelAt }, NewID: bed.newID})
}

func argValue(argv []string, flag string) string {
	for index := 0; index+1 < len(argv); index++ {
		if argv[index] == flag {
			return argv[index+1]
		}
	}
	return ""
}

func readArgv(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the test run child did not run: %v", err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

func passed(groups ...string) *proofrun.TestResult {
	result := &proofrun.TestResult{AttemptID: "run-1"}
	for _, group := range groups {
		result.Groups = append(result.Groups, proofrun.GroupResult{ID: group, Status: "passed"})
	}
	result.Delivery.Sufficient = true
	return result
}

// K4, K6: begin records the canonical series before anything runs; prove
// runs each subject's exact tree as a lane-charged run that names the
// lane's checkout and control root, and records every attempt with its
// subject: the candidate as a delivery run, B, and B plus exactly the
// member's two dependent builds and their fold.
func TestProveRunsEachSubjectOnItsTree(t *testing.T) {
	t.Parallel()
	bed := newKernelBed(t)
	outcome, err := bed.begin()
	if err != nil || !outcome.Changed {
		t.Fatalf("begin: %+v %v", outcome, err)
	}
	if ref := bed.git(string(bed.layout.Checkout), "rev-parse", CandidateRef(bed.batchID)); ref != outcome.Opening.Candidate {
		t.Fatalf("the candidate is kept at %s, not %s", ref, outcome.Opening.Candidate)
	}
	account := lane.AccountID(string(bed.layout.Checkout))
	cases := []struct {
		subject, tree, purpose, groups string
	}{
		{batch.SubjectBatch, outcome.Opening.Tree, "delivery", ""},
		{batch.SubjectBase, bed.tree(bed.base), "diagnostic", "change-standard,feature-standard"},
		{batch.SubjectMember + ":" + bed.goalID, bed.tree(bed.goalTip), "diagnostic", "feature-standard"},
		{batch.SubjectMember + ":" + bed.changeID, bed.tree(bed.change), "diagnostic", "change-standard"},
	}
	for index, want := range cases {
		executable, argvFile := bed.fakeChild(fmt.Sprint(index), passed("feature-standard"), verbresult.Result{Outcome: verbresult.Confirmed, Summary: "passed"}, 0)
		attempt, err := bed.prove(want.subject, executable)
		if err != nil {
			t.Fatalf("prove %s: %v", want.subject, err)
		}
		argv := readArgv(t, argvFile)
		if argValue(argv, "--tree") != want.tree || argValue(argv, "--purpose") != want.purpose || argValue(argv, "--groups") != want.groups ||
			argValue(argv, "--lane") != account || argValue(argv, "--lane-checkout") != string(bed.layout.Checkout) ||
			argValue(argv, "--control-root") != string(bed.layout.Install) || !slices.Contains(argv, "--hold-host-proving") {
			t.Fatalf("prove %s ran %q; want tree %s, purpose %s, groups %q on the lane's named checkout and control root", want.subject, argv, want.tree, want.purpose, want.groups)
		}
		if root := argValue(argv, "--root"); filepath.Base(root) != "metasystem" || strings.HasPrefix(root, string(bed.layout.Checkout)+string(filepath.Separator)) {
			t.Fatalf("prove %s ran in %s; want the installation of its own projection of the tree", want.subject, root)
		}
		if attempt.Status != batch.AttemptGreen || attempt.Tree != want.tree || attempt.OpID != outcome.Opening.OpID || attempt.Child == nil {
			t.Fatalf("prove %s recorded %+v; want green on %s with its child's identity", want.subject, attempt, want.tree)
		}
	}
	record, err := bed.store().Load(bed.batchID)
	if err != nil || len(record.Attempts) != len(cases) {
		t.Fatalf("recorded attempts = %d, %v; want %d", len(record.Attempts), err, len(cases))
	}
}

// K2: the pause holds. Begin and prove read it under the host flock right
// before they act: while the lane is stopped nothing is recorded and no
// child starts.
func TestPauseHoldsBeginAndProve(t *testing.T) {
	t.Parallel()
	bed := newKernelBed(t)
	if _, err := lane.SetPause(bed.home, "Wido", kernelAt); err != nil {
		t.Fatal(err)
	}
	var paused *lane.Refusal
	if _, err := bed.begin(); !errors.As(err, &paused) || paused.Code != lane.CodePaused {
		t.Fatalf("begin while paused = %v; want the pause's refusal", err)
	}
	if record, err := bed.store().Load(bed.batchID); err != nil || len(record.Openings) != 0 || record.State != batch.StateOpen {
		t.Fatalf("a paused begin recorded %+v, %v", record.Openings, err)
	}
	if _, err := lane.ClearPause(bed.home); err != nil {
		t.Fatal(err)
	}
	if _, err := bed.begin(); err != nil {
		t.Fatal(err)
	}
	if _, err := lane.SetPause(bed.home, "Wido", kernelAt); err != nil {
		t.Fatal(err)
	}
	executable, argvFile := bed.fakeChild("paused", passed("feature-standard"), verbresult.Result{Outcome: verbresult.Confirmed}, 0)
	if _, err := bed.prove(batch.SubjectBatch, executable); !errors.As(err, &paused) || paused.Code != lane.CodePaused {
		t.Fatalf("prove while paused = %v; want the pause's refusal", err)
	}
	if _, err := os.Stat(argvFile); !os.IsNotExist(err) {
		t.Fatalf("a paused prove started its child")
	}
	if record, err := bed.store().Load(bed.batchID); err != nil || len(record.Attempts) != 0 {
		t.Fatalf("a paused prove recorded %+v, %v", record.Attempts, err)
	}
}

// K6, design §8: an attempt is typed. A failed test is red; a refused or
// unfinished run, or one without a readable result, is unavailable, never
// red.
func TestProveTypesItsOutcome(t *testing.T) {
	t.Parallel()
	bed := newKernelBed(t)
	if _, err := bed.begin(); err != nil {
		t.Fatal(err)
	}
	failed := passed("feature-standard")
	failed.Groups = append(failed.Groups, proofrun.GroupResult{ID: "change-standard", Status: "failed"})
	failed.Delivery.Sufficient = false
	cases := []struct {
		name   string
		result *proofrun.TestResult
		reply  verbresult.Result
		exit   int
		want   string
	}{
		{"red", failed, verbresult.Result{Outcome: verbresult.Failed, Summary: "tests failed"}, 1, batch.AttemptRed},
		{"refused", nil, verbresult.Result{Outcome: verbresult.Refused, Code: "LANE_ACCOUNT_UNRESOLVED", Summary: "the lane cannot be named"}, 2, batch.AttemptUnavailable},
		{"no-result", nil, verbresult.Result{Outcome: verbresult.Failed, Summary: "the engine is not enrolled"}, 1, batch.AttemptUnavailable},
		{"cancelled", &proofrun.TestResult{AttemptID: "run-2", Groups: []proofrun.GroupResult{{ID: "feature-standard", Status: "cancelled"}}},
			verbresult.Result{Outcome: verbresult.Failed, Summary: "cancelled"}, 1, batch.AttemptUnavailable},
	}
	for _, want := range cases {
		executable, _ := bed.fakeChild(want.name, want.result, want.reply, want.exit)
		attempt, err := bed.prove(batch.SubjectMember+":"+bed.changeID, executable)
		if err != nil || attempt.Status != want.want {
			t.Fatalf("%s child: attempt %+v, %v; want %s", want.name, attempt, err, want.want)
		}
		if want.want == batch.AttemptRed && !slices.Equal(attempt.RedGroups, []string{"change-standard"}) {
			t.Fatalf("red attempt names %v; want change-standard", attempt.RedGroups)
		}
	}
}

// K6: prove needs the batch's recorded series, and a subject of it.
func TestProveRefusesWhatItCannotRun(t *testing.T) {
	t.Parallel()
	bed := newKernelBed(t)
	executable, argvFile := bed.fakeChild("none", passed("x"), verbresult.Result{Outcome: verbresult.Confirmed}, 0)
	var refusal *Refusal
	if _, err := bed.prove(batch.SubjectBatch, executable); !errors.As(err, &refusal) || !strings.Contains(refusal.Reason, "no series") {
		t.Fatalf("prove before begin = %v; want a refusal", err)
	}
	if _, err := bed.begin(); err != nil {
		t.Fatal(err)
	}
	for _, subject := range []string{"everything", batch.SubjectMember + ":", batch.SubjectMember + ":stranger"} {
		if _, err := bed.prove(subject, executable); !errors.As(err, &refusal) {
			t.Fatalf("prove %q = %v; want a refusal", subject, err)
		}
	}
	if _, err := os.Stat(argvFile); !os.IsNotExist(err) {
		t.Fatalf("a refused prove started its child")
	}
}
