package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

// claimQuotaCheckout extends the seat tick checkout's accepted ledger with a
// goal this machine (m1) claims and a ready priority-1 goal behind it.
func claimQuotaCheckout(t *testing.T) (top, install string) {
	t.Helper()
	top, install = seatTickCheckout(t)
	budget := goal.Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 2}
	approved := func(id, intent string) *goal.GoalFile {
		opid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAW", "m1", "approval-"+id)
		return &goal.GoalFile{Id: id, State: goal.StateApproved, Tier: 3, Intent: intent, Origin: goal.OriginMain,
			NextStep: "Build it.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2, Budget: &budget,
			History: []goal.HistoryLine{
				{At: "2026-08-23T00:00:00Z", Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAV", "m1", "coordinator-"+id), Verb: "open",
					Actor: "m1+coordinator", Targets: []string{id}, Keep: -1},
				{At: "2026-08-23T00:01:00Z", Opid: opid, Verb: "approve", Actor: "human:Wido", Targets: []string{id}, Keep: -1},
			},
			Approved: &goal.ApprovalRecord{By: "human:Wido", At: "2026-08-23T00:01:00Z", Revision: 2, Opid: opid,
				Authority: goal.ApprovalAuthorityProven, Digest: goal.ApprovalDigest(intent, 3, budget)}}
	}
	held := approved("held-h", "Hold the claim")
	held.State, held.Revision = goal.StateClaimed, 3
	held.Claimed = &goal.ClaimRecord{Machine: "m1", Lineage: "coordinator", At: "2026-08-23T01:00:00Z", Revision: 3, AccountingRevision: 3}
	held.StopCapability = &goal.StopCapability{Generation: 3, Revision: 3, Machine: "m1", ClaimEpoch: 1}
	held.History = append(held.History, goal.HistoryLine{At: "2026-08-23T01:00:00Z",
		Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC1", "m1", "coordinator"), Verb: "claim",
		Actor: "m1+coordinator", Targets: []string{"held-h"}, Keep: -1})
	ready := approved("ready-r", "Fix the phone")
	ready.Priority, ready.Sequence, ready.Pinned = 1, 1, "m1"
	for _, file := range []*goal.GoalFile{held, ready} {
		if err := os.WriteFile(filepath.Join(install, "plans", "goals", file.Id+".md"), goal.RenderFile(file), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The template signal makes the installation its own state root, as on
	// this repository's seats, so the claim's root selection finds the ledger.
	conf, err := os.ReadFile(filepath.Join(install, "metasystem.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), append(conf, "metasystem.template=true\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	seatTickGit(t, top, "add", "-A")
	seatTickGit(t, top, "commit", "-qm", "claim quota")
	seatTickGit(t, top, "update-ref", goal.AcceptedRef, "HEAD")
	return top, install
}

func claimQuotaVerdict(t *testing.T, result steward.TickResult) steward.RoleVerdict {
	t.Helper()
	for _, role := range result.Health.Roles {
		if role.Role == steward.RoleClaimQuota {
			return role
		}
	}
	t.Fatalf("the tick's health has no claim-quota role: %+v", result.Health.Roles)
	return steward.RoleVerdict{}
}

func TestTheStewardTickReadsTheClaimQuotaThroughTheLaneReader(t *testing.T) {
	t.Parallel()
	top, install := claimQuotaCheckout(t)
	lane := newResolveVerbFixture(t)
	owners := defaultIntentOwners()
	delivery := defaultIntentDeliveryOwners()
	delivery.laneRoot = func(string, time.Time) (string, bool, error) { return lane.root, true, nil }
	delivery.laneInstall = func(string) (string, error) { return lane.install, nil }
	delivery.laneLatest = func(laneInstall, id, main string) (plain.Entry, bool, error) {
		if laneInstall != lane.install || main == "" {
			t.Errorf("lane read install=%q main=%q", laneInstall, main)
		}
		return plain.Latest(laneInstall, id)
	}
	delivery.laneContains = func(sha, main string) (bool, error) {
		if main == "" {
			t.Errorf("lane containment read sha=%q main=%q", sha, main)
		}
		return false, nil
	}
	owners.delivery = delivery

	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	config := steward.TickConfig{Now: now, WorkStateRoot: t.TempDir(), ProviderHome: testprovider.Register(t, install)}
	wireStewardSeat(&config, owners)
	// The same checkout registered as the host's landing lane starts no seat.
	asLane := newStewardSeatLauncher()
	asLane.laneRoot = func() (string, bool, error) { return top, true, nil }
	config.Seat = asLane

	result, err := steward.RunTick(install, config, seatTickCensus{})
	if err != nil {
		t.Fatal(err)
	}
	if role := claimQuotaVerdict(t, result); role.Status != steward.HealthDead ||
		!strings.Contains(role.Reason, "ready-r (priority 1) is ready but held-h holds this machine's one claim") ||
		!strings.Contains(role.Reason, "next: metasystem goal release held-h --reason TEXT, then metasystem goal claim ready-r") {
		t.Fatalf("with no hand-in the row names the ready goal and the claim: %+v", role)
	}

	queueClaimGoal(t, lane.install, "held-h")
	config.Now = now.Add(10 * time.Minute)
	result, err = steward.RunTick(install, config, seatTickCensus{})
	if err != nil {
		t.Fatal(err)
	}
	if role := claimQuotaVerdict(t, result); role.Status != steward.HealthAlive {
		t.Fatalf("a waiting hand-in frees the claim through the lane reader: %+v", role)
	}
}
