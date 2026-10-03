package config_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

func TestSetRosterRowDurability(t *testing.T) {
	t.Parallel()
	home := filepath.Join(t.TempDir(), "home")
	write := config.RosterWriter(func(path string, data []byte, mode os.FileMode, anchor string) (bool, error) {
		if anchor != filepath.Dir(home) {
			t.Errorf("anchor = %q; want the home's parent %q", anchor, filepath.Dir(home))
		}
		_, err := atomicfile.WriteFile(path, data, mode, anchor)
		return false, err
	})
	changed, err := write.SetRow(home, "seat", "seat", "codex:gpt-6-sol:xhigh", installed, runs)
	if !changed || !errors.Is(err, config.ErrRosterNotDurable) {
		t.Fatalf("SetRow = %v, %v; want changed and ErrRosterNotDurable", changed, err)
	}
	wantRow(t, home, "seat", "seat", config.RosterAgent{Runtime: "codex", Model: "gpt-6-sol", Effort: "xhigh"})
}

// runs resolves claude's alias claude-fable-5 to claude-fable-5-1, never unknown.
func runs(runtime, model string) (string, error) {
	if model == "unknown" {
		return "", errors.New("no such model")
	} else if runtime == "claude" && model == "claude-fable-5" {
		return "claude-fable-5-1", nil
	}
	return model, nil
}

func setRow(t *testing.T, home, roster, row, value string) {
	t.Helper()
	if changed, err := config.SetRosterRow(home, roster, row, value, installed, runs); err != nil || !changed {
		t.Fatalf("SetRosterRow(%s, %s, %s) = %v, %v; want changed", roster, row, value, changed, err)
	}
}

func wantRow(t *testing.T, home, roster, row string, want config.RosterAgent) {
	t.Helper()
	if agent, err := config.RosterRow(home, roster, row, installed); err != nil || agent != want {
		t.Errorf("RosterRow(%s, %s) = %+v, %v; want %+v", roster, row, agent, err, want)
	}
}

// TestSetRosterRowKeepsARowAnotherWriterAdded: writers read under the lock.
func TestSetRosterRowKeepsARowAnotherWriterAdded(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	rows := []string{"design", "design-critique", "build", "code-critique", "verify", "investigate", "warden", "behavior-judge"}
	start, errs := make(chan struct{}), make([]error, len(rows))
	var writers sync.WaitGroup
	for i, row := range rows {
		writers.Go(func() {
			<-start
			_, errs[i] = config.SetRosterRow(home, "tier-2", row, "claude:model-"+row+":high", installed, runs)
		})
	}
	close(start)
	writers.Wait()
	for i, row := range rows {
		if errs[i] != nil {
			t.Errorf("writing tier-2 %s: %v", row, errs[i])
		}
		wantRow(t, home, "tier-2", row, config.RosterAgent{Runtime: "claude", Model: "model-" + row, Effort: "high"})
	}
}

// TestSetRosterRowRefusesAndLeavesTheFileAsItWas: a refusal names why.
func TestSetRosterRowRefusesAndLeavesTheFileAsItWas(t *testing.T) {
	t.Parallel()
	const (
		malformed = "a row is RUNTIME:MODEL:EFFORT, and effort is low, medium, high, xhigh or max; nothing was changed"
		partner   = "the Partner's row is RUNTIME:MODEL; nothing was changed"
		builds    = "tier-3 build and code-critique would both run claude-opus-5-5; the author and the reviewer of a piece of work are different models; nothing was changed"
		designs   = "tier-3 design and design-critique would both run claude-opus-5-5; the author and the reviewer of a piece of work are different models; nothing was changed"
		alias     = "tier-3 design and design-critique would both run claude-fable-5-1 (design is written claude-fable-5); the author and the reviewer of a piece of work are different models; nothing was changed"
	)
	tierThree := func(row, model string) string {
		return rostersJSON(`"tier-3": {"type": "tier", "rows": {"` + row + `": {"runtime": "claude", "model": "` + model + `", "effort": "high"}}}`)
	}
	sound := rostersJSON(tierOneDesign + `}}`)
	cases := []struct{ name, content, roster, row, value, want string }{
		{"an unknown roster", "", "tier-9", "build", "claude:claude-opus-5-5:high", "tier-9 is not a roster; the rosters are tier-1, tier-2, tier-3, seat, landing and partner; nothing was changed"},
		{"an unknown row", sound, "tier-1", "tester", "claude:claude-opus-5-5:high", "tester is not a row of tier-1; its rows are design, design-critique, build, code-critique, verify, investigate, warden and behavior-judge; nothing was changed"},
		{"an unknown effort", sound, "tier-1", "build", "claude:claude-opus-5-5:huge", malformed},
		{"no effort", "", "tier-1", "build", "claude:claude-opus-5-5", malformed},
		{"a fourth field", sound, "tier-1", "build", "claude:claude-opus-5-5:high:max", malformed},
		{"a trailing colon", "", "tier-1", "build", "claude:claude-opus-5-5:high:", malformed},
		{"an empty model", "", "tier-1", "build", "claude::high", malformed},
		{"an empty value", "", "tier-1", "build", "", malformed},
		{"a runtime not installed", sound, "tier-1", "build", "gemini:gemini-3-pro:high", "gemini is not one of this installation's runtimes (claude, codex); nothing was changed"},
		{"a Partner effort", "", "partner", "partner", "claude:claude-opus-5-5:high", "the Partner takes no effort; write RUNTIME:MODEL; nothing was changed"},
		{"a Partner empty effort", "", "partner", "partner", "claude:claude-opus-5-5:", partner},
		{"a Partner fourth field", "", "partner", "partner", "claude:claude-opus-5-5:high:max", partner},
		{"build after code-critique", tierThree("code-critique", "claude-opus-5-5"), "tier-3", "build", "claude:claude-opus-5-5:xhigh", builds},
		{"code-critique after build", tierThree("build", "claude-opus-5-5"), "tier-3", "code-critique", "claude:claude-opus-5-5:xhigh", builds},
		{"design after design-critique", tierThree("design-critique", "claude-opus-5-5"), "tier-3", "design", "claude:claude-opus-5-5:max", designs},
		{"design-critique after design", tierThree("design", "claude-opus-5-5"), "tier-3", "design-critique", "claude:claude-opus-5-5:max", designs},
		{"design-critique on what design's alias runs", tierThree("design", "claude-fable-5"), "tier-3", "design-critique", "claude:claude-fable-5-1:xhigh", alias},
		{"design on an alias of design-critique", tierThree("design-critique", "claude-fable-5-1"), "tier-3", "design", "claude:claude-fable-5:max", alias},
		{"design-critique on an alias of design", tierThree("design", "claude-fable-5-1"), "tier-3", "design-critique", "claude:claude-fable-5:max", strings.Replace(alias, "design is", "design-critique is", 1)},
		{"a model that does not resolve", tierThree("design", "claude-opus-5-5"), "tier-3", "design-critique", "claude:unknown:high", "can't tell which model claude runs for unknown: no such model; nothing was changed"},
		{"main in build", sound, "tier-1", "build", "main", "main is only for verify, investigate, warden and behavior-judge; nothing was changed"},
		{"main for the Partner", "", "partner", "partner", "main", "main is only for verify, investigate, warden and behavior-judge; nothing was changed"},
		{"main with a model", "", "tier-1", "verify", "main:claude-opus-5-5", "main takes no model or effort; nothing was changed"},
		{"a placeholder model", sound, "tier-1", "build", "codex:<model>:high", "<model> is a template placeholder, not a model; nothing was changed"},
		{"runtime auto", sound, "tier-1", "build", "auto:claude-opus-5-5:high", "auto is not a runtime; a row names this computer's agent; nothing was changed"},
		{"an unusable version", `{"version": 2}`, "tier-1", "build", "claude:claude-opus-5-5:high", "the rosters file FILE can't be used: its version is 2, and this engine reads version 1; nothing was changed"},
		{"a broken row elsewhere", rostersJSON(tierOneDesign + `, "build": {"runtime": "claude", "model": "claude-opus-5-5"}}}`), "seat", "seat", "claude:claude-opus-5-5:high", "the rosters file FILE can't be used: tier-1 build: a row is RUNTIME:MODEL:EFFORT, and effort is low, medium, high, xhigh or max; nothing was changed"},
		{"an unknown field", `{"version": 1, "colour": "red"}`, "seat", "seat", "claude:claude-opus-5-5:high", `the rosters file FILE can't be used: it is not valid rosters JSON (json: unknown field "colour"); nothing was changed`},
		{"trailing bytes", rostersJSON(tierOneDesign+`}}`) + "\n}", "tier-1", "design", "codex:gpt-5.5:high", "the rosters file FILE can't be used: it holds more after its JSON object; nothing was changed"},
	}
	for _, c := range cases {
		home := rosterHome(t, c.content)
		before, beforeErr := os.ReadFile(config.RostersPath(home))
		changed, err := config.SetRosterRow(home, c.roster, c.row, c.value, installed, runs)
		if want := strings.ReplaceAll(c.want, "FILE", config.RostersPath(home)); changed || err == nil || err.Error() != want {
			t.Errorf("%s: SetRosterRow(%s, %s, %q) = %v, %v;\nwant false, %q", c.name, c.roster, c.row, c.value, changed, err, want)
		}
		after, afterErr := os.ReadFile(config.RostersPath(home))
		if errors.Is(afterErr, fs.ErrNotExist) != (c.content == "") || c.content != "" && (beforeErr != nil || afterErr != nil) || string(after) != string(before) {
			t.Errorf("%s: the refused write changed the rosters file from %q to %q (%v, %v)", c.name, before, after, beforeErr, afterErr)
		}
	}
	if changed, err := config.SetRosterRow("relative", "seat", "seat", "claude:claude-opus-5-5:high", installed, runs); changed || err == nil || err.Error() != `the rosters need an absolute home, got "relative"; nothing was changed` {
		t.Errorf("a relative home got %v, %v; want it refused", changed, err)
	}
}

// TestSetRosterRowRepeatedWritesNothing: a row's own value is no error.
func TestSetRosterRowRepeatedWritesNothing(t *testing.T) {
	t.Parallel()
	home := rosterHome(t, "")
	setRow(t, home, "tier-1", "build", "codex:gpt-5.5:high")
	before, beforeData := stat(t, home)
	if changed, err := config.SetRosterRow(home, "tier-1", "build", "codex:gpt-5.5:high", installed, runs); changed || err != nil {
		t.Errorf("repeating tier-1 build = %v, %v; want false, nil", changed, err)
	}
	after, afterData := stat(t, home)
	if !os.SameFile(before, after) || !after.ModTime().Equal(before.ModTime()) || string(afterData) != string(beforeData) {
		t.Errorf("repeating a row replaced the file: %v at %v became %v at %v", before.Sys(), before.ModTime(), after.Sys(), after.ModTime())
	}
	setRow(t, home, "tier-1", "build", "codex:gpt-5.5:xhigh")
	wantRow(t, home, "tier-1", "build", config.RosterAgent{Runtime: "codex", Model: "gpt-5.5", Effort: "xhigh"})
}

func stat(t *testing.T, home string) (fs.FileInfo, []byte) {
	t.Helper()
	info, err := os.Stat(config.RostersPath(home))
	data, readErr := os.ReadFile(config.RostersPath(home))
	if err != nil || readErr != nil {
		t.Fatal(err, readErr)
	}
	return info, data
}

// TestSetRosterRowCreatesARosterWithItsType: a roster's first row types it;
// the Partner's row is RUNTIME:MODEL, and main is the calling session.
func TestSetRosterRowCreatesARosterWithItsType(t *testing.T) {
	t.Parallel()
	for _, content := range []string{"", `{"version": 1}`, rostersJSON(`"seat": {"type": "seat"}`), rostersJSON(tierOneDesign + `}}`)} {
		home := rosterHome(t, content)
		setRow(t, home, "seat", "seat", "claude:claude-opus-5-5:xhigh")
		var file struct {
			Version int
			Rosters map[string]struct{ Type string }
		}
		data, err := os.ReadFile(config.RostersPath(home))
		if err != nil || json.Unmarshal(data, &file) != nil || file.Version != 1 || file.Rosters["seat"].Type != "seat" {
			t.Errorf("after writing into %q the file reads %s (%v); want version 1 and roster seat of type seat", content, data, err)
		}
		wantRow(t, home, "seat", "seat", config.RosterAgent{Runtime: "claude", Model: "claude-opus-5-5", Effort: "xhigh"})
		if content == rostersJSON(tierOneDesign+`}}`) {
			wantRow(t, home, "tier-1", "design", config.RosterAgent{Runtime: "claude", Model: "claude-opus-5-5", Effort: "xhigh"})
		}
		setRow(t, home, "partner", "partner", "claude:claude-opus-5-5")
		setRow(t, home, "tier-2", "verify", "main")
		wantRow(t, home, "partner", "partner", config.RosterAgent{Runtime: "claude", Model: "claude-opus-5-5"})
		wantRow(t, home, "tier-2", "verify", config.RosterAgent{Runtime: "main"})
		if info, err := os.Stat(filepath.Dir(config.RostersPath(home))); content == "" && (err != nil || info.Mode().Perm() != 0o700) {
			t.Errorf("the host directory is %v, %v; want mode 0700", info, err)
		}
	}
}

// TestSetRosterRowAllowsOneModelAcrossRosters: the author-and-reviewer rule
// holds within one roster and compares the models each row's runtime runs.
func TestSetRosterRowAllowsOneModelAcrossRosters(t *testing.T) {
	t.Parallel()
	home := rosterHome(t, "")
	setRow(t, home, "tier-1", "build", "claude:claude-opus-5-5:high")
	setRow(t, home, "tier-2", "code-critique", "claude:claude-opus-5-5:high")
	setRow(t, home, "tier-1", "design-critique", "claude:claude-opus-5-5:xhigh")
	setRow(t, home, "tier-1", "code-critique", "codex:gpt-5.5:high")
	setRow(t, home, "tier-1", "build", "claude:claude-opus-5-5:max")
	setRow(t, home, "tier-1", "design", "claude:claude-fable-5:high")
	setRow(t, home, "tier-1", "design-critique", "codex:claude-fable-5:high")
	setRow(t, home, "tier-2", "design", "claude:claude-fable-5-1:high")
	setRow(t, home, "tier-2", "design-critique", "codex:claude-fable-5:high")
	wantRow(t, home, "tier-1", "build", config.RosterAgent{Runtime: "claude", Model: "claude-opus-5-5", Effort: "max"})
	wantRow(t, home, "tier-2", "code-critique", config.RosterAgent{Runtime: "claude", Model: "claude-opus-5-5", Effort: "high"})
}
