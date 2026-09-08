package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

func captureMissionStderr(t *testing.T, run func() int) (string, int) {
	t.Helper()
	original := os.Stderr
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = write
	code := run()
	_ = write.Close()
	os.Stderr = original
	output, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}
	return string(output), code
}

func missionFenceFixture(t *testing.T, terminal bool) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "metasystem"), []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "-C", root, "init", "-q")
	command.Env = gittree.ScrubbedEnviron()
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	root, err := canonicalPath(root)
	if err != nil {
		t.Fatal(err)
	}
	table := filepath.Join(t.TempDir(), "identities.json")
	body := fmt.Sprintf(`{"%d":{"terminal":%t}}`, os.Getpid(), terminal)
	if err := os.WriteFile(table, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", table)
	return root
}

func TestMissionLaunchHumanOpensClosedFenceBeforeArming(t *testing.T) {
	for _, mode := range []string{"start", "resume"} {
		t.Run(mode, func(t *testing.T) {
			root := missionFenceFixture(t, true)
			record := stopfence.Record{
				State: stopfence.StateClosed, Phase: stopfence.PhaseStopped,
				Generation: 9, ChangedAt: "2026-09-07T09:30:00Z", Checkout: root,
				NotStopped: []stopfence.Survivor{},
				By:         stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 4321}},
			}
			if err := stopfence.Write(root, record); err != nil {
				t.Fatal(err)
			}
			generation, code := missionFenceBeforeArm(root, mode)
			if code != 0 || generation != 10 {
				t.Fatalf("human handover = generation %d code %d", generation, code)
			}
			opened, err := stopfence.Read(root)
			if err != nil || opened.State != stopfence.StateOpen || opened.Phase != stopfence.PhaseArmed || opened.By.Verb != "mission-"+mode {
				t.Fatalf("opened fence = %#v err=%v", opened, err)
			}
		})
	}
}

func TestMissionLaunchNonHumanKeepsStoppedRefusalAheadOfArming(t *testing.T) {
	root := missionFenceFixture(t, false)
	if err := stopfence.Write(root, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopped,
		Generation: 9, ChangedAt: "2026-09-07T09:30:00Z", Checkout: root,
		NotStopped: []stopfence.Survivor{},
		By:         stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 4321}},
	}); err != nil {
		t.Fatal(err)
	}
	output, code := captureMissionStderr(t, func() int {
		_, code := missionFenceBeforeArm(root, "resume")
		return code
	})
	expected := "the metasystem is stopped for " + root + " since 2026-09-07T09:30:00Z, by stop pid 4321\n" +
		"at an agent-free terminal, run: metasystem mission resume --root " + root + " --mission <id>\n"
	if code == 0 || output != expected {
		t.Fatalf("non-human stopped refusal = code %d output %q, want %q", code, output, expected)
	}
	record, err := stopfence.Read(root)
	if err != nil || record.State != stopfence.StateClosed {
		t.Fatalf("non-human changed fence = %#v err=%v", record, err)
	}
}

func TestMissionLaunchClassificationDataFailureNamesRepairBeforeRetry(t *testing.T) {
	root := missionFenceFixture(t, true)
	if err := stopfence.Write(root, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopped,
		Generation: 9, ChangedAt: "2026-09-07T09:30:00Z", Checkout: root,
		By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 4321}},
	}); err != nil {
		t.Fatal(err)
	}
	jobPath := filepath.Join(root, "artifacts", "agents", "jobs", "damaged.json")
	if err := os.MkdirAll(filepath.Dir(jobPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jobPath, []byte("{\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	printedJobPath, err := canonicalPath(jobPath)
	if err != nil {
		t.Fatal(err)
	}
	fencePath := stopfence.TransitionPath(root)
	before, err := os.ReadFile(fencePath)
	if err != nil {
		t.Fatal(err)
	}
	output, code := captureMissionStderr(t, func() int {
		_, code := missionFenceBeforeArm(root, "resume")
		return code
	})
	want := "metasystem mission resume: caller classification is blocked by job record " + printedJobPath + ": invalid JSON: unexpected end of JSON input.\n" +
		"repair " + printedJobPath + ", then at an agent-free terminal, run: metasystem mission resume --root " + root + " --mission <id>\n"
	if code != 1 || output != want {
		t.Fatalf("mission classification refusal = code %d output %q, want %q", code, output, want)
	}
	after, err := os.ReadFile(fencePath)
	if err != nil || string(after) != string(before) {
		t.Fatalf("mission classification refusal changed the fence: %v", err)
	}
}

func TestMissionFenceClassificationUsesTheNestedInstallationAndKeepsFenceClosed(t *testing.T) {
	root := t.TempDir()
	if output, err := fixtureGitCommand("init", "--quiet", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	root, err := canonicalPath(root)
	if err != nil {
		t.Fatal(err)
	}
	installation := filepath.Join(root, "metasystem")
	if err := os.MkdirAll(filepath.Join(installation, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installation, "bin", "metasystem"), []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	adapter := filepath.Join(installation, "scripts", "agents", "adapters", "fake.sh")
	if err := os.MkdirAll(filepath.Dir(adapter), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(adapter, []byte("#!/bin/sh\n[ \"$1\" = signature ] && printf 'match .*\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	table := filepath.Join(t.TempDir(), "identities.json")
	if err := os.WriteFile(table, []byte(fmt.Sprintf(`{"%d":{"terminal":false}}`, os.Getpid())), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", table)
	if err := stopfence.Write(root, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 4,
		ChangedAt: "2026-09-08T04:00:00Z", Checkout: root,
		By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 72}},
	}); err != nil {
		t.Fatal(err)
	}
	previous := classifyProcessVerbCaller
	var gotRoot, gotInstallation string
	classifyProcessVerbCaller = func(stateRoot, installed string, _ int64) (lease.Classification, error) {
		gotRoot, gotInstallation = stateRoot, installed
		return lease.ClassifyAt(stateRoot, installed, int64(os.Getpid()))
	}
	t.Cleanup(func() { classifyProcessVerbCaller = previous })
	_, code := missionFenceBeforeArm(root, "start")
	if code != 1 || gotRoot != root || gotInstallation != installation {
		t.Fatalf("mission classifier root=%q installation=%q code=%d, want %q and %q", gotRoot, gotInstallation, code, root, installation)
	}
	classification, err := lease.ClassifyAt(root, installation, int64(os.Getpid()))
	if err != nil || classification.Class != lease.ClassDelegate {
		t.Fatalf("nested installation did not classify the mission caller as an agent: %+v, %v", classification, err)
	}
	record, err := stopfence.Read(root)
	if err != nil || record.State != stopfence.StateClosed || record.Generation != 4 {
		t.Fatalf("agent-classified mission changed the fence: %#v, %v", record, err)
	}
}

// The public resolve-taint parser speaks exactly the design grammar:
// one typed act — either --restore <treeId> or --adopt — beside the
// taint id; anything else is a usage refusal (exit 2) before any
// engine work.
func TestResolveTaintParserGrammar(t *testing.T) {
	refusals := [][]string{
		{},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--restore", "abc", "--adopt"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "0", "--adopt"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "x", "--adopt"},
		{"--root", "/nonexistent", "--mission", "alpha", "--restore", "abc"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--variant", "restore", "--tree", "abc"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--restore", "abc", "--reason", "r"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--restore", "abc", "--by", "Wido"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--adopt", "--by", "Wido", "--reason", "r"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--adopt", "--by", "Wido", "--reason", "r", "--waives", "   "},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--restore", "abc", "--by", "Wido", "--reason", "r", "--waives", "claim"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--adopt", "--by", "  ", "--reason", "r", "--waives", "claim"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--adopt", "--by", "--reason", "--reason", "restored", "--waives", "claim"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--adopt", "--by", "\uFEFF", "--reason", "r", "--waives", "claim"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--restore", "not-a-tree", "--by", "Wido", "--reason", "r"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--restore", "--reason", "--restore", "0123456789012345678901234567890123456789", "--by", "Wido", "--reason", "restored"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--adopt", "--by", "--adopt", "Wido", "--reason", "r", "--waives", "claim"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--adopt", "--adopt", "--by", "Wido", "--reason", "r", "--waives", "claim"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--adopt", "--by", "\u2061", "--reason", "r", "--waives", "claim"},
	}
	for _, args := range refusals {
		if code := runMissionRunnerResolveTaint(args); code != 2 {
			t.Fatalf("args %v must refuse as usage (2), got %d", args, code)
		}
	}
	// Both typed acts PARSE — the engine then refuses on the fake root
	// with its own exit (3), proving the grammar accepted the shape.
	accepted := [][]string{
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--restore", "0123456789012345678901234567890123456789", "--by", "Wido", "--reason", "r"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--adopt", "--by", "Wido", "--reason", "r", "--waives", "claim"},
	}
	for _, args := range accepted {
		if code := runMissionRunnerResolveTaint(args); code == 2 {
			t.Fatalf("args %v must parse past usage, got 2", args)
		}
	}
}
