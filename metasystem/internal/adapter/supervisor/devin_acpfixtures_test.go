package supervisor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/acp"
)

// The ported scripts/agents/acp-fixtures.sh ACP-V scenarios: the former
// `acp preflight` / `acp turn` verb boundary is now acp.PreflightACP and
// acp.RunFileTurn called in process, driven over real fifos made and served
// by the supervisor's own pipe plumbing (ACPPipes), with a stub server that
// speaks canned frames. Client request ids are deterministic (1, 2, 3).

const fixtureEnvelope = `{"readRoots":["/repo"],"writeRoots":["/repo/work"],"network":"deny","approvals":"deny","tools":"read-only"}`

type acpFixture struct {
	t                     *testing.T
	dir, envelope, prompt string
	pipes                 *ACPPipes
	server                *exec.Cmd
}

func newACPFixture(t *testing.T, serverScript string) *acpFixture {
	t.Helper()
	f := &acpFixture{t: t, dir: resolvedTempDir(t)}
	f.envelope = filepath.Join(f.dir, "envelope.json")
	f.prompt = filepath.Join(f.dir, "prompt.txt")
	writeFile(t, f.envelope, fixtureEnvelope)
	writeFile(t, f.prompt, "reply with pong\n")
	if serverScript == "" {
		return f
	}
	server := filepath.Join(f.dir, "server.sh")
	writeFile(t, server, "#!/bin/bash\n"+serverScript)
	if err := os.Chmod(server, 0o755); err != nil {
		t.Fatal(err)
	}
	pipes, err := ACPPipeNames(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := pipes.Make(); err != nil {
		t.Fatal(err)
	}
	f.pipes = pipes
	command, err := pipes.ServerCommand(server, "devin-delegate-acp", f.dir, []string{"PATH=/usr/bin:/bin"}, os.Stderr)
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	pipes.Started()
	f.server = command
	t.Cleanup(f.stop)
	return f
}

func (f *acpFixture) stop() {
	if f.server != nil {
		_ = f.server.Process.Kill()
		_ = f.server.Wait()
		f.server = nil
	}
	f.pipes.Remove()
}

func (f *acpFixture) config(journal string) acp.FileTurnConfig {
	return acp.FileTurnConfig{
		ServerOut: f.pipes.ServerOut, ServerIn: f.pipes.ServerIn,
		JournalPath: journal, Workspace: f.dir,
		EnvelopePath: f.envelope, PromptFile: f.prompt,
		LateFrameWindow: 100 * time.Millisecond,
	}
}

// ACP-V-001: preflight admits a v1-eligible envelope and refuses
// network=ask.
func TestACPFixturePreflight(t *testing.T) {
	t.Parallel()
	f := newACPFixture(t, "")
	envelope, err := acp.LoadEnvelopeFile(f.envelope)
	if err != nil || acp.PreflightACP(envelope) != "" {
		t.Fatalf("a v1-eligible envelope must be admitted: %v %q", err, acp.PreflightACP(envelope))
	}
	ask := filepath.Join(f.dir, "envelope-ask.json")
	writeFile(t, ask, `{"readRoots":[],"writeRoots":[],"network":"ask","approvals":"deny","tools":"read-only"}`)
	envelope, err = acp.LoadEnvelopeFile(ask)
	if err != nil || acp.PreflightACP(envelope) == "" {
		t.Fatal("network=ask must be refused")
	}
	// The in-process turn refuses the same envelope before any wire open.
	_, err = acp.RunFileTurn(context.Background(), acp.FileTurnConfig{
		ServerOut: "/nonexistent/out", ServerIn: "/nonexistent/in",
		JournalPath: filepath.Join(f.dir, "journal.log"), Workspace: f.dir,
		EnvelopePath: ask, PromptFile: f.prompt,
	})
	if err == nil || !strings.Contains(err.Error(), "preflight refused") {
		t.Fatalf("err %v", err)
	}
}

// ACP-V-002: a full delivered turn over real fifos — candidate assembled,
// usage verbatim, both directions journaled.
func TestACPFixtureDeliveredOverFifos(t *testing.T) {
	t.Parallel()
	f := newACPFixture(t, `read -r _
echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":1,"agentCapabilities":{"loadSession":true},"authMethods":[]}}'
read -r _
echo '{"jsonrpc":"2.0","id":2,"result":{"sessionId":"fx-1"}}'
read -r _
echo '{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"fx-1","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"pong"}}}}'
echo '{"jsonrpc":"2.0","id":3,"result":{"stopReason":"end_turn","usage":{"inputTokens":5,"outputTokens":1,"totalTokens":6}}}'
exec cat >/dev/null
`)
	journal := filepath.Join(f.dir, "journal.log")
	outcome, err := acp.RunFileTurn(context.Background(), f.config(journal))
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Row != "delivered" || outcome.Candidate == nil || *outcome.Candidate != "pong" || outcome.SessionID != "fx-1" {
		t.Fatalf("outcome %+v", outcome)
	}
	if !strings.Contains(string(outcome.Usage), `"totalTokens":6`) {
		t.Fatalf("usage not verbatim: %s", outcome.Usage)
	}
	outcomeFile := filepath.Join(f.dir, "outcome.json")
	if err := acp.WriteFileTurnOutcome(outcomeFile, outcome); err != nil {
		t.Fatal(err)
	}
	written, _ := os.ReadFile(outcomeFile)
	if !strings.Contains(string(written), `"row":"delivered"`) || !strings.Contains(string(written), `"candidate":"pong"`) {
		t.Fatalf("outcome document %s", written)
	}
	data, _ := os.ReadFile(journal)
	if !strings.Contains(string(data), "\n> ") && !strings.HasPrefix(string(data), "> ") {
		t.Fatalf("the outbound direction must be journaled: %s", data)
	}
	if !strings.Contains(string(data), "\n< ") && !strings.HasPrefix(string(data), "< ") {
		t.Fatalf("the inbound direction must be journaled: %s", data)
	}
}

// ACP-V-003: a pre-existing journal is a refused collision, never appended
// evidence.
func TestACPFixtureJournalCollisionRefused(t *testing.T) {
	t.Parallel()
	f := newACPFixture(t, "exec cat >/dev/null\n")
	journal := filepath.Join(f.dir, "journal.log")
	writeFile(t, journal, "")
	if _, err := acp.RunFileTurn(context.Background(), f.config(journal)); err == nil || !strings.Contains(err.Error(), "journal create") {
		t.Fatalf("an existing journal must refuse: %v", err)
	}
	if data, _ := os.ReadFile(journal); len(data) != 0 {
		t.Fatalf("the colliding journal was written: %q", data)
	}
}

// ACP-V-004: the signal bridge, now cancellation — cancelling mid-prompt
// yields the TYPED cancelled outcome with the courtesy session/cancel on
// the wire, never a bare death. (The supervisor's TERM/INT handler cancels
// exactly this context: TestDevinACPSignalCancelsTheClient.)
func TestACPFixtureCancelMidPrompt(t *testing.T) {
	t.Parallel()
	observed := filepath.Join(resolvedTempDir(t), "cancel-observed")
	f := newACPFixture(t, `read -r _
echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":1,"agentCapabilities":{"loadSession":true},"authMethods":[]}}'
read -r _
echo '{"jsonrpc":"2.0","id":2,"result":{"sessionId":"fx-c"}}'
read -r _
read -r cancel_line || cancel_line=""
case "$cancel_line" in *session/cancel*) echo cancel-seen > `+observed+` ;; esac
exec cat >/dev/null
`)
	journal := filepath.Join(f.dir, "journal.log")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for {
			if data, _ := os.ReadFile(journal); strings.Contains(string(data), "session/prompt") {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		cancel()
	}()
	outcome, err := acp.RunFileTurn(ctx, f.config(journal))
	if err != nil {
		t.Fatalf("a cancelled attempt still produces a typed outcome: %v", err)
	}
	if outcome.Row != "cancelled" {
		t.Fatalf("outcome %+v", outcome)
	}
	// The server reads the cancel line in its own time; the wire, not a
	// sleep, decides: the journal must already hold the outbound cancel.
	if data, _ := os.ReadFile(journal); !strings.Contains(string(data), "session/cancel") {
		t.Fatalf("the courtesy session/cancel was not sent: %s", data)
	}
	for {
		if _, err := os.Stat(observed); err == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

}
