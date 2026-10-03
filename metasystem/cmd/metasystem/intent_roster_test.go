package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func init() {
	registerIdempotency("roster list", idemRead, "", nil)
	registerIdempotency("roster show", idemRead, "", nil)
	registerIdempotency("roster set", idemStateful, "the same row value changes nothing", TestRosterSetNeedsThePersonAndWritesOneRow)
}

// rosterRun runs roster verbs in a checkout listing runtimes, with a home holding rosters.
func rosterRun(t *testing.T, runtimes, rosters string, person ...bool) (home string, run func(words ...string) (int, string)) {
	t.Helper()
	base := realpath.Resolve(t.TempDir())
	checkout, home := filepath.Join(base, "checkout"), filepath.Join(base, "home")
	helmMust(t, os.MkdirAll(filepath.Join(checkout, ".git"), 0o755), os.MkdirAll(filepath.Join(checkout, "metasystem"), 0o755), os.MkdirAll(filepath.Join(home, "host"), 0o700),
		os.WriteFile(filepath.Join(checkout, "metasystem", "metasystem.conf"), []byte("metasystem.template=true\nmetasystem.runtimes="+runtimes+"\n"), 0o644))
	if rosters != "" {
		helmMust(t, os.WriteFile(config.RostersPath(home), []byte(rosters), 0o600))
	}
	owners := intentOwners{resolver: stateroot.NewResolver(func(string) (string, error) { return checkout, nil }, os.Executable),
		commandNow: func(string) (time.Time, error) { return helmNow, nil },
		prove: func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
			return humanauthority.Proof{}, errors.New("not the person")
		},
		rosters: rosterOwners{home: func() (string, error) { return home, nil }, lookupEnv: func(string) (string, bool) { return "", false }}}
	if len(person) > 0 && person[0] {
		owners.prove = enrolledPersonProver(t, filepath.Join(checkout, "metasystem"), helmNow)
	}
	return home, func(words ...string) (int, string) {
		command, _ := findIntentAction("roster", words[0])
		var out bytes.Buffer
		return runIntentIn(command, words[1:], &out, &out, checkout, owners), out.String()
	}
}

const rosterRows = `{"version": 1, "rosters": {"tier-1": {"type": "tier", "rows": {"build": {"runtime": "claude", "model": "claude-opus-5-5", "effort": "xhigh"}}}, "seat": {"type": "seat", "rows": {"seat": {"runtime": "codex", "model": "gpt-5.5", "effort": "high"}}}`

// TestRosterShowPrintsRowsAndNotSet: a row no person set reads "not set" and
// names no agent, in the text and in --json; roster list prints every roster.
func TestRosterShowPrintsRowsAndNotSet(t *testing.T) {
	t.Parallel()
	_, run := rosterRun(t, "claude,codex", rosterRows+`}}`)
	tierOne := "tier-1 (type tier)\n  design  not set\n  design-critique  not set\n  build  claude  claude-opus-5-5  xhigh\n  code-critique  not set\n  verify  not set\n  investigate  not set\n  warden  not set\n  behavior-judge  not set\n"
	if code, out := run("show", "tier-1"); code != 0 || !strings.HasSuffix(out, "\n"+tierOne) || strings.Contains(out, "seat") {
		t.Errorf("roster show tier-1 = %d\n%s\nwant its eight rows:\n%s", code, out, tierOne)
	}
	if code, out := run("list"); code != 0 || !strings.Contains(out, "\n"+tierOne+"tier-2 (type tier)\n") || !strings.Contains(out, "tier-3 (type tier)\n  design  not set\n") ||
		!strings.HasSuffix(out, "seat (type seat)\n  seat  codex  gpt-5.5  high\n  steward  not set\nlanding (type landing)\n  landing  not set\npartner (type partner)\n  partner  not set\n") {
		t.Errorf("roster list = %d\n%s\nwant all six rosters in order", code, out)
	}
	var data struct{ Rosters []config.RosterView }
	code, out := run("list", "--json")
	if err := json.Unmarshal([]byte(out), &struct{ Data any }{&data}); code != 0 || err != nil || len(data.Rosters) != 6 || data.Rosters[5].ID != "partner" {
		t.Fatalf("roster list --json = %d %v\n%s", code, err, out)
	}
	if rows := data.Rosters[0].Rows; rows[2] != (config.RosterLine{Row: "build", Set: true, RosterAgent: config.RosterAgent{Runtime: "claude", Model: "claude-opus-5-5", Effort: "xhigh"}}) || rows[3] != (config.RosterLine{Row: "code-critique"}) {
		t.Errorf("roster list --json tier-1 rows = %+v, want build set and code-critique not set", rows)
	}
}

// An unknown roster is refused before the file is read; an unusable file
// prints no row, even a sound one, and never sends the person to roster list.
func TestRosterVerbsRefuseAnUnknownRosterAndAnUnusableFile(t *testing.T) {
	t.Parallel()
	home, run := rosterRun(t, "claude,codex", rosterRows+`, "tier-4": {"type": "tier", "rows": {}}}}`)
	if code, out := run("show", "tier-9"); code != 2 || !strings.HasPrefix(out, "✗ tier-9 is not a roster; the rosters are tier-1, tier-2, tier-3, seat, landing and partner; nothing was read\n") {
		t.Errorf("roster show tier-9 = %d\n%s", code, out)
	}
	want := "The rosters file " + config.RostersPath(home) + " can't be used: tier-4 is not a roster. Nothing was read."
	for _, words := range [][]string{{"list"}, {"show", "tier-1"}, {"list", "--json"}} {
		if code, out := run(words...); code != 1 || !strings.Contains(out, want) ||
			!strings.Contains(out, "a person repairs or removes the file; metasystem roster set refuses to write over it") || strings.Contains(out, "claude-opus") || strings.Contains(out, "metasystem roster list") || strings.Contains(out, `"next"`) {
			t.Errorf("roster %v = %d\n%s\nwant exit 1, %q and no row", words, code, out, want)
		}
	}
}

func TestRosterSetNeedsThePersonAndWritesOneRow(t *testing.T) {
	t.Run("not the person", func(t *testing.T) {
		home, run := rosterRun(t, "claude,codex", "")
		if code, out := run("set", "tier-1", "build", "codex:gpt-6-sol:xhigh"); code != 1 ||
			!strings.HasPrefix(out, "✗ only you set roster set, at your enrolled terminal, and not the person; nothing was done\n") {
			t.Errorf("an agent's roster set = %d\n%s", code, out)
		}
		if _, err := os.Stat(config.RostersPath(home)); !os.IsNotExist(err) {
			t.Fatalf("the refused act created a rosters file: %v", err)
		}
	})
	t.Run("the person", func(t *testing.T) {
		home, run := rosterRun(t, "claude,codex", "", true)
		if code, out := run("set", "tier-1", "build", "codex:gpt-6-sol:xhigh"); code != 0 ||
			out != "tier-1 build is now codex gpt-6-sol xhigh on this computer; the next build of a tier-1 goal uses it.\n" {
			t.Fatalf("the person's roster set = %d\n%s", code, out)
		}
		agent, err := config.RosterRow(home, "tier-1", "build", []string{"claude", "codex"})
		if err != nil || agent != (config.RosterAgent{Runtime: "codex", Model: "gpt-6-sol", Effort: "xhigh"}) {
			t.Fatalf("the written row = %+v, %v", agent, err)
		}
		views, err := config.Rosters(home)
		helmMust(t, err)
		set := 0
		for _, view := range views {
			for _, row := range view.Rows {
				if row.Set {
					set++
				}
			}
		}
		if set != 1 {
			t.Fatalf("roster set wrote %d rows, want exactly one", set)
		}
		before, err := os.ReadFile(config.RostersPath(home))
		helmMust(t, err)
		if code, out := run("set", "tier-1", "build", "codex:gpt-6-sol:xhigh"); code != 0 || out != "tier-1 build already is codex gpt-6-sol xhigh; nothing was changed.\n" {
			t.Fatalf("a repeat = %d\n%s", code, out)
		}
		code, out := run("set", "tier-1", "build", "codex:gpt-6-sol:xhigh", "--json")
		var result intentResult
		err = json.Unmarshal([]byte(out), &result)
		want := map[string]any{"roster": "tier-1", "row": "build", "changed": false, "runtime": "codex", "model": "gpt-6-sol", "effort": "xhigh"}
		if code != 0 || err != nil || result.Outcome != intentUnchanged || !reflect.DeepEqual(result.Data, want) {
			t.Fatalf("repeat JSON = %d %v\n%s", code, err, out)
		}
		after, err := os.ReadFile(config.RostersPath(home))
		helmMust(t, err)
		if !bytes.Equal(before, after) {
			t.Fatal("a repeat changed the file")
		}
	})
}

func TestRosterSetTrimsInstalledRuntimes(t *testing.T) {
	t.Parallel()
	home, run := rosterRun(t, "claude,\tcodex", "", true)
	if code, out := run("set", "tier-1", "build", "codex:gpt-6-sol:xhigh"); code != 0 {
		t.Fatalf("roster set with whitespace = %d\n%s", code, out)
	}
	got, err := config.RosterRow(home, "tier-1", "build", []string{"claude", "codex"})
	if want := (config.RosterAgent{Runtime: "codex", Model: "gpt-6-sol", Effort: "xhigh"}); err != nil || got != want {
		t.Fatalf("stored build row = %+v, %v; want %+v", got, err, want)
	}
	for _, words := range [][]string{{"show", "tier-1"}, {"list"}} {
		if code, out := run(words...); code != 0 || !strings.Contains(out, "\n  build  codex  gpt-6-sol  xhigh\n") {
			t.Errorf("roster %s with whitespace = %d\n%s", strings.Join(words, " "), code, out)
		}
	}
}

func TestRosterSetAnswersWhenEachKindUsesTheRow(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ roster, row, value, use string }{
		{"tier-2", "build", "codex:gpt-6-sol:high", "the next build of a tier-2 goal uses it"},
		{"tier-3", "build", "codex:gpt-6-sol:xhigh", "the next build of a tier-3 goal, or of work with no goal, uses it"},
		{"seat", "seat", "claude:claude-opus-5-5:xhigh", "the next seat agent uses it"},
		{"seat", "steward", "claude:claude-opus-5-5:xhigh", "the next steward continuation uses it"},
		{"landing", "landing", "claude:claude-opus-5-5:xhigh", "the next landing agent uses it"},
		{"partner", "partner", "claude:claude-opus-5-5", "the Project Partner uses it from its next turn"},
		{"tier-1", "verify", "main", "the calling session does that work itself"},
	} {
		t.Run(test.roster+" "+test.row, func(t *testing.T) {
			_, run := rosterRun(t, "claude,codex", "", true)
			want := test.roster + " " + test.row + " is now " + strings.ReplaceAll(test.value, ":", " ") + " on this computer; " + test.use + ".\n"
			if code, out := run("set", test.roster, test.row, test.value); code != 0 || out != want {
				t.Fatalf("roster set = %d\n%s\nwant %s", code, out, want)
			}
			_, jsonRun := rosterRun(t, "claude,codex", "", true)
			code, out := jsonRun("set", test.roster, test.row, test.value, "--json")
			var result intentResult
			err := json.Unmarshal([]byte(out), &result)
			fields := append(strings.Split(test.value, ":"), "", "")
			data := map[string]any{"roster": test.roster, "row": test.row, "changed": true, "runtime": fields[0], "model": fields[1], "effort": fields[2]}
			if code != 0 || err != nil || result.Outcome != intentConfirmed || result.Summary != strings.TrimSpace(want) || !reflect.DeepEqual(result.Data, data) {
				t.Fatalf("set JSON = %d %v\n%s", code, err, out)
			}
		})
	}
}

func TestRosterSetRefusesTheCompiledAliasPair(t *testing.T) {
	t.Parallel()
	home, run := rosterRun(t, "claude,codex", "", true)
	if code, out := run("set", "tier-1", "design", "claude:claude-fable-5:high"); code != 0 {
		t.Fatalf("set the design row = %d\n%s", code, out)
	}
	before, err := os.ReadFile(config.RostersPath(home))
	helmMust(t, err)
	want := "tier-1 design and design-critique would both run claude-fable-5-1 (design is written claude-fable-5); the author and the reviewer of a piece of work are different models; nothing was changed"
	if code, out := run("set", "tier-1", "design-critique", "claude:claude-fable-5-1:high"); code != 2 || out != "✗ "+want+"\n" {
		t.Fatalf("the alias pair = %d\n%s\nwant %s", code, out, want)
	}
	after, err := os.ReadFile(config.RostersPath(home))
	helmMust(t, err)
	if !bytes.Equal(before, after) {
		t.Fatal("the refused alias pair changed the file")
	}
}

func TestRosterSetShowsTheStoreRefusal(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ roster, row, value, problem string }{
		{"tier-9", "build", "codex:gpt-6-sol:xhigh", "tier-9 is not a roster; the rosters are tier-1, tier-2, tier-3, seat, landing and partner"},
		{"tier-1", "tester", "codex:gpt-6-sol:xhigh", "tester is not a row of tier-1; its rows are design, design-critique, build, code-critique, verify, investigate, warden and behavior-judge"},
		{"tier-1", "build", "codex:gpt-6-sol", "a row is RUNTIME:MODEL:EFFORT, and effort is low, medium, high, xhigh or max"},
		{"partner", "partner", "claude:claude-opus-5-5:high", "the Partner takes no effort; write RUNTIME:MODEL"},
		{"tier-1", "build", "gemini:gemini-3-pro:high", "gemini is not one of this installation's runtimes (claude, codex)"},
	} {
		t.Run(test.problem, func(t *testing.T) {
			home, run := rosterRun(t, "claude,codex", "", true)
			if code, out := run("set", test.roster, test.row, test.value); code != 2 || out != "✗ "+test.problem+"; nothing was changed\n" {
				t.Fatalf("store refusal = %d\n%s", code, out)
			}
			code, out := run("set", test.roster, test.row, test.value, "--json")
			var result intentResult
			if err := json.Unmarshal([]byte(out), &result); code != 2 || err != nil || result.Summary != test.problem+"; nothing was changed" {
				t.Fatalf("store refusal JSON = %d %v\n%s", code, err, out)
			}
			if _, err := os.Stat(config.RostersPath(home)); !os.IsNotExist(err) {
				t.Fatalf("the refused input created a file: %v", err)
			}
		})
	}
	home, run := rosterRun(t, "claude,codex", `{"version":2}`, true)
	if code, out := run("set", "tier-1", "build", "codex:gpt-6-sol:xhigh"); code != 1 || out != "✗ the rosters file "+config.RostersPath(home)+" can't be used: its version is 2, and this engine reads version 1; nothing was changed\n" {
		t.Fatalf("an unusable file = %d\n%s", code, out)
	}
	if after, err := os.ReadFile(config.RostersPath(home)); err != nil || string(after) != `{"version":2}` {
		t.Fatalf("the unusable file was changed: %q %v", after, err)
	}
}

// A runtime is judged by the installation's own settings; main and the Partner print fewer fields.
func TestRosterShowMarksARuntimeTheInstallationDoesNotRun(t *testing.T) {
	t.Parallel()
	rosters := `{"version": 1, "rosters": {"tier-2": {"type": "tier", "rows": {"build": {"runtime": "gemini", "model": "gemini-3-pro", "effort": "high"}, "verify": {"runtime": "main"}}}, "partner": {"type": "partner", "rows": {"partner": {"runtime": "claude", "model": "claude-opus-5-5"}}}}}`
	for runtimes, build := range map[string]string{"claude,codex": "  build  gemini  gemini-3-pro  high  (this installation does not run gemini)\n", "claude, gemini": "  build  gemini  gemini-3-pro  high\n"} {
		_, run := rosterRun(t, runtimes, rosters)
		if code, out := run("show", "tier-2"); code != 0 || !strings.Contains(out, "\n"+build+"  code-critique  not set\n  verify  main\n") {
			t.Errorf("with runtimes %s roster show tier-2 = %d\n%s\nwant %q and verify main", runtimes, code, out, build)
		}
		if code, out := run("show", "partner"); code != 0 || !strings.HasSuffix(out, "\npartner (type partner)\n  partner  claude  claude-opus-5-5\n") {
			t.Errorf("roster show partner = %d\n%s", code, out)
		}
	}
}
