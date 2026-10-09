package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestProviderRecoveryAskPublicLifecycle(t *testing.T) {
	t.Parallel()
	if os.Getenv("PROVIDER_RECOVERY_ISOLATED") == "" {
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
		selection := "^TestProviderRecoveryAskPublicLifecycle$"
		if _, subtests, ok := strings.Cut(flag.Lookup("test.run").Value.String(), "/"); ok {
			selection += "/" + subtests
		}
		command := exec.CommandContext(t.Context(), os.Args[0], "-test.run="+selection, "-test.timeout=30m", "-test.v")
		for _, entry := range os.Environ() {
			name, _, _ := strings.Cut(entry, "=")
			if name != "PATH" && name != "METASYSTEM_SUPERVISION_REGISTRY_HOME" && name != "METASYSTEM_TESTENV_REGISTRY_NONCE" {
				command.Env = append(command.Env, entry)
			}
		}
		command.Env = append(command.Env, "PATH="+tools+":"+os.Getenv("PATH"), "PROVIDER_RECOVERY_ISOLATED=1", "METASYSTEM_SUPERVISION_REGISTRY_HOME="+home)
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		pid := 0
		testenv.ReapFixtureProcessGroups(t, []testenv.FixtureProcessGroup{{Verb: "isolated provider recovery", Resolve: func() (int, bool, error) {
			return pid, pid != 0, nil
		}}})
		var output bytes.Buffer
		command.Stdout, command.Stderr = &output, &output
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		pid = command.Process.Pid
		if err := command.Wait(); err != nil {
			t.Fatalf("public machine revival: %v\n%s", err, &output)
		}
		return
	}
	for _, scenario := range []string{"person", "automatic", "wrong-launch", "wrong-episode", "failed-start", "reserved", "text", "unknown-reset", "late-tick", "stopped", "helm", "person-held", "relapse", "released", "dependent", "registration", "delivery", "unreadable", "old-interval", "no-terminal", "bad-time"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed := newWorkBedWith(t, func(f *goal.GoalFile) { workApprovedBox(f); f.Claimed.Lineage = steward.SeatLineage })
			other := newWorkBedWith(t, func(f *goal.GoalFile) { workApprovedBox(f); f.Claimed.Lineage = steward.SeatLineage })
			if root, next := bed.root(), other.root(); root >= next {
				t.Fatalf("fixture must visit the dependent seat first: %s >= %s", root, next)
			}
			root, home, start := bed.root(), bed.manager.CapacityHome, bed.manager.Now()
			providers, err := outage.ReadProviders(home)
			if err != nil {
				t.Fatal(err)
			}
			owner := providers.Owner.Install
			registryPath := filepath.Join(root, "host-registry")
			for _, path := range []string{root, other.root()} {
				fleetWriteJSON(t, filepath.Join(path, ".git", "fixture.json"), map[string]string{})
				frame, _ := json.Marshal(map[string]any{"schemaVersion": 1, "event": registry.EventRelaunched, "checkoutPath": path, "ownerTag": path, "at": start.Format(time.RFC3339), "generation": 1, "watcherTag": "w", "reaperTag": "r", "retiredThrough": 0})
				if err := registry.AppendFrame(registryPath, frame); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(owner, "metasystem.conf"), []byte("metasystem.runtimes=fake\nprovider.recovery-alert-after=6m\n"), 0600); err != nil {
				t.Fatal(err)
			}
			bed.manager.Adapters["claude-headless"] = launch.ClaudeHeadless{Binary: "claude"}
			bed.starter.hold = "seat"
			for _, seat := range []*workBed{bed, other} {
				if err := os.WriteFile(filepath.Join(seat.root(), "metasystem.conf"), []byte("metasystem.runtimes=claude,codex\nlaunch.seat.runtime=claude\n"), 0600); err != nil {
					t.Fatal(err)
				}
				if _, err := goal.ReadClaimableBudgetedWork(seat.root(), start); err != nil {
					t.Fatal(err)
				}
			}
			launcher := newStewardSeatLauncher()
			launcher.manager, launcher.repositoryTop, launcher.laneRoot = func() *launch.Manager { return bed.manager }, fakeTop(root), laneRootAt(home)
			markAt := start.Add(-time.Minute)
			detail := fmt.Sprintf("Claude AI usage limit reached|%d", start.Add(time.Minute).Unix())
			if scenario == "unknown-reset" {
				detail = "HTTP 429 Too Many Requests"
			}
			mark, err := outage.Observe(home, "claude", "fixture-model", outage.ProviderLimit, detail, "failed-seat", markAt)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := outage.Observe(home, "codex", "fixture-model", outage.ProviderLimit, "HTTP 429 Too Many Requests", "other-failed", markAt); err != nil {
				t.Fatal(err)
			}
			failed := steward.SeatRecord{Schema: 1, LaunchID: "failed-seat", Goal: bed.id, Held: true, ApprovalOpid: bed.goalFile(bed.id).Approved.Opid, Machine: "mac-cli", StartedAt: markAt.Add(-time.Minute).Format(time.RFC3339Nano), ReapedAt: start.Format(time.RFC3339Nano), Outcome: steward.SeatProviderLimit, LaunchState: "failed"}
			for _, seat := range []*workBed{bed, other} {
				r, adapter := failed, "claude-headless"
				if seat == other {
					r.LaunchID, r.Goal, r.ApprovalOpid, r.Machine, adapter = "other-failed", other.id, other.goalFile(other.id).Approved.Opid, "other-seat", "codex-exec"
				}
				fleetWriteJSON(t, filepath.Join(seat.root(), "artifacts", "agents", "steward", "seats", r.LaunchID+".json"), r)
				state := launch.Failed
				if seat == bed && scenario == "no-terminal" {
					state = launch.Starting
				}
				if err := bed.manager.Store.Create(launch.Record{ID: r.LaunchID, Kind: "seat", State: state, Adapter: adapter, WorkingDirectory: seat.root(), StartedAt: r.StartedAt, FinishedAt: start.Format(time.RFC3339Nano)}); err != nil {
					t.Fatal(err)
				}
			}
			tick := func(at time.Time) {
				t.Helper()
				now := at
				clock := &steward.HandoffClock{Now: func() time.Time { return now }, Sleep: func(d time.Duration) {
					now = now.Add(d)
					if err := os.WriteFile(filepath.Join(owner, "artifacts", "agents", "steward", "stop"), nil, 0600); err != nil {
						t.Fatal(err)
					}
				}}
				registered := families()
				for i := range registered {
					if registered[i].name == "steward" {
						for j := range registered[i].verbs {
							if registered[i].verbs[j].name == "run" {
								registered[i].verbs[j].run = func(args []string, out, errout io.Writer) int {
									return runStewardRunWithDependencies(args, out, errout, func(r string, _ steward.WorkerCensus, revive func() error, interval time.Duration, cfg steward.TickConfig) error {
										cfg.Now, cfg.ProviderHome, cfg.WorkStateRoot, cfg.RecoveryRegistry, cfg.Seat = at, home, root, registryPath, launcher
										cfg.RearmAtBoundary, cfg.CompletedBoundary, cfg.KeepLandingLane = nil, nil, nil
										return steward.RunLoop(r, seatTickCensus{}, revive, interval, cfg)
									}, func(string, string) error { return exec.ErrNotFound }, clock, func(string) int { return 1 }, nil)
								}
							}
						}
					}
				}
				var out, errout bytes.Buffer
				if code := dispatchWithFamiliesAndRepositoryTop([]string{"steward", "run", "--repo", owner}, &out, &errout, registered, fakeTop(owner)); code != 0 {
					t.Fatalf("public runner exit %d: %s", code, errout.String())
				}
				if strings.Contains(errout.String(), "seat recovery requests:") && scenario != "unreadable" && scenario != "old-interval" && scenario != "no-terminal" && scenario != "bad-time" {
					t.Fatalf("recovery failed: %s", errout.String())
				}
			}
			assertCount := func(want int) channel.Question {
				t.Helper()
				all, unreadable := channel.WalkQuestions(root)
				if len(unreadable) != 0 || len(all) != want {
					t.Fatalf("questions want %d, got %+v, unreadable %v", want, all, unreadable)
				}
				if q, _ := channel.WalkQuestions(other.root()); len(q) != 0 {
					t.Fatalf("unrelated provider requested: %+v", q)
				}
				if want > 0 {
					return all[0]
				}
				return channel.Question{}
			}
			// Fail reset probes beyond the former reset-based deadline.
			for _, at := range []time.Time{start.Add(2 * time.Minute), start.Add(8 * time.Minute)} {
				if _, err := outage.Observe(home, "claude", "fixture-model", outage.ProviderLimit, detail, "failed-probe", at); err != nil {
					t.Fatal(err)
				}
				tick(at)
				assertCount(0)
			}
			// A stale close starts no recovery deadline.
			success := start.Add(40 * time.Minute)
			tick(success.Add(-time.Minute))
			assertCount(0)
			if err := os.WriteFile(filepath.Join(owner, "metasystem.conf"), []byte("metasystem.runtimes=fake\nprovider.recovery-alert-after=20m\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := outage.Observe(home, "claude", "fixture-model", "", "", "other-seat-success", success); err != nil {
				t.Fatal(err)
			}
			if _, err := outage.Observe(home, "claude", "fixture-model", "", "", "second-success", success.Add(3*time.Minute)); err != nil {
				t.Fatal(err)
			}
			tick(success.Add(5 * time.Minute))
			assertCount(0)
			switch scenario {
			case "stopped":
				if err := stopfence.Write(root, stopfence.Record{SchemaVersion: 1, State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 1, ChangedAt: success.Format(time.RFC3339)}); err != nil {
					t.Fatal(err)
				}
			case "helm":
				location, err := helm.Locate(root)
				if err != nil {
					t.Fatal(err)
				}
				fleetWriteJSON(t, location.Signature, helm.Record{Schema: 1, By: "Wido", At: success.Format(time.RFC3339)})
			case "person-held":
				if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=claude,codex\nseat.driver=person\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "relapse":
				if _, err := outage.Observe(home, "claude", "fixture-model", outage.ProviderLimit, "HTTP 429 Too Many Requests", "relapsed-provider", success.Add(5*time.Minute+time.Second)); err != nil {
					t.Fatal(err)
				}
			case "released":
				file := bed.goalFile(bed.id)
				file.Claimed = nil
				file.History = append(file.History, goal.HistoryLine{At: success.Format(time.RFC3339), Verb: "release", Actor: "mac-cli+" + steward.SeatLineage, Targets: []string{file.Id}, Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAX", "mac-cli", steward.SeatLineage), Keep: -1})
				file.Revision++
				bed.addGoal(file)
			case "dependent":
				fleetWriteJSON(t, filepath.Join(root, "unit", "active", "run.json"), launch.UnitRunRecord{ID: "active", Goal: bed.id, State: "running", Rounds: []launch.UnitRound{{Steps: []launch.UnitStep{{Name: "build", State: launch.StepStarting}}}}})
			case "unreadable":
				if err := os.WriteFile(filepath.Join(root, "artifacts", "agents", "steward", "seats", failed.LaunchID+".json"), []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			at := success.Add(6 * time.Minute)
			if scenario == "old-interval" || scenario == "no-terminal" || scenario == "bad-time" {
				if _, err := outage.Observe(home, "codex", "fixture-model", "", "", "other-seat-success", success); err != nil {
					t.Fatal(err)
				}
				if scenario == "old-interval" {
					current, err := outage.ReadProviders(home)
					if err != nil {
						t.Fatal(err)
					}
					condition := current.Current["anthropic"]
					for i := range condition.Intervals {
						condition.Intervals[i].RecoveryAfter = ""
					}
					current.Current["anthropic"] = condition
					fleetWriteJSON(t, filepath.Join(owner, "artifacts", "agents", fmt.Sprintf("providers-%d.json", current.Owner.CustodyEpoch)), current)
				} else if scenario == "bad-time" {
					record, err := bed.manager.Store.Read(failed.LaunchID)
					if err != nil {
						t.Fatal(err)
					}
					record.FinishedAt = "unreadable"
					if _, err := bed.manager.Store.Update(record.ID, func(stored *launch.Record) error { *stored = record; return nil }); err != nil {
						t.Fatal(err)
					}
				}
				for range 2 {
					tick(at)
					if questions, unreadable := channel.WalkQuestions(root); len(questions) != 0 || len(unreadable) != 0 {
						t.Fatalf("unknown recovery evidence must not invent a request: %+v %v", questions, unreadable)
					}
					questions, unreadable := channel.WalkQuestions(other.root())
					if len(questions) != 1 || len(unreadable) != 0 || questions[0].Recovery == nil || questions[0].Recovery.Launch != "other-failed" {
						t.Fatalf("second seat was skipped or duplicated: %+v %v", questions, unreadable)
					}
					obligation := steward.ReconcileRecoveryRequests(owner, steward.TickConfig{Now: at, ProviderHome: home, RecoveryRegistry: registryPath, Seat: launcher, WorkStateRoot: root}, seatTickCensus{})
					want := map[string]string{"old-interval": "lacks a readable recovery interval declaration", "no-terminal": "has no confirmed terminal observation", "bad-time": "provider episode or failed launch time is unreadable"}[scenario]
					if obligation == nil || !strings.Contains(obligation.Error(), "seat "+root+":") || !strings.Contains(obligation.Error(), want) {
						t.Fatalf("first seat's source obligation was hidden: %v", obligation)
					}
				}
				return
			}
			if scenario == "late-tick" {
				at = success.Add(time.Hour)
			}
			tick(at)
			if scenario == "stopped" || scenario == "helm" || scenario == "person-held" || scenario == "relapse" || scenario == "released" || scenario == "dependent" || scenario == "unreadable" {
				assertCount(0)
				return
			}
			q := assertCount(1)
			if q.Recovery == nil || q.Recovery.Episode != mark.Since || q.Recovery.Due != success.Add(6*time.Minute).Format(time.RFC3339Nano) {
				t.Fatalf("subject/deadline: %+v", q)
			}
			tick(at)
			assertCount(1)
			owners := bed.owners()
			owners.processes.question = channel.ReadQuestion
			owners.processes.launches = func() *launch.Manager { return bed.manager }
			owners.processes.process.repositoryTop = fakeTop(root)
			owners.landing.home = func() (string, error) { return home, nil }
			owners.machines.registryPath = func() (string, error) { return registryPath, nil }
			owners.machines.nickname = func(path string) (string, bool) {
				if path == root {
					return "mac-cli", true
				}
				return "other-seat", path == other.root()
			}
			owners.machines.seatLauncher, owners.machines.seatCensus = &launcher, seatTickCensus{}
			owners.prove = enrolledPersonProver(t, root, at)
			owners.commandNow = func(string) (time.Time, error) { return at, nil }
			for _, argv := range [][]string{{"question", "list", "--json"}, {"question", "show", q.ID, "--json"}} {
				code, result := bed.runJSON(owners, argv...)
				data, _ := json.Marshal(result)
				if code != 0 || !bytes.Contains(data, []byte("machine revive")) {
					t.Fatalf("%v exit %d: %s", argv, code, data)
				}
			}
			if scenario == "person" {
				code, data, problem := runCLIHelp([]string{"help", "settings", "show"}, families())
				if code != 0 || !strings.Contains(data, "provider.recovery-alert-after") {
					t.Fatalf("setting help exit %d: %s %s", code, data, problem)
				}
			}
			if scenario == "delivery" {
				provider := &questionProvider{fail: true}
				owners.processes.channelLink = func(string) (channel.Provider, channel.DestinationConfig) {
					return provider, channel.DestinationConfig{}
				}
				bed.runJSON(owners, "question", "retry", q.ID, "--json")
				if retry := assertCount(1); retry.Undelivered != 1 || retry.ID != q.ID {
					t.Fatalf("failed delivery lost request: %+v", retry)
				}
				tick(at)
				assertCount(1)
				provider.fail = false
				if code, result := bed.runJSON(owners, "question", "retry", q.ID, "--json"); code != 0 {
					t.Fatalf("delivery retry: %d %+v", code, result)
				}
				if assertCount(1).Thread == nil {
					t.Fatal("retry failed to deliver original question")
				}
			}
			if scenario == "registration" {
				if err := os.MkdirAll(filepath.Join(owner, "metasystem"), 0700); err != nil {
					t.Fatal(err)
				}
				if _, _, err := lane.Register(home, lane.Layout{Checkout: lane.CheckoutRoot(owner), Install: lane.InstallRoot(filepath.Join(owner, "metasystem"))}, "replacement", at.Add(time.Minute)); err != nil {
					t.Fatal(err)
				}
				tick(at)
				if assertCount(1).State != "closed" {
					current, _ := outage.ReadProviders(home)
					t.Fatalf("stale request remained executable: old %+v current owner %+v", q.Recovery, current.Owner)
				}
				enrollReviveRunner(t, root, at)
				code, refusal := bed.runJSON(owners, "machine", "revive", "mac-cli", "--after", failed.LaunchID, "--json")
				if code == 0 {
					t.Fatal("old registration command executed")
				}
				_, remedy, found := strings.Cut(refusal.Summary, "; run ")
				remedy, _, complete := strings.Cut(remedy, " without --after")
				if !found || !complete {
					t.Fatalf("stale refusal lacks the current-launch remedy: %+v", refusal)
				}
				words := strings.Fields(remedy)
				for i, word := range words {
					if strings.HasPrefix(word, "\"") {
						words[i], err = strconv.Unquote(word)
						if err != nil {
							t.Fatal(err)
						}
					}
					if word == "--after" {
						t.Fatalf("remedy repeated the stale subject: %s", remedy)
					}
				}
				before := len(bed.starter.launched())
				for range 2 {
					if code, result := bed.runJSON(owners, append(words[1:], "--json")...); code != 0 {
						t.Fatalf("printed stale-request remedy failed: %d %+v", code, result)
					}
				}
				if started := len(bed.starter.launched()) - before; started != 1 {
					t.Fatalf("current-launch remedy must start exactly one successor: %d", started)
				}
				return
			}
			if scenario == "person" {
				enrollReviveRunner(t, root, at)
				words := strings.Fields(q.Wants)
				for i, word := range words {
					if strings.HasPrefix(word, "\"") {
						parsed, err := strconv.Unquote(word)
						if err != nil {
							t.Fatal(err)
						}
						words[i] = parsed
					}
				}
				code, result := bed.runJSON(owners, append(words[1:], "--json")...)
				if code != 0 {
					refusals, _ := bed.manager.Store.Refusals()
					t.Fatalf("printed remedy exit %d: %+v; refusals %+v", code, result, refusals)
				}
			} else if scenario == "automatic" {
				selection := steward.SeatSelection{Goal: bed.id, Held: true, ApprovalOpid: failed.ApprovalOpid}
				if record, err := steward.StartSeat(root, steward.TickConfig{Now: at, Seat: launcher, ProviderHome: home, WorkStateRoot: root}, seatTickCensus{}, selection); err != nil || record.LaunchID == "" {
					refusals, _ := bed.manager.Store.Refusals()
					t.Fatalf("automatic recovery: %+v %v; refusals %+v", record, err, refusals)
				}
			} else if scenario == "text" {
				q.Answer = &channel.Answer{Text: q.Wants, At: at, Phase: "matched"}
				fleetWriteJSON(t, filepath.Join(root, "artifacts", "agents", "channel", "questions", q.ID+".json"), q)
				if _, err := channel.Poll(context.Background(), channel.PollConfig{RepoRoot: root, Now: at, Provider: &questionProvider{}}); err != nil && !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			} else if scenario != "late-tick" && scenario != "unknown-reset" && scenario != "delivery" {
				r := steward.SeatRecord{Schema: 1, LaunchID: "successor", Goal: failed.Goal, Machine: failed.Machine, StartedAt: at.Format(time.RFC3339Nano), RecoveryOf: &steward.SeatRecovery{LaunchID: failed.LaunchID, Provider: "anthropic", Episode: mark.Since}}
				state := launch.Completed
				if scenario == "wrong-launch" {
					r.RecoveryOf.LaunchID = "another-seat"
				}
				if scenario == "wrong-episode" {
					r.RecoveryOf.Episode = "older-episode"
				}
				if scenario == "failed-start" {
					state = launch.Failed
				}
				if scenario == "reserved" {
					state = launch.Starting
				}
				fleetWriteJSON(t, filepath.Join(root, "artifacts", "agents", "steward", "seats", r.LaunchID+".json"), r)
				if err := bed.manager.Store.Create(launch.Record{ID: r.LaunchID, Kind: "seat", State: state, Adapter: "claude-headless", WorkingDirectory: root, StartedAt: at.Format(time.RFC3339Nano)}); err != nil {
					t.Fatal(err)
				}
			}
			tick(at)
			after := assertCount(1)
			closed := scenario == "person" || scenario == "automatic"
			if (after.State == "closed") != closed {
				t.Fatalf("%s closure: %+v", scenario, after)
			}
			if closed && !strings.Contains(after.ClosedBecause, "matching recovery launch") {
				t.Fatalf("closure lacks successor identity: %+v", after)
			}
			if closed {
				code, result := bed.runJSON(owners, "question", "show", q.ID, "--json")
				if code != 0 || !strings.Contains(result.Summary, after.ClosedBecause) {
					t.Fatalf("closed question lost its recovery result: %d %+v", code, result)
				}
				duplicate, found, err := channel.AskOrFind(channel.AskRequest{RepoRoot: root, About: "machine", Kind: "other", Machine: q.Machine, Now: at.Add(time.Minute), Wants: q.Wants, Facts: q.Facts, Recovery: q.Recovery})
				if err != nil || !found || duplicate.ID != q.ID || duplicate.State != "closed" {
					t.Fatalf("closed subject was asked again: %+v %v", duplicate, err)
				}
			}
			tick(at.Add(time.Minute))
			assertCount(1)
		})
	}
}
