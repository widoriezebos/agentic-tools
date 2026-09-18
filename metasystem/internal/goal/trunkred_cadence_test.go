package goal

import (
	"crypto/sha256"
	"strings"
	"testing"
	"time"
)

func cadenceKeyFixture() CadenceClaimKey {
	return CadenceClaimKey{
		TrunkTree:         strings.Repeat("a", 40),
		WeightGeneration:  7,
		ForcedWindowStart: "2026-09-17T00:00:00Z",
	}
}

func cadenceStatusFixture(key CadenceClaimKey, started time.Time, status string) CadenceStatus {
	return CadenceStatus{
		TrunkCommit: strings.Repeat("b", 40), TrunkTree: key.TrunkTree, Trigger: CadenceTriggerIdentityChanged,
		RunID: "run-21", AttemptID: "attempt-21", StartedAt: started.UTC().Format(time.RFC3339),
		EndedAt: started.Add(time.Minute).UTC().Format(time.RFC3339), WeightGeneration: key.WeightGeneration,
		ForcedWindowStart: key.ForcedWindowStart,
		Groups: []CadenceGroupStatus{{Group: "section/deep-only", ExecutionIdentity: strings.Repeat("c", 64),
			Status: status, EvidenceDigest: strings.Repeat("d", 64)}},
	}
}

func TestTrunkRedBatchGuardsExcludeCadenceRecords(t *testing.T) {
	root := soloLedgerRepo(t)
	now := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	batchRequest := trunkRedVerbReq(root, "01J5X0000000000000000000D1", "mac-a")
	batchRequest.Now = now

	withoutBatch := trunkRedRecordFixture("tr-fast-no-batch", "", "attempt", "base", batchRequest.stamp())
	if _, err := RecordTrunkRed(batchRequest, withoutBatch); err == nil || err.Error() != "trunk-red record requires a batch" {
		t.Fatalf("batch record without batch error = %v", err)
	}
	withoutGroups := TrunkRedRecordArgs{Batch: "batch", Attempt: "attempt", BaseCommit: "base", BaseTree: "tree", SeenAt: batchRequest.stamp()}
	if _, err := RecordTrunkRed(batchRequest, withoutGroups); err == nil || err.Error() != "trunk-red record requires at least one group" {
		t.Fatalf("batch record without groups error = %v", err)
	}

	claimRequest := trunkRedVerbReq(root, "01J5X0000000000000000000D2", "mac-a")
	claimRequest.Now = now.Add(time.Minute)
	claim := CadenceClaim{Key: cadenceKeyFixture(), OwnerMachine: "mac-a", Opid: claimRequest.opid(),
		ClaimedAt: claimRequest.stamp(), LeaseUntil: claimRequest.Now.Add(time.Hour).Format(time.RFC3339)}
	claimed, err := RecordTrunkRed(claimRequest, TrunkRedRecordArgs{CadenceClaim: &claim})
	if err != nil || claimed.Outcome != OutcomeConfirmed {
		t.Fatalf("cadence claim without batch = %+v, %v", claimed, err)
	}

	publishRequest := trunkRedVerbReq(root, "01J5X0000000000000000000D3", "mac-a")
	publishRequest.Now = now.Add(2 * time.Minute)
	status := cadenceStatusFixture(claim.Key, claimRequest.Now, "passed")
	status.Opid = publishRequest.opid()
	published, err := RecordTrunkRed(publishRequest, TrunkRedRecordArgs{Cadence: &status, CadenceClaimOpid: claim.Opid})
	if err != nil || published.Outcome != OutcomeConfirmed {
		t.Fatalf("cadence publication without batch or red groups = %+v, %v", published, err)
	}
	projection, err := ProjectAt(root, published.Tip, publishRequest.Now)
	if err != nil || projection.Tree.Cadence == nil || projection.Tree.Cadence.Opid != publishRequest.opid() || projection.Tree.CadenceClaim != nil || len(projection.Tree.TrunkRed) != 0 {
		t.Fatalf("cadence-only publication = %+v, %v", projection.Tree, err)
	}
}

func TestCadenceClaimDeduplicatesTreeAcrossMachines(t *testing.T) {
	root := soloLedgerRepo(t)
	now := time.Date(2026, 9, 17, 1, 0, 0, 0, time.UTC)
	first := trunkRedVerbReq(root, "01J5X0000000000000000000C1", "mac-a")
	first.Now = now
	acquired, err := ClaimCadence(first, cadenceKeyFixture(), time.Hour)
	if err != nil || acquired.Outcome != CadenceClaimAcquired || acquired.Claim == nil {
		t.Fatalf("first claim = %+v, %v", acquired, err)
	}

	second := trunkRedVerbReq(root, "01J5X0000000000000000000C2", "mac-b")
	second.Now = now.Add(30 * time.Minute)
	joined, err := ClaimCadence(second, cadenceKeyFixture(), time.Hour)
	if err != nil || joined.Outcome != CadenceClaimJoined || joined.Claim == nil || joined.Claim.Opid != first.opid() || joined.Claim.OwnerMachine != "mac-a" {
		t.Fatalf("second machine claim = %+v, %v", joined, err)
	}
	projection, err := ProjectAt(root, joined.Tip, second.Now)
	if err != nil || projection.Tree.CadenceClaim == nil || projection.Tree.CadenceClaim.Opid != first.opid() {
		t.Fatalf("shared claim = %+v, %v", projection.Tree.CadenceClaim, err)
	}

	publish := trunkRedVerbReq(root, "01J5X0000000000000000000C9", "mac-a")
	publish.Now = now.Add(31 * time.Minute)
	status := cadenceStatusFixture(cadenceKeyFixture(), now, "passed")
	published, err := PublishCadence(publish, CadencePublishArgs{ClaimOpid: acquired.Claim.Opid, Status: status})
	if err != nil || published.Outcome != OutcomeConfirmed {
		t.Fatalf("terminal cadence = %+v, %v", published, err)
	}
	reader := trunkRedVerbReq(root, "01J5X0000000000000000000CA", "mac-c")
	reader.Now = now.Add(32 * time.Minute)
	complete, err := ClaimCadence(reader, cadenceKeyFixture(), time.Hour)
	if err != nil || complete.Outcome != CadenceClaimComplete || complete.Status == nil || !complete.Status.Green() {
		t.Fatalf("terminal claim read = %+v, %v", complete, err)
	}
}

func TestCadenceRedPublishesOwnerlessUntilHumanNamesGoal(t *testing.T) {
	root := soloLedgerRepo(t)
	now := time.Date(2026, 9, 17, 2, 0, 0, 0, time.UTC)
	claimRequest := trunkRedVerbReq(root, "01J5X0000000000000000000C3", "mac-a")
	claimRequest.Now = now
	claim, err := ClaimCadence(claimRequest, cadenceKeyFixture(), time.Hour)
	if err != nil || claim.Outcome != CadenceClaimAcquired {
		t.Fatalf("claim = %+v, %v", claim, err)
	}

	publishRequest := trunkRedVerbReq(root, "01J5X0000000000000000000C4", "mac-a")
	publishRequest.Now = now.Add(time.Minute)
	status := cadenceStatusFixture(cadenceKeyFixture(), now, "failed")
	invalid := status
	invalid.Groups = append([]CadenceGroupStatus(nil), status.Groups...)
	invalid.Groups[0].EvidenceDigest = ""
	if _, invalidErr := PublishCadence(publishRequest, CadencePublishArgs{ClaimOpid: claim.Claim.Opid, Status: invalid}); invalidErr == nil {
		t.Fatal("cadence publication accepted a group without an evidence digest")
	}
	identity := "tr-section-deep-only-0123456789ab"
	result, err := PublishCadence(publishRequest, CadencePublishArgs{ClaimOpid: claim.Claim.Opid, Status: status,
		Groups: []TrunkRedRecordGroup{{Identity: identity, Group: "section/deep-only", Status: "failed",
			LogPath: "artifacts/deep-only.log", LogDigest: strings.Repeat("d", 64), Failures: []TrunkRedFailure{}}}})
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("publish cadence red = %+v, %v", result, err)
	}
	projection, err := ProjectAt(root, result.Tip, publishRequest.Now)
	if err != nil || projection.Tree.Cadence == nil || projection.Tree.Cadence.Green() || projection.Tree.CadenceClaim != nil || len(projection.Tree.TrunkRed) != 1 {
		t.Fatalf("cadence publication = %+v, %v", projection.Tree, err)
	}
	entry := projection.Tree.TrunkRed[0]
	if entry.Owner != (TrunkRedOwner{}) || entry.FixGoal != "" || len(entry.Holds) != 0 {
		t.Fatalf("cadence red acquired ownership or a batch hold: %+v", entry)
	}

	human := trunkRedVerbReq(root, "01J5X0000000000000000000C5", "mac-human")
	human.Now, human.Actor.Human = now.Add(2*time.Minute), "Wido"
	owned, err := OwnTrunkRed(human, TrunkRedOwnArgs{Entry: entry.ID, Goal: "solo-goal", To: "mac-fix", By: "Wido"})
	if err != nil || owned.Outcome != OutcomeConfirmed {
		t.Fatalf("human names existing goal = %+v, %v", owned, err)
	}
	projection, err = ProjectAt(root, owned.Tip, human.Now)
	entry = projection.Tree.TrunkRed[0]
	if err != nil || entry.FixGoal != "solo-goal" || entry.Owner.Machine != "mac-fix" || entry.Owner.How != "hand" || entry.Owner.By != "Wido" || projection.Tree.Cadence == nil {
		t.Fatalf("human ownership = %+v cadence=%+v, %v", entry, projection.Tree.Cadence, err)
	}
}

func TestCadenceStateAbsentLeavesLedgerBytesUnchanged(t *testing.T) {
	root := soloLedgerRepo(t)
	seed := publishTrunkRed(t, root, []TrunkRedEntry{})
	before, err := goalGit(root, nil, "show", seed.Tip+":"+trunkRedPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeDigest := sha256.Sum256([]byte(before))

	request := trunkRedVerbReq(root, "01J5X0000000000000000000C6", "mac-a")
	request.Now = time.Date(2026, 9, 17, 3, 0, 0, 0, time.UTC)
	result, err := RecordTrunkRed(request, TrunkRedRecordArgs{})
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("legacy no-op write = %+v, %v", result, err)
	}
	after, err := goalGit(root, nil, "show", result.Tip+":"+trunkRedPath)
	if err != nil {
		t.Fatal(err)
	}
	afterDigest := sha256.Sum256([]byte(after))
	projection, projectErr := ProjectAt(root, result.Tip, request.Now)
	if before != after || beforeDigest != afterDigest || projectErr != nil || projection.Tree.Cadence != nil || projection.Tree.CadenceClaim != nil {
		t.Fatalf("legacy ledger changed: before=%x after=%x cadence=%+v claim=%+v project=%v", beforeDigest, afterDigest, projection.Tree.Cadence, projection.Tree.CadenceClaim, projectErr)
	}
}

func TestCadenceDeadClaimIsRecoverableAfterLease(t *testing.T) {
	root := soloLedgerRepo(t)
	now := time.Date(2026, 9, 17, 4, 0, 0, 0, time.UTC)
	first := trunkRedVerbReq(root, "01J5X0000000000000000000C7", "mac-a")
	first.Now = now
	dead, err := ClaimCadence(first, cadenceKeyFixture(), 10*time.Minute)
	if err != nil || dead.Outcome != CadenceClaimAcquired {
		t.Fatalf("initial claim = %+v, %v", dead, err)
	}

	recovery := trunkRedVerbReq(root, "01J5X0000000000000000000C8", "mac-b")
	recovery.Now = now.Add(10*time.Minute + time.Second)
	recovered, err := ClaimCadence(recovery, cadenceKeyFixture(), time.Hour)
	if err != nil || recovered.Outcome != CadenceClaimAcquired || recovered.Claim == nil || recovered.Claim.Opid != recovery.opid() || recovered.Claim.OwnerMachine != "mac-b" {
		t.Fatalf("recovered claim = %+v, %v", recovered, err)
	}
}
