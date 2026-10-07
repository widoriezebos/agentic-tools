package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/roots"
)

const (
	forceOutsideText = "goal done --force works only while you hold the helm: metasystem helm take --reason TEXT"
	forceAgentText   = "goal done --force is your own act, in the terminal you took the helm in; this shell isn't it"
	forceNoProofText = forceAgentText
)

func TestForceAdmissionAdmitsOnlyThePersonsOwnProofAtTheHelm(t *testing.T) {
	t.Parallel()
	b := newHelmBed(t, 20, true)
	real, err := humanauthority.Prove(b.inst, 20, person(), helmNow)
	helmMust(t, err)
	helmProof, err := humanauthority.HelmProof(b.inst, humanauthority.HelmGrant{By: "wido", Since: helmNow.Format(time.RFC3339), Class: "DELEGATE"}, helmNow)
	helmMust(t, err)
	fixture := real
	fixture.FixtureOnly = true
	active := helm.State{Active: true, Record: helm.Record{By: "wido"}}
	for _, leg := range []struct {
		name  string
		state helm.State
		proof *humanauthority.Proof
		want  string
	}{
		{"helm inactive", helm.State{}, &real, forceOutsideText},
		{"no proof (a session lineage)", active, nil, forceNoProofText},
		{"a helm proof (the agent beside the person)", active, &helmProof, forceAgentText},
		{"a fixture proof", active, &fixture, forceNoProofText},
		{"the person's real proof", active, &real, ""},
	} {
		err := forceAdmission(leg.state, leg.proof, b.inst)
		if got := ""; err != nil {
			got = err.Error()
			if got != leg.want {
				t.Fatalf("%s: %q, want %q", leg.name, got, leg.want)
			}
		} else if leg.want != "" {
			t.Fatalf("%s: admitted", leg.name)
		}
	}
}

// helmForceBed is the goal CLI bed at the helm with ship-widget (claimed by
// another pair, opened by the human) holding one open read item; the person
// proof walks from *invoker: 20 is Wido's enrolled zsh, 80 the agent's bash
// beside him.
func helmForceBed(t *testing.T) (*goalCLIBed, *atomic.Int64, func() *fakeHelm) {
	t.Helper()
	bed := newGoalCLIBed(t, goalCLISeed{noEnrollment: true, amend: func(goals map[string]*goal.GoalFile) {
		goals["ship-widget"].ReadItems = []goal.ReadItem{{ID: "r1-1", Read: "r1", Text: "Name the invariant.", State: goal.ReadItemOpen, AddedAt: goalCLISeedNow.Format(time.RFC3339)}}
	}})
	bed.lineage = ""
	helmMust(t, os.MkdirAll(filepath.Join(bed.root, ".git"), 0o755))
	_, err := humanauthority.Enroll(bed.root, 20, person(), "Wido", helmNow)
	helmMust(t, err)
	invoker := &atomic.Int64{}
	invoker.Store(80)
	bed.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, now time.Time) (humanauthority.Proof, error) {
		return humanauthority.Prove(root, invoker.Load(), person(), now)
	}
	act := func() *fakeHelm {
		fake := newFakeHelm(bed.root, filepath.Join(bed.root, ".git"), "DELEGATE")
		fake.admitter.owners.verb = func() string { return "goal done ship-widget" }
		useHelmAdmitter(bed.root, fake.admitter)
		return fake
	}
	return bed, invoker, act
}

func TestHelmForceIsThePersonsOwnActAtTheHelm(t *testing.T) {
	t.Parallel()
	bed, invoker, act := helmForceBed(t)
	record := bed.goalRecord("ship-widget")
	proposal := "; at the helm you may conclude anyway and record what was overridden: metasystem goal done ship-widget --reason 'landed by hand' --force"

	// Outside the helm: the plain refusal is today's, byte for byte, and the
	// person's --force is refused as outside the helm.
	invoker.Store(20)
	code, out, errOut := bed.public("goal", "done", "ship-widget", "--reason", "landed by hand")
	if code == 0 || !strings.Contains(out+errOut, "goal ship-widget has open review notes r1-1, so it can't be concluded yet") || strings.Contains(out+errOut, "at the helm you may") {
		t.Fatalf("plain done outside the helm: %d %q %q", code, out, errOut)
	}
	code, out, errOut = bed.public("goal", "done", "ship-widget", "--reason", "landed by hand", "--force")
	if code == 0 || !strings.Contains(out+errOut, forceOutsideText) || bed.goalRecord("ship-widget") != record {
		t.Fatalf("--force outside the helm: %d %q %q", code, out, errOut)
	}

	takeHelmAt(t, bed.root)
	// At the helm the plain refusal proposes the force.
	code, out, errOut = bed.public("goal", "done", "ship-widget", "--reason", "landed by hand")
	if code == 0 || !strings.Contains(out+errOut, "→ metasystem goal notes ship-widget"+proposal) {
		t.Fatalf("plain done at the helm: %d %q %q", code, out, errOut)
	}

	// The agent beside the person: refused, one person-proof yield, nothing written (HF-03).
	invoker.Store(80)
	fake := act()
	code, out, errOut = bed.public("goal", "done", "ship-widget", "--reason", "landed by hand", "--force")
	if code != 1 || !strings.Contains(out+errOut, forceAgentText) || len(fake.yields) != 1 || bed.goalRecord("ship-widget") != record {
		t.Fatalf("the agent's --force at the helm: %d %q %q yields %d", code, out, errOut, len(fake.yields))
	}

	// The person at the enrolled terminal: concluded, the override recorded.
	invoker.Store(20)
	code, out, errOut = bed.public("goal", "done", "ship-widget", "--reason", "landed by hand", "--force")
	archived := bed.accepted("records/goals/ship-widget.md")
	if code != 0 || !strings.Contains(archived, "landed by hand — overridden by Wido at the helm: read items r1-1") ||
		!strings.Contains(archived, "overridden by Wido at the helm") || !strings.Contains(archived, " done actor=human:Wido ") {
		t.Fatalf("the person's --force at the helm: %d %q %q\n%s", code, out, errOut, archived)
	}
}

const (
	forceReadRefusal = "goal g1 has open review notes r1-1, so it can't be concluded yet\nrun: metasystem goal notes g1"
	forceQuestion    = "Conclude anyway and record what was overridden? [y/N] "
	forcedCommand    = "metasystem goal done g1 --reason 'landed at the helm by Wido: 2 commits 1111111, 2222222' --force"
)

func TestHelmReturnAsksToForceAnOverridableRefusal(t *testing.T) {
	t.Parallel()
	b := newReturnBed(t, "refs/heads/goal/g1")
	b.refuse = forceReadRefusal
	b.answers = []string{"y", "", "y", "n"}
	out := b.wantReturn(0, "goal done g1: confirmed")
	if len(b.done) != 2 || b.forces[0] || !b.forces[1] || b.done[1] != b.done[0] || len(b.asked) != 4 || b.asked[2] != forceQuestion {
		t.Fatalf("done %+v forces %v asked %q", b.done, b.forces, b.asked)
	}
	// The forced call carries return's own walk: the person's real proof.
	inst, err := filepath.EvalSymlinks(b.inst)
	helmMust(t, err)
	if proof := b.proofs[1]; proof.Helm != nil || proof.FixtureOnly || !proof.TerminalValidFor(inst) {
		t.Fatalf("the forced conclusion's proof %+v", proof)
	}
	// The reason is printed when the question is asked, before the forced
	// conclusion's own line.
	if reason := strings.Index(out, "goal done g1: refused: goal g1 has open review notes r1-1"); reason < 0 || reason > strings.Index(out, "goal done g1: confirmed") {
		t.Fatalf("the reason does not precede the question:\n%s", out)
	}
}

func TestHelmReturnKeepsTheGoalOpenWhenTheForceIsDeclined(t *testing.T) {
	t.Parallel()
	b := newReturnBed(t, "refs/heads/goal/g1")
	b.refuse = forceReadRefusal
	b.answers = []string{"y", "", "n", "n"}
	out := b.wantReturn(0, "goal g1 stays open; to conclude anyway, take the helm again at your terminal and run: "+forcedCommand+"\n")
	if len(b.done) != 1 || !strings.Contains(out, "goal done g1: refused: goal g1 has open review notes r1-1") {
		t.Fatalf("done %+v\n%s", b.done, out)
	}
}

func TestHelmReturnDoesNotAskToForceFromAShellThatIsNotThePersons(t *testing.T) {
	t.Parallel()
	b := newReturnBed(t, "refs/heads/goal/g1")
	b.owners.helm.pid = func() int64 { return 80 }
	b.refuse = forceReadRefusal
	b.answers = []string{"y", "", "n"}
	b.wantReturn(3, "an agent started this shell")
	if len(b.done) != 0 || len(b.asked) != 0 || !helm.Active(b.root).Active {
		t.Fatalf("agent return acted: done %+v asked %q", b.done, b.asked)
	}
}

func TestHelmReturnPrintsTheForcedCommandWithoutATerminal(t *testing.T) {
	t.Parallel()
	b := newReturnBed(t, "refs/heads/goal/g1")
	b.owners.helm.stdinTerminal = func() bool { return false }
	b.wantReturn(0, "if the conclusion refuses for an open read item, review obligation, carry word or blocked dependency, take the helm again at your terminal and run: "+forcedCommand+"\n")
	if len(b.asked) != 0 || len(b.done) != 0 {
		t.Fatalf("without a terminal: asked %q done %+v", b.asked, b.done)
	}
}

// TestHelmReturnForcesThroughTheRealDoneOwner: return's forced conclusion
// runs the real goal done owner in this process, the removed record
// answering as the helm and return's own walk from the enrolled zsh as the
// person's proof; the archive names the override.
func TestHelmReturnForcesThroughTheRealDoneOwner(t *testing.T) {
	t.Parallel()
	bed, _, _ := helmForceBed(t)
	takeHelmAt(t, bed.root)
	answers := []string{"ship-widget", "landed by hand", "y"}
	var asked []string
	var stdout, stderr bytes.Buffer
	owners := bed.owners(&stdout, &stderr)
	owners.helm = helmOwners{reader: person(), pid: func() int64 { return 20 }, now: func() time.Time { return helmNow },
		stdinTerminal: func() bool { return true },
		ask: func(prompt string) (string, bool) {
			asked = append(asked, prompt)
			answer := answers[0]
			answers = answers[1:]
			return answer, true
		},
		holder: func(string) (lease.CurrentHolderView, error) {
			return lease.CurrentHolderView{}, errors.New("no lease")
		},
		recover: func(processScope) string { return "supervision: recovered" },
		fence:   func(roots.Installation) error { return nil },
	}
	command, rest, _ := resolveIntentArgv([]string{"helm", "return"})
	code := runIntentIn(command, rest, &stdout, &stderr, bed.root, owners)
	archived := bed.accepted("records/goals/ship-widget.md")
	if code != 0 || len(asked) != 3 || asked[2] != forceQuestion || !strings.Contains(stdout.String(), "goal done ship-widget: refused: goal ship-widget has open review notes r1-1") ||
		!strings.Contains(archived, "landed by hand — overridden by wido at the helm: read items r1-1") {
		t.Fatalf("return: %d asked %q\n%s\n%s\n%s", code, asked, stdout.String(), stderr.String(), archived)
	}
}

// TestHelmForceEndToEnd drives the real engine with no controlling terminal,
// so no real walk can prove a person: --force from the checkout is refused as
// the helm's admission with the signature written and as outside the helm
// without it, and the plain refusal at the helm ends with the command.
func TestHelmForceEndToEnd(t *testing.T) {
	t.Parallel()
	root := helmLedgerRepo(t, "force-machine")
	helmOpenGoal(t, root, "helm-force")
	if code, out := helmEngine(t, root, "goal", "notes", "helm-force", "--read", "r1", "--add", "Name the invariant.", "--lineage", "fixture-lineage"); code != 0 {
		t.Fatalf("notes: %d\n%s", code, out)
	}
	record := helmLedgerRecord(t, root, "plans/goals/helm-force.md")

	code, out := helmEngine(t, root, "goal", "done", "helm-force", "--reason", "x", "--force", "--lineage", "fixture-lineage")
	if code != 1 || !strings.Contains(out, forceOutsideText) {
		t.Fatalf("--force outside the helm: %d\n%s", code, out)
	}

	seat, err := helm.Locate(root)
	helmMust(t, err)
	_, err = helm.Write(root, helm.Record{By: "wido", At: time.Now().UTC().Format(time.RFC3339), Reason: "e2e", Checkout: seat.Checkout})
	helmMust(t, err)
	code, out = helmEngine(t, root, "goal", "done", "helm-force", "--reason", "x")
	if code != 1 || !strings.Contains(out, "; at the helm you may conclude anyway and record what was overridden: metasystem goal done helm-force --reason x --force") {
		t.Fatalf("plain done at the helm: %d\n%s", code, out)
	}
	code, out = helmEngine(t, root, "goal", "done", "helm-force", "--reason", "x", "--force")
	// The helm let the act through, but --force refused it: the refusal is
	// printed alone, never after a notice that the helm admitted it.
	if code != 1 || !strings.Contains(out, forceAgentText) || strings.Contains(out, "HUMAN AT THE HELM") {
		t.Fatalf("--force at the helm without a terminal: %d\n%s", code, out)
	}
	if after := helmLedgerRecord(t, root, "plans/goals/helm-force.md"); after != record {
		t.Fatalf("a refused force changed the goal:\n%s", after)
	}
}
