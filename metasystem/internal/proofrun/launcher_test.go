package proofrun

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLaunchSuiteWritesBannerProgressAndReapsWatchdog(t *testing.T) {
	root := t.TempDir()
	watchdog := filepath.Join(root, "watchdog.sh")
	writeExecutable(t, watchdog, `#!/usr/bin/env bash
done_path=
while (($#)); do
  if [[ "$1" == --done ]]; then done_path=$2; shift 2; else shift; fi
done
while [[ ! -e "$done_path" ]]; do sleep 0.01; done
`)
	progress := filepath.Join(root, "progress.jsonl")
	logPath := filepath.Join(root, "logs", "suite.log")
	banner := "suite-cost suite=fixture witness=armed duration=minutes heartbeat=progress.jsonl logs=logs/suite.log"
	sectionCommand := `printf '{"suite":"fixture","section":"only","event":"start","at":"%s","depth":0}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$1"
echo suite-output
printf '{"suite":"fixture","section":"only","event":"end","at":"%s","depth":0}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$1"`
	var output bytes.Buffer
	var errors bytes.Buffer
	result := LaunchSuite(LaunchOptions{
		Suite: "fixture", Root: root, ConfPath: filepath.Join(root, "metasystem.conf"), ProgressPath: progress, LogPath: logPath,
		TmpPaths: []string{filepath.Join(root, "tmp")}, Banner: banner,
		ExpectedSections: []string{"only"}, TwiceConsulted: map[string]bool{},
		Silence: time.Second, SectionCap: time.Second, EvidenceTimeout: time.Second, EvidenceMax: 1024,
		Poll: 10 * time.Millisecond, TermGrace: time.Millisecond, KillGrace: time.Millisecond,
		WatchdogExecutable: watchdog, Command: []string{"bash", "-c", sectionCommand, "fixture", progress},
		Output: &output, ErrorOutput: &errors,
	})
	if result != 0 {
		t.Fatalf("result = %d, output = %q, errors = %q", result, output.String(), errors.String())
	}
	if !strings.Contains(output.String(), banner+"\n") || !strings.Contains(output.String(), "suite-output") {
		t.Fatalf("output = %q", output.String())
	}
	if _, err := os.Stat(logPath + ".done"); !os.IsNotExist(err) {
		t.Fatalf("launcher did not remove reaped watchdog done file: %v", err)
	}
	run, err := ReadLatestProgressRun(progress)
	if err != nil || len(run.Events) != 2 || len(run.Header.TmpPaths) != 1 {
		t.Fatalf("progress run = %+v, %v", run, err)
	}
}

func TestLaunchHelpersRejectBadInputsAndParseSelectorRows(t *testing.T) {
	var errors bytes.Buffer
	if result := LaunchSuite(LaunchOptions{ErrorOutput: &errors}); result != 2 || !strings.Contains(errors.String(), "required") {
		t.Fatalf("invalid result = %d, errors = %q", result, errors.String())
	}
	selector := filepath.Join(t.TempDir(), "selector")
	if err := os.WriteFile(selector, []byte("first\tFirst section\nsecond\tSecond section\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sections, err := ReadSelectorSections(selector)
	if err != nil || strings.Join(sections, ",") != "first,second" {
		t.Fatalf("sections = %v, %v", sections, err)
	}
	if err := os.WriteFile(selector, []byte("invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSelectorSections(selector); err == nil {
		t.Fatal("invalid selector row passed")
	}
	if exitStatus(nil) != 0 || exitStatus(os.ErrInvalid) != 1 {
		t.Fatal("exit status helper returned an invalid result")
	}
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
}
