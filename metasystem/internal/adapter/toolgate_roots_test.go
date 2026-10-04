package adapter

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

// TestToolGateSeparatedRootsDecidesFromTheInstallation runs the gate where the
// installation and the state root are different directories. The thresholds
// come from the installation's metasystem.conf: a ceiling of 120K and a margin
// of 40K put the trigger at 80K, where the defaults put it at 105K, so a call at
// 85K is denied only when both settings are read. The call samples the gate
// reads and the rows it writes are run state under the installation's
// artifacts/agents/context, and nothing is created in the state root.
func TestToolGateSeparatedRootsDecidesFromTheInstallation(t *testing.T) {
	t.Parallel()
	installation, state, transcripts := t.TempDir(), t.TempDir(), t.TempDir()
	conf := "context.toolgate.mode=deny\ncontext.ceiling.tokens=120000\ncontext.handoff.margin.tokens=40000\n"
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	birth := time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)
	transcriptOf := func(session string, tokens int64) string {
		transcript := filepath.Join(transcripts, session+".jsonl")
		writeToolGateTranscript(t, transcript, session, tokens)
		return transcript
	}
	gate := func(session, transcript string) string {
		t.Helper()
		stdout := &bytes.Buffer{}
		opts := ToolGateOptions{
			ShellStartedAt: birth, Clock: func() time.Time { return birth.Add(time.Millisecond) }, Mode: "deny",
			Installation: stateroottest.Installation(t, installation),
			Stdin:        bytes.NewReader(toolGatePayloadBytes(session, transcript, bashToolGateCall("rm x"), "")),
			Stdout:       stdout, Stderr: &bytes.Buffer{},
		}
		if err := RunToolGate(opts); err != nil {
			t.Fatalf("session %s: %v", session, err)
		}
		return stdout.String()
	}

	if out := gate("under", transcriptOf("under", 79000)); out != "" {
		t.Fatalf("a call at 79K, under the configured trigger, was answered %q; want it allowed silently", out)
	}
	out := gate("over", transcriptOf("over", 85000))
	want := "CONTEXT AT 85K (trigger 80K): this call is denied; run metasystem session handoff --root " + installation + " alone"
	if !strings.Contains(out, `"permissionDecision":"deny"`) || !strings.Contains(out, want) {
		t.Fatalf("a call at 85K = %q; want a denial naming %q", out, want)
	}

	// The call store under the installation already holds the 85K sample of
	// this session, so the gate finds no new sample and allows. Had it read its
	// samples anywhere else, it would have read the 85K call and denied it.
	seeded := transcriptOf("seeded", 85000)
	if _, err := usage.LatestCall(stateroottest.Installation(t, installation), "claude", "seeded", usage.ReadOptions{Transcript: seeded, Capability: usage.PerCall}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(usage.SamplesPath(installation, "claude", "seeded")); err != nil {
		t.Fatalf("the seeded samples are not in the installation's call store: %v", err)
	}
	if out := gate("seeded", seeded); out != "" {
		t.Fatalf("a call whose sample the installation's store already holds was answered %q; want it allowed silently", out)
	}

	rows := readToolGateRows(t, installation)
	if len(rows) != 2 || rows[0].Session != "over" || rows[0].Decision != "deny" || rows[0].Tokens != 85000 ||
		rows[1].Session != "seeded" || rows[1].Decision != "allow" || rows[1].Cause != "no-sample" {
		t.Fatalf("installation rows = %+v; want the denial at 85K, then the seeded session's no-sample allow", rows)
	}
	entries, err := os.ReadDir(state)
	if err != nil || len(entries) != 0 {
		t.Fatalf("the state root holds %v (err %v); the gate must create nothing there", entries, err)
	}
}
