package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	resolver "github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The manifest is what stops a Partner describing an interface that is not
// there. Everything below turns on the three fields being separate: what a
// section is for, whether this build has it, and what the Partner may do.

func TestTheJoinCarriesBothHalves(t *testing.T) {
	t.Parallel()
	joined := composed(t)

	testutil.Expect(t, "the built half is carried", len(joined.Sections), 2)
	testutil.Expect(t, "with its lanes", len(joined.Lanes), 2)
	testutil.Expect(t, "and its terms", len(joined.Terms), 2)
	testutil.Expect(t, "the acts are the server's", len(joined.Acts), 1)
	testutil.Expect(t, "the runtimes are this build's", joined.Runtimes[0].Name, "claude")
	testutil.Expect(t, "and nothing went unread", joined.Problems, []string(nil))
}

// An unprojected section is said to be absent, with the gate that brings it.
// This is the finding the whole manifest exists for: a Partner asked where to
// inspect the fleet must not describe a page that is a placeholder.
func TestAnUnprojectedSectionIsSaidToBeAbsent(t *testing.T) {
	t.Parallel()
	lines, known := composed(t).Lines(PartSections)
	testutil.Require(t, "sections is a part", known, true)
	whole := strings.Join(lines, "\n")

	testutil.Expect(t, "the purpose is its own field",
		strings.Contains(whole, "- For: The machines and seats."), true)
	testutil.Expect(t, "what the page shows is another",
		strings.Contains(whole, "- Shows: A row per machine."), true)
	testutil.Expect(t, "and availability is a third",
		strings.Contains(whole, "- In this build: This build does not project Fleet. Arrives with g1-s13."), true)
	testutil.Expect(t, "a projected section says so too",
		strings.Contains(whole, "- In this build: This build projects Backlog."), true)
}

// The summary is what a reader is given when it names no part, and the first
// thing it says is what the Partner may and may not do.
func TestTheSummarySaysThePartnerCannotWrite(t *testing.T) {
	t.Parallel()
	lines, known := composed(t).Lines("")
	testutil.Require(t, "the empty part is the summary", known, true)
	whole := strings.Join(lines, "\n")

	testutil.Expect(t, "what it may read",
		strings.Contains(whole, "read the page and the records"), true)
	testutil.Expect(t, "what it may not do",
		strings.Contains(whole, "writing a file and the network"), true)
	testutil.Expect(t, "and that it cannot write, in so many words",
		strings.Contains(whole, "It cannot write."), true)
	testutil.Expect(t, "the sections this build projects are named",
		strings.Contains(whole, "Sections this build projects (1): Backlog"), true)
	testutil.Expect(t, "and the ones it does not",
		strings.Contains(whole, "Sections this build does not project (1): Fleet"), true)
}

// The acts carry the hand each needs, and say the Partner performs none of
// them: a human asking "can you approve this?" is owed both halves.
func TestTheActsSayWhatEachOneNeedsAndThatThePartnerDoesNone(t *testing.T) {
	t.Parallel()
	lines, known := composed(t).Lines(PartActs)
	testutil.Require(t, "acts is a part", known, true)
	whole := strings.Join(lines, "\n")
	testutil.Expect(t, "the requirement is carried",
		strings.Contains(whole, "Requires a signed-in human."), true)
	testutil.Expect(t, "and the Partner performs none",
		strings.Contains(whole, "The Project Partner performs none of these."), true)
}

// The settings are read through the configuration reader, with the default
// beside what this seat resolves — and a family says it is a pattern rather
// than reporting one seat's value for all of them.
func TestTheSettingsCarryDefaultAndEffective(t *testing.T) {
	t.Parallel()
	lines, known := composed(t).Lines(PartSettings)
	testutil.Require(t, "settings is a part", known, true)
	whole := strings.Join(lines, "\n")
	testutil.Expect(t, "the default is named",
		strings.Contains(whole, "Default: \"127.0.0.1:7878\"."), true)
	testutil.Expect(t, "and what this seat resolves it to",
		strings.Contains(whole, "On this seat: \"127.0.0.1:9999\"."), true)
	testutil.Expect(t, "an unset key says so rather than showing an empty string",
		strings.Contains(whole, "On this seat: unset."), true)
	testutil.Expect(t, "and a family is named as a pattern",
		strings.Contains(whole, "The last segment is a runtime's name"), true)
}

// Where a kind of record lives is resolved for THIS checkout, because the same
// bundle serves the template and an adopted application and their homes are
// not the same paths.
func TestTheRecordKindsCarryThisCheckoutsOwnHomes(t *testing.T) {
	t.Parallel()
	lines, known := composed(t).Lines(PartRecords)
	testutil.Require(t, "records is a part", known, true)
	whole := strings.Join(lines, "\n")
	testutil.Expect(t, "the intent's home is resolved",
		strings.Contains(whole, "intent: "), true)
	testutil.Expect(t, "from the resolver rather than written down",
		strings.Contains(whole, "docs/intent"), true)
	testutil.Expect(t, "a book says its index is a record",
		strings.Contains(whole, "index.md is itself a record"), true)
	testutil.Expect(t, "the questions register says it is rows",
		strings.Contains(whole, "a table of rows rather than a page each"), true)
	testutil.Expect(t, "and the help register's own words explain the kind",
		strings.Contains(whole, "Why this project exists."), true)
}

// A half this build cannot read is a problem the manifest states. A reader
// asking what this interface is made of must be able to tell "there is none"
// from "I could not look".
func TestAMissingBuiltHalfIsSaidRatherThanLeftEmpty(t *testing.T) {
	t.Parallel()
	joined := Compose(Sources{ConfPath: confFile(t), Roots: rootsFor(t)})
	testutil.Require(t, "one problem", len(joined.Problems), 1)
	testutil.Expect(t, "naming what could not be read",
		strings.Contains(joined.Problems[0], "the interface's own half of this manifest could not be read"), true)
	lines, _ := joined.Lines(PartSections)
	testutil.Expect(t, "and the sections say so rather than being empty",
		strings.Contains(strings.Join(lines, "\n"), "could not read the interface's own half"), true)
}

// A part this manifest does not have is a refusal that names the parts, not an
// empty answer a reader would take for "there are none".
func TestAnUnknownPartIsRefusedByName(t *testing.T) {
	t.Parallel()
	joined := composed(t)
	_, known := joined.Lines("colours")
	testutil.Expect(t, "it is not a part", known, false)
	answered := 0
	for _, part := range Parts {
		if _, said := joined.Lines(part); !said {
			t.Fatalf("%s is listed as a part and answers nothing", part)
		}
		answered++
	}
	testutil.Expect(t, "and every part this manifest lists does answer", answered, len(Parts))
}

/* ---------------------------------------------------------------- reading -- */

func composed(t *testing.T) Manifest {
	t.Helper()
	return Compose(Sources{
		Dist:     builtHalf(),
		ConfPath: confFile(t),
		Roots:    rootsFor(t),
		Acts: []Act{{ID: "approve-goal", Title: "Approve a goal",
			Does: "Authorises a goal.", Requires: "a signed-in human"}},
		Runtimes: []Runtime{{Name: "claude", Model: "claude-opus-5-5", Command: "claude-agent-acp"}},
		Partner: Partner{
			Configured: true,
			Reads:      "the page and the records",
			Refused:    "writing a file and the network",
		},
	})
}

func rootsFor(t *testing.T) resolver.Roots {
	t.Helper()
	root := t.TempDir()
	return resolver.Roots{Checkout: root, Installation: root, StateRoot: root}
}

// confFile is a seat that has moved one setting off its default, so the
// register can be read for both halves of the same row.
func confFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "metasystem.conf")
	testutil.Require(t, "wrote the configuration",
		os.WriteFile(path, []byte("ui.listen=127.0.0.1:9999\n"), 0o600), nil)
	return path
}

// builtHalf is a bundle's interface.json, with one projected section and one
// that is not.
func builtHalf() fstest.MapFS {
	return fstest.MapFS{BuiltPath: {Data: []byte(`{
  "schemaVersion": 1,
  "sections": [
    {"id": "backlog", "title": "Backlog", "path": "/backlog",
     "purpose": "The goals, as a board.", "shows": "The goals lane by lane.",
     "projected": true, "availability": "This build projects Backlog."},
    {"id": "fleet", "title": "Fleet", "path": "/fleet",
     "purpose": "The machines and seats.", "shows": "A row per machine.",
     "projected": false, "availability": "This build does not project Fleet. Arrives with g1-s13."}
  ],
  "lanes": [
    {"id": "ready", "title": "Ready for Work", "purpose": "Approved goals.", "shown": true},
    {"id": "draft", "title": "Draft", "purpose": "Not through intake.", "shown": false}
  ],
  "terms": [
    {"id": "intent", "term": "Intent", "text": "Why this project exists."},
    {"id": "questions", "term": "Open questions", "text": "What nobody has answered yet."}
  ],
  "questions": [
    {"subject": "goal", "questions": [{"text": "Why is it here?", "scope": "this goal's record"}]}
  ]
}
`)}}
}
