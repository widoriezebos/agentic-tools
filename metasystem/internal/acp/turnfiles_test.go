package acp

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

const fileTurnEnvelope = `{"readRoots":["/repo"],"writeRoots":["/repo/work"],"network":"deny","approvals":"deny","tools":"read-only"}`

// fileTurnBed is one attempt's directory: an admitted envelope, a prompt,
// and the paths the attempt names. Pipes are only made on request, so the
// refusal cases prove they never reach a wire open.
type fileTurnBed struct {
	dir string
	cfg FileTurnConfig
}

func newFileTurnBed(t *testing.T) *fileTurnBed {
	t.Helper()
	dir := t.TempDir()
	bed := &fileTurnBed{dir: dir}
	bed.cfg = FileTurnConfig{
		ServerOut:    filepath.Join(dir, "server-out"),
		ServerIn:     filepath.Join(dir, "server-in"),
		JournalPath:  filepath.Join(dir, "journal.log"),
		Workspace:    "/work",
		EnvelopePath: filepath.Join(dir, "envelope.json"),
		PromptFile:   filepath.Join(dir, "prompt.txt"),
	}
	bed.write(t, "envelope.json", fileTurnEnvelope)
	bed.write(t, "prompt.txt", "do the thing")
	return bed
}

func (b *fileTurnBed) write(t *testing.T, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(b.dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// serve makes the fifo pair and runs the scripted stub server on it in the
// launch order RunFileTurn expects (its read side first). The server stays
// on the wire until the client closes its write side, so the outcome never
// depends on when the server hangs up.
func (b *fileTurnBed) serve(t *testing.T, steps []stubStep) *stubServer {
	t.Helper()
	for _, path := range []string{b.cfg.ServerOut, b.cfg.ServerIn} {
		if err := syscall.Mkfifo(path, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	server := &stubServer{t: t, consumed: make(chan int, 1)}
	go func() {
		out, err := os.OpenFile(b.cfg.ServerOut, os.O_WRONLY, 0)
		if err != nil {
			t.Errorf("server-out open: %v", err)
			server.consumed <- 0
			return
		}
		defer out.Close()
		in, err := os.OpenFile(b.cfg.ServerIn, os.O_RDONLY, 0)
		if err != nil {
			t.Errorf("server-in open: %v", err)
			server.consumed <- 0
			return
		}
		defer in.Close()
		server.reader = NewReader(in, nil)
		server.writer = NewWriter(out, nil)
		server.closer = out
		// run reports its count on consumed; then hold the wire until
		// the client hangs up its write side.
		server.run(steps)
		_, _ = io.Copy(io.Discard, in)
	}()
	return server
}

func deliveredFileSteps(extra ...stubStep) []stubStep {
	steps := []stubStep{initStep("[]"), newSessionStep()}
	steps = append(steps, extra...)
	return append(steps, stubStep{
		expectMethod:  "session/prompt",
		expectParams:  []string{`"sessionId":"s-1"`, "do the thing"},
		notifications: []string{chunkFor("s-1", "pong")},
		result:        `{"stopReason":"end_turn","usage":{"inputTokens":5,"outputTokens":1,"totalTokens":6}}`,
	})
}

func TestRunFileTurnDeliversOverFifosAndJournalsBothDirections(t *testing.T) {
	t.Parallel()
	bed := newFileTurnBed(t)
	bed.cfg.Mode = "plan"
	bed.cfg.SessionFile = filepath.Join(bed.dir, "session-id")
	bed.cfg.LateFrameWindow = 1 // the smallest positive window: the server stays silent after the response
	server := bed.serve(t, deliveredFileSteps(stubStep{
		expectMethod: "session/set_mode",
		expectParams: []string{`"modeId":"plan"`, `"sessionId":"s-1"`},
		result:       `{}`,
	}))

	outcome, err := RunFileTurn(context.Background(), bed.cfg)
	if err != nil {
		t.Fatal(err)
	}
	server.requireConsumed(4)
	if outcome.Row != string(RowDelivered) || outcome.Candidate == nil || *outcome.Candidate != "pong" {
		t.Fatalf("outcome %+v", outcome)
	}
	if outcome.SessionID != "s-1" || outcome.StopReason != "end_turn" || outcome.JournalError != "" || outcome.Violations != 0 {
		t.Fatalf("outcome %+v", outcome)
	}
	if !strings.Contains(string(outcome.Usage), `"totalTokens":6`) {
		t.Fatalf("usage must be carried verbatim: %s", outcome.Usage)
	}
	if session, err := os.ReadFile(bed.cfg.SessionFile); err != nil || string(session) != "s-1\n" {
		t.Fatalf("session file %q %v", session, err)
	}
	journal, err := os.ReadFile(bed.cfg.JournalPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"session/prompt", "session/set_mode", `"protocolVersion":1`, "pong"} {
		if !strings.Contains(string(journal), want) {
			t.Fatalf("journal missing %q:\n%s", want, journal)
		}
	}

	// The outcome document is one JSON line carrying the typed fields.
	outcomePath := filepath.Join(bed.dir, "outcome.json")
	if err := WriteFileTurnOutcome(outcomePath, outcome); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(outcomePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(written), "}\n") || strings.Count(string(written), "\n") != 1 {
		t.Fatalf("outcome document must be one JSON line: %q", written)
	}
	var decoded FileTurnOutcome
	if err := json.Unmarshal(written, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Row != "delivered" || decoded.Candidate == nil || *decoded.Candidate != "pong" || decoded.SessionID != "s-1" {
		t.Fatalf("decoded %+v", decoded)
	}
}

func TestRunFileTurnSetupRefusalIsATypedOutcomeWithoutCandidate(t *testing.T) {
	t.Parallel()
	bed := newFileTurnBed(t)
	// ExpectedProtocol zero defaults to protocol 1 on the wire.
	server := bed.serve(t, []stubStep{{
		expectMethod: "initialize",
		expectParams: []string{`"protocolVersion":1`},
		errorCode:    -32603,
		errorMessage: "boom",
	}})

	outcome, err := RunFileTurn(context.Background(), bed.cfg)
	if err != nil {
		t.Fatal(err)
	}
	server.requireConsumed(1)
	if outcome.Row != string(RowSetupError) || outcome.Candidate != nil || !strings.Contains(outcome.Detail, "initialize refused: -32603 boom") {
		t.Fatalf("outcome %+v", outcome)
	}

	outcomePath := filepath.Join(bed.dir, "outcome.json")
	if err := WriteFileTurnOutcome(outcomePath, outcome); err != nil {
		t.Fatal(err)
	}
	written, _ := os.ReadFile(outcomePath)
	if strings.Contains(string(written), "candidate") || strings.Contains(string(written), "journalError") {
		t.Fatalf("a non-delivered outcome must omit candidate and a healthy journal must omit journalError: %s", written)
	}
	if !strings.Contains(string(written), `"violations":0`) {
		t.Fatalf("violations is always present: %s", written)
	}
}

func TestRunFileTurnVersionMismatchHonoursExpectedProtocol(t *testing.T) {
	t.Parallel()
	bed := newFileTurnBed(t)
	bed.cfg.ExpectedProtocol = 2
	server := bed.serve(t, []stubStep{{
		expectMethod: "initialize",
		expectParams: []string{`"protocolVersion":2`},
		result:       `{"protocolVersion":1,"agentCapabilities":{},"authMethods":[]}`,
	}})
	outcome, err := RunFileTurn(context.Background(), bed.cfg)
	if err != nil {
		t.Fatal(err)
	}
	server.requireConsumed(1)
	if outcome.Row != string(RowVersionMismatch) || outcome.Detail != "negotiated 1, expected 2" {
		t.Fatalf("outcome %+v", outcome)
	}
}

func TestRunFileTurnRefusesBeforeAnyWireOpen(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		arrange func(t *testing.T, b *fileTurnBed)
		want    string
	}{
		{"missing envelope", func(t *testing.T, b *fileTurnBed) {
			b.cfg.EnvelopePath = filepath.Join(b.dir, "absent.json")
		}, "absent.json"},
		{"malformed envelope", func(t *testing.T, b *fileTurnBed) {
			b.write(t, "envelope.json", "{not json")
		}, "envelope " + "%DIR%/envelope.json"},
		{"preflight refusal", func(t *testing.T, b *fileTurnBed) {
			b.write(t, "envelope.json", `{"readRoots":[],"writeRoots":[],"network":"deny","approvals":"allow","tools":"read-only"}`)
		}, "preflight refused: approvals=allow is unsupported"},
		{"missing prompt", func(t *testing.T, b *fileTurnBed) {
			b.cfg.PromptFile = filepath.Join(b.dir, "no-prompt.txt")
		}, "no-prompt.txt"},
		{"journal collision", func(t *testing.T, b *fileTurnBed) {
			b.write(t, "journal.log", "stale session\n")
		}, "record file could not be created"},
		{"server-out unopenable", func(t *testing.T, b *fileTurnBed) {}, "server-out open"},
		{"server-in unopenable", func(t *testing.T, b *fileTurnBed) {
			b.write(t, "server-out", "")
		}, "server-in open"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bed := newFileTurnBed(t)
			tc.arrange(t, bed)
			want := strings.ReplaceAll(tc.want, "%DIR%", bed.dir)
			outcome, err := RunFileTurn(context.Background(), bed.cfg)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("err = %v, want %q", err, want)
			}
			if outcome.Row != "" {
				t.Fatalf("a refused attempt has no outcome row: %+v", outcome)
			}
			if tc.name == "journal collision" {
				if data, _ := os.ReadFile(bed.cfg.JournalPath); string(data) != "stale session\n" {
					t.Fatalf("the colliding journal was touched: %q", data)
				}
			}
		})
	}
}

func TestLoadEnvelopeFileReadsTheFiveFields(t *testing.T) {
	t.Parallel()
	bed := newFileTurnBed(t)
	envelope, err := LoadEnvelopeFile(bed.cfg.EnvelopePath)
	if err != nil {
		t.Fatal(err)
	}
	want := workspaceEnvelope("read-only")
	if strings.Join(envelope.ReadRoots, ",") != "/repo" || strings.Join(envelope.WriteRoots, ",") != "/repo/work" ||
		envelope.Network != want.Network || envelope.Approvals != want.Approvals || envelope.Tools != want.Tools {
		t.Fatalf("envelope %+v", envelope)
	}
}

func TestWriteFileTurnOutcomeReportsAnUnwritablePath(t *testing.T) {
	t.Parallel()
	err := WriteFileTurnOutcome(filepath.Join(t.TempDir(), "missing-dir", "outcome.json"), FileTurnOutcome{Row: "delivered"})
	if err == nil {
		t.Fatal("writing into a missing directory must fail")
	}
}

func TestRowsIsTheWholeVocabularyOnce(t *testing.T) {
	t.Parallel()
	seen := map[Row]bool{}
	for _, row := range Rows() {
		if seen[row] {
			t.Fatalf("row %q listed twice", row)
		}
		seen[row] = true
	}
	for _, row := range []Row{RowDelivered, RowVersionMismatch, RowAuthRequired, RowSetupError,
		RowProtocolError, RowTurnFailed, RowCancelled, RowRefused, RowIncomplete} {
		if !seen[row] {
			t.Fatalf("row %q missing from Rows()", row)
		}
	}
	if len(seen) != 9 {
		t.Fatalf("Rows() has %d rows", len(seen))
	}
}
