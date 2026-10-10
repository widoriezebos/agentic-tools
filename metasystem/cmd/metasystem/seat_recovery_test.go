package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

func TestWorkStopFinishedDeadMutationWithUnreadableCustody(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"cancelled", "completed", "running"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			b, owners, _ := rebaseIntentBed(t)
			runner := &launch.UnitRunner{Root: b.unitRoot, Manager: b.manager, Git: workGit{b}}
			if _, err := runner.ReserveMutation(b.worktree, b.id, "rebase"); err != nil {
				t.Fatal(err)
			}
			paths, err := filepath.Glob(filepath.Join(b.unitRoot, "*", "run.json"))
			if err != nil || len(paths) != 1 {
				t.Fatalf("mutation records: %v %v", paths, err)
			}
			data, err := os.ReadFile(paths[0])
			if err != nil {
				t.Fatal(err)
			}
			var record launch.UnitRunRecord
			if err := json.Unmarshal(data, &record); err != nil {
				t.Fatal(err)
			}
			record.State = state
			if state != "running" {
				record.Mutation = &identity.Ref{Pid: 99, StartedAtSec: 99}
			}
			data, err = json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(paths[0], data, 0600); err != nil {
				t.Fatal(err)
			}
			jobs := filepath.Join(b.worktree, "artifacts", "agents", "jobs")
			if err := os.MkdirAll(jobs, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(jobs, "broken.json"), []byte("{broken"), 0600); err != nil {
				t.Fatal(err)
			}
			now, err := b.commandNow(b.root())
			if err != nil {
				t.Fatal(err)
			}
			owners.prove = enrolledPersonProver(t, b.root(), now)
			code, result := b.runJSON(owners, "work", "stop", "run:"+record.ID)
			reservations, err := filepath.Glob(filepath.Join(b.unitRoot, ".trees", "*.json"))
			if err != nil {
				t.Fatal(err)
			}
			if state == "running" {
				if code != 1 || len(reservations) != 1 || !strings.Contains(result.Summary, "pid 10, start 10") {
					t.Fatalf("live mutation not named/retained: %d %+v %v", code, result, reservations)
				}
			} else if code != 0 || len(reservations) != 0 || !strings.Contains(result.Summary, "worktree is released") {
				t.Fatalf("finished mutation not released: %d %+v %v", code, result, reservations)
			}
		})
	}
}

func TestHeldElsewhereUnclaimedGoalGivesPlainClaimRemedy(t *testing.T) {
	t.Parallel()
	bed, owners, _ := rebaseIntentBed(t)
	file := bed.goalFile(bedGoal)
	file.State, file.Claimed, file.StopCapability, file.StopFence = goal.StateApproved, nil, nil, nil
	bed.addGoal(file)
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("self identity: %s %v", state, err)
	}
	if _, err := lease.AnnounceWithPair(bed.root(), "writer", exact.Pid, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "writer", "fake", "writer"); err != nil {
		t.Fatal(err)
	}
	owners.connection.endpoint = func(string) (goal.Endpoint, error) { return owners.dependencies.endpoint(bed.root()) }
	owners.connection.claimCheck = func(root, id string, endpoint goal.Endpoint) func() error {
		return goalBranchClaimCheckWith(root, id, endpoint,
			func(string, string) (string, error) { return "m1", nil }, func(string) string { return bed.root() })
	}
	// The branch write verb renders the claim owner's refusal before publishing.
	code, result := bed.runJSON(owners, "work", "commit", bedGoal, "--work", "unit")
	if code == 0 || !strings.Contains(result.Summary, "nobody does") || !strings.Contains(resultWords(result), "metasystem goal claim "+bedGoal) || strings.Contains(resultWords(result), "--take-over") {
		t.Fatalf("unclaimed goal remedy: %d %+v", code, result)
	}
}

func TestGoalClaimTakeOverWarnsForLiveOrUnknownSession(t *testing.T) {
	t.Parallel()
	for _, liveness := range []identity.Liveness{identity.Dead, identity.Alive, identity.Unknown} {
		t.Run(liveness.String(), func(t *testing.T) {
			t.Parallel()
			bed, owners := personClaimsBed(t)
			reservePersonGoal(t, bed, owners, bedGoal)
			holder := bed.goalFile(bedGoal).Claimed
			exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
			if err != nil || state != identity.Alive {
				t.Fatalf("self identity: %s %v", state, err)
			}
			ref := exact.Ref()
			if liveness == identity.Dead {
				ref.Pid = 2147483647
			} else if liveness == identity.Unknown {
				// A live pid without a recorded start cannot establish identity.
				ref = identity.Ref{Pid: exact.Pid}
			}
			if got := identity.LiveRef(identity.KernelProber{}, ref); got != liveness {
				t.Fatalf("holding process fixture: got %s, want %s", got, liveness)
			}
			dir := filepath.Join(bed.root(), "artifacts", "agents", "mains")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(map[string]any{"mainId": "held-main", "ownerLineage": holder.Lineage,
				"pid": ref.Pid, "pidStartedAt": ref.StartedAtSec, "pidStartedAtExactMicro": ref.StartedAtUnixMicro,
				"pidStartTicks": ref.StartTicks, "bootId": ref.BootID})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "held-main.json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			owners.dependencies.claimHolder.Holder = func(string) (lease.CurrentHolderView, error) {
				return lease.CurrentHolderView{MainId: "recovering-main", OwnerLineage: "recovering-session"}, nil
			}
			before := bed.publications()
			code, result := bed.runJSON(owners, "goal", "claim", bedGoal, "--take-over", "--reason", "The former session ended.", "--by", "Wido", "--fixture-human-authority")
			file := bed.goalFile(bedGoal)
			if code != 0 || bed.publications() <= before || file.Claimed.Lineage != "recovering-session" || file.Claimed.By != "human:Wido" {
				t.Fatalf("session not taken over as person: %d %+v %+v", code, result, file.Claimed)
			}
			recordedReason := file.History[len(file.History)-1].Reason
			if liveness == identity.Dead {
				if !strings.Contains(recordedReason, "The former session ended.") || strings.Contains(recordedReason, "the holding session") || strings.Contains(resultWords(result), "the holding session") {
					t.Fatalf("dead session warned: result=%+v reason=%q", result, recordedReason)
				}
			} else {
				status := "is live"
				if liveness == identity.Unknown {
					status = "cannot be proven dead"
				}
				warning := fmt.Sprintf("the holding session %s on %s %s (pid %d, start %d): its further writes to the goal are refused by the claim check", holder.Lineage, holder.Machine, status, ref.Pid, ref.StartedAtSec)
				if !strings.Contains(result.Summary, warning) || !strings.Contains(recordedReason, warning) || !strings.Contains(recordedReason, "The former session ended.") {
					t.Fatalf("take-over warning missing: result=%+v reason=%q", result, recordedReason)
				}
			}
		})
	}
}

func TestWorkStopNamesLiveCriticProcess(t *testing.T) {
	t.Parallel()
	b, owners, run, _ := criticCustodyBed(t)
	writeCustodyCritic(t, b, "critic", "completed")
	owners.work.cancelCritic = func(root, job string) (map[string]any, int, error) {
		writeCustodyCritic(t, b, job, "cancelled")
		return nil, 0, nil
	}
	code, result := b.runJSON(owners, "work", "stop", "run:"+run)
	if code != 1 || !strings.Contains(result.Summary, "critic critic process pid 20, start 400") {
		t.Fatalf("live critic not identified: %d %+v", code, result)
	}
}
