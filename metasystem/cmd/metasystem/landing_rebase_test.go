package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	b := &landRebaseBed{deliveryBed: newDeliveryBedWith(t, nil), registered: true}
	b.landing = &landingOwners{status: readBranch(2, "critic-root", "critic-root")}
	b.landing.install(b.deliveryBed)
	b.lane = filepath.Join(t.TempDir(), "lane")
	b.deliveryBed.owners.laneRoot = func(string, time.Time) (string, bool, error) { return "/lane", true, nil }
	b.deliveryBed.owners.laneInstall = func(string) (string, error) { return b.lane, nil }
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

func TestWorkLandRebasesBeforeHandIn(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"rebased", "held", "carried", "pushed"} {
		t.Run(state, func(t *testing.T) {
			b := newLandRebaseBed(t)
			b.rebase.State = state
			if state == "held" {
				b.rebase.NewTip = b.rebase.OldTip
			}
			code, result := b.land()
			entries, err := plain.Entries(b.lane)
			wantRecords, wantReads, line := 1, 2, "on main "
			if state == "held" {
				wantRecords, wantReads = 0, 1
			}
			if state == "rebased" {
				line = "rebased onto main "
			}
			if code != 0 || result.Outcome != intentConfirmed || b.calls != 1 || b.records != wantRecords || b.reads != wantReads || err != nil || len(entries) != 1 || entries[0].SHA != b.rebase.NewTip {
				t.Fatalf("hand-in: %d %+v; calls=%d records=%d reads=%d entries=%+v %v", code, result, b.calls, b.records, b.reads, entries, err)
			}
			if !strings.Contains(result.Summary, "handed to the lane, "+line+shortCommit(b.rebase.MainTip)) {
				t.Fatalf("rebase output: %+v", result)
			}
			if data := result.Data.(map[string]any)["rebase"].(map[string]any); data["state"] != state || data["newTip"] != b.rebase.NewTip {
				t.Fatalf("rebase JSON: %+v", data)
			}
		})
	}
}

func TestWorkLandSkipsBoundCommitsAndMissingWorktree(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"through", "waiting", "waiting elsewhere", "landed", "human", "no worktree"} {
		t.Run(kind, func(t *testing.T) {
			b := newLandRebaseBed(t)
			args, reason := []string{}, ""
			switch kind {
			case "through":
				args = []string{"--through", b.landing.status.Status.Units[0].Commit}
				reason = "--through names a commit"
			case "waiting", "waiting elsewhere", "landed":
				sha := b.landing.status.BranchTip
				if kind == "waiting elsewhere" {
					sha = strings.Repeat("9", 40)
				}
				_, _, err := plain.HandIn(b.lane, plain.Line{Goal: "standing-validation", Branch: "goal/standing-validation", SHA: sha, Seat: "seat", At: time.Now().UTC().Format(time.RFC3339)})
				if err != nil {
					t.Fatal(err)
				}
				reason = "its hand-in at " + plain.Short(sha) + " still waits in the lane"
				if kind == "landed" {
					b.landing.status.EndpointTip = sha
					reason = "its hand-in at " + plain.Short(sha) + " already landed on main"
				}
			case "human":
				file := b.goalFile("standing-validation")
				file.Tier = 3
				file.History = append(file.History, goal.HistoryLine{At: "2026-09-01T12:00:00Z", Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FB1", "mac-cli", "m1"),
					Verb: goal.LandWithoutSittingVerb, Actor: "human:Wido", Targets: []string{file.Id}, Keep: -1,
					Reason: "landed-without-sitting tip=" + b.landing.status.BranchTip + " by=Wido because=ready"})
				file.Revision++
				b.addGoal(file)
				reason = "a person's word stands at this tip"
			case "no worktree":
				b.registered = false
				reason = "the goal has no worktree"
			}
			code, result := b.land(args...)
			if code != 0 || b.calls != 0 || b.records != 0 || !strings.Contains(strings.Join(result.text, "\n"), "not rebased: "+reason) {
				t.Fatalf("skip: %d %+v calls=%d records=%d", code, result, b.calls, b.records)
			}
			if data := result.Data.(map[string]any)["rebase"].(map[string]any); data["state"] != "skipped" || data["reason"] != reason {
				t.Fatalf("skip JSON: %+v", data)
			}
		})
	}
}

func TestWorkLandRebaseSkipReadFailuresNameGoal(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"queue", "worktree", "settings"} {
		t.Run(kind, func(t *testing.T) {
			b := newLandRebaseBed(t)
			switch kind {
			case "queue":
				b.deliveryBed.owners.laneLatest = func(string, string, string) (plain.Entry, bool, error) {
					return plain.Entry{}, false, errors.New("queue unavailable")
				}
			case "worktree":
				b.owners.work.git = func(string, ...string) ([]byte, error) { return nil, errors.New("worktree unavailable") }
			case "settings":
				writeTestingFixtureFile(t, filepath.Join(b.install, "metasystem.conf.local"), []byte("landing.review.human-from-tier = two\n"), 0o644)
			}
			code, result := b.land()
			entries, _ := plain.Entries(b.lane)
			if code == 0 || b.calls != 0 || len(entries) != 0 || result.Next == nil || len(result.Targets) != 1 || result.Targets[0] != (intentTarget{Kind: "goal", ID: "standing-validation"}) {
				t.Fatalf("%s failure: %d %+v calls=%d queue=%+v", kind, code, result, b.calls, entries)
			}
		})
	}
}

func TestWorkLandRebaseFailuresHandNothingIn(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"conflict", "needs review"} {
		t.Run(kind, func(t *testing.T) {
			b := newLandRebaseBed(t)
			switch kind {
			case "conflict":
				b.rebaseErr = &branch.OpError{Code: branch.RebaseConflictCode, Message: "rebase stopped at build u1; nothing was changed\npaths:\nowned.go\nrun: metasystem work status standing-validation"}
			case "needs review":
				b.rebase.NeedsReview = []string{"u2"}
				b.landing.status = readBranch(1, "critic-root")
			}
			code, result := b.land()
			entries, _ := plain.Entries(b.lane)
			if code == 0 || len(entries) != 0 || result.Next == nil {
				t.Fatalf("refusal: %d %+v queue=%+v", code, result, entries)
			}
			switch kind {
			case "conflict":
				if result.Data.(map[string]any)["code"] != branch.RebaseConflictCode || !strings.Contains(strings.Join(result.text, "\n"), "owned.go") || b.records != 0 {
					t.Fatalf("conflict: %+v", result)
				}
			case "needs review":
				if strings.Join(result.Next.Argv, " ") != "metasystem work review standing-validation --work u2" {
					t.Fatalf("review command: %+v", result)
				}
			}
		})
	}
}

func TestWorkLandRebaseHistoryFailureKeepsHandIn(t *testing.T) {
	t.Parallel()
	b := newLandRebaseBed(t)
	b.recordErr = errors.New("history unavailable")
	code, result := b.land()
	entries, _ := plain.Entries(b.lane)
	if code != 0 || len(entries) != 1 || b.records != 1 || !strings.Contains(strings.Join(result.text, "\n"), "history line was not written; run: metasystem goal sync") {
		t.Fatalf("history failure: %d %+v queue=%+v", code, result, entries)
	}
}

func TestWorkLandRebaseHandsInAfterItsHistoryMovesMain(t *testing.T) {
	t.Parallel()
	b := newLandRebaseBed(t)
	record := b.owners.connection.recordRebase
	b.owners.connection.recordRebase = func(inv *intentInvocation, id string, result branch.RebaseResult) error {
		if err := record(inv, id, result); err != nil {
			return err
		}
		b.landing.status.EndpointTip = strings.Repeat("4", 40)
		return nil
	}
	code, result := b.land()
	entries, err := plain.Entries(b.lane)
	if code != 0 || result.Outcome != intentConfirmed || b.records != 1 || b.reads != 2 || err != nil || len(entries) != 1 || entries[0].SHA != b.rebase.NewTip {
		t.Fatalf("history advanced main: %d %+v queue=%+v %v", code, result, entries, err)
	}
}

func TestWorkLandByHandRebasesFirst(t *testing.T) {
	t.Parallel()
	b := newLandRebaseBed(t)
	b.deliveryBed.owners.laneRoot = func(string, time.Time) (string, bool, error) { return "", false, nil }
	code, result := b.land()
	if code != 0 || b.calls != 1 || len(b.landing.pushes) != 1 || b.reads != 2 || len(result.text) == 0 || !strings.Contains(result.text[0], "rebased onto main") {
		t.Fatalf("hand landing: %d %+v calls=%d pushes=%v", code, result, b.calls, b.landing.pushes)
	}
}

func TestWorkLandRebasePlainOutput(t *testing.T) {
	t.Parallel()
	b := newLandRebaseBed(t)
	code, stdout, stderr := b.run(b.owners, "work", "land", "standing-validation", "--delivered", "the change is ready")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "handed to the lane") || !strings.Contains(stdout, "rebased onto main "+shortCommit(b.rebase.MainTip)) || !strings.Contains(stdout, "review carried: u1") {
		t.Fatalf("plain output: %d stdout=%q stderr=%q", code, stdout, stderr)
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
			rebase := branch.RebaseResult{State: state, OldTip: strings.Repeat("1", 40), NewTip: strings.Repeat("2", 40), MainTip: strings.Repeat("3", 40), Carried: []string{"u1"}, NeedsReview: []string{"u2"}}
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
			reason += "; reviews carried: u1; needs review: u2"
			if line.Verb != "rebase" || line.Reason != reason {
				t.Fatalf("history: %+v", line)
			}
		})
	}
}
