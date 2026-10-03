package missionrunner

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// These tests port scripts/agents/mission-fixtures.sh. The bash bed built a
// real Git checkout with a bare origin, live supervisor holders, and a real
// runner process; here the launch gate runs in-process on the Git-free
// preflight bed (the contract source stands in for every Git read), and the
// runner end states and stop conclusions are read through their owners.

// The candidate-bad gate is the bash bed's frozen instrument: it measures a
// clean candidate and refuses to measure one carrying candidate-bad.
const missionBedCandidateBadGate = "#!/usr/bin/env bash\nset -euo pipefail\n[[ ! -e candidate-bad ]] || exit 4\nprintf 'metric=score=1\\nmetric=audit=1\\n'\n"

// contract-and-state preflight refusals: with a sealed and signed contract
// on origin and fresh supervision, the launch gate refuses an unarmed
// supervisor set, a mission lease another holder owns, and a candidate the
// frozen gate cannot measure, each by name.
func TestMissionBedPreflightRefusesByName(t *testing.T) {
	t.Parallel()
	t.Run("unarmed supervisor set", func(t *testing.T) {
		engine, source := newGitFreePreflightBed(t, "")
		t.Cleanup(source.done)
		state := filepath.Join(engine.installation(), "artifacts", "agents", "supervision", "state.json")
		if err := os.Rename(state, state+".unarmed"); err != nil {
			t.Fatal(err)
		}
		if err := engine.armAndPreflight("start"); err == nil || !strings.Contains(err.Error(), "supervisor set is unarmed") {
			t.Fatalf("an unarmed supervisor set was not refused by name: %v", err)
		}
		if err := os.Rename(state+".unarmed", state); err != nil {
			t.Fatal(err)
		}
		if err := engine.armAndPreflight("start"); err != nil {
			t.Fatalf("the rearmed supervisor set still refused: %v", err)
		}
	})
	t.Run("held mission lease", func(t *testing.T) {
		engine, source := newGitFreePreflightBed(t, "")
		t.Cleanup(source.done)
		held := filepath.Join(engine.missionDir(), "lease.d")
		if err := os.MkdirAll(held, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := engine.armAndPreflight("start"); err == nil || !strings.Contains(err.Error(), "lease is not acquirable") {
			t.Fatalf("a held mission lease was not refused by name: %v", err)
		}
		if _, err := os.Stat(held); err != nil {
			t.Fatalf("the preflight removed another holder's lease marker: %v", err)
		}
	})
	t.Run("unmeasurable candidate", func(t *testing.T) {
		engine, source := newGitFreePreflightBedWithGate(t, "", []byte(missionBedCandidateBadGate))
		t.Cleanup(source.done)
		source.candidateSHA = "dddddddddddddddddddddddddddddddddddddddd"
		source.candidateFiles = map[string][]byte{
			"scripts/gate.sh":     source.files["scripts/gate.sh"],
			"truth/reference.txt": source.files["truth/reference.txt"],
			"candidate-bad":       []byte("bad candidate\n"),
		}
		if err := engine.armAndPreflight("start"); err == nil || !strings.Contains(err.Error(), "gate measurement failed") {
			t.Fatalf("an unmeasurable candidate was not refused by name: %v", err)
		}
	})
}

// runner-end-state: the gate-and-close and runner-closes-chain missions run
// one fake-host turn to completion, and the driver-facing status reports
// that terminal as completed with exit 10.
func TestMissionBedEndStatesReportCompleted(t *testing.T) {
	t.Parallel()
	for _, behavior := range []string{"FAKEHOST:close-stream", "FAKEHOST:dispatch-terminal"} {
		t.Run(behavior, func(t *testing.T) {
			engine := buildGitFreeHostCycle(t, behavior)
			signal := filepath.Join(t.TempDir(), "start.json")
			engine.internalRun("start", "metasystem-mission-runner-alpha-fixture", signal)
			var output bytes.Buffer
			engine.Output = &output
			if code := engine.Status(); code != 10 || !strings.HasPrefix(output.String(), "mission=alpha status=completed ") {
				t.Fatalf("%s end state = exit %d %q, want completed exit 10", behavior, code, output.String())
			}
		})
	}
}

// mission-stop status: a running mission whose runner record the stop
// concluded reports stopped by the metasystem stop, exit 13, so a driver
// knows to resume rather than wait.
func TestMissionBedStoppedRunnerStatusNamesTheStop(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	engine := &Engine{Root: root, Mission: "cooperating-host"}
	contract := "```mission\ncandidate.branch=main\nstream.primary=Do the work\n```\n```mission-seal\ncandidate.branch=main\n```\n"
	writeText(t, engine.approvedContractPath(), contract)
	sum := sha256.Sum256([]byte(contract))
	writeJSONFile(t, engine.fencesPath(), map[string]any{"cycles": 0, "approvedContractSha256": hex.EncodeToString(sum[:])})
	ledger := filepath.Join(engine.missionDir(), "ledger.md")
	if err := mission.InitLedger(ledger, 5, 3); err != nil {
		t.Fatal(err)
	}
	origins := map[string]any{
		"headCommit": recoveryHead, "topTree": nil, "topStaged": nil,
		"refMap":         map[string]any{"refs/heads/main": recoveryHead},
		"worktreeCensus": []any{}, "capturedAt": "2026-09-27T00:00:00Z",
	}
	statePath := filepath.Join(engine.missionDir(), "state.json")
	if err := mission.InitStateWithBaseline(statePath, engine.approvedContractPath(), ledger, "", "main", recoveryPre, origins); err != nil {
		t.Fatal(err)
	}
	if state := readTestDoc(t, statePath); state["status"] != "running" {
		t.Fatalf("born mission state status = %v, want running", state["status"])
	}
	recordPath, _, _ := engine.runnerPaths()
	writeJSONFile(t, recordPath, map[string]any{"missionId": engine.Mission, "status": "stopped", "error": nil})
	var output bytes.Buffer
	engine.Output = &output
	if code := engine.Status(); code != 13 || output.String() != "mission=cooperating-host status=stopped reason=metasystem-stop\n" {
		t.Fatalf("stopped mission status = exit %d %q, want exit 13 naming the stop", code, output.String())
	}
}

// ignores-term, turn half: when the runner concluded under the stop and
// itself killed its TERM-ignoring host, the stop reads the recorded kill and
// reports the turn as stopped by the runner, sending nothing further.
func TestMissionBedRunnerKilledHostIsReportedAsTheRunners(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	recordPath := filepath.Join(root, "artifacts", "agents", "missions", "ignores-term", "turns", "ignores-term-t1", "turn.json")
	record := map[string]any{
		"missionId": "ignores-term", "turnId": "ignores-term-t1", "runtime": "fake",
		"status": "failed", "outcome": "failed", "error": "turn-lost",
		"detail": "stopped by metasystem system stop", "hostTermination": TerminationKill,
	}
	writeJSONFile(t, recordPath, record)
	item := Item{Kind: ItemTurn, Root: root, MissionID: "ignores-term", TurnID: "ignores-term-t1", Runtime: "fake",
		RecordPath: recordPath, Pid: 999960, PidStartedAt: 1, Pgid: 999960, Tag: "fixture-host",
		Liveness: identity.Alive.String()}
	outcome, err := Stop(item, StopOptions{RunnerConcluded: true})
	if err != nil || outcome.Result != "stopped" || outcome.Signal != TerminationKill || !outcome.ByRunner {
		t.Fatalf("runner-killed host outcome = %+v, %v; want stopped by the runner with a kill", outcome, err)
	}
	if after := readTestDoc(t, recordPath); after["hostTermination"] != TerminationKill || after["error"] != "turn-lost" {
		t.Fatalf("the stop rewrote the runner's turn conclusion: %v", after)
	}
}
