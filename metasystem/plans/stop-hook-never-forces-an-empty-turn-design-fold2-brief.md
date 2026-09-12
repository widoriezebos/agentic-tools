Finding Id: stop-hook-design-fold-2
Disposition: accepted

# Finding Being Corrected

Round 2 stopped under the gap rule on finding 3: the idle-with-backlog
refusal needs a seat-executable command, and no authorized source for a
dispatch brief exists when the ready goal has none. The orchestrator answers
the question here; finish the six folds and add section 7.

# Disposition Reasoning and Evidence

The answer to finding 3. DONE clause 4 keeps the idle-with-backlog path as
it is; DONE clause 1's "one command" for that path is the seat's first
lawful act, not a whole workflow: `bin/metasystem goal claim --root . --id
<ready goal> --lineage <the seat's lineage>`, rendered with the real goal id
and the lineage the verdict already knows. Authoring the brief and
dispatching are the seat's own work under the claimed goal's next-step,
which the refusal quotes as it does today; the steward-continuation launcher
and its authority stay the steward's and are not the seat's command. So:
the idle refusal names the condition (approved backlog, no job), the ready
goal, the claim command, and the goal's next-step; it stays mandatory and
counts toward the three-refusals handoff exactly as today. Write that into
section 2 in place of the open question, and keep the five seat-actionable
branches' command eligibility rule as round 2 left it.

Then finish what round 2 left unfinished, in place, on the page you have
(stop-hook-never-forces-an-empty-turn-design.md in the metasystem plans
directory); touch nothing else; do not commit; every cited path must exist
in the worktree under the metasystem/ prefix; stay under 280 lines, plain
English, short sentences:

- Fold 1: the migration of every fail-closed producer in
  metasystem/internal/goal/turnverdict.go and
  metasystem/internal/goal/sessionstop.go to the typed classification, and
  the removal or replacement of the launcher's raw block in
  metasystem/scripts/enforcement/claude-code-hooks.json line 25.
- Fold 2: the common host boundary every runtime's stop gate calls, and
  the two-runtime proof on two production runtimes of the roster (claude,
  codex, devin), with the fake as an auxiliary bed.
- Fold 4: the steward notification path, and its fallback: either an
  independent durable record with a named steward reader, or the weaker
  "delivery unconfirmed" guarantee stated in DONE's terms.
- Fold 5: the idle digest amendment (relevant activity only) stated as the
  contract change it is, with the test that pins it, and DONE clause 4's
  three-refusals handoff intact.
- Fold 6: the seen-state episode keyed to the hook's turn generation and
  deadline end, not a rolling window.
- Section 7, "Slices": member goals of one mechanism each, in landing
  order, each with a goal id in the program's spelling (lower-case words
  joined by hyphens), its own DONE sentence, the umbrella DONE clauses it
  carries, and the section 5 fixtures that belong to it. The rule (Wido,
  2026-09-10): no large goals; an umbrella record for context and
  one-mechanism members with their own DONE.

Wall clock: 25 minutes. If a fold cannot be finished, write what is
settled and mark the open point in one sentence on the page; do not stop
the round for it.

evidence carries exactly two `{command, observed, level}` items,
replayable from the worktree's repository root: (1)
`git -C metasystem status --short` observing the one changed file; (2)
`( cd metasystem/plans && wc -l stop-hook-never-forces-an-empty-turn-design.md )`
observing the line count. whatWasDone names each fold's outcome in one
line and the member goals of section 7.

# Unchanged Return Contract

The original role and return schema remain binding without additions, removals, or relaxations.

Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.

Schema: the same scripts/agents/schemas/implementer.schema.json used for the original dispatch
