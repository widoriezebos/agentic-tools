# The memory system, step 1: the structure, visible in the UI

- Kind: design
- Id: 01M348YTJ2EY11F37Y5KBVNSER
- Status: accepted
- Revision: 9, 2026-09-23, on Wido's ruling: intent and doctrine never name goals — they are the project's own, so `check` and the write route refuse a `Goals` line on either (rule 1 and section 4); revision 8 was: 2026-09-22, on Wido's ruling of the same afternoon — areas are removed: goals are the only subdivision, and per goal the Project section shows its intent, its designs, its decisions and its open questions; revision 7 was: structure first, the smallest thing that already works in the interface, nothing that blocks a hole nobody has fallen into yet. Revisions 1 to 6 and Astra's three rounds are records of how the structure was found: [revision 6](../memory-system-r6.md), [round 1](../memory-system-critique-astra.md), [round 2](../memory-system-critique-astra-r2.md), [round 3](../memory-system-critique-astra-r3.md). Everything they settled that step 1 does not need is in section 6, to be built when it hurts, on top of the same fields.

Paths are relative to the checkout root. `<root>` is the state root the engine resolves; in this checkout that is `metasystem/`, and the host's `plans/` at the checkout root is read as a second home for designs.

## 1. What this is

The Project pane today lists documents by filename and calls them architecture, designs, and open questions (`metasystem/internal/ui/project/catalogue.go:97-124`). Nothing in the kit declares a document's kind, and a project's intent, architecture decisions and open questions have no home at all. Step 1 gives them one, in three rules, and shows it in the pane. It adds no verb of authority, no landing rule, no adoption change, no engine change beyond one read-only package.

## 2. Rule 1: a record declares itself

A record is a Markdown file whose first line is its title and whose head is a short `- Key: value` list, ended by the first blank line. Three keys, all required, and one optional:

```markdown
# Events are the source of truth

- Kind: intent | doctrine | decision | design
- Id: 01K5R2H3ZQ8V6M7N9P0A1B2C3D
- Status: draft | accepted | superseded | done
- Goals: refund-worker checkout-v2        optional: the ledger goals this record is about
```

- `Id` is any string unique across the project and never changed; `metasystem project id` prints a fresh ULID for those who want one. References to a record use its id, so the file may be renamed or moved.
- `Status` is maintained by hand, as every ADR log does it. `done` is for a design whose work shipped.
- `Goals` names goals by their ledger id (`plans/goals/<id>.md`, concluded ones under `records/goals/`); a record with no `Goals` line is about the project as a whole. Never on intent or doctrine, which are the project's own: what it is for and how it is shaped are about the whole by definition, and `check` refuses a `Goals` line there (Wido, 2026-09-23).
- Optional, unvalidated in step 1, kept as plain references for the pane to show: `Cites:`, `Affects:`, `By:` (who accepted it), `Supersedes:`.

The body is free prose. Unknown keys are kept and ignored.

## 3. Rule 2: the homes

| Kind | Home | Shape |
| --- | --- | --- |
| intent | `<root>/docs/intent/` | a book: `index.md` plus chapters |
| doctrine | `<root>/docs/doctrine/` | a book: `index.md` plus chapters |
| decision | `<root>/docs/decisions/` | one page per decision, any file name |
| design | `<root>/plans/designs/` and, in this checkout, `plans/designs/` | one page per design, subdirectories allowed |
| question | `<root>/memory/questions.md` | one table |

A **book** is a directory with an `index.md` that is itself a record of the kind, plus one list in its body:

```markdown
## Chapters
- 01K6… — Users
- 01K7… — Billing
- doc:metasystem/docs/paper/01-the-shift.md — 1. The Shift
```

`## Chapters` is reading order: a chapter is a record of the same kind by id, or an existing document by checkout-relative path, unchanged where it is — which is how the paper becomes the MetaSystem's intent and `architecture.md` its doctrine without moving a byte. A book of one page has an `index.md` and no chapters.

The **questions register** is one table:

```markdown
# Open questions

| id | opened | question | goals | status |
| --- | --- | --- | --- | --- |
| Q-01K8… | 2026-09-22 | Where does an adopted application's intent live? | | open |
```

`status` is `open`, `answered: <reference>`, or `withdrawn`. Rows are appended; status is edited in place.

Everything else stays where it is: `covenant.json`, `docs/covenant-evidence.md`, `docs/project-rules.md`, the goal ledger, `memory/rulings.md`, `memory/known-issues.md`, the 102 historical designs under `metasystem/plans/`, which are not typed and stay browsable by path.

## 4. Rule 3: one read-only resolver

`metasystem/internal/project` reads the homes above and nothing else. It parses heads, reads the two indexes, reads the register, and answers:

- `metasystem project list <kind> [--goal <id>] [--status <s>]` — `question` is a kind here too, read from the register
- `metasystem project show <id>` — the record, its home, its status, its areas, its references, and what references it
- `metasystem project tree` — the ledger's goals (live ones from `plans/goals/`, concluded ones from `records/goals/`), each with its state and counts of the records about it by kind and status, then the project-wide bucket
- `metasystem project check` — refuses: a duplicate id (pages and question rows share one id space); a head missing a required key; a head line that is not `- Key: value`; a key declared twice; an unknown kind or status; a `Goals` line naming a goal the ledger does not have; a `Goals` line on an intent or doctrine record at all ("an intent or doctrine record names goals"), which the write route refuses in the same words; a chapter id or path that does not exist, or a chapter record of another kind than its book; a chapter path that is not a regular file inside the checkout; a register row without an id or with an unknown status
- `metasystem project id` — a fresh ULID

It imports `stateroot` and nothing of the UI. `internal/ui/project` becomes a thin adapter over it and the catalogue table is deleted. Tests: temporary repositories with a one-page book, a book with chapters and a bound document, two areas, three designs in two homes, and every refusal above.

## 5. The pane

`/project` shows the goals from `project tree` on the left, under a "Project" entry for the whole. The project entry shows, in this order: intent (the index, its chapters in reading order), doctrine (the same), decisions, designs, open questions — each as a list of titles with status, opening in the reading view that exists today. A goal entry shows the goal's own `Intent:` line from the ledger, then the designs, decisions and open questions whose `Goals` name it; doctrine is project-level and is not repeated per goal. A bound chapter opens its document. Nothing is listed that has no head; the rest of the checkout's Markdown stays browsable by path, in a separate "Documents" list, with no kind claimed. The row layout that wraps paths character by character is fixed. Designed as `g1-s21` revision 3, one slice.

## 6. Later, when it hurts

Each of these was designed in revisions 4 to 6 and critiqued to convergence; none is needed for the structure to exist and be used. Each is a check or a verb on top of the same four keys and the same homes, so nothing in step 1 is redone when it comes.

| Hole | Builds on | Comes when |
| --- | --- | --- |
| Proven acceptance: a human-only verb that records who accepted what, at which bytes, from the enrolled terminal; corrections; the `given` compare-and-set | `Status`, `By:`, the rulings register | a second decision-maker joins, or an acceptance is disputed |
| Exact revisions by Git blob in references | `Id` | a goal needs `DefinitionRefs` (gate 5) |
| Landing admission for pages under `docs/` and nested `plans/designs/`; the adopted-application root; the host-root home's guarantee | the homes | the first landing that must carry one of these pages |
| Adoption seeding and inception writing the books | the homes | the first adopted application after step 1 |
| Principals with stable handles; contributors on pages | `By:` | many humans |
| Facts and proposals registers | the questions register's shape | someone needs them |
| Areas — a structural level between the project and its goals | `Goals:` | Wido asks for it; removed 2026-09-22 as premature |
| Concluded designs moving to `records/designs/` | `Status: done` | `plans/designs/` gets crowded |

## 7. Slices

1. **Resolver and the kit's own record.** `internal/project` with its tests and the five verbs; then, by hand: `metasystem/docs/intent/index.md` (vision from the paper, `## Areas`, chapters bound to the paper's files), `metasystem/docs/doctrine/index.md` (chapters bound to `architecture.md`, `concepts.md`, `design/design-principles.md`), heads on the live interface designs under `plans/designs/` at the checkout root (moved from `plans/user-interface/`), this design's head, and `metasystem/memory/questions.md` with the questions this work left open. `project check` passes. ~4h. Roster: Claude on Opus builds, Codex Sol reviews the code.
2. **The pane**, `g1-s21` revision 3, over the resolver; the row-wrap fix; a browser walkthrough by Claude, then Wido. ~4h.

Nothing else is scheduled.
