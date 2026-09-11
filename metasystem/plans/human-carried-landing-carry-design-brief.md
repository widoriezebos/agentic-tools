Working Mode: design
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, coordinator under goal human-carried-landing-carry)
Date: 2026-09-11

# Design brief: the carried landing, revision 3

## Your authority to author this design

You are dispatched to AUTHOR a design. The composed prompt's Working Modes
table forbids design authoring to every delegate; that row is scoped to the
mode, not the role, and goal `design-prohibition-is-role-scoped` exists to
fix it. Until it lands this paragraph is the override, and it is standing
authority: R-89-m1d (Wido, 2026-09-09: "switch back to Claude using Fable
5.1 for designs now") and R-25 put design authoring on this lane. Author the
design; do not stop to report an authority conflict. Dispositions,
certification, the ledger and the receipt stay with the orchestrator.

## What this goal is

`metasystem/plans/goals/human-carried-landing-carry.md`: Wido's ruling of
2026-09-04 ("All approved") moved the carry — points 02 and 04 to 09 of the
human-carried landing design — to this goal as a fresh chain, with the full
file list declared. The audit (point 03) landed under the sibling goal
`metasystem/plans/goals/human-carried-landing.md` as ef296c39: the refusal
register `metasystem/internal/refusal/register.go` now holds every refusal
code, and 57 of its rows name `land.sh --carried` as the human verb that
carries past them, 48 of them marked `Pending: "human-carried-landing"`
because the verb is not in the tree (the `Row` struct is at lines 13-21; one
row: `{Code: "evaluator-unavailable", Owner: "scripts/agents", Site:
"commit.sh:458", Shape: Agent, Override: "land.sh --carried", Commands: 1,
Pending: "human-carried-landing"}`).

Why it matters now, from the umbrella
`metasystem/plans/goals/the-metasystem-validates-itself-with-itself.md`:
on 2026-09-10 every landing on one seat was blocked by the receipt
machinery for a whole day with no lawful way past it, and tonight this
seat landed its own chain (612b1e578) only through a hand-made publication
under a delegated word, because the deep receipt cannot go green on trunk
(goal-full-coverage's ten-minute timeout) and the wrapper has no verb for
a verified human to carry a landing past a named refusal. The verb you
design is the lawful form of what was done by hand.

## What you are revising

`metasystem/plans/human-carried-landing-design.md`, revision 2.1, landed
4ad38918c on 2026-09-04. Write revision 3 as a NEW page,
`plans/human-carried-landing-carry-design.md` (a new file under `metasystem/`), that carries
points 02 and 04 to 09 forward (01 and 03 stay on the old page; cite them,
do not copy them) and answers the thirteen findings the old page accepted
and deferred to "revision 3, the carry rewrite" (its last table, "Revision
2.1 dispositions of the second critique"). The old page's line numbers
are a week stale: since it landed, `scripts/agents/land.sh` moved five
times, `scripts/agents/commit.sh` four, `internal/landing/observe.go`
three, `internal/goal/verbs.go` five, `cmd/metasystem/goalsync_mutations.go`
four, `internal/channel/question.go` four, `internal/refusal/register.go`
nine, and the shared testing contract (1b12f534) and the workspace
receipt (612b1e578) changed what a landing runs. Read the tree as it is
and cite it at HEAD; never carry a line number over from the old page.

## The thirteen standing findings, each to be answered in the text

From the old page's revision 2.1 table (evidence at the old lines; you
re-find each in the current tree):

- HCL-C-02: `governance/types.go` admits an empty tuple or
  TEMPORARY_HUMAN_WORD; no terminal row can carry HUMAN_AUTHORITY_PROVEN
  and the human name has no source. Design the terminal row's authority
  wire and the name source, with a byte-level fixture. The proof entry
  points today are `Prove` (metasystem/internal/humanauthority/authority.go:611)
  and `ProveOrTemporaryGoalAuthority` (:302); `goal set-budget` and
  `goal enroll-terminal` are the verbs whose proof the carry must equal
  (tonight's fact: a terminal enrolled through a wrapper script records
  the wrapper's pid and every later act says TERMINAL_NOT_REACHED; the
  enrollment must be typed at the pane).
- HCL-C-03: commit.sh builds the evaluator through the same gate the
  carried mode makes advisory; land.sh keeps judgment refusals. Design
  the evaluator from the base tree and name every remaining judgment
  refusal advisory. Today's commit.sh: `test verify` on the proved index
  (:272, agent branch only), the proof engine (:313), the LANDING
  comparison (:321-342), `landing observe` (:462), the landed-tree
  postcondition (:573-580); the human branch is taken only when the
  checkout holds no lease claim epoch (:34-52), which a seated checkout
  always holds.
- HCL-C-04: land.sh refuses an empty staged set after the rebase; the
  fresh word has no landing path. Design the amend-and-rebind rule after
  a rebase, with a fixture that lands the second word. Tonight's fact:
  land.sh's push loop (:597-618) has no path that pushes an existing
  commit; the seat reset, re-staged, re-received and pushed by hand.
- HCL-C-05: `goal done` archives between the push and `goal carried`;
  archived goals refuse. Write the obligation before the push, or make
  `done` refuse an open carry row.
- HCL-C-06: obligations key on Finding and Chain alone; seven digits
  collide. Finding carries the full forty-digit id.
- HCL-C-07: the command layer needs the register finding for the
  counselor record and the acceptance. Name every command-layer effect
  for chain `human-carried`.
- HCL-C-09: `claim.go` stamps unconditionally; `review_reference.go`
  requires an implementer job. Add `review_reference.go` as the fourth
  seam of the `commit:<sha40>` subject, with its fixture.
- HCL-C-10: audit grammar (point 03): tokens inside literals; the landing
  set from `wouldRefuse`, `carriageError`, `knownRefusalCode`;
  `chain-full-gate-refused` recorded as a defect. Cite what slice 1
  landed and what remains.
- HCL-C-11: slice 1 carried pending rows; slice 2 writes
  NO-PENDING-AFTER-SLICE-2. Say which rows this chain flips and how the
  test proves none is left.
- HCL-C-12: the channel appends the wanted token to any authenticated
  reply, so an authenticated "no" binds. The carry token must occur in
  the human's own text; negative fixture. Today's question kinds are
  `budget-above-norm`, `fork`, `reserved-decision`, `stop`, `other`
  (metasystem/internal/channel/question.go:201-205); `carry` is new.
- HCL-C-18: every command mints a fresh operation; recover.go has no
  carried case. Idempotence keyed by ApprovedRef; recover.go in the
  build list. The consumers of `AuthenticatedChannelApproval` today are
  `resume` and `set-obligation` (metasystem/internal/goal/verbs.go:82).
- HCL-C-19: land.sh lands any branch and writes origin and transport
  separately. Bind main, order the two remote writes, recover each
  interval.
- HCL-C-20: the chain question (which root reviews the carry) went to
  Wido; his ruling made it this goal on a fresh chain. Say so and close
  it.

## Two additions the umbrella asks for (point 08)

- A tracked fleet cap on open carries per seat, default 1, a budget-law
  key refused in `metasystem.conf.local` like the other budget-law keys
  (name the key; read how the existing budget-law keys refuse a local
  override).
- A carry may not stack on a base whose carry debt is unpaid: the debt is
  the obligation of 07; say what "unpaid" means and where the refusal
  sits (shape c, an ask, never a silent stop).

## What the landing looks like today, so the carried form fits it

The ordinary chain landing: `land.sh -m <msg> --goal <id> --chain <root>
--test-receipt <receipt> --staged-only`; commit.sh's agent branch runs
`test verify`, builds a proof engine, compares the LANDING projection,
observes the candidate (`landing observe --chain … --test-receipt …`),
commits with the trailers `Machine`, `Landing-Provenance`,
`Landing-Provenance-Verdict`, `Goal-Item`, and requires the landed tree
to equal the proved tree; land.sh then fetches, rebases, runs `test
verify` on the rebased tree, pushes with a retry loop, and syncs
transport. The receipt is schema 2 (`landing test-receipt --mode auto`),
keyed since 612b1e578 by the selected groups' execution identities with
the workspace projection as the floor. The refusal codes a carry may pass
are the 57 rows; the ones it may never pass are the Identity-shaped rows
and the ledger-meaning checks (goal binding, held, claim revision), as
the goal `human-carried-landing-verb` (a duplicate that now points here)
phrased it: "never past Identity-shaped refusals or the ledger-meaning
checks".

## What must not change

- Point 01: refusals bind agents; a verified human is never refused. The
  carry is one named refusal or one named receipt group per word, never a
  blanket.
- The identity gate is exactly `goal set-budget`'s: the enrolled terminal
  or the authenticated channel answer; the temporary word is not a proof.
- The deferred review is never deleted: an obligation on the landing goal,
  discharged by a critic of the commit or by accepted risk, and `goal
  done` refuses while it is open.
- Every use counts and speaks (point 08); no threshold refuses a verified
  human, the cap asks.

## Fixtures

Name each fixture under the point it proves, as the old page did (its
thirty-eight are the floor for points 02 and 04 to 09; add the two
additions' fixtures and the thirteen findings' fixtures). Each with a
decidable pass condition and a two-minute ceiling. The land bed
`metasystem/scripts/agents/land-fixtures.sh` (thirteen legs today, with a
contract-bearing seed, a peer clone and enrolled fixture engines) and the
goal command bed `metasystem/scripts/agents/goal-cli-fixtures.sh` are the
beds to extend.

## Deliverable

Write this new file: `plans/human-carried-landing-carry-design.md` (a new file under `metasystem/`)
— the full file list declared in a "Files this design touches" section
(land.sh, commit.sh, dispatch.sh, the goal command layer, recover.go, the
channel, the refusal register, the beds), every decision stated so two
implementers would build the same thing, and a revision record naming
each of the thirteen findings and the two additions and what answers it.
Ground every claim in the tree at HEAD; state plainly anything the code
cannot answer. Wall-clock budget: 60 minutes. Design only; you implement
nothing and run no bed.
