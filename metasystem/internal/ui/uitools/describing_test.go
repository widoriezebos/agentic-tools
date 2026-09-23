package uitools_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/manifest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// The two tools that answer about the interface and about the kit.
//
// Both answer from owners: one from the manifest this build composes, one from
// the kit's own glossary, catalogue, register and routes. Neither restates a
// fact, and every assertion below names something that can only be in the
// answer because its owner supplied it.

func TestTheCatalogueOffersTheInterfaceAndKitTools(t *testing.T) {
	t.Parallel()
	named := map[string]string{}
	for _, tool := range uitools.Catalogue() {
		named[tool.Name] = tool.Description
	}
	testutil.Expect(t, "interface is offered", strings.Contains(named[uitools.OpInterface], "What this interface is made of"), true)
	testutil.Expect(t, "and says it is bounded and parted",
		strings.Contains(named[uitools.OpInterface], "name a part and page within it"), true)
	testutil.Expect(t, "kit is offered", strings.Contains(named[uitools.OpKit], "What the metasystem itself means"), true)
	testutil.Expect(t, "and keeps rules apart from observations",
		strings.Contains(named[uitools.OpKit], "comes from the board, goal and search tools"), true)
	testutil.Expect(t, "the permission rule admits interface", uitools.Names(uitools.OpInterface), true)
	testutil.Expect(t, "and kit", uitools.Names(uitools.OpKit), true)
}

// interface() answers a named part, names the reading it was of, and refuses a
// part it does not have by naming the parts it does.
func TestInterfaceAnswersOnePartAtATime(t *testing.T) {
	t.Parallel()
	readers := uitools.Readers{Interface: func() (manifest.Manifest, error) { return described(), nil }}

	summary := readers.Answer(uitools.OpInterface, uitools.Args{}).Text()
	testutil.Expect(t, "the summary names its source",
		strings.Contains(summary, "Source: this build's own interface"), true)
	testutil.Expect(t, "and says the Partner cannot write",
		strings.Contains(summary, "It cannot write."), true)

	sections := readers.Answer(uitools.OpInterface, uitools.Args{"part": "sections"}).Text()
	testutil.Expect(t, "a part is answered",
		strings.Contains(sections, "In this build: This build does not project Fleet."), true)

	unknown := readers.Answer(uitools.OpInterface, uitools.Args{"part": "colours"})
	testutil.Expect(t, "an unknown part is a failed read", unknown.Failed(), true)
	testutil.Expect(t, "naming the parts it has",
		strings.Contains(unknown.Text(), strings.Join(manifest.Parts, ", ")), true)

	absent := uitools.Readers{}.Answer(uitools.OpInterface, uitools.Args{})
	testutil.Expect(t, "a build that cannot describe itself says so", absent.Failed(), true)

	broken := uitools.Readers{Interface: func() (manifest.Manifest, error) {
		return manifest.Manifest{}, errors.New("the bundle is unreadable")
	}}.Answer(uitools.OpInterface, uitools.Args{})
	testutil.Expect(t, "and a composition that failed says why",
		strings.Contains(broken.Text(), "the bundle is unreadable"), true)
}

// kit() answers from the four owners, names them, and finds the glossary
// through the contract's own pointer rather than a path written here.
func TestKitAnswersFromTheKitsOwnOwners(t *testing.T) {
	t.Parallel()
	readers := uitools.Readers{
		Kit: uitools.Kit{Root: kitRoot(t), Commands: fixtureCatalogue},
	}

	index := readers.Answer(uitools.OpKit, uitools.Args{}).Text()
	testutil.Expect(t, "the index names the glossary it found",
		strings.Contains(index, "docs/glossary.md (named by AGENTS.md"), true)
	testutil.Expect(t, "and the rulings register", strings.Contains(index, "memory/rulings.md"), true)
	testutil.Expect(t, "and the routes", strings.Contains(index, "wow.md"), true)
	testutil.Expect(t, "and the engine's catalogue",
		strings.Contains(index, "the engine's command catalogue"), true)
	testutil.Expect(t, "and it keeps rules apart from the ledger's own answers",
		strings.Contains(index, "Observations of the ledger are not here"), true)

	term := readers.Answer(uitools.OpKit, uitools.Args{"topic": "lease epoch"}).Text()
	testutil.Expect(t, "a term is answered from the glossary",
		strings.Contains(term, "glossary · Epoch (`claimEpoch`): the ownership generation of a lease. It increments only on a takeover."), true)

	verb := readers.Answer(uitools.OpKit, uitools.Args{"topic": "next"}).Text()
	testutil.Expect(t, "a verb is answered from the catalogue in its own words",
		strings.Contains(verb, "command · metasystem goal next: print the next ready goal"), true)

	ruling := readers.Answer(uitools.OpKit, uitools.Args{"topic": "R-121"}).Text()
	testutil.Expect(t, "a ruling is answered from the register",
		strings.Contains(ruling, "ruling · R-121 (2026-09-01): Critique before building."), true)
	testutil.Expect(t, "with its context", strings.Contains(ruling, "Context: designs"), true)

	route := readers.Answer(uitools.OpKit, uitools.Args{"topic": "verification"}).Text()
	testutil.Expect(t, "a workflow is answered by its route",
		strings.Contains(route, "route · End-to-end verification"), true)
	testutil.Expect(t, "naming the skill and its description line",
		strings.Contains(route, "The skill is verify: Prove a change works."), true)
	testutil.Expect(t, "and not its body",
		strings.Contains(route, "Choose the Surface"), false)

	nothing := readers.Answer(uitools.OpKit, uitools.Args{"topic": "kubernetes"}).Text()
	testutil.Expect(t, "a topic the kit does not carry says so rather than inventing one",
		strings.Contains(nothing, "Nothing in the kit's glossary, command catalogue, rulings register or routes names kubernetes"), true)
}

// A kit this process cannot reach is said, and a source it could not read is
// named beside the answer rather than silently dropped.
func TestKitSaysWhatItCouldNotRead(t *testing.T) {
	t.Parallel()
	testutil.Expect(t, "no kit at all is a failed read",
		uitools.Readers{}.Answer(uitools.OpKit, uitools.Args{}).Failed(), true)

	empty := uitools.Readers{Kit: uitools.Kit{Root: t.TempDir(), Commands: fixtureCatalogue}}
	index := empty.Answer(uitools.OpKit, uitools.Args{}).Text()
	testutil.Expect(t, "a kit with no contract says the glossary was not found",
		strings.Contains(index, "AGENTS.md does not point at a glossary"), true)
	testutil.Expect(t, "and names the register it could not read",
		strings.Contains(index, "memory/rulings.md could not be read"), true)
	testutil.Expect(t, "but still answers from the catalogue it has",
		strings.Contains(index, "command: 2 entries"), true)
}

/* ---------------------------------------------------------------- reading -- */

func described() manifest.Manifest {
	return manifest.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Partner:       manifest.Partner{Configured: true, Reads: "the page", Refused: "writing"},
		Sections: []manifest.Section{
			{ID: "fleet", Title: "Fleet", Path: "/fleet", Purpose: "The seats.",
				Shows: "A row per machine.", Projected: false,
				Availability: "This build does not project Fleet. Arrives with g1-s13."},
		},
	}
}

func fixtureCatalogue() []uitools.CommandFamily {
	return []uitools.CommandFamily{{
		Name: "goal", Summary: "the backlog ledger",
		Verbs: []uitools.Command{
			{Name: "next", Summary: "print the next ready goal"},
			{Name: "open", Summary: "open a goal at intake"},
		},
	}}
}

// kitRoot is a kit with the four owners in it, each in the shape the real one
// writes: a contract that points at the glossary, a glossary of bullets, a
// register of rows, a routes table, and one skill with its own front matter.
func kitRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for path, body := range map[string]string{
		"AGENTS.md": "# Repository Agent Contract\n\n" +
			"- The system's terms (lease, epoch, census) are defined in `docs/glossary.md`; " +
			"backlog laws live in `docs/backlog-mechanism.md`.\n",
		"docs/glossary.md": "# Glossary\n\n## Custody\n\n" +
			"- **Checkout lease** — the single-writer claim on one repository checkout.\n" +
			"- **Epoch** (`claimEpoch`) — the ownership generation of a lease. It\n" +
			"  increments only on a takeover.\n",
		"memory/rulings.md": "# Standing rulings register\n\nAppend-only.\n\n" +
			"| id | date | ruling | context | owner | review condition |\n" +
			"|---|---|---|---|---|---|\n" +
			"| R-121 | 2026-09-01 | Critique before building. | designs | Wido | none |\n",
		"wow.md": "# Ways of Working Index\n\n" +
			"| Need | Canonical owner | Load when |\n| --- | --- | --- |\n" +
			"| End-to-end verification | `skills/verify/SKILL.md` | A change claims to work |\n",
		"skills/verify/SKILL.md": "---\nname: verify\ndescription: Prove a change works.\n---\n\n" +
			"# Verify\n\n## Choose the Surface\n\nDrive it.\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		testutil.Require(t, "made "+path, os.MkdirAll(filepath.Dir(full), 0o755), nil)
		testutil.Require(t, "planted "+path, os.WriteFile(full, []byte(body), 0o644), nil)
	}
	return root
}
