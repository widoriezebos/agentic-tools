package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	usagepkg "github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

// separatedContextRoots is a checkout whose installation, metasystem/, is not
// the template: the state root resolves to the checkout and the installation
// is the directory beneath it, so the two roots are different directories.
func separatedContextRoots(t *testing.T) (checkout, installation string) {
	t.Helper()
	checkout = t.TempDir()
	installation = filepath.Join(checkout, "metasystem")
	for _, directory := range []string{filepath.Join(checkout, ".git"), installation} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	conf := "context.ceiling.tokens=240000\ncontext.handoff.margin.tokens=140000\n"
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	return checkout, installation
}

// requireSamplesUnderInstallation: the call samples are run state, kept under
// the installation and never under the state root.
func requireSamplesUnderInstallation(t *testing.T, checkout, installation, session string) {
	t.Helper()
	if _, err := os.Stat(usagepkg.SamplesPath(installation, "claude", session)); err != nil {
		t.Fatalf("the call samples are not under the installation: %v", err)
	}
	if _, err := os.Stat(filepath.Join(checkout, "artifacts", "agents", "context")); !os.IsNotExist(err) {
		t.Fatalf("the state root gained context run state: %v", err)
	}
}

func TestContextStatusSeparatedRootsReadsTheInstallation(t *testing.T) {
	checkout, installation := separatedContextRoots(t)
	writeDerivedContextCommandTranscript(t, checkout, "separated", 50000, 1, true)
	code, output, problem := captureChannelOutput(t, func(stdout, stderr io.Writer) int {
		return runContextStatus([]string{"--root", checkout, "--runtime", "claude", "--session", "separated", "--json"}, stdout, stderr)
	})
	var decoded contextStatusOutput
	if code != 0 || problem != "" || json.Unmarshal([]byte(output), &decoded) != nil || decoded.Window.Ceiling != 240000 ||
		decoded.Reading.Latest == nil || decoded.Reading.Latest.PromptTokens != 50000 {
		t.Fatalf("status over separated roots: code=%d stderr=%q output=%q", code, problem, output)
	}
	requireSamplesUnderInstallation(t, checkout, installation, "separated")
}

func TestTurnVerdictContextLineSeparatedRootsReadsTheInstallation(t *testing.T) {
	checkout, installation := separatedContextRoots(t)
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-09-16T12:00:00Z")
	transcript := writeContextCommandTranscript(t, checkout, "separated", 120500, 1, true)
	code, output, problem := captureChannelOutput(t, func(stdout, stderr io.Writer) int {
		return runReportTurnVerdict([]string{"--root", checkout, "--session", "separated", "--transcript", transcript, "--runtime", "claude"}, stdout, stderr)
	})
	var verdict struct {
		Display string `json:"display"`
	}
	want := "CONTEXT: 121K of trigger 100K (proof line 150K, maximum 200K, ceiling 240K)"
	if code != 0 || problem != "" || json.Unmarshal([]byte(output), &verdict) != nil || !strings.Contains(verdict.Display, want) {
		t.Fatalf("turn verdict over separated roots: code=%d stderr=%q display=%q", code, problem, verdict.Display)
	}
	requireSamplesUnderInstallation(t, checkout, installation, "separated")
}
