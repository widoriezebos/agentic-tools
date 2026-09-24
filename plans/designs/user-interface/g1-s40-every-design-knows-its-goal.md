# g1-s40 Every design knows its goal

- Kind: design
- Id: 01M39ZE2PMKCAHV8708BJY386F
- Status: draft
- Cites: 01M348YTJ2EY11F37Y5KBVNSER

Revision 1, 2026-09-24, Claude on Fable, on Wido's checks and ruling: "can you check that goal designs have indeed the goal designated?" (they do not: the 102 historical designs under `metasystem/plans/` have no head at all, and 73 of them are named for a ledger goal) and "Will these have the proper metadata?" going forward (from the interface yes; from a seat no, because the kit's own instructions never learned the record head). Ruled: "Create a design for this, get it critiqued by Astra, and then implement it." Draft until Astra has read it.

## What is true today

A record's scope is its `Goals:` line (memory system, rule 1). The interface writes it from context. The kit does not: the design-obligation gate, the design-critique skill and the project rules say nothing about the head, the `Goals:` line or the `plans/designs/` home, so a seat writing a design during a goal produces what the 102 historical designs are, a file by convention, invisible to the resolver, to the goal page and to the Partner. The convention was the file name, `<goal id>-design.md`, which is metadata enough to repair the past mechanically.

## Step 1

**The kit learns the head** (four small changes, all in the kit).

1. `docs/design/design-obligation-gate.md` gains "A design is a record": the exact head to write (`- Kind: design`, `- Id:` from `metasystem project id`, `- Status: draft`, and `- Goals: <goal id>` for a goal's design), the home `plans/designs/` under the installation, and that a design with no head is not a design for the gate. The design-principles' slicing law points at it.
2. `skills/design-critique/SKILL.md` reads the head first: a critique of a design written for a goal that names none, or one that fails `metasystem project check`, opens with that refusal and stops.
3. `metasystem project check` joins the fast gate's checks (`scripts/agents/go-gate.sh` or wherever the fast gate's list lives) so a design that fails the grammar cannot land.
4. `goal open` and `goal claim` print, in their confirmation, the head a design for that goal must carry, so the words are in front of the seat at the moment it starts. One line each, from one helper.

**The past is repaired** (one pass, one commit).

5. The resolver accepts a second design home, the kit's history: `plans/*-design.md` directly under the installation, beside `plans/designs/`. No file moves, so every path the ledger's goals and the docs mention stays valid; the pane shows the home as "designs (historical)".
6. A typing pass, `metasystem project type-designs --root <installation> [--apply]`, gives each headless `plans/*-design.md` a head: `Kind: design`, a fresh id, `Goals: <id>` where the file name `<id>-design.md` matches a ledger goal (live or concluded), `Status: done` when that goal concluded and `accepted` otherwise; a file matching no goal is typed with no `Goals:` line and `Status: accepted`. Without `--apply` it prints the plan; with it, it writes, runs the check, and prints the files typed without a goal for the human's eye. It refuses to touch a file that already has a head. Run once on this checkout in the same commit.
7. The register's open question on the historical designs is answered by the pass.

## Later, when it hurts

- Decisions and questions written by seats: the same paragraph in the gate once seats write those.
- A goal file naming its design (the `Design:` line of g1-s26's later list).
- The 29 designs matching no goal: a human pass over the list the verb prints.
