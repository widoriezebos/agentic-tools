package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// These tests port the command-level scenarios of
// scripts/agents/suite-progress-fixtures.sh. Each drives the owning Go verb
// in-process; the watchdog and launcher scenarios live in internal/proofrun.

// bounded-preserve: the bounded copier names the exact truncated source in
// its loud result and in its durable copy note, keeping the bytes that fit.
func TestSuiteProgressBedBoundedPreserveNamesTheTruncatedSource(t *testing.T) {
	t.Parallel()
	bed := t.TempDir()
	source := filepath.Join(bed, "source")
	if err := os.MkdirAll(source, 0o700); err != nil {
		t.Fatal(err)
	}
	evidence := filepath.Join(source, "evidence")
	if err := os.WriteFile(evidence, []byte("eight-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := filepath.Join(bed, "result")
	code, stdout, _ := captureCommandOutput(t, true, false, func() int {
		return runProofRunPreserve([]string{"--destination", result, "--max-bytes", "4", "--source", source})
	})
	if code != 0 || !strings.Contains(stdout, "DROPPED "+evidence) {
		t.Fatalf("bounded evidence result code=%d did not name the dropped source %s:\n%s", code, evidence, stdout)
	}
	note, err := os.ReadFile(filepath.Join(result, "copy-note.txt"))
	if err != nil || !strings.Contains(string(note), "DROPPED "+evidence) || !strings.Contains(string(note), "copied-bytes=4\n") {
		t.Fatalf("bounded evidence note did not name the dropped source or the retained bytes: %q, %v", note, err)
	}
}

// deepest-heartbeat and background watcher: the public heartbeat verb and
// the background job watcher read the same journal view and both relay the
// deepest open section; the watcher prefixes its one reportable job note.
func TestSuiteProgressBedHeartbeatAndWatcherRelayTheDeepestSection(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	journal := filepath.Join(workspace, "artifacts", "agents", "supervision", "suite-progress.jsonl")
	if err := proofrun.AppendProgressHeader(journal, proofrun.ProgressHeader{LogPaths: []string{"suite.log"}}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, event := range []proofrun.SectionEvent{
		{Suite: "outer", Section: "parent", Event: "start", At: now, Depth: 0},
		{Suite: "inner", Section: "child", Event: "start", At: now, Depth: 1},
	} {
		if err := proofrun.AppendSectionEvent(journal, event); err != nil {
			t.Fatal(err)
		}
	}

	code, heartbeat, _ := captureCommandOutput(t, true, false, func() int {
		return runProofRunHeartbeat([]string{"--root", workspace})
	})
	if code != 0 || !regexp.MustCompile(`^inner:child since [0-9]+min\n$`).MatchString(heartbeat) {
		t.Fatalf("deepest live heartbeat was not selected: code=%d output=%q", code, heartbeat)
	}

	installation := t.TempDir()
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	jobs := filepath.Join(t.TempDir(), "jobs")
	if err := os.MkdirAll(jobs, 0o700); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(jobs, "prefix-job.json")
	if err := os.WriteFile(record, []byte(`{"status":"completed","workspaceRoot":"`+workspace+`"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(t.TempDir(), "watch.state")
	if err := os.WriteFile(state, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	code, watched, _ := captureCommandOutput(t, true, false, func() int {
		return runReportWatchJobs([]string{"--root", installation, "--dir", jobs, "--scope", workspace, "--state", state, "--once"})
	})
	note := regexp.MustCompile(`(?m)^inner:child since [0-9]+min DONE prefix-job status=completed age=[0-9]+m record=` + regexp.QuoteMeta(record) + `$`)
	if code != 0 || len(note.FindAllString(watched, -1)) != 1 || strings.Count(watched, "DONE prefix-job") != 1 {
		t.Fatalf("background watcher did not emit exactly one complete deepest-heartbeat job note: code=%d\n%s", code, watched)
	}
}

// delivery-context: under the delivery contract the real validation
// selector reports the adopted section set, omitting the five template-only
// sections, and the completion check still owes every active section: a
// journal missing only engine-delivery-contract fails by naming it. The
// selector runs with a PATH that holds no Git, so its context never depends
// on one.
func TestSuiteProgressBedDeliveryContextOwesEveryActiveSection(t *testing.T) {
	t.Parallel()
	tools := t.TempDir()
	for _, name := range []string{"bash", "cat", "dirname"} {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Fatalf("locate %s: %v", name, err)
		}
		if err := os.Symlink(path, filepath.Join(tools, name)); err != nil {
			t.Fatal(err)
		}
	}
	selector, err := filepath.Abs(filepath.Join("..", "..", "scripts", "agents", "validate-section-selector.sh"))
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(filepath.Join(tools, "bash"), selector, "list")
	command.Env = []string{"PATH=" + tools, "METASYSTEM_DELIVERY_CONTRACT=1"}
	output, err := command.Output()
	if err != nil {
		t.Fatalf("delivery-context selector list: %v", err)
	}
	var sections []string
	for _, row := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		id, _, found := strings.Cut(row, "\t")
		if !found || id == "" {
			t.Fatalf("selector emitted invalid row %q", row)
		}
		sections = append(sections, id)
	}
	listed := map[string]bool{}
	for _, section := range sections {
		listed[section] = true
	}
	for _, inactive := range []string{"adoption-fixtures", "witness-gate-fixtures", "suite-progress-fixtures", "land-fixtures", "fixture-bed-scenarios-fixtures"} {
		if listed[inactive] {
			t.Fatalf("delivery context retained inactive section %s: %v", inactive, sections)
		}
	}
	if !listed["engine-delivery-contract"] {
		t.Fatalf("delivery context lost the active section engine-delivery-contract: %v", sections)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	journal := func(omit string) proofrun.ProgressRun {
		run := proofrun.ProgressRun{Header: proofrun.ProgressHeader{LogPaths: []string{"suite.log"}}}
		for _, section := range sections {
			if section == omit {
				continue
			}
			run.Events = append(run.Events,
				proofrun.SectionEvent{Suite: "context-active", Section: section, Event: "start", At: now},
				proofrun.SectionEvent{Suite: "context-active", Section: section, Event: "end", At: now})
		}
		return run
	}
	if err := proofrun.AssertSectionProgress(journal(""), "context-active", sections, map[string]bool{}); err != nil {
		t.Fatalf("a journal covering every active section was refused: %v", err)
	}
	err = proofrun.AssertSectionProgress(journal("engine-delivery-contract"), "context-active", sections, map[string]bool{})
	if err == nil || !strings.Contains(err.Error(), "engine-delivery-contract has 0 starts and 0 ends") {
		t.Fatalf("an active delivery section was not required by name: %v", err)
	}
}
