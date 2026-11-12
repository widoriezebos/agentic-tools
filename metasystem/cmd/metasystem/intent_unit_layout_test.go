package main

import (
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// Git's amendment, replay and read publication must agree on the installation
// inside the candidate repository; a stub cannot prove that history.
func testIntentUnitAmendPublicationGitAdapter(t *testing.T) {
	forUnitLayouts(t, func(t *testing.T, installation string) {
		c := newConnectionBed(t, installation)
		build := func(unit, content string) string {
			t.Helper()
			c.edits = map[string]string{unit + ".txt": content}
			code, built := c.do(append([]string{"work", "build", c.id, unit, "--brief", c.brief(unit+".md", "Build the unit.\n"), "--lines", "5"}, workCheck...)...)
			if code != 0 {
				t.Fatalf("build %s: %d %+v", unit, code, built)
			}
			run := resultData(t, built)["run"].(string)
			if code, reviewed := c.do("work", "review", "run:"+run); reviewed.Outcome != intentInProgress {
				t.Fatalf("review %s: %d %+v", unit, code, reviewed)
			}
			return run
		}
		run := build("connect", "the built result\n")
		first := c.runRecord(run).Subjects[0].Commit
		c.writeCritic(c.workInstallation(), "crit1", first, "completed", false)
		if code, reviewed := c.do("work", "review", "run:"+run); code != 0 || reviewed.Outcome != intentConfirmed {
			t.Fatalf("publish first read: %d %+v", code, reviewed)
		}
		later := build("later", "a later unit\n")
		c.writeCritic(c.workInstallation(), "crit2", c.runRecord(later).Subjects[0].Commit, "cancelled", false)
		if _, err := (&launch.UnitRunner{Root: c.unitRoot, Manager: c.manager, Git: launch.OSGitRunner{}}).CancelRun(later); err != nil {
			t.Fatal(err)
		}
		c.edits = map[string]string{"connect.txt": "the built result, fixed\n"}
		if code, revised := c.do("work", "revise", "run:"+run, "--brief", c.brief("amend.md", "Correct the unit.\n"), "--reason", "Amend the reviewed unit", "--by", "Wido"); code != 0 {
			t.Fatalf("revise: %d %+v", code, revised)
		}
		if code, reviewed := c.do("work", "review", "run:"+run); reviewed.Outcome != intentInProgress || len(c.delegates) != 3 {
			t.Fatalf("amend: %d %+v delegates=%v", code, reviewed, c.delegates)
		}
		subjects := c.runRecord(run).Subjects
		if len(subjects) != 2 || subjects[1].Amends != first || subjects[1].Commit == first || subjects[1].Commit == subjects[1].Tip || c.delegates[2] != subjects[1].Commit {
			t.Fatalf("the replacement must be reviewed separately from the replayed tip: %+v", subjects)
		}
		if len(c.gateDirectories) == 0 {
			t.Fatal("the amend never ran its gate")
		}
		for _, directory := range c.gateDirectories {
			if installation != "." && filepath.Base(directory) != installation {
				t.Fatalf("amend gate outside the installation: %s", directory)
			}
		}
		c.writeCritic(c.workInstallation(), "crit3", subjects[1].Commit, "completed", false)
		if code, reviewed := c.do("work", "review", "run:"+run); code != 0 || reviewed.Outcome != intentConfirmed {
			t.Fatalf("publish amended read: %d %+v", code, reviewed)
		}
		if status := c.landAdmission(); status.Prefix != 1 || len(status.Units) != 2 || status.Units[0].Commit != subjects[1].Commit {
			t.Fatalf("landing must see the amended read before the later unit: %+v", status)
		}
	})
}
