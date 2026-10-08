package main

import (
	"bytes"
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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// Selection does not ask Git about candidate topology. These queue and main
// reads are isolated from Git; the adapter tests own actual merge containment.
type selectionBed struct {
	*policyBed
	home        string
	record      lane.Record
	keeper      lane.AgentKeeper
	starts      int
	calls       []string
	pid         int64
	callingRoot string
	proof       humanauthority.Proof
	proofErr    error
	policyErr   error
	machineErr  error
	ids         int
}

func newSelectionBed(t *testing.T) *selectionBed {
	t.Helper()
	b := &selectionBed{policyBed: newPolicyBed(t), home: t.TempDir(), pid: 20}
	helmMust(t, os.MkdirAll(lane.HostDir(b.home), 0700))
	registration, _ := json.Marshal(lane.Record{Root: b.lane, Install: b.lane, CustodyEpoch: 1, RegisteredBy: "Wido"})
	helmMust(t, os.WriteFile(lane.RecordPath(b.home), registration, 0600))
	var err error
	b.record, _, err = lane.Read(b.home)
	helmMust(t, err)
	helmMust(t, os.WriteFile(filepath.Join(b.lane, "metasystem.conf"), []byte("landing.batch=person\nproof.trunk-every=1h\n"), 0600))
	b.owners.processes.question = channel.ReadQuestion
	b.owners.policies.Registry = func(string) (config.PolicyRegistry, error) { return config.PolicyRegistry{Lane: b.lane}, b.policyErr }
	falseState := replayFalseState(t)
	seams := plain.ProveSeams{Now: func() time.Time { return b.now }, NewID: func() string { b.ids++; return fmt.Sprintf("batch-%d", b.ids) },
		Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil },
		Git: func(_ string, args ...string) (string, error) {
			switch args[0] {
			case "fetch", "cat-file":
				return "", nil
			case "rev-parse":
				return "main", nil
			case "merge-base":
				return "", &exec.ExitError{ProcessState: falseState}
			default:
				t.Fatalf("unexpected Git read %v", args)
				return "", nil
			}
		},
	}
	// The observer reads fresh trunk inputs through the same fixture boundary (lane-reads-its-policies.md:145).
	b.keeper = newLandingAgentKeeper(b.lane, b.home, landingAgent{settings: func(string) (launch.Settings, error) { return launch.DefaultSettings(), nil }, proofEffects: seams, now: func() time.Time { return b.now }, machine: func(string) (string, error) { return "fixture", b.machineErr }})
	b.keeper.Running = func() (string, bool, error) { return "", false, nil }
	b.keeper.Fingerprint = nil
	b.keeper.Holds[0] = func(string) (string, error) { return plain.ProofHold(b.lane, seams) }
	b.keeper.Reap = nil
	b.keeper.Waiting = nil
	b.keeper.Sources.Reasons = func(string) ([]string, error) { return []string{plain.WakeQueued}, nil }
	b.keeper.Start = func(string, lane.Wake) (string, error) { b.starts++; return "agent", nil }
	b.owners.landing = laneVerbOwners{home: func() (string, error) { return b.home, nil }, now: func() time.Time { return b.now },
		machine: func(string) (string, error) { return "fixture", b.machineErr }, ready: func(string) error { return nil },
		probe:  func(string) (lane.OwnerProbe, error) { return lane.OwnerProbe{}, nil },
		keeper: func(string, string) lane.AgentKeeper { return b.keeper }, plainProve: seams,
		wake: func(string) lane.WakeSources { return b.keeper.Sources },
	}
	b.owners.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
		b.calls = append(b.calls, root)
		calling := b.lane
		if root == b.seat {
			calling = b.seat
		}
		if root == b.callingRoot {
			calling = b.callingRoot
		}
		b.proof, b.proofErr = humanauthority.Prove(calling, b.pid, person(), at)
		return b.proof, b.proofErr
	}
	for _, g := range []string{"a", "b"} {
		_, _, err := plain.HandIn(b.lane, plain.Line{Goal: g, SHA: "sha-" + g, At: b.now.Format(time.RFC3339Nano)})
		helmMust(t, err)
	}
	return b
}

func (b *selectionBed) run(t *testing.T, cwd string, words ...string) (int, string) {
	t.Helper()
	command, rest, ok := resolveIntentArgv(words)
	if !ok {
		t.Fatalf("no public verb: %v", words)
	}
	var out bytes.Buffer
	code := runIntentIn(command, rest, &out, &out, cwd, b.owners)
	return code, out.String()
}
func (b *selectionBed) question(t *testing.T) channel.Question {
	t.Helper()
	questions, err := channel.WalkOpenQuestions(b.lane)
	if err != nil || len(questions) != 1 {
		t.Fatalf("pending questions: %+v %v", questions, err)
	}
	return questions[0]
}
func (b *selectionBed) proposal(t *testing.T) []string {
	t.Helper()
	_, out := b.run(t, b.lane, "landing", "run")
	q := b.question(t)
	code, text := b.run(t, b.lane, "question", "show", "channel:"+q.ID)
	command := strings.Fields(channel.LaneStopCommand(q))
	if code != 0 || len(command) == 0 || !strings.Contains(text, channel.LaneStopCommand(q)) {
		t.Fatalf("show: %d %s; preparation: %s", code, text, out)
	}
	_, text = b.run(t, b.lane, "landing", "status", "--json")
	if !strings.Contains(text, "pending-actions") || !strings.Contains(text, "--goals") {
		t.Fatalf("status lost selection request: %s", text)
	}
	return command[1:]
}
func (b *selectionBed) batch(t *testing.T) *plain.Batch {
	t.Helper()
	batch, err := plain.ReadBatch(b.lane)
	if err != nil || batch == nil {
		t.Fatalf("batch: %+v %v", batch, err)
	}
	return batch
}
func (b *selectionBed) engine(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(b.lane, "bin", "metasystem")
	trace := filepath.Join(t.TempDir(), "handoff")
	helmMust(t, os.MkdirAll(filepath.Dir(binary), 0755))
	script := "#!/bin/sh\nprintf '%s|%s\\n' \"$(pwd -P)\" \"$*\" > " + shellCommand([]string{trace}) + "\nprintf '%s\\n' '{\"schemaVersion\":1,\"verb\":\"landing run\",\"targets\":[],\"outcome\":\"confirmed\",\"summary\":\"continued\",\"data\":{\"outcome\":\"started\"}}'\n"
	helmMust(t, testexec.WriteFile(binary, []byte(script), 0755))
	return trace
}

func TestLandingSelectionAuthorityAndCrossCheckoutRecord(t *testing.T) {
	t.Parallel()
	b := newSelectionBed(t)
	helmMust(t, os.MkdirAll(filepath.Join(b.seat, ".git", "metasystem"), 0755))
	_, err := helm.Write(b.seat, helm.Record{By: "Wido", At: b.now.Format(time.RFC3339), Reason: "manual", Checkout: b.seat})
	helmMust(t, err)
	_, err = helm.Write(b.lane, helm.Record{By: "Wido", At: b.now.Format(time.RFC3339), Reason: "manual", Checkout: b.lane})
	helmMust(t, err)
	_, err = lane.SetPause(b.home, "Wido", b.now)
	helmMust(t, err)
	command := b.proposal(t)
	before, _ := json.Marshal(b.batch(t))
	q := b.question(t)
	_, err = humanauthority.Enroll(b.seat, 20, person(), "Wido", b.now)
	helmMust(t, err)
	trace := b.engine(t)
	if proof, err := humanauthority.Prove(b.seat, 60, person(), b.now); err == nil || proof.Helm != nil {
		t.Fatalf("the non-machinery caller's real enrolled-terminal walk did not fail: %+v %v", proof, err)
	}
	fake := newFakeHelm(b.seat, filepath.Join(b.seat, ".git"), "DELEGATE")
	useHelmAdmitter(b.seat, fake.admitter)
	t.Cleanup(func() { helmAdmitters.Delete(resolvedHelmRoot(b.seat)) })
	fake.machinery = true
	b.pid = 80
	if code, out := b.run(t, b.seat, command...); code == 0 || b.proofErr == nil {
		t.Fatalf("machinery caller admitted: %d %s %+v", code, out, b.proof)
	}
	fake.machinery = false
	b.pid = 60
	if code, out := b.run(t, b.seat, command...); code == 0 || b.proofErr != nil || b.proof.Helm == nil || len(fake.yields) != 1 {
		t.Fatalf("helm fallback was not reached and refused: %d %s %+v %v yields=%v", code, out, b.proof, b.proofErr, fake.yields)
	}
	b.owners.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
		return humanauthority.HelmProof(b.seat, humanauthority.HelmGrant{By: "Wido", Grant: "attorney"}, at)
	}
	if code, out := b.run(t, b.seat, command...); code == 0 {
		t.Fatalf("attorney selection admitted: %s", out)
	}
	after, _ := json.Marshal(b.batch(t))
	if !bytes.Equal(before, after) || b.question(t).ID != q.ID {
		t.Fatal("refused authority changed the effect or closed the request")
	}
	if _, err := os.Stat(trace); !os.IsNotExist(err) {
		t.Fatal("an unproved caller was routed to the lane engine")
	}
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	words := append(slices.Clone(command), "--by", "human:Wido", "--repo", b.lane, "--json")
	if code, out := b.run(t, b.seat, words...); code != 0 {
		t.Fatalf("direct selection: %d %s", code, out)
	}
	selected := b.batch(t)
	if selected.Person == nil || selected.Person.Person != "Wido" || selected.Person.Root != b.seat || selected.Person.TerminalGeneration != 1 || selected.Person.Destination != b.record || !slices.Equal(selected.Members, []plain.GoalSHA{{Goal: "a", SHA: "sha-a"}, {Goal: "b", SHA: "sha-b"}}) {
		t.Fatalf("selection provenance: %+v", selected)
	}
	seen, err := os.ReadFile(trace)
	if err != nil || strings.TrimSpace(string(seen)) != b.lane+"|landing run --batch "+selected.ID+" --json" {
		t.Fatalf("handoff must carry only batch identity: %q %v", seen, err)
	}
	// The recorded effect answers the matching act request (lane-reads-its-policies.md:86).
	ended, err := channel.ReadQuestion(b.lane, q.ID)
	if err != nil || ended.State != "closed" || !strings.HasPrefix(ended.ClosedBecause, "answered by the recorded person act") {
		t.Fatalf("effect did not close matching subject: %+v %v", ended, err)
	}
	if _, err := humanauthority.ReadEnrollment(b.lane); err == nil {
		t.Fatal("destination enrollment was invented")
	}
	if !helm.Active(b.lane).Active {
		t.Fatal("selection returned the helm")
	}
	if _, paused := lane.ReadPause(b.home); !paused || b.starts != 0 {
		t.Fatal("selection removed a pause or started local execution")
	}
	if code, out := b.run(t, b.seat, append(slices.Clone(command), "--by", "Wido")...); code != 0 || b.batch(t).ID != selected.ID || b.batch(t).Person.Person != "Wido" {
		t.Fatalf("plain enrollment name did not rejoin its selection: %d %s", code, out)
	}
	if code, out := b.run(t, b.seat, append(slices.Clone(command), "--by", "Someone")...); code == 0 {
		t.Fatalf("false --by accepted: %s", out)
	}
	b.owners.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
		return humanauthority.Prove(b.seat, 60, person(), at)
	}
	_, err = humanauthority.Enroll(b.lane, 60, person(), "Destination", b.now)
	helmMust(t, err)
	helmAdmitters.Delete(resolvedHelmRoot(b.seat))
	if code, out := b.run(t, b.seat, append(slices.Clone(command), "--repo", b.lane)...); code == 0 {
		t.Fatalf("failed caller proof borrowed destination enrollment: %s", out)
	}
}

func TestLandingSelectionInternalSelectorNeedsItsRecord(t *testing.T) {
	t.Parallel()
	automatic := newSelectionBed(t)
	helmMust(t, os.WriteFile(filepath.Join(automatic.lane, "metasystem.conf"), []byte("landing.batch=auto\n"), 0600))
	if code, out := automatic.run(t, automatic.lane, "landing", "run"); code != 0 || automatic.starts != 1 {
		t.Fatalf("automatic preparation: %d %s starts=%d", code, out, automatic.starts)
	}
	proposal := automatic.batch(t)
	if code, out := automatic.run(t, automatic.lane, "landing", "run", "--batch", proposal.ID); code == 0 || automatic.starts != 1 {
		t.Fatalf("internal selector without person record launched: %d %s starts=%d", code, out, automatic.starts)
	}
	b := newSelectionBed(t)
	command := b.proposal(t)
	before := b.batch(t)
	if code, out := b.run(t, b.lane, "landing", "run", "--batch", before.ID); code == 0 || b.starts != 0 {
		t.Fatalf("proposal used as authority: %d %s", code, out)
	}
	if code, out := b.run(t, b.lane, "landing", "run", "--batch", "copied-id"); code == 0 || b.starts != 0 {
		t.Fatalf("copied selector executed: %d %s", code, out)
	}
	b.owners.prove = enrolledPersonProver(t, b.lane, b.now)
	if code, out := b.run(t, b.lane, command...); code != 0 || b.starts != 1 {
		t.Fatalf("person selection: %d %s starts=%d", code, out, b.starts)
	}
	selected := b.batch(t)
	_, _, err := plain.HandIn(b.lane, plain.Line{Goal: "c", SHA: "sha-c"})
	helmMust(t, err)
	if code, out := b.run(t, b.lane, "landing", "run", "--batch", selected.ID); code != 0 || b.starts != 2 {
		t.Fatalf("record retry: %d %s", code, out)
	}
	if got := b.batch(t); got.ID != selected.ID || !slices.Equal(got.Members, selected.Members) {
		t.Fatalf("retry selected fresh work: %+v", got)
	}
	// A live landing agent owns this selection; an exact repeated choice can rejoin it,
	// but a different choice must not replace its batch identity.
	selected.State = plain.BatchRunning
	data, _ := json.Marshal(selected)
	helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.lane), "batch.json"), data, 0600))
	b.keeper.Running = func() (string, bool, error) { return "live-agent", true, nil }
	if code, out := b.run(t, b.lane, "landing", "run", "--goals", "c"); code == 0 {
		t.Fatalf("running owner replaced: %s", out)
	}
	if got := b.batch(t); got.ID != selected.ID || !slices.Equal(got.Members, selected.Members) {
		t.Fatal("different choice overwrote running selection")
	}
}

func TestLandingSelectionEffectFailureAndQuestionRecovery(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"record write", "question sync", "broken index", "unreadable policy", "launcher"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			b := newSelectionBed(t)
			command := b.proposal(t)
			q := b.question(t)
			b.owners.prove = enrolledPersonProver(t, b.lane, b.now)
			path := filepath.Join(plain.Dir(b.lane), "batch.json")
			switch failure {
			case "record write":
				b.owners.policies.Registry = func(string) (config.PolicyRegistry, error) {
					// Replace the target after its read and before atomic publication.
					// The resulting rename error is the filesystem's actual failure.
					helmMust(t, os.Remove(path), os.Mkdir(path, 0700))
					return config.PolicyRegistry{Lane: b.lane}, nil
				}
			case "question sync":
				b.machineErr = errors.New("machine temporarily unreadable")
			case "broken index":
				helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.lane), "stop-question"), []byte("{broken"), 0600))
			case "unreadable policy":
				b.policyErr = errors.New("policy unreadable")
			case "launcher":
				b.keeper.Start = func(string, lane.Wake) (string, error) { return "", errors.New("launcher unavailable") }
			}
			code, out := b.run(t, b.lane, command...)
			if failure == "record write" {
				if code == 0 {
					t.Fatalf("failed record claimed success: %s", out)
				}
				current, err := channel.ReadQuestion(b.lane, q.ID)
				if err != nil || current.State == "closed" {
					t.Fatalf("failed effect closed request: %+v %v", current, err)
				}
				return
			}
			selected := b.batch(t)
			if code != 0 || selected.Person == nil {
				t.Fatalf("advice vetoed recorded choice: %d %s %+v", code, out, selected)
			}
			if failure == "question sync" {
				if b.starts != 1 {
					t.Fatal("its own pending policy question vetoed the recorded selection")
				}
				current, err := channel.ReadQuestion(b.lane, q.ID)
				if err != nil || current.State == "closed" {
					t.Fatalf("sync failure falsely closed request: %+v %v", current, err)
				}
				b.machineErr = nil
				b.now = b.now.Add(time.Minute)
				startProofs := 0
				b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
					startProofs++
					return humanauthority.Proof{}, errors.New("the retry is an agent act")
				}
				if code, out := b.run(t, b.lane, "landing", "run"); code != 0 {
					t.Fatalf("public question recovery: %d %s", code, out)
				}
				if b.batch(t).Person.CheckedAt != selected.Person.CheckedAt {
					t.Fatal("question recovery repeated the act")
				}
				if startProofs != 1 {
					t.Fatalf("question recovery checked its invocation's actor %d times, want once", startProofs)
				}
			}
			current, err := channel.ReadQuestion(b.lane, q.ID)
			if err != nil || current.State != "closed" {
				t.Fatalf("recorded effect request not closed: %+v %v", current, err)
			}
			if failure == "launcher" && !strings.Contains(out, "selection recorded, execution not started") {
				t.Fatalf("launcher lost the pending effect: %s", out)
			}
		})
	}
}

func TestLandingSelectionCallingRootOutsideAndLinkedCheckout(t *testing.T) {
	t.Parallel()
	for _, calling := range []string{"outside", "linked", "nameless"} {
		t.Run(calling, func(t *testing.T) {
			t.Parallel()
			b := newSelectionBed(t)
			cwd, root := b.outside, b.lane
			b.engine(t)
			if calling == "linked" {
				cwd = resolvedPath(t.TempDir())
				root = cwd
				gitdir := filepath.Join(b.seat, ".git", "worktrees", "linked")
				helmMust(t, os.MkdirAll(gitdir, 0755), os.WriteFile(filepath.Join(gitdir, "commondir"), []byte("../..\n"), 0600), os.WriteFile(filepath.Join(cwd, ".git"), []byte("gitdir: "+gitdir+"\n"), 0600), os.MkdirAll(filepath.Join(cwd, "scripts", "agents"), 0755), os.WriteFile(filepath.Join(cwd, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0600))
				b.owners.resolver = stateroot.NewResolver(func(path string) (string, error) {
					if strings.HasPrefix(path, cwd) {
						return cwd, nil
					}
					seat, err := helm.Locate(path)
					return seat.Checkout, err
				}, noExecutable)
				b.engine(t)
			} else if calling == "nameless" {
				cwd, root = b.lane, b.lane
			}
			b.callingRoot = root
			name := "Wido"
			enrollment, err := humanauthority.Enroll(root, 20, person(), name, b.now)
			helmMust(t, err)
			if calling == "nameless" {
				enrollment.Human = ""
				data, _ := json.Marshal(enrollment)
				helmMust(t, os.WriteFile(filepath.Join(root, "artifacts", "agents", "authority", "human-terminal.json"), data, 0600))
			}
			code, out := b.run(t, cwd, "landing", "run", "--goals", "b,a", "--repo", b.lane)
			if code != 0 || b.batch(t).Person.Root != root || !slices.Equal(b.calls, []string{root}) {
				t.Fatalf("calling authority root: %d %s calls=%v batch=%+v", code, out, b.calls, b.batch(t))
			}
			if calling == "nameless" {
				if b.batch(t).Person.Person != "author unknown" {
					t.Fatal("nameless enrollment invented attribution")
				}
				if code, out := b.run(t, cwd, "landing", "run", "--goals", "b,a", "--by", "Wido"); code == 0 || !strings.Contains(out, "no recorded name") {
					t.Fatalf("--by fabricated a name: %d %s", code, out)
				}
			}
		})
	}
}

func selectionLines(t *testing.T, path string, values ...any) {
	t.Helper()
	var data []byte
	for _, value := range values {
		line, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, append(line, '\n')...)
	}
	helmMust(t, os.WriteFile(path, data, 0600))
}

func (b *selectionBed) barrenKeeper() {
	b.keeper.Fingerprint = func(string) (string, error) { return "unchanged", nil }
	b.keeper.PersonSelection = func(record lane.Record) bool {
		_, err := plain.RecordedPersonBatch(record.Install, record, "")
		return err == nil
	}
	b.keeper.Prepare = func(record lane.Record) error {
		_, err := plain.SelectBatch(record.Install, record.Root, record, b.owners.landing.plainProve)
		return err
	}
	b.keeper.BarrenStop = func(record lane.Record, state lane.AgentState) error {
		seams := b.owners.landing.plainProve
		seams.Lane = func() (lane.Record, error) { return record, nil }
		return plain.RecordBarrenStop(record.Install, state, lane.AgentStatePath(b.home), b.now, seams)
	}
}

func TestLandingSelectionBarrenTickRefreshesStaleCommand(t *testing.T) {
	t.Parallel()
	for _, stale := range []struct {
		name     string
		required []string
	}{
		{name: "old stop without required command"},
		{name: "trunk proof with newly eligible goals", required: []string{"metasystem", "landing", "prove", "--trunk"}},
		{name: "outdated goal list", required: []string{"metasystem", "landing", "run", "--goals", "a"}},
	} {
		t.Run(stale.name, func(t *testing.T) {
			t.Parallel()
			b := newSelectionBed(t)
			helmMust(t, os.WriteFile(filepath.Join(b.lane, "metasystem.conf"), []byte("landing.batch=auto\n"), 0600))
			b.barrenKeeper()
			state := lane.AgentState{Barren: 2, BarrenFingerprint: "unchanged"}
			data, _ := json.Marshal(state)
			helmMust(t, os.WriteFile(lane.AgentStatePath(b.home), data, 0600))
			stop := plain.Stop{Loop: "lane-return", Subject: "lane", Attempt: 2, Budget: 2,
				Decision: "stop", Handoff: "ask lane", Required: stale.required,
				Evidence: lane.AgentStatePath(b.home), At: b.now.Add(-time.Minute).Format(time.RFC3339Nano)}
			selectionLines(t, filepath.Join(plain.Dir(b.lane), "stops.jsonl"), stop)
			helmMust(t, plain.SyncPolicyQuestion(b.lane, b.owners.landing.machine, b.now))
			oldQuestion := b.question(t)
			if run := b.keeper.Run(); run.Outcome != lane.AgentHeld || b.starts != 0 {
				t.Fatalf("keeper must retain the barren hold: %+v starts=%d", run, b.starts)
			}
			want := "metasystem landing run --goals a,b"
			q := b.question(t)
			if channel.LaneStopCommand(q) != want {
				t.Fatalf("keeper retained stale command %q; want %q", channel.LaneStopCommand(q), want)
			}
			ended, err := channel.ReadQuestion(b.lane, oldQuestion.ID)
			if err != nil || ended.State != "closed" {
				t.Fatalf("stale notification remained open: %+v %v", ended, err)
			}
			for _, words := range [][]string{{"landing", "status"}, {"question", "show", "channel:" + q.ID}} {
				if code, out := b.run(t, b.lane, words...); code != 0 || !strings.Contains(out, want) {
					t.Fatalf("%v did not print refreshed command: %d %s", words, code, out)
				}
			}
			if code, out := b.run(t, b.lane, "landing", "run"); code == 0 || !strings.Contains(oneSpaced(out), want) {
				t.Fatalf("generic refusal lost refreshed remedy: %d %s", code, out)
			}
			b.owners.prove = enrolledPersonProver(t, b.lane, b.now)
			if code, out := b.run(t, b.lane, strings.Fields(channel.LaneStopCommand(q))[1:]...); code != 0 || b.starts != 1 {
				t.Fatalf("following refreshed remedy failed: %d %s starts=%d", code, out, b.starts)
			}
			recorded, err := lane.ReadAgentState(b.home)
			if err != nil || recorded.Barren != 0 || b.batch(t).Person == nil {
				t.Fatalf("remedy did not record selection and clear barren hold: %+v %v", recorded, err)
			}
			ended, err = channel.ReadQuestion(b.lane, q.ID)
			if err != nil || ended.State != "closed" {
				t.Fatalf("successful remedy left its request open: %+v %v", ended, err)
			}
		})
	}
}

func TestLandingSelectionBarrenRemedyIsolatesStopsAndProofBudget(t *testing.T) {
	t.Parallel()
	b := newSelectionBed(t)
	helmMust(t, os.WriteFile(filepath.Join(b.lane, "metasystem.conf"), []byte("landing.batch=auto\n"), 0600))
	b.barrenKeeper()
	state := lane.AgentState{Barren: 2, BarrenFingerprint: "unchanged", BarrenLaunches: []string{"empty-1", "empty-2"}}
	data, _ := json.Marshal(state)
	helmMust(t, os.WriteFile(lane.AgentStatePath(b.home), data, 0600))
	// The stopped goal return and proof are independent subjects, even though
	// their loops share the same installation and the same asking agent.
	goalStop := plain.Stop{Loop: "lane-return", Subject: "a", Attempt: 2, Budget: 2, Decision: "stop", Handoff: "ask lane", At: b.now.Format(time.RFC3339Nano), Cause: &plain.Cause{Kind: "own", Goal: "a"}}
	proofStop := plain.Stop{Loop: "lane-proof", Subject: "a, b", Attempt: 2, Budget: 2, Decision: "stop", Handoff: "ask lane", At: b.now.Format(time.RFC3339Nano), Cause: &plain.Cause{Kind: "environment"}}
	selectionLines(t, filepath.Join(plain.Dir(b.lane), "stops.jsonl"), goalStop, proofStop)
	proof := plain.Result{Attempt: "spent", Result: plain.Red, CountedFull: true, Goals: []plain.GoalSHA{{Goal: "a", SHA: "sha-a"}, {Goal: "b", SHA: "sha-b"}}, At: b.now.Format(time.RFC3339Nano)}
	resultsPath := filepath.Join(plain.Dir(b.lane), "results.jsonl")
	selectionLines(t, resultsPath, proof)
	before, _ := os.ReadFile(resultsPath)
	if code, out := b.run(t, b.lane, "landing", "run"); code == 0 || b.starts != 0 {
		t.Fatalf("generic machinery run lifted barren hold: %d %s", code, out)
	}
	recorded, err := lane.ReadAgentState(b.home)
	if err != nil || recorded.Barren != 2 {
		t.Fatalf("generic run cleared barren count: %+v %v", recorded, err)
	}
	questions, unread := channel.WalkOpenQuestions(b.lane)
	if len(unread) != 0 || len(questions) != 3 {
		t.Fatalf("independent requests: %+v %v", questions, unread)
	}
	var barren channel.Question
	for _, q := range questions {
		if strings.Contains(q.Facts[1], `"subject":"lane"`) {
			barren = q
		}
	}
	if barren.ID == "" || channel.LaneStopCommand(barren) != "metasystem landing run --goals a,b" {
		t.Fatalf("barren command: %+v", barren)
	}
	code, out := b.run(t, b.lane, "question", "show", "channel:"+barren.ID)
	if code != 0 || !strings.Contains(out, channel.LaneStopCommand(barren)) {
		t.Fatalf("question command: %d %s", code, out)
	}
	// Text answers and withdrawing the notification leave its owning stop intact.
	answered := barren
	answered.State = "answered"
	answered.Answer = &channel.Answer{Text: "yes"}
	answerData, _ := json.Marshal(answered)
	helmMust(t, os.WriteFile(filepath.Join(b.lane, "artifacts", "agents", "channel", "questions", barren.ID+".json"), answerData, 0600))
	if code, out := b.run(t, b.lane, "landing", "run"); code == 0 || b.starts != 0 {
		t.Fatalf("text answer became authority: %d %s", code, out)
	}
	_, err = channel.Withdraw(b.lane, barren.ID, "not now", nil, channel.DestinationConfig{})
	helmMust(t, err)
	if code, out := b.run(t, b.lane, "landing", "run"); code == 0 || b.starts != 0 {
		t.Fatalf("withdrawal became authority: %d %s", code, out)
	}
	_, _, err = plain.HandIn(b.lane, plain.Line{Goal: "unrelated", SHA: "sha-unrelated"})
	helmMust(t, err)
	// An unrelated hand-in must not close any request, even if the keeper's
	// fingerprint subsequently allows a new automatic observation.
	helmMust(t, plain.SyncPolicyQuestion(b.lane, b.owners.landing.machine, b.now))
	open, _ := channel.WalkOpenQuestions(b.lane)
	if len(open) != 3 {
		t.Fatalf("unrelated hand-in closed a request: %+v", open)
	}
	b.owners.prove = enrolledPersonProver(t, b.lane, b.now)
	command := strings.Fields(channel.LaneStopCommand(barren))[1:]
	if code, out := b.run(t, b.lane, command...); code != 0 {
		t.Fatalf("printed choice: %d %s", code, out)
	}
	recorded, err = lane.ReadAgentState(b.home)
	if err != nil || recorded.Barren != 0 {
		t.Fatalf("recorded selection did not clear barren count: %+v %v", recorded, err)
	}
	open, _ = channel.WalkOpenQuestions(b.lane)
	if len(open) != 2 || !slices.ContainsFunc(open, func(q channel.Question) bool { return strings.Contains(q.Facts[1], `"subject":"a"`) }) || !slices.ContainsFunc(open, func(q channel.Question) bool { return strings.Contains(q.Facts[1], `"subject":"a, b"`) }) {
		t.Fatalf("selection closed unrelated subjects: %+v", open)
	}
	after, _ := os.ReadFile(resultsPath)
	if !bytes.Equal(before, after) {
		t.Fatal("selection or generic run reopened spent proof allowance")
	}
	selected := b.batch(t)
	data, _ = json.Marshal(state)
	helmMust(t, os.WriteFile(lane.AgentStatePath(b.home), data, 0600))
	if code, out := b.run(t, b.lane, "landing", "run"); code != 0 {
		t.Fatalf("recorded-person generic retry: %d %s", code, out)
	}
	recorded, err = lane.ReadAgentState(b.home)
	if err != nil || recorded.Barren != 0 || !slices.Equal(b.batch(t).Members, selected.Members) {
		t.Fatalf("retry did not retain its recorded selection: %+v %v", recorded, err)
	}
}

func TestLandingSelectionBarrenWithoutEligibleGoalsOffersTrunkProof(t *testing.T) {
	t.Parallel()
	for _, subject := range []string{"all waiting lines held", "no waiting goals"} {
		t.Run(subject, func(t *testing.T) {
			t.Parallel()
			b := newSelectionBed(t)
			if subject == "all waiting lines held" {
				b.owners.landing.plainProve.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
					return []goal.TrunkRedEntry{{Identity: "red-main"}}, nil
				}
			} else {
				helmMust(t, os.Remove(filepath.Join(plain.Dir(b.lane), "queue.jsonl")))
			}
			b.barrenKeeper()
			b.keeper.Sources.Reasons = func(string) ([]string, error) {
				return plain.WakeReasons(b.lane, b.lane, time.Time{}, b.now, b.owners.landing.plainProve)
			}
			for range 2 {
				if code, out := b.run(t, b.lane, "landing", "run"); code != 0 {
					t.Fatalf("full-due launch: %d %s", code, out)
				}
				b.now = b.now.Add(time.Minute)
			}
			if code, out := b.run(t, b.lane, "landing", "run"); code == 0 || b.starts != 2 {
				t.Fatalf("barren launches did not hold: %d %s starts=%d", code, out, b.starts)
			}
			q := b.question(t)
			want := "metasystem landing prove --trunk"
			if channel.LaneStopCommand(q) != want {
				t.Fatalf("no-eligible-goal remedy: %q", channel.LaneStopCommand(q))
			}
			for _, words := range [][]string{{"landing", "status"}, {"question", "show", "channel:" + q.ID}} {
				if code, out := b.run(t, b.lane, words...); code != 0 || !strings.Contains(out, want) || strings.Contains(out, "run --goals") {
					t.Fatalf("%v did not expose executable trunk remedy: %d %s", words, code, out)
				}
			}
		})
	}
}

func TestLandingSelectionCrossCheckoutMissingEngineRetainsChoice(t *testing.T) {
	t.Parallel()
	b := newSelectionBed(t)
	command := b.proposal(t)
	q := b.question(t)
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	if code, out := b.run(t, b.seat, command...); code != 0 || !strings.Contains(out, "selection recorded, execution not started") || !strings.Contains(out, "devgate build") {
		t.Fatalf("missing engine revoked recorded choice: %d %s", code, out)
	}
	selected := b.batch(t)
	if selected.Person == nil || b.starts != 0 {
		t.Fatalf("missing engine lost or executed selection: %+v starts=%d", selected, b.starts)
	}
	ended, err := channel.ReadQuestion(b.lane, q.ID)
	if err != nil || ended.State != "closed" {
		t.Fatalf("recorded choice did not close its subject: %+v %v", ended, err)
	}
	trace := b.engine(t)
	b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		t.Fatal("generic retry asked for another person proof")
		return humanauthority.Proof{}, errors.New("no person")
	}
	if code, out := b.run(t, b.seat, "landing", "run"); code != 0 {
		t.Fatalf("repaired engine retry: %d %s", code, out)
	}
	got, err := os.ReadFile(trace)
	if err != nil || strings.TrimSpace(string(got)) != b.lane+"|landing run --batch "+selected.ID {
		t.Fatalf("retry dropped recorded identity: %q %v", got, err)
	}
	if b.batch(t).Person.CheckedAt != selected.Person.CheckedAt {
		t.Fatal("retry repeated the person's act")
	}
}

func TestLandingSelectionSupersededSubjectIsNotAnswered(t *testing.T) {
	t.Parallel()
	b := newSelectionBed(t)
	command := b.proposal(t)
	q := b.question(t)
	_, _, err := plain.HandIn(b.lane, plain.Line{Goal: "a", SHA: "new-a"})
	helmMust(t, err)
	b.owners.prove = enrolledPersonProver(t, b.lane, b.now)
	if code, out := b.run(t, b.lane, command...); code != 0 {
		t.Fatalf("current subject choice: %d %s", code, out)
	}
	selected := b.batch(t)
	if selected.Members[0].SHA != "new-a" {
		t.Fatalf("selection bound an obsolete commit: %+v", selected)
	}
	ended, err := channel.ReadQuestion(b.lane, q.ID)
	if err != nil || ended.State != "closed" || !strings.HasPrefix(ended.ClosedBecause, "superseded") {
		t.Fatalf("old subject falsely answered: %+v %v", ended, err)
	}
	if code, out := b.run(t, b.lane, "question", "show", "channel:"+q.ID); code != 0 || !strings.Contains(out, "superseded") {
		t.Fatalf("question reader hid supersession: %d %s", code, out)
	}
	_, _, err = plain.HandIn(b.lane, plain.Line{Goal: "a", SHA: "newer-a"})
	helmMust(t, err)
	before := b.starts
	if code, out := b.run(t, b.lane, "landing", "run", "--batch", selected.ID); code == 0 || b.starts != before {
		t.Fatalf("stale recorded selector executed replacement: %d %s", code, out)
	}
}

func TestLandingSelectionCorruptRecordCannotAdmitAndPersonCanReplaceIdle(t *testing.T) {
	t.Parallel()
	b := newSelectionBed(t)
	b.owners.prove = enrolledPersonProver(t, b.lane, b.now)
	if code, out := b.run(t, b.lane, "landing", "run", "--goals", "a,b"); code != 0 {
		t.Fatalf("selection: %d %s", code, out)
	}
	selected := b.batch(t)
	selected.Person.Root = ""
	corrupt, _ := json.Marshal(selected)
	path := filepath.Join(plain.Dir(b.lane), "batch.json")
	helmMust(t, os.WriteFile(path, corrupt, 0600))
	before := b.starts
	if code, out := b.run(t, b.lane, "landing", "run", "--batch", selected.ID); code == 0 || b.starts != before {
		t.Fatalf("incomplete record admitted authority: %d %s", code, out)
	}
	if code, out := b.run(t, b.lane, "landing", "run", "--goals", "a,b"); code != 0 {
		t.Fatalf("idle damaged advice vetoed direct choice: %d %s", code, out)
	}
	if b.batch(t).Person.Root != b.lane {
		t.Fatal("person selection did not replace incomplete provenance")
	}
	saved, err := filepath.Glob(filepath.Join(plain.Dir(b.lane), "batch-unreadable-*.json"))
	if err != nil || len(saved) != 1 {
		t.Fatalf("unread bytes not retained: %v %v", saved, err)
	}
	bytesKept, err := os.ReadFile(saved[0])
	if err != nil || !bytes.Equal(bytesKept, corrupt) {
		t.Fatalf("replacement lost unread record bytes: %q %v", bytesKept, err)
	}
}

func TestLandingSelectionRecordedActDoesNotCloseLaterBarrenRequest(t *testing.T) {
	t.Parallel()
	b := newSelectionBed(t)
	helmMust(t, os.WriteFile(filepath.Join(b.lane, "metasystem.conf"), []byte("landing.batch=auto\n"), 0600))
	b.barrenKeeper()
	state := lane.AgentState{Barren: 2, BarrenFingerprint: "unchanged", BarrenLaunches: []string{"empty-1", "empty-2"}}
	data, _ := json.Marshal(state)
	helmMust(t, os.WriteFile(lane.AgentStatePath(b.home), data, 0600))
	if code, out := b.run(t, b.lane, "landing", "run"); code == 0 {
		t.Fatalf("initial barren hold was not observed: %d %s", code, out)
	}
	initial := b.question(t)
	b.owners.prove = enrolledPersonProver(t, b.lane, b.now)
	if code, out := b.run(t, b.lane, "landing", "run", "--goals", "a,b"); code != 0 || b.starts != 1 {
		t.Fatalf("recorded selection did not restart the lane: %d %s starts=%d", code, out, b.starts)
	}
	selected := b.batch(t)
	if selected.Person.BarrenStopAt == "" {
		t.Fatal("selection did not identify the barren stop it answered")
	}
	b.now = b.now.Add(time.Minute)
	if result := b.keeper.Run(); result.Outcome != lane.AgentStarted || b.starts != 2 {
		t.Fatalf("first automatic retry: %+v starts=%d", result, b.starts)
	}
	b.now = b.now.Add(time.Minute)
	if result := b.keeper.Run(); result.Outcome != lane.AgentHeld || b.starts != 2 {
		t.Fatalf("later barren run did not stop: %+v starts=%d", result, b.starts)
	}
	later := b.question(t)
	if later.ID == initial.ID {
		t.Fatal("later barren run reused an answered request")
	}
	if result := b.keeper.Run(); result.Outcome != lane.AgentHeld || b.starts != 2 {
		t.Fatalf("old person selection approved another automatic retry: %+v starts=%d", result, b.starts)
	}
	stillOpen := b.question(t)
	current, err := lane.ReadAgentState(b.home)
	if err != nil || current.Barren != 2 || stillOpen.ID != later.ID || b.batch(t).Person.CheckedAt != selected.Person.CheckedAt {
		t.Fatalf("recovery changed the later stop or replayed the act: %+v %+v %v", current, stillOpen, err)
	}
}
