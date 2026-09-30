package goal

import (
	"errors"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// fencedflake (2026-09-30): the steward runner died after MarkPushed, and its
// pushed entry refused every later publish on the landing clone — the batch
// owner's handover included — because nothing on that path ever ran
// recovery. A publish blocked by a pushed entry whose owner is provably dead
// now runs the recovery rule for that entry (the opid on a fresh capture
// decides; the dead owner's work is completed from its intent, never pushed
// blindly) and retries once.
func TestAHandoverRecoversADeadOwnersPushedBlocker(t *testing.T) {
	t.Parallel()
	endpoint, req := handoverBed(t, "handed", false)
	blocker := Opid("01J5X00000000000000000DB01", "mac-a", "lin-1")
	strandEntry(t, endpoint.Root, blocker, PhasePushed, Intent{
		Verb: "open", Targets: []string{"orphaned"},
		Args: map[string]string{"intent": "The dead owner's work.", "origin": "main", "next": "Continue.", "labels": ""},
	})
	req.Ulid, req.Now = "01J5X00000000000000000HB02", req.Now.Add(1)
	result, err := Handover(req, "handed", "landing", "landing-lineage", 11, "batch-a", func() (identity.Liveness, error) { return identity.Alive, nil })
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("the handover met the dead owner's pushed entry: %+v %v", result, err)
	}
	entry, err := ReadEntry(endpoint.Root, blocker)
	if err != nil || entry.Phase != PhaseTerminal || entry.Outcome != OutcomeConfirmed {
		t.Fatalf("the blocker was not recovered: %+v %v", entry, err)
	}
	tree, _ := loadTreeFor(endpoint, acceptedTipForEndpoint(t, endpoint))
	if tree.Live["orphaned"] == nil || tree.Live["orphaned"].History[0].Opid != blocker {
		t.Fatal("recovery did not complete the dead owner's open under its own opid")
	}
	if tree.Live["handed"].Claimed.Machine != "landing" {
		t.Fatalf("the handover did not land after recovery: %+v", tree.Live["handed"].Claimed)
	}
}

// Recovery by a publish is for a PROVABLY dead owner only: a live owner's
// pushed entry and an owner whose liveness cannot be proved refuse as they
// always did, with the entry untouched (fail closed).
func TestAPublishLeavesALiveOrUnprovenOwnersPushedBlockerAlone(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"live", "unproven"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			endpoint, _ := fakeGoalEndpoint(t)
			blocker := Opid("01J5X00000000000000000DB10", "mac-a", "lin-1")
			if _, err := CreateEntry(endpoint.Root, blocker, "mac-a", "lin-1", Intent{Verb: "open", Targets: []string{"held-back"},
				Args: map[string]string{"intent": "Held.", "origin": "main", "next": "Wait.", "labels": ""}}); err != nil {
				t.Fatal(err)
			}
			if err := MarkPushed(endpoint.Root, blocker, "sometip", 1, timeNowUTC()); err != nil {
				t.Fatal(err)
			}
			if name == "live" {
				spawnForeignOwner(t, endpoint.Root, blocker)
			} else {
				entry, err := ReadEntry(endpoint.Root, blocker)
				if err != nil {
					t.Fatal(err)
				}
				entry.Owner = OwnerIdentity{}
				if err := writeEntry(endpoint.Root, entry); err != nil {
					t.Fatal(err)
				}
			}
			_, err := Open(verbReqFor(endpoint, "01J5X00000000000000000DB11", "mac-a"), "blocked-out", "Waits.", "main", "Go.")
			if err == nil || !strings.Contains(err.Error(), "an earlier goal change ("+blocker+") was pushed, but whether it took effect is unknown") {
				t.Fatalf("a %s owner's pushed entry did not refuse: %v", name, err)
			}
			if entry, readErr := ReadEntry(endpoint.Root, blocker); readErr != nil || entry.Phase != PhasePushed {
				t.Fatalf("the %s owner's entry was touched: %+v %v", name, entry, readErr)
			}
		})
	}
}

// A dead owner's breach-stop needs live budget authority to be recovered
// (recover.go completeFromIntent). A publish with no recovery policy bound
// refuses and names the path that has it; with the policy bound it runs
// recovery through that policy.
func TestADeadOwnersBreachStopRecoversOnlyThroughTheBoundPolicy(t *testing.T) {
	t.Parallel()
	endpoint, _ := fakeGoalEndpoint(t)
	blocker := Opid("01J5X00000000000000000DB20", "mac-a", "goal-stop-custodian")
	if _, err := CreateEntry(endpoint.Root, blocker, "mac-a", "goal-stop-custodian", Intent{Verb: "breach-stop", Targets: []string{"goal-c"}}); err != nil {
		t.Fatal(err)
	}
	if err := MarkPushed(endpoint.Root, blocker, "sometip", 1, timeNowUTC()); err != nil {
		t.Fatal(err)
	}
	cmd := spawnForeignOwner(t, endpoint.Root, blocker)
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()

	_, err := Open(verbReqFor(endpoint, "01J5X00000000000000000DB21", "mac-a"), "unbound", "Waits.", "main", "Go.")
	if err == nil || !strings.Contains(err.Error(), "whether it took effect is unknown") || !strings.Contains(err.Error(), "\nrun: metasystem goal sync --recover") {
		t.Fatalf("an unbound publish did not refuse the breach-stop by name: %v", err)
	}
	if entry, readErr := ReadEntry(endpoint.Root, blocker); readErr != nil || entry.Phase != PhasePushed {
		t.Fatalf("the unbound publish touched the breach-stop: %+v %v", entry, readErr)
	}

	policy := &recordingBreachPolicy{}
	bound := endpoint
	bound.ConfigureBlockedRecovery(policy)
	if res, err := Open(verbReqFor(bound, "01J5X00000000000000000DB22", "mac-a"), "bound", "Flows.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("the bound publish did not recover and proceed: %+v %v", res, err)
	}
	if strings.Join(policy.targets, ",") != "goal-c" {
		t.Fatalf("the bound policy was not asked for the breach-stop: %v", policy.targets)
	}
	if entry, readErr := ReadEntry(endpoint.Root, blocker); readErr != nil || entry.Phase != PhaseTerminal {
		t.Fatalf("the breach-stop was not classified: %+v %v", entry, readErr)
	}
}

type recordingBreachPolicy struct{ targets []string }

func (p *recordingBreachPolicy) BreachStop(_ Endpoint, entry Entry) (PublishRequest, func(), error) {
	p.targets = append(p.targets, entry.Intent.Targets...)
	return PublishRequest{}, nil, errors.New("goal goal-c revision 3 is not over its live budget")
}
