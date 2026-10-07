package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

func TestProofRestampReadsLiveHolderThroughSuppliedLedger(t *testing.T) {
	t.Parallel()
	repository, now := proofAdmissionExtensionFixture(t)
	root := repository.root
	ref, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe caller: %s %v", state, err)
	}
	if _, err := lease.AnnounceWithPair(root, "proof-holder", ref.Pid, ref.StartedAt.Unix(),
		ref.StartTicks, ref.BootID, "proof-holder", "fake", "m1"); err != nil {
		t.Fatal(err)
	}
	leasePath := filepath.Join(root, "artifacts", "agents", "mains", "worktree-lease.json")
	data, err := os.ReadFile(leasePath)
	if err != nil {
		t.Fatal(err)
	}
	var holder lease.Lease
	if err := json.Unmarshal(data, &holder); err != nil {
		t.Fatal(err)
	}
	holder.ClaimEpoch, holder.Revision = 5, holder.Revision+1
	data, err = json.Marshal(holder)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(leasePath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	endpoint, err := repository.reads().ResolveEndpoint(root)
	if err != nil {
		t.Fatal(err)
	}
	before, err := goal.Project(endpoint, false, now)
	if err != nil {
		t.Fatal(err)
	}
	request := goal.VerbRequest{Endpoint: endpoint, Actor: goal.Actor{Machine: "mac-cli", Lineage: "m1"},
		Ulid: "01J5X00000000000000000RH10", Now: now, ClaimEpoch: 6, CallerClass: lease.ClassMain, EpochAuthority: goal.EpochAuthorityHolder}
	result, err := goal.Restamp(request, "standing-validation")
	if err != nil || result.Outcome != goal.OutcomeRejected || result.Code != "REBIND_EPOCH_UNAUTHENTICATED" {
		t.Fatalf("invented holder epoch admitted: %+v %v", result, err)
	}
	request.Ulid, request.ClaimEpoch = "01J5X00000000000000000RH20", 5
	result, err = goal.Restamp(request, "standing-validation")
	if err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("live holder restamp: %+v %v", result, err)
	}
	after, err := goal.Project(endpoint, false, now)
	if err != nil {
		t.Fatal(err)
	}
	original, renewed := before.Tree.Live["standing-validation"], after.Tree.Live["standing-validation"]
	if renewed.StopCapability == nil || renewed.StopCapability.ClaimEpoch != 5 ||
		!reflect.DeepEqual(original.Claimed, renewed.Claimed) || !reflect.DeepEqual(original.Approved, renewed.Approved) ||
		!reflect.DeepEqual(original.Budget, renewed.Budget) {
		t.Fatalf("restamp lost claim authority or changed accounting: %+v", renewed)
	}
}
