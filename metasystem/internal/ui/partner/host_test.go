package partner_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
)

// The host is the many-turn owner, which is the whole difference between a
// job's driver and a conversation's: a second prompt runs on the same session,
// on the same process, without a second initialize.
func TestTheHostRunsManyTurnsOnOneSession(t *testing.T) {
	t.Parallel()
	host := hostOn(t, fakeacp.Script{Chunks: []string{"one ", "two"}})
	fresh, err := host.Ready(context.Background())
	testutil.Require(t, "ready", err, nil)
	testutil.Expect(t, "the first session is fresh", fresh, true)

	first, answer := runTurn(t, host, "first")
	testutil.Expect(t, "the first turn completes", first.Outcome, partner.OutcomeComplete)
	testutil.Expect(t, "and streams its answer", answer, "one two")

	again, err := host.Ready(context.Background())
	testutil.Require(t, "ready again", err, nil)
	testutil.Expect(t, "the session is not reopened", again, false)
	second, _ := runTurn(t, host, "second")
	testutil.Expect(t, "the second turn completes too", second.Outcome, partner.OutcomeComplete)
}

// A permission request is refused by the permission point and becomes an
// activity line, while a tool call becomes the muted line the drawer shows.
func TestTheHostRefusesAPermissionRequestAndSaysSo(t *testing.T) {
	t.Parallel()
	host := hostOn(t, fakeacp.Script{
		Activity: "Read plans/goals/backlog.md", Permission: "Write a file", PermissionKind: "edit",
		Chunks: []string{"I cannot write."},
	})
	_, err := host.Ready(context.Background())
	testutil.Require(t, "ready", err, nil)
	// What the Partner is doing at a moment and what a human has to be told
	// are two beats, deliberately: the first is replaced by the next and kept
	// nowhere, and the second is read again afterwards.
	var activity, doing []string
	result, err := host.Prompt(context.Background(), "write something", func(update partner.Update) {
		switch update.Kind {
		case partner.UpdateActivity:
			activity = append(activity, update.Text)
		case partner.UpdateDoing:
			doing = append(doing, update.Text)
		}
	})
	testutil.Require(t, "prompted", err, nil)
	testutil.Expect(t, "the turn completes", result.Outcome, partner.OutcomeComplete)
	testutil.Require(t, "one thing it was doing", len(doing), 1)
	testutil.Expect(t, "the tool call", doing[0], "Read plans/goals/backlog.md")
	testutil.Require(t, "one activity line", len(activity), 1)
	testutil.Expect(t, "the refusal", strings.HasPrefix(activity[0], "Refused: the Partner reads this checkout"), true)
	testutil.Expect(t, "and what was asked for", strings.Contains(activity[0], "Write a file (edit)"), true)
}

// Stop cancels through the protocol and waits for the cancelled prompt's own
// response, which is what leaves the connection alive for the next question.
func TestStopSettlesTheTurnAndKeepsTheSession(t *testing.T) {
	t.Parallel()
	host := hostOn(t, fakeacp.Script{
		Chunks: []string{"a", "b", "c", "d", "e", "f"}, Pause: 200 * time.Millisecond,
	})
	_, err := host.Ready(context.Background())
	testutil.Require(t, "ready", err, nil)

	started := make(chan struct{})
	done := make(chan partner.Result, 1)
	go func() {
		var seen int
		result, _ := host.Prompt(context.Background(), "a long answer", func(update partner.Update) {
			if update.Kind == partner.UpdateText {
				seen++
				if seen == 1 {
					close(started)
				}
			}
		})
		done <- result
	}()
	<-started
	testutil.Require(t, "stopped", host.Stop(context.Background()), nil)
	result := <-done
	testutil.Expect(t, "the turn is stopped", result.Outcome, partner.OutcomeStopped)
	testutil.Expect(t, "the session survives the cancellation", host.Alive(), true)

	// And the next question runs on it, which is the whole point of settling.
	next, _ := runTurn(t, host, "and now?")
	testutil.Expect(t, "the next turn runs on the same session", next.Outcome, partner.OutcomeComplete)
}

// A runtime that refuses to initialise refuses the turn, in its own words, and
// with the line that installs it.
func TestARuntimeThatRefusesToStartCarriesItsOwnWordsAndTheInstallLine(t *testing.T) {
	t.Parallel()
	host := partner.NewHostOn(
		partner.Runtime{Name: "claude", Install: "npm install -g @agentclientprotocol/claude-agent-acp"},
		t.TempDir(), fakeacp.Open(fakeacp.Script{InitError: "Authentication required: run claude login"}))
	t.Cleanup(host.Close)
	_, err := host.Ready(context.Background())
	testutil.Require(t, "refused", err != nil, true)
	var start *partner.StartError
	testutil.Require(t, "it is a start refusal", errors.As(err, &start), true)
	testutil.Expect(t, "the server's words", start.Reason, "Authentication required: run claude login")
	testutil.Expect(t, "and the install line", start.Install, "npm install -g @agentclientprotocol/claude-agent-acp")
}

// The configured model is selected at session setup, and a server that cannot
// select it refuses the turn with its own sentence.
func TestAModelTheRuntimeCannotSelectRefusesTheTurn(t *testing.T) {
	t.Parallel()
	host := partner.NewHostOn(
		partner.Runtime{Name: "claude", Model: "claude-opus-5-5"},
		t.TempDir(), fakeacp.Open(fakeacp.Script{
			Models: []string{"claude-opus-5-5"}, ModelError: "this account cannot use claude-opus-5-5",
		}))
	t.Cleanup(host.Close)
	_, err := host.Ready(context.Background())
	testutil.Require(t, "refused", err != nil, true)
	testutil.Expect(t, "in the server's words", err.Error(), "this account cannot use claude-opus-5-5")
}

// A runtime that offers no model selection at all cannot honour a configured
// model, and says which setting to clear.
func TestARuntimeWithNoModelSelectionRefusesAConfiguredModel(t *testing.T) {
	t.Parallel()
	host := partner.NewHostOn(
		partner.Runtime{Name: "devin", Model: "opus"},
		t.TempDir(), fakeacp.Open(fakeacp.Script{}))
	t.Cleanup(host.Close)
	_, err := host.Ready(context.Background())
	testutil.Require(t, "refused", err != nil, true)
	testutil.Expect(t, "it names the setting", strings.Contains(err.Error(), "clear ui.partner.model"), true)
}

// A runtime that names no model runs the runtime's own default, and nothing is
// selected.
func TestNoConfiguredModelSelectsNothing(t *testing.T) {
	t.Parallel()
	host := partner.NewHostOn(partner.Runtime{Name: "devin"}, t.TempDir(),
		fakeacp.Open(fakeacp.Script{Chunks: []string{"hello"}}))
	t.Cleanup(host.Close)
	_, err := host.Ready(context.Background())
	testutil.Require(t, "ready", err, nil)
	result, answer := runTurn(t, host, "hello?")
	testutil.Expect(t, "it answers", result.Outcome, partner.OutcomeComplete)
	testutil.Expect(t, "with its answer", answer, "hello")
}

// A server speaking another protocol version is refused rather than talked to.
func TestAnotherProtocolVersionIsRefused(t *testing.T) {
	t.Parallel()
	host := partner.NewHostOn(partner.Runtime{Name: "claude"}, t.TempDir(),
		fakeacp.Open(fakeacp.Script{Protocol: 2}))
	t.Cleanup(host.Close)
	_, err := host.Ready(context.Background())
	testutil.Require(t, "refused", err != nil, true)
	testutil.Expect(t, "it says which", strings.Contains(err.Error(), "speaks ACP 2 and this client speaks 1"), true)
}

// A refusal on the wire is a refused turn, not a completed one.
func TestARefusedTurnIsRefusedAndNotComplete(t *testing.T) {
	t.Parallel()
	host := hostOn(t, fakeacp.Script{StopReason: "refusal"})
	_, err := host.Ready(context.Background())
	testutil.Require(t, "ready", err, nil)
	result, _ := runTurn(t, host, "something")
	testutil.Expect(t, "refused", result.Outcome, partner.OutcomeRefused)
}

func hostOn(t *testing.T, script fakeacp.Script) *partner.Host {
	t.Helper()
	host := partner.NewHostOn(partner.Runtime{Name: "claude"}, t.TempDir(), fakeacp.Open(script))
	t.Cleanup(host.Close)
	return host
}

func runTurn(t *testing.T, host *partner.Host, text string) (partner.Result, string) {
	t.Helper()
	var answer strings.Builder
	result, err := host.Prompt(context.Background(), text, func(update partner.Update) {
		if update.Kind == partner.UpdateText {
			answer.WriteString(update.Text)
		}
	})
	if err != nil {
		t.Fatalf("the turn must run: %v", err)
	}
	return result, answer.String()
}
