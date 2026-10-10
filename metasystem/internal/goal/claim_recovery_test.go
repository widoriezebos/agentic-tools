package goal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type recoveryProber struct{ state identity.Liveness }

func (p recoveryProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{Pid: pid, StartedAt: time.Unix(99, 0)}, p.state, nil
}

func TestGoalClaimTakeOverWarnsForLiveOrUnknownHolder(t *testing.T) {
	t.Parallel()
	for _, state := range []identity.Liveness{identity.Dead, identity.Alive, identity.Unknown} {
		t.Run(state.String(), func(t *testing.T) {
			t.Parallel()
			endpoint, _ := fakeGoalEndpoint(t)
			holder := verbReqFor(endpoint, "01J5X00000000000000000DR00", "mac-a")
			if result, err := openClaimForTest(t, holder, "recover", "Recover a dead session.", OriginMain, "Build.", testBudget()); err != nil || result.Outcome != OutcomeConfirmed {
				t.Fatalf("claim: %+v %v", result, err)
			}
			dir := filepath.Join(endpoint.Root, "artifacts", "agents", "mains")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(map[string]any{"mainId": "holder", "ownerLineage": holder.Actor.Lineage, "pid": 99, "pidStartedAt": 99})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "holder.json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			person := personalRequest(t, endpoint, "01J5X00000000000000000DR10")
			person.Actor.Lineage = "recovering"
			person.claimHolderProber = recoveryProber{state}
			result, err := StealWithReason(person, "recover", "The original session ended.")
			if err != nil {
				t.Fatal(err)
			}
			tree, _ := acceptedTreeForEndpoint(t, endpoint)
			file := tree.Live["recover"]
			if result.Outcome != OutcomeConfirmed || file.Claimed.Lineage != "recovering" || file.Claimed.By != "human:Wido" {
				t.Fatalf("holder not reassigned as a person: %+v %+v", result, file)
			}
			recordedReason := file.History[len(file.History)-1].Reason
			warnings := strings.Join(file.Claimed.Warnings, "; ")
			if state == identity.Dead {
				if !strings.Contains(recordedReason, "The original session ended.") || strings.Contains(recordedReason, "the holding session") || strings.Contains(warnings, "the holding session") {
					t.Fatalf("dead holder warned: reason=%q warnings=%q", recordedReason, warnings)
				}
			} else {
				status := "is live"
				if state == identity.Unknown {
					status = "cannot be proven dead"
				}
				warning := "the holding session " + holder.Actor.Lineage + " on mac-a " + status + " (pid 99, start 99): its further writes to the goal are refused by the claim check"
				if !strings.Contains(recordedReason, warning) || !strings.Contains(warnings, warning) || !strings.Contains(recordedReason, "The original session ended.") {
					t.Fatalf("take-over warning not recorded: reason=%q warnings=%q", recordedReason, warnings)
				}
			}
		})
	}
}
