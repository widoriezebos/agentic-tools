package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// writeTemp writes a JSON file and returns its path.
func writeTemp(t *testing.T, dir, name string, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// captureStdout runs fn with stdout redirected and returns what it printed.
func captureStdout(t *testing.T, fn func() int) (string, int) {
	t.Helper()
	original := os.Stdout
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = write
	code := fn()
	write.Close()
	os.Stdout = original
	out, _ := io.ReadAll(read)
	return string(out), code
}

func TestCommandTaggedProcessScannerHonorsEmptyConfiguredUniverse(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	processes := writeTemp(t, t.TempDir(), "processes.json", []any{})
	identities := writeTemp(t, t.TempDir(), "identities.json", map[string]any{})
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", processes)
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", identities)

	result := (commandTaggedProcessScanner{root: root}).ScanTag("metasystem-job-empty-nonce", time.Time{})
	if !result.Complete() || result.EnumerationError != "" || len(result.Tagged) != 0 {
		t.Fatalf("empty configured process universe was not a complete absence proof: %+v", result)
	}
}

// TestDispatchRecordVerbsPath drives the whole record lifecycle through the CLI
// verbs the shell invokes, proving the flag parsing, exit-code mapping, and the
// lost-compare stdout witness all work end to end.
func TestDispatchRecordVerbsPath(t *testing.T) {
	root := t.TempDir()
	tmp := t.TempDir()
	job := "job-cli"

	create := writeTemp(t, tmp, "create.json", map[string]any{
		"jobId": job, "status": "pending-setup", "mainId": "main-1", "claimEpoch": 7,
	})
	if code := runDispatchRecordCreate([]string{"--root", root, "--job", job, "--source", create}); code != 0 {
		t.Fatalf("record-create exit = %d, want 0", code)
	}
	// A second create on the same id is a collision (exit 1).
	if code := runDispatchRecordCreate([]string{"--root", root, "--job", job, "--source", create}); code != 1 {
		t.Fatalf("record-create collision exit = %d, want 1", code)
	}

	setup := writeTemp(t, tmp, "setup.json", map[string]any{
		"jobId": job, "status": "pending", "mainId": "main-1", "claimEpoch": 7,
		"startedAt": "2026-08-10T00:00:00Z",
	})
	if code := runDispatchRecordSetup([]string{"--root", root, "--job", job, "--source", setup}); code != 0 {
		t.Fatalf("record-setup exit = %d, want 0", code)
	}

	run := writeTemp(t, tmp, "run.json", map[string]any{"sessionId": "s"})
	if code := runDispatchRecordCAS([]string{"--root", root, "--job", job, "--expect", "pending", "--status", "running", "--patch", run}); code != 0 {
		t.Fatalf("record-cas pending->running exit = %d, want 0", code)
	}

	// A lost compare prints the observed status on stdout and exits 3.
	stale := writeTemp(t, tmp, "stale.json", map[string]any{"note": "x"})
	out, code := captureStdout(t, func() int {
		return runDispatchRecordCAS([]string{"--root", root, "--job", job, "--expect", "pending", "--status", "failed", "--patch", stale})
	})
	if code != 3 {
		t.Fatalf("stale record-cas exit = %d, want 3", code)
	}
	if strings.TrimSpace(out) != "observed=running" {
		t.Fatalf("stale record-cas stdout = %q, want observed=running", out)
	}

	// A missing required flag is a usage error (exit 2).
	if code := runDispatchRecordCAS([]string{"--root", root, "--job", job, "--expect", "running"}); code != 2 {
		t.Fatalf("record-cas missing-flags exit = %d, want 2", code)
	}
}

func TestDispatchCritiqueAdvanceVerbsPath(t *testing.T) {
	repo := t.TempDir()
	agents := filepath.Join(repo, "artifacts", "agents")
	jobs := filepath.Join(agents, "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	facts := map[string]any{
		"local": true, "recoverable": true,
		"proofBoundaryCrossed": false, "authorityBoundaryCrossed": false,
		"secretsBoundaryCrossed": false, "irreversibleDataBoundaryCrossed": false,
		"externalSideEffectBoundaryCrossed": false,
	}
	for round := 1; round <= 3; round++ {
		job := "critic"
		parent := any(nil)
		if round > 1 {
			job = "critic-r" + strconv.Itoa(round)
			if round == 2 {
				parent = "critic"
			} else {
				parent = "critic-r2"
			}
		}
		record := map[string]any{
			"jobId": job, "role": "design-critic", "round": round,
			"parentJob": parent, "status": "completed",
		}
		if round == 1 {
			record["findingRegister"] = []any{}
			record["findingRegisterRound"] = 0
			record["boundedCritiqueStart"] = nil
			record["critiqueExhaustions"] = []any{}
		}
		writeTemp(t, jobs, job+".json", record)
		findings := []any{}
		rigor := []any{}
		if round == 1 {
			findings = []any{map[string]any{
				"id": "S-1", "severity": "high", "material": true,
				"claim": "severe finding", "evidence": "direct evidence",
			}}
			rigor = []any{map[string]any{
				"findingId": "S-1", "rigorClass": "severe", "facts": facts,
				"reopeningTrigger": "reopen if the defect recurs",
			}}
		}
		roundDir := filepath.Join(agents, "critic", "rounds", strconv.Itoa(round))
		if err := os.MkdirAll(roundDir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeTemp(t, roundDir, "return.json", map[string]any{
			"schemaVersion": 3, "jobId": job, "round": round,
			"findings": findings, "rigor": rigor,
		})
		out, code := captureStdout(t, func() int {
			return runDispatchCritiqueRegisterAdvance([]string{"--repo", repo, "--root-job", "critic", "--round-job", job})
		})
		if code != 0 || strings.TrimSpace(out) != "advanced" {
			t.Fatalf("register round %d: exit=%d out=%q", round, code, out)
		}
	}
	out, code := captureStdout(t, func() int {
		return runDispatchCritiqueOpenFindingIDs([]string{"--repo", repo, "--root-job", "critic"})
	})
	if code != 0 || strings.TrimSpace(out) != "S-1" {
		t.Fatalf("open finding identifiers: exit=%d out=%q", code, out)
	}
	message := filepath.Join(t.TempDir(), "message.md")
	if err := os.WriteFile(message, []byte("Address S-1.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code = captureStdout(t, func() int {
		return runDispatchCritiqueExhaustionAdvance([]string{
			"--repo", repo, "--root-job", "critic", "--role", "design-critic",
			"--message", message, "--successor", "critic-r4",
		})
	})
	if code != 0 || strings.TrimSpace(out) != "recorded" {
		t.Fatalf("exhaustion advance: exit=%d out=%q", code, out)
	}
	rootRecord, err := os.ReadFile(filepath.Join(jobs, "critic.json"))
	if err != nil || !strings.Contains(string(rootRecord), `"successorJobId": "critic-r4"`) {
		t.Fatalf("exhaustion was not written directly: %v, %s", err, rootRecord)
	}
	if code := runDispatchCritiqueRegisterAdvance([]string{"--repo", repo}); code != 2 {
		t.Fatalf("register usage error exit=%d, want 2", code)
	}
	if code := runDispatchCritiqueOpenFindingIDs([]string{"--repo", repo}); code != 2 {
		t.Fatalf("open finding identifiers usage error exit=%d, want 2", code)
	}
}
