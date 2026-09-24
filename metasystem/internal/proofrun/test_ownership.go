package proofrun

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

// Test ownership is written in the same transaction as the attempt. The
// inventory is complete before any worker starts, and only TestOwned creates
// a live observation. A consumer never becomes the producer of a wait edge.
func validateTestOwnership(attempt Attempt) error {
	if len(attempt.TestInventory) == 0 {
		if len(attempt.TestOwned) != 0 || len(attempt.TestWaits) != 0 || len(attempt.TestSources) != 0 || len(attempt.TestFreshGroups) != 0 ||
			attempt.TestAdmission != 0 || attempt.FreshnessEpisode != "" || attempt.FreshnessBinding != "" || attempt.FreshnessExpiresAt != "" {
			return fmt.Errorf("testing ownership has no admitted inventory")
		}
		return nil
	}
	if attempt.TestAdmission == 0 {
		return fmt.Errorf("testing ownership has no admission order")
	}
	if attempt.FreshnessEpisode != "" && (!validSHA256(attempt.FreshnessEpisode) || !validSHA256(attempt.FreshnessBinding)) ||
		attempt.FreshnessEpisode == "" && attempt.FreshnessBinding != "" {
		return fmt.Errorf("testing freshness has no valid episode binding")
	}
	if attempt.FreshnessExpiresAt != "" {
		if attempt.FreshnessEpisode == "" {
			return fmt.Errorf("testing freshness expiry has no episode")
		}
		if _, err := time.Parse(time.RFC3339Nano, attempt.FreshnessExpiresAt); err != nil {
			return fmt.Errorf("testing freshness expiry is invalid: %w", err)
		}
	}
	for id, fresh := range attempt.TestFreshGroups {
		if !fresh || attempt.FreshnessEpisode == "" || attempt.TestInventory[id] == "" {
			return fmt.Errorf("testing fresh group %s is outside its admitted episode", id)
		}
	}
	for id, identity := range attempt.TestInventory {
		if id == "" || !validSHA256(identity) {
			return fmt.Errorf("testing inventory has an invalid component identity")
		}
		if owned, ok := attempt.TestOwned[id]; ok && owned != identity {
			return fmt.Errorf("testing producer %s differs from admitted identity", id)
		}
		claims := 0
		if attempt.TestOwned[id] != "" {
			claims++
		}
		if attempt.TestWaits[id] != "" {
			claims++
		}
		if attempt.TestSources[id] != "" {
			claims++
		}
		if claims != 1 {
			return fmt.Errorf("testing component %s has %d sources, want one", id, claims)
		}
		if owner, ok := attempt.TestWaits[id]; ok {
			if !safeAttemptID(owner) || owner == attempt.AttemptID || attempt.TestOwned[id] != "" {
				return fmt.Errorf("testing wait for %s has an invalid producer", id)
			}
		}
	}
	for id := range attempt.TestOwned {
		if attempt.TestInventory[id] == "" {
			return fmt.Errorf("testing producer %s is outside the inventory", id)
		}
	}
	for id := range attempt.TestWaits {
		if attempt.TestInventory[id] == "" {
			return fmt.Errorf("testing wait %s is outside the inventory", id)
		}
	}
	for id, owner := range attempt.TestSources {
		if attempt.TestInventory[id] == "" || !safeAttemptID(owner) || owner == attempt.AttemptID {
			return fmt.Errorf("testing source %s is outside the inventory or invalid", id)
		}
	}
	return nil
}

func allocateTestOwnershipLocked(attempt *Attempt, request AdmissionRequest) error {
	if len(request.ComponentIdentities) == 0 {
		return fmt.Errorf("shared testing admission requires a complete component inventory")
	}
	attempts, err := ReadAttempts(request.ControlRoot)
	if err != nil {
		return err
	}
	attempt.TestInventory = make(map[string]string, len(request.ComponentIdentities))
	attempt.TestOwned = map[string]string{}
	attempt.TestWaits = map[string]string{}
	attempt.TestSources = map[string]string{}
	attempt.FreshnessEpisode = request.FreshnessEpisode
	attempt.FreshnessBinding = request.FreshnessBinding
	attempt.FreshnessExpiresAt = request.FreshnessExpiresAt
	attempt.TestFreshGroups = cloneFreshGroups(request.FreshGroups)
	if request.FreshnessExpiresAt != "" {
		expires, err := time.Parse(time.RFC3339Nano, request.FreshnessExpiresAt)
		if err != nil || !expires.After(request.Now) {
			return fmt.Errorf("freshness episode has expired; renew the proof decision")
		}
	}
	sequencePath := filepath.Join(attemptsDir(request.ControlRoot), "test-admission-sequence")
	if data, readErr := os.ReadFile(sequencePath); readErr == nil {
		stored, parseErr := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
		if parseErr != nil {
			return fmt.Errorf("testing admission sequence is unreadable: %w", parseErr)
		}
		attempt.TestAdmission = stored
	} else if !os.IsNotExist(readErr) {
		return readErr
	}
	for _, existing := range attempts {
		if existing.AttemptID == attempt.AttemptID {
			continue
		}
		if existing.TestAdmission > attempt.TestAdmission {
			attempt.TestAdmission = existing.TestAdmission
		}
	}
	if attempt.TestAdmission == math.MaxUint64 {
		return fmt.Errorf("testing admission sequence is exhausted")
	}
	attempt.TestAdmission++
	ids := make([]string, 0, len(request.ComponentIdentities))
	for id, identity := range request.ComponentIdentities {
		if id == "" || !validSHA256(identity) {
			return fmt.Errorf("testing component %q has invalid identity", id)
		}
		ids = append(ids, id)
		attempt.TestInventory[id] = identity
	}
	sort.Strings(ids)
	for _, id := range ids {
		identity := request.ComponentIdentities[id]
		episode, binding := requestGroupFreshness(request, id)
		expiry := ""
		if episode != "" {
			expiry = request.FreshnessExpiresAt
		}
		observation, found := newestOwnedObservation(attempts, id, identity, episode, binding, expiry, request.Now)
		if found && observation.ambiguous {
			if !request.ForceGroups || observation.live {
				return fmt.Errorf("testing history for %s has ambiguous admission order", id)
			}
			// Explicit group execution creates a new numbered owner without
			// treating either tied terminal record as reusable evidence.
			found = false
		}
		if found && observation.live {
			if observation.attempt.TestAdmission >= attempt.TestAdmission {
				return fmt.Errorf("testing wait for %s would point to a later admission", id)
			}
			attempt.TestWaits[id] = observation.attempt.AttemptID
			continue
		}
		if found && observation.passed && !request.ForceGroups {
			attempt.TestSources[id] = observation.attempt.AttemptID
			continue
		}
		attempt.TestOwned[id] = identity
	}
	if err := validateTestOwnership(*attempt); err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(sequencePath, strconv.FormatUint(attempt.TestAdmission, 10)+"\n", request.ControlRoot)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("testing admission sequence durability is unknown")
	}
	return nil
}

// BindJoinedTestOwnershipLocked freezes a nested worker's complete selection
// on its already admitted parent. The proof mutation lock is held by caller.
func BindJoinedTestOwnershipLocked(root, attemptID string, request AdmissionRequest) (Attempt, error) {
	attempt, err := ReadAttempt(root, attemptID)
	if err != nil {
		return Attempt{}, err
	}
	if !testingAttemptSchema(attempt.SchemaVersion) || attempt.Terminal != nil || attempt.CancellationIntent != "" ||
		attempt.TestResult != nil || len(attempt.PendingTestGroups) != 0 || len(attempt.TestInventory) != 0 {
		return Attempt{}, fmt.Errorf("live proof attempt without an existing testing owner is required")
	}
	if attempt.SchemaVersion == CandidateAttemptSchemaVersion {
		attempt.SchemaVersion = IdentityAttemptSchemaVersion
	}
	if err := allocateTestOwnershipLocked(&attempt, request); err != nil {
		return Attempt{}, err
	}
	attempt.PendingTestGroups = make(map[string]string, len(attempt.TestInventory))
	for id, identity := range attempt.TestInventory {
		attempt.PendingTestGroups[id] = identity
	}
	if err := writeAttempt(attempt); err != nil {
		return Attempt{}, err
	}
	return attempt, nil
}

type ownedObservation struct {
	attempt   Attempt
	at        time.Time
	live      bool
	passed    bool
	failed    bool
	ambiguous bool
}

func cloneFreshGroups(groups map[string]bool) map[string]bool {
	if len(groups) == 0 {
		return nil
	}
	copy := make(map[string]bool, len(groups))
	for id, fresh := range groups {
		if fresh {
			copy[id] = true
		}
	}
	return copy
}

func requestGroupFreshness(request AdmissionRequest, id string) (string, string) {
	if request.FreshnessEpisode == "" {
		return "", ""
	}
	if len(request.FreshGroups) == 0 || request.FreshGroups[id] {
		return request.FreshnessEpisode, request.FreshnessBinding
	}
	return "", ""
}

func attemptGroupFreshness(attempt Attempt, id string) (string, string) {
	if attempt.FreshnessEpisode == "" {
		return "", ""
	}
	// Older attempts recorded only whole-attempt freshness. Treat every group
	// in those attempts as fresh rather than silently widening its reuse.
	if len(attempt.TestFreshGroups) == 0 || attempt.TestFreshGroups[id] {
		return attempt.FreshnessEpisode, attempt.FreshnessBinding
	}
	return "", ""
}

func resultGroupFreshness(result TestResult, id string) (string, string) {
	if result.FreshnessEpisode == "" {
		return "", ""
	}
	if len(result.FreshGroups) == 0 || result.FreshGroups[id] {
		return result.FreshnessEpisode, result.FreshnessBinding
	}
	return "", ""
}

// TestGroupFresh reports whether this result requires a native observation
// from its exact episode for one selected group.
func TestGroupFresh(result TestResult, id string) bool {
	episode, _ := resultGroupFreshness(result, id)
	return episode != ""
}

// MatchesTestResultFreshness checks one producer against one group's decision.
func MatchesTestResultFreshness(owner Attempt, result TestResult, id string) bool {
	ownerEpisode, ownerBinding := attemptGroupFreshness(owner, id)
	needEpisode, needBinding := resultGroupFreshness(result, id)
	if ownerEpisode != needEpisode || ownerBinding != needBinding {
		return false
	}
	return needEpisode == "" || owner.FreshnessExpiresAt == result.FreshnessExpiresAt
}

func sameGroupFreshness(owner, consumer Attempt, id string) bool {
	ownerEpisode, ownerBinding := attemptGroupFreshness(owner, id)
	consumerEpisode, consumerBinding := attemptGroupFreshness(consumer, id)
	if ownerEpisode != consumerEpisode || ownerBinding != consumerBinding {
		return false
	}
	return ownerEpisode == "" || owner.FreshnessExpiresAt == consumer.FreshnessExpiresAt
}

// An older format's live plan is treated conservatively as a producer. New
// attempts publish TestOwned, so borrowed inventory never masks its source.
func ownsTestComponent(attempt Attempt, id, identity string) bool {
	if len(attempt.TestInventory) != 0 {
		return attempt.TestOwned[id] == identity
	}
	if attempt.Terminal != nil && attempt.TestResult != nil {
		for _, group := range attempt.TestResult.Groups {
			if group.ID == id && group.ExecutionIdentity == identity {
				return NativeTestProducer(attempt, group)
			}
		}
	}
	if attempt.PendingTestGroups[id] == identity {
		return true
	}
	// A proof identity describes the selected plan, including borrowed
	// evidence. It is never a native producer reservation.
	return false
}

// NativeTestProducer checks the execution reservation behind an observation.
// Old attempts can prove native ownership only through PendingTestGroups.
func NativeTestProducer(attempt Attempt, group GroupResult) bool {
	if !group.NativeLaunched || group.Status == "reused" || group.ReuseAttempt != "" {
		return false
	}
	if len(attempt.TestInventory) != 0 {
		if attempt.TestResult != nil && !admittedTestFreshnessMatches(attempt, *attempt.TestResult) {
			return false
		}
		return attempt.TestOwned[group.ID] == group.ExecutionIdentity
	}
	return attempt.PendingTestGroups[group.ID] == group.ExecutionIdentity
}

func validateNativeProducerClaims(attempt Attempt) error {
	if attempt.TestResult == nil || len(attempt.TestInventory) == 0 {
		// Earlier attempts did not publish an execution-owner inventory.
		// Their native marker remains readable for their original receipts,
		// but cannot become a new shared producer reservation.
		return nil
	}
	for _, group := range attempt.TestResult.Groups {
		if group.NativeLaunched && !NativeTestProducer(attempt, group) {
			return fmt.Errorf("testing group %s claims native evidence without a producer reservation", group.ID)
		}
	}
	return nil
}

func newestOwnedObservation(attempts []Attempt, id, identity, episode, binding, expiry string, now time.Time) (ownedObservation, bool) {
	return newestOwnedObservationWhere(attempts, id, identity, episode, binding, expiry, now, nil)
}

func newestOwnedObservationWhere(attempts []Attempt, id, identity, episode, binding, expiry string, now time.Time, include func(Attempt) bool) (ownedObservation, bool) {
	var newest ownedObservation
	found := false
	for _, attempt := range attempts {
		if include != nil && !include(attempt) {
			continue
		}
		if attempt.SchemaVersion == IdentityAttemptSchemaVersion && attempt.Terminal != nil && attempt.TestResult != nil &&
			len(attempt.TestInventory) != 0 && !admittedTestFreshnessMatches(attempt, *attempt.TestResult) {
			continue
		}
		observedEpisode, observedBinding := attemptGroupFreshness(attempt, id)
		if observedEpisode != episode || observedBinding != binding {
			continue
		}
		if episode != "" && attempt.FreshnessExpiresAt != expiry {
			continue
		}
		if observedEpisode != "" && attempt.FreshnessExpiresAt != "" {
			expires, err := time.Parse(time.RFC3339Nano, attempt.FreshnessExpiresAt)
			if err != nil || !now.IsZero() && !expires.After(now) {
				continue
			}
		}
		candidate := ownedObservation{attempt: attempt}
		if attempt.Terminal == nil && ownsTestComponent(attempt, id, identity) {
			candidate.live = true
			candidate.at, _ = time.Parse(time.RFC3339Nano, attempt.StartedAt)
		} else if attempt.Terminal != nil && attempt.TestResult != nil {
			for _, group := range attempt.TestResult.Groups {
				if group.ID != id || group.ExecutionIdentity != identity || !NativeTestProducer(attempt, group) {
					continue
				}
				candidate.at, _ = time.Parse(time.RFC3339Nano, group.EndedAt)
				if candidate.at.IsZero() {
					candidate.at, _ = time.Parse(time.RFC3339Nano, attempt.StartedAt)
				}
				candidate.passed = group.Status == "passed" && group.CollectionComplete
				candidate.failed = !candidate.passed
				break
			}
		}
		if attempt.Terminal != nil && !candidate.passed && !candidate.failed && ownsTestComponent(attempt, id, identity) &&
			!blockedWithoutNativeProducer(attempt, id, identity) {
			candidate.failed = true
			candidate.at, _ = time.Parse(time.RFC3339Nano, attempt.Terminal.At)
		}
		if !candidate.live && !candidate.passed && !candidate.failed {
			continue
		}
		if !found {
			newest, found = candidate, true
			continue
		}
		newestStarted, _ := time.Parse(time.RFC3339Nano, newest.attempt.StartedAt)
		candidateStarted, _ := time.Parse(time.RFC3339Nano, candidate.attempt.StartedAt)
		switch compareTestingAttemptObservations(
			testingAttemptObservation{attempt: newest.attempt, live: newest.live, at: newest.at, started: newestStarted},
			testingAttemptObservation{attempt: candidate.attempt, live: candidate.live, at: candidate.at, started: candidateStarted},
		) {
		case testingAttemptNewer:
			newest = candidate
		case testingAttemptAmbiguous:
			newest.ambiguous = true
			newest.passed = false
			newest.failed = true
		}
	}
	return newest, found
}

// An identity-bound blocked dependent never launched. Its owned reservation is not
// a failed native producer: once its prerequisite is repaired, it must be
// eligible for a first execution. Legacy not-run reservations retain their
// conservative retry fence.
func blockedWithoutNativeProducer(attempt Attempt, id, identity string) bool {
	if attempt.TestResult == nil || !identityBoundTestResultSchema(attempt.TestResult.SchemaVersion) {
		return false
	}
	for _, group := range attempt.TestResult.Groups {
		if group.ID == id && group.ExecutionIdentity == identity {
			return group.Status == "blocked" && !group.NativeLaunched
		}
	}
	return false
}

// Shared component admission may reserve a missing subset while another
// attempt produces the rest. A failed native producer is never a cache miss.
func sharedComponentDecisionLocked(request AdmissionRequest) (*Attempt, LaunchResult, bool, error) {
	if request.FreshnessEpisode != "" && (!validSHA256(request.FreshnessEpisode) || !validSHA256(request.FreshnessBinding)) ||
		request.FreshnessEpisode == "" && (request.FreshnessBinding != "" || request.FreshnessExpiresAt != "") {
		return nil, LaunchResult{}, false, fmt.Errorf("testing freshness has no valid episode binding")
	}
	if request.FreshnessExpiresAt != "" {
		now := request.Now
		if now.IsZero() {
			now = time.Now().UTC()
		}
		expires, err := time.Parse(time.RFC3339Nano, request.FreshnessExpiresAt)
		if err != nil || !expires.After(now) {
			return nil, LaunchResult{}, false, fmt.Errorf("freshness episode has expired; renew the proof decision")
		}
	}
	attempts, err := ReadAttempts(request.ControlRoot)
	if err != nil {
		return nil, LaunchResult{}, false, err
	}
	allPassed := len(request.ComponentIdentities) > 0
	var failed *Attempt
	requestTreeKnown := validTreeDigest(request.CandidateTree)
	for id, identity := range request.ComponentIdentities {
		episode, binding := requestGroupFreshness(request, id)
		expiry := ""
		if episode != "" {
			expiry = request.FreshnessExpiresAt
		}
		observation, found := newestOwnedObservation(attempts, id, identity, episode, binding, expiry, request.Now)
		if found && observation.ambiguous {
			if !request.ForceGroups || observation.live {
				return nil, LaunchResult{}, false, fmt.Errorf("testing history for %s has ambiguous admission order", id)
			}
			// A forced run may supersede tied terminal history, but the tied
			// records remain unusable and cannot nominate one retry source.
			allPassed = false
			continue
		}
		if !found || observation.live || !observation.passed {
			allPassed = false
		}
		// The global newest red vetoes an older pass across trees. Separately,
		// the latest observation on this candidate (or an unknown legacy tree)
		// decides whether its own failed producer still needs an accountable
		// retry. A newer pass on this candidate supersedes its earlier red.
		if found && observation.failed {
			scoped, scopedFound := newestOwnedObservationWhere(attempts, id, identity, episode, binding, expiry, request.Now,
				func(attempt Attempt) bool {
					return failureBelongsToCandidate(attempt, request.CandidateTree, requestTreeKnown)
				})
			if !scopedFound || !scoped.failed {
				continue
			}
			copy := scoped.attempt
			if failed == nil || copy.TestAdmission > failed.TestAdmission {
				failed = &copy
			}
		}
	}
	if failed != nil && request.RetryDecisionPath == "" {
		return failed, LaunchResult{SchemaVersion: 1, Disposition: DispositionRetryRequired,
			AttemptID: failed.AttemptID, PriorAttempt: failed.AttemptID,
			EvidencePath: mustAttemptPath(request.ControlRoot, failed.AttemptID), ExitStatus: ExitRetryRequired}, true, nil
	}
	if allPassed && !request.ForceAttempt && !request.ForceGroups {
		return nil, LaunchResult{SchemaVersion: 1, Disposition: DispositionReusableSuccess,
			EvidencePath: attemptsDir(request.ControlRoot), ExitStatus: ExitReusableSuccess}, true, nil
	}
	return failed, LaunchResult{}, false, nil
}

// WaitForTestProducer observes the original terminal record. It never holds a
// native execution slot or changes the producer's cancellation state.
func WaitForTestProducer(ctx context.Context, root string, consumer Attempt, id string) (GroupResult, error) {
	return WaitForTestProducerWithWaitCheck(ctx, root, consumer, id, nil)
}

// WaitForTestProducerWithWaitCheck lets the caller end a producer wait when
// its admission authority changes. Cancellation remains a separate
// infrastructure failure supplied by the caller's context.
func WaitForTestProducerWithWaitCheck(ctx context.Context, root string, consumer Attempt, id string, check func() error) (GroupResult, error) {
	ownerID := consumer.TestWaits[id]
	identity := consumer.TestInventory[id]
	if ownerID == "" || identity == "" {
		return GroupResult{}, fmt.Errorf("testing wait %s is outside admitted inventory", id)
	}
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		if check != nil {
			if err := check(); err != nil {
				return GroupResult{}, err
			}
		}
		owner, err := ReadAttempt(root, ownerID)
		if err != nil {
			return GroupResult{}, fmt.Errorf("read testing producer %s: %w", ownerID, err)
		}
		if owner.TestAdmission >= consumer.TestAdmission || !ownsTestComponent(owner, id, identity) ||
			!sameGroupFreshness(owner, consumer, id) {
			return GroupResult{}, fmt.Errorf("testing producer %s does not own the earlier matching reservation for %s", ownerID, id)
		}
		if owner.Terminal != nil {
			if owner.TestResult == nil {
				return GroupResult{}, fmt.Errorf("testing producer %s ended without complete result for %s: %s", ownerID, id, owner.Terminal.Result)
			}
			for _, group := range owner.TestResult.Groups {
				if group.ID != id || group.ExecutionIdentity != identity || !group.NativeLaunched {
					continue
				}
				group.NativeLaunched = false
				group.ReuseAttempt = ownerID
				if group.Status == "passed" && group.CollectionComplete {
					group.Status = "reused"
				}
				return group, nil
			}
			return GroupResult{}, fmt.Errorf("testing producer %s ended without native evidence for %s", ownerID, id)
		}
		select {
		case <-ctx.Done():
			return GroupResult{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

func validateAdmittedTestResult(attempt Attempt, result TestResult, now time.Time) error {
	if len(attempt.TestInventory) == 0 {
		return nil
	}
	if !admittedTestFreshnessMatches(attempt, result) ||
		len(result.Groups) != len(attempt.TestInventory) {
		return fmt.Errorf("retained testing result differs from admitted inventory or freshness")
	}
	if len(result.FreshGroups) != len(attempt.TestFreshGroups) {
		return fmt.Errorf("retained testing result changes admitted fresh groups")
	}
	for id := range attempt.TestFreshGroups {
		if !result.FreshGroups[id] {
			return fmt.Errorf("retained testing result removes fresh group %s", id)
		}
	}
	for _, group := range result.Groups {
		if attempt.TestInventory[group.ID] != group.ExecutionIdentity {
			return fmt.Errorf("retained testing group %s differs from admitted identity", group.ID)
		}
		if attempt.TestOwned[group.ID] != "" {
			if group.Status == "reused" || group.ReuseAttempt != "" {
				return fmt.Errorf("reserved producer %s cannot reuse another attempt", group.ID)
			}
			continue
		}
		if ownerID := attempt.TestWaits[group.ID]; ownerID != "" {
			if group.Status == "not-run" && !group.NativeLaunched && group.ReuseAttempt == "" {
				continue
			}
			if group.NativeLaunched || group.ReuseAttempt != ownerID {
				return fmt.Errorf("waited group %s does not reference its producer", group.ID)
			}
			owner, err := ReadAttempt(attempt.ControlRoot, ownerID)
			if err != nil || owner.Terminal == nil || owner.TestAdmission >= attempt.TestAdmission ||
				!ownsTestComponent(owner, group.ID, group.ExecutionIdentity) {
				return fmt.Errorf("waited group %s has no earlier terminal producer", group.ID)
			}
			original, err := nativeSourceGroup(attempt, group, now)
			if err != nil {
				return err
			}
			wantStatus := original.Status
			if wantStatus == "passed" && original.CollectionComplete {
				wantStatus = "reused"
			}
			if group.Status != wantStatus || group.StartedAt != original.StartedAt || group.EndedAt != original.EndedAt || group.LogDigest != original.LogDigest {
				return fmt.Errorf("waited group %s changes its producer observation", group.ID)
			}
			continue
		}
		if group.Status != "reused" && group.Status != "not-run" || group.NativeLaunched {
			return fmt.Errorf("unowned group %s cannot carry native evidence", group.ID)
		}
		if group.Status == "reused" {
			if group.ReuseAttempt != attempt.TestSources[group.ID] {
				return fmt.Errorf("reused group %s changes its admitted source", group.ID)
			}
			original, err := nativeSourceGroup(attempt, group, now)
			if err != nil || original.Status != "passed" || !original.CollectionComplete {
				return fmt.Errorf("reused group %s has no complete native source: %v", group.ID, err)
			}
		}
	}
	return nil
}

// normalizeInvalidInheritedTestEvidence keeps an unsuccessful diagnostic
// record structurally complete when an exact admitted reuse disappears before
// the terminal commit. It does not make the inherited evidence usable: the
// group becomes explicitly not run and delivery is recomputed as insufficient.
func normalizeInvalidInheritedTestEvidence(attempt Attempt, result TestResult, now time.Time) TestResult {
	if len(attempt.TestInventory) == 0 || !admittedTestFreshnessMatches(attempt, result) ||
		len(result.Groups) != len(attempt.TestInventory) {
		return result
	}
	normalized := result
	normalized.Groups = append([]GroupResult(nil), result.Groups...)
	normalized.Uncertainty = append([]string(nil), result.Uncertainty...)
	changed := false
	for index, group := range result.Groups {
		if attempt.TestInventory[group.ID] != group.ExecutionIdentity || group.Status != "reused" || group.NativeLaunched {
			continue
		}
		source := attempt.TestSources[group.ID]
		if source == "" {
			source = attempt.TestWaits[group.ID]
		}
		if source == "" || group.ReuseAttempt != source {
			continue
		}
		original, err := nativeSourceGroup(attempt, group, now)
		if err == nil && original.Status == "passed" && original.CollectionComplete {
			continue
		}
		if err == nil {
			err = fmt.Errorf("group %s source is not a complete native pass", group.ID)
		}
		reason := "admitted inherited evidence became invalid before terminal retention: " + err.Error()
		normalized.Groups[index] = GroupResult{ID: group.ID, Kind: group.Kind, Obligations: append([]string(nil), group.Obligations...),
			InputDigest: group.InputDigest, InputManifest: append([]string(nil), group.InputManifest...), ExecutionIdentity: group.ExecutionIdentity,
			IdentityVersion: group.IdentityVersion, CWD: group.CWD, Status: "not-run", NotRunReason: reason,
			ToolIdentities: map[string]string{}, ReportDigests: map[string]string{}}
		switch group.Kind {
		case "build":
			if normalized.LaunchCounts.ReusedBuild > 0 {
				normalized.LaunchCounts.ReusedBuild--
			}
		default:
			if normalized.LaunchCounts.ReusedTest > 0 {
				normalized.LaunchCounts.ReusedTest--
			}
		}
		normalized.Uncertainty = append(normalized.Uncertainty, reason)
		changed = true
	}
	if changed {
		normalized.Cost.ReusedLaunches = normalized.LaunchCounts.ReusedTest + normalized.LaunchCounts.ReusedBuild + normalized.LaunchCounts.ReusedOther
		normalized.RecomputeDelivery()
	}
	return normalized
}

func admittedTestFreshnessMatches(attempt Attempt, result TestResult) bool {
	return result.FreshnessEpisode == attempt.FreshnessEpisode &&
		result.FreshnessBinding == attempt.FreshnessBinding &&
		result.FreshnessExpiresAt == attempt.FreshnessExpiresAt
}

func nativeSourceGroup(consumer Attempt, group GroupResult, now time.Time) (GroupResult, error) {
	owner, err := ReadAttempt(consumer.ControlRoot, group.ReuseAttempt)
	if err != nil || owner.Terminal == nil || owner.TestResult == nil ||
		!sameGroupFreshness(owner, consumer, group.ID) {
		return GroupResult{}, fmt.Errorf("group %s has no terminal source in the same freshness episode", group.ID)
	}
	episode, _ := attemptGroupFreshness(owner, group.ID)
	if episode != "" && owner.FreshnessExpiresAt != "" {
		if now.IsZero() {
			now = time.Now().UTC()
		}
		expires, err := time.Parse(time.RFC3339Nano, owner.FreshnessExpiresAt)
		if err != nil || !expires.After(now) {
			return GroupResult{}, fmt.Errorf("group %s native source has expired", group.ID)
		}
	}
	for _, native := range owner.TestResult.Groups {
		if native.ID == group.ID && native.ExecutionIdentity == group.ExecutionIdentity && native.NativeLaunched &&
			native.Status != "reused" {
			return native, nil
		}
	}
	return GroupResult{}, fmt.Errorf("group %s has no matching native source", group.ID)
}
