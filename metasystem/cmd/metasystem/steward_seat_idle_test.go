package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func TestStewardSeatIdleByLineage(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, main, unit, job string
		busy, mismatch        bool
	}{
		{"unknown running unit", "", "running", "", true, false},
		{"unknown running critic", "", "", "running", true, false},
		{"unknown idle", "", "", "", false, false},
		{"different main", "other", "", "", false, true},
		{"matching main", "main-100-41-aaaaaa", "", "", false, false},
		{"completed work", "", "awaiting-judgement", "completed", false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if test.main == "" && steward.ClassifySeatStop(test.main, "main-100-41-aaaaaa") != "unknown" {
				t.Fatal("an empty Stop main must be unknown")
			}
			bed := newGoalCLIBed(t, goalCLISeed{remote: "local", amend: func(files map[string]*goal.GoalFile) {
				files["ship-widget"].Tier = 1
				files["ship-widget"].Budget = &goal.Budget{ElapsedLimit: "1d", AttemptLimit: 2, ReservedJobMinutesLimit: 120, ActiveJobLimit: 1}
				file := files["fix-docs"]
				file.State, file.Tier = goal.StateApproved, 1
				file.Revision = 2
				file.Budget = &goal.Budget{ElapsedLimit: "1d", AttemptLimit: 2, ReservedJobMinutesLimit: 120, ActiveJobLimit: 1}
				file.Approved = &goal.ApprovalRecord{By: "human:Wido", At: goalCLISeedNow.Format(time.RFC3339), Revision: 2, Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FA6", "fixture-machine", "fixture-lineage"), Authority: goal.ApprovalAuthorityProven, Digest: goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget)}
				file.History = append(file.History, goal.HistoryLine{At: file.Approved.At, Opid: file.Approved.Opid, Verb: "approve", Actor: file.Approved.By, Targets: []string{file.Id}, Keep: -1})
			}})
			write := func(relative string, value any) {
				data, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				gcliBrainWrite(t, filepath.Join(bed.root, relative), data)
			}
			write("artifacts/agents/mains/worktree-lease.json", map[string]any{"holderMainId": "main-100-41-aaaaaa", "ownerLineage": bed.lineage, "claimEpoch": 7, "revision": 1, "pid": 41})
			write("artifacts/agents/mains/main-100-41-aaaaaa.json", lease.Announcement{MainId: "main-100-41-aaaaaa", OwnerLineage: bed.lineage, SessionId: "session", CommandHash: strings.Repeat("a", 64), Pid: 41, PidStartedAt: 100, Pgid: 41, Runtime: "fake", InstanceTag: "fixture", AnnouncedAt: bed.clock().Format(time.RFC3339)})
			units := t.TempDir()
			if test.unit != "" {
				data := `{"id":"run-1","goal":"ship-widget","state":"` + test.unit + `","rounds":[{"steps":[{"name":"build","launchId":"launch-1","startedAt":"` + bed.clock().Add(-time.Minute).Format(time.RFC3339) + `","state":"running"}]}]}`
				gcliBrainWrite(t, filepath.Join(units, "run-1", "run.json"), []byte(data))
				store := launch.Store{Root: filepath.Join(filepath.Dir(units), "launch")}
				if err := store.Create(launch.Record{ID: "launch-1", State: launch.Running, Supervisor: &identity.Ref{Pid: 10, StartedAtSec: 10}}); err != nil {
					t.Fatal(err)
				}
			}
			if test.job != "" {
				write("artifacts/agents/jobs/code-critic-1.json", map[string]any{"goalId": "ship-widget", "status": test.job, "pid": 20, "pidStartedAt": 20})
			}
			store := &goal.Store{Root: bed.root, Now: bed.clock, Prober: seatTickProber{}, BootClock: func() (string, time.Duration, error) { return "boot", time.Hour, nil }}
			store.ObserveIdleSeat = observeSeatIdle(bed.root, test.main, func(root string, work goal.ClaimableBudgetedWork) (string, error) {
				busy, reason, _ := steward.SeatBusyAt(root, units, work, steward.SeatBusyOptions{Now: bed.clock(), Alive: func(ref identity.Ref) bool { return identity.AliveRef(seatTickProber{}, ref) == identity.Alive }})
				if busy {
					return reason, nil
				}
				return "", nil
			})
			store.ResolveIdleSeat = resolveSeatIdleActorWithMachine(bed.root, test.main, func(string) (string, error) { return bed.machine, nil })
			prepared := 0
			store.PrepareIdleContinuation = func(goal.IdleEscalationEvent) (string, error) { prepared++; return "intent", nil }
			store.RecordIdleIncident = func(goal.IdleEscalationEvent) (string, error) { return "incident", nil }
			store.RaiseIdleAlarm = func(event goal.IdleEscalationEvent) error { t.Fatalf("unexpected idle alarm: %+v", event); return nil }
			endpoint, _ := bed.endpoint(bed.root)
			sources := 0
			for stop := 0; stop < 3; stop++ {
				verdict, err := store.TurnVerdictAtEndpoint(endpoint, bed.machine, goal.ScanResult{}, "session", "", test.main, goal.TurnVerdictOptions{StopHookActive: true})
				if err != nil {
					t.Fatal(err)
				}
				if verdict.IdleRefusal != (!test.busy && !test.mismatch) {
					t.Fatalf("stop %d: %+v", stop, verdict)
				}
				if strings.Contains(verdict.Display, "source:") {
					sources++
				}
				if test.main == "" && !test.busy && stop == 0 && !strings.Contains(verdict.Display, "unknown; source: checkout main record; holder main-100-41-aaaaaa") {
					t.Fatalf("missing holder source: %s", verdict.Display)
				}
			}
			wantSources, wantPrepared := 1, 1
			if test.busy {
				wantSources, wantPrepared = 0, 0
			}
			if test.mismatch {
				wantPrepared = 0
			}
			if sources != wantSources || prepared != wantPrepared {
				t.Fatalf("source lines=%d, continuations=%d", sources, prepared)
			}
			data, err := os.ReadFile(filepath.Join(bed.root, "artifacts/agents/turn-verdict-state.json"))
			if err != nil {
				t.Fatal(err)
			}
			var state struct {
				Sessions map[string]struct{ IdleBlocks int }
			}
			if err := json.Unmarshal(data, &state); err != nil {
				t.Fatal(err)
			}
			if (test.busy || test.mismatch) && state.Sessions["session"].IdleBlocks != 0 {
				t.Fatal("idle refusal was counted")
			}
		})
	}
}
