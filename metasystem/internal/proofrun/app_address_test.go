package proofrun

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// The launch contract's check reaches this runner through one declared
// input. It must arrive in the group's environment in BOTH environment
// modes: an explicit group builds its environment from the contract alone,
// and a check whose address never reached it would run against nothing.
func TestAppAddressReachesAGroupInBothEnvironmentModes(t *testing.T) {
	for _, mode := range []string{"", "explicit"} {
		group := testpolicy.Group{ID: "app-smoke", Adapter: "command", EnvironmentMode: mode,
			Env: map[string]string{"FIXED": "1"}}
		request := TestRunRequest{Environment: []string{"PATH=/bin"}, AppAddress: "127.0.0.1:7981"}
		joined := strings.Join(groupTestEnvironment(request, group), "\n")
		if !strings.Contains(joined, AppAddressEnvironment+"=127.0.0.1:7981") {
			t.Fatalf("environment mode %q must carry the run's address:\n%s", mode, joined)
		}
	}
}

// A group may no more set the address than the worker count: it is the
// runner's to say, and a group that set it would decide what it is checked
// against.
func TestAGroupCannotSetTheAppAddressItself(t *testing.T) {
	group := testpolicy.Group{ID: "app-smoke", Env: map[string]string{testpolicy.AppAddressEnvironment: "127.0.0.1:1"}}
	request := TestRunRequest{Workers: 1, Contract: testpolicy.Contract{Groups: []testpolicy.Group{group}}}
	err := ValidateTestWorkerRequest(request)
	if err == nil || !strings.Contains(err.Error(), testpolicy.AppAddressEnvironment) {
		t.Fatalf("a group that sets the reserved address must be refused by the runner, got %v", err)
	}
}

// The address is part of the group's execution identity, so a result for one
// application run is never reused for a run at another address.
func TestTheAppAddressIsPartOfAGroupsIdentity(t *testing.T) {
	for _, mode := range []string{"", "explicit"} {
		group := testpolicy.Group{ID: "app-smoke", EnvironmentMode: mode, Env: map[string]string{"FIXED": "1"}}
		first := digestGroupEnvironment(group, groupTestEnvironment(
			TestRunRequest{Environment: []string{"PATH=/bin"}, AppAddress: "127.0.0.1:7981"}, group))
		second := digestGroupEnvironment(group, groupTestEnvironment(
			TestRunRequest{Environment: []string{"PATH=/bin"}, AppAddress: "127.0.0.1:7982"}, group))
		if first == second {
			t.Fatalf("environment mode %q: two addresses must be two identities", mode)
		}
	}
}
