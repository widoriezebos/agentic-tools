package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	landinglane "github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

// admitAsLaneOwner admits a launch as the lane checkout's lease holder does.
func admitAsLaneOwner(t *testing.T, r *proofAdmissionRepository, request proofLaunchAdmission, class string) (proofrun.Attempt, proofrun.LaunchResult, bool, error) {
	t.Helper()
	request.BeforePublish = func(reservation *proofrun.AdmissionRequest) {
		*reservation = privateProofAdmissionRequest(proofrun.WithTestHostLoadSampler(*reservation, "0"))
	}
	reads := r.reads()
	classify := func(string, int64) (lease.ClassifyResult, error) {
		return lease.ClassifyResult{Class: class, Holder: class == lease.ClassMain}, nil
	}
	return admitProofLaunchWithReadsAndClassifier(candidateProofLaunchAdmission(request), func() dispatchcore.ProofAdmissionReads { return reads }, classify)
}

// TestLaneProofChargesTheLaneNotAGoal (U11b): a proof launched with the
// lane's accounting identity instead of a goal is admitted with the lane as
// the attempt's owner, and no goal's budget moves; a lane identity that does
// not resolve refuses before anything is reserved, and a lane proof never
// names a goal too.
func TestLaneProofChargesTheLaneNotAGoal(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)
	repository := newProofAdmissionRepositoryFixture(t, now, false)
	root := repository.root
	file := repository.goalFile(t, "standing-validation")
	before := dispatchcore.ProjectBudget(root, file, now)
	const account = "lane:0123456789ab"
	request := proofLaunchAdmission{ControlRoot: root, ExecutionRoot: root, ConfPath: filepath.Join(root, "metasystem.conf"), LaneID: account,
		CapMin: "1", ScopeClass: "full", CommandClass: "testing", Now: now,
		laneAccount: func(string) (string, error) { return account, nil },
		laneOwner:   func(string, int64) error { return nil }}
	attempt, decision, joined, err := admitAsLaneOwner(t, repository, request, lease.ClassMain)
	if err != nil || joined || decision.Disposition != proofrun.DispositionExecuted || attempt.GoalID != account || attempt.AccountedGoal() != account {
		t.Fatalf("lane proof: attempt=%+v decision=%+v joined=%v err=%v", attempt, decision, joined, err)
	}
	after := dispatchcore.ProjectBudget(root, repository.goalFile(t, "standing-validation"), now)
	if after.Attempts != before.Attempts || after.ReservedJobMinutes != before.ReservedJobMinutes {
		t.Fatalf("a lane proof moved a goal budget: before=%+v after=%+v", before, after)
	}
	retained, err := proofrun.ReadAttempts(root)
	if err != nil || len(retained) != 1 || retained[0].GoalID != account {
		t.Fatalf("retained attempts=%+v err=%v", retained, err)
	}

	unresolved := request
	unresolved.laneAccount = func(string) (string, error) {
		return "", errors.New("LANE_ACCOUNT_UNRESOLVED: no landing lane is registered on this host")
	}
	if _, _, _, err := admitAsLaneOwner(t, repository, unresolved, lease.ClassMain); err == nil || !strings.Contains(refusalDetail(err), "LANE_ACCOUNT_UNRESOLVED") {
		t.Fatalf("unresolved lane: %v", err)
	}
	other := request
	other.laneAccount = func(string) (string, error) { return "lane:ffffffffffff", nil }
	if _, _, _, err := admitAsLaneOwner(t, repository, other, lease.ClassMain); err == nil || !strings.Contains(refusalDetail(err), "LANE_ACCOUNT_UNRESOLVED") {
		t.Fatalf("another lane's identity: %v", err)
	}
	both := request
	both.GoalID = "standing-validation"
	if _, _, _, err := admitAsLaneOwner(t, repository, both, lease.ClassMain); err == nil || !strings.Contains(err.Error(), "--goal") {
		t.Fatalf("lane and goal together: %v", err)
	}
	// A seat coordinator holding a checkout that happens to be the lane is not
	// its owner process (N-1).
	seat := request
	seat.laneOwner = func(string, int64) error {
		return errors.New("the lane checkout is held by lineage m1e-coordinator, not its landing owner")
	}
	if _, _, _, err := admitAsLaneOwner(t, repository, seat, lease.ClassMain); err == nil || !strings.Contains(err.Error(), "owner process") {
		t.Fatalf("a seat coordinator charged the lane: %v", err)
	}
}

// TestTrustedPolicyEngineForwardsTheLane (U11b F-3): the pinned trusted-base
// engine plans a lane-charged run on the lane, never on some goal; an engine
// that answers without its envelope is refused, never asked with a goal.
func TestTrustedPolicyEngineForwardsTheLane(t *testing.T) {
	t.Parallel()
	argsFile := filepath.Join(t.TempDir(), "args")
	engine := filepath.Join(t.TempDir(), "policy-engine")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > '" + argsFile + "'\nprintf '%s\\n' '{\"schemaVersion\":1,\"verb\":\"internal test plan\",\"targets\":[],\"outcome\":\"confirmed\",\"summary\":\"\",\"data\":{\"schemaVersion\":1}}'\n"
	if err := testexec.WriteFile(engine, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	request := testrun.SelectionRequest{Mode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery, LaneID: "lane:0123456789ab"}
	if _, err := testrun.PlanWithTrustedPolicyEngine(engine, request, t.TempDir(), "candidate"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Fields(string(data))
	if strings.Join(args[:3], " ") != "internal test plan" || !slices.Contains(args, "--lane") || !slices.Contains(args, "lane:0123456789ab") || slices.Contains(args, "--goal") {
		t.Fatalf("policy child argv=%v", args)
	}
	// An engine that answers in words, not the envelope, is read as unknown:
	// the plan is refused, never guessed from its text (structured-output U1).
	old := filepath.Join(t.TempDir(), "old-engine")
	if err := testexec.WriteFile(old, []byte("#!/bin/sh\necho 'flag provided but not defined: -lane' >&2\nexit 2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := testrun.PlanWithTrustedPolicyEngine(old, request, t.TempDir(), "candidate"); err == nil || !strings.Contains(err.Error(), "could not be read") {
		t.Fatalf("an engine answering in words: %v", err)
	}
}

// TestLaneOwnerProofReadsTheLeaseTheOwnerHoldsOnANestedCheckout drives the
// owner-to-proof-child identity path with no seam doubled: on a checkout that
// nests the module, the supervised owner takes the checkout lease at the
// lane's toplevel (landingOwnerCheckoutRoot), while its tip proof runs with
// --control-root at the module root (batch.ModuleRoot). The proof's owner
// check must find the owner's lease there, and still refuse a process that
// does not descend from the owner.
func TestLaneOwnerProofReadsTheLeaseTheOwnerHoldsOnANestedCheckout(t *testing.T) {
	t.Setenv("METASYSTEM_OWNER_LINEAGE", "")
	t.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", t.TempDir())
	home, err := board.Home()
	if err != nil {
		t.Fatal(err)
	}
	checkout := t.TempDir()
	if err := os.MkdirAll(filepath.Join(checkout, "metasystem"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(checkout, "metasystem", "go.mod"), []byte("module example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	registerLane(t, home, checkout, "test", time.Now())
	held, err := batchowner.AcquireBatchOwnerForComponent(checkout)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := held.Retire(); err != nil {
			t.Errorf("retire owner: %v", err)
		}
	})
	controlRoot := batch.ModuleRoot(checkout)
	if controlRoot == checkout {
		t.Fatalf("fixture is not nested: control root %s", controlRoot)
	}
	if _, err := landinglane.ResolveAccount(home, controlRoot); err != nil {
		t.Fatalf("the proof's control root is not in the lane: %v", err)
	}
	if err := proveLaneOwnerCaller(controlRoot, int64(os.Getpid())); err != nil {
		t.Fatalf("the owner's own proof was refused: %v", err)
	}
	if err := proveLaneOwnerCaller(controlRoot, 1); err == nil || !strings.Contains(err.Error(), "does not descend") {
		t.Fatalf("a process outside the owner's lineage was accepted: %v", err)
	}
}

// The landing agent charges a proof to the lane by launch descent (lane
// design r10 K7): the lane checkout's lease is held under lineage
// landing-agent and the caller descends from that holder. A seat session
// holding the checkout is not the agent, and a process outside the agent's
// descent is refused.
func TestLaneAgentDescentChargesAProofToTheLane(t *testing.T) {
	t.Setenv("METASYSTEM_OWNER_LINEAGE", "")
	t.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", t.TempDir())
	home, err := board.Home()
	if err != nil {
		t.Fatal(err)
	}
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe: %s %v", state, err)
	}
	hold := func(lineage string) string {
		t.Helper()
		checkout := t.TempDir()
		if err := os.MkdirAll(filepath.Join(checkout, "metasystem"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(checkout, "metasystem", "go.mod"), []byte("module example\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		registerLane(t, home, checkout, "test", time.Now())
		if batch.ModuleRoot(checkout) == checkout {
			t.Fatalf("fixture is not nested: %s", checkout)
		}
		if _, err := lease.AnnounceWithPair(checkout, "session-"+lineage, int64(os.Getpid()), exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "lane-agent-test", "metasystem", lineage); err != nil {
			t.Fatal(err)
		}
		if holder, err := lease.RequireHolder(checkout, int64(os.Getpid()), nil); err != nil || !holder.Holder {
			t.Fatalf("hold the lane checkout as %s: %+v %v", lineage, holder, err)
		}
		return batch.ModuleRoot(checkout)
	}
	controlRoot := hold("steward-seat")
	if err := proveLaneOwnerCaller(controlRoot, int64(os.Getpid())); err == nil || !strings.Contains(err.Error(), "not by its landing agent") {
		t.Fatalf("a seat session holding the lane checkout charged the lane: %v", err)
	}
	controlRoot = hold(landinglane.AgentLineage)
	if err := proveLaneOwnerCaller(controlRoot, int64(os.Getpid())); err != nil {
		t.Fatalf("the landing agent's own proof was refused: %v", err)
	}
	if err := proveLaneOwnerCaller(controlRoot, 1); err == nil || !strings.Contains(err.Error(), "does not descend") {
		t.Fatalf("a process outside the agent's descent was accepted: %v", err)
	}
}
