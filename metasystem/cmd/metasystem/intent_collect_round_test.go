package main

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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// doneRoundStarter supplies the builder's result after the fixture finishes
// the launch; its files and the unit's collection are production paths.
type doneRoundStarter struct {
	launch.SupervisorStarter
	bed *workBed
}

func (s doneRoundStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	ref, err := s.SupervisorStarter.StartSupervisor(id, state)
	r, _ := s.bed.manager.Store.Read(id)
	if r.Kind == "build" {
		if writeErr := os.WriteFile(filepath.Join(state, "last-message.txt"), []byte("RESULT: DONE\n"), 0600); writeErr != nil {
			return ref, writeErr
		}
	}
	return ref, err
}

func TestADoneVerifyRoundIsNoGap(t *testing.T) {
	t.Parallel()
	b := realOutcomeBed(t)
	b.manager.Supervisor = doneRoundStarter{outcomeStarter{bed: b, build: func() {
		if err := os.WriteFile(filepath.Join(b.worktree, "unit.go"), []byte("new\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}}, b}
	code, built, _ := outcomeBuild(t, b)
	if code != 0 {
		t.Fatalf("first build: %d %+v", code, built)
	}
	code, verified, _ := b.work("work", "revise", b.id, "--work", "outcome", "--after", "1", "--brief", b.brief("verify.md", "Verify the complete change.\n"), "--reason", "Verify the change", "--by", "Wido")
	if code != 0 || resultData(t, verified)["outcome"] == "build-gap" || !slices.Equal(b.starter.launched(), []string{"build", "proof", "build", "proof"}) {
		t.Fatalf("DONE verification became a gap: %d %+v %v", code, verified, b.starter.launched())
	}
	r, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(resultData(t, built)["run"].(string))
	if err != nil || r.Rounds[1].BuildLines != 0 {
		t.Fatalf("counted prior lines: %+v %v", r, err)
	}
}

func TestADoneRoundWithoutAUnitChangeIsGap(t *testing.T) {
	t.Parallel()
	for _, records := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty", true: "records only"}[records], func(t *testing.T) {
			t.Parallel()
			b := realOutcomeBed(t)
			b.manager.Supervisor = doneRoundStarter{outcomeStarter{bed: b, build: func() {
				if records {
					for _, prefix := range []string{"records", "metasystem/records"} {
						path := filepath.Join(b.worktree, prefix, "read.json")
						if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(path, []byte("{}\n"), 0600); err != nil {
							t.Fatal(err)
						}
					}
				}
			}}, b}
			code, held, _ := outcomeBuild(t, b)
			if code != 1 || resultData(t, held)["outcome"] != "build-gap" || !slices.Equal(b.starter.launched(), []string{"build"}) {
				t.Fatalf("true gap passed: %d %+v %v", code, held, b.starter.launched())
			}
		})
	}
}

// completedRound leaves exactly the state a finished, uncollected build has.
func completedRound(t *testing.T) (*workBed, string, string) {
	t.Helper()
	b := outcomeBed(t, "diff --git a/unit.go b/unit.go\n--- a/unit.go\n+++ b/unit.go\n+new\n")
	b.starter.hold = "build"
	_, built, _ := outcomeBuild(t, b)
	id := resultData(t, built)["run"].(string)
	r, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(id)
	if err != nil {
		t.Fatal(err)
	}
	launchID := r.Rounds[0].Steps[0].LaunchID
	if _, err := b.manager.Store.Update(launchID, func(r *launch.Record) error {
		zero := 0
		r.State, r.ExitCode, r.FinishedAt = launch.Completed, &zero, b.manager.Now().Format(time.RFC3339)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	b.starter.hold = ""
	return b, id, launchID
}

func collectionOwnership(t *testing.T, b *workBed, holderLineage string, holder bool) {
	t.Helper()
	pid := int64(os.Getpid())
	if !holder {
		command := exec.Command("/bin/sh", "-c", testexec.ReadyPrologue+"read item")
		input, err := command.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := testexec.StartReady(command); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = input.Close(); _ = command.Wait() })
		pid = int64(command.Process.Pid)
	}
	exact, state, err := (identity.KernelProber{}).Probe(pid)
	if err != nil || state != identity.Alive {
		t.Fatalf("holder identity: %v %v", state, err)
	}
	if _, err := lease.CurrentHolder(b.root()); err == nil {
		data, err := os.ReadFile(filepath.Join(b.root(), "artifacts", "agents", "mains", "worktree-lease.json"))
		if err != nil {
			t.Fatal(err)
		}
		var current lease.Lease
		if err := json.Unmarshal(data, &current); err != nil {
			t.Fatal(err)
		}
		current.OwnerLineage = holderLineage
		data, _ = json.Marshal(current)
		writeTestingFixtureFile(t, filepath.Join(b.root(), "artifacts", "agents", "mains", "worktree-lease.json"), data, 0600)
		for _, ann := range lease.AnnouncementsFor(b.root(), pid) {
			ann.OwnerLineage = holderLineage
			data, _ = json.Marshal(ann)
			writeTestingFixtureFile(t, filepath.Join(b.root(), "artifacts", "agents", "mains", fmt.Sprintf("collection-holder-%d.json", pid)), data, 0600)
		}
		return
	}
	if _, err := lease.AnnounceWithPair(b.root(), "collection-holder", pid, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "fixture", "fake", holderLineage); err != nil {
		t.Fatal(err)
	}
}

func collectWithOwners(t *testing.T, b *workBed, id string, owners intentOwners, extra ...string) (int, intentResult) {
	t.Helper()
	timeout := "30m"
	if len(extra) > 0 {
		timeout = "1m"
	}
	return b.runJSON(owners, append([]string{"work", "wait", "run:" + id, "--timeout", timeout}, extra...)...)
}

func collectionOwners(t *testing.T, b *workBed) intentOwners {
	t.Helper()
	o := b.workOwners()
	endpoint, err := b.dependencies().endpoint(b.root())
	if err != nil {
		t.Fatal(err)
	}
	config := func(string, string) (string, error) { return "mac-cli", nil }
	holderRoot := func(string) string { return b.root() }
	o.connection.claimCheck = func(root, id string, _ goal.Endpoint) func() error {
		return goalBranchClaimCheckWith(root, id, endpoint, config, holderRoot)
	}
	o.connection.ownershipCheck = func(root, id string, _ goal.Endpoint) func() error {
		return goalBranchOwnershipCheckWith(root, id, endpoint, config, holderRoot)
	}
	o.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("not the person's terminal")
	}
	return o
}

func TestTheHolderCollectsUnderItsOwnAuthority(t *testing.T) {
	t.Parallel()
	b, id, _ := completedRound(t)
	o := collectionOwners(t, b)
	collectionOwnership(t, b, "another-lineage", true)
	if code, result := collectWithOwners(t, b, id, o); code == 0 || !strings.Contains(result.Summary, "may not start") {
		t.Fatalf("non-holder collected: %d %+v", code, result)
	}
	file := b.goalFile(b.id)
	collectionOwnership(t, b, file.Claimed.Lineage, true)
	code, result := collectWithOwners(t, b, id, o)
	if code != 0 || resultData(t, result)["state"] != "awaiting-judgement" || !slices.Equal(b.starter.launched(), []string{"build", "proof"}) {
		t.Fatalf("holder could not collect after refusal: %d %+v %v", code, result, b.starter.launched())
	}
}

func TestCollectItFromTheChannelCollectsTheRound(t *testing.T) {
	t.Parallel()
	command, _ := findIntentCommand("work wait")
	if flag, ok := command.lookupFlag("answer"); !ok || flag.value != "QUESTION" {
		t.Fatal("work wait has no answer flag")
	}
	for _, other := range []bool{false, true} {
		t.Run(map[bool]string{false: "chosen round", true: "another run"}[other], func(t *testing.T) {
			t.Parallel()
			b, id, launchID := completedRound(t)
			enrol(t, b, "Wido")
			channelFor(t, b.root())
			o := collectionOwners(t, b)
			collectionOwnership(t, b, b.goalFile(b.id).Claimed.Lineage, false)
			q, err := channel.Ask(channel.AskRequest{RepoRoot: b.root(), About: "machine", Kind: "other", Machine: "mac-cli", Contract: true, Now: b.manager.Now(), Facts: []string{"finished round"}, Decision: &channel.Decision{Kind: "uncollected-round", Subject: b.id, State: launchID}, Recommendation: "Collect it", Plain: &channel.Plain{Decision: "The round finished and waits.", IfNothing: "it waits."}, Options: []channel.Option{{Label: "Collect it", Act: []string{"metasystem", "work", "wait", "run:" + id, "--timeout", "1m"}, Consequence: "I collect the round. Undo: metasystem work stop on the goal."}, {Label: "Leave it", Consequence: "it waits."}}})
			if err != nil {
				t.Fatal(err)
			}
			q = recordChoice(t, b.root(), q, func(q *channel.Question) { q.Answer.Text, q.Answer.Lineage = "collect it", "steward" })
			if other {
				r, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(id)
				if err != nil {
					t.Fatal(err)
				}
				r.ID = "another-run"
				data, _ := json.Marshal(r)
				writeTestingFixtureFile(t, filepath.Join(b.unitRoot, r.ID, "run.json"), data, 0600)
				id = r.ID
			}
			if !other {
				if code, result := collectWithOwners(t, b, id, o); code == 0 {
					t.Fatalf("a process without the lease collected: %+v", result)
				}
			}
			code, result := collectWithOwners(t, b, id, o, "--answer", q.ID)
			state, _ := answerState(t, result)
			if other {
				if code == 0 || state != "refused" || !strings.Contains(result.Summary, "this answer authorizes") {
					t.Fatalf("another run admitted: %d %+v", code, result)
				}
			} else if code != 0 || state != "started" || resultData(t, result)["state"] != "awaiting-judgement" {
				t.Fatalf("channel did not collect: %d %+v", code, result)
			}
		})
	}
}

func witnessWorkWaitRepeat(t *testing.T) {
	b, id, _ := completedRound(t)
	o := b.workOwners()
	if code, result := collectWithOwners(t, b, id, o); code != 0 {
		t.Fatalf("first collection: %d %+v", code, result)
	}
	files := workIdemSnapshot(t, b.unitRoot)
	launches := b.starter.launched()
	if code, result := collectWithOwners(t, b, id, o); code != 0 || !slices.Equal(launches, b.starter.launched()) {
		t.Fatalf("repeat collection: %d %+v %v", code, result, b.starter.launched())
	}
	workIdemSameFiles(t, "work wait", files, workIdemSnapshot(t, b.unitRoot))
}
