package main

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// The Partner's proposal catalogue and the public command table are one table,
// read two ways.
//
// The catalogue is in the tool server, because that is where the Partner's
// arguments are validated; the descriptor table is here, because this is where
// the public grammar lives. This package is the one that has BOTH, which is why
// the join is here and not in either of them (g1-s62 D2).
//
// It reads the descriptors themselves rather than `commandCatalogue()`, which
// projects a command's name, summary, scope and usage and drops exactly what
// this join needs: the audience, the flag list and the advanced and hidden
// marks on each flag. A join against the projection could agree while the table
// disagreed with both.
//
// The other half of the join is in internal/ui/httpd, where the routes are:
// every catalogue row's route is a ledger act on a goal, and every field its
// arguments become is that route body's own. So the catalogue is held from both
// sides, and neither the public grammar nor the interface can grow an act the
// other has not heard of.

// goalActionDescriptors are the goal object's public actions, by action name.
func goalActionDescriptors() map[string]intentCommand {
	held := map[string]intentCommand{}
	for _, command := range publicIntentCommands() {
		if command.object == "goal" {
			held[command.action] = command
		}
	}
	return held
}

// proposedDescriptors are the public actions the catalogue's acts belong to, by
// each act's own verb: the goal object's actions, and the app object's start
// that runs a goal's candidate (g1-s69 D3), which is the one act of another
// object the grammar carries.
func proposedDescriptors() map[string]intentCommand {
	held := goalActionDescriptors()
	for _, command := range publicIntentCommands() {
		if command.object == uitools.ObjectApp {
			held[command.object+" "+command.action] = command
		}
	}
	return held
}

// flagNamed is one flag of a command under the DESCRIPTOR'S OWN spelling — its
// name and not one of its aliases — with whether the table found it at all.
func flagNamed(command intentCommand, name string) (intentFlag, bool) {
	for _, flag := range command.flags {
		if flag.name == name {
			return flag, true
		}
	}
	return intentFlag{}, false
}

// flagOrAlias is one flag of a command under its name or any of its aliases. It
// is what the refusal list is held against: `--why` is the reason flag's other
// name, and a Partner that sends `why` is sending something the command has.
func flagOrAlias(command intentCommand, name string) (intentFlag, bool) {
	for _, flag := range command.flags {
		if flag.name == name {
			return flag, true
		}
		for _, alias := range flag.aliases {
			if alias == name {
				return flag, true
			}
		}
	}
	return intentFlag{}, false
}

// Every act the Partner may propose is a current public action of the goal
// object, with an audience that includes a human and no hidden mark.
//
// The audience matters because a proposal is a human's press: an action the
// table declares an agent's alone is an action no card should offer, and a
// hidden row is a process entrypoint that public help does not even list.
func TestProposableActsAreCurrentPublicGoalActions(t *testing.T) {
	t.Parallel()
	described := proposedDescriptors()
	for _, act := range uitools.ProposedActs {
		command, known := described[act.Verb()]
		testutil.Require(t, act.Command()+" is a public action of its object", known, true)
		testutil.Expect(t, act.Command()+" is named object then action", command.name, act.Command())
		testutil.Expect(t, act.Command()+" is for a human or for both",
			command.audience == "human" || command.audience == "both", true)
		testutil.Expect(t, act.Command()+" is not a hidden row", command.hidden, false)
	}
}

// Every field the catalogue admits is one of that action's own flags, under the
// descriptor's own spelling, and not a hidden one.
//
// Advanced is admitted deliberately: the descriptors mark `label`, `unlabel`,
// `blocked-by`, `blocks` and `sequence` advanced, and advanced in the terminal's
// help means off the short page, not reserved. Hidden is another matter — a
// hidden flag is plumbing the public grammar does not offer at all — and so is
// an alias: a Partner that learned `blockedBy` where the descriptor says
// `blocked-by` would have its field dropped in silence, so the spelling asserted
// here is the descriptor's own name.
func TestProposableFieldsAreTheirActionsOwnFlags(t *testing.T) {
	t.Parallel()
	described := proposedDescriptors()
	advanced := 0
	for _, act := range uitools.ProposedActs {
		command, known := described[act.Verb()]
		testutil.Require(t, act.Command()+" is in the table", known, true)
		// The app object names its goal with --goal, which is the subject every
		// proposal already carries under goal.
		if act.Object == uitools.ObjectApp {
			_, there := flagNamed(command, "goal")
			testutil.Expect(t, act.Command()+" names its goal with --goal", there, true)
		}
		for _, field := range act.Fields() {
			flag, there := flagNamed(command, field)
			testutil.Expect(t, act.Command()+" has a --"+field+" flag of its own", there, true)
			testutil.Expect(t, act.Command()+"'s --"+field+" is not hidden", flag.hidden, false)
			if flag.advanced {
				advanced++
			}
		}
	}
	testutil.Expect(t, "and an advanced data flag is admitted rather than reserved", advanced > 0, true)

	// The one field a terminal gives positionally. `metasystem goal prioritize
	// G 1|2|3` names the band after the goal, and the descriptor carries
	// `--priority` as the same thing under a flag, which is the spelling the
	// catalogue uses. Both are the descriptor's own.
	prioritize := described[uitools.ActionPrioritize]
	testutil.Expect(t, "goal prioritize takes the goal and the band as words",
		prioritize.maxArgs, 2)
	testutil.Expect(t, "and names the band in its usage line",
		strings.Contains(strings.Join(prioritize.usage, " "), "1|2|3"), true)
}

// No field the catalogue admits is an authority or a plumbing flag, and every
// flag the tool refuses by name is a flag one of the nine actually carries.
//
// The second half is what keeps the refusal list from rotting. A refusal named
// after a flag no proposable command has is a refusal no Partner will ever meet;
// a flag the verbs seat renames is a refusal that would quietly stop firing and
// leave the plumbing admitted.
func TestNoProposableFieldIsAnAuthorityAndEveryRefusalIsARealFlag(t *testing.T) {
	t.Parallel()
	described := proposedDescriptors()
	refused := uitools.RefusedProposalFlags()
	for _, act := range uitools.ProposedActs {
		for _, field := range act.Fields() {
			there := false
			for _, name := range refused {
				there = there || name == field
			}
			// `risk` and `basis` are refused on an edit and admitted on an open,
			// which is the one overlap: the refusal is scoped to the act whose
			// route cannot carry them.
			if field == uitools.FlagRisk || field == uitools.FlagBasis {
				testutil.Expect(t, "goal "+act.Action+"'s --"+field+" is the open's own",
					act.Action, uitools.ActionOpen)
				continue
			}
			testutil.Expect(t, "goal "+act.Action+"'s --"+field+" is not a refused flag", there, false)
		}
	}
	for _, name := range refused {
		carried := []string{}
		for _, act := range uitools.ProposedActs {
			if _, there := flagOrAlias(described[act.Verb()], name); there {
				carried = append(carried, act.Verb())
			}
		}
		testutil.Expect(t, "--"+name+" is a flag one of the nine actually carries",
			len(carried) > 0, true)
	}
}

// Every goal action the tool refuses by name is a public action of the goal
// object with a usage line, and the nine and the refused ones together are every
// public action the object has but its two reads.
//
// The usage line is what the refusal quotes, so a Partner asked for `goal done`
// is told the form to type rather than a form this build invented; and the
// completeness check is what keeps a new goal action from being met with a bare
// "unknown act".
func TestEveryGoalActionIsProposableOrRefusedWithItsUsageLine(t *testing.T) {
	t.Parallel()
	described := goalActionDescriptors()
	readers := uitools.Readers{Kit: uitools.Kit{Commands: commandCatalogue}}
	for _, action := range uitools.ActsNotInTheInterface() {
		command, known := described[action]
		testutil.Require(t, "goal "+action+" is a public action of the goal object", known, true)
		testutil.Require(t, "goal "+action+" has a usage line", len(command.allUsage()) > 0, true)
		result := readers.Answer(uitools.OpPropose,
			uitools.Args{"verb": action, "goal": "g", "explanation": "x"})
		testutil.Expect(t, "goal "+action+" is refused", result.Failed(), true)
		testutil.Expect(t, "goal "+action+" is refused with its own usage line",
			strings.Contains(result.Problem, "at a terminal: "+command.allUsage()[0]), true)
	}

	// Every public action of the goal object is one of the three: one of the
	// nine, one the refusal names, or a read (goal list, goal show), which is
	// not an act at all.
	reads := []string{"list", "show"}
	for action := range described {
		_, proposable := uitools.ProposedActionOf(action)
		named := false
		for _, other := range uitools.ActsNotInTheInterface() {
			named = named || other == action
		}
		read := false
		for _, other := range reads {
			read = read || other == action
		}
		testutil.Expect(t, "goal "+action+" is proposable, refused by name, or a read",
			proposable || named || read, true)
	}
}
