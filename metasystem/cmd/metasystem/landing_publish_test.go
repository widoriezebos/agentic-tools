package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/laneengine"
)

// publishEvidence is the fixture of K-b's begin and proof records: what
// landing publish reads of a batch.
type publishEvidence struct {
	begin    lane.BatchBegin
	proof    lane.ProofAttempt
	verified error
}

func (e *publishEvidence) Begin(batch string) (lane.BatchBegin, error) {
	if batch != e.begin.Batch {
		return lane.BatchBegin{}, errors.New("no begin record for " + batch)
	}
	return e.begin, nil
}

func (e *publishEvidence) Proof(batch string) (lane.ProofAttempt, error) {
	if batch != e.proof.Batch {
		return lane.ProofAttempt{}, errors.New("no proof for " + batch)
	}
	return e.proof, nil
}

func (e *publishEvidence) Verify(lane.ProofAttempt) error { return e.verified }

// publishBed is a real nested lane (kernelBed: the Git toplevel is not the
// installation) whose enrolled engine is this test binary, registered by
// landing set with its real pre-push hook, with a bare file:// origin, a
// seat clone, a goal claimed on main, and a composed series B..C: a
// member's commit and the lane's listed integration commit.
type publishBed struct {
	*kernelBed
	origin, seat  string
	base, head    string
	evidence      *publishEvidence
	enrolledPlain string
}

const publishBatch = "4gr18nm8t3nyev9sssda9jgtsq"

func (bed *publishBed) git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=lane", "-c", "user.email=lane@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func newPublishBed(t *testing.T) *publishBed {
	t.Helper()
	bed := &publishBed{kernelBed: newKernelBed(t)}
	bed.enroll(t, runningTestBinary(t))
	claimed := goal.RenderFile(&goal.GoalFile{Id: "ship-widget", State: goal.StateClaimed, Intent: "Fixture ownership.", Origin: goal.OriginMain,
		NextStep: "Land the widget.", OpenedAt: "2026-09-30T08:00:00Z", Revision: 1,
		Claimed: &goal.ClaimRecord{Machine: "m9", Lineage: "L1", At: "2026-09-30T08:00:00Z", Revision: 1, AccountingRevision: 1},
		History: []goal.HistoryLine{{At: "2026-09-30T08:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAW-m9-00000001", Verb: "claim", Actor: "m9+L1", Targets: []string{"ship-widget"}, Keep: -1}}})
	if err := os.MkdirAll(filepath.Join(bed.installation, "plans", "goals"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bed.installation, "plans", "goals", "ship-widget.md"), claimed, 0o644); err != nil {
		t.Fatal(err)
	}
	bed.base = bed.landMain(t)
	bed.origin = filepath.Join(filepath.Dir(bed.checkout), "origin.git")
	bed.seat = filepath.Join(filepath.Dir(bed.checkout), "seat")
	bed.git(t, filepath.Dir(bed.checkout), "clone", "--quiet", "file://"+bed.origin, bed.seat)
	bed.git(t, bed.checkout, "checkout", "--quiet", "-b", "lane/"+publishBatch)
	write := func(name string) {
		if err := os.WriteFile(filepath.Join(bed.checkout, name), []byte(name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		bed.git(t, bed.checkout, "add", name)
	}
	write("widget.txt")
	bed.git(t, bed.checkout, "commit", "--quiet", "-m", "ship the widget", "--trailer", "Machine: m9+L1", "--trailer", "Goal-Item: ship-widget", "--trailer", "Goal-Revision: 1")
	write("seam.txt")
	bed.git(t, bed.checkout, "commit", "--quiet", "-m", "seam between members", "--trailer", "Machine: landing+landing-lane", "--trailer", "Lane-Integration: "+publishBatch)
	bed.head = bed.git(t, bed.checkout, "rev-parse", "HEAD")
	tree := bed.git(t, bed.checkout, "rev-parse", "HEAD^{tree}")
	enrolled, err := laneengine.Enrollment(bed.installation)
	if err != nil {
		t.Fatal(err)
	}
	bed.evidence = &publishEvidence{
		begin: lane.BatchBegin{Batch: publishBatch, OpID: "op-1", Members: []string{"ship-widget"}, Base: bed.base, Head: bed.head, Tree: tree, LaneCommits: []string{bed.head}},
		proof: lane.ProofAttempt{Batch: publishBatch, Attempt: "2", Subject: lane.SubjectBatch, Outcome: lane.OutcomeGreen, Base: bed.base, Commit: bed.head, Tree: tree,
			PolicyEngineDigest: enrolled.InstallDigest, CandidateEngineDigest: "sha256:" + strings.Repeat("c", 64)},
	}
	return bed
}

func (bed *publishBed) owners() intentOwners {
	owners := bed.kernelBed.owners()
	owners.landing.evidence = bed.evidence
	return owners
}

func (bed *publishBed) publish(t *testing.T, extra ...string) (int, string, string) {
	t.Helper()
	command, ok := findIntentAction("landing", "publish")
	if !ok {
		t.Fatal("no public command landing publish")
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, append([]string{"--batch", publishBatch}, extra...), &stdout, &stderr, bed.cwd, bed.owners())
	return code, stdout.String(), stderr.String()
}

func (bed *publishBed) main(t *testing.T) string {
	t.Helper()
	return bed.git(t, bed.origin, "rev-parse", "refs/heads/main")
}

// The design's K-c witness for the verb: landing publish puts on main
// exactly the series landing begin recorded, commit by commit with the
// same trees and a Landing-Proof trailer naming the batch and attempt,
// through the lane checkout's real hook; a repeat changes nothing.
func TestLandingPublishPutsTheProvenSeriesOnMain(t *testing.T) {
	t.Parallel()
	bed := newPublishBed(t)
	code, stdout, stderr := bed.publish(t, "--json")
	var result intentResult
	if err := json.Unmarshal([]byte(stdout+stderr), &result); code != 0 || err != nil || result.Outcome != intentConfirmed {
		t.Fatalf("landing publish = %d %+v %v\n%s%s", code, result, err, stdout, stderr)
	}
	published := bed.main(t)
	if published == bed.head || published == bed.base {
		t.Fatalf("main is %s; want the trailed series, not the begin head %s or the base", published, bed.head)
	}
	original := strings.Fields(bed.git(t, bed.checkout, "rev-list", "--reverse", bed.base+".."+bed.head))
	landed := strings.Fields(bed.git(t, bed.origin, "rev-list", "--reverse", bed.base+".."+published))
	if len(landed) != len(original) {
		t.Fatalf("main gained %d commits; the series has %d", len(landed), len(original))
	}
	for index := range original {
		if want, got := bed.git(t, bed.checkout, "rev-parse", original[index]+"^{tree}"), bed.git(t, bed.origin, "rev-parse", landed[index]+"^{tree}"); want != got {
			t.Fatalf("commit %d landed tree %s; the proven series has %s", index, got, want)
		}
		if proof := bed.git(t, bed.origin, "log", "-1", "--format=%(trailers:key=Landing-Proof,valueonly)", landed[index]); proof != lane.ProofValue(publishBatch, "2") {
			t.Fatalf("commit %d carries Landing-Proof %q", index, proof)
		}
	}
	if code, stdout, stderr := bed.publish(t); code != 0 || !strings.Contains(stdout, "already on main") || bed.main(t) != published {
		t.Fatalf("a repeated publish = %d %q %q, main %s", code, stdout, stderr, bed.main(t))
	}
}

// landing publish publishes nothing that was not proven as it stands: a
// proof of another tree, a red or member-only proof, retained evidence
// that no longer verifies, or a proof judged by an engine the lane did
// not enroll; each is refused with its fix and main does not move.
func TestLandingPublishRefusesUnprovenWork(t *testing.T) {
	t.Parallel()
	bed := newPublishBed(t)
	good := *bed.evidence
	for _, refusal := range []struct {
		name  string
		bend  func(*publishEvidence)
		words string
	}{
		{"another tree", func(e *publishEvidence) { e.proof.Tree = strings.Repeat("e", 40) }, "not a green proof"},
		{"a red proof", func(e *publishEvidence) { e.proof.Outcome = "red" }, "not a green proof"},
		{"a member proof", func(e *publishEvidence) { e.proof.Subject = "member:ship-widget" }, "not a green proof"},
		{"stale retained evidence", func(e *publishEvidence) { e.verified = errors.New("retained result digest moved") }, "no longer verifies"},
		{"another policy engine", func(e *publishEvidence) { e.proof.PolicyEngineDigest = "sha256:" + strings.Repeat("d", 64) }, "judged by an engine that isn't the landing lane's enrolled one"},
	} {
		*bed.evidence = good
		refusal.bend(bed.evidence)
		code, stdout, stderr := bed.publish(t)
		if code == 0 || !strings.Contains(stdout+stderr, refusal.words) {
			t.Errorf("%s: landing publish = %d\n%s%s", refusal.name, code, stdout, stderr)
		}
		if main := bed.main(t); main != bed.base {
			t.Fatalf("%s moved main to %s", refusal.name, main)
		}
	}
}

// A moved base goes back to the agent (design r10 K3): landing publish
// pushes nothing onto a main that moved since the series was composed,
// names the new main and the way on, and never republishes; a paused lane
// publishes nothing either.
func TestLandingPublishReturnsAMovedBase(t *testing.T) {
	t.Parallel()
	bed := newPublishBed(t)
	if _, err := lane.SetPause(bed.home, "Wido", laneTestNow); err != nil {
		t.Fatal(err)
	}
	if code, stdout, stderr := bed.publish(t); code == 0 || !strings.Contains(stdout+stderr, "stopped") || bed.main(t) != bed.base {
		t.Fatalf("publish on a paused lane = %d\n%s%s", code, stdout, stderr)
	}
	if _, err := lane.ClearPause(bed.home); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bed.seat, "seat.txt"), []byte("seat\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.git(t, bed.seat, "add", "seat.txt")
	bed.git(t, bed.seat, "commit", "--quiet", "-m", "a seat lands itself")
	bed.git(t, bed.seat, "push", "--quiet", "origin", "HEAD:refs/heads/main")
	moved := bed.main(t)
	code, stdout, stderr := bed.publish(t, "--json")
	var result intentResult
	if err := json.Unmarshal([]byte(stdout+stderr), &result); err != nil || code == 0 || result.Outcome != intentRefused ||
		!strings.Contains(strings.Join(result.Details, "\n"), lane.CodeBaseMoved) || !strings.Contains(result.Summary, "main moved to "+shortLandingID(moved)) {
		t.Fatalf("publish on a moved base = %d %+v %v\n%s%s", code, result, err, stdout, stderr)
	}
	if main := bed.main(t); main != moved {
		t.Fatalf("a moved base was republished: main %s, the seat's %s", main, moved)
	}
}

func init() {
	registerIdempotency("landing publish", idemStateful,
		"the batch's proven series is already on main: success, nothing pushed", witnessLandingPublishRepeat)
}

// witnessLandingPublishRepeat publishes a proven batch twice: the second
// is success and pushes nothing.
func witnessLandingPublishRepeat(t *testing.T) {
	bed := newPublishBed(t)
	if code, stdout, stderr := bed.publish(t); code != 0 {
		t.Fatalf("first publish = %d %q %q", code, stdout, stderr)
	}
	published := bed.main(t)
	if code, stdout, stderr := bed.publish(t); code != 0 || !strings.Contains(stdout, "already on main") || bed.main(t) != published {
		t.Fatalf("repeated publish = %d %q %q", code, stdout, stderr)
	}
}

// The kernel verb's layout golden joins G1b through the group hook: a
// publish refused because the batch's begin record can't be read.
var _ = func() bool {
	layoutGroupCases = append(layoutGroupCases, func() []layoutCase {
		return []layoutCase{{name: "landing-publish-refusal", args: []string{"landing", "publish", "--batch", publishBatch}, bed: landingPublishLayoutBed}}
	})
	return true
}()

func landingPublishLayoutBed(t *testing.T) layoutBed {
	bed := landingEngineLayoutBed(false)(t)
	bed.owners.landing.evidence = lane.NoEvidence{}
	return bed
}
