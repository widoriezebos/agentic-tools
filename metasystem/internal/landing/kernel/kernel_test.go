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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

const kernelActor = "m1e+landing-lane"

var kernelAt = time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)

// kernelBed is a registered lane with a nested checkout (checkout root ≠
// installation root) and a bare origin, and two hand-offs queued in it:
// two batch records of one change each, whose heads branch off main.
type kernelBed struct {
	t                 *testing.T
	home, dir         string
	origin            string
	layout            lane.Layout
	base              string
	first, second     string
	firstID, secondID string
	ids               int
}

var (
	firstBatch  = "01j5x00000000000000000kb01"
	secondBatch = "01j5x00000000000000000kb02"
)

func newKernelBed(t *testing.T) *kernelBed {
	t.Helper()
	bed := &kernelBed{t: t, dir: t.TempDir()}
	bed.home = filepath.Join(bed.dir, "home")
	checkout := filepath.Join(bed.dir, "lane")
	bed.origin = filepath.Join(bed.dir, "origin.git")
	run(t, "", "git", "init", "-q", "--bare", "-b", "main", bed.origin)
	run(t, "", "git", "init", "-q", "-b", "main", checkout)
	bed.write(checkout, ".gitignore", "/artifacts/\n/metasystem/artifacts/\n")
	bed.write(checkout, "metasystem/metasystem.conf", "metasystem.template=true\n")
	bed.write(checkout, "metasystem/app/base.txt", "base\n")
	bed.base = bed.commit(checkout, "base")
	bed.git(checkout, "remote", "add", "origin", bed.origin)
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
	bed.first = bed.change(checkout, "one.txt")
	bed.second = bed.change(checkout, "two.txt")
	bed.firstID = bed.queue(firstBatch, bed.first, "metasystem/app/one.txt")
	bed.secondID = bed.queue(secondBatch, bed.second, "metasystem/app/two.txt")
	return bed
}

// change is a seat's change on main's base, off to the side.
func (bed *kernelBed) change(checkout, name string) string {
	bed.git(checkout, "checkout", "-q", "--detach", bed.base)
	bed.write(checkout, "metasystem/app/"+name, name+"\n")
	commit := bed.commit(checkout, "record: "+name)
	bed.git(checkout, "checkout", "-q", "main")
	return commit
}

// queue is one hand-off: a batch record holding the change as its joined
// member.
func (bed *kernelBed) queue(id, commit, path string) string {
	unit := batch.NewChangeUnit(batch.ChangeMember{Commit: commit, Parent: bed.base, AskedBy: "m1e+human", Subject: "record"},
		"/seat", "m1e", "human", []string{path}, nil)
	unit.State = batch.UnitJoined
	tree := bed.tree(bed.base)
	if err := bed.store().Create(batch.Record{Schema: 1, BatchID: id, BaseTree: tree, TipTree: tree, State: batch.StateOpen, Units: []batch.Unit{unit}}); err != nil {
		bed.t.Fatal(err)
	}
	return unit.GoalID
}

// merge is the agent's step 2: main with the named heads merged in.
func (bed *kernelBed) merge(heads ...string) string {
	checkout := string(bed.layout.Checkout)
	for _, head := range heads {
		bed.git(checkout, "merge", "-q", "--no-ff", "--no-edit", head)
	}
	return bed.git(checkout, "rev-parse", "HEAD")
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

func (bed *kernelBed) originMain() string {
	return run(bed.t, "", "git", "-C", bed.origin, "rev-parse", "refs/heads/main")
}

func (bed *kernelBed) newID() (string, error) {
	bed.ids++
	return fmt.Sprintf("01j5x00000000000000000k%03d", bed.ids), nil
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

func (bed *kernelBed) prove(tree, executable string) (TreeProof, error) {
	bed.t.Helper()
	return Prove(ProveRequest{Home: bed.home, Layout: bed.layout, Tree: tree, Actor: kernelActor},
		ProveSeams{Executable: func() (string, error) { return executable, nil }, Now: func() time.Time { return kernelAt }, NewID: bed.newID})
}

func (bed *kernelBed) push() (PushOutcome, error) {
	bed.t.Helper()
	return Push(PushRequest{Home: bed.home, Layout: bed.layout, Actor: kernelActor}, ProductionPushSeams())
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

func failed(group string) *proofrun.TestResult {
	return &proofrun.TestResult{AttemptID: "run-2", Groups: []proofrun.GroupResult{{ID: group, Status: "failed"}}}
}

// landing prove proves the lane checkout's HEAD tree as one lane-charged
// delivery run (the same selected tests a seat's own landing runs), starts
// its child itself, and records the attempt on every queued hand-off's
// batch record and as the tree's result that landing push reads.
func TestProveRecordsTheHeadTreeOnEveryQueuedBatch(t *testing.T) {
	t.Parallel()
	bed := newKernelBed(t)
	head := bed.merge(bed.first, bed.second)
	executable, argvFile := bed.fakeChild("green", passed("app-standard"), verbresult.Result{Outcome: verbresult.Confirmed, Summary: "passed"}, 0)
	proof, err := bed.prove("", executable)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}
	tree := bed.tree(head)
	argv := readArgv(t, argvFile)
	if argValue(argv, "--tree") != tree || argValue(argv, "--purpose") != "delivery" || !slices.Contains(argv, "--batch-tip") ||
		argValue(argv, "--mode") != "auto" || argValue(argv, "--lane") != lane.AccountID(string(bed.layout.Checkout)) ||
		argValue(argv, "--lane-checkout") != string(bed.layout.Checkout) || argValue(argv, "--control-root") != string(bed.layout.Install) {
		t.Fatalf("prove ran %q; want a lane-charged delivery run of tree %s", argv, tree)
	}
	if proof.Tree != tree || proof.Commit != head || proof.Status != batch.AttemptGreen || !slices.Equal(proof.Batches, []string{firstBatch, secondBatch}) {
		t.Fatalf("prove = %+v; want green on HEAD's tree %s for both hand-offs", proof, tree)
	}
	kept, ok, err := ReadTreeProof(bed.layout, tree)
	if err != nil || !ok || kept.Status != batch.AttemptGreen || kept.Attempt != proof.Attempt {
		t.Fatalf("the tree's kept result = %+v %v %v; want the green attempt", kept, ok, err)
	}
	for id, member := range map[string]string{firstBatch: bed.firstID, secondBatch: bed.secondID} {
		record, err := bed.store().Load(id)
		if err != nil || len(record.Attempts) != 1 {
			t.Fatalf("batch %s attempts = %+v %v; want the one", id, record.Attempts, err)
		}
		attempt := record.Attempts[0]
		if attempt.ID != proof.Attempt || attempt.Tree != tree || attempt.Status != batch.AttemptGreen || !slices.Equal(attempt.Covers, []string{member}) {
			t.Fatalf("batch %s recorded %+v; want the green attempt covering %s", id, attempt, member)
		}
	}

	// A red names its failing tests, and --tree proves another tree.
	executable, argvFile = bed.fakeChild("red", failed("app-standard"), verbresult.Result{Outcome: verbresult.Failed, Summary: "tests failed"}, 1)
	red, err := bed.prove(bed.first, executable)
	if err != nil || red.Status != batch.AttemptRed || red.Tree != bed.tree(bed.first) || !slices.Equal(red.RedGroups, []string{"app-standard"}) {
		t.Fatalf("prove --tree of a red tree = %+v %v; want red naming app-standard", red, err)
	}
	if argValue(readArgv(t, argvFile), "--tree") != bed.tree(bed.first) {
		t.Fatalf("prove --tree ran another tree")
	}
}

// A stopped lane starts nothing: no child, no attempt, no kept result.
func TestPauseHoldsProve(t *testing.T) {
	t.Parallel()
	bed := newKernelBed(t)
	bed.merge(bed.first)
	if _, err := lane.SetPause(bed.home, "Wido", kernelAt); err != nil {
		t.Fatal(err)
	}
	executable, argvFile := bed.fakeChild("paused", passed("app-standard"), verbresult.Result{Outcome: verbresult.Confirmed}, 0)
	var paused *lane.Refusal
	if _, err := bed.prove("", executable); !errors.As(err, &paused) || paused.Code != lane.CodePaused {
		t.Fatalf("prove while paused = %v; want the pause's refusal", err)
	}
	if _, err := os.Stat(argvFile); !os.IsNotExist(err) {
		t.Fatalf("a paused prove started its child")
	}
	if record, err := bed.store().Load(firstBatch); err != nil || len(record.Attempts) != 0 {
		t.Fatalf("a paused prove recorded %+v, %v", record.Attempts, err)
	}
}

// An attempt is typed: a refused run, or one without a readable result, is
// unavailable, never red.
func TestProveTypesItsOutcome(t *testing.T) {
	t.Parallel()
	bed := newKernelBed(t)
	bed.merge(bed.first)
	cases := []struct {
		name   string
		result *proofrun.TestResult
		reply  verbresult.Result
		exit   int
		want   string
	}{
		{"refused", nil, verbresult.Result{Outcome: verbresult.Refused, Code: "LANE_ACCOUNT_UNRESOLVED", Summary: "the lane cannot be named"}, 2, batch.AttemptUnavailable},
		{"no-result", nil, verbresult.Result{Outcome: verbresult.Failed, Summary: "it broke"}, 1, batch.AttemptUnavailable},
		{"cancelled", &proofrun.TestResult{AttemptID: "run-2", Groups: []proofrun.GroupResult{{ID: "app-standard", Status: "cancelled"}}},
			verbresult.Result{Outcome: verbresult.Failed, Summary: "cancelled"}, 1, batch.AttemptUnavailable},
	}
	for _, want := range cases {
		executable, _ := bed.fakeChild(want.name, want.result, want.reply, want.exit)
		proof, err := bed.prove("", executable)
		if err != nil || proof.Status != want.want {
			t.Fatalf("%s child: %+v, %v; want %s", want.name, proof, err, want.want)
		}
	}
}

// Rail 1: landing push puts only a tree landing prove recorded green on
// main, and only as a fast-forward of main. It then marks every queued
// member whose head the pushed commit contains as landed; the others stay
// queued.
func TestPushPushesOnlyAProvenFastForward(t *testing.T) {
	t.Parallel()
	bed := newKernelBed(t)
	bed.merge(bed.first)
	var refusal *Refusal
	if _, err := bed.push(); !errors.As(err, &refusal) || refusal.Code != CodePushUnproven {
		t.Fatalf("push of an unproven HEAD = %v; want %s", err, CodePushUnproven)
	}
	executable, _ := bed.fakeChild("red", failed("app-standard"), verbresult.Result{Outcome: verbresult.Failed, Summary: "tests failed"}, 1)
	if _, err := bed.prove("", executable); err != nil {
		t.Fatal(err)
	}
	if _, err := bed.push(); !errors.As(err, &refusal) || refusal.Code != CodePushUnproven {
		t.Fatalf("push of a red HEAD = %v; want %s", err, CodePushUnproven)
	}
	if bed.originMain() != bed.base {
		t.Fatalf("a refused push moved main")
	}

	// Main moves past the proven HEAD: a green proof does not make a
	// rewrite of main a push.
	other := filepath.Join(bed.dir, "other")
	run(t, "", "git", "clone", "-q", bed.origin, other)
	bed.write(other, "metasystem/app/other.txt", "other\n")
	moved := bed.commit(other, "someone else's landing")
	bed.git(other, "push", "-q", "origin", "main")
	executable, _ = bed.fakeChild("green", passed("app-standard"), verbresult.Result{Outcome: verbresult.Confirmed, Summary: "passed"}, 0)
	if _, err := bed.prove("", executable); err != nil {
		t.Fatal(err)
	}
	if _, err := bed.push(); !errors.As(err, &refusal) || refusal.Code != CodePushNotFastForward {
		t.Fatalf("push over a moved main = %v; want %s", err, CodePushNotFastForward)
	}
	if bed.originMain() != moved {
		t.Fatalf("a refused push rewrote main")
	}

	// The agent merges main again and proves the new HEAD: it pushes.
	checkout := string(bed.layout.Checkout)
	bed.git(checkout, "fetch", "-q", "origin")
	head := bed.merge("origin/main")
	if _, err := bed.push(); !errors.As(err, &refusal) || refusal.Code != CodePushUnproven {
		t.Fatalf("push of the re-merged, unproven HEAD = %v; want %s", err, CodePushUnproven)
	}
	executable, _ = bed.fakeChild("green-again", passed("app-standard"), verbresult.Result{Outcome: verbresult.Confirmed, Summary: "passed"}, 0)
	if _, err := bed.prove("", executable); err != nil {
		t.Fatal(err)
	}
	outcome, err := bed.push()
	if err != nil || !outcome.Changed || outcome.Commit != head || outcome.Old != moved {
		t.Fatalf("push of a proven fast-forward = %+v %v; want main moved from %s to %s", outcome, err, moved, head)
	}
	if bed.originMain() != head {
		t.Fatalf("main is %s; want the pushed %s", bed.originMain(), head)
	}
	if !slices.Equal(outcome.Landed, []string{bed.firstID}) {
		t.Fatalf("landed = %q; want only the contained %s", outcome.Landed, bed.firstID)
	}
	first, _ := bed.store().Load(firstBatch)
	if first.Units[0].State != batch.UnitLanded || first.State != batch.StateLanded {
		t.Fatalf("the contained hand-off is %s/%s; want landed", first.State, first.Units[0].State)
	}
	second, _ := bed.store().Load(secondBatch)
	if second.Units[0].State != batch.UnitJoined {
		t.Fatalf("the hand-off main does not contain is %s; want it still queued", second.Units[0].State)
	}
	published, err := lane.ReadPublications(bed.home)
	if err != nil || len(published) != 1 || published[0].New != head || published[0].Old != moved {
		t.Fatalf("publication record = %+v %v; want the one push", published, err)
	}

	// The same push again changes nothing.
	if again, err := bed.push(); err != nil || again.Changed {
		t.Fatalf("a repeated push = %+v %v; want it unchanged", again, err)
	}

	// A stopped lane pushes nothing.
	if _, err := lane.SetPause(bed.home, "Wido", kernelAt); err != nil {
		t.Fatal(err)
	}
	head = bed.merge(bed.second)
	executable, _ = bed.fakeChild("green-paused", passed("app-standard"), verbresult.Result{Outcome: verbresult.Confirmed, Summary: "passed"}, 0)
	if _, err := lane.ClearPause(bed.home); err != nil {
		t.Fatal(err)
	}
	if _, err := bed.prove("", executable); err != nil {
		t.Fatal(err)
	}
	if _, err := lane.SetPause(bed.home, "Wido", kernelAt); err != nil {
		t.Fatal(err)
	}
	var paused *lane.Refusal
	if _, err := bed.push(); !errors.As(err, &paused) || paused.Code != lane.CodePaused {
		t.Fatalf("push while paused = %v; want the pause's refusal", err)
	}
	if bed.originMain() == head {
		t.Fatalf("a paused lane pushed")
	}
}
