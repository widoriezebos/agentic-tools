# g1-s40 Every design knows its goal

- Kind: design
- Id: 01M39ZE2PMKCAHV8708BJY386F
- Status: done
- Goals: browser-interface
- Cites: 01M348YTJ2EY11F37Y5KBVNSER

Revision 2, 2026-09-24, Claude on Fable, after Astra's critique of revision 1 ([g1-s40-astra-critique.md](g1-s40-astra-critique.md): eight material findings, "build after the eight listed changes", all eight taken; the first of them, that revision 1 had joined what Wido separated, splits this design into two slices). Revision 1 was on Wido's checks and ruling: "can you check that goal designs have indeed the goal designated?" (they do not: the 102 historical designs under `metasystem/plans/` have no head at all, and 73 of them are named for a ledger goal) and "Will these have the proper metadata?" going forward (from the interface yes; from a seat no, because the kit's own instructions never learned the record head). Ruled: "Create a design for this, get it critiqued by Astra, and then implement it." Draft until Astra has read it.

## What is true today

A record's scope is its `Goals:` line (memory system, rule 1). The interface writes it from context. The kit does not: the design-obligation gate, the design-critique skill and the project rules say nothing about the head, the `Goals:` line or the `plans/designs/` home, so a seat writing a design during a goal produces what the 102 historical designs are, a file by convention, invisible to the resolver, to the goal page and to the Partner. The convention was the file name, `<goal id>-design.md`, which is metadata enough to repair the past mechanically.

## Slice 1: the next seat-authored design is a discoverable, goal-linked record

Four changes in the kit, each in the file a seat reads at that moment, and one boundary that proves it.

1. **The gate says what a design is.** `docs/design/design-obligation-gate.md` gains "A design is a record": the exact head to write (`- Kind: design`, `- Id:` from `metasystem project id`, `- Status: draft`, and `- Goals: <goal id>` for a goal's design), and the home: `plans/designs/` **under the resolved state root**, which is the installation in the self-hosted layout and the application's repository root in an adopted one, never `vendor/metasystem/plans/designs/`. The design-principles' slicing law points at it; `wow.md` already loads the principles during design work, so nothing else is added to the always-on instructions.
2. **The boundary that proves it.** A read-only verb, `metasystem project design-of --root <checkout> --goal <id>`, answers whether a design record exists in a resolved home whose `Goals:` names that goal, printing its id and path or the one-line refusal. It is what the design-critique skill runs first (a critique of a design for a goal that `design-of` cannot find opens with that refusal and stops), what the seat's own gate runs before it claims a design done, and what the interface's goal page already computes. A headless document anywhere stays lawful; `project check` keeps its grammar and is not pretended to prove more.
3. **The check is bound to what it reads.** `metasystem project check` joins the fast gate reusing the executable the gate has just built, and `testing.json`'s declared inputs for that group gain the design homes, the decision home, the intent and doctrine books and the question register, so a change to any of them invalidates the retained proof; the gate says that a failing check blocks every delivery that consumes it, code changes included. In an adopted installation the same check runs from the installed executable.
4. **The hint at the moment it matters.** After a confirmed `goal open` (with and without `--claim`) and `goal claim`, one line on **stderr**, never on the JSON stdout: "A design for <id> is a record: see docs/design/design-obligation-gate.md, A design is a record." One helper, the gate's text as the source.

## Slice 2, after slice 1 has landed: the past is typed from a reviewed mapping

5. **The kit's history is a home of its own**, only in the self-hosted layout: the flat `plans/*-design.md` directly under the installation, beside `plans/designs/`, shown by the pane as "designs (historical)". Flat, never recursive: `plans/goals/` is the ledger and `plans/designs/` is already read. Adopted applications never acquire it. No file moves, so every path the ledger and the docs mention stays valid.
6. **A mapping, proposed by the machine and reviewed by a human, never guessed into the files.** `metasystem project type-designs --root <installation>` prints a plan, one line per historical file: the goal proposed from the file name where `<id>-design.md` matches a ledger goal, live or concluded; the status proposed from that goal, `done` only when the goal concluded as done, `superseded` when it was abandoned or superseded (its own words), `accepted` for a live goal; and **unresolved** for a file matching no goal, or whose own first lines name a goal or a status that contradicts the proposal (Astra's examples: a design naming `channel-tells-me-when-something-lands` in its third line, one saying "superseded"). The plan is a file, `plans/designs-typing.md`, that a human edits: an override per line, or leave unresolved. `--apply` reads the reviewed plan and types only resolved lines; unresolved files stay untyped and listed.
7. **How a head is written.** After the title line, a blank line, the head, a blank line, then the original body byte for byte; a file whose first line is not a title gets one from its file name and its original first line stays the body's first. Legacy bullets such as `- Owner:` are body, not head: they stay where they are, after the new head's blank line, so the parser reads one head. Before any write: every proposed head is validated and every proposed id checked against the whole namespace, pages and register rows alike; a file that already carries a valid record is left unchanged; a malformed existing declaration is reported by name and not touched; a repeat run is a no-op. Writes are per file; the write set is the selected historical files and nothing else; the pass never moves a file and never touches either goal-ledger directory.
8. The register's open question on the historical designs is answered by the pass, and the files left unresolved are the human's list.

## Later, when it hurts

- Decisions and questions written by seats: the same paragraph in the gate once seats write those.
- A goal file naming its design (the `Design:` line of g1-s26's later list).
- The unresolved files after slice 2: a human pass over the plan's list.
