package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

type drainVerbBed struct {
	*resolveVerbFixture
	landed    map[string]bool
	person    bool
	launches  int
	keeper    lane.AgentKeeper
	incidents []goal.TrunkRedEntry
}

func newDrainVerbBed(t *testing.T) *drainVerbBed {
	t.Helper()
	b := &drainVerbBed{resolveVerbFixture: newResolveVerbFixture(t), landed: map[string]bool{}, person: true}
	b.owners.resolver = stateroot.NewResolver(fakeTop(b.root), noExecutable)
	b.owners.landing.person = func(string) (string, error) {
		if b.person {
			return "Wido", nil
		}
		return "", errors.New("no enrolled person's terminal")
	}
	b.owners.landing.helm = func(string) helm.State { return helm.State{Active: true} }
	b.owners.landing.view = func(string) lane.View {
		return lane.View{Root: &b.root, Owner: lane.OwnerView{State: lane.OwnerIdle}, Summary: "the landing lane is idle"}
	}
	falseState := replayFalseState(t)
	effects := plain.ProveSeams{Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) { return b.incidents, nil }, Now: func() time.Time { return laneTestNow }, Alive: func(r plain.Running) bool { return r.Pid == 42 }, Git: func(_ string, args ...string) (string, error) {
		switch args[0] {
		case "rev-parse":
			return "main", nil
		case "cat-file":
			return "", nil
		case "merge-base":
			if b.landed[args[2]] {
				return "", nil
			}
			return "", &exec.ExitError{ProcessState: falseState}
		case "ls-tree":
			return "", nil
		case "rev-list", "fetch":
			return "", nil
		}
		t.Fatalf("unexpected Git boundary %v", args)
		return "", nil
	}}
	b.owners.landing.plainProve = effects
	manager := &launch.Manager{Store: launch.Store{Root: t.TempDir()}}
	agent := landingAgent{manager: func() *launch.Manager { return manager }, now: func() time.Time { return laneTestNow }, machine: func(string) (string, error) { return "lane-machine", nil }, proofEffects: effects}
	b.keeper = newLandingAgentKeeper(b.root, b.home, agent)
	b.keeper.Running = func() (string, bool, error) { return "", false, nil }
	b.keeper.Start = func(string, lane.Wake) (string, error) {
		b.launches++
		return fmt.Sprintf("launch-%d", b.launches), nil
	}
	b.keeper.Sources = lane.WakeSources{Reasons: func(string) ([]string, error) {
		return plain.WakeReasons(b.install, b.root, time.Time{}, laneTestNow, effects)
	}}
	b.keeper.Waiting = nil
	b.owners.landing.keeper = func(string, string) lane.AgentKeeper { return b.keeper }
	return b
}

func (b *drainVerbBed) verb(t *testing.T, words ...string) (int, intentResult) {
	t.Helper()
	command, ok := findIntentAction("landing", words[0])
	if !ok {
		t.Fatal(words)
	}
	var out, errOut bytes.Buffer
	code := runIntentIn(command, append(words[1:], "--json"), &out, &errOut, b.root, b.owners)
	var result intentResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("verb %v: exit=%d %s %s", words, code, &out, &errOut)
	}
	return code, result
}
func (b *drainVerbBed) readStatus(t *testing.T) plain.Status {
	t.Helper()
	code, result := b.verb(t, "status")
	data, err := json.Marshal(result.Data)
	var status plain.Status
	if code != 0 || err != nil || json.Unmarshal(data, &status) != nil {
		t.Fatalf("status: %+v %v", result, err)
	}
	return status
}
func seedDrainLine(t *testing.T, install, g, sha string) {
	t.Helper()
	if _, _, err := plain.HandIn(install, plain.Line{Goal: g, SHA: sha}); err != nil {
		t.Fatal(err)
	}
}

func TestLandingDrainAuthorityAndBrokenReadiness(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"pause", "drain", "both", "corrupt"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b := newDrainVerbBed(t)
			if kind == "pause" || kind == "both" {
				if _, err := lane.SetPauseBecause(b.home, "Wido", "maintenance", laneTestNow); err != nil {
					t.Fatal(err)
				}
			}
			if kind != "pause" {
				code, result := b.verb(t, "drain", "--reason", "maintenance")
				expectOutcome(t, "drain", code, result, intentConfirmed)
			}
			if kind == "corrupt" {
				if err := os.WriteFile(plain.DrainPath(b.install), []byte("{broken"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before, readErr := os.ReadFile(plain.DrainPath(b.install))
			_, paused := lane.ReadPause(b.home)
			b.person = false
			code, result := b.verb(t, "start")
			if code == 0 || result.Outcome != intentRefused {
				t.Fatalf("agent reopened %s: %+v", kind, result)
			}
			after, afterErr := os.ReadFile(plain.DrainPath(b.install))
			_, afterPause := lane.ReadPause(b.home)
			if !bytes.Equal(before, after) || errors.Is(readErr, os.ErrNotExist) != errors.Is(afterErr, os.ErrNotExist) || paused != afterPause {
				t.Fatal("agent changed fences")
			}
			code, result = b.verb(t, "drain")
			if code == 0 || result.Outcome != intentRefused {
				t.Fatal("agent closed admission")
			}
			b.person = true
			b.owners.landing.ready = func(string) error {
				return &lane.Refusal{Code: lane.CodeUnarmed, Message: "supervision is not armed", Argv: []string{"metasystem", "session", "start", "--repo", b.root}}
			}
			code, result = b.verb(t, "start")
			if code == 0 || !strings.Contains(result.Summary, "removed") || result.Next == nil || strings.Join(result.Next.Argv, " ") != "metasystem session start --repo "+b.root {
				t.Fatalf("readiness result lost removal or repair: %+v", result)
			}
			if _, err := os.Stat(plain.DrainPath(b.install)); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("person failed to clear drain", err)
			}
			if _, paused := lane.ReadPause(b.home); paused {
				t.Fatal("person failed to clear pause")
			}
			if !b.owners.landing.helm(b.root).Active {
				t.Fatal("start returned the helm")
			}
			if status := b.readStatus(t); status.Admission != "admission open" {
				t.Fatalf("not reopened: %+v", status)
			}
		})
	}
}

func TestLandingDrainWorkLandRemedyAndPersonProvenance(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"goal", "new-commit", "again", "corrupt", "person"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b, owners, install := plainLaneBedWith(t, true, "critic-root", "critic-root")
			actor := kind == "person"
			publicOwners := b.intentBed.owners()
			publicOwners.delivery = b.owners
			publicOwners.connection = b.connection
			publicOwners.work = b.work
			publicOwners.landing.person = func(string) (string, error) {
				if actor {
					return "Wido", nil
				}
				return "", errors.New("agent terminal")
			}
			args := []string{"work", "land", "standing-validation"}
			if kind == "new-commit" {
				seedDrainLine(t, install, "standing-validation", strings.Repeat("1", 40))
			}
			if kind == "again" {
				seedDrainLine(t, install, "standing-validation", owners.status.BranchTip)
				if _, _, err := plain.Return(install, "standing-validation", "own", laneTestNow); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--again")
			}
			if kind == "records" {
				args = append(args, "--records")
			}
			_, _, err := plain.SetDrain(install, plain.Drain{By: "Wido", At: laneTestNow.Format(time.RFC3339), Source: plain.DrainSource{Kind: "person"}})
			if err != nil {
				t.Fatal(err)
			}
			if kind == "person" {
				p, err := plain.AdvanceDrain(install, "checkout", plain.ProveSeams{Git: func(string, ...string) (string, error) { return "main", nil }})
				if err != nil || p.Drain.State != plain.DrainHeld {
					t.Fatal(p, err)
				}
			}
			if kind == "corrupt" {
				if err := os.WriteFile(plain.DrainPath(install), []byte("{bad"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			code, result := b.runJSON(publicOwners, args...)
			if kind == "person" {
				expectOutcome(t, "person hand-in", code, result, intentConfirmed)
				entry, _, _ := plain.Latest(install, "standing-validation")
				drain, _ := plain.ReadDrain(install)
				if entry.DrainBy != "Wido" || drain.State != plain.DrainDraining || !strings.Contains(result.Summary, "extended the drain") {
					t.Fatalf("person extension lost provenance: %+v %+v %+v", entry, drain, result)
				}
				return
			}
			expectOutcome(t, "closed hand-in", code, result, intentRefused)
			if result.Next == nil || !strings.Contains(result.Next.Reason, "landing status --repo /landing") || !strings.Contains(result.Next.Reason, "person reopens admission") || !strings.Contains(strings.Join(result.Next.Argv, " "), "metasystem work land standing-validation") || strings.Contains(strings.Join(result.Next.Argv, " "), "landing start") {
				t.Fatalf("ineffective remedy: %+v", result)
			}
			retry := append([]string(nil), result.Next.Argv[1:]...)
			reopen := newDrainVerbBed(t)
			reopen.install = install
			rec := lane.Record{Root: reopen.root, Install: install, CustodyEpoch: 1, RegisteredBy: "Wido"}
			// The command's canonical layout requires the installation beneath its checkout.
			rec.Root = filepath.Dir(install)
			reopen.root = rec.Root
			b.writeJSON(lane.RecordPath(reopen.home), rec)
			reopen.owners.resolver = stateroot.NewResolver(fakeTop(reopen.root), noExecutable)
			code, start := reopen.verb(t, "start")
			if code != 0 {
				t.Fatalf("person could not reopen: %+v", start)
			}
			if status := reopen.readStatus(t); status.Admission != "admission open" {
				t.Fatalf("reopened status: %+v", status)
			}
			code, result = b.runJSON(publicOwners, retry...)
			expectOutcome(t, "exact retry", code, result, intentConfirmed)
			entry, _, err := plain.Latest(install, "standing-validation")
			if err != nil || entry.SHA != owners.status.BranchTip || entry.State != plain.StateWaiting {
				t.Fatalf("retry did not queue exact commit: %+v %v", entry, err)
			}
			code, result = b.runJSON(publicOwners, args...)
			expectOutcome(t, "identical repeat", code, result, intentUnchanged)
		})
	}
}

func TestLandingDrainKeeperUnknownMembershipAndCompletion(t *testing.T) {
	t.Parallel()
	b := newDrainVerbBed(t)
	b.owners.landing.helm = func(string) helm.State { return helm.State{} }
	seedDrainLine(t, b.install, "known", "known-sha")
	code, result := b.verb(t, "drain")
	expectOutcome(t, "drain", code, result, intentConfirmed)
	queue := filepath.Join(plain.Dir(b.install), "queue.jsonl")
	valid, err := os.ReadFile(queue)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(queue, append(append([]byte(nil), valid...), []byte("{broken\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	run := b.keeper.Run()
	if run.Outcome != lane.AgentStarted {
		t.Fatalf("eligible work stopped: %+v", run)
	}
	drain, _ := plain.ReadDrain(b.install)
	status := b.readStatus(t)
	if b.launches != 1 || drain.State != plain.DrainDraining || !strings.Contains(status.DrainUnknown, "drain membership") || !strings.Contains(status.DrainUnknown, "decoded") {
		t.Fatalf("unknown membership stopped work or looked empty: launches=%d drain=%+v status=%+v", b.launches, drain, status)
	}
	if err := os.WriteFile(queue, valid, 0600); err != nil {
		t.Fatal(err)
	}
	b.incidents = []goal.TrunkRedEntry{{Identity: "incident", Opened: laneTestNow.Format(time.RFC3339)}}
	heldRun := b.keeper.Run()
	drain, _ = plain.ReadDrain(b.install)
	if heldRun.Outcome != lane.AgentIdle || drain.State != plain.DrainDraining || b.launches != 1 {
		t.Fatalf("incident-held work disappeared or woke an agent: %+v %+v", heldRun, drain)
	}
	if status := b.readStatus(t); status.DrainWaiting != 1 || len(status.Queue) != 1 || !status.Queue[0].Held || status.Admission != "draining, 1 waiting" {
		t.Fatalf("incident hold was hidden: %+v", status)
	}
	b.incidents = nil
	if err := os.WriteFile(filepath.Join(plain.Dir(b.install), "running.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if status := b.readStatus(t); !strings.Contains(status.DrainUnknown, "test run process is undecidable") {
		t.Fatalf("unknown process looked finished: %+v", status)
	}
	if err := os.Remove(filepath.Join(plain.Dir(b.install), "running.json")); err != nil {
		t.Fatal(err)
	}
	b.landed["known-sha"] = true
	code, result = b.verb(t, "run")
	if code == 0 || !strings.Contains(result.Summary, "holding") {
		t.Fatalf("completed drain did not hold: %+v", result)
	}
	drain, _ = plain.ReadDrain(b.install)
	if drain.State != plain.DrainHeld || b.launches != 1 {
		t.Fatalf("empty drain launched: %+v %d", drain, b.launches)
	}
	if status := b.readStatus(t); status.Admission != "drained and holding" || status.DrainWaiting != 0 {
		t.Fatalf("held status: %+v", status)
	}
}

func TestLandingDrainKeeperFailuresStaySeparate(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"drain", "question", "both", "write"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b := newDrainVerbBed(t)
			b.owners.landing.helm = func(string) helm.State { return helm.State{} }
			seedDrainLine(t, b.install, "known", "known-sha")
			code, result := b.verb(t, "drain")
			expectOutcome(t, "drain", code, result, intentConfirmed)
			if kind == "drain" || kind == "both" {
				if err := os.WriteFile(plain.DrainPath(b.install), []byte("{broken"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "question" || kind == "both" {
				if err := os.Mkdir(filepath.Join(plain.Dir(b.install), "stop-question"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "write" {
				b.landed["known-sha"] = true
				if err := os.Chmod(plain.Dir(b.install), 0500); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(plain.Dir(b.install), 0700) })
			}
			run := b.keeper.Run()
			if kind == "write" {
				drain, _ := plain.ReadDrain(b.install)
				if run.Outcome != lane.AgentIdle || drain.State != plain.DrainDraining || !strings.Contains(run.Line, "write drain progress") || !strings.Contains(run.Line, "repair") {
					t.Fatalf("write error held keeper or completed drain: %+v %+v", run, drain)
				}
				code, result := b.verb(t, "run")
				if code != 0 || !strings.Contains(result.Summary, "write drain progress") || !strings.Contains(result.Summary, "drain.json") {
					t.Fatalf("public keeper result hid drain failure: %+v", result)
				}
			} else {
				if run.Outcome != lane.AgentHeld {
					t.Fatalf("damaged source bypassed its own fence: %+v", run)
				}
				drainFailure := strings.Contains(run.Line, "read drain ") && strings.Contains(run.Line, "drain.json")
				questionFailure := strings.Contains(run.Line, "synchronize the lane's stop question") && strings.Contains(run.Line, "repair that question source")
				if drainFailure != (kind == "drain" || kind == "both") || questionFailure != (kind == "question" || kind == "both") || strings.Contains(run.Line, "stop question cannot be reconciled") {
					t.Fatalf("lost or mislabeled cause: %+v", run)
				}
			}
		})
	}
}

func TestLandingDrainRechecksWakeUnderHomeLock(t *testing.T) {
	t.Parallel()
	b := newDrainVerbBed(t)
	b.owners.landing.helm = func(string) helm.State { return helm.State{} }
	b.keeper.Sources.Reasons = func(string) ([]string, error) {
		code, result := b.verb(t, "drain")
		expectOutcome(t, "drain during wake", code, result, intentConfirmed)
		return []string{plain.WakeFullDue}, nil
	}
	checked := false
	original := b.keeper.AdmitWake
	b.keeper.AdmitWake = func(root string, wake lane.Wake) (lane.Wake, error) {
		checked = true
		held, err := lock.File(lane.LockPath(b.home), 0600, lock.TryExclusive)
		if err == nil {
			if held != nil {
				_ = held.Release()
			}
			t.Fatal("wake was rechecked outside the home lock")
		}
		return original(root, wake)
	}
	code, result := b.verb(t, "run")
	if code != 0 || b.launches != 0 || !checked || !strings.Contains(result.Summary, "has no queued work") {
		t.Fatalf("stale timer admitted an empty agent: %+v launches=%d checked=%v", result, b.launches, checked)
	}
}

func TestLandingDrainRecordsHandInRetriesExactOptions(t *testing.T) {
	t.Parallel()
	b := newRecordsBed(t)
	seedDrainLine(t, b.lane, "other-goal", strings.Repeat("c", 40))
	b.owners.landing.person = func(string) (string, error) { return "", errors.New("agent terminal") }
	if _, _, err := plain.SetDrain(b.lane, plain.Drain{By: "Wido", At: laneTestNow.Format(time.RFC3339), Source: plain.DrainSource{Kind: "person"}}); err != nil {
		t.Fatal(err)
	}
	code, result := b.land(recordsPath)
	expectOutcome(t, "records fence", code, result, intentRefused)
	if result.Next == nil || !strings.Contains(strings.Join(result.Next.Argv, " "), "--records "+recordsPath) {
		t.Fatalf("record remedy lost options: %+v", result)
	}
	entries, err := plain.Entries(b.lane)
	if err != nil || len(entries) != 1 || entries[0].State != plain.StateWaiting {
		t.Fatalf("records admitted: %+v %v", entries, err)
	}
	reopen := newDrainVerbBed(t)
	if err := os.WriteFile(filepath.Join(b.lane, "metasystem.conf"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(b.lane, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	reopen.root, reopen.install = b.lane, b.lane
	writeQuestionFixture(t, lane.RecordPath(reopen.home), lane.Record{Root: b.lane, Install: b.lane, CustodyEpoch: 1, RegisteredBy: "Wido"})
	reopen.owners.resolver = stateroot.NewResolver(fakeTop(b.lane), noExecutable)
	code, start := reopen.verb(t, "start")
	if code != 0 {
		t.Fatalf("reopen: %+v", start)
	}
	if status := reopen.readStatus(t); status.Admission != "admission open" {
		t.Fatalf("closed status: %+v", status)
	}
	code, result = b.runJSON(b.owners, result.Next.Argv[1:]...)
	expectOutcome(t, "record retry", code, result, intentConfirmed)
	entries, err = plain.Entries(b.lane)
	if err != nil || len(entries) != 2 || entries[0].State != plain.StateWaiting || !entries[1].Records || entries[1].SHA != b.published {
		t.Fatalf("retry did not queue published record: %+v %v", entries, err)
	}
}

func TestLandingStartRepairsUnreadableKeeperRecordForAgent(t *testing.T) {
	t.Parallel()
	b := newDrainVerbBed(t)
	b.person = false
	if err := os.WriteFile(lane.AgentStatePath(b.home), []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := lane.ReadAgentState(b.home); err == nil {
		t.Fatal("keeper record fixture is readable")
	}
	checkedReadiness := false
	b.owners.landing.ready = func(string) error {
		checkedReadiness = true
		if _, err := lane.ReadAgentState(b.home); err == nil {
			t.Fatal("keeper record was repaired before readiness")
		}
		return nil
	}
	code, result := b.verb(t, "start")
	if code != 0 || !checkedReadiness {
		t.Fatalf("agent start did not pass readiness: %+v checked=%v", result, checkedReadiness)
	}
	state, err := lane.ReadAgentState(b.home)
	if err != nil {
		t.Fatalf("agent start did not repair the keeper record: %+v %v", state, err)
	}
	if _, starting, err := lane.AgentStarting(b.home, laneTestNow); err != nil || starting {
		t.Fatalf("repaired keeper still refuses to start: starting=%v err=%v", starting, err)
	}
}

func TestLandingDrainObserverReleasesHomeLock(t *testing.T) {
	t.Parallel()
	b := newDrainVerbBed(t)
	seedDrainLine(t, b.install, "known", "known-sha")
	code, result := b.verb(t, "drain")
	expectOutcome(t, "drain", code, result, intentConfirmed)
	observed := false
	original := b.keeper.Observe
	b.keeper.Observe = func(record lane.Record) error {
		observed = true
		home, err := lock.File(lane.LockPath(b.home), 0600, lock.TryExclusive)
		if err != nil {
			t.Error("keeper retained home lock before taking queue lock")
			return err
		}
		if err := home.Release(); err != nil {
			return err
		}
		return original(record)
	}
	run := b.keeper.Run()
	if !observed || run.Outcome != lane.AgentStarted {
		t.Fatalf("observation prevented admitted work: %+v observed=%v", run, observed)
	}
}

func TestLandingDrainFinishesAdmittedProofAndPush(t *testing.T) {
	t.Parallel()
	delivery, state, install := plainLaneBedWith(t, true, "critic-root", "critic-root")
	publicOwners := delivery.intentBed.owners()
	publicOwners.delivery = delivery.owners
	publicOwners.connection = delivery.connection
	publicOwners.work = delivery.work
	publicOwners.landing.person = func(string) (string, error) { return "", errors.New("agent terminal") }
	code, result := delivery.runJSON(publicOwners, "work", "land", "standing-validation")
	expectOutcome(t, "first hand-in", code, result, intentConfirmed)
	second := delivery.goalFile("standing-validation")
	second.Id = "second"
	delivery.addGoal(second)
	delivery.writeFile(filepath.Join(delivery.root(), "plans", "designs", "second.md"), "# Second goal\n\n- Kind: design\n- Id: second-design\n- Status: accepted\n- Goals: second\n\n## Units\n\n| Unit | Lines |\n| --- | ---: |\n| u1 | 5 |\n| u2 | 5 |\n")
	code, result = delivery.runJSON(publicOwners, "work", "land", "second")
	expectOutcome(t, "second hand-in", code, result, intentConfirmed)
	b := newReplayVerbBed(t)
	b.install = install
	b.root = filepath.Dir(install)
	delivery.writeJSON(lane.RecordPath(b.home), lane.Record{Root: b.root, Install: install, CustodyEpoch: 1, RegisteredBy: "Wido"})
	if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), []byte("metasystem.template=true\nproof.full=fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	b.owners.resolver = stateroot.NewResolver(fakeTop(b.root), noExecutable)
	falseState := replayFalseState(t)
	main := "main"
	// All Git operations are injected; proof execution and push use their real owners.
	git := b.owners.landing.plainProve.Git
	b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "merge-base" {
			if args[3] == "merge-b" || main == "merge-b" && args[3] == "main" {
				return "", nil
			}
			return "", &exec.ExitError{ProcessState: falseState}
		}
		if args[0] == "push" {
			main = "merge-b"
			return "", nil
		}
		if args[0] == "fetch" {
			return "", nil
		}
		if args[0] == "show" && strings.HasSuffix(args[len(args)-1], ":metasystem/metasystem.conf") {
			return "metasystem.template=true\nproof.full=fixture\n", nil
		}
		if args[0] == "rev-parse" && strings.Contains(strings.Join(args, " "), "refs/remotes/origin/main") {
			return main, nil
		}
		return git(dir, args...)
	}
	effects := b.owners.landing.plainProve
	b.owners.landing.person = func(string) (string, error) { return "", errors.New("agent terminal") }
	b.owners.landing.contained = func(_ string, ref string) func(string) (bool, error) {
		return func(sha string) (bool, error) { return ref == "merge-b" || main == "merge-b" && ref == "main", nil }
	}
	delivery.repo.commits["merge-b"] = delivery.repo.commit(delivery.repo.canonical)
	b.owners.landing.mainEndpoint = func(string) (goal.Endpoint, error) { return delivery.dependencies().endpoint(delivery.root()) }
	b.owners.landing.machine = func(string) (string, error) { return "mac-cli", nil }
	b.owners.landing.push = func(install, checkout string, now time.Time, before func(string, string) error) (plain.PushOutcome, error) {
		return plain.PushChecked(install, checkout, now, before, effects)
	}
	b.fail = func(*exec.Cmd, string) (string, error) { return "LANDING-CHECKED\t0\n", nil }
	// Start is the proof already admitted before drain closes; its detached child finishes afterward.
	b.owners.landing.plainProve.Executable = func() (string, error) { return "engine", nil }
	b.owners.landing.plainProve.Launch = func([]string, string, string) (int64, error) { return 42, nil }
	b.owners.landing.plainProve.Alive = func(r plain.Running) bool { return r.Pid == 42 }
	code, text := b.run(t, b.root, "prove", "--json")
	if code != 0 {
		t.Fatal(text)
	}
	var started struct{ Data plain.Running }
	if err := json.Unmarshal([]byte(text), &started); err != nil {
		t.Fatal(err)
	}
	person := b.owners.landing.person
	b.owners.landing.person = func(string) (string, error) { return "Wido", nil }
	code, text = b.run(t, b.root, "drain", "--json")
	if code != 0 {
		t.Fatal(text)
	}
	b.owners.landing.person = person
	progress, err := plain.AdvanceDrain(install, b.root, b.owners.landing.plainProve)
	if err != nil || progress.Waiting != 2 || progress.Drain.State != plain.DrainDraining {
		t.Fatalf("active drain: %+v %v", progress, err)
	}
	code, text = b.run(t, b.root, "prove", "--wait", "--attempt", started.Data.Attempt, "--json")
	if code != 0 {
		t.Fatalf("admitted proof stopped: %s", text)
	}
	proof, found, err := plain.ResultFor(install, "merge-b-tree")
	if err != nil || !found || proof.Result != plain.Green {
		t.Fatalf("proof not persisted: %+v %v", proof, err)
	}
	code, text = b.run(t, b.root, "push", "--json")
	if code != 0 {
		t.Fatalf("proved work could not push: %s", text)
	}
	if main != "merge-b" {
		t.Fatal("push did not move main")
	}
	if _, ok, err := plain.LastPush(install); err != nil || !ok {
		t.Fatal("push was not persisted", err)
	}
	progress, err = plain.AdvanceDrain(install, b.root, effects)
	if err != nil || progress.Waiting != 0 || progress.Drain.State != plain.DrainHeld {
		t.Fatalf("landed drain did not hold: %+v %v", progress, err)
	}
	if state.candidates != 0 || len(state.pushes) != 0 {
		t.Fatal("seat proved or pushed instead of handing in")
	}
}

func TestLandingDrainExplicitPersonProof(t *testing.T) {
	t.Parallel()
	b := newReplayVerbBed(t)
	for _, g := range []string{"a", "b", "c"} {
		if _, _, err := plain.Return(b.install, g, "done", laneTestNow); err != nil {
			t.Fatal(err)
		}
	}
	code, text := b.run(t, b.root, "drain", "--json")
	if code != 0 {
		t.Fatal(text)
	}
	base := b.owners.landing.plainProve.Git
	b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		if strings.Join(args, " ") == "rev-parse --verify origin/main^{commit}" {
			return "main", nil
		}
		if args[0] == "fetch" {
			return "", nil
		}
		return base(dir, args...)
	}
	b.owners.landing.person = func(string) (string, error) { return "", errors.New("agent terminal") }
	code, text = b.run(t, b.root, "prove", "--trunk", "--wait", "--json")
	if code == 0 {
		t.Fatal("automatic empty proof crossed drain", text)
	}
	if _, ok, err := plain.LastResult(b.install); err != nil || ok {
		t.Fatal("refused proof wrote result", err)
	}
	b.owners.landing.person = func(string) (string, error) { return "Wido", nil }
	b.fail = func(*exec.Cmd, string) (string, error) { return "LANDING-CHECKED\t0\n", nil }
	ledger := newIntentBed(t, false, nil)
	ledger.repo.commits["main"] = ledger.repo.commit(ledger.repo.canonical)
	b.owners.landing.mainEndpoint = func(string) (goal.Endpoint, error) { return ledger.dependencies().endpoint(ledger.root()) }
	b.owners.landing.machine = func(string) (string, error) { return "mac-cli", nil }
	code, text = b.run(t, b.root, "prove", "--trunk", "--wait", "--json")
	if code != 0 {
		t.Fatalf("person proof refused under drain: %s", text)
	}
	if _, _, err := plain.HandIn(b.install, plain.Line{Goal: "a", SHA: "sha-a", Again: true}, "Wido"); err != nil {
		t.Fatal(err)
	}
	b.owners.landing.plainProve.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
		return []goal.TrunkRedEntry{{Identity: "incident", Opened: laneTestNow.Format(time.RFC3339)}}, nil
	}
	p, err := plain.AdvanceDrain(b.install, b.root, b.owners.landing.plainProve)
	if err != nil || p.Waiting != 1 || p.Drain.State != plain.DrainDraining {
		t.Fatalf("incident waiting membership lost: %+v %v", p, err)
	}
	code, text = b.run(t, b.root, "prove", "--trunk", "--wait", "--json")
	if code != 0 {
		t.Fatalf("person incident proof refused under drain: %s", text)
	}
	proof, ok, err := plain.LastResult(b.install)
	drain, _ := plain.ReadDrain(b.install)
	if err != nil || !ok || !proof.Trunk || proof.Result != plain.Green || drain == nil {
		t.Fatalf("person proof not persisted or cleared drain: %+v %+v %v", proof, drain, err)
	}
}

func TestLandingDrainRegenerationAndKeeperComplete(t *testing.T) {
	t.Parallel()
	b := newDrainVerbBed(t)
	seedDrainLine(t, b.install, "goal", "goal-sha")
	code, result := b.verb(t, "drain")
	expectOutcome(t, "drain", code, result, intentConfirmed)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	b.owners.landing.plainResolve = plain.ResolveSeams{Now: func() time.Time { return laneTestNow }, Run: func(_ []string, _ string, _ *os.File, started func(int64) error) error {
		if err := started(0); err != nil {
			return err
		}
		once.Do(func() { close(entered); <-release })
		return nil
	}, Git: func(_ string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "show HEAD:metasystem/testing.json":
			return b.contract, nil
		case "diff --name-only --diff-filter=U -z":
			return "metasystem/out/conflict\x00", nil
		case "rev-parse --verify HEAD^{commit}":
			return "main-sha", nil
		case "rev-parse --verify MERGE_HEAD^{commit}":
			return "goal-sha", nil
		case "diff --name-only -z AUTO_MERGE --", "ls-files --others --exclude-standard -z":
			return "", nil
		case "ls-files -z":
			return "metasystem/out/conflict\x00", nil
		}
		if args[0] == "ls-tree" {
			return "metasystem/out/conflict\x00", nil
		}
		if args[0] == "ls-files" {
			return "100644 base 1\tmetasystem/out/conflict\x00100644 main 2\tmetasystem/out/conflict\x00100644 goal 3\tmetasystem/out/conflict\x00", nil
		}
		if args[0] == "diff" {
			return "@@ -2,0 +3 @@\n+insert\n", nil
		}
		if args[0] == "restore" || args[0] == "add" {
			return "", nil
		}
		t.Fatalf("unexpected regeneration Git %v", args)
		return "", nil
	}}
	resolveDone := make(chan int, 1)
	go func() { code, _ := b.verb(t, "resolve"); resolveDone <- code }()
	<-entered
	observing := make(chan struct{})
	original := b.keeper.Observe
	b.keeper.Observe = func(record lane.Record) error {
		home, err := lock.File(lane.LockPath(b.home), 0600, lock.TryExclusive)
		if err != nil {
			t.Error("keeper retained the home lock before waiting for regeneration")
			close(observing)
			return err
		}
		_ = home.Release()
		close(observing)
		return original(record)
	}
	keeperDone := make(chan lane.AgentRun, 1)
	go func() { keeperDone <- b.keeper.Run() }()
	<-observing
	// A second explicit drain also waits for regeneration's queue lock, without retaining the home lock.
	draining := make(chan struct{})
	drainDone := make(chan int, 1)
	go func() { close(draining); code, _ := b.verb(t, "drain"); drainDone <- code }()
	<-draining
	home, err := lock.File(lane.LockPath(b.home), 0600, lock.TryExclusive)
	if err != nil {
		t.Fatal("a drain wait retained the home lock", err)
	}
	_ = home.Release()
	close(release)
	if code := <-resolveDone; code != 0 {
		t.Fatal("regeneration did not finish", code)
	}
	if code := <-drainDone; code != 0 {
		t.Fatal("waiting drain did not finish", code)
	}
	run := <-keeperDone
	if run.Outcome != lane.AgentStarted {
		t.Fatalf("keeper did not resume admitted work: %+v", run)
	}
	drain, _ := plain.ReadDrain(b.install)
	if drain.State != plain.DrainDraining {
		t.Fatal("regeneration hid waiting membership")
	}
}

func TestLandingDrainStartReportsPartialRemoval(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"drain", "pause"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			b := newDrainVerbBed(t)
			code, result := b.verb(t, "drain")
			expectOutcome(t, "drain", code, result, intentConfirmed)
			if _, err := lane.SetPause(b.home, "Wido", laneTestNow); err != nil {
				t.Fatal(err)
			}
			dir := plain.Dir(b.install)
			if failure == "pause" {
				dir = lane.HostDir(b.home)
			}
			if err := os.Chmod(dir, 0500); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(dir, 0700) })

			checkedReadiness := false
			b.owners.landing.ready = func(string) error { checkedReadiness = true; return nil }
			code, result = b.verb(t, "start")
			if code == 0 || result.Outcome != intentFailed || !strings.Contains(result.Summary, "partly reopened") {
				t.Fatalf("partial removal misreported: %+v", result)
			}
			if !checkedReadiness {
				t.Fatal("partial removal hid independent readiness")
			}

			_, paused := lane.ReadPause(b.home)
			_, drainErr := os.Stat(plain.DrainPath(b.install))
			if paused != (failure == "pause") || errors.Is(drainErr, os.ErrNotExist) != (failure == "pause") {
				t.Fatalf("partial state lost: pause=%v drain=%v", paused, drainErr)
			}
			if failure == "pause" && strings.Contains(result.Summary, "removed the pause") {
				t.Fatal("failed pause removal reported success")
			}
			if err := os.Chmod(dir, 0700); err != nil {
				t.Fatal(err)
			}
			code, result = b.verb(t, "start")
			if code != 0 {
				t.Fatalf("retry did not complete removals: %+v", result)
			}
			if status := b.readStatus(t); status.Admission != "admission open" || status.Paused {
				t.Fatalf("partial removal retry left a fence: %+v", status)
			}
		})
	}
}

func TestLandingDrainAgentStartKeepsLateFences(t *testing.T) {
	t.Parallel()
	b := newDrainVerbBed(t)
	b.owners.landing.person = func(string) (string, error) {
		if _, err := lane.SetPause(b.home, "Wido", laneTestNow); err != nil {
			t.Fatal(err)
		}
		if _, _, err := plain.SetDrain(b.install, plain.Drain{By: "Wido", At: laneTestNow.Format(time.RFC3339), Source: plain.DrainSource{Kind: "person"}}); err != nil {
			t.Fatal(err)
		}
		return "", errors.New("agent terminal")
	}
	code, result := b.verb(t, "start")
	drain, _ := plain.ReadDrain(b.install)
	_, paused := lane.ReadPause(b.home)
	if code == 0 || result.Outcome != intentRefused || drain == nil || !paused {
		t.Fatalf("late fences were cleared: %+v %+v pause=%v", result, drain, paused)
	}
}
