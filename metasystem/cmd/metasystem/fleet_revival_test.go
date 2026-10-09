package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

// The subprocess isolates executable discovery while every scenario uses
// production arbitration, persistence, reaping and the public steward verbs.
func TestFleetRevivalAndBuildPublicAdmission(t *testing.T) {
	t.Parallel()
	if os.Getenv("FLEET_REVIVAL_ISOLATED") == "" {
		tools := t.TempDir()
		if err := testexec.WriteFile(filepath.Join(tools, "git"), []byte(`#!/bin/sh
root=$(pwd -P)
if [ "$1" = -C ]; then root=$2; shift 2; fi
case "$*" in
 'rev-parse --show-toplevel') printf '%s\n' "$root" ;;
 'rev-parse HEAD') cat "$root/head" ;;
 *) printf 'fatal: not a git repository\n' >&2; exit 128 ;;
esac
`), 0700); err != nil {
			t.Fatal(err)
		}
		command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestFleetRevivalAndBuildPublicAdmission$", "-test.timeout=30m", "-test.v")
		for _, entry := range os.Environ() {
			name, _, _ := strings.Cut(entry, "=")
			if name != "PATH" && name != "METASYSTEM_SUPERVISION_REGISTRY_HOME" && name != "METASYSTEM_TESTENV_REGISTRY_NONCE" {
				command.Env = append(command.Env, entry)
			}
		}
		command.Env = append(command.Env, "FLEET_REVIVAL_ISOLATED=1", "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"))
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("isolated public revival: %v\n%s", err, output)
		}
		return
	}
	for _, scenario := range []string{"hour-cap", "repeated-class", "no-progress", "rolling-hour", "unknown-dispatch", "provider-hold", "history-unreadable"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			home := testprovider.Register(t, root)
			write := func(relative string, value any) {
				t.Helper()
				data, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, relative)
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.MkdirAll(filepath.Join(root, "plans"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "plans", "goals.md"), []byte("# Goals\n\n## Current goal: fix-it — Repair the thing\n- Origin: main\n- Next step: Repair it.\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0600); err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
			identity := steward.InstallIdentity{RepoIdentity: root, Generation: 1, InstallPath: "/bin/true", MintedAt: now.Format(time.RFC3339)}
			if err := os.MkdirAll(filepath.Dir(steward.RepoIdentityPath(root)), 0700); err != nil {
				t.Fatal(err)
			}
			if err := steward.MintIdentity(steward.RepoIdentityPath(root), identity); err != nil {
				t.Fatal(err)
			}
			if err := steward.SaveEvidence(root, steward.EvidencePath(root), steward.Evidence{Marks: steward.Marks{HeadOid: "head-0"}}); err != nil {
				t.Fatal(err)
			}
			launched := 0
			read := func() steward.Evidence {
				t.Helper()
				var output, problem bytes.Buffer
				if code := runStewardStatus([]string{"--repo", root}, &output, &problem); code != 0 {
					t.Fatalf("public status exit %d: %s %s", code, &output, &problem)
				}
				var report struct {
					Evidence steward.Evidence `json:"evidence"`
				}
				if err := json.Unmarshal(output.Bytes(), &report); err != nil {
					t.Fatal(err)
				}
				return report.Evidence
			}
			progress := func(n int) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(root, "head"), []byte(fmt.Sprintf("head-%d\n", n)), 0600); err != nil {
					t.Fatal(err)
				}
				if _, err := steward.RunTick(root, steward.TickConfig{Now: now, ProviderHome: home, WorkStateRoot: t.TempDir()}, seatTickCensus{}); err != nil {
					t.Fatal(err)
				}
			}
			attempt := func(n int, status string, dispatchUnknown bool) steward.ReviveOutcome {
				t.Helper()
				intent := steward.Intent{Nonce: fmt.Sprintf("rev-%d", n), RepoIdentity: root, InstallGen: identity.Generation, Goal: "fix-it", JobId: fmt.Sprintf("steward-rev-%d", n), Runtime: "claude"}
				if err := steward.MintIntent(root, intent); err != nil {
					t.Fatal(err)
				}
				var outcome steward.ReviveOutcome
				var problem bytes.Buffer
				code := runStewardRunWithDependencies([]string{"--repo", root}, io.Discard, &problem,
					func(_ string, _ steward.WorkerCensus, _ func() error, _ time.Duration, cfg steward.TickConfig) error {
						cfg.Now = now
						cfg.ProviderHome = home
						var err error
						outcome, err = steward.CompleteRevival(root, cfg, seatTickCensus{}, intent.Nonce, func(consumed steward.Intent) error {
							ev := read()
							if ev.AbnormalCount == 0 || ev.Abnormal[ev.AbnormalCount-1].Nonce != consumed.Nonce || !ev.Abnormal[ev.AbnormalCount-1].Pending {
								t.Fatalf("launch preceded durable attempt: %+v launched=%d", ev, launched)
							}
							launched++
							if dispatchUnknown {
								return errors.New("process creation outcome unavailable")
							}
							write(filepath.Join("artifacts", "agents", "jobs", consumed.JobId+".json"), map[string]any{"jobId": consumed.JobId, "status": status, "endedAt": now.Format(time.RFC3339), "chainClosed": true})
							return nil
						}, nil)
						return err
					}, nil, nil, func(string) int { return 1 }, func() (bool, error) { return false, nil })
				if code != 0 {
					t.Fatalf("public steward run exit %d: %s", code, &problem)
				}
				return outcome
			}
			if scenario == "provider-hold" {
				if _, err := outage.Observe(home, "claude", "fixture-model", outage.ProviderLimit, "usage limit reached", "fixture", now); err != nil {
					t.Fatal(err)
				}
				held := attempt(0, "failed", false)
				ev := read()
				if held.Launched || launched != 0 || ev.AbnormalCount != 0 || ev.DryRevivals != 0 || !strings.Contains(held.Reason, "provider is overloaded") {
					t.Fatalf("provider hold spent an attempt: %+v", held)
				}
				return
			}
			if scenario == "repeated-class" {
				write(filepath.Join("artifacts", "agents", "steward", "seats", "predecessor.json"), steward.SeatRecord{LaunchID: "predecessor", StartedAt: now.Add(-time.Minute).Format(time.RFC3339), ReapedAt: now.Format(time.RFC3339), LaunchState: "failed", Outcome: steward.SeatProgress})
			}
			first := attempt(1, "failed", scenario == "unknown-dispatch")
			if scenario == "unknown-dispatch" {
				if !first.Escalate || first.Launched || read().AbnormalCount != 1 {
					t.Fatalf("unknown dispatch: %+v", first)
				}
				progress(1)
				now = now.Add(2 * time.Hour)
			} else if !first.Launched || launched != 1 {
				t.Fatalf("first restart: %+v", first)
			}
			if scenario != "no-progress" && scenario != "unknown-dispatch" {
				progress(1)
			}
			if scenario == "history-unreadable" {
				if err := os.WriteFile(steward.EvidencePath(root), []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
				intent := steward.Intent{Nonce: "unreadable", RepoIdentity: root, InstallGen: 1, Goal: "fix-it"}
				if err := steward.MintIntent(root, intent); err != nil {
					t.Fatal(err)
				}
				result, err := steward.CompleteRevival(root, steward.TickConfig{Now: now, ProviderHome: home}, seatTickCensus{}, intent.Nonce, func(steward.Intent) error { t.Fatal("unreadable history launched"); return nil }, nil)
				if err == nil || result.Launched {
					t.Fatalf("unreadable history: %+v %v", result, err)
				}
				return
			}

			second := attempt(2, "cancelled", false)
			if scenario == "hour-cap" || scenario == "rolling-hour" {
				if !second.Launched || launched != 2 {
					t.Fatalf("second distinct restart: %+v", second)
				}
				progress(2)
				// Re-enrollment changes the engine identity, never the stable seat's history.
				identity.Generation++
				if err := steward.MintIdentity(steward.RepoIdentityPath(root), identity); err != nil {
					t.Fatal(err)
				}
				if read().AbnormalCount != 2 {
					t.Fatal("re-arm erased history")
				}
				if scenario == "rolling-hour" {
					now = now.Add(time.Hour)
				}
				third := attempt(3, "failed", false)
				if scenario == "rolling-hour" {
					if !third.Launched || launched != 3 || read().AbnormalCount != 1 {
						t.Fatalf("expired rolling window: %+v", third)
					}
					return
				}
				second = third
			}
			expected := map[string]string{"hour-cap": "two automatic abnormal restarts", "repeated-class": "same death class", "no-progress": "no retained work progress", "unknown-dispatch": "unknown launch outcome"}[scenario]
			if second.Launched || !second.Held || !second.Escalate || !strings.Contains(second.Reason, expected) || !strings.Contains(second.Reason, "machine revive "+root+" (unavailable until fleet-provider-and-session-recovery R2 lands)") {
				t.Fatalf("automatic restart did not stop with its remedy: %+v", second)
			}
			var output, problem bytes.Buffer
			if code := runStewardStatus([]string{"--repo", root}, &output, &problem); code != 0 || !strings.Contains(output.String(), expected) {
				t.Fatalf("public status omitted stop: %d %s %s", code, &output, &problem)
			}
		})
	}
}
