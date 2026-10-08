package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

type landRebaseBed struct {
	*deliveryBed
	landing               *landingOwners
	owners                intentOwners
	lane                  string
	rebase                branch.RebaseResult
	calls, records, reads int
	registered            bool
	rebaseErr, recordErr  error
}

// The branch, worktree listing and ancestry are per-invocation facts; this
// bed never invokes Git, including during its setup.
func newLandRebaseBed(t *testing.T) *landRebaseBed {
	t.Helper()
	// Hand-in reads the registered lane and its trunk policy even when a
	// records line is already queued (lane-reads-its-policies, Decision 5).
	delivery, landing, install := plainLaneBedWith(t, true, "critic-root", "critic-root")
	b := &landRebaseBed{deliveryBed: delivery, landing: landing, lane: install, registered: true}
	b.deliveryBed.owners.landingGate = func(*intentInvocation, string, string) (string, error) { return "admitted", nil }
	b.deliveryBed.owners.recordLanded = func(*intentInvocation, string) error { return nil }
	b.deliveryBed.owners.laneLatest = func(install, id, main string) (plain.Entry, bool, error) {
		entry, ok, err := plain.Latest(install, id)
		if err == nil && ok {
			derived, deriveErr := plain.Landed([]plain.Entry{entry}, func(sha string) (bool, error) { return sha == main, nil })
			entry, err = derived[0], deriveErr
		}
		return entry, ok, err
	}
	b.deliveryBed.owners.branchState = func(string, string) (intentBranchState, error) {
		b.reads++
		return b.landing.status, nil
	}
	b.rebase = branch.RebaseResult{State: "rebased", OldTip: b.landing.status.BranchTip,
		NewTip: strings.Repeat("3", 40), MainTip: b.landing.status.EndpointTip, Carried: []string{"u1", "u2"}, NeedsReview: []string{}}
	b.owners = b.intentBed.owners()
	b.owners.delivery = b.deliveryBed.owners
	b.owners.work.git = func(_ string, args ...string) ([]byte, error) {
		if strings.Join(args, " ") == "rev-parse --verify -q refs/heads/goal/standing-validation" {
			return []byte(b.landing.status.BranchTip + "\n"), nil
		}
		if strings.Join(args, " ") != "worktree list --porcelain" {
			t.Fatalf("unexpected Git read: %v", args)
		}
		if !b.registered {
			return nil, nil
		}
		return []byte("worktree " + b.root() + "\nbranch refs/heads/goal/standing-validation\n"), nil
	}
	b.owners.connection = intentConnectionOwners{
		endpoint:    func(string) (goal.Endpoint, error) { return goal.Endpoint{Remote: "origin"}, nil },
		endpointTip: func(string, goal.Endpoint) (string, error) { return b.landing.status.EndpointTip, nil },
		claimCheck:  func(string, string, goal.Endpoint) func() error { return func() error { return nil } },
		section:     func(_ string, body func(func(func() error) error) error) error { return body(nil) },
		rebase: func(req branch.RebaseRequest) (branch.RebaseResult, error) {
			b.calls++
			if req.GoalID != "standing-validation" || req.EndpointTip != b.landing.status.EndpointTip || req.CheckClaim == nil {
				t.Fatalf("rebase request: %+v", req)
			}
			if b.rebaseErr == nil && b.rebase.State != "held" {
				b.landing.status.BranchTip = b.rebase.NewTip
			}
			return b.rebase, b.rebaseErr
		},
		recordRebase: func(_ *intentInvocation, id string, result branch.RebaseResult) error {
			b.records++
			if id != "standing-validation" || result.NewTip != b.landing.status.BranchTip {
				t.Fatalf("history before publication: %s %+v", id, result)
			}
			return b.recordErr
		},
	}
	b.laneInputs(&b.owners)
	return b
}

func (b *landRebaseBed) land(args ...string) (int, intentResult) {
	b.t.Helper()
	command, _ := findIntentCommand("work land")
	var captured intentResult
	command.run = func(inv *intentInvocation) int {
		if problem := inv.selectRoot(); problem != nil {
			return inv.render(*problem)
		}
		captured = inv.landGoal("standing-validation", inv.input.text("through"))
		return inv.render(captured)
	}
	var stdout, stderr bytes.Buffer
	argv := append([]string{"standing-validation", "--json"}, args...)
	code := runIntentIn(command, argv, &stdout, &stderr, b.root(), b.owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		b.t.Fatalf("no JSON result: %v stdout=%s stderr=%s", err, &stdout, &stderr)
	}
	result.text = captured.text
	return code, result
}

// Landing preserves the reviewed commits even when main has advanced.
func TestWorkLandKeepsBranchBehindMain(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"lane", "hand"} {
		t.Run(route, func(t *testing.T) {
			t.Parallel()
			b := newLandRebaseBed(t)
			tip := b.landing.status.BranchTip
			if route == "hand" {
				b.deliveryBed.owners.laneRoot = func(string, time.Time) (string, bool, error) { return "", false, nil }
			}
			code, result := b.runJSON(b.owners, "work", "land", "standing-validation", "--delivered", "Makes landing reliable")
			if code != 0 || result.Outcome != intentConfirmed || b.calls != 0 || b.records != 0 || b.reads != 1 || b.landing.status.BranchTip != tip {
				t.Fatalf("%s landing: %d %+v; rebases=%d records=%d reads=%d tip=%s", route, code, result, b.calls, b.records, b.reads, b.landing.status.BranchTip)
			}
			if route == "lane" {
				entries, err := plain.Entries(b.lane)
				if err != nil || len(entries) != 1 || entries[0].SHA != tip || len(b.landing.pushes) != 0 {
					t.Fatalf("hand-in changed the tip: %+v %v; pushes=%v", entries, err, b.landing.pushes)
				}
			} else if len(b.landing.pushes) != 1 || resultData(t, result)["subject"] != tip {
				t.Fatalf("hand landing did not replay the original tip: preps=%v pushes=%v", b.landing.preps, b.landing.pushes)
			}
			if _, ok := resultData(t, result)["rebase"]; ok {
				t.Fatalf("landing reported a rebase: %+v", result)
			}
		})
	}
}

func TestWorkLandReturnedConflictNamesRebase(t *testing.T) {
	t.Parallel()
	b := newLandRebaseBed(t)
	tip := b.landing.status.BranchTip
	line := plain.Line{Goal: "standing-validation", Branch: "goal/standing-validation", SHA: tip}
	if _, _, err := plain.HandIn(b.lane, line); err != nil {
		t.Fatal(err)
	}
	line.Outcome = plain.StateReturned
	line.Reason = "it does not merge with main (owned.go: both sides inserted at one place)."
	line.Conflict = &conflict.Return{Paths: []conflict.Path{{Path: "owned.go", Class: conflict.Builder}}}
	queue := filepath.Join(plain.Dir(b.lane), "queue.jsonl")
	before, err := os.ReadFile(queue)
	if err != nil {
		t.Fatal(err)
	}
	returned, err := json.Marshal(line)
	if err != nil {
		t.Fatal(err)
	}
	before = append(before, append(returned, '\n')...)
	b.writeFile(queue, string(before))
	code, stdout, stderr := b.run(b.owners, "work", "land", line.Goal, "--delivered", "Makes landing reliable")
	want := "goal standing-validation at " + plain.Short(tip) + " was returned: " + line.Reason + " → metasystem work rebase standing-validation"
	after, err := os.ReadFile(queue)
	if code != 1 || stdout != "" || oneSpaced(stderr) != "✗ "+want || err != nil || string(after) != string(before) || b.calls != 0 || b.records != 0 {
		t.Fatalf("returned conflict: %d stdout=%q stderr=%q queue=%q err=%v rebases=%d records=%d", code, stdout, stderr, after, err, b.calls, b.records)
	}
}

func TestWorkRebaseRecordsItsReasonAfterPublishing(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"rebased", "carried", "pushed"} {
		t.Run(state, func(t *testing.T) {
			b, owners, _ := rebaseIntentBed(t)
			b.lineage = b.goalFile(b.id).Claimed.Lineage
			owners.connection.recordRebase = func(inv *intentInvocation, id string, result branch.RebaseResult) error {
				err := productionRecordRebase(inv, id, result)
				if err != nil {
					t.Fatalf("record: %v", err)
				}
				return err
			}
			rebase := branch.RebaseResult{State: state, OldTip: strings.Repeat("1", 40), NewTip: strings.Repeat("2", 40), MainTip: strings.Repeat("3", 40), Carried: []string{"u1"}, NeedsReview: []string{"u2"}, Regenerated: []string{"gen/out"}}
			owners.connection.rebase = func(branch.RebaseRequest) (branch.RebaseResult, error) { return rebase, nil }
			code, stdout, stderr := b.run(owners, "work", "rebase", b.id)
			if code != 0 || stderr != "" || strings.Contains(stdout, "history line was not written") {
				t.Fatalf("rebase: %d %q %q", code, stdout, stderr)
			}
			file := b.goalFile(b.id)
			line := file.History[len(file.History)-1]
			reason := "on main 333333333333"
			if state == "rebased" {
				reason = "rebased 111111111111 onto main 333333333333"
			}
			reason += "; reviews carried: u1; needs review: u2; regenerated: gen/out"
			if line.Verb != "rebase" || line.Reason != reason {
				t.Fatalf("history: %+v", line)
			}
		})
	}
}

// TestWorkLandUnreadableQueueNamesGoal: when the lane's queue cannot be read,
// work land fails, names the goal and hands nothing in.
func TestWorkLandUnreadableQueueNamesGoal(t *testing.T) {
	t.Parallel()
	b := newLandRebaseBed(t)
	b.deliveryBed.owners.laneLatest = func(string, string, string) (plain.Entry, bool, error) {
		return plain.Entry{}, false, errors.New("queue unavailable")
	}
	code, result := b.land()
	entries, _ := plain.Entries(b.lane)
	if code == 0 || b.calls != 0 || len(entries) != 0 || result.Next == nil || len(result.Targets) != 1 || result.Targets[0] != (intentTarget{Kind: "goal", ID: "standing-validation"}) {
		t.Fatalf("unreadable queue: %d %+v calls=%d queue=%+v", code, result, b.calls, entries)
	}
}
