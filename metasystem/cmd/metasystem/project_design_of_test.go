package main

// design-of is the boundary a seat runs before it calls a design done, and
// the design critique runs before it attacks one. These tests drive it over
// its own streams, in both layouts, because the layout is where the home is
// decided and where a seat writing into the installation goes wrong.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// projectAdoptedFixture is the other layout: an application repository with
// the installation vendored beneath it. There the state root is the
// application, so the application's designs are the ones the resolver reads
// and the installation's own plans/designs is a decoy — the path a seat
// reaches for when it mistakes the installation for the home.
func projectAdoptedFixture(t *testing.T) string {
	t.Helper()

	checkout := filepath.Join(t.TempDir(), "application")
	if err := os.MkdirAll(checkout, 0o755); err != nil {
		t.Fatalf("create the checkout: %v", err)
	}
	if output, err := exec.Command("git", "init", "-q", "-b", "main", checkout).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	canonical, err := filepath.EvalSymlinks(checkout)
	if err != nil {
		t.Fatalf("canonicalize the checkout: %v", err)
	}
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(canonical, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create the directory for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	write("vendor/metasystem/metasystem.conf", "metasystem.runtimes=claude\n")
	if err := os.MkdirAll(filepath.Join(canonical, "vendor", "metasystem", "scripts", "agents"), 0o755); err != nil {
		t.Fatalf("create the installation's scripts: %v", err)
	}
	write("plans/goals/backlog.md", "# backlog\n\n- SyncMode: local\n")
	write("plans/goals/refund-worker.md",
		"# refund-worker\n\n- State: queued\n- Intent: Refunds are paid back within a day\n")
	write("plans/designs/refunds.md",
		"# The refund worker\n\n- Kind: design\n- Id: design-refunds\n- Status: draft\n- Goals: refund-worker\n")
	write("vendor/metasystem/plans/designs/decoy.md",
		"# Never read\n\n- Kind: design\n- Id: design-decoy\n- Status: draft\n- Goals: refund-worker\n")
	return filepath.Join(canonical, "vendor", "metasystem")
}

// A goal's designs are the design records that name it, wherever they are
// written: the installation's home and the checkout's second home read alike,
// and the answer is ordered the way every listing is ordered, by path.
func TestProjectDesignOfVerbFindsEveryDesignThatNamesTheGoal(t *testing.T) {
	t.Parallel()

	root := projectFixture(t)
	code, out, problem := runProjectVerb(projectDesignOf, []string{"--root", root, "--goal", "billing-run"})
	if code != 0 || problem != "" {
		t.Fatalf("project design-of = code %d, stderr %q", code, problem)
	}
	want := "design-ledger\tdone\tmetasystem/plans/designs/ledger.md\n" +
		"design-interface\tdraft\tplans/designs/interface.md\n"
	if out != want {
		t.Fatalf("project design-of printed\n%q\nwant\n%q", out, want)
	}

	code, out, problem = runProjectVerb(projectDesignOf,
		[]string{"--root", root, "--goal", "billing-run", "--json"})
	wantJSON := `{"goal":"billing-run","designs":[` +
		`{"id":"design-ledger","status":"done","path":"metasystem/plans/designs/ledger.md"},` +
		`{"id":"design-interface","status":"draft","path":"plans/designs/interface.md"}]}` + "\n"
	if code != 0 || problem != "" || out != wantJSON {
		t.Fatalf("project design-of --json = code %d, stdout %q, stderr %q", code, out, problem)
	}
}

// One design is the ordinary case: the record a seat has just written, found
// by the goal it names.
func TestProjectDesignOfVerbFindsTheOneDesignAGoalHas(t *testing.T) {
	t.Parallel()

	root := projectFixture(t)
	design := filepath.Join(root, "plans", "designs", "shipping.md")
	if err := os.WriteFile(design,
		[]byte("# The first release\n\n- Kind: design\n- Id: design-shipping\n- Status: accepted\n- Goals: shipped\n"),
		0o644); err != nil {
		t.Fatalf("write the design: %v", err)
	}
	code, out, problem := runProjectVerb(projectDesignOf, []string{"--root", root, "--goal", "shipped"})
	if code != 0 || problem != "" {
		t.Fatalf("project design-of = code %d, stderr %q", code, problem)
	}
	if out != "design-shipping\taccepted\tmetasystem/plans/designs/shipping.md\n" {
		t.Fatalf("project design-of printed %q", out)
	}
}

// No design is a refusal, and the refusal names the section that says what a
// design record is rather than repeating it. A reader that asked for JSON
// still gets JSON: the empty list beside the refusal.
func TestProjectDesignOfVerbRefusesAGoalWithNoDesign(t *testing.T) {
	t.Parallel()

	root := projectFixture(t)
	code, out, problem := runProjectVerb(projectDesignOf, []string{"--root", root, "--goal", "shipped"})
	wantProblem := "no design record names goal shipped: a design is a record, " +
		"see docs/design/design-obligation-gate.md\n"
	if code != 1 || out != "" || problem != wantProblem {
		t.Fatalf("a goal with no design = code %d, stdout %q, stderr %q", code, out, problem)
	}

	code, out, problem = runProjectVerb(projectDesignOf, []string{"--root", root, "--goal", "shipped", "--json"})
	if code != 1 || out != `{"goal":"shipped","designs":[]}`+"\n" || problem != wantProblem {
		t.Fatalf("the JSON refusal = code %d, stdout %q, stderr %q", code, out, problem)
	}
}

// A goal the ledger does not have is refused before the answer: "no design
// names it" would be a true sentence about a goal nobody opened.
func TestProjectDesignOfVerbRefusesAGoalTheLedgerDoesNotHave(t *testing.T) {
	t.Parallel()

	root := projectFixture(t)
	code, out, problem := runProjectVerb(projectDesignOf, []string{"--root", root, "--goal", "invoicing"})
	if code != 2 || out != "" ||
		problem != "metasystem project design-of: the goal invoicing is not in the ledger\n" {
		t.Fatalf("an unknown goal = code %d, stdout %q, stderr %q", code, out, problem)
	}
	if code, _, _ := runProjectVerb(projectDesignOf, []string{"--root", root}); code != 2 {
		t.Fatalf("a missing --goal must be usage: code %d", code)
	}
}

// In an adopted installation the home is the application's own
// plans/designs: a design written beneath the installation is read by
// nothing, which is exactly the mistake the gate's section names.
func TestProjectDesignOfVerbReadsTheAdoptedApplicationsHome(t *testing.T) {
	t.Parallel()

	root := projectAdoptedFixture(t)
	code, out, problem := runProjectVerb(projectDesignOf, []string{"--root", root, "--goal", "refund-worker"})
	if code != 0 || problem != "" {
		t.Fatalf("project design-of = code %d, stderr %q", code, problem)
	}
	if out != "design-refunds\tdraft\tplans/designs/refunds.md\n" {
		t.Fatalf("project design-of printed %q; the installation's own designs are no home", out)
	}
	if strings.Contains(out, "design-decoy") {
		t.Fatalf("the decoy under the installation counted: %q", out)
	}
}
