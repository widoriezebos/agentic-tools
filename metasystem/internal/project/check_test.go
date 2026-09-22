package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// A seeded project refuses nothing: every check below is a real fault, not the
// fixture's own shape.
func TestCheckPassesOnASeededProject(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	f.seed()
	read := f.read()

	testutil.Expect(t, "the refusals a whole project carries", problemLines(read.Problems), []string{})
	testutil.Expect(t, "the records read", len(read.Records), 9)
}

// An empty project is empty, not broken: a checkout that has declared no
// memory yet passes, and says so by carrying nothing.
func TestCheckPassesWithNoRecordsAtAll(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	read := f.read()

	testutil.Expect(t, "the refusals an empty project carries", problemLines(read.Problems), []string{})
	testutil.Expect(t, "the records an empty project declares", len(read.Records), 0)
	testutil.Expect(t, "the goals an empty checkout's ledger carries", read.Goals, []Goal(nil))
}

// Every refusal the check owes, one fault at a time, each anchored at the file
// and line a human can act on. Each case seeds a whole project first, so the
// expected line is the only refusal the project carries.
func TestCheckRefusesEachFault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// fault writes the one fault and returns the file it is in and the text
		// on the line that carries it.
		fault func(f *fixture) (rel, anchor string)
		want  string
	}{
		{
			name: "a duplicate id",
			fault: func(f *fixture) (string, string) {
				f.write(f.state+"docs/decisions/0003-again.md",
					record("One binary, again", "decision", "decision-one-binary", "draft", ""))
				return f.state + "docs/decisions/0003-again.md", "- Id:"
			},
			want: "the id decision-one-binary is already declared by metasystem/docs/decisions/0001-one-binary.md",
		},
		{
			name: "a head missing a required key",
			fault: func(f *fixture) (string, string) {
				f.write(f.state+"docs/decisions/0003-headless.md",
					"# A decision with no status\n\n- Kind: decision\n- Id: decision-no-status\n")
				return f.state + "docs/decisions/0003-headless.md", "- Kind:"
			},
			want: "the head does not declare Status",
		},
		{
			name: "a required key declared empty",
			fault: func(f *fixture) (string, string) {
				f.write(f.state+"docs/decisions/0003-empty.md",
					"# A decision with an empty id\n\n- Kind: decision\n- Id:\n- Status: draft\n")
				return f.state + "docs/decisions/0003-empty.md", "- Kind:"
			},
			want: "the head does not declare Id",
		},
		{
			name: "an unknown kind",
			fault: func(f *fixture) (string, string) {
				f.write(f.state+"docs/decisions/0003-policy.md",
					record("A policy", "policy", "decision-policy", "draft", ""))
				return f.state + "docs/decisions/0003-policy.md", "- Kind:"
			},
			want: "the kind policy is not one of intent, doctrine, decision, design",
		},
		{
			name: "an unknown status",
			fault: func(f *fixture) (string, string) {
				f.write(f.state+"docs/decisions/0003-pending.md",
					record("A pending decision", "decision", "decision-pending", "pending", ""))
				return f.state + "docs/decisions/0003-pending.md", "- Status:"
			},
			want: "the status pending is not one of draft, accepted, superseded, done",
		},
		{
			name: "a goal the ledger does not have",
			fault: func(f *fixture) (string, string) {
				f.write(f.state+"docs/decisions/0003-shipping.md",
					record("A shipping decision", "decision", "decision-shipping", "draft", "shipping"))
				return f.state + "docs/decisions/0003-shipping.md", "- Goals:"
			},
			want: "the goal shipping is not in the ledger",
		},
		{
			name: "a register row naming a goal the ledger does not have",
			fault: func(f *fixture) (string, string) {
				f.appendToRegister("| Q-6 | 2026-09-22 | A question about nothing | shipping | open |\n")
				return f.state + "memory/questions.md", "Q-6"
			},
			want: "the goal shipping is not in the ledger",
		},
		{
			name: "a chapter id that names nothing",
			fault: func(f *fixture) (string, string) {
				f.appendToDoctrineIndex("- doctrine-missing — A chapter nobody wrote\n")
				return f.state + "docs/doctrine/index.md", "doctrine-missing"
			},
			want: "the chapter names doctrine-missing, which no record declares",
		},
		{
			name: "a bound document that is not there",
			fault: func(f *fixture) (string, string) {
				f.appendToDoctrineIndex("- doc:" + f.state + "docs/absent.md — Absent\n")
				return f.state + "docs/doctrine/index.md", "docs/absent.md"
			},
			want: "the chapter binds metasystem/docs/absent.md, which is not in this checkout",
		},
		{
			name: "a chapter of another kind",
			fault: func(f *fixture) (string, string) {
				f.appendToDoctrineIndex("- design-ledger — The ledger, in the wrong book\n")
				return f.state + "docs/doctrine/index.md", "design-ledger"
			},
			want: "the chapter names design-ledger, a design record, in the doctrine book",
		},
		{
			name: "a register row with no id",
			fault: func(f *fixture) (string, string) {
				f.appendToRegister("|  | 2026-09-22 | A question nobody numbered |  | open |\n")
				return f.state + "memory/questions.md", "A question nobody numbered"
			},
			want: "the row declares no id",
		},
		{
			name: "a register row with an unknown status",
			fault: func(f *fixture) (string, string) {
				f.appendToRegister("| Q-4 | 2026-09-22 | A question in limbo |  | pondering |\n")
				return f.state + "memory/questions.md", "Q-4"
			},
			want: "the status pondering is not open, answered: <reference>, or withdrawn",
		},
		{
			name: "an answer that names nothing",
			fault: func(f *fixture) (string, string) {
				f.appendToRegister("| Q-5 | 2026-09-22 | A question answered by nothing |  | answered: |\n")
				return f.state + "memory/questions.md", "Q-5"
			},
			want: "the status answered: is not open, answered: <reference>, or withdrawn",
		},
		{
			name: "a register row taking a page's id",
			fault: func(f *fixture) (string, string) {
				f.write(f.state+"docs/decisions/0003-numbered.md",
					record("A decision numbered like a question", "decision", "Q-1", "draft", ""))
				return f.state + "memory/questions.md", "Q-1"
			},
			want: "the id Q-1 is already declared by metasystem/docs/decisions/0003-numbered.md",
		},
		{
			name: "two register rows with one id",
			fault: func(f *fixture) (string, string) {
				f.appendToRegister("| Q-1 | 2026-09-22 | A question numbered twice |  | open |\n")
				return f.state + "memory/questions.md", "numbered twice"
			},
			want: "the id Q-1 is already declared by metasystem/memory/questions.md",
		},
		{
			name: "a head line that is not a declaration",
			fault: func(f *fixture) (string, string) {
				f.write(f.state+"docs/decisions/0003-broken.md",
					"# A broken head\n\n- Kind: decision\n- this line declares nothing\n"+
						"- Id: decision-broken\n- Status: draft\n")
				return f.state + "docs/decisions/0003-broken.md", "declares nothing"
			},
			want: "the head line is not a `- Key: value` declaration: - this line declares nothing",
		},
		{
			name: "a key declared twice",
			fault: func(f *fixture) (string, string) {
				f.write(f.state+"docs/decisions/0003-twice.md",
					"# A head that says it twice\n\n- Kind: decision\n- Id: decision-twice\n"+
						"- Status: draft\n- Status: accepted\n")
				return f.state + "docs/decisions/0003-twice.md", "- Status: accepted"
			},
			want: "the head declares Status twice; it is already declared on line 5",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t, true)
			f.seed()
			rel, anchor := test.fault(f)
			read := f.read()

			testutil.Expect(t, "the one refusal the fault raises", problemLines(read.Problems),
				[]string{rel + ":" + itoa(f.lineOf(rel, anchor)) + ": " + test.want})
		})
	}
}

// A refusal never hides what parsed: the rest of the project still lists and
// still counts while one page is wrong.
func TestARefusalDoesNotHideTheRest(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	f.seed()
	f.write(f.state+"docs/decisions/0003-shipping.md",
		record("A shipping decision", "decision", "decision-shipping", "draft", "shipping"))
	read := f.read()

	testutil.Require(t, "the fault is refused", len(read.Problems), 1)
	testutil.Expect(t, "the designs still list", ids(read.List(KindDesign, ListOptions{})),
		[]string{"design-ledger", "design-reading", "design-interface"})
	testutil.Expect(t, "the faulty record is still read", read.Record("decision-shipping") != nil, true)
}

// A bound chapter names a document this checkout holds, and nothing else: a
// parent step out of it, a symlink pointing out of it, and a directory are all
// refused, whatever is at the other end.
func TestAChapterMustNameAFileInsideTheCheckout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		fault func(f *fixture) (rel, anchor string)
		want  string
	}{
		{
			name: "a chapter that steps out of the checkout",
			fault: func(f *fixture) (string, string) {
				f.outsideDocument()
				f.appendToDoctrineIndex("- doc:../outside.md — Outside\n")
				return f.state + "docs/doctrine/index.md", "../outside.md"
			},
			want: "the chapter binds ../outside.md, which is not in this checkout",
		},
		{
			name: "a chapter bound to a symlink that leaves the checkout",
			fault: func(f *fixture) (string, string) {
				f.link(f.outsideDocument(), f.state+"docs/linked.md")
				f.appendToDoctrineIndex("- doc:" + f.state + "docs/linked.md — Linked\n")
				return f.state + "docs/doctrine/index.md", "docs/linked.md"
			},
			want: "the chapter binds metasystem/docs/linked.md, which is not in this checkout",
		},
		{
			name: "a chapter bound to a directory",
			fault: func(f *fixture) (string, string) {
				f.appendToDoctrineIndex("- doc:" + f.state + "docs/intent — The intent\n")
				return f.state + "docs/doctrine/index.md", "docs/intent —"
			},
			want: "the chapter binds metasystem/docs/intent, which is not in this checkout",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t, true)
			f.seed()
			rel, anchor := test.fault(f)
			read := f.read()

			testutil.Expect(t, "the one refusal the binding raises", problemLines(read.Problems),
				[]string{rel + ":" + itoa(f.lineOf(rel, anchor)) + ": " + test.want})
		})
	}
}

// The register is read from inside its own home. A questions.md that is a
// symlink to a file outside it is refused where it stands rather than followed,
// and the rows at the other end are no part of this project.
func TestARegisterThatLeavesItsHomeIsRefused(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	f.seed()
	target := filepath.Join(filepath.Dir(f.checkout), "questions.md")
	mustNot(t, os.WriteFile(target, []byte("# Elsewhere\n\n"+
		"| id | opened | question | goals | status |\n"+
		"| --- | --- | --- | --- | --- |\n"+
		"| Q-9 | 2026-09-22 | A question from outside |  | open |\n"), 0o644),
		"write the register outside the checkout")
	register := f.state + "memory/questions.md"
	mustNot(t, os.Remove(filepath.Join(f.checkout, filepath.FromSlash(register))), "remove the register")
	f.link(target, register)
	read := f.read()

	testutil.Expect(t, "the questions a register outside its home declares", len(read.Questions), 0)
	testutil.Require(t, "the refusals it raises", len(read.Problems), 1)
	testutil.Expect(t, "where the refusal is anchored", read.Problems[0].Path, register)
	testutil.Expect(t, "what the refusal says",
		strings.HasPrefix(read.Problems[0].Message, "cannot be read:"), true)
}

// outsideDocument writes one Markdown file beside the checkout, which is where
// a test puts what the containment must not bind.
func (f *fixture) outsideDocument() string {
	f.t.Helper()
	path := filepath.Join(filepath.Dir(f.checkout), "outside.md")
	mustNot(f.t, os.WriteFile(path, []byte("# Outside\n"), 0o644), "write the outside document")
	return path
}

// link puts a symlink in the checkout, at a checkout-relative name.
func (f *fixture) link(target, rel string) {
	f.t.Helper()
	name := filepath.Join(f.checkout, filepath.FromSlash(rel))
	mustNot(f.t, os.MkdirAll(filepath.Dir(name), 0o755), "create the directory for "+rel)
	mustNot(f.t, os.Symlink(target, name), "link "+rel)
}

// appendToDoctrineIndex adds one line to the book's reading order, leaving the
// head and the rest of the order as they were.
func (f *fixture) appendToDoctrineIndex(line string) {
	f.t.Helper()
	f.write(f.state+"docs/doctrine/index.md",
		record("The project's doctrine", "doctrine", "doctrine-index", "accepted", "")+
			"\n## Chapters\n"+
			"- doctrine-events — Events are the source of truth\n"+
			"- doctrine-budgets — Every run is budgeted\n"+
			"- doc:"+f.state+"docs/architecture.md — The engine\n"+
			line)
}

// appendToRegister adds one row to the questions table, as a register is
// maintained: rows are appended.
func (f *fixture) appendToRegister(row string) {
	f.t.Helper()
	f.write(f.state+"memory/questions.md",
		"# Open questions\n\n"+
			"| id | opened | question | goals | status |\n"+
			"| --- | --- | --- | --- | --- |\n"+
			"| Q-1 | 2026-09-22 | Where does an adopted application's intent live? |  | open |\n"+
			"| Q-2 | 2026-09-22 | Who accepts a decision? | ledger-sync | answered: decision-one-binary |\n"+
			"| Q-3 | 2026-09-22 | Do designs move when they conclude? | reading-pane | withdrawn |\n"+
			row)
}
