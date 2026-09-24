package main

// The project family drives the one read-only resolver. These tests run the
// verbs' own decisions over their own streams, so the whole family is proved
// without a process's single standard output between the cases.

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// projectFixture is a self-hosted checkout carrying a small declared project:
// a ledger of two goals, an intent book, a decision, and two designs, one of
// them at the checkout root's own second design home.
func projectFixture(t *testing.T) string {
	t.Helper()

	checkout := filepath.Join(t.TempDir(), "checkout")
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
	write("metasystem/metasystem.conf", "metasystem.runtimes=claude\n")
	if err := os.MkdirAll(filepath.Join(canonical, "metasystem", "scripts", "agents"), 0o755); err != nil {
		t.Fatalf("create the installation's scripts: %v", err)
	}
	write("development/metasystem-design.md", "# The metasystem's design\n")

	// The ledger the records are about: one live goal, one concluded, and the
	// root record, which sits among the goal files and is not a goal.
	write("metasystem/plans/goals/backlog.md", "# backlog\n\n- SyncMode: local\n")
	write("metasystem/plans/goals/billing-run.md",
		"# billing-run\n\n- State: queued\n- Intent: Billing runs nightly, and says what it did\n")
	write("metasystem/records/goals/shipped.md",
		"# shipped\n\n- State: done\n- Intent: The first release shipped\n")

	write("metasystem/docs/intent/index.md",
		"# The project's intent\n\n- Kind: intent\n- Id: intent-index\n- Status: accepted\n")
	write("metasystem/docs/decisions/0001-one-binary.md",
		"# One binary\n\n- Kind: decision\n- Id: decision-one-binary\n- Status: accepted\n")
	write("metasystem/plans/designs/ledger.md",
		"# The ledger\n\n- Kind: design\n- Id: design-ledger\n- Status: done\n- Goals: billing-run\n"+
			"- Affects: decision-one-binary\n")
	write("plans/designs/interface.md",
		"# The interface\n\n- Kind: design\n- Id: design-interface\n- Status: draft\n- Goals: billing-run\n"+
			"- Cites: design-ledger\n")
	write("metasystem/memory/questions.md",
		"# Open questions\n\n| id | opened | question | goals | status |\n| --- | --- | --- | --- | --- |\n"+
			"| Q-1 | 2026-09-22 | Where does intent live? |  | open |\n")
	return filepath.Join(canonical, "metasystem")
}

// runProjectVerb drives one verb over its own streams.
func runProjectVerb(verb func([]string, io.Writer, io.Writer) int, args []string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := verb(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestProjectIDVerbMintsOne(t *testing.T) {
	t.Parallel()

	code, out, problem := runProjectVerb(projectID, nil)
	if code != 0 || problem != "" {
		t.Fatalf("project id = code %d, stderr %q", code, problem)
	}
	minted := strings.TrimSuffix(out, "\n")
	if len(minted) != 26 || strings.Trim(minted, "0123456789ABCDEFGHJKMNPQRSTVWXYZ") != "" {
		t.Fatalf("project id printed %q, which is not a 26-character Crockford identity", minted)
	}
	if code, _, _ := runProjectVerb(projectID, []string{"extra"}); code != 2 {
		t.Fatalf("a positional argument must be usage, not a silent mint: code %d", code)
	}
}

func TestProjectListVerbNarrows(t *testing.T) {
	t.Parallel()

	root := projectFixture(t)
	code, out, problem := runProjectVerb(projectList, []string{"--root", root, "design"})
	if code != 0 || problem != "" {
		t.Fatalf("project list design = code %d, stderr %q", code, problem)
	}
	want := "design\tdesign-ledger\tdone\tbilling-run\tThe ledger\tmetasystem/plans/designs/ledger.md\n" +
		"design\tdesign-interface\tdraft\tbilling-run\tThe interface\tplans/designs/interface.md\n"
	if out != want {
		t.Fatalf("project list design printed\n%q\nwant\n%q", out, want)
	}

	_, out, _ = runProjectVerb(projectList, []string{"--root", root, "design", "--status", "draft"})
	if out != "design\tdesign-interface\tdraft\tbilling-run\tThe interface\tplans/designs/interface.md\n" {
		t.Fatalf("--status did not narrow: %q", out)
	}
	_, out, _ = runProjectVerb(projectList, []string{"--root", root, "decision", "--goal", "billing-run"})
	if out != "" {
		t.Fatalf("--goal did not narrow: %q", out)
	}
	// The register's rows list under the fifth kind, as the pages do under
	// their four.
	_, out, _ = runProjectVerb(projectList, []string{"--root", root, "question"})
	if out != "question\tQ-1\topen\t\tWhere does intent live?\tmetasystem/memory/questions.md\n" {
		t.Fatalf("project list question printed %q", out)
	}
	if code, _, _ := runProjectVerb(projectList, []string{"--root", root, "policy"}); code != 2 {
		t.Fatalf("an unknown kind must be usage: code %d", code)
	}
	if code, _, _ := runProjectVerb(projectList, []string{"--root", root, "design", "--status", "pending"}); code != 2 {
		t.Fatalf("an unknown status must be usage: code %d", code)
	}
	if code, _, _ := runProjectVerb(projectList, []string{"--root", root}); code != 2 {
		t.Fatalf("a missing kind must be usage: code %d", code)
	}
}

func TestProjectShowVerbNamesBothHalvesOfAReference(t *testing.T) {
	t.Parallel()

	root := projectFixture(t)
	code, out, problem := runProjectVerb(projectShow, []string{"--root", root, "decision-one-binary"})
	if code != 0 || problem != "" {
		t.Fatalf("project show = code %d, stderr %q", code, problem)
	}
	want := "One binary\nkind: decision\nid: decision-one-binary\nstatus: accepted\ngoals: \n" +
		"path: metasystem/docs/decisions/0001-one-binary.md\nhome: metasystem/docs/decisions\n" +
		"referenced by: design-ledger affects metasystem/plans/designs/ledger.md\n"
	if out != want {
		t.Fatalf("project show printed\n%q\nwant\n%q", out, want)
	}

	_, out, _ = runProjectVerb(projectShow, []string{"--root", root, "design-ledger"})
	if !strings.Contains(out, "affects: decision-one-binary\n") ||
		!strings.Contains(out, "referenced by: design-interface cites plans/designs/interface.md\n") {
		t.Fatalf("project show omitted a record's own references or what references it: %q", out)
	}
	code, _, problem = runProjectVerb(projectShow, []string{"--root", root, "nobody"})
	if code != 1 || !strings.Contains(problem, "no record declares the id nobody") {
		t.Fatalf("an unknown id must refuse: code %d, stderr %q", code, problem)
	}
}

func TestProjectTreeVerbCountsEveryBucket(t *testing.T) {
	t.Parallel()

	root := projectFixture(t)
	code, out, problem := runProjectVerb(projectTree, []string{"--root", root})
	if code != 0 || problem != "" {
		t.Fatalf("project tree = code %d, stderr %q", code, problem)
	}
	want := "billing-run — queued — Billing runs nightly, and says what it did\n" +
		"  kinds: intent 0, doctrine 0, decision 0, design 2, question 0\n" +
		"  status: draft 1, accepted 0, superseded 0, done 1, open 0, answered 0, withdrawn 0\n" +
		"shipped — done — The first release shipped\n" +
		"  kinds: intent 0, doctrine 0, decision 0, design 0, question 0\n" +
		"  status: draft 0, accepted 0, superseded 0, done 0, open 0, answered 0, withdrawn 0\n" +
		"project\n" +
		"  kinds: intent 1, doctrine 0, decision 1, design 0, question 1\n" +
		"  status: draft 0, accepted 2, superseded 0, done 0, open 1, answered 0, withdrawn 0\n"
	if out != want {
		t.Fatalf("project tree printed\n%q\nwant\n%q", out, want)
	}
}

// A goal's intent is a paragraph in this ledger, and the tree is a shape: the
// line carries at most a hundred characters of it, the ellipsis included.
func TestProjectTreeVerbTruncatesALongIntent(t *testing.T) {
	t.Parallel()

	root := projectFixture(t)
	long := strings.Repeat("the reason this goal is open, at length, ", 6)
	if err := os.WriteFile(filepath.Join(root, "plans", "goals", "billing-run.md"),
		[]byte("# billing-run\n\n- State: queued\n- Intent: "+long+"\n"), 0o644); err != nil {
		t.Fatalf("write the long intent: %v", err)
	}
	_, out, _ := runProjectVerb(projectTree, []string{"--root", root})
	first := strings.SplitN(out, "\n", 2)[0]
	intent := strings.TrimPrefix(first, "billing-run — queued — ")
	if len([]rune(intent)) != 100 || !strings.HasSuffix(intent, "…") {
		t.Fatalf("the truncated intent was %q, of %d characters", intent, len([]rune(intent)))
	}
}

func TestProjectCheckVerbPassesAndRefuses(t *testing.T) {
	t.Parallel()

	root := projectFixture(t)
	code, out, problem := runProjectVerb(projectCheck, []string{"--root", root})
	if code != 0 || problem != "" {
		t.Fatalf("project check = code %d, stderr %q", code, problem)
	}
	if out != "project check passed: 4 record(s) in 7 home(s), 1 question(s), 2 ledger goal(s)\n" {
		t.Fatalf("project check's summary line was %q", out)
	}

	fault := filepath.Join(filepath.Dir(root), "metasystem", "docs", "decisions", "0002-shipping.md")
	if err := os.WriteFile(fault,
		[]byte("# A shipping decision\n\n- Kind: decision\n- Id: decision-shipping\n- Status: draft\n- Goals: shipping\n"),
		0o644); err != nil {
		t.Fatalf("write the fault: %v", err)
	}
	code, out, problem = runProjectVerb(projectCheck, []string{"--root", root})
	if code != 1 || problem != "" {
		t.Fatalf("a fault must refuse: code %d, stderr %q", code, problem)
	}
	if out != "metasystem/docs/decisions/0002-shipping.md:6: the goal shipping is not in the ledger\n" {
		t.Fatalf("project check printed %q", out)
	}
}

// A root that is no installation refuses, and says so on standard error rather
// than answering from nowhere.
func TestProjectVerbsRefuseARootThatIsNoInstallation(t *testing.T) {
	t.Parallel()

	nowhere := t.TempDir()
	for _, run := range []struct {
		name string
		verb func([]string, io.Writer, io.Writer) int
		args []string
	}{
		{"check", projectCheck, []string{"--root", nowhere}},
		{"tree", projectTree, []string{"--root", nowhere}},
		{"list", projectList, []string{"--root", nowhere, "design"}},
		{"show", projectShow, []string{"--root", nowhere, "anything"}},
	} {
		code, out, problem := runProjectVerb(run.verb, run.args)
		if code != 1 || out != "" || problem == "" {
			t.Errorf("project %s over a bare directory = code %d, stdout %q, stderr %q",
				run.name, code, out, problem)
		}
	}
}
