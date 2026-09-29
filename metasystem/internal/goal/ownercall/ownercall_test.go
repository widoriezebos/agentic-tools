package ownercall

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type fixedProber struct {
	exact identity.Exact
	state identity.Liveness
	err   error
}

func (p fixedProber) Probe(int64) (identity.Exact, identity.Liveness, error) {
	return p.exact, p.state, p.err
}

// A supplied pid is classifiable while its recorded start time still names
// the live process; a reused or vanished pid is refused, and an identity
// without a start time (or without a prober) is taken as supplied.
func TestClassifiablePidRefusesAReusedPid(t *testing.T) {
	t.Parallel()
	started := time.Unix(1_700_000_000, 0)
	supplied := Process{Pid: 42, StartedAt: started.Unix()}
	same := fixedProber{exact: identity.Exact{Pid: 42, StartedAt: started}, state: identity.Alive}
	if pid, err := supplied.ClassifiablePid(same); err != nil || pid != 42 {
		t.Fatalf("the same process = %d, %v", pid, err)
	}
	reused := fixedProber{exact: identity.Exact{Pid: 42, StartedAt: started.Add(time.Second)}, state: identity.Alive}
	if _, err := supplied.ClassifiablePid(reused); err == nil || !strings.Contains(err.Error(), "is no longer that process") {
		t.Fatalf("a reused pid = %v; want the refusal", err)
	}
	gone := fixedProber{state: identity.Dead}
	if _, err := supplied.ClassifiablePid(gone); err == nil {
		t.Fatal("a vanished pid was classifiable")
	}
	unreadable := fixedProber{state: identity.Unknown, err: errors.New("unreadable")}
	if _, err := supplied.ClassifiablePid(unreadable); err == nil || !strings.Contains(err.Error(), "unreadable") {
		t.Fatalf("an unreadable pid = %v", err)
	}
	if pid, err := (Process{Pid: 7}).ClassifiablePid(reused); err != nil || pid != 7 {
		t.Fatalf("an identity without a start time = %d, %v", pid, err)
	}
	if pid, err := supplied.ClassifiablePid(nil); err != nil || pid != 42 {
		t.Fatalf("no prober = %d, %v", pid, err)
	}
}

// This process supplies itself with its kernel start time; a process entry
// supplies its parent; an edge that replaced a child carries its lineage.
func TestTheSuppliedIdentities(t *testing.T) {
	t.Parallel()
	current := CurrentProcess()
	if current.Pid != int64(os.Getpid()) || current.StartedAt < 1 {
		t.Fatalf("CurrentProcess = %+v", current)
	}
	if caller := EntryCaller(); caller.Pid != int64(os.Getppid()) || caller.StartedAt != 0 {
		t.Fatalf("EntryCaller = %+v", caller)
	}
	invocation := FromThisProcess("landing-m1l")
	if invocation.Lineage != "landing-m1l" || invocation.Caller.Pid != int64(os.Getpid()) {
		t.Fatalf("FromThisProcess = %+v", invocation)
	}
}

// A handover names its goal, its whole target pair at a positive epoch, and
// its batch; anything less is refused with the words to supply.
func TestHandoverRequestUsageNamesEveryPart(t *testing.T) {
	t.Parallel()
	complete := HandoverRequest{GoalID: "g", TargetMachine: "m", TargetLineage: "l", TargetEpoch: 1, Batch: "b"}
	if problem := complete.Usage(); problem != "" {
		t.Fatalf("a complete handover = %q", problem)
	}
	for _, missing := range []func(*HandoverRequest){
		func(r *HandoverRequest) { r.GoalID = "" },
		func(r *HandoverRequest) { r.TargetMachine = "" },
		func(r *HandoverRequest) { r.TargetLineage = "" },
		func(r *HandoverRequest) { r.TargetEpoch = 0 },
		func(r *HandoverRequest) { r.Batch = "" },
	} {
		request := complete
		missing(&request)
		if problem := request.Usage(); !strings.HasPrefix(problem, "goal handover needs --id") {
			t.Fatalf("an incomplete handover %+v = %q", request, problem)
		}
	}
}

// A confirmed publish is success; a refusal and an unconfirmed outcome
// become the errors the former child's exit carried.
func TestPublishErrorNamesTheVerbAndOutcome(t *testing.T) {
	t.Parallel()
	if err := PublishError("edit", goal.PublishResult{Outcome: goal.OutcomeConfirmed}, nil); err != nil {
		t.Fatalf("confirmed = %v", err)
	}
	refused := errors.New("refused")
	if err := PublishError("edit", goal.PublishResult{}, refused); !errors.Is(err, refused) || !strings.HasPrefix(err.Error(), "goal edit: ") {
		t.Fatalf("refusal = %v", err)
	}
	err := PublishError("release", goal.PublishResult{Outcome: "lost", Tip: "abc", Detail: "raced"}, nil)
	if err == nil || err.Error() != "goal release: outcome=lost tip=abc detail=raced" {
		t.Fatalf("unconfirmed = %v", err)
	}
}
