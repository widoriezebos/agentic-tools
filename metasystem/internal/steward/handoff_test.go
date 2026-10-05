package steward

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func handoffLegacyDependencies(prober identity.Prober) openWorkDependencies {
	return openWorkDependencies{
		HandoffProber: prober,
		NewWorld:      func(string) bool { return false },
		ReadClaimableBudgetedWork: func(root string, _ time.Time) (goal.ClaimableBudgetedWork, error) {
			return goal.ReadLegacyClaimableWork(root, identity.KernelProber{})
		},
	}
}

func completeHandoffRevival(root string, prober identity.Prober, cfg TickConfig, census WorkerCensus, nonce string, launch LaunchSeam, admissions ...AdmitSeam) (ReviveOutcome, error) {
	var admit AdmitSeam
	if len(admissions) > 0 {
		admit = admissions[0]
	}
	return completeRevivalWithDependencies(root, cfg, census, nonce, launch, admit, handoffLegacyDependencies(prober))
}

func TestARefusedContinuationBriefHoldsTheHandoff(t *testing.T) {
	t.Parallel()
	root, intent := prepareRevivalHandoff(t, "500000000000000d")
	prober := handoffProbe(*intent.Handoff, identity.Dead, false, nil)
	path := "artifacts/agents/handoff/state.json"
	refusal := &dispatch.BriefAuthorityRefusal{MissingPaths: []string{path}, Details: map[string]string{path: "runtime path; looked in work trees dispatch and primary"}}
	launches, admissions := 0, 0
	launch := func(Intent) error { launches++; return nil }
	admit := func(it Intent) error {
		admissions++
		if it.Nonce != intent.Nonce {
			t.Fatalf("admission checked another handoff: %+v", it)
		}
		return refusal
	}
	for tick := 0; tick < 2; tick++ {
		held, err := completeHandoffRevival(root, prober, TickConfig{}, deadCensus(), intent.Nonce, launch, admit)
		if err != nil || !held.Held || held.Launched || held.Escalate || launches != 0 {
			t.Fatalf("refused admission did not hold: %+v %v launches=%d", held, err, launches)
		}
		if live, err := LiveIntents(root); err != nil || len(live) != 1 || live[0].Nonce != intent.Nonce {
			t.Fatalf("refused admission consumed the handoff: %+v %v", live, err)
		}
		if ev, err := LoadEvidence(EvidencePath(root)); err != nil || ev.DryRevivals != 0 {
			t.Fatalf("refused admission counted a revival: %+v %v", ev, err)
		}
	}
	if pending, err := PendingNotifications(root); err != nil || len(pending) != 1 || !strings.Contains(pending[0].Message, refusal.Error()) {
		t.Fatalf("hold notice lost the path or its class: %+v %v", pending, err)
	}
	passed, err := completeHandoffRevival(root, prober, TickConfig{}, deadCensus(), intent.Nonce, launch, func(it Intent) error { admissions++; return nil })
	if err != nil || !passed.Launched || launches != 1 || admissions != 3 {
		t.Fatalf("a later admitted tick did not launch: %+v %v launches=%d admissions=%d", passed, err, launches, admissions)
	}
	if live, err := LiveIntents(root); err != nil || len(live) != 0 {
		t.Fatalf("admitted launch left a live intent: %+v %v", live, err)
	}
	if ev, err := LoadEvidence(EvidencePath(root)); err != nil || ev.DryRevivals != 1 {
		t.Fatalf("admitted launch must count once: %+v %v", ev, err)
	}
	if pending, err := PendingNotifications(root); err != nil || len(pending) != 0 {
		t.Fatalf("admitted launch left a hold notice: %+v %v", pending, err)
	}
}

func TestHandoffDefaultHasNoExpiry(t *testing.T) {
	recordedAt := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
	if handoffExpired(recordedAt, now) {
		t.Fatal("the unanswered expiry policy must keep an otherwise admitted handoff without a time limit")
	}

	root, intent := stagedRevivalHandoff(t, "5000000000000001")
	prober := handoffProbe(*intent.Handoff, identity.Dead, false, nil)
	decision, _, err := decideForHandoffWithReader(root, TickConfig{}.withDefaults(), Workers{CensusComplete: true}, Evidence{}, intent, 0, false, "owned", now, handoffLegacyDependencies(prober).ReadClaimableBudgetedWork, prober)
	if err != nil || decision.Action != ActRevive {
		t.Fatalf("age alone must not change an admitted handoff: %+v %v", decision, err)
	}
}

func TestHandoffAdmissionUsesOneExpiryRule(t *testing.T) {
	root, intent := stagedRevivalHandoff(t, "5000000000000002")
	prober := handoffProbe(*intent.Handoff, identity.Alive, true, nil)
	now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	called := 0
	previous := handoffExpiryRule
	handoffExpiryRule = func(recordedAt, observedAt time.Time) bool {
		called++
		if !recordedAt.Equal(intent.Handoff.RecordedAt) || !observedAt.Equal(now) {
			t.Fatalf("expiry did not receive the binding and caller's one clock sample: recorded=%s now=%s", recordedAt, observedAt)
		}
		return true
	}
	t.Cleanup(func() { handoffExpiryRule = previous })

	decision, _, err := decideForHandoffWithReader(root, TickConfig{}.withDefaults(), Workers{CensusComplete: true}, Evidence{}, intent, 0, false, "owned", now, handoffLegacyDependencies(prober).ReadClaimableBudgetedWork, prober)
	if err != nil || called != 1 || decision.Action != ActNotify {
		t.Fatalf("expiry must be consulted exactly once before predecessor liveness: called=%d decision=%+v err=%v", called, decision, err)
	}
}

func TestHandoffRevivalPassesItsOneClockSampleToAdmission(t *testing.T) {
	root, intent := stagedRevivalHandoff(t, "5000000000000005")
	prober := handoffProbe(*intent.Handoff, identity.Alive, true, nil)
	fixed := time.Date(2031, 2, 3, 4, 5, 6, 0, time.UTC)

	previousNow := revivalNow
	clockCalls := 0
	revivalNow = func() time.Time {
		clockCalls++
		return fixed
	}
	t.Cleanup(func() { revivalNow = previousNow })

	previousExpiry := handoffExpiryRule
	expiryCalls := 0
	handoffExpiryRule = func(_ time.Time, observedAt time.Time) bool {
		expiryCalls++
		if !observedAt.Equal(fixed) {
			t.Fatalf("handoff admission resampled its clock instead of using the revival observation: %s", observedAt)
		}
		return true
	}
	t.Cleanup(func() { handoffExpiryRule = previousExpiry })

	decision, _, err := decideForRevivalWithDependencies(root, TickConfig{}, deadCensus(), Evidence{}, intent, handoffLegacyDependencies(prober))
	if err != nil || decision.Action != ActNotify || clockCalls != 1 || expiryCalls != 1 {
		t.Fatalf("revival must pass one clock observation into admission: decision=%+v err=%v clock=%d expiry=%d", decision, err, clockCalls, expiryCalls)
	}
}

func TestHandoffBindingValidationPrecedesExpiry(t *testing.T) {
	root, intent := stagedRevivalHandoff(t, "5000000000000006")
	intent.Handoff.StateDigest = "invalid"
	previous := handoffExpiryRule
	expiryCalls := 0
	handoffExpiryRule = func(_, _ time.Time) bool {
		expiryCalls++
		return true
	}
	t.Cleanup(func() { handoffExpiryRule = previous })

	decision, _, err := decideForHandoffWithReader(root, TickConfig{}.withDefaults(), Workers{CensusComplete: true}, Evidence{}, intent, 0, false, "owned", time.Now(), handoffLegacyDependencies(nil).ReadClaimableBudgetedWork, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid handoff binding") || decision.Action == ActNotify || expiryCalls != 0 {
		t.Fatalf("an invalid binding must refuse before expiry can supersede it: decision=%+v err=%v expiry=%d", decision, err, expiryCalls)
	}
}

func TestHandoffAdmissionKeepsALandingGoal(t *testing.T) {
	landing := approvedStewardGoal("fix-it", "Built work waiting to land", "Land it.", "2026-08-23T00:00:00Z")
	landing.State = goal.StateClaimed
	landing.Revision++
	claimRevision := landing.Revision
	landing.Claimed = &goal.ClaimRecord{
		Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-23T01:00:00Z",
		Revision: claimRevision, AccountingRevision: claimRevision,
	}
	landing.StopCapability = &goal.StopCapability{
		Generation: claimRevision, Revision: claimRevision, Machine: "bed-m1", ClaimEpoch: 1,
	}
	landing.History = append(landing.History, goal.HistoryLine{
		At: "2026-08-23T01:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAX-bed-m1-00000001",
		Verb: "claim", Actor: "bed-m1+coordinator", Targets: []string{landing.Id}, Keep: -1,
	})
	landing.Revision++
	landing.History = append(landing.History, goal.HistoryLine{
		At: "2026-08-23T03:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAY-bed-m1-00000002",
		Verb: "land-ready", Actor: "bed-m1+coordinator", Targets: []string{landing.Id}, Keep: -1,
	})
	landing.Landing = &goal.LandingRecord{At: "2026-08-23T03:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAY-bed-m1-00000002"}
	bed := newDecisionTickRepository(t)
	bed.files["plans/goals/"+landing.Id+".md"] = goal.RenderFile(landing)
	root := bed.root
	fixture := writeStagedHandoffFixture(t, root, "5000000000000003")
	intent := testIntent("5000000000000003")
	intent.Reason = seatHandoffReason
	intent.Goal = landing.Id
	intent.Handoff = &fixture.binding
	prober := handoffProbe(*intent.Handoff, identity.Dead, false, nil)

	now := time.Now()
	reads := 0
	reader := func(actualRoot string, observedAt time.Time) (goal.ClaimableBudgetedWork, error) {
		reads++
		if actualRoot != root || !observedAt.Equal(now) || reads != 1 {
			t.Fatalf("unexpected landing projection read: root=%q time=%s reads=%d", actualRoot, observedAt, reads)
		}
		tree, problems := goal.ParseTreeFiles(bed.files)
		if len(problems) != 0 {
			t.Fatalf("declared landing goal files did not parse: %v", problems)
		}
		return goal.ClaimableWorkFromProjection(goal.Projection{
			Root: root, Tree: tree, Horizon: goal.ApprovalHorizon{Now: observedAt},
		}, "bed-m1", identity.KernelProber{})
	}
	decision, _, err := decideForHandoffWithReader(root, TickConfig{}.withDefaults(), Workers{CensusComplete: true}, Evidence{}, intent, 0, false, "landing", now, reader, prober)
	if reads != 1 {
		t.Fatalf("landing projection reads = %d, want 1", reads)
	}
	if err != nil || decision.Action != ActRevive {
		t.Fatalf("a goal in this machine's landing slot must remain admitted: %+v %v", decision, err)
	}
}

func TestHandoffLaunchRefusesWhenItsNoticeCannotBeCleared(t *testing.T) {
	root, intent := prepareRevivalHandoff(t, "5000000000000004")
	prober := handoffProbe(*intent.Handoff, identity.Dead, false, nil)
	path := filepath.Join(pendingDir(root), handoffNoticeNonce(intent.Nonce)+".json")
	if err := QueueNotification(root, PendingNotification{Nonce: handoffNoticeNonce(intent.Nonce), Message: "steward: held"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "obstruction"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	launches := 0
	outcome, err := completeHandoffRevival(root, prober, TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error {
		launches++
		return nil
	})
	if err == nil || outcome.Launched || outcome.Held || launches != 0 || !strings.Contains(err.Error(), "launch refused because its hold notice could not be cleared") {
		t.Fatalf("notice cleanup damage must refuse before consumption: %+v %v launches=%d", outcome, err, launches)
	}
	if live, liveErr := LiveIntents(root); liveErr != nil || len(live) != 1 || live[0].Nonce != intent.Nonce {
		t.Fatalf("cleanup refusal must preserve the staged authorization for recovery: %+v %v", live, liveErr)
	}
	if evidence, evidenceErr := LoadEvidence(EvidencePath(root)); evidenceErr != nil || evidence.DryRevivals != 0 {
		t.Fatalf("cleanup refusal spent a dry launch: %+v %v", evidence, evidenceErr)
	}
}

func TestHandoffNoticeAuthorizationIsRecheckedBeforeDelivery(t *testing.T) {
	root, intent := prepareRevivalHandoff(t, "5000000000000007")
	prober := handoffProbe(*intent.Handoff, identity.Alive, true, nil)
	held, err := completeHandoffRevival(root, prober, TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error {
		t.Fatal("a live predecessor launched its successor")
		return nil
	})
	if err != nil || !held.Held {
		t.Fatalf("fixture did not stage its hold notice: %+v %v", held, err)
	}

	snapshotRead := make(chan struct{})
	releaseSnapshot := make(chan struct{})
	var deliveryCalls atomic.Int32
	deps := pendingDeliveryDependencies{
		afterSnapshot: func() {
			close(snapshotRead)
			<-releaseSnapshot
		},
		transport: func(_, _ string) error {
			deliveryCalls.Add(1)
			return nil
		},
	}

	type deliveryResult struct {
		delivered int
		err       error
	}
	deliveryDone := make(chan deliveryResult, 1)
	go func() {
		delivered, err := deliverPendingWithDependencies(root, deps)
		deliveryDone <- deliveryResult{delivered: delivered, err: err}
	}()
	<-snapshotRead

	prober = handoffProbe(*intent.Handoff, identity.Dead, false, nil)
	launches := 0
	launched, launchErr := completeHandoffRevival(root, prober, TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error {
		launches++
		return nil
	})
	if launchErr != nil || !launched.Launched || launches != 1 {
		t.Fatalf("terminal launch did not win the staged-notice race: %+v %v launches=%d", launched, launchErr, launches)
	}
	close(releaseSnapshot)
	delivery := <-deliveryDone
	if delivery.err != nil || delivery.delivered != 0 || deliveryCalls.Load() != 0 {
		t.Fatalf("a stale notice snapshot was delivered after launch: result=%+v delivery-calls=%d", delivery, deliveryCalls.Load())
	}
	if pending, pendingErr := PendingNotifications(root); pendingErr != nil || len(pending) != 0 {
		t.Fatalf("terminal launch left a stale handoff notice: %+v %v", pending, pendingErr)
	}
}

func TestHandoffNoticeDeliveryDoesNotReacquireArbitration(t *testing.T) {
	root, intent := prepareRevivalHandoff(t, "500000000000000b")
	prober := handoffProbe(*intent.Handoff, identity.Alive, true, nil)
	if held, err := completeHandoffRevival(root, prober, TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error { return nil }); err != nil || !held.Held {
		t.Fatalf("fixture did not stage its hold notice: %+v %v", held, err)
	}

	arbitration, err := AcquireArbitration(root)
	if err != nil {
		t.Fatal(err)
	}
	type deliveryResult struct {
		delivered int
		err       error
	}
	done := make(chan deliveryResult, 1)
	previousWait := beforeArbitrationWait
	beforeArbitrationWait = func() { t.Error("handoff notice delivery tried to acquire steward arbitration") }
	t.Cleanup(func() { beforeArbitrationWait = previousWait })
	go func() {
		delivered, err := deliverPendingWith(root, func(_, _ string) error { return nil })
		done <- deliveryResult{delivered: delivered, err: err}
	}()
	result := <-done
	arbitration.Release()
	if result.err != nil || result.delivered != 1 {
		t.Fatalf("handoff notice delivery failed while its caller held arbitration: %+v", result)
	}
}

func TestHandoffNoticeDeliveryUsesTheInitialQueueSnapshot(t *testing.T) {
	root, intent := prepareRevivalHandoff(t, "500000000000000c")
	prober := handoffProbe(*intent.Handoff, identity.Alive, true, nil)
	if held, err := completeHandoffRevival(root, prober, TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error { return nil }); err != nil || !held.Held {
		t.Fatalf("fixture did not stage its hold notice: %+v %v", held, err)
	}

	var deliveryCalls atomic.Int32
	deps := pendingDeliveryDependencies{
		afterSnapshot: func() {
			if err := os.WriteFile(filepath.Join(pendingDir(root), "late-malformed.json"), []byte("{"), 0o644); err != nil {
				t.Fatal(err)
			}
		},
		transport: func(_, _ string) error {
			deliveryCalls.Add(1)
			return nil
		},
	}
	delivered, err := deliverPendingWithDependencies(root, deps)
	if err != nil || delivered != 1 || deliveryCalls.Load() != 1 {
		t.Fatalf("a later queue change altered the already-decoded delivery pass: delivered=%d calls=%d err=%v", delivered, deliveryCalls.Load(), err)
	}
	if _, err := os.Stat(filepath.Join(pendingDir(root), "late-malformed.json")); err != nil {
		t.Fatalf("the later queue entry was unexpectedly consumed in the same pass: %v", err)
	}
}

func TestHandoffNoticeDeliveryRequiresLiveAuthorization(t *testing.T) {
	var deliveryCalls atomic.Int32
	deliver := func(_, _ string) error {
		deliveryCalls.Add(1)
		return nil
	}

	t.Run("live bound handoff delivers", func(t *testing.T) {
		deliveryCalls.Store(0)
		root, intent := prepareRevivalHandoff(t, "5000000000000008")
		prober := handoffProbe(*intent.Handoff, identity.Alive, true, nil)
		if held, err := completeHandoffRevival(root, prober, TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error { return nil }); err != nil || !held.Held {
			t.Fatalf("fixture handoff did not hold: %+v %v", held, err)
		}
		delivered, err := deliverPendingWith(root, deliver)
		if err != nil || delivered != 1 || deliveryCalls.Load() != 1 {
			t.Fatalf("a live bound handoff notice did not deliver: delivered=%d calls=%d err=%v", delivered, deliveryCalls.Load(), err)
		}
		if live, liveErr := LiveIntents(root); liveErr != nil || len(live) != 1 || live[0].Nonce != intent.Nonce {
			t.Fatalf("notice delivery changed the live handoff: %+v %v", live, liveErr)
		}
	})

	t.Run("crash after cancellation authority removal retires notice", func(t *testing.T) {
		deliveryCalls.Store(0)
		root, intent := prepareRevivalHandoff(t, "5000000000000009")
		prober := handoffProbe(*intent.Handoff, identity.Alive, true, nil)
		if held, err := completeHandoffRevival(root, prober, TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error { return nil }); err != nil || !held.Held {
			t.Fatalf("fixture handoff did not hold: %+v %v", held, err)
		}
		// Cancellation deliberately removes launch authority first. Model a
		// crash before its later notice cleanup, leaving a regular queue file.
		if err := os.Remove(filepath.Join(intentsDir(root), intent.Nonce+".json")); err != nil {
			t.Fatal(err)
		}
		delivered, err := deliverPendingWith(root, deliver)
		if err != nil || delivered != 0 || deliveryCalls.Load() != 0 {
			t.Fatalf("an orphaned terminal notice was delivered: delivered=%d calls=%d err=%v", delivered, deliveryCalls.Load(), err)
		}
		if pending, pendingErr := PendingNotifications(root); pendingErr != nil || len(pending) != 0 {
			t.Fatalf("the orphaned terminal notice was not retired: %+v %v", pending, pendingErr)
		}
	})

	t.Run("malformed live authority refuses and preserves evidence", func(t *testing.T) {
		deliveryCalls.Store(0)
		root, intent := prepareRevivalHandoff(t, "500000000000000a")
		prober := handoffProbe(*intent.Handoff, identity.Alive, true, nil)
		if held, err := completeHandoffRevival(root, prober, TickConfig{}, deadCensus(), intent.Nonce, func(Intent) error { return nil }); err != nil || !held.Held {
			t.Fatalf("fixture handoff did not hold: %+v %v", held, err)
		}
		intent.Handoff = nil
		if err := UpdateIntent(root, intent); err != nil {
			t.Fatal(err)
		}
		delivered, err := deliverPendingWith(root, deliver)
		if err == nil || !strings.Contains(err.Error(), "does not name a live bound seatHandoff intent") || delivered != 0 || deliveryCalls.Load() != 0 {
			t.Fatalf("malformed live authority did not refuse delivery: delivered=%d calls=%d err=%v", delivered, deliveryCalls.Load(), err)
		}
		if pending, pendingErr := PendingNotifications(root); pendingErr != nil || len(pending) != 1 {
			t.Fatalf("the refusal discarded malformed-state evidence: %+v %v", pending, pendingErr)
		}
	})
}
