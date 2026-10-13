package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

type shadowCorrectionBed struct {
	*laneVerbBed
	owners intentOwners
}

func (b *shadowCorrectionBed) run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	command, ok := findIntentAction(args[0], args[1])
	if !ok {
		t.Fatalf("missing public action: %v", args)
	}
	var out, err bytes.Buffer
	code := runIntentIn(command, args[2:], &out, &err, b.cwd, b.owners)
	return code, out.String(), err.String()
}

func shadowCorrectionVerbBed(t *testing.T) *shadowCorrectionBed {
	t.Helper()
	root, home := realpath.Resolve(t.TempDir()), realpath.Resolve(t.TempDir())
	b := &shadowCorrectionBed{laneVerbBed: &laneVerbBed{cwd: root, landingA: root, home: home}}
	install := filepath.Join(root, "metasystem")
	for _, dir := range []string{install, filepath.Join(root, ".git"), lane.HostDir(home)} {
		helmMust(t, os.MkdirAll(dir, 0700))
	}
	helmMust(t, os.WriteFile(filepath.Join(install, "metasystem.conf"), []byte("metasystem.template=true\n"), 0600))
	data, err := json.Marshal(lane.Record{Root: root, Install: install, CustodyEpoch: 1, RegisteredBy: "Wido", At: laneTestNow.Format(time.RFC3339Nano)})
	helmMust(t, err)
	helmMust(t, os.WriteFile(lane.RecordPath(home), data, 0600))
	b.plainProve.Git = func(_ string, args ...string) (string, error) {
		if args[0] == "rev-parse" || args[0] == "symbolic-ref" {
			return "main", nil
		}
		return "", nil
	}
	b.plainProve.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil }
	b.policies.Registry = func(string) (config.PolicyRegistry, error) { return config.PolicyRegistry{Lane: root}, nil }
	b.policies.ConfPath = func(string) (string, error) { return filepath.Join(install, "metasystem.conf"), nil }
	b.owners = b.laneVerbBed.owners()
	b.owners.resolver = stateroot.NewResolver(func(string) (string, error) { return root, nil }, os.Executable)
	return b
}

func TestLandingShadowResumeRecordFailureIsFinding(t *testing.T) {
	t.Parallel()
	b := shadowCorrectionVerbBed(t)
	record, _, err := lane.Read(b.home)
	helmMust(t, err)
	if _, err := lane.SetPause(b.home, "Wido", laneTestNow); err != nil {
		t.Fatal(err)
	}
	// A directory at the shadow-only record path makes its replacement fail.
	helmMust(t, os.Mkdir(filepath.Join(lane.HostDir(b.home), "landing-lane-resumed.json"), 0500))
	code, text, stderr := b.run(t, "landing", "start")
	if code != 0 || !strings.Contains(text+stderr, "the resume time could not be recorded:") {
		t.Fatalf("resume refused: %d %s %s", code, text, stderr)
	}
	if _, paused := lane.ReadPause(b.home); paused {
		t.Fatal("resume kept pause")
	}
	code, text, stderr = b.run(t, "landing", "status", "--json")
	var result struct{ Data plain.Status }
	helmMust(t, json.Unmarshal([]byte(text), &result))
	if code != 0 || len(result.Data.Problems) != 0 || !strings.Contains(strings.Join(result.Data.Findings, ";"), "the resume time could not be recorded:") || result.Data.Paused {
		t.Fatalf("resume finding missing: %d %s %s", code, text, stderr)
	}
	shadow, exists, err := state.Read(plain.Dir(record.Install))
	if err != nil || !exists || shadow.Lane.Word != "running" || !shadow.Lane.Since.Observed {
		t.Fatalf("resume fallback: %+v %v %v", shadow.Lane, exists, err)
	}
	code, text, stderr = b.run(t, "landing", "health")
	if code != 0 || !strings.Contains(text+stderr, "the resume time could not be recorded:") {
		t.Fatalf("health lost advisory resume failure: %d %s %s", code, text, stderr)
	}
	// A later successful resume clears the advisory failure.
	helmMust(t, os.Remove(filepath.Join(lane.HostDir(b.home), "landing-lane-resumed.json")))
	_, err = lane.SetPause(b.home, "Wido", laneTestNow)
	helmMust(t, err)
	code, text, stderr = b.run(t, "landing", "start")
	if code != 0 {
		t.Fatalf("second resume: %d %s %s", code, text, stderr)
	}
	status := plain.ObserveStatus(b.home, record, b.plainProve, nil)
	if len(status.Findings) != 0 || status.Raw.Resume.At == "" {
		t.Fatalf("successful resume kept finding: %+v", status)
	}
}

func TestLandingShadowVerbAndKeeperDriftReport(t *testing.T) {
	t.Parallel()
	b := shadowCorrectionVerbBed(t)
	record, _, err := lane.Read(b.home)
	helmMust(t, err)
	_, err = plain.RefreshShadow(b.home, laneTestNow, "keeper", b.plainProve)
	helmMust(t, err)
	for _, action := range []string{"stop", "start", "health"} {
		code, text, stderr := b.run(t, "landing", action)
		if code != 0 {
			t.Fatalf("%s: %d %s %s", action, code, text, stderr)
		}
	}
	events, skipped, err := state.ReadEvents(plain.Dir(record.Install))
	helmMust(t, err)
	actors := []string{}
	for _, event := range events {
		if event.Kind == "drift" {
			actors = append(actors, event.By)
		}
	}
	if skipped != 0 || len(actors) != 0 {
		t.Fatalf("healthy verbs emitted drift: %v", actors)
	}
	report := func(now time.Time) string {
		t.Helper()
		status := plain.ObserveStatus(b.home, record, b.plainProve, nil)
		_, err := plain.StewardReport(record.Install, record.Root, status, nil, now, b.plainProve, launch.Store{Root: t.TempDir()})
		helmMust(t, err)
		data, err := os.ReadFile(filepath.Join(plain.Dir(record.Install), "steward-report.md"))
		helmMust(t, err)
		_, drift, _ := strings.Cut(string(data), "### drift\n")
		drift, _, _ = strings.Cut(drift, "### Tokens")
		return strings.TrimSpace(drift)
	}
	if got := report(laneTestNow); got != "" {
		t.Fatalf("verbs appeared in keeper report: %s", got)
	}
	data, err := json.Marshal(lane.Pause{By: "hand edit", At: laneTestNow.Add(time.Minute).Format(time.RFC3339Nano)})
	helmMust(t, err)
	helmMust(t, os.WriteFile(filepath.Join(lane.HostDir(b.home), "landing-lane-paused.json"), data, 0600))
	_, err = plain.RefreshShadow(b.home, laneTestNow.Add(time.Minute), "keeper", b.plainProve)
	helmMust(t, err)
	events, _, err = state.ReadEvents(plain.Dir(record.Install))
	helmMust(t, err)
	count := 0
	for _, event := range events {
		if event.Kind == "drift" && event.By == "keeper" {
			count++
			if !strings.Contains(event.Evidence, "lane.") {
				t.Fatal(event)
			}
		}
	}
	if count != 1 || !strings.Contains(report(laneTestNow.Add(time.Hour)), "lane.word: running → paused") {
		t.Fatalf("keeper drift missing: count=%d", count)
	}
}

func TestLandingHealthFailedObservationCLIExitsOne(t *testing.T) {
	t.Parallel()
	b := newPlainVerbBed(t)
	_, err := plain.RefreshShadow(b.home, laneTestNow, "fixture", b.owners.landing.plainProve)
	helmMust(t, err)
	helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.installation), "queue.jsonl"), []byte(`{torn`), 0600))
	for _, format := range []string{"text", "json"} {
		args := []string{"landing", "health"}
		if format == "json" {
			args = append(args, "--json")
		}
		cmd := exec.CommandContext(t.Context(), os.Getenv("METASYSTEM_WAIT_BINARY"), args...)
		cmd.Dir = b.checkout
		cmd.Env = fixtureCommandEnvironment(t, "METASYSTEM_SUPERVISION_REGISTRY_HOME="+b.registry)
		out, err := cmd.CombinedOutput()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 || !strings.Contains(string(out), "the lane health cannot be read") || !strings.Contains(string(out), "observation-failed") || !strings.Contains(string(out), "queue") {
			t.Fatalf("health %s: %v %s", format, err, out)
		}
	}
	helmMust(t, os.Remove(filepath.Join(plain.Dir(b.installation), "queue.jsonl")))
	helmMust(t, os.WriteFile(filepath.Join(lane.HostDir(b.home), "landing-lane-resumed.json"), []byte(`{torn`), 0600))
	file, err := os.OpenFile(filepath.Join(plain.Dir(b.installation), "events.jsonl"), os.O_WRONLY|os.O_APPEND, 0600)
	helmMust(t, err)
	_, err = file.WriteString(`{"kind":`)
	helmMust(t, errors.Join(err, file.Close()))
	cmd := exec.CommandContext(t.Context(), os.Getenv("METASYSTEM_WAIT_BINARY"), "landing", "health")
	cmd.Dir = b.checkout
	cmd.Env = fixtureCommandEnvironment(t, "METASYSTEM_SUPERVISION_REGISTRY_HOME="+b.registry)
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "resume record cannot be read") || !strings.Contains(string(out), "events.jsonl: 1 undecodable line skipped") {
		t.Fatalf("shadow-only corruption blocked health: %v %s", err, out)
	}
	events, skipped, err := state.ReadEvents(plain.Dir(b.installation))
	var rebuilt state.Event
	for _, event := range events {
		if event.Rebuild {
			rebuilt = event
		}
	}
	if err != nil || skipped != 1 || !strings.Contains(rebuilt.Evidence, "events.jsonl: 1 undecodable line skipped") {
		t.Fatalf("CLI refresh lost damage evidence: %+v %d %v", events, skipped, err)
	}
}
