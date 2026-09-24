package main

// type-designs is the pass that gives the kit's own history the head a design
// carries. These tests drive it over its own streams, against a checkout that
// holds one file per rule the design states: a goal of each ending, a file
// that names a neighbouring goal, one that contradicts the status the ledger
// implies, one with no title, one that opens with legacy bullets, one already
// typed and one whose head is broken.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// typingFixture is a self-hosted checkout with a ledger of five endings and a
// historical home holding one file per rule.
func typingFixture(t *testing.T) string {
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

	// The ledger: one live goal, one that shipped, one abandoned, one whose
	// own conclusion says something else took the work over, and one that is
	// only ever named by another file's opening.
	write("metasystem/plans/goals/backlog.md", "# backlog\n\n- SyncMode: local\n")
	write("metasystem/plans/goals/live-pane.md",
		"# live-pane\n\n- State: approved\n- Intent: The pane reads a document\n")
	write("metasystem/plans/goals/retired-idea.md",
		"# retired-idea\n\n- State: queued\n- Intent: An idea nobody has started\n")
	write("metasystem/plans/goals/naming-neighbour.md",
		"# naming-neighbour\n\n- State: queued\n- Intent: A design that names its neighbour\n")
	write("metasystem/plans/goals/disk-hygiene.md",
		"# disk-hygiene\n\n- State: queued\n- Intent: The disk stays tidy\n")
	write("metasystem/plans/goals/round-boundary.md",
		"# round-boundary\n\n- State: queued\n- Intent: A brief declares the round boundary\n")
	write("metasystem/records/goals/shipped-ledger.md",
		"# shipped-ledger\n\n- State: done\n- Intent: The ledger syncs\n- Concluded: Landed on both hosts.\n")
	write("metasystem/records/goals/dropped-idea.md",
		"# dropped-idea\n\n- State: abandoned\n- Intent: An idea that was dropped\n")
	write("metasystem/records/goals/old-steward.md",
		"# old-steward\n\n- State: done\n- Intent: A standing role\n"+
			"- Concluded: Superseded by the counselor, which watches the same process.\n")
	write("metasystem/records/goals/other-goal.md",
		"# other-goal\n\n- State: done\n- Intent: Something a neighbour names\n- Concluded: Landed.\n")

	// The register, so the namespace a fresh id is judged against is the
	// project's whole one and not only its pages.
	write("metasystem/memory/questions.md",
		"# Open questions\n\n"+
			"| id | opened | question | goals | status |\n"+
			"| --- | --- | --- | --- | --- |\n"+
			"| Q-HISTORY | 2026-09-22 | What are the untyped designs about? |  | open |\n")

	// The historical home: one file per rule, plus two decoys the glob must
	// not select.
	write("metasystem/plans/live-pane-design.md",
		"# The reading pane\n\nProse about the pane.\n")
	write("metasystem/plans/shipped-ledger-design.md",
		"# The ledger\n\nProse about the ledger.\n")
	write("metasystem/plans/dropped-idea-design.md",
		"# The dropped idea\n\nProse about the idea.\n")
	write("metasystem/plans/old-steward-design.md",
		"# The standing role\n\nProse about the role.\n")
	write("metasystem/plans/no-such-goal-design.md",
		"# A design for no goal in the ledger\n\nProse.\n")
	write("metasystem/plans/naming-neighbour-design.md",
		"# The neighbour\n\nThis design belongs to other-goal, whatever its file name says.\n")
	write("metasystem/plans/retired-idea-design.md",
		"# The retired idea\n\nStatus: superseded. Kept for the record.\n")
	write("metasystem/plans/disk-hygiene-design.md",
		"Status: parked, 2026-01-01, and this file never had a title.\n\n# Disk hygiene\n\nProse.\n")
	write("metasystem/plans/round-boundary-design.md",
		"# The round boundary\n\n- Owner: wido\n- Date: 2026-01-01\n\nProse after the bullets.\n")
	write("metasystem/plans/already-typed-design.md",
		"# Already a record\n\n- Kind: design\n- Id: design-already\n- Status: draft\n\nProse.\n")
	write("metasystem/plans/broken-head-design.md",
		"# A head with no id\n\n- Kind: design\n- Status: draft\n\nProse.\n")
	write("metasystem/plans/live-pane-brief.md", "# A brief, not a design\n\nProse.\n")
	write("metasystem/plans/nested/buried-design.md", "# One directory down\n\nProse.\n")

	return filepath.Join(canonical, "metasystem")
}

// typingMinter is a deterministic identity source, so a test can assert the
// exact bytes a typed file becomes.
func typingMinter() func() (string, error) {
	minted := 0
	return func() (string, error) {
		minted++
		return fmt.Sprintf("01TYPE%020d", minted), nil
	}
}

func typingReadFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

// typingSnapshot is every file of the checkout with the bytes it holds, so a
// test can say that a pass wrote nothing at all.
func typingSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	taken := map[string]string{}
	walk := func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || strings.Contains(path, string(filepath.Separator)+".git"+string(filepath.Separator)) {
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		taken[filepath.ToSlash(rel)] = string(data)
		return nil
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return taken
}

func typingPlanTable(plan string) []string {
	var rows []string
	for _, line := range strings.Split(plan, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "|") {
			rows = append(rows, trimmed)
		}
	}
	return rows
}

// The proposal comes from the file name and the ledger, and from nothing else.
// Every ending has its status, a file the ledger cannot match or the file's own
// opening disagrees with is left unresolved with the reason beside it, a file
// that already declares a head is left alone, and the glob selects the flat
// designs and neither the briefs beside them nor a file one directory down.
func TestTypeDesignsProposesFromTheFileNameAndTheLedger(t *testing.T) {
	t.Parallel()

	root := typingFixture(t)
	checkout := filepath.Dir(root)
	before := typingSnapshot(t, root)

	code, out, problem := runProjectVerb(projectTypeDesigns, []string{"--root", root})
	if code != 0 {
		t.Fatalf("project type-designs = code %d, stderr %q", code, problem)
	}
	if !strings.Contains(problem, "wrote metasystem/plans/designs-typing.md: 6 resolved, 3 unresolved, 1 record, 1 malformed") {
		t.Fatalf("the tally must say what the plan proposes; stderr was %q", problem)
	}

	// An empty cell is rendered as the two spaces a Markdown table puts around
	// nothing, which is what a human editing the plan types over.
	want := []string{
		"| file | proposed goal | proposed status | resolution | note |",
		"| --- | --- | --- | --- | --- |",
		"| already-typed-design.md |  | draft | record | already declares design-already |",
		"| broken-head-design.md |  |  | malformed | the head does not declare Id |",
		"| disk-hygiene-design.md | disk-hygiene | accepted | resolved |  |",
		"| dropped-idea-design.md | dropped-idea | superseded | resolved |  |",
		"| live-pane-design.md | live-pane | accepted | resolved |  |",
		"| naming-neighbour-design.md | naming-neighbour | accepted | unresolved | the file names goal other-goal |",
		"| no-such-goal-design.md |  |  | unresolved | no goal matched |",
		"| old-steward-design.md | old-steward | superseded | resolved |  |",
		"| retired-idea-design.md | retired-idea | accepted | unresolved | the file says superseded |",
		"| round-boundary-design.md | round-boundary | accepted | resolved |  |",
		"| shipped-ledger-design.md | shipped-ledger | done | resolved |  |",
	}
	rows := typingPlanTable(out)
	if len(rows) != len(want) {
		t.Fatalf("the plan printed %d table rows:\n%s\nwant %d", len(rows), strings.Join(rows, "\n"), len(want))
	}
	for index := range want {
		if rows[index] != want[index] {
			t.Fatalf("row %d was\n%s\nwant\n%s", index, rows[index], want[index])
		}
	}
	if onDisk := typingReadFile(t, checkout, "metasystem/plans/designs-typing.md"); onDisk != out {
		t.Fatalf("the plan on disk is not what was printed:\n%q", onDisk)
	}

	// Nothing but the plan was written, and a second run writes the same plan.
	after := typingSnapshot(t, root)
	delete(after, "plans/designs-typing.md")
	for name, text := range before {
		if after[name] != text {
			t.Fatalf("%s changed without --apply", name)
		}
	}
	if len(after) != len(before) {
		t.Fatalf("a proposal added a file: %d before, %d after", len(before), len(after))
	}
	if code, again, _ := runProjectVerb(projectTypeDesigns, []string{"--root", root}); code != 0 || again != out {
		t.Fatalf("the plan is not idempotent: code %d", code)
	}
}

// --apply types every resolved line: the title, a blank line, the head, a
// blank line, and then the body exactly as it was. A file with no title gets
// one from its name and keeps its own first line as the body's first; legacy
// bullets stay in the body, where the parser reads them as the prose they
// always were. Nothing else in the checkout is touched, and a second --apply
// has nothing left to do.
func TestTypeDesignsApplyWritesTheHeadAndKeepsTheBody(t *testing.T) {
	t.Parallel()

	root := typingFixture(t)
	checkout := filepath.Dir(root)
	if code, _, problem := runProjectVerb(projectTypeDesigns, []string{"--root", root}); code != 0 {
		t.Fatalf("propose = code %d, stderr %q", code, problem)
	}
	before := typingSnapshot(t, root)

	var applied, problems strings.Builder
	code := typeDesigns(root, true, typingMinter(), &applied, &problems)
	out := applied.String()
	if code != 0 {
		t.Fatalf("project type-designs --apply = code %d, stderr %q", code, problems.String())
	}
	if !strings.HasPrefix(out, "typed 6 file(s) in metasystem/plans:\n") {
		t.Fatalf("--apply must say what it typed; stdout was %q", out)
	}
	if !strings.Contains(out, "unresolved 3 file(s), for a human to read:\n"+
		"  naming-neighbour-design.md\tthe file names goal other-goal\n") {
		t.Fatalf("--apply must list the unresolved files with their notes; stdout was %q", out)
	}

	// The plain case: a title, a blank line, the head, a blank line, the body.
	if got := typingReadFile(t, checkout, "metasystem/plans/live-pane-design.md"); got !=
		"# The reading pane\n\n- Kind: design\n- Id: 01TYPE00000000000000000003\n"+
			"- Status: accepted\n- Goals: live-pane\n\nProse about the pane.\n" {
		t.Fatalf("the plain write shape was\n%q", got)
	}
	// A file with no title keeps its own first line as the body's first.
	if got := typingReadFile(t, checkout, "metasystem/plans/disk-hygiene-design.md"); got !=
		"# Disk Hygiene Design\n\n- Kind: design\n- Id: 01TYPE00000000000000000001\n"+
			"- Status: accepted\n- Goals: disk-hygiene\n\n"+
			"Status: parked, 2026-01-01, and this file never had a title.\n\n# Disk hygiene\n\nProse.\n" {
		t.Fatalf("the titleless write shape was\n%q", got)
	}
	// Legacy bullets are body: they sit after the head's blank line.
	if got := typingReadFile(t, checkout, "metasystem/plans/round-boundary-design.md"); got !=
		"# The round boundary\n\n- Kind: design\n- Id: 01TYPE00000000000000000005\n"+
			"- Status: accepted\n- Goals: round-boundary\n\n"+
			"- Owner: wido\n- Date: 2026-01-01\n\nProse after the bullets.\n" {
		t.Fatalf("the legacy-bullet write shape was\n%q", got)
	}
	// A concluded goal's design is done only when the goal shipped.
	if got := typingReadFile(t, checkout, "metasystem/plans/shipped-ledger-design.md"); !strings.Contains(got, "- Status: done\n") {
		t.Fatalf("a shipped goal's design was\n%q", got)
	}
	for _, superseded := range []string{"dropped-idea-design.md", "old-steward-design.md"} {
		if got := typingReadFile(t, checkout, "metasystem/plans/"+superseded); !strings.Contains(got, "- Status: superseded\n") {
			t.Fatalf("%s was\n%q", superseded, got)
		}
	}

	// Everything else is exactly as it was: the unresolved files, the file
	// that already declared a head, the broken one, the briefs, and both
	// goal-ledger directories.
	after := typingSnapshot(t, root)
	typed := map[string]bool{
		"plans/live-pane-design.md": true, "plans/shipped-ledger-design.md": true,
		"plans/dropped-idea-design.md": true, "plans/old-steward-design.md": true,
		"plans/disk-hygiene-design.md": true, "plans/round-boundary-design.md": true,
		"plans/designs-typing.md": true,
	}
	for name, text := range before {
		if typed[name] {
			continue
		}
		if after[name] != text {
			t.Fatalf("%s changed, and only the resolved historical files may", name)
		}
	}
	if len(after) != len(before) {
		t.Fatalf("--apply added or removed a file: %d before, %d after", len(before), len(after))
	}

	// The refreshed plan says every typed file is a record now, so a repeat
	// --apply writes nothing.
	plan := typingReadFile(t, checkout, "metasystem/plans/designs-typing.md")
	for _, row := range typingPlanTable(plan) {
		if strings.HasPrefix(row, "| live-pane-design.md ") && !strings.Contains(row, "| record |") {
			t.Fatalf("a typed file is still proposed: %s", row)
		}
	}
	settled := typingSnapshot(t, root)
	if code := typeDesigns(root, true, typingMinter(), &strings.Builder{}, &strings.Builder{}); code != 0 {
		t.Fatalf("a repeat --apply = code %d", code)
	}
	for name, text := range typingSnapshot(t, root) {
		if settled[name] != text {
			t.Fatalf("a repeat --apply rewrote %s", name)
		}
	}
}

// A fresh id is judged against the project's whole namespace before a byte is
// written, register rows included, and a collision leaves every file exactly
// as it was rather than half a collection typed.
func TestTypeDesignsRefusesAnIdCollisionBeforeWriting(t *testing.T) {
	t.Parallel()

	root := typingFixture(t)
	if code, _, problem := runProjectVerb(projectTypeDesigns, []string{"--root", root}); code != 0 {
		t.Fatalf("propose = code %d, stderr %q", code, problem)
	}
	before := typingSnapshot(t, root)

	var stdout, stderr strings.Builder
	code := typeDesigns(root, true, func() (string, error) { return "Q-HISTORY", nil }, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("an id collision = code %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "nothing was written") ||
		!strings.Contains(stderr.String(), "the id Q-HISTORY is already declared by the question register") {
		t.Fatalf("the refusal must name the collision and say nothing was written: %q", stderr.String())
	}
	if stdout.String() != "" {
		t.Fatalf("a refused pass printed %q", stdout.String())
	}
	for name, text := range typingSnapshot(t, root) {
		if before[name] != text {
			t.Fatalf("%s was written although the pass refused", name)
		}
	}
}

// --apply is a read of a reviewed plan, so there is nothing to apply until one
// has been written and read.
func TestTypeDesignsApplyWithoutAPlanRefuses(t *testing.T) {
	t.Parallel()

	root := typingFixture(t)
	var stdout, stderr strings.Builder

	code := typeDesigns(root, true, typingMinter(), &stdout, &stderr)

	if code != 1 || !strings.Contains(stderr.String(), "no reviewed plan at metasystem/plans/designs-typing.md") {
		t.Fatalf("--apply without a plan = code %d, stderr %q", code, stderr.String())
	}
}

// A human's edit is honoured: a line whose goal or status was changed, and a
// line lifted from unresolved to resolved, are what --apply writes.
func TestTypeDesignsHonoursTheReviewedPlan(t *testing.T) {
	t.Parallel()

	root := typingFixture(t)
	checkout := filepath.Dir(root)
	if code, _, problem := runProjectVerb(projectTypeDesigns, []string{"--root", root}); code != 0 {
		t.Fatalf("propose = code %d, stderr %q", code, problem)
	}
	plan := typingReadFile(t, checkout, "metasystem/plans/designs-typing.md")
	plan = strings.Replace(plan,
		"| naming-neighbour-design.md | naming-neighbour | accepted | unresolved | the file names goal other-goal |",
		"| naming-neighbour-design.md | other-goal | done | resolved | a human read it |", 1)
	plan = strings.Replace(plan,
		"| live-pane-design.md | live-pane | accepted | resolved |  |",
		"| live-pane-design.md | live-pane | draft | resolved |  |", 1)
	if err := os.WriteFile(filepath.Join(checkout, "metasystem", "plans", "designs-typing.md"), []byte(plan), 0o644); err != nil {
		t.Fatalf("review the plan: %v", err)
	}

	var stdout, stderr strings.Builder
	if code := typeDesigns(root, true, typingMinter(), &stdout, &stderr); code != 0 {
		t.Fatalf("--apply over a reviewed plan = code %d, stderr %q", code, stderr.String())
	}

	if got := typingReadFile(t, checkout, "metasystem/plans/naming-neighbour-design.md"); !strings.Contains(got, "- Goals: other-goal\n") ||
		!strings.Contains(got, "- Status: done\n") {
		t.Fatalf("the human's goal and status were not honoured:\n%q", got)
	}
	if got := typingReadFile(t, checkout, "metasystem/plans/live-pane-design.md"); !strings.Contains(got, "- Status: draft\n") {
		t.Fatalf("the human's status was not honoured:\n%q", got)
	}
}

// A reviewed line the project cannot accept refuses the whole pass rather than
// typing the rest around it: the human asked for one mapping, and half of it
// is not the one they asked for.
func TestTypeDesignsRefusesAReviewedLineTheLedgerDoesNotCarry(t *testing.T) {
	t.Parallel()

	root := typingFixture(t)
	checkout := filepath.Dir(root)
	if code, _, problem := runProjectVerb(projectTypeDesigns, []string{"--root", root}); code != 0 {
		t.Fatalf("propose = code %d, stderr %q", code, problem)
	}
	plan := typingReadFile(t, checkout, "metasystem/plans/designs-typing.md")
	plan = strings.Replace(plan,
		"| no-such-goal-design.md |  |  | unresolved | no goal matched |",
		"| no-such-goal-design.md | nowhere | accepted | resolved |  |", 1)
	if err := os.WriteFile(filepath.Join(checkout, "metasystem", "plans", "designs-typing.md"), []byte(plan), 0o644); err != nil {
		t.Fatalf("review the plan: %v", err)
	}
	before := typingSnapshot(t, root)

	var stdout, stderr strings.Builder
	code := typeDesigns(root, true, typingMinter(), &stdout, &stderr)

	if code != 1 || !strings.Contains(stderr.String(), "the goal nowhere is not in the ledger") {
		t.Fatalf("a goal the ledger does not carry = code %d, stderr %q", code, stderr.String())
	}
	for name, text := range typingSnapshot(t, root) {
		if before[name] != text {
			t.Fatalf("%s was written although the pass refused", name)
		}
	}
}

// An adopted application never acquires the kit's history, so the verb has
// nothing to propose there and says so rather than typing the application's
// own flat plans.
func TestTypeDesignsRefusesAnAdoptedInstallation(t *testing.T) {
	t.Parallel()

	root := projectAdoptedFixture(t)

	code, out, problem := runProjectVerb(projectTypeDesigns, []string{"--root", root})

	if code != 1 || out != "" {
		t.Fatalf("an adopted installation = code %d, stdout %q", code, out)
	}
	if !strings.Contains(problem, "no historical design home") {
		t.Fatalf("the refusal was %q", problem)
	}
}
