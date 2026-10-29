package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

var _ = addLayoutCases(layoutCase{name: "machine-revive-refusal", args: []string{"machine", "revive"}, bed: helpLayoutBed})

func TestMachineRevivePublicAct(t *testing.T) {
	t.Parallel()
	if os.Getenv("MACHINE_REVIVE_ISOLATED") == "" {
		tools, home := t.TempDir(), t.TempDir()
		// Only version-control process answers are replaced; every caller uses
		// production goal parsing, authority, arbitration and launch storage.
		git := `#!/bin/sh
root=$(/bin/pwd -P)
while [ "$#" -gt 0 ]; do
 case "$1" in -C) root=$2; shift 2 ;; -c) shift 2 ;; *) break ;; esac
done
case "$*" in
 'config --get metasystem.goal.machine') echo mac-cli ;;
 'config --get goal.sync-remote') echo local ;;
 config*) exit 1 ;;
 'rev-parse --show-toplevel') echo "$root" ;;
 'rev-parse --verify'*|'rev-parse HEAD') echo 0000000000000000000000000000000000000001 ;;
 'ls-tree --name-only'*) printf 'plans/goals/backlog.md\n' ;;
 'ls-tree -r --name-only'*) printf 'plans/goals/backlog.md\nplans/goals/standing-validation.md\n' ;;
 'cat-file --batch') while IFS= read -r input; do path=${input#*:}; path=${path#./}; size=$(/usr/bin/wc -c < "$root/$path"); printf '0000000000000000000000000000000000000001 blob %s\n' "$size"; /bin/cat "$root/$path"; printf '\n'; done ;;
 'cat-file -p '*) path=${3#*:}; /bin/cat "$root/${path#./}" ;;
 'cat-file -e '*) exit 0 ;;
 'show -s --format=%cI '*) echo 2026-09-01T09:55:00Z ;;
 *) exit 128 ;;
esac
`
		if err := testexec.WriteFile(filepath.Join(tools, "git"), []byte(git), 0700); err != nil {
			t.Fatal(err)
		}
		selection := "^TestMachineRevivePublicAct$"
		parts, depth := strings.Split(flag.Lookup("test.run").Value.String(), "/"), strings.Count(t.Name(), "/")+1
		if len(parts) > depth {
			selection += "/" + strings.Join(parts[depth:], "/")
		}
		command := exec.CommandContext(t.Context(), os.Args[0], "-test.run="+selection, "-test.timeout="+fixtureDeadlineRemaining(t).String(), "-test.v")
		for _, entry := range os.Environ() {
			name, _, _ := strings.Cut(entry, "=")
			if name != "PATH" && name != "METASYSTEM_SUPERVISION_REGISTRY_HOME" && name != "METASYSTEM_TESTENV_REGISTRY_NONCE" {
				command.Env = append(command.Env, entry)
			}
		}
		command.Env = append(command.Env, "PATH="+tools+":"+os.Getenv("PATH"), "MACHINE_REVIVE_ISOLATED=1", "METASYSTEM_SUPERVISION_REGISTRY_HOME="+home)
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("public machine revival: %v\n%s", err, out)
		}
		t.Logf("public machine revival:\n%s", out)
		return
	}
	for _, scenario := range []string{"closed-stranded-continuation", "closed-live-continuation", "closed-not-held", "closed-completed", "person", "omitted", "agent", "stale", "stopped", "unreadable", "missing-episode", "invalid-finished", "unreadable-episode", "automatic-missing-episode", "automatic-invalid-finished", "automatic-old-interval", "missing-launch", "missing-launch-live", "older-start-failed", "launcher-refused", "reservation", "response-loss", "automatic"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed := newWorkBedWith(t, func(f *goal.GoalFile) {
				workApprovedBox(f)
				f.Claimed.Lineage = steward.SeatLineage
				if scenario == "closed-not-held" {
					f.Claimed.Lineage = "other-seat"
				}
			})
			root, home, now := bed.root(), bed.manager.CapacityHome, bed.manager.Now()
			bed.manager.Adapters["codex-exec"] = launch.CodexExec{Binary: "codex", Now: bed.manager.Now}
			bed.manager.Adapters["claude-headless"] = launch.ClaudeHeadless{Binary: "claude"}
			owners := bed.owners()
			owners.processes.launches = func() *launch.Manager { return bed.manager }
			owners.processes.process.repositoryTop = fakeTop(root)
			owners.landing.home = func() (string, error) { return home, nil }
			owners.machines.registryPath = func() (string, error) { return filepath.Join(root, "host-registry"), nil }
			owners.machines.nickname = func(path string) (string, bool) { return "mac-cli", path == root }
			owners.machines.seatCensus = seatTickCensus{}
			owners.machines.runnerRepair = func(string) (steward.RunnerRepairOutcome, error) {
				return steward.RunnerRepairOutcome{Status: "CURRENT"}, nil
			}
			launcher := newStewardSeatLauncher()
			launcher.manager, launcher.repositoryTop, launcher.laneRoot = owners.processes.launches, fakeTop(root), laneRootAt(home)
			owners.machines.seatLauncher = &launcher
			owners.prove = enrolledPersonProver(t, root, now)
			payload, _ := json.Marshal(map[string]any{"schemaVersion": 1, "event": registry.EventRelaunched, "checkoutPath": root, "ownerTag": "revive", "at": now.Format(time.RFC3339), "generation": 1, "watcherTag": "w", "reaperTag": "r", "retiredThrough": 0})
			if err := registry.AppendFrame(filepath.Join(root, "host-registry"), payload); err != nil {
				t.Fatal(err)
			}
			conf := []byte("metasystem.runtimes=claude,codex\nlaunch.seat.runtime=off\nseat.driver=person\n")
			if strings.HasPrefix(scenario, "automatic") {
				conf = []byte("metasystem.runtimes=claude,codex\nlaunch.seat.runtime=claude\n")
			}
			if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), conf, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(root, ".git"), 0700); err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(scenario, "automatic") {
				seat, err := helm.Locate(root)
				if err != nil {
					t.Fatal(err)
				}
				data, _ := json.Marshal(helm.Record{Schema: 1, By: "Wido", At: now.Format(time.RFC3339)})
				if err := os.MkdirAll(filepath.Dir(seat.Signature), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(seat.Signature, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			enrollReviveRunner(t, root, now)
			mark, err := outage.Observe(home, "claude", "fixture-model", outage.ProviderLimit, "HTTP 429 Too Many Requests", "failed-seat", now.Add(-time.Minute))
			if err != nil {
				t.Fatal(err)
			}
			failed := steward.SeatRecord{Schema: 1, LaunchID: "failed-seat", Goal: bed.id, Held: true, ApprovalOpid: bed.goalFile(bed.id).Approved.Opid, Machine: "mac-cli", StartedAt: now.Add(-2 * time.Minute).UTC().Format("2006-01-02T15:04:05.000000000Z"), ReapedAt: now.Add(-time.Minute).Format(time.RFC3339), LaunchState: "failed", Outcome: steward.SeatProviderLimit}
			seats := filepath.Join(root, "artifacts", "agents", "steward", "seats")
			if err := os.MkdirAll(seats, 0700); err != nil {
				t.Fatal(err)
			}
			data, _ := json.Marshal(failed)
			if err := os.WriteFile(filepath.Join(seats, "failed-seat.json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			launchState := launch.Failed
			if scenario == "closed-completed" || strings.HasSuffix(scenario, "-continuation") {
				launchState = launch.Completed
			}
			if err := bed.manager.Store.Create(launch.Record{ID: failed.LaunchID, Kind: "seat", State: launchState, Adapter: "claude-headless", WorkingDirectory: root, StartedAt: failed.StartedAt, FinishedAt: now.Add(-time.Minute).Format(time.RFC3339)}); err != nil {
				t.Fatal(err)
			}
			before := steward.Evidence{AbnormalCount: 2, DryRevivals: 3}
			before.Abnormal[0] = steward.AbnormalRestart{At: now.Add(-2 * time.Minute), Class: "failed"}
			before.Abnormal[1] = steward.AbnormalRestart{At: now.Add(-time.Minute), Class: "failed", Pending: true}
			if strings.HasPrefix(scenario, "automatic") {
				before = steward.Evidence{}
				if err := outage.Clear(home, "anthropic", now); err != nil {
					t.Fatal(err)
				}
			}
			if strings.HasSuffix(scenario, "-continuation") {
				before.CurrentContinuation = "stranded-continuation"
				consumed := filepath.Join(root, "artifacts", "agents", "steward", "consumed")
				jobs := filepath.Join(root, "artifacts", "agents", "jobs")
				for _, dir := range []string{consumed, jobs} {
					if err := os.MkdirAll(dir, 0700); err != nil {
						t.Fatal(err)
					}
				}
				data, _ := json.Marshal(steward.Intent{Nonce: before.CurrentContinuation, JobId: "steward-stranded", Reason: "seatHandoff"})
				if err := os.WriteFile(filepath.Join(consumed, before.CurrentContinuation+".json"), data, 0600); err != nil {
					t.Fatal(err)
				}
				job := map[string]any{"status": "failed", "phase": "handshake", "error": "runtime_error"}
				if scenario == "closed-stranded-continuation" {
					job["endedAt"] = now.Add(-time.Minute).Format(time.RFC3339)
				}
				data, _ = json.Marshal(job)
				if err := os.WriteFile(filepath.Join(jobs, "steward-stranded.json"), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := steward.SaveEvidence(root, steward.EvidencePath(root), before); err != nil {
				t.Fatal(err)
			}
			endpoint, err := goal.ResolveEndpoint(root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := goal.Project(endpoint, false, now); err != nil {
				t.Fatalf("fixture committed ledger: %v", err)
			}
			if _, err := goal.ReadClaimableBudgetedWork(root, now); err != nil {
				t.Fatalf("fixture held work: %v", err)
			}
			bed.starter.hold = "seat"
			if scenario == "agent" {
				owners.prove = func(r string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
					return humanauthority.Prove(r, 80, person(), at)
				}
			}
			if scenario == "stopped" {
				exited, _ := json.Marshal(map[string]any{"schemaVersion": 1, "event": registry.EventExited, "checkoutPath": root, "ownerTag": "revive", "at": now.Format(time.RFC3339), "reason": "shutdown", "teardownComplete": true})
				if err := registry.AppendFrame(filepath.Join(root, "host-registry"), exited); err != nil {
					t.Fatal(err)
				}
				if err := stopfence.Write(root, stopfence.Record{SchemaVersion: 1, State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 1, ChangedAt: now.Format(time.RFC3339), By: stopfence.Actor{}}); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "unreadable" {
				dir, _ := bed.manager.Store.StateDir(failed.LaunchID)
				if err := os.WriteFile(filepath.Join(dir, "record.json"), []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "missing-episode" || scenario == "automatic-missing-episode" || scenario == "automatic-old-interval" {
				providers, err := outage.ReadProviders(home)
				if err != nil {
					t.Fatal(err)
				}
				providers.Current = map[string]outage.Condition{}
				if scenario == "automatic-old-interval" {
					providers.Current["anthropic"] = outage.Condition{Intervals: []outage.Interval{{Since: now.Add(-time.Hour).Format(time.RFC3339), Until: now.Add(-30 * time.Minute).Format(time.RFC3339)}}}
				}
				data, _ := json.Marshal(providers)
				path := filepath.Join(providers.Owner.Install, "artifacts", "agents", fmt.Sprintf("providers-%d.json", providers.Owner.CustodyEpoch))
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "invalid-finished" || scenario == "automatic-invalid-finished" {
				if _, err := bed.manager.Store.Update(failed.LaunchID, func(r *launch.Record) error { r.FinishedAt = "unreadable"; return nil }); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "unreadable-episode" {
				providers, err := outage.ReadProviders(home)
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(providers.Owner.Install, "artifacts", "agents", fmt.Sprintf("providers-%d.json", providers.Owner.CustodyEpoch))
				if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "missing-launch" || scenario == "missing-launch-live" || scenario == "older-start-failed" {
				dir, _ := bed.manager.Store.StateDir(failed.LaunchID)
				if err := os.Remove(filepath.Join(dir, "record.json")); err != nil {
					t.Fatal(err)
				}
				failed.LaunchState, failed.Outcome = "missing", steward.SeatNoProgress
				if scenario == "older-start-failed" {
					failed.LaunchState, failed.Outcome = "", steward.SeatStartFailed
				}
				data, _ := json.Marshal(failed)
				if err := os.WriteFile(filepath.Join(seats, "failed-seat.json"), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "missing-launch-live" {
				owners.machines.seatCensus = seatTickLiveMain{}
			}
			starts := 0
			if scenario == "reservation" || scenario == "response-loss" || scenario == "launcher-refused" {
				launcher.start = func(spec launch.StartSpec) (launch.Record, error) {
					starts++
					if scenario == "launcher-refused" && starts == 1 {
						manager := *bed.manager
						manager.Settings.SeatRuntime = "unsupported"
						return manager.Start(spec)
					}
					manager := *bed.manager
					manager.Settings.SeatRuntime, manager.Settings.SeatModel = "codex", "fixture-model"
					record, err := manager.Start(spec)
					if err != nil {
						return record, err
					}
					if scenario == "launcher-refused" {
						return record, nil
					}
					// A lost response can leave Starting with no recorded process identity.
					record, err = manager.Store.Update(record.ID, func(r *launch.Record) error {
						r.State = launch.Starting
						if scenario == "reservation" {
							r.Child, r.Supervisor = nil, nil
						}
						return nil
					})
					if err != nil {
						return record, err
					}
					return record, errors.New("response lost after process creation")
				}
			}
			run := func(after bool) (int, intentResult) {
				t.Helper()
				args := []string{"machine", "revive", "mac-cli", "--json"}
				if scenario == "stopped" {
					args[2] = root
				}
				if after {
					name := failed.LaunchID
					if scenario == "stale" {
						name = "older-seat"
					}
					args = append(args, "--after", name)
				}
				return bed.runJSON(owners, args...)
			}
			if strings.HasPrefix(scenario, "automatic") {
				record, err := steward.StartSeat(root, steward.TickConfig{Now: now, ProviderHome: home, Seat: launcher}, seatTickCensus{}, steward.SeatSelection{Goal: bed.id, Held: true, ApprovalOpid: failed.ApprovalOpid})
				want := &steward.SeatRecovery{LaunchID: failed.LaunchID, Provider: "anthropic", Episode: mark.Since}
				if scenario != "automatic" {
					want = nil
				}
				if err != nil || record.LaunchID == "" || !reflect.DeepEqual(record.RecoveryOf, want) {
					t.Fatalf("automatic recovery link: %+v %v", record, err)
				}
				actual, err := bed.manager.Status(record.LaunchID)
				if err != nil || actual.State != launch.Running {
					t.Fatalf("automatic actual launch: %+v %v", actual, err)
				}
				return
			}
			code, result := run(scenario != "omitted")
			if strings.HasPrefix(scenario, "closed-") {
				if code != 0 || result.Outcome != intentConfirmed || result.Next != nil ||
					!strings.Contains(result.Summary, "is closed by your word") || !strings.Contains(result.Summary, "restart history cleared") ||
					len(bed.starter.launched()) != 0 {
					t.Fatalf("unbound launch was not closed without a start: %d %+v", code, result)
				}
				if scenario == "closed-completed" && !strings.Contains(result.Summary, "it ended completed, not failed") {
					t.Fatalf("the completed launch's close lost its reason: %+v", result)
				}
				after, err := steward.LoadEvidence(steward.EvidencePath(root))
				if err != nil || after.AbnormalCount != 0 || after.Abnormal != [2]steward.AbnormalRestart{} || after.CurrentSeat != before.CurrentSeat || after.DryRevivals != before.DryRevivals {
					t.Fatalf("close did not clear only restart history: %+v %v", after, err)
				}
				if strings.HasSuffix(scenario, "-continuation") {
					live := scenario == "closed-live-continuation"
					if live && (after.CurrentContinuation != before.CurrentContinuation || !strings.Contains(result.Summary, "still has running job steward-stranded")) ||
						!live && (after.CurrentContinuation != "" || !strings.Contains(result.Summary, "continuation stranded-continuation settled")) {
						t.Fatalf("continuation close lost its outcome: %+v %+v", after, result)
					}
				}
				path := steward.EvidencePath(root)
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				// Lawful whitespace makes an unnecessary rewrite observable.
				data = append(data, '\n')
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				againCode, again := run(true)
				unchanged, err := os.ReadFile(path)
				if againCode != 0 || again.Outcome != intentConfirmed || again.Next != nil ||
					!strings.Contains(again.Summary, "was already clear") || len(bed.starter.launched()) != 0 || err != nil || !bytes.Equal(data, unchanged) {
					t.Fatalf("close repeat wrote or started work: %d %+v %v", againCode, again, err)
				}
				if scenario == "closed-stranded-continuation" {
					if _, err := helm.Remove(root); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=claude,codex\nlaunch.seat.runtime=claude\n"), 0600); err != nil {
						t.Fatal(err)
					}
					if err := outage.Clear(home, "anthropic", now); err != nil {
						t.Fatal(err)
					}
					record, err := steward.StartSeat(root, steward.TickConfig{Now: now, ProviderHome: home, Seat: launcher}, seatTickCensus{}, steward.SeatSelection{Goal: bed.id, Held: true, ApprovalOpid: failed.ApprovalOpid})
					if err != nil || record.LaunchID == "" {
						t.Fatalf("closed continuation still blocks a seat: %+v %v", record, err)
					}
				}
				return
			}
			if scenario == "agent" || scenario == "stale" || scenario == "stopped" || scenario == "unreadable" || scenario == "missing-launch-live" || scenario == "reservation" {
				if code == 0 || scenario != "reservation" && len(bed.starter.launched()) != 0 {
					t.Fatalf("%s reported a start: %d %+v", scenario, code, result)
				}
				wantReason := map[string]string{"agent": "only you", "stale": "stale", "stopped": "intentionally stopped", "unreadable": "cannot be read", "missing-launch-live": "does not prove the seat is free", "reservation": "unknown launch outcome"}[scenario]
				if !strings.Contains(result.Summary, wantReason) {
					t.Fatalf("%s failed at the wrong boundary: %+v", scenario, result)
				}
				if scenario == "stopped" && (result.Next == nil || !strings.Contains(strings.Join(result.Next.Argv, " "), "system start --repo ")) {
					t.Fatalf("wrong stop repair: %+v", result)
				}
				if scenario == "reservation" {
					code, result = run(true)
					if code == 0 || starts != 1 {
						t.Fatalf("unknown reservation duplicated or confirmed: %d %+v starts=%d", code, result, starts)
					}
				}
				return
			}
			if scenario == "launcher-refused" {
				if code == 0 {
					t.Fatal("launcher refusal was reported as a start")
				}
				var refused steward.SeatRecord
				data, _ := json.Marshal(result.Data)
				if err := json.Unmarshal(data, &refused); err != nil {
					t.Fatal(err)
				}
				if refused.ReapedAt == "" || refused.Outcome != steward.SeatStartFailed {
					t.Fatalf("launcher refusal did not close its seat record: %+v", refused)
				}
				failed = refused
				owners.commandNow = func(string) (time.Time, error) { return now.Add(time.Second), nil }
				code, result = run(false)
			}
			if scenario == "response-loss" {
				if code == 0 {
					t.Fatal("lost response was reported as a start")
				}
				code, result = run(true)
			}
			if code != 0 || !strings.Contains(result.Summary, "automatic policy and restart history stay unchanged") {
				t.Fatalf("person's act: %d %+v", code, result)
			}
			var record steward.SeatRecord
			data, _ = json.Marshal(result.Data)
			if err := json.Unmarshal(data, &record); err != nil {
				t.Fatal(err)
			}
			want := &steward.SeatRecovery{LaunchID: failed.LaunchID, Provider: "anthropic", Episode: mark.Since}
			if scenario == "missing-episode" || scenario == "invalid-finished" || scenario == "unreadable-episode" || scenario == "missing-launch" || scenario == "older-start-failed" || scenario == "launcher-refused" {
				want = &steward.SeatRecovery{LaunchID: failed.LaunchID}
			}
			if !reflect.DeepEqual(record.RecoveryOf, want) {
				t.Fatalf("person recovery link: %+v", record)
			}
			actual, err := bed.manager.Status(record.LaunchID)
			if err != nil || actual.State != launch.Running && !(scenario == "response-loss" && actual.State == launch.Starting) {
				t.Fatalf("actual start: %+v %v", actual, err)
			}
			againCode, again := run(true)
			againData, _ := json.Marshal(again.Data)
			if againCode != 0 || string(againData) != string(data) || len(bed.starter.launched()) != 1 {
				t.Fatalf("replay duplicated launch: %d %+v", againCode, again)
			}
			after, err := steward.LoadEvidence(steward.EvidencePath(root))
			if err != nil || after.Abnormal != before.Abnormal || after.AbnormalCount != before.AbnormalCount || after.DryRevivals != before.DryRevivals {
				t.Fatalf("person spent/reset automatic history: %+v %v", after, err)
			}
			current, _ := os.ReadFile(filepath.Join(root, "metasystem.conf"))
			if !bytes.Equal(conf, current) || !helm.Active(root).Active {
				t.Fatal("person override changed standing policy")
			}
			marks, err := outage.ReadProviders(home)
			wantMark := mark
			if scenario == "missing-episode" {
				wantMark = outage.Mark{}
			}
			if scenario != "unreadable-episode" && (err != nil || !reflect.DeepEqual(marks.Current["anthropic"].Mark, wantMark)) {
				t.Fatalf("person changed provider policy: %+v %v", marks, err)
			}
			// An ordinary successor keeps its own admission, with no inherited actor.
			if _, err := bed.manager.Store.Update(record.LaunchID, func(r *launch.Record) error { r.State = launch.Failed; return nil }); err != nil {
				t.Fatal(err)
			}
			if next, err := steward.StartSeat(root, steward.TickConfig{Now: now, ProviderHome: home, Seat: launcher}, seatTickCensus{}, steward.SeatSelection{Goal: bed.id, Held: true}); err != nil || next.LaunchID != "" || len(bed.starter.launched()) != 1 {
				t.Fatalf("later automatic tick inherited override: %+v %v", next, err)
			}
			fmt.Println("confirmed public revival and exact successor", record.LaunchID)
		})
	}
}

func TestMachineReviveRefusesUnknownRunnerExecution(t *testing.T) {
	t.Parallel()
	bed := newWorkBedWith(t, func(f *goal.GoalFile) { workApprovedBox(f); f.Claimed.Lineage = steward.SeatLineage })
	root, home, now := bed.root(), bed.manager.CapacityHome, bed.manager.Now()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=claude,codex\n"), 0600); err != nil {
		t.Fatal(err)
	}
	owners := bed.owners()
	owners.processes.launches = func() *launch.Manager { return bed.manager }
	owners.processes.process.repositoryTop = fakeTop(root)
	owners.landing.home = func() (string, error) { return home, nil }
	registryPath := filepath.Join(root, "host-registry")
	owners.machines.registryPath = func() (string, error) { return registryPath, nil }
	owners.machines.nickname = func(path string) (string, bool) { return "mac-cli", path == root }
	owners.machines.seatCensus = seatTickCensus{}
	launcher := newStewardSeatLauncher()
	launcher.manager, launcher.repositoryTop, launcher.laneRoot = owners.processes.launches, fakeTop(root), laneRootAt(home)
	owners.machines.seatLauncher = &launcher
	owners.prove = enrolledPersonProver(t, root, now)
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{"schemaVersion": 1, "event": registry.EventRelaunched, "checkoutPath": root, "ownerTag": "revive", "at": now.Format(time.RFC3339), "generation": 1, "watcherTag": "w", "reaperTag": "r", "retiredThrough": 0})
	if err := registry.AppendFrame(registryPath, payload); err != nil {
		t.Fatal(err)
	}
	enrollReviveRunner(t, root, now)
	code, result := bed.runJSON(owners, "machine", "revive", "mac-cli")
	if code == 0 || !strings.Contains(result.Summary, "the enrolled runner could not be repaired") || !strings.Contains(result.Summary, "execution is unknown") {
		t.Fatalf("unknown runner execution must refuse revival: exit=%d result=%+v", code, result)
	}
	if starts := bed.starter.launched(); len(starts) != 0 {
		t.Fatalf("unknown runner execution launched seats: %v", starts)
	}
}

func enrollReviveRunner(t *testing.T, root string, now time.Time) {
	t.Helper()
	engine := filepath.Join(root, "bin", "metasystem")
	if err := os.MkdirAll(filepath.Dir(engine), 0700); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(engine, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(steward.RepoIdentityPath(root)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := steward.MintIdentity(steward.RepoIdentityPath(root), steward.InstallIdentity{RepoIdentity: root, Generation: 1, InstallPath: engine, InstallDigest: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("#!/bin/sh\nexit 0\n"))), MintedAt: now.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	process, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("fixture process: %v %v", state, err)
	}
	runner := steward.RunnerRecord{Pid: process.Pid, PidStartedAt: process.StartedAt.Unix(), StartTicks: process.StartTicks, BootID: process.BootID}
	runnerData, _ := json.Marshal(runner)
	if err := os.WriteFile(filepath.Join(root, "artifacts", "agents", "steward", "runner.json"), runnerData, 0600); err != nil {
		t.Fatal(err)
	}
	observedAt := time.Now()
	attempt, err := steward.BeginComponentAttempt(root, "steward-tick", 1, process.Ref(), observedAt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := steward.CompleteComponentAttempt(root, "steward-tick", 1, attempt.AttemptSeq, steward.ComponentOK, "OK", "fixture runner pass", observedAt); err != nil {
		t.Fatal(err)
	}
}
