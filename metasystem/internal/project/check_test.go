package project

import (
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
	testutil.Expect(t, "the areas an empty project declares", read.DeclaredAreas(),
		map[string]bool{ProjectArea: true})
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
					record("One binary, again", "decision", "decision-one-binary", "draft", "project"))
				return f.state + "docs/decisions/0003-again.md", "- Id:"
			},
			want: "the id decision-one-binary is already declared by metasystem/docs/decisions/0001-one-binary.md",
		},
		{
			name: "a head missing a required key",
			fault: func(f *fixture) (string, string) {
				f.write(f.state+"docs/decisions/0003-headless.md",
					"# A decision with no status\n\n- Kind: decision\n- Id: decision-no-status\n- Areas: project\n")
				return f.state + "docs/decisions/0003-headless.md", "- Kind:"
			},
			want: "the head does not declare Status",
		},
		{
			name: "a required key declared empty",
			fault: func(f *fixture) (string, string) {
				f.write(f.state+"docs/decisions/0003-empty.md",
					"# A decision with an empty id\n\n- Kind: decision\n- Id:\n- Status: draft\n- Areas: project\n")
				return f.state + "docs/decisions/0003-empty.md", "- Kind:"
			},
			want: "the head does not declare Id",
		},
		{
			name: "an unknown kind",
			fault: func(f *fixture) (string, string) {
				f.write(f.state+"docs/decisions/0003-policy.md",
					record("A policy", "policy", "decision-policy", "draft", "project"))
				return f.state + "docs/decisions/0003-policy.md", "- Kind:"
			},
			want: "the kind policy is not one of intent, doctrine, decision, design",
		},
		{
			name: "an unknown status",
			fault: func(f *fixture) (string, string) {
				f.write(f.state+"docs/decisions/0003-pending.md",
					record("A pending decision", "decision", "decision-pending", "pending", "project"))
				return f.state + "docs/decisions/0003-pending.md", "- Status:"
			},
			want: "the status pending is not one of draft, accepted, superseded, done",
		},
		{
			name: "an undeclared area",
			fault: func(f *fixture) (string, string) {
				f.write(f.state+"docs/decisions/0003-shipping.md",
					record("A shipping decision", "decision", "decision-shipping", "draft", "shipping"))
				return f.state + "docs/decisions/0003-shipping.md", "- Areas:"
			},
			want: "the area shipping is declared by no intent index",
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
				f.appendToRegister("|  | 2026-09-22 | A question nobody numbered | project | open |\n")
				return f.state + "memory/questions.md", "A question nobody numbered"
			},
			want: "the row declares no id",
		},
		{
			name: "a register row with an unknown status",
			fault: func(f *fixture) (string, string) {
				f.appendToRegister("| Q-4 | 2026-09-22 | A question in limbo | project | pondering |\n")
				return f.state + "memory/questions.md", "Q-4"
			},
			want: "the status pondering is not open, answered: <reference>, or withdrawn",
		},
		{
			name: "an answer that names nothing",
			fault: func(f *fixture) (string, string) {
				f.appendToRegister("| Q-5 | 2026-09-22 | A question answered by nothing | project | answered: |\n")
				return f.state + "memory/questions.md", "Q-5"
			},
			want: "the status answered: is not open, answered: <reference>, or withdrawn",
		},
		{
			name: "a head line that is not a declaration",
			fault: func(f *fixture) (string, string) {
				f.write(f.state+"docs/decisions/0003-broken.md",
					"# A broken head\n\n- Kind: decision\n- this line declares nothing\n"+
						"- Id: decision-broken\n- Status: draft\n- Areas: project\n")
				return f.state + "docs/decisions/0003-broken.md", "declares nothing"
			},
			want: "the head line is not a `- Key: value` declaration: - this line declares nothing",
		},
		{
			name: "a key declared twice",
			fault: func(f *fixture) (string, string) {
				f.write(f.state+"docs/decisions/0003-twice.md",
					"# A head that says it twice\n\n- Kind: decision\n- Id: decision-twice\n"+
						"- Status: draft\n- Status: accepted\n- Areas: project\n")
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

// appendToDoctrineIndex adds one line to the book's reading order, leaving the
// head and the rest of the order as they were.
func (f *fixture) appendToDoctrineIndex(line string) {
	f.t.Helper()
	f.write(f.state+"docs/doctrine/index.md",
		record("The project's doctrine", "doctrine", "doctrine-index", "accepted", "project")+
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
			"| id | opened | question | areas | status |\n"+
			"| --- | --- | --- | --- | --- |\n"+
			"| Q-1 | 2026-09-22 | Where does an adopted application's intent live? | project | open |\n"+
			"| Q-2 | 2026-09-22 | Who accepts a decision? | billing | answered: decision-one-binary |\n"+
			"| Q-3 | 2026-09-22 | Do designs move when they conclude? | security | withdrawn |\n"+
			row)
}
