package project

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

const validCovenant = `{
  "schemaVersion": 1,
  "identity": {
    "name": "taskrun",
    "entryPoint": "java -jar target/taskrun.jar",
    "sourcePaths": ["src/main/**"]
  },
  "requirements": [
    {"id": "10", "ref": "spec.md requirement 10", "proof": "gap_handled"}
  ],
  "battery": {"command": "bash gate.sh", "metric": "self-assessment", "direction": "max", "threshold": ">=26"},
  "budgets": [{"metric": "dependency_count", "bound": 0, "direction": "min"}],
  "guards": [{"name": "no-runtime-deps", "command": "bash guard-deps.sh", "cadence": 1, "floor": 1}],
  "guardrails": ["gate.sh"]
}
`

// plantSelfHosted fills the template's own layout with one of everything the
// catalogue names, and a few things it must not list.
func plantSelfHosted(t *testing.T) Roots {
	t.Helper()
	roots := selfHostedFixture(t)
	installation := roots.Installation

	plant(t, installation, "covenant.json", validCovenant)
	plant(t, installation, "docs/project-rules.md", "# Project Rules\n\n- Purpose: A tool for the test.\n")
	plant(t, installation, "docs/app-doctrine.md", "# The doctrine\n")
	plant(t, installation, "docs/architecture.md", "# The map\n")
	plant(t, installation, "docs/concepts.md", "# The concepts\n")
	plant(t, installation, "docs/covenant-evidence.md", "# The evidence\n")
	plant(t, installation, "docs/paper/index.md", "# The paper\n")
	plant(t, installation, "docs/paper/01-first.md", "# First\n")
	plant(t, installation, "docs/paper/02-second.md", "# Second\n")
	plant(t, installation, "docs/paper/notes.md", "# Notes that are not a chapter\n")
	plant(t, installation, "docs/design/alpha.md", "# Alpha\n")
	plant(t, installation, "docs/design/beta.md", "# Beta\n")
	plant(t, installation, "docs/design/readme.txt", "not markdown\n")
	plant(t, installation, "plans/one-design.md", "# One\n")
	plant(t, installation, "plans/user-interface/two-design.md", "# Two\n")
	plant(t, installation, "plans/goals/goal-design.md", "# A goal, which is not a live design\n")
	plant(t, installation, "plans/goals-drafts/draft-design.md", "# A draft, which is not a live design\n")
	plant(t, installation, "plans/notes.md", "# Notes, which are not a design\n")
	plant(t, installation, "memory/known-issues.md", "# Known issues\n")
	plant(t, installation, "memory/proposal-drafts.md", "# Proposal drafts\n")

	plant(t, roots.Checkout, "plans/host-design.md", "# The host's design\n")
	plant(t, roots.Checkout, "development/project-rules-local.md", "# Local rules\n")
	return roots
}

// The catalogue, row by row, in the template's own layout.
func TestThreadListsTheCatalogueSelfHosted(t *testing.T) {
	t.Parallel()

	roots := plantSelfHosted(t)

	thread, err := ReadThread(roots, readAt)

	testutil.Require(t, "read the thread", err, nil)
	testutil.Expect(t, "the schema version", thread.SchemaVersion, SchemaVersion)
	testutil.Expect(t, "read at", thread.ReadAt, "2026-09-21T10:11:12Z")
	testutil.Expect(t, "the six subsections", subsectionIDs(thread), []string{
		"intent", "architecture", "designs", "constraints", "open-questions", "sittings",
	})

	intent := sectionOf(thread, "intent")
	testutil.Expect(t, "intent's state", intent.State, stateRecorded)
	testutil.Require(t, "intent carries a covenant", intent.Covenant != nil, true)
	testutil.Expect(t, "the covenant's path", intent.Covenant.Path, filepath.Join(roots.Installation, "covenant.json"))
	testutil.Expect(t, "the covenant's identity", intent.Covenant.Identity.Name, "taskrun")
	testutil.Require(t, "intent carries a purpose", intent.Purpose != nil, true)
	testutil.Expect(t, "the purpose", intent.Purpose.Text, "A tool for the test.")

	architecture := sectionOf(thread, "architecture")
	testutil.Expect(t, "architecture's state", architecture.State, stateRecorded)
	testutil.Expect(t, "architecture's documents", entriesOf(architecture), []string{
		"metasystem/docs/app-doctrine.md", "metasystem/docs/architecture.md", "metasystem/docs/concepts.md",
	})

	designs := sectionOf(thread, "designs")
	testutil.Expect(t, "designs' groups", groupTitles(designs), []string{
		"The paper", "Design documents", "Live designs in metasystem/plans", "Live designs in plans",
	})
	testutil.Expect(t, "the paper, in reading order", documentIDs(groupOf(designs, "paper")), []string{
		"metasystem/docs/paper/index.md", "metasystem/docs/paper/01-first.md", "metasystem/docs/paper/02-second.md",
	})
	testutil.Expect(t, "the design documents", documentIDs(groupOf(designs, "design-documents")), []string{
		"metasystem/docs/design/alpha.md", "metasystem/docs/design/beta.md",
	})
	testutil.Expect(t, "the live designs beneath the state root",
		documentIDs(groupOf(designs, "live-designs-metasystem-plans")), []string{
			"metasystem/plans/one-design.md", "metasystem/plans/user-interface/two-design.md",
		})
	testutil.Expect(t, "the live designs beneath the checkout",
		documentIDs(groupOf(designs, "live-designs-plans")), []string{"plans/host-design.md"})

	constraints := sectionOf(thread, "constraints")
	testutil.Expect(t, "constraints' state", constraints.State, stateRecorded)
	testutil.Require(t, "constraints carries a covenant", constraints.Covenant != nil, true)
	testutil.Expect(t, "the battery", constraints.Covenant.Battery.Command, "bash gate.sh")
	testutil.Expect(t, "the guardrails", constraints.Covenant.Guardrails, []string{"gate.sh"})
	testutil.Expect(t, "constraints' documents", entriesOf(constraints), []string{
		"metasystem/docs/covenant-evidence.md", "metasystem/docs/project-rules.md",
		"development/project-rules-local.md",
	})

	questions := sectionOf(thread, "open-questions")
	testutil.Expect(t, "open questions are never recorded", questions.State, stateNotRecorded)
	testutil.Expect(t, "the nearest registers", documentIDs(groupOf(questions, "registers")), []string{
		"metasystem/memory/known-issues.md", "metasystem/memory/proposal-drafts.md",
	})
	testutil.Expect(t, "the registers' group title", groupOf(questions, "registers").Title, "Nearest living registers")

	sittings := sectionOf(thread, "sittings")
	testutil.Expect(t, "sittings are not projected", sittings.State, stateNotProjected)
	testutil.Expect(t, "sittings list nothing", sittings.Groups, []Group{})

	// What was found is not also reported as missing: a subsection names only
	// the sources it looked for and did not get.
	testutil.Expect(t, "designs looked for nothing it found", designs.LookedFor, []string{})
	testutil.Expect(t, "architecture looked for nothing it found", architecture.LookedFor, []string{})
}

// An adopted workspace sees its own material, and none of the kit's: the
// paper, the design documents, the architecture map and the concepts are the
// machinery's there, and Settings' at gate 7.
func TestThreadLeavesTheKitsDocumentsOutOfAnAdoptedWorkspace(t *testing.T) {
	t.Parallel()

	roots := adoptedFixture(t)
	plant(t, roots.Checkout, "covenant.json", validCovenant)
	plant(t, roots.Checkout, "docs/app-doctrine.md", "# The application's doctrine\n")
	plant(t, roots.Checkout, "docs/architecture.md", "# The kit's map\n")
	plant(t, roots.Checkout, "docs/concepts.md", "# The kit's concepts\n")
	plant(t, roots.Checkout, "docs/design/alpha.md", "# The kit's design\n")
	plant(t, roots.Checkout, "docs/paper/index.md", "# The kit's paper\n")
	plant(t, roots.Checkout, "docs/project-rules.md", "# Rules\n\n- Purpose: The application.\n")
	plant(t, roots.Checkout, "development/project-rules-local.md", "# The template's own local rules\n")
	plant(t, roots.Checkout, "plans/a-design.md", "# A design\n")

	thread, err := ReadThread(roots, readAt)

	testutil.Require(t, "read the thread", err, nil)
	testutil.Expect(t, "architecture's documents", entriesOf(sectionOf(thread, "architecture")),
		[]string{"docs/app-doctrine.md"})
	testutil.Expect(t, "designs' groups", groupTitles(sectionOf(thread, "designs")),
		[]string{"Live designs in plans"})
	testutil.Expect(t, "designs' documents", entriesOf(sectionOf(thread, "designs")),
		[]string{"plans/a-design.md"})
	testutil.Expect(t, "constraints' documents", entriesOf(sectionOf(thread, "constraints")),
		[]string{"docs/project-rules.md"})
	testutil.Expect(t, "the purpose", sectionOf(thread, "intent").Purpose.Text, "The application.")
}

// A subsection whose every source is absent says so, and names what it looked
// for with absolute paths, so the statement is one a human can act on.
func TestThreadNamesWhatItLookedForWhenNothingIsThere(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)

	thread, err := ReadThread(roots, readAt)

	testutil.Require(t, "read the thread", err, nil)
	intent := sectionOf(thread, "intent")
	testutil.Expect(t, "intent's state", intent.State, stateNotRecorded)
	testutil.Expect(t, "intent carries no covenant", intent.Covenant, (*Covenant)(nil))
	testutil.Expect(t, "intent carries no purpose", intent.Purpose, (*Purpose)(nil))
	testutil.Expect(t, "intent looked for", intent.LookedFor, []string{
		filepath.Join(roots.Installation, "covenant.json"),
		filepath.Join(roots.Checkout, "covenant.json"),
		filepath.Join(roots.Installation, "docs", "project-rules.md"),
	})
	architecture := sectionOf(thread, "architecture")
	testutil.Expect(t, "architecture's state", architecture.State, stateNotRecorded)
	testutil.Expect(t, "architecture looked for the doctrine",
		contains(architecture.LookedFor, filepath.Join(roots.Installation, "docs", "app-doctrine.md")), true)
	testutil.Expect(t, "architecture lists nothing", architecture.Groups, []Group{})
}

// The template's own placeholder is not a purpose, and neither is a rules file
// without the line at all.
func TestThreadRefusesTheTemplatesPlaceholderPurpose(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	plant(t, roots.Installation, "docs/project-rules.md", "# Project Rules\n\n- Purpose: `<one paragraph>`\n")

	thread, err := ReadThread(roots, readAt)

	testutil.Require(t, "read the thread", err, nil)
	intent := sectionOf(thread, "intent")
	testutil.Expect(t, "intent carries no purpose", intent.Purpose, (*Purpose)(nil))
	testutil.Expect(t, "intent looked for the rules file",
		contains(intent.LookedFor, filepath.Join(roots.Installation, "docs", "project-rules.md")), true)
}

// A covenant that is there but cannot be read is shown where it was found,
// with the refusal, never hidden.
func TestThreadShowsACovenantThatCannotBeRead(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	plant(t, roots.Installation, "covenant.json", "{ this is not JSON")

	thread, err := ReadThread(roots, readAt)

	testutil.Require(t, "read the thread", err, nil)
	intent := sectionOf(thread, "intent")
	testutil.Require(t, "intent carries a covenant", intent.Covenant != nil, true)
	testutil.Expect(t, "the covenant's path", intent.Covenant.Path, filepath.Join(roots.Installation, "covenant.json"))
	testutil.Expect(t, "the covenant's refusal", strings.Contains(intent.Covenant.Error, "not valid JSON"), true)
	testutil.Expect(t, "no identity is invented", intent.Covenant.Identity, (*Identity)(nil))
}

// A listed document that cannot be read says why, and offers no link. An
// oversized one carries the size the filesystem reported at that moment, which
// the route decides again from its own read.
func TestThreadListsWhatItCannotRead(t *testing.T) {
	t.Parallel()

	roots := plantSelfHosted(t)
	elsewhere := outside(t)
	plant(t, elsewhere, "secret.md", "# Outside\n")
	plantBytes(t, roots.Installation, "plans/big-design.md", bytes.Repeat([]byte("a"), maxDocumentBytes+1))
	plantBytes(t, roots.Installation, "plans/binary-design.md", []byte{0xff, 0xfe, 0x00})
	link(t, filepath.Join(elsewhere, "secret.md"), filepath.Join(roots.Installation, "plans", "escape-design.md"))

	thread, err := ReadThread(roots, readAt)

	testutil.Require(t, "read the thread", err, nil)
	designs := sectionOf(thread, "designs")
	big := entryOf(designs, "metasystem/plans/big-design.md")
	testutil.Expect(t, "the oversized state", big.State, stateTooLarge)
	testutil.Expect(t, "the oversized size", big.Bytes, int64(maxDocumentBytes+1))
	binary := entryOf(designs, "metasystem/plans/binary-design.md")
	testutil.Expect(t, "the unreadable state", binary.State, stateUnreadable)
	testutil.Expect(t, "the unreadable reason", binary.Reason, "the file is not valid UTF-8 text")
	escape := entryOf(designs, "metasystem/plans/escape-design.md")
	testutil.Expect(t, "the escaping link's state", escape.State, stateUnreadable)
	testutil.Expect(t, "the escaping link says why", escape.Reason != "", true)
}

// Every listed document is named the way the resolver names it, and the route
// answers that same id.
func TestThreadListsDocumentsTheRouteCanOpen(t *testing.T) {
	t.Parallel()

	roots := plantSelfHosted(t)

	thread, err := ReadThread(roots, readAt)
	testutil.Require(t, "read the thread", err, nil)

	opened := 0
	for _, section := range thread.Subsections {
		for _, group := range section.Groups {
			for _, entry := range group.Documents {
				if entry.State != stateReadable {
					continue
				}
				document, readErr := Read(roots, entry.ID, readAt)
				if readErr != nil {
					t.Fatalf("the route refuses a listed document %q: %v", entry.ID, readErr)
				}
				if document.Title != entry.Title {
					t.Fatalf("the listing and the route disagree about %q: %q and %q", entry.ID, entry.Title, document.Title)
				}
				opened++
			}
		}
	}
	testutil.Expect(t, "every listed document opens", opened > 10, true)
	testutil.Expect(t, "every entry names its kind", everyEntryIsADocument(thread), true)
}

// The ownership answer is shown beside every path, as the engine gives it.
func TestThreadShowsTheOwnershipAnswer(t *testing.T) {
	t.Parallel()

	roots := plantSelfHosted(t)

	thread, err := ReadThread(roots, readAt)

	testutil.Require(t, "read the thread", err, nil)
	testutil.Expect(t, "the kit's own design document",
		entryOf(sectionOf(thread, "designs"), "metasystem/docs/design/alpha.md").Owner, "metasystem-generic")
	testutil.Expect(t, "the host's own design document",
		entryOf(sectionOf(thread, "designs"), "plans/host-design.md").Owner, "app-owned")
}

// An installation that no layout resolves is a failure the route reports, not
// an empty thread that reads as an absence of material.
func TestThreadRefusesAnInstallationWithNoLayout(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	roots.Installation = filepath.Join(roots.Checkout, "not-an-installation")

	_, err := ReadThread(roots, readAt)

	testutil.Expect(t, "the failure", err != nil, true)
}

func subsectionIDs(thread Thread) []string {
	ids := []string{}
	for _, section := range thread.Subsections {
		ids = append(ids, section.ID)
	}
	return ids
}

func documentIDs(group Group) []string {
	ids := []string{}
	for _, entry := range group.Documents {
		ids = append(ids, entry.ID)
	}
	return ids
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func everyEntryIsADocument(thread Thread) bool {
	for _, section := range thread.Subsections {
		for _, group := range section.Groups {
			for _, entry := range group.Documents {
				if entry.Kind != "document" {
					return false
				}
			}
		}
	}
	return true
}
