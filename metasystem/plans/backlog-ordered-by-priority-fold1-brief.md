Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal backlog-ordered-by-priority)
Date: 2026-09-08

# Fold brief: round 2 of chain backlogorder-build1

The closing read of round 1 (job backlogorder-crit2) returned two
material findings and three notes. Its register landed on trunk after
this worktree was cut, so the findings are restated in full below rather
than cited. The specification is unchanged:
metasystem/plans/backlog-ordered-by-priority-design.md, revision 2,
final.

Round 1 is good work. Thirty focused canaries passed, and the invariant,
the human verb and the ordered listing all do what section 1 to 3 ask.
What follows is the gap those canaries could not see.

## D1 - BOB-01, high - the goal-command bed must read the new table

`goal list --pretty` used to print each goal as two spaces followed by
its identifier, grouped under state headings. It now prints an aligned
table whose rows start with the priority column. Four places in
`scripts/agents/goal-cli-fixtures.sh` still test the old shape, inside
the check that repeated label filters combine correctly:

- two positive checks that a filtered listing contains a line beginning
  with two spaces and the goal name, around lines 754 and 760;
- one negative check that an unlabelled goal does not appear in that
  same shape, around line 756;
- an extractor that pulls identifiers out of lines that are exactly two
  spaces plus a lowercase name and compares the result to one expected
  name, around lines 766 to 769.

Confirmed by execution, twice and independently: the critic applied the
bed's patterns to the new printer's output and matched nothing, and the
orchestrator ran `scripts/agents/goal-cli-fixtures.sh` on your round-1
tree, where it exits 1 with "one-label list filtering lost a match".

Teach the bed the new column layout and keep all three proofs. The
negative check matters most: under the new output it can no longer match
anything, so it silently stopped proving that a filtered listing
excludes unlabelled goals. A check that cannot fail is not a check.

Do not relax any of the three, and do not change the printer to satisfy
the old patterns: the new table is what the design asks for.

## D2 - BOB-02, low - the mechanism document gains the new field and verb

The design's build-boundary paragraph names `docs/backlog-mechanism.md`
among the files this build changes, and the build changed only the
command help. That document explains the sibling human act, set-pin, in
prose, and says nothing about the priority and sequence fields or about
`goal set-priority`, which is exactly the part of the story slice 1
delivers.

Add it, in the document's own voice and length: what the two fields
mean, that both absent means unranked and sorts last, that rank is a
human act at the enrolled terminal, and that inserting a position
re-sequences the others in that priority. Nothing in the existing text
is false, so change no existing sentence unless it becomes wrong.

The other two guidance files the design names describe ordered
work-taking and belong with slice 2. Leave them.

## D3 - BOB-04 - one dead clause inside a live canary

In `TestGoalPriorityListing`, the assertion that the table still carries
state, pin, the unranked marker and the intent detail is a chain of
substring checks, and one link looks for the single-character string
"-". Every listing contains a hyphen somewhere: in the banner, in
identifiers such as "a-unranked", in the empty cells. So that link
passes whatever the code does, and the unranked marker is not actually
proven.

Replace that link with one that fails when the marker is wrong. The rest
of the chain and the ordering checks above it are real; leave them.

This is folded although the critic marked it not material, because a
canary that cannot fail is the defect this page's design read found
twice, and it is one line.

## D4 - BOB-05 - the refusal names the repair route

A canonical branch that violated the dense-rank rule would refuse every
verb, including the one that could renumber the offending priority.
The route into that state is closed, because reconcile refuses a direct
rank edit, so nothing here changes how transactions validate their tip
and you must not widen that.

What to add is one sentence of help: when the tree validation refuses
because a priority's positions have a hole or a duplicate, the refusal
names the ledger's existing repair process, so a person facing a queue
that refuses every verb is told what to do rather than left guessing.
If the refusal already names it, say so in the return and change
nothing.

## Not in this round

BOB-03, the read-side fetch option on the frontier reader, belongs to
slice 2 with the rest of section 4. It is recorded in the goal's next
step. Do not build it here.

# Scope

These four and their tests. No printer change. No new field, verb or
authority. Nothing from section 4, section 5 or the seat projections.

# Verification

Canary first, as before, and report each one:

- The command-level listing canary you corrected under D3.
- `go test ./cmd/metasystem -run 'TestGoalPriority' -count=1 -timeout 5m`
- `bash -n scripts/agents/goal-cli-fixtures.sh`
- `scripts/agents/go-gate.sh --fast`, once, at the end. If staticcheck
  cannot write its cache in your sandbox, redirect the cache and say so;
  that refusal is environmental and the orchestrator reruns the gate
  outside the sandbox anyway.

The orchestrator runs the goal-cli and dispatch beds outside your
sandbox, and those two are the proof that D1 is fixed. Do not attempt
them.

Gap rule: stop and report a gap; never fill it silently.
