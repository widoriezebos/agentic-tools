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
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
)

// The hand-in and publication use the real queue and public handlers. Only
// Git transport/containment and the ledger repository are per-test facts.
type trunkPermissionFixture struct {
	b         *deliveryBed
	l         *resolveVerbFixture
	owners    intentOwners
	incidents []goal.TrunkRedEntry
	readErr   error
	conf      string
	tip       *landingOwners
	pushes    *int
}

func newTrunkPermissionFixture(t *testing.T) *trunkPermissionFixture {
	t.Helper()
	b, tip, _ := plainLaneBedWith(t, true, "critic-root", "critic-root")
	incidents := holdIncidentFixture(t, b.intentBed)
	l, pushes := holdLaneFixture(t, incidents)
	b.owners.laneRoot = func(string, time.Time) (string, bool, error) { return l.root, true, nil }
	b.owners.laneInstall = func(string) (string, error) { return l.install, nil }
	owners := b.intentBed.owners()
	owners.delivery, owners.connection, owners.work = b.owners, b.connection, b.work
	owners.landing = l.owners.landing
	owners.landing.machine = func(string) (string, error) { return "fixture", nil }
	owners.prove = enrolledPersonProver(t, b.root(), syncRequestTestNow)
	owners.commandNow = func(string) (time.Time, error) { return syncRequestTestNow, nil }
	owners.processes.question = channel.ReadQuestion
	f := &trunkPermissionFixture{b: b, l: l, owners: owners, incidents: incidents, tip: tip, pushes: pushes}
	f.conf = filepath.Join(l.install, "metasystem.conf")
	helmMust(t, os.WriteFile(f.conf, []byte("metasystem.template=true\nlanding.trunk-red=person\n"), 0600))
	f.owners.policies = config.PolicyReaders{Registry: func(string) (config.PolicyRegistry, error) { return config.PolicyRegistry{Lane: l.root}, nil }, Helm: func(string) helm.State { return helm.State{} }, ConfPath: func(string) (string, error) { return f.conf, nil }}
	notAncestor := exec.Command("/usr/bin/false").Run()
	git := f.owners.landing.plainProve.Git
	f.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "merge-base" {
			return "", notAncestor
		}
		return git(dir, args...)
	}
	f.owners.landing.plainProve.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) { return f.incidents, f.readErr }
	return f
}

func (f *trunkPermissionFixture) land(t *testing.T, args ...string) (int, intentResult) {
	t.Helper()
	return f.b.runJSON(f.owners, append([]string{"work", "land", bedGoal}, args...)...)
}

func (f *trunkPermissionFixture) exception(t *testing.T, name string) (int, intentResult) {
	t.Helper()
	return f.land(t, "--exception", goal.LandTrunkRedCode, "--reason", "Ship this independent work", "--by", name)
}

func (f *trunkPermissionFixture) entry(t *testing.T) plain.Entry {
	t.Helper()
	entry, known, err := plain.Latest(f.l.install, bedGoal)
	if err != nil || !known {
		t.Fatalf("queue subject: %+v %v", entry, err)
	}
	return entry
}

func (f *trunkPermissionFixture) sync(t *testing.T) {
	t.Helper()
	record, known, err := lane.Read(f.l.home)
	if err != nil || !known {
		t.Fatal(err)
	}
	inv := &intentInvocation{owners: f.owners, cwd: f.b.root()}
	layout, err := f.owners.resolver.ResolveLayout(f.b.root())
	helmMust(t, err)
	inv.layout = layout
	seams := inv.laneBatchSeams(f.l.home, record, f.owners.landing.plainProve)
	helmMust(t, plain.SyncPolicyQuestion(f.l.install, f.owners.landing.machine, laneTestNow, seams))
}

func (f *trunkPermissionFixture) question(t *testing.T) channel.Question {
	t.Helper()
	code, held := f.land(t)
	expectOutcome(t, "public held hand-in", code, held, intentRefused)
	questions, unread := channel.WalkOpenQuestions(f.l.install)
	if len(unread) != 0 || len(questions) != 1 {
		t.Fatalf("held-entry questions: %+v %v", questions, unread)
	}
	q := questions[0]
	command := channel.LaneStopCommand(q)
	if !strings.Contains(command, "work land "+bedGoal+" --exception "+goal.LandTrunkRedCode) {
		t.Fatalf("exception remedy: %s", command)
	}
	code, result := f.b.runJSON(f.owners, "question", "show", "channel:"+q.ID, "--repo", f.l.install)
	if code != 0 || !strings.Contains(result.Summary+strings.Join(result.Details, " ")+result.Decision, command) && (result.Next == nil || strings.Join(result.Next.Argv, " ") != command) {
		// The public prose projection carries the facts; the JSON data owns them.
		raw, _ := json.Marshal(result)
		if code != 0 || !bytes.Contains(raw, []byte(command)) {
			t.Fatalf("question show: %d %s", code, raw)
		}
	}
	f.l.owners.policies = f.owners.policies
	f.l.owners.landing = f.owners.landing
	code, text := f.l.run(t, f.l.root, "status", "--json")
	if code != 0 || !strings.Contains(text, "landing.trunk-red") || !strings.Contains(text, "--exception") {
		t.Fatalf("status lost held-entry act: %d %s", code, text)
	}
	return q
}

func TestLandingTrunkProofDefersExceptionQuestionAndStatusAct(t *testing.T) {
	t.Parallel()
	f := newTrunkPermissionFixture(t)
	_, _, err := plain.HandIn(f.l.install, plain.Line{Goal: bedGoal, SHA: f.tip.status.BranchTip, Seat: f.b.root()})
	helmMust(t, err)
	f.question(t)
	f.owners.landing.plainProve.Alive = func(plain.Running) bool { return true }
	writeCauseProof(t, f.l.install, "results.jsonl", plain.Result{Trunk: true, Commit: "main", Result: plain.Red, Failed: []plain.FailedUnit{{Unit: "fixture"}}})
	running := filepath.Join(plain.Dir(f.l.install), "running.json")
	data, err := json.Marshal(plain.Running{Trunk: true, Attempt: "trunk-1", Commit: "main", Tree: "main-tree", Since: laneTestNow.Format(time.RFC3339)})
	helmMust(t, err)
	helmMust(t, os.WriteFile(running, data, 0o600))
	f.l.owners.landing, f.l.owners.policies = f.owners.landing, f.owners.policies
	code, text := f.l.run(t, f.l.root, "status", "--json")
	want := "main main red; its trunk proof runs since " + laneTestNow.Local().Format("15:04") + " (attempt trunk-1); wait"
	if code != 0 || !strings.Contains(text, want) || strings.Contains(text, "--exception") || strings.Contains(text, "pending-actions") {
		t.Fatalf("landing status --json during trunk proof: exit=%d %s; want %q with no person act", code, text, want)
	}
	t.Logf("landing status --json: exit=%d, headline=%q", code, want)
	f.sync(t)
	questions, unread := channel.WalkOpenQuestions(f.l.install)
	if len(questions) != 0 || len(unread) != 0 {
		t.Fatalf("running trunk proof asks for an exception: %+v %v", questions, unread)
	}
	helmMust(t, os.Remove(running))
	f.sync(t)
	questions, unread = channel.WalkOpenQuestions(f.l.install)
	if len(questions) != 1 || len(unread) != 0 || !strings.Contains(channel.LaneStopCommand(questions[0]), "--exception") {
		t.Fatalf("ended trunk proof does not restore the exception question: %+v %v", questions, unread)
	}
}

func TestLandingTrunkExceptionAuthorityMatrix(t *testing.T) {
	t.Parallel()
	f := newTrunkPermissionFixture(t)
	// A pre-existing hand-in must remain visible even when permission holds it.
	_, _, err := plain.HandIn(f.l.install, plain.Line{Goal: bedGoal, SHA: f.tip.status.BranchTip, Seat: f.b.root()})
	helmMust(t, err)
	q := f.question(t)
	invoke := func(name string) (int, intentResult) {
		words := strings.Fields(channel.LaneStopCommand(q))[1:]
		for i := range words {
			if words[i] == "TEXT" {
				words[i] = "Ship this independent work"
			}
			if i > 0 && words[i-1] == "--by" {
				words[i] = name
			}
		}
		return f.b.runJSON(f.owners, words...)
	}
	calling, root := realpath.Resolve(f.b.root()), realpath.Resolve(f.b.root())
	helmMust(t, os.MkdirAll(filepath.Join(calling, ".git"), 0755))
	_, err = humanauthority.Enroll(root, 20, person(), "Wido", syncRequestTestNow)
	helmMust(t, err)
	_, err = helm.Write(calling, helm.Record{By: "Wido", At: syncRequestTestNow.Format(time.RFC3339), Checkout: calling, Reason: "manual"})
	helmMust(t, err)
	fake := newFakeHelm(calling, filepath.Join(calling, ".git"), "DELEGATE")
	useHelmAdmitter(root, fake.admitter)
	t.Cleanup(func() { helmAdmitters.Delete(resolvedHelmRoot(root)) })
	pid := int64(80)
	var observed humanauthority.Proof
	var proofErr error
	f.owners.prove = func(at string, _ int64, _ humanauthority.Reader, _, _ string, now time.Time) (humanauthority.Proof, error) {
		if at != root {
			t.Errorf("exception borrowed enrollment at %s", at)
		}
		observed, proofErr = humanauthority.Prove(root, pid, person(), now)
		return observed, proofErr
	}
	queue := filepath.Join(plain.Dir(f.l.install), "queue.jsonl")
	before, err := os.ReadFile(queue)
	helmMust(t, err)
	fake.machinery = true
	code, result := invoke("Wido")
	expectOutcome(t, "machinery exception", code, result, intentRefused)
	if proofErr == nil {
		t.Fatal("machinery negative did not reach authority walk")
	}
	fake.machinery, pid = false, 60
	if proof, err := humanauthority.Prove(root, pid, person(), syncRequestTestNow); err != nil || proof.Helm == nil {
		t.Fatalf("helm fallback not reached: %+v %v", proof, err)
	}
	code, result = invoke("Wido")
	expectOutcome(t, "helm exception", code, result, intentRefused)
	if proofErr != nil || observed.Helm == nil {
		t.Fatalf("helm fallback missing: %+v %v", observed, proofErr)
	}
	f.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.HelmProof(root, humanauthority.HelmGrant{By: "Wido", Grant: "attorney"}, syncRequestTestNow)
	}
	code, result = invoke("Wido")
	expectOutcome(t, "attorney exception", code, result, intentRefused)
	after, err := os.ReadFile(queue)
	current, qErr := channel.ReadQuestion(f.l.install, q.ID)
	if err != nil || qErr != nil || !bytes.Equal(before, after) || current.State == "closed" {
		t.Fatalf("refused exception changed authority: %+v %v %v", current, err, qErr)
	}
	f.owners.prove = enrolledPersonProver(t, root, syncRequestTestNow)
	code, result = invoke("someone else")
	expectOutcome(t, "wrong person name", code, result, intentRefused)
	_, err = lane.SetPause(f.l.home, "Wido", laneTestNow)
	helmMust(t, err)
	// Incident changes between capture and append refuse without answering.
	originalRead := f.owners.landing.plainProve.Incidents
	reads := 0
	f.owners.landing.plainProve.Incidents = func(install, checkout, main string) ([]goal.TrunkRedEntry, error) {
		reads++
		incidents, err := originalRead(install, checkout, main)
		if reads > 1 {
			changed := append([]goal.TrunkRedEntry(nil), incidents...)
			changed[0].Opened = laneTestNow.Add(time.Hour).Format(time.RFC3339)
			return changed, err
		}
		return incidents, err
	}
	code, result = invoke("Wido")
	if code == 0 {
		t.Fatalf("stale captured incidents admitted: %+v", result)
	}
	after, err = os.ReadFile(queue)
	current, qErr = channel.ReadQuestion(f.l.install, q.ID)
	if err != nil || qErr != nil || !bytes.Equal(before, after) || current.State == "closed" {
		t.Fatal("failed exception recorded authority or closed its request")
	}
	f.owners.landing.plainProve.Incidents = originalRead
	// Unknown queue subjects are a data constraint even at an enrolled terminal.
	damaged := append(append([]byte(nil), before...), []byte("{unreadable queue subject}\n")...)
	helmMust(t, os.WriteFile(queue, damaged, 0600))
	code, result = invoke("Wido")
	after, err = os.ReadFile(queue)
	current, qErr = channel.ReadQuestion(f.l.install, q.ID)
	if code == 0 || err != nil || qErr != nil || !bytes.Equal(damaged, after) || current.State == "closed" {
		t.Fatalf("unknown queue subject acquired exception authority: code=%d result=%+v question=%+v", code, result, current)
	}
	helmMust(t, os.WriteFile(queue, before, 0600))
	code, result = invoke("human:Wido")
	expectOutcome(t, "enrolled exception", code, result, intentUnchanged)
	entry := f.entry(t)
	exception := entry.Exception
	if exception == nil || exception.Code != goal.LandTrunkRedCode || exception.By != "Wido" || exception.Reason != "Ship this independent work" || exception.Goal != bedGoal || exception.SHA != entry.SHA || exception.Person == nil || exception.Person.Root != root || exception.Person.Destination.Install != f.l.install || exception.Incidents == nil || len(exception.Incidents.Open) != 1 || exception.Incidents.Open[0].Identity != f.incidents[0].Identity {
		t.Fatalf("exception subject/provenance: %+v", exception)
	}
	current, err = channel.ReadQuestion(f.l.install, q.ID)
	if err != nil || current.State != "closed" || !strings.Contains(current.ClosedBecause, "recorded person act") {
		t.Fatalf("exception effect did not close request: %+v %v", current, err)
	}
	after, err = os.ReadFile(queue)
	helmMust(t, err)
	code, result = invoke("Wido")
	expectOutcome(t, "identical exception", code, result, intentUnchanged)
	repeated, err := os.ReadFile(queue)
	if err != nil || !bytes.Equal(after, repeated) {
		t.Fatal("identical proved act appended metadata")
	}
	if !helm.Active(calling).Active {
		t.Fatal("exception removed helm")
	}
	if _, paused := lane.ReadPause(f.l.home); !paused {
		t.Fatal("exception removed pause")
	}
	if _, err := humanauthority.ReadEnrollment(f.l.install); err == nil {
		t.Fatal("destination borrowed enrollment")
	}
}

func TestLandingTrunkExceptionFreshCoverageAndReplacement(t *testing.T) {
	t.Parallel()
	f := newTrunkPermissionFixture(t)
	f.incidents[0].FixGoal = bedGoal
	code, result := f.land(t)
	expectOutcome(t, "person policy holds fix goal", code, result, intentRefused)
	code, result = f.exception(t, "Wido")
	expectOutcome(t, "fresh exception", code, result, intentConfirmed)
	original := f.entry(t)
	// A repeated sighting and main commit are not a new incident generation.
	f.incidents[0].Sightings = append(f.incidents[0].Sightings, f.incidents[0].Sightings[0])
	f.incidents[0].Sightings[1].Opid = "01J5X0000000000000000000X3-mac-cli-bbbbbbbb"
	f.incidents[0].Sightings[1].BaseCommit = strings.Repeat("6", 40)
	f.incidents[0].Opened = laneTestNow.Add(time.Hour).Format(time.RFC3339)
	oldQuestion := f.question(t)
	f.l.owners.landing = f.owners.landing
	f.l.owners.policies = f.owners.policies
	f.l.owners.landing.contained = func(_ string, ref string) func(string) (bool, error) {
		return func(sha string) (bool, error) { return ref == "head" && sha == original.SHA, nil }
	}
	code, text := f.l.run(t, f.l.root, "push", "--json")
	if code == 0 || *f.pushes != 0 || !strings.Contains(text, "--exception") {
		t.Fatalf("reopened incident crossed push: %d pushes=%d %s", code, *f.pushes, text)
	}
	f.incidents[0].Sightings[0].Opid = "01J5X0000000000000000000X2-mac-cli-aaaaaaaa"
	code, result = f.exception(t, "Wido")
	expectOutcome(t, "replacement exception", code, result, intentUnchanged)
	current, err := channel.ReadQuestion(f.l.install, oldQuestion.ID)
	if err != nil || current.State != "closed" || !strings.Contains(current.ClosedBecause, "superseded") {
		t.Fatalf("changed subject answered the old request: %+v %v", current, err)
	}
	replacement := f.entry(t)
	entries, err := plain.Entries(f.l.install)
	if err != nil || len(entries) != 1 || replacement.At != original.At || replacement.Exception.Incidents.Open[0].Opened != f.incidents[0].Opened || replacement.Exception.Incidents.Open[0].FirstSeen != f.incidents[0].Sightings[0].Opid {
		t.Fatalf("metadata replacement added a hand-in or retained stale coverage: %+v %v", entries, err)
	}
	f.incidents[0].FixGoal = ""
	f.incidents[0].Sightings = append(f.incidents[0].Sightings, f.incidents[0].Sightings[1])
	f.sync(t)
	if qs, _ := channel.WalkOpenQuestions(f.l.install); len(qs) != 0 {
		t.Fatalf("repeated sighting invalidated binding: %+v", qs)
	}
	code, text = f.l.run(t, f.l.root, "push", "--json")
	if code != 0 || *f.pushes != 1 {
		t.Fatalf("covered exception did not reach push: %d %s", code, text)
	}
	// A new tip has no authority from the old queue entry.
	f.tip.status.BranchTip = strings.Repeat("4", 40)
	code, result = f.land(t)
	expectOutcome(t, "new tip needs exception", code, result, intentRefused)
	if f.entry(t).SHA != original.SHA {
		t.Fatal("held tip replaced old queue authority")
	}
	code, result = f.exception(t, "Wido")
	expectOutcome(t, "person admits new tip", code, result, intentConfirmed)
	if current := f.entry(t); current.SHA != f.tip.status.BranchTip || current.Exception.SHA != current.SHA {
		t.Fatalf("new tip borrowed old exception: %+v", current)
	}
	f.incidents[0].Closed = &goal.TrunkRedClosure{At: laneTestNow.Format(time.RFC3339)}
	f.sync(t)
	if qs, _ := channel.WalkOpenQuestions(f.l.install); len(qs) != 0 {
		t.Fatal("closed incident kept a permission request")
	}
}

func TestLandingTrunkExceptionUnknownBinding(t *testing.T) {
	t.Parallel()
	f := newTrunkPermissionFixture(t)
	register := filepath.Join(t.TempDir(), "trunk-red.json")
	unread := []byte("{unreadable incident bytes\n")
	helmMust(t, os.WriteFile(register, unread, 0600))
	f.owners.landing.plainProve.Incidents = nil
	git := f.owners.landing.plainProve.Git
	f.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		if strings.Join(args, " ") == "show main:metasystem/plans/goals/trunk-red.json" {
			raw, err := os.ReadFile(register)
			return string(raw), err
		}
		return git(dir, args...)
	}
	helmMust(t, os.WriteFile(f.conf, []byte("metasystem.template=true\nlanding.trunk-red=broken\n"), 0600))
	code, result := f.land(t)
	expectOutcome(t, "agent holds on unreadable inputs", code, result, intentRefused)
	if result.Next == nil {
		t.Fatal("unreadable inputs have no remedy")
	}
	words := append([]string(nil), result.Next.Argv[1:]...)
	for i := range words {
		if words[i] == "TEXT" {
			words[i] = "Ship this independent work"
		}
		if i > 0 && words[i-1] == "--by" {
			words[i] = "Wido"
		}
	}
	code, result = f.b.runJSON(f.owners, words...)
	expectOutcome(t, "person records unknown coverage", code, result, intentConfirmed)
	if !strings.Contains(result.Summary, "incident binding unknown") || !f.entry(t).Exception.BindingUnknown {
		t.Fatalf("unknown binding missing: %+v", result)
	}
	f.l.owners.landing, f.l.owners.policies = f.owners.landing, f.owners.policies
	f.l.owners.landing.contained = func(_ string, ref string) func(string) (bool, error) {
		return func(sha string) (bool, error) { return ref == "head" && sha == f.tip.status.BranchTip, nil }
	}
	code, text := f.l.run(t, f.l.root, "push", "--json")
	if code == 0 || *f.pushes != 0 {
		t.Fatalf("unknown binding published: %d %s", code, text)
	}
	retained, err := os.ReadFile(register)
	if err != nil || !bytes.Equal(retained, unread) {
		t.Fatal("person hand-in changed unread incident bytes")
	}
	helmMust(t, os.WriteFile(f.conf, []byte("metasystem.template=true\nlanding.trunk-red=auto\n"), 0600))
	f.incidents[0].FixGoal = bedGoal
	helmMust(t, os.WriteFile(register, goal.RenderTrunkRed(f.incidents), 0600))
	code, text = f.l.run(t, f.l.root, "push", "--json")
	if code == 0 || *f.pushes != 0 {
		t.Fatalf("repair silently bound exception: %d %s", code, text)
	}
	code, result = f.exception(t, "Wido")
	expectOutcome(t, "fresh binding after repair", code, result, intentUnchanged)
	if f.entry(t).Exception.BindingUnknown {
		t.Fatal("replacement did not bind readable incidents")
	}
	code, text = f.l.run(t, f.l.root, "push", "--json")
	if code != 0 || *f.pushes != 1 {
		t.Fatalf("fresh bound exception held: %d %s", code, text)
	}
}

// Real ancestry, proof records and the leased push are required to show that
// exception permission leaves the existing proof and membership gates intact.
func TestLandingTrunkExceptionProvenBatchPush(t *testing.T) {
	t.Parallel()
	b := newBatchVerbBed(t, "auto")
	sha := b.seat(t, "a")
	incidents := holdIncidentFixture(t, newIntentBed(t, false, nil))
	b.configuration += "landing.trunk-red=person\n"
	b.policy(t, "auto")
	b.owners.landing.plainProve.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) { return incidents, nil }
	record, known, err := lane.Read(b.home)
	if err != nil || !known {
		t.Fatal(err)
	}
	proofRoot := filepath.Join(t.TempDir(), "person")
	helmMust(t, os.MkdirAll(proofRoot, 0755))
	prove := enrolledPersonProver(t, proofRoot, b.now)
	proof, err := prove(proofRoot, 20, nil, "", "", b.now)
	helmMust(t, err)
	bind := func() {
		t.Helper()
		exception, err := plain.BindException(plain.GoalSHA{Goal: "a", SHA: sha}, record, proof, proofRoot, "Wido", "Ship this work", b.now, incidents, nil)
		helmMust(t, err)
		_, _, err = plain.HandIn(b.installation, plain.Line{Goal: "a", SHA: sha, Exception: exception})
		helmMust(t, err)
	}
	bind()
	b.success(t, "landing", "run")
	b.assemble(t, sha)
	b.success(t, "landing", "prove", "--gate", "--wait")
	code, text := b.run(t, "landing", "push")
	if code == 0 || !strings.Contains(text, plain.CodeUnproven) {
		t.Fatalf("exception supplied full green: %d %s", code, text)
	}
	b.success(t, "landing", "prove", "--wait")
	later := incidents[0]
	later.Identity += "-new"
	later.Opened = b.now.Add(time.Hour).Format(time.RFC3339)
	incidents = append(incidents, later)
	code, text = b.run(t, "landing", "push")
	if code == 0 || !strings.Contains(text, "--exception") {
		t.Fatalf("new incident escaped fresh push guard: %d %s", code, text)
	}
	if _, found, err := plain.LastPush(b.installation); err != nil || found {
		t.Fatalf("held push wrote evidence: %v %v", found, err)
	}
	bind()
	// Closing one covered incident leaves the other generation authorized.
	incidents[0].Closed = &goal.TrunkRedClosure{At: b.now.Format(time.RFC3339), How: "hand", By: "Wido", Why: "fixed", Opid: incidents[0].Sightings[0].Opid}
	b.success(t, "landing", "push")
	pushed, found, err := plain.LastPush(b.installation)
	if err != nil || !found || pushed.Commit != b.git(t, b.checkout, "rev-parse", "HEAD") {
		t.Fatalf("covered proven batch did not push: %+v %v", pushed, err)
	}
	if incidents[1].Closed != nil {
		t.Fatal("exception closed main incidents")
	}
}

func TestWorkLandLaneContextRemedy(t *testing.T) {
	t.Parallel()
	f := newTrunkPermissionFixture(t)
	f.incidents = nil
	home := f.owners.landing.home
	f.owners.landing.home = func() (string, error) { return "", errors.New("$HOME is not defined") }
	code, result := f.land(t)
	if code == 0 || result.Next == nil || strings.Join(result.Next.Argv, " ") != "export HOME=PATH" {
		t.Fatalf("unknown home needs its own repair: %d %+v", code, result)
	}
	if entries, err := plain.Entries(f.l.install); err != nil || len(entries) != 0 {
		t.Fatalf("unknown lane context queued work: %+v %v", entries, err)
	}
	code, result = f.exception(t, "Wido")
	if code == 0 || result.Next == nil || strings.Join(result.Next.Argv, " ") != "export HOME=PATH" {
		t.Fatalf("exception needs the same lane repair: %d %+v", code, result)
	}
	// Restoring the shell home supplies the same registered lane on retry.
	f.owners.landing.home = home
	code, result = f.land(t)
	expectOutcome(t, "hand-in after home repair", code, result, intentConfirmed)
}
