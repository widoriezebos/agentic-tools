package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func TestMovedMainObserverAndStatus(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"running", "update next", "helm", "pause and busy", "older proof", "pause", "repair", "drain", "batch green", "trunk red", "restart failure", "unwatched", "batch", "trunk green", "green before observer", "inherited", "no repeat"} {
		t.Run(mode, func(t *testing.T) {
			b := newCadenceBed(t, "", 0, false, mode == "trunk red", mode != "batch green" && mode != "trunk red")
			b.owners.landing.plainProve = b.agent.proofEffects
			stamp := b.now.Add(-time.Minute).Format(time.RFC3339)
			want, launches := "proof batch-1 running since", 1
			switch mode {
			case "update next":
				b.agent.engineHasProofSlot = func() bool { return true }
				b.keeper = newLandingAgentKeeper(b.lane, b.home, b.agent)
				want, launches = "the pending engine update has the next free proof slot", 0
			case "helm":
				_, err := helm.Write(b.lane, helm.Record{By: "Noor", At: stamp})
				helmMust(t, err)
				want, launches = "held at the helm by Noor since", 0
			case "pause", "pause and busy":
				if mode == "pause and busy" {
					b.alive = true
					writeQuestionFixture(t, filepath.Join(plain.Dir(b.lane), "running.json"), plain.Running{Attempt: "busy", Commit: "older", Tree: "older-tree", Since: stamp, Pid: int64(os.Getpid())})
				}

				_, err := lane.SetPause(b.home, "Ari", b.now)
				helmMust(t, err)
				want, launches = "paused by Ari at", 0
			case "repair":
				helmMust(t, plain.WriteFix(b.lane, &plain.Fix{State: "building", Goal: "repair-17", Attempt: "fix", Job: "build", Units: []string{"repair"}}))
				want, launches = "a repair is pending (fix repair-17)", 0
			case "drain":
				_, _, err := plain.SetDrain(b.lane, plain.Drain{By: "Ari", At: stamp, State: plain.DrainHeld})
				helmMust(t, err)
				want, launches = "admission is closed", 0
			case "batch":
				writeQuestionFixture(t, filepath.Join(plain.Dir(b.lane), "batch.json"), plain.Batch{ID: "work", Base: "inside", State: plain.BatchPrepared, Lane: b.record, Members: []plain.GoalSHA{{Goal: "g", SHA: "candidate"}}})
				want, launches = "batch work waits for merge", 0
			case "older proof":
				b.alive = true
				writeQuestionFixture(t, filepath.Join(plain.Dir(b.lane), "running.json"), plain.Running{Commit: "old", Tree: "old-tree", Attempt: "older", Trunk: true, Since: stamp, Pid: int64(os.Getpid())})
				want, launches = "proof of old holds the slot since", 0
			case "batch green":
				want, launches = "proven by batch proof batch-full at", 0
			case "trunk red":
				want, launches = "red by proof batch-full at", 0
			case "restart failure":
				helmMust(t, os.Mkdir(filepath.Join(plain.Dir(b.lane), "deploys.jsonl"), 0700))
			case "unwatched":
				want, launches = "nothing is watching", 0
			case "trunk green", "green before observer", "inherited", "no repeat":
				r := plain.Result{Trunk: true, Commit: "outside-main", Tree: "outside-tree", Attempt: "tested", Result: plain.Green, At: stamp, FullAt: stamp, FullTree: "outside-tree", Scope: "full", Environment: "fixture"}
				want, launches = "proven green by tested at", 0
				if mode == "inherited" {
					r.Scope = "inherited"
					r.FullTree = "base-tree"
					r.Base = "base-tree"
					want = "inherits green from tree base-tree"
				}
				if mode == "no repeat" {
					r.Trunk = false
					r.Result = plain.Red
					r.Cause = &plain.Cause{Kind: "environment"}
					want = "another automatic check"
				}
				writeCauseProof(t, b.lane, "results.jsonl", r)
			}
			if mode != "unwatched" && mode != "green before observer" {
				if mode == "restart failure" {
					if err := b.keeper.Observe(b.record); err == nil {
						t.Fatal("restart failure was lost")
					}
				} else {
					for range 2 {
						helmMust(t, b.keeper.CheckMain(b.record, func() bool { return mode == "update next" }))
					}
				}
			}
			code, out := b.run(t, b.lane, "landing", "status")
			text := oneSpaced(out)
			if code != 0 || !strings.Contains(text, want) || len(b.argv) != launches {
				t.Fatalf("want %q, %d starts; got %d, %d starts: %s", want, launches, code, len(b.argv), text)
			}
			logMainStatus(t, out)
		})
	}
}

func logMainStatus(t *testing.T, out string) {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "main ") || strings.Contains(line, "stop S724 for main") {
			t.Log(strings.TrimSpace(line))
		}
	}
}

func TestMovedMainFirstSightingAndFetchOutage(t *testing.T) {
	t.Parallel()
	b := newSelectionBed(t)
	agent := newTestLandingAgent(func(a *landingAgent) {
		a.now = func() time.Time { return b.now }
		a.proofEffects = b.owners.landing.plainProve
	})
	git := agent.proofEffects.Git
	fails := false
	agent.proofEffects.Git = func(dir string, args ...string) (string, error) {
		if fails && args[0] == "fetch" {
			return "", errors.New("origin unavailable")
		}
		return git(dir, args...)
	}
	pass := func() error {
		k := newLandingAgentKeeper(b.lane, b.home, agent)
		return k.CheckMain(b.record, func() bool { return false })
	}
	helmMust(t, pass())
	seen := b.now
	b.now = seen.Add(30 * time.Second)
	helmMust(t, pass())
	b.now = seen.Add(time.Minute)
	fails = true
	for range 2 {
		if err := pass(); err == nil {
			t.Fatal("fetch error lost")
		}
		b.now = b.now.Add(time.Minute)
	}
	code, out := b.run(t, b.lane, "landing", "status")
	logMainStatus(t, out)
	if code != 0 || !strings.Contains(oneSpaced(out), "origin unreachable since "+seen.Add(time.Minute).Local().Format("15:04")) || !strings.Contains(oneSpaced(out), "main main seen "+seen.Local().Format("15:04")) {
		t.Fatal(out)
	}
}

func TestMovedMainStopHistoryFollowsItsQuestion(t *testing.T) {
	t.Parallel()
	b := newSelectionBed(t)
	stop := plain.Stop{Loop: "lane-proof", Subject: "main", Class: "failed check", Decision: "stop", Handoff: "ask lane", At: b.now.Format(time.RFC3339), Required: []string{"metasystem", "landing", "prove", "--trunk"}}
	data, err := json.Marshal(stop)
	helmMust(t, err)
	helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.lane), "stops.jsonl"), append(bytes.Repeat([]byte("\n"), 723), data...), 0600))
	check := func(history bool) {
		code, out := b.run(t, b.lane, "landing", "status")
		logMainStatus(t, out)
		if code != 0 || history && !strings.Contains(out, "stop S724 for main") || !history && !strings.Contains(out, "the lane stopped") || strings.Contains(out, "holds nothing") != history {
			t.Fatalf("history=%v: %s", history, out)
		}
	}
	check(true)
	machine := func(string) (string, error) { return "fixture", nil }
	b.owners.landing.machine = machine
	helmMust(t, plain.SyncPolicyQuestion(b.lane, machine, b.now))
	check(false)
}

type observerCensus struct{}

func (observerCensus) Workers(string) (steward.Workers, error) { return steward.Workers{}, nil }

// The resident runner drives the same keeper and detached proof admission as the
// command. Only the remote, child launcher and the duration of a cycle are fixtures.
func TestMovedMainResidentRunnerStartsExactlyOneProof(t *testing.T) {
	t.Parallel()
	b := newSelectionBed(t)
	helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.lane), "queue.jsonl"), nil, 0600))
	start := b.now
	var elapsed atomic.Int64
	now := func() time.Time { return start.Add(time.Duration(elapsed.Load())) }
	moved := start.Add(time.Minute + time.Second)
	seedApplicableMainProof(t, b.lane, "main", "main", start)
	ticks := make(chan time.Time)
	passed := make(chan struct{}, 1)
	fetches, starts := 0, 0
	var first time.Time
	a := newTestLandingAgent(func(a *landingAgent) {
		a.now = now
		a.proofEffects = b.owners.landing.plainProve
		a.machine = func(string) (string, error) { return "fixture", nil }
		a.proofEffects.Policy = func(string) (plain.PolicyValue, error) {
			return plain.PolicyValue{Value: "auto", Source: "fixture"}, nil
		}
		a.settings = func(string) (launch.Settings, error) { return launch.DefaultSettings(), nil }
		git := a.proofEffects.Git
		a.proofEffects.Git = func(dir string, args ...string) (string, error) {
			if args[0] == "fetch" {
				fetches++
				return "", nil
			}
			if !now().Before(moved) {
				if args[0] == "rev-parse" {
					if strings.HasSuffix(args[len(args)-1], "^{tree}") {
						return "new-tree", nil
					}
					return "new-main", nil
				}
				if args[0] == "diff" {
					return "source.go", nil
				}
				if args[0] == "show" {
					return "proof.full=fixture-proof", nil
				}
			}
			return git(dir, args...)
		}
		a.proofEffects.Launch = func([]string, string, string) (int64, error) {
			starts++
			first = now()
			return int64(os.Getpid()), nil
		}
	})
	cfg := steward.TickConfig{ProviderHome: b.home, WorkStateRoot: b.home}
	configureLandingRunner(b.lane, b.home, a, &cfg)
	var foreign steward.TickConfig
	configureLandingRunner(b.outside, b.home, a, &foreign)
	if foreign.OwnsLandingLane() {
		t.Fatal("foreign runner claimed the lane")
	}
	helmMust(t, foreign.ObserveMovedMain(func() bool { return false }))
	if fetches != 0 {
		t.Fatal("foreign runner fetched main")
	}
	observe := cfg.ObserveMovedMain
	cfg.ObserveMovedMain = func(stopping func() bool) error {
		err := observe(stopping)
		if err != nil {
			t.Errorf("observer at %v: %v", now().Sub(start), err)
		}
		if fetches > 1 {
			passed <- struct{}{}
		}
		return err
	}
	cfg.RunnerClock = &steward.HandoffClock{Now: now, Sleep: func(d time.Duration) { elapsed.Add(int64(d)) }, Ticker: func(d time.Duration) (<-chan time.Time, func()) {
		if d != 15*time.Second {
			t.Fatalf("timer cadence %v", d)
		}
		return ticks, func() {}
	}}
	cfg.Patterns = func(string, time.Time) error {
		if err := steward.RunLoop(b.lane, observerCensus{}, nil, time.Hour, cfg); err == nil || !strings.Contains(err.Error(), "a runner already guards") {
			t.Fatalf("second runner: %v", err)
		}
		for range 80 {
			elapsed.Add(int64(15 * time.Second))
			ticks <- now()
			<-passed
		}
		return nil
	}
	cfg.FollowDeployedEngine = func() (bool, bool, error) { return true, false, nil }

	if err := steward.RunLoop(b.lane, observerCensus{}, nil, time.Hour, cfg); err != nil {
		t.Fatal(err)
	}
	running, exists, _, err := plain.ReadRunning(b.lane, a.proofEffects)
	if err != nil || !exists || running.Commit != "new-main" || !running.Trunk || starts != 1 || first.Before(moved) || first.Sub(moved) > 15*time.Second || now().Before(start.Add(20*time.Minute)) {
		t.Fatalf("record=%+v, starts=%d first=%v fetches=%d err=%v", running, starts, first, fetches, err)
	}
}

func TestMovedMainStatusWhileAdmissionStarts(t *testing.T) {
	t.Parallel()
	b := newCadenceBed(t, "", 0, false, false, true)
	b.owners.landing.now = func() time.Time { return b.now }
	agent := b.agent
	launch := agent.proofEffects.Launch
	agent.proofEffects.Launch = func(argv []string, dir, log string) (int64, error) {
		code, out := b.run(t, b.lane, "landing", "status")
		logMainStatus(t, out)
		if code != 0 || !strings.Contains(oneSpaced(out), "nothing holds it; its proof starts within 15 seconds") {
			t.Fatalf("active observer: %s", out)
		}
		b.now = b.now.Add(time.Minute)
		code, out = b.run(t, b.lane, "landing", "status")
		logMainStatus(t, out)
		if code != 0 || !strings.Contains(oneSpaced(out), "nothing is watching") || strings.Contains(out, "within 15 seconds") {
			t.Fatalf("expired observer: %s", out)
		}
		return launch(argv, dir, log)
	}
	keeper := newLandingAgentKeeper(b.lane, b.home, agent)
	helmMust(t, keeper.CheckMain(b.record, func() bool { return false }))
	if len(b.argv) != 1 {
		t.Fatalf("starts=%d", len(b.argv))
	}
}

func TestMovedMainDrainingDuringAdmissionStartsNothing(t *testing.T) {
	t.Parallel()
	b := newCadenceBed(t, "", 0, false, false, true)
	agent := b.agent
	stopping := false
	agent.proofEffects.Executable = func() (string, error) { stopping = true; return "fixture-engine", nil }
	keeper := newLandingAgentKeeper(b.lane, b.home, agent)
	helmMust(t, keeper.CheckMain(b.record, func() bool { return stopping }))
	if len(b.argv) != 0 {
		t.Fatalf("starts after drain=%d", len(b.argv))
	}
}
