package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// peerBoard is a fixture host board: the environment a hook resolves it
// from, and its home.
func peerBoard(t *testing.T) (map[string]string, string) {
	t.Helper()
	registry := t.TempDir()
	env := map[string]string{"METASYSTEM_SUPERVISION_REGISTRY_HOME": registry}
	home, err := board.HomeWith(func(name string) (string, bool) { value, ok := env[name]; return value, ok })
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	return env, home
}

func peerAsk(t *testing.T, home string, to board.Address, text string) board.Message {
	t.Helper()
	published, err := board.Publish(home, board.Request{Kind: board.KindAsk, From: board.Sender{Machine: "m1a", Lineage: "L"}, To: to, Text: text}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	return published.Message
}

func markerExists(home string, message board.Message, machine string) bool {
	mailbox := filepath.Join(board.Dir(home), message.To.Machine, "mailbox")
	if message.To.Goal != "" {
		mailbox = filepath.Join(board.Dir(home), board.GoalNamespace, message.To.Goal, "mailbox")
	}
	_, err := os.Stat(filepath.Join(mailbox, "delivered", message.ID, machine+".json"))
	return err == nil
}

// packetOps is the full start path with Claude's declared field (10,000
// bytes) and a brain packet of the given text, on seat m1b.
func packetOps(t *testing.T, installation hookInstallation, packet string) *fakeOps {
	t.Helper()
	ops := fullPathOps(t, installation, "context")
	ops.startContext = func(runtime string) (string, int) {
		if runtime == "claude" {
			return "field=hookSpecificOutput.additionalContext event=SessionStart bytes=10000 sources=startup,resume,clear,compact\n", 0
		}
		return "", 1
	}
	encoded, _ := json.Marshal(packet)
	ops.brainBoot = func(context.Context) (string, string, int) {
		return `{"declared":true,"state":"declared","payload":` + string(encoded) + `,"bytes":` + itoa(len(packet)) + `,"sections":{"asks":"complete","held":"complete","fleet":"complete","digest":"complete"},"digestEmitted":false,"digestCursor":0,"digestPrefixSha256":"","declarationSha256":"` + strings.Repeat("a", 64) + `"}`, "", 0
	}
	ops.peerSeat = func(string) (string, int) { return "m1b\n", 0 }
	return ops
}

func itoa(value int) string { encoded, _ := json.Marshal(value); return string(encoded) }

func startContextField(t *testing.T, stdout string) string {
	t.Helper()
	var response struct {
		HookSpecificOutput struct {
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSuffix(stdout, "\n")), &response); err != nil {
		t.Fatalf("the start published no JSON object: %v: %q", err, stdout)
	}
	return response.HookSpecificOutput.AdditionalContext
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("injected: stdout closed") }

// TestDeliveryFieldIsTheRuntimesDeclaration (R26, U10f-1; D14C-07): every
// runtime that declares a tool context field declares at least the
// envelope bound; the offer measures bytes, never characters; Claude's start
// places the oldest pending message after the role packet in the declared
// field with the byte-exact preface and marks it only after the emission; a
// 9,000-byte packet and a 2,000-byte message leave the packet intact, the
// field within 10,000 bytes and the message pending with no marker, and the
// next tool offer delivers it; a screen outcome, Codex and Devin offer
// nothing and mark nothing; a start whose emission fails writes no marker
// and the next start offers the same id.
func TestDeliveryFieldIsTheRuntimesDeclaration(t *testing.T) {
	t.Parallel()
	for _, declaration := range runtimes.All() {
		if declaration.ToolContextField != "" && board.MaxEnvelopeBytes > declaration.ToolContextBytes {
			t.Errorf("%s declares %d tool context bytes, under the envelope bound %d", declaration.Name, declaration.ToolContextBytes, board.MaxEnvelopeBytes)
		}
	}

	_, home := peerBoard(t)
	wide := peerAsk(t, home, board.Address{Machine: "m1b"}, strings.Repeat("\U0001F600", board.MaxTextBytes/4))
	rendered := board.Render(wide, time.Now(), time.Local)
	if offer, ok := OfferPeerMessage(home, "m1b", "L", nil, "tool", len(rendered)-1, time.Now()); ok {
		offer.Release()
		t.Fatal("an offer counted characters: the rendered bytes exceed the room and it was offered")
	}
	offer, ok := OfferPeerMessage(home, "m1b", "L", nil, "tool", 10000, time.Now())
	if !ok || offer.Text != rendered || len(offer.Text) > 10000 {
		t.Fatalf("8 KiB of four-byte runes at a tool call: ok=%v, %d bytes", ok, len(offer.Text))
	}
	offer.Release()

	t.Run("start places the message after the packet", func(t *testing.T) {
		t.Parallel()
		env, home := peerBoard(t)
		message := peerAsk(t, home, board.Address{Machine: "m1b"}, "is the lane green?")
		later, err := board.Publish(home, board.Request{Kind: board.KindAsk, From: board.Sender{Machine: "m1c"}, To: board.Address{Machine: "m1b"}, Text: "a later question"}, message.At.Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		installation := newHookInstallation(t)
		run := runHook(t, installation, packetOps(t, installation, "role packet"), hookCall{runtime: "claude", event: "start", env: env, payload: `{"session_id":"s-1","source":"startup"}` + "\n"})
		want := "role packet\n\n" + board.Render(message, time.Now(), time.Local)
		if field := startContextField(t, run.stdout); run.status != 0 || field != want {
			t.Fatalf("start field = %q (status %d, stderr %q), want %q", field, run.status, run.stderr, want)
		}
		if !markerExists(home, message, "m1b") || markerExists(home, later.Message, "m1b") {
			t.Fatal("the emitted oldest message has no marker, or the later one, not emitted, has one")
		}
	})

	t.Run("a message that does not fit waits for the tool call", func(t *testing.T) {
		t.Parallel()
		env, home := peerBoard(t)
		message := peerAsk(t, home, board.Address{Machine: "m1b"}, strings.Repeat("m", 2000))
		packet := strings.Repeat("p", 9000)
		installation := newHookInstallation(t)
		run := runHook(t, installation, packetOps(t, installation, packet), hookCall{runtime: "claude", event: "start", env: env, payload: `{"session_id":"s-1","source":"startup"}` + "\n"})
		if field := startContextField(t, run.stdout); field != packet || len(field) > 10000 {
			t.Fatalf("the packet was not left intact: %d bytes", len(field))
		}
		if markerExists(home, message, "m1b") {
			t.Fatal("a message that did not fit was marked")
		}
		offer, ok := OfferPeerMessage(home, "m1b", "L", nil, "tool", 10000, time.Now())
		if !ok || !strings.Contains(offer.Text, strings.Repeat("m", 2000)) {
			t.Fatalf("the next tool offer = %v %q", ok, offer.Text)
		}
		if err := offer.Mark(); err != nil || !markerExists(home, message, "m1b") {
			t.Fatalf("the tool offer's marker: %v", err)
		}
		offer.Release()
	})

	t.Run("a screen outcome, Codex and Devin offer nothing", func(t *testing.T) {
		t.Parallel()
		env, home := peerBoard(t)
		message := peerAsk(t, home, board.Address{Machine: "m1b"}, "SECRET-PEER-TEXT")
		for _, runtimeName := range []string{"claude", "codex", "devin"} {
			installation := newHookInstallation(t)
			ops := packetOps(t, installation, "role packet")
			if runtimeName == "claude" {
				ops.startContext = func(string) (string, int) { return "", 1 }
			}
			ops.identityRuntime = runtimeName
			ops.findAncestor = nil
			run := runHook(t, installation, ops, hookCall{runtime: runtimeName, event: "start", env: env, payload: `{"session_id":"s-1","source":"startup"}` + "\n"})
			if strings.Contains(run.stdout+run.stderr, "SECRET-PEER-TEXT") || markerExists(home, message, "m1b") {
				t.Fatalf("%s start offered the message: stdout %q stderr %q", runtimeName, run.stdout, run.stderr)
			}
		}
	})

	t.Run("an emission that fails writes no marker", func(t *testing.T) {
		t.Parallel()
		env, home := peerBoard(t)
		message := peerAsk(t, home, board.Address{Machine: "m1b"}, "try again")
		installation := newHookInstallation(t)
		runHook(t, installation, packetOps(t, installation, "role packet"), hookCall{runtime: "claude", event: "start", env: env, stdout: failingWriter{}, payload: `{"session_id":"s-1","source":"startup"}` + "\n"})
		if markerExists(home, message, "m1b") {
			t.Fatal("a failed emission was marked")
		}
		run := runHook(t, installation, packetOps(t, installation, "role packet"), hookCall{runtime: "claude", event: "start", env: env, payload: `{"session_id":"s-2","source":"startup"}` + "\n"})
		if !strings.Contains(startContextField(t, run.stdout), "id "+message.ID+":") || !markerExists(home, message, "m1b") {
			t.Fatalf("the next start did not offer the same id: %q", run.stdout)
		}
	})

	t.Run("a goal message follows the claims and waits while they are unreadable", func(t *testing.T) {
		t.Parallel()
		env, home := peerBoard(t)
		message := peerAsk(t, home, board.Address{Goal: "goal-x"}, "who reviews?")
		installation := newHookInstallation(t)
		ops := packetOps(t, installation, "role packet")
		ops.peerClaims = func(string) (string, int) { return "the accepted ledger tip is unreadable\n", 1 }
		run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "start", env: env, payload: `{"session_id":"s-1","source":"startup"}` + "\n"})
		if strings.Contains(run.stdout, "who reviews?") || markerExists(home, message, "m1b") || !strings.Contains(run.stderr, "1 goal messages wait: the ledger is unreadable") {
			t.Fatalf("with the ledger unreadable: stdout %q stderr %q", run.stdout, run.stderr)
		}
		ops = packetOps(t, installation, "role packet")
		ops.peerClaims = func(string) (string, int) { return `{"live":{"goal-x":"m1b"},"concluded":{}}` + "\n", 0 }
		run = runHook(t, installation, ops, hookCall{runtime: "claude", event: "start", env: env, payload: `{"session_id":"s-1","source":"startup"}` + "\n"})
		if !strings.Contains(startContextField(t, run.stdout), "about goal goal-x, id "+message.ID) || !markerExists(home, message, "m1b") {
			t.Fatalf("the holder's start = %q", run.stdout)
		}
	})
}

// TestStopOutputIgnoresPendingMessages (R26, U10f-1; D14B-01): the Stop
// output of every runtime is byte-identical with and without a pending
// message, and the Stop path marks nothing.
func TestStopOutputIgnoresPendingMessages(t *testing.T) {
	t.Parallel()
	for _, runtimeName := range []string{"claude", "codex", "devin", "fake"} {
		t.Run(runtimeName, func(t *testing.T) {
			t.Parallel()
			var outputs []string
			for _, pending := range []bool{false, true} {
				env, home := peerBoard(t)
				env[stopDeadlineParentEnv] = "777"
				var message board.Message
				if pending {
					message = peerAsk(t, home, board.Address{Machine: "m1b"}, "SECRET-PEER-TEXT")
				}
				installation := newHookInstallation(t)
				ops := newFakeOps(t, installation)
				ops.identityRuntime = runtimeName
				ops.peerSeat = func(string) (string, int) { return "m1b\n", 0 }
				run := runHook(t, installation, ops, hookCall{runtime: runtimeName, event: "stop", env: env, payload: `{"session_id":"line-fixture","hook_event_name":"Stop"}`})
				if pending && markerExists(home, message, "m1b") {
					t.Fatal("the Stop path marked a message")
				}
				outputs = append(outputs, run.stdout)
			}
			if outputs[0] != outputs[1] || strings.Contains(outputs[1], "SECRET") {
				t.Fatalf("the Stop output changed with a pending message:\nwithout %q\nwith    %q", outputs[0], outputs[1])
			}
		})
	}
}
