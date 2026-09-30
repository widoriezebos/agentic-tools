package main

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// renderOnce renders one result of a public command with the given words
// and text environment, and returns both streams.
func renderOnce(t *testing.T, name string, words []string, env *textui.Env, result intentResult) (int, string, string) {
	t.Helper()
	command, ok := findIntentCommand(name)
	if !ok {
		t.Fatalf("no command %q", name)
	}
	input, problem := parseIntentArgs(command, words)
	if problem != nil {
		t.Fatalf("%s %v: %+v", name, words, problem)
	}
	var stdout, stderr bytes.Buffer
	inv := &intentInvocation{command: command, input: input, stdout: &stdout, stderr: &stderr}
	if env != nil {
		inv.owners.textEnv = func(io.Writer) textui.Env { return *env }
	}
	code := inv.render(result)
	return code, stdout.String(), stderr.String()
}

// §4 step 4, the legacy shape: an unconverted verb's Summary is its
// headline, its lines print as they are, and its next step is the one hint.
func TestIntentRenderGivesEveryVerbTheHeadlineAndHintShape(t *testing.T) {
	t.Parallel()
	confirmed := intentResult{Outcome: intentConfirmed, Summary: "3 machines on this computer", text: []string{"m1e  reachable", "  indented as it was"},
		next: []string{"metasystem", "work", "status", "--all"}, nextReason: "also lists ended jobs"}
	code, stdout, stderr := renderOnce(t, "work status", nil, nil, confirmed)
	want := "3 machines on this computer\nm1e  reachable\n  indented as it was\n→ metasystem work status --all  also lists ended jobs\n"
	if code != 0 || stdout != want || stderr != "" {
		t.Errorf("confirmed = %d %q %q, want %q", code, stdout, stderr, want)
	}

	// D2: the refusal leads with its sentence, not the command the person
	// just typed; the remedy is the indented hint.
	refused := intentResult{Outcome: intentRefused, code: 2, Summary: "status takes one goal at most; nothing was done ",
		Decision: "one goal's work is metasystem status G"}
	code, stdout, stderr = renderOnce(t, "status", nil, nil, refused)
	want = "✗ status takes one goal at most; nothing was done\n  → one goal's work is metasystem status G\n"
	if code != 2 || stdout != "" || stderr != want {
		t.Errorf("refused = %d %q %q, want %q", code, stdout, stderr, want)
	}
	// --verbose keeps the prefix.
	_, _, stderr = renderOnce(t, "status", []string{"--verbose"}, nil, refused)
	if want := "✗ metasystem status: status takes one goal at most; nothing was done\n"; !strings.HasPrefix(stderr, want) {
		t.Errorf("verbose refusal = %q, want the prefix %q", stderr, want)
	}

	failed := intentResult{Outcome: intentFailed, Summary: "status is unknown: the fence is unreadable", text: []string{"a line the owner printed"},
		next: []string{"metasystem", "system", "start"}, nextReason: "repairs the fence"}
	code, _, stderr = renderOnce(t, "status", nil, nil, failed)
	want = "✗ status is unknown: the fence is unreadable\na line the owner printed\n  → metasystem system start  repairs the fence\n"
	if code != 1 || stderr != want {
		t.Errorf("failed = %d %q, want %q", code, stderr, want)
	}
	hinted := intentResult{Outcome: intentRefused, Summary: "not yet", nextReason: "the batch is still proving"}
	if _, _, stderr = renderOnce(t, "status", nil, nil, hinted); stderr != "✗ not yet\n  → the batch is still proving\n" {
		t.Errorf("a reason-only refusal = %q", stderr)
	}
	partial := intentResult{Outcome: intentPartial, Summary: "landed; the follow-up was not recorded"}
	if _, _, stderr = renderOnce(t, "status", nil, nil, partial); stderr != "! landed; the follow-up was not recorded\n" {
		t.Errorf("partial = %q", stderr)
	}
	underway := intentResult{Outcome: intentInProgress, Summary: "the build runs"}
	if _, _, stderr = renderOnce(t, "status", nil, nil, underway); stderr != "● the build runs\n" {
		t.Errorf("in progress = %q", stderr)
	}
}

// Off a terminal a legacy line keeps its width, so a pipe and every test of
// today's words reads it unchanged; on a terminal it wraps under a hanging
// indent, and colour marks the headline.
func TestIntentRenderWrapsLegacyLinesOnlyOnATerminal(t *testing.T) {
	t.Parallel()
	long := "landing lane /Users/wido/LocalStorage/GitHub/agentic-tools-landing: owner running (pid 38928); batch 4gr18nm8t3nyev9sssda9jgtsq collecting, 1 member"
	result := intentResult{Outcome: intentConfirmed, Summary: long}
	pipe := textui.Env{Width: 100}
	if _, stdout, _ := renderOnce(t, "landing status", nil, &pipe, result); stdout != long+"\n" {
		t.Errorf("a pipe = %q", stdout)
	}
	tty := textui.Env{Width: 100, TTY: true, Color: true}
	_, stdout, _ := renderOnce(t, "landing status", nil, &tty, result)
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "\x1b[1m") || !strings.HasPrefix(lines[1], "\x1b[1m  ") && !strings.HasPrefix(lines[1], "  ") {
		t.Errorf("a terminal = %q", stdout)
	}
}

// §4 step 5 and P11: the banner is text only; --json keeps the Summary and
// data it always had.
func TestIntentRenderPrintsTheBannerFirstAndLeavesJSONAlone(t *testing.T) {
	t.Parallel()
	summary := "HUMAN AT THE HELM since 12:39 CEST (2026-09-28) by wido: reason — metasystem helm return ends it"
	headline := "status of /work/m1e"
	result := intentResult{Outcome: intentConfirmed, Summary: summary, headline: &headline, Data: map[string]any{"helm": []string{summary}},
		attention: func(env textui.Env) []textui.Attention {
			return []textui.Attention{{State: textui.Alert, Text: "wido has the helm " + env.Since(time.Date(2026, 9, 28, 10, 39, 0, 0, time.UTC)) + ": reason",
				Hint: textui.Hint{Argv: []string{"metasystem", "helm", "return"}, Reason: "gives the seat back to the machinery"}}}
		}}
	env := textui.Env{Width: 100, Now: time.Date(2026, 9, 30, 8, 58, 0, 0, time.UTC), Zone: time.FixedZone("CEST", 2*3600)}
	_, stdout, _ := renderOnce(t, "status", nil, &env, result)
	want := "! wido has the helm since Mon 12:39: reason\n  → metasystem helm return  gives the seat back to the machinery\n\nstatus of /work/m1e\n"
	if stdout != want {
		t.Errorf("text = %q, want %q", stdout, want)
	}
	_, encoded, _ := renderOnce(t, "status", []string{"--json"}, &env, result)
	var decoded intentResult
	if err := json.Unmarshal([]byte(encoded), &decoded); err != nil || decoded.Summary != summary || strings.Contains(encoded, "has the helm") {
		t.Errorf("json = %s", encoded)
	}
}

// A converted verb's view draws the page; its next step is still the hint.
func TestIntentRenderDrawsAView(t *testing.T) {
	t.Parallel()
	result := intentResult{Outcome: intentConfirmed, Summary: "1 power(s) of attorney", next: []string{"metasystem", "grant", "list", "--all"},
		view: func(page *textui.Page) {
			page.Headline("1 grant, live")
			page.Section("", "").Item(textui.Live, "everything · by wido").KV("id", textui.Plain("G-1"))
		}}
	_, stdout, _ := renderOnce(t, "grant list", nil, nil, result)
	want := "1 grant, live\n\n● everything · by wido\n  id   G-1\n\n→ metasystem grant list --all\n"
	if stdout != want {
		t.Errorf("view = %q, want %q", stdout, want)
	}
}

// The production environment of a stream that is not a file is fixed: no
// colour, the full width and the symbols, whatever the test's shell says.
func TestIntentTextEnvOfABuffer(t *testing.T) {
	t.Parallel()
	inv := &intentInvocation{cwd: "/work/m1e/sub"}
	inv.layout.GitRoot = "/work/m1e"
	env := inv.textEnv(&bytes.Buffer{})
	if env.Width != textui.MaxWidth || env.Color || env.ASCII || env.TTY || !env.InRepo || env.Repo != "/work/m1e" || env.Zone == nil || env.Now.IsZero() {
		t.Errorf("env = %+v", env)
	}
}

// A grant's checkout is named from the home directory even when it lies
// inside the repository the list runs in: a bare "metasystem" names nothing.
func TestGrantListNamesTheCheckoutFromHome(t *testing.T) {
	t.Parallel()
	until := time.Date(2026, 10, 1, 8, 56, 0, 0, time.UTC)
	entry := goal.PowerOfAttorneyEntry{ID: "G-1", By: "human:wido", Verbs: []string{goal.GeneralAct}, For: "m1e",
		Checkout: "/Users/wido/GitHub/m1e/metasystem", Until: until.Format(time.RFC3339)}
	page := textui.New(textui.Env{Width: 100, Now: until.Add(-time.Hour), Zone: time.UTC, Home: "/Users/wido", Repo: "/Users/wido/GitHub/m1e"})
	grantListView([]grantShown{{entry: entry, live: true}}, false)(page)
	if got := page.String(); !strings.Contains(got, "for   the main session of m1e (~/GitHub/m1e/metasystem)") {
		t.Errorf("grant list = %q", got)
	}
}
