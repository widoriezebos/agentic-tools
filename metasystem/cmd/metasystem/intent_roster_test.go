package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func init() {
	registerIdempotency("roster list", idemRead, "", nil)
	registerIdempotency("roster show", idemRead, "", nil)
}

// rosterRun runs roster verbs in a checkout listing runtimes, with a home holding rosters.
func rosterRun(t *testing.T, runtimes, rosters string) (home string, run func(words ...string) (int, string)) {
	base := realpath.Resolve(t.TempDir())
	checkout, home := filepath.Join(base, "checkout"), filepath.Join(base, "home")
	helmMust(t, os.MkdirAll(filepath.Join(checkout, ".git"), 0o755), os.MkdirAll(filepath.Join(checkout, "metasystem"), 0o755), os.MkdirAll(filepath.Join(home, "host"), 0o700),
		os.WriteFile(filepath.Join(checkout, "metasystem", "metasystem.conf"), []byte("metasystem.template=true\nmetasystem.runtimes="+runtimes+"\n"), 0o644), os.WriteFile(config.RostersPath(home), []byte(rosters), 0o600))
	owners := intentOwners{resolver: stateroot.NewResolver(func(string) (string, error) { return checkout, nil }, os.Executable),
		rosters: rosterOwners{home: func() (string, error) { return home, nil }, lookupEnv: func(string) (string, bool) { return "", false }}}
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
			!strings.Contains(out, "a person repairs or removes the file; setting a row refuses to write over it") || strings.Contains(out, "claude-opus") || strings.Contains(out, "metasystem roster list") || strings.Contains(out, `"next"`) {
			t.Errorf("roster %v = %d\n%s\nwant exit 1, %q and no row", words, code, out, want)
		}
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
