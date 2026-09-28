package steward

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
)

var helmFixtureClock = time.Date(2026, 9, 28, 19, 14, 3, 0, time.UTC)

// takeHelmFixture makes root a seat (a .git directory) and writes the helm
// signature into its common dir, as helm take does.
func takeHelmFixture(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := helm.Write(root, helm.Record{By: "Wido", At: helmFixtureClock.Format(time.RFC3339), Reason: "fixture"}); err != nil {
		t.Fatal(err)
	}
}

type countingCensus struct{ calls *atomic.Int32 }

func (c countingCensus) Workers(string) (Workers, error) {
	c.calls.Add(1)
	return Workers{Live: 1, CensusComplete: true}, nil
}

func helmTickUnderTest(t *testing.T, now time.Time) {
	t.Helper()
	root := t.TempDir()
	writeLedger(t, root, "# Goals\n\n## Current goal: fix-it — Repair the thing\n- Origin: main\n- Next step: Repair it.\n")
	takeHelmFixture(t, root)
	var census, scans, stops atomic.Int32
	cfg := TickConfig{Now: now,
		BreachStopReady: func() bool { scans.Add(1); return true },
		BreachStop:      func(string, uint64) (string, error) { stops.Add(1); return "", nil },
	}
	result, err := RunTick(root, cfg, countingCensus{&census})
	if err != nil {
		t.Fatalf("HM-7: a tick under the helm failed: %v", err)
	}
	if result.Decision.Verdict != VerdictHelm || result.Decision.Action != ActNone {
		t.Fatalf("HM-7: the tick under the helm decided %+v; want the HELM verdict and no action", result.Decision)
	}
	if scans.Load() != 0 || stops.Load() != 0 || census.Load() != 0 {
		t.Fatalf("HM-7: the tick under the helm ran the breach stop (%d scans, %d stops) or the decider's census (%d)",
			scans.Load(), stops.Load(), census.Load())
	}
	tickRecord, err := os.ReadFile(ComponentEvidencePath(root, "steward-tick"))
	if err != nil || !strings.Contains(string(tickRecord), `"HELM"`) {
		t.Fatalf("HM-7: the tick attempt did not complete as HELM: %v\n%s", err, tickRecord)
	}
	// Presence is still published: the component ran (with no runner context
	// it records the manual-tick skip) and the result carries its report.
	if _, err := os.Stat(ComponentEvidencePath(root, "seat-presence")); err != nil || result.SeatPresence.Outcome != seat.OutcomeSkipped {
		t.Fatalf("HM-7: the tick under the helm did not run seat presence: %v %+v", err, result.SeatPresence)
	}
	// No reap, ledger attention, evidence aging, narration, health or notification.
	for _, component := range []string{"ledger-attention", "narrator"} {
		if _, err := os.Stat(ComponentEvidencePath(root, component)); !os.IsNotExist(err) {
			t.Fatalf("HM-7: the tick under the helm ran %s: %v", component, err)
		}
	}
	if _, err := os.Stat(EvidencePath(root)); !os.IsNotExist(err) {
		t.Fatalf("HM-7: the tick under the helm wrote the evidence store: %v", err)
	}
	if pending, err := PendingNotifications(root); err != nil || len(pending) != 0 {
		t.Fatalf("HM-7: the tick under the helm queued notifications: %+v %v", pending, err)
	}
}

func TestTickUnderHelmRecordsHelmAndActsNot(t *testing.T) {
	t.Parallel()
	helmTickUnderTest(t, helmFixtureClock.Add(time.Minute))
}

func TestTickAtPlusThirtyDaysStillHelm(t *testing.T) {
	t.Parallel()
	// HM-10: no clock ends the helm.
	helmTickUnderTest(t, helmFixtureClock.Add(30*24*time.Hour))
}

func TestRevivalUnderHelmHoldsEveryKind(t *testing.T) {
	check := func(t *testing.T, root string, nonce string, outcome ReviveOutcome, err error) {
		t.Helper()
		if err != nil || !outcome.Held || outcome.Launched || outcome.Reason != "human at the helm" {
			t.Fatalf("HM-7: the revival under the helm was not held quietly: %+v %v", outcome, err)
		}
		if pending, err := PendingNotifications(root); err != nil || len(pending) != 0 {
			t.Fatalf("HM-7: the held revival queued a notification: %+v %v", pending, err)
		}
		if resumable, ok, err := ResumableIntent(root); err != nil || !ok || resumable != nonce {
			t.Fatalf("HM-7: the held intent is no longer resumable: %q %t %v", resumable, ok, err)
		}
	}
	t.Run("HM-7 seatIdle", func(t *testing.T) {
		revival := newRevivalFixture(t, 0, 0)
		intent := testIntent("seat-idle-helm")
		intent.Reason = "seatIdle"
		intent.ClaimNeeded = true
		if err := PrepareIntent(revival.root, filepath.Join(revival.root, "memory", "receipts.log"), intent); err != nil {
			t.Fatal(err)
		}
		takeHelmFixture(t, revival.root)
		outcome, err := revival.complete(TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error {
			t.Fatal("HM-7: a revival launched under the helm")
			return nil
		}, func(Intent) error {
			t.Fatal("HM-7: a seat-idle claim was made under the helm")
			return nil
		})
		check(t, revival.root, intent.Nonce, outcome, err)
	})
	t.Run("HM-7 seatHandoff", func(t *testing.T) {
		root, intent := prepareRevivalHandoff(t, "4000000000000077")
		takeHelmFixture(t, root)
		outcome, err := completeHandoffRevival(root, TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error {
			t.Fatal("HM-7: a handoff launched under the helm")
			return nil
		})
		check(t, root, intent.Nonce, outcome, err)
	})
}
