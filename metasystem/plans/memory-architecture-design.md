# Memory architecture: the information quadrants (design draft, round 2 folded)

Goal: memory-architecture (HIGH, Wido). Returns to Wido as a draft
before any file moves (R-2). Critique loop: design-critic to
convergence, FAILSAFE round 3 — survivors go to Wido with the
draft. Round 1: 7 material folded. Round 2
(design-critic-20260827t105400z-671b): 7 material (4 critical),
folded below, each marked [MAC-R2-00N].

## The evidence (R-7's hard look)

The plans/ area holds 432 tracked files: critique/disposition/
facts/review records of concluded work, 72 concluded goals under
plans/goals/done (materialized there by the goal engine), ~12
living registers, ~40 designs (a handful active), and the thin
true-plan slice: goals/, goals-drafts/, backlog notes. docs/ is
already clean static documentation.

## The architecture: one rule, five trees, four answers

OWNERSHIP RULE (the oracle): engine verb `metasystem path owner
<path>`: resolve against the repository root, lexically clean,
judge the RELATIVE result by prefix — symlinks judged by entry
path, never referent:

- not contained in the repository (leading `..` after cleaning, or
  an absolute path outside it) → `outside`, nonzero exit
  [MAC-R2-004]
- `metasystem/` prefix (the vendored dir's actual name), and the
  directory itself → `metasystem-generic`
- `artifacts/`, `bin/` under either owner → `runtime` (machine
  state; excluded from documentation law and upgrade law alike)
- everything else → `app-owned`

The verb also prints the repo mode (`template` | `adopted`, by the
existing marker test) so the self-hosting distinction comes from
the same mouth.

PLACEMENT RULE: within an owner's tree, four places, same names
both sides:

- docs/    — static documentation; changes by deliberate landing.
- memory/  — dynamic registers that accrete; never "done".
- plans/   — live intent ONLY: goals/, goals-drafts/, active
             designs.
- records/ — history. Append-only for agents and humans; the goal
             engine is the SOLE mutator of records/goals/ under its
             ledger rules — reopen moves a goal back to plans/goals
             as a recorded ledger event, prune replaces pruned
             files with tombstone entries in the ledger history, so
             auditability survives the mutation [MAC-R2-005].

## The state-root mapping [MAC-R2-003, critical]

A classifier is not enough: adopted machinery (receipts, steward
writes, future registers) would recreate metasystem/memory at
runtime. The design therefore adds ONE mode-aware engine function —
`stateRoot(kind)` — that every register/receipt writer routes
through: in template mode it resolves inside the metasystem tree
(self-hosted working set, as today); in adopted mode it resolves to
the OUTER app trees (memory/, records/). No writer computes its own
register path again. The migration's behavioral-owner slice IS the
rerouting of the known writers through this function; a
no-new-paths fixture (guard against a vendored-tree memory/ or
receipts file appearing at runtime in an adopted fixture repo)
keeps it true.

## The memory preservation contract [MAC-R1-002]

Adopted repos carry no metasystem/memory: adopt.sh excludes it as
it excludes development/ today. With stateRoot() routing all
accretion to the app's trees, the vendored tree holds zero
accreting bytes and is 100% replaceable. In the template, the
metasystem tree's registers are the self-hosted app's working set;
no upgrade law applies to the source of upgrades.

## The upgrade invariant and its honest dependency [MAC-R2-001, critical]

There is no executable cross-version upgrade workflow today:
same-version re-adoption no-ops early, an older-version target is
refused toward the manual procedure in
docs/metasystem-reconciliation.md. The rehearsal R-10 demands
cannot be faked around that absence, so the design splits it:

- THIS design defines the INVARIANT as a contract artifact: the
  DECLARED TOUCH LIST (below) plus the rule "every byte outside
  metasystem/ and outside the list and outside `runtime` paths is
  identical after any upgrade enactment."
- The EXECUTABLE rehearsal lands with an upgrade workflow — a
  follow-up goal (upgrade-workflow, drafted beside this design for
  Wido's word) that turns the reconciliation doc's manual steps
  into a drivable `metasystem upgrade` path. Its acceptance
  INCLUDES this design's rehearsal fixture: enact an upgrade from
  template A to template B against a target carrying real app
  content in all four outer trees; assert the invariant
  byte-for-byte.
- Until then, a WRITE-TRACER fixture (below) guards the half that
  is provable now: adoption's writes match the touch list exactly.

## The declared touch list [MAC-R2-002]

Generated from adopt.sh's actual write inventory and asserted 1:1
by a write-tracer fixture (adoption run under a tracer that records
every path written; the fixture diffs the trace against the list,
so the list cannot rot silently). Initial inventory: the vendored
metasystem/ tree (minus memory/, development/); hook enrollment;
.gitignore enrolled lines; .gitattributes appended lines; tailored
metasystem.conf; seeded plans/goals ledger files and the
goals-accepted.json baseline; runtime registrations under .claude/
.devin/.agents; bin/metasystem; docs/project-rules.md only-if-
absent. Growing the list is a reviewed design change.

## The goal engine slice, honestly scoped [MAC-R2-006]

Concluded goals archive to records/goals/. The contract surfaces
that change with it, enumerated: done (write target), reopen (move
back + ledger event), prune (tombstone), reconciliation's capture
and guard of the goals subtree, commit validation's
single-subtree read, and the migration that materializes done
goals. One certified slice, appetite 3h, its own fixture legs.
Until it lands, plans/goals/done is declared an engine-owned record
area — interim by name.

## Migration (this repo), sliced

1. (2h) memory/ + stateRoot(): move the registers; route the
   behavioral owners (receipt.sh default, steward writes, audit
   required paths, mission protected paths, behavior-surface
   classifications, flake-protocol citations — enumerated by sweep
   as the slice's first task) through stateRoot(); acceptance:
   suite green + zero stale-path references + the no-new-paths
   guard.
2. (1.5h) records/: one-time MIGRATION MANIFEST, human-reviewed at
   execution, mapping THE RECORD-CLASS FILES ONLY — live goals,
   goal drafts, active designs, registers routed to memory/, and
   the plans README stay in place by class exclusion [MAC-R2-007].
   Flat records/<area>/ (proposed ~15 areas), one level deep.
3. (1h) oracle verb + write-tracer fixture + touch-list contract.
4. (3h) goal engine slice as scoped above.
5. adopt.sh seeds the outer trees (empty memory/, plans/, records/
   with one-sentence README law lines).
6. upgrade-workflow goal (separate draft, Wido's word): the
   drivable upgrade + the full rehearsal fixture.

Nothing moves before Wido ratifies this draft.

## What this deliberately does NOT do (R-11)

No memory database, no index machinery, no per-file ownership
tags, no new roles, no runtime records-classification. The additions
forced by critique — stateRoot(), the write-tracer, the touch list
— are each one function, one fixture, one table: mechanisms that
keep the one rule true, not new structure.

## Open questions carried to Wido

1. receipts.log: memory (draft's position) or records?
2. Concluded designs: records/, with a pointer line from docs
   where a design explains a shipped mechanism?
3. The ~15-area manifest list: object now or at slice 2's review.
4. The upgrade-workflow follow-up goal: fund now (it gates the full
   R-10 rehearsal) or after the migration slices land?

## ROUND 3 (failsafe): four disputes for Wido

The declared loop ended at round 3 with four material findings, all
marked DISPUTE-FOR-WIDO by the critic. Full texts in
records-to-be: artifacts/agents/design-critic-20260827t110050z-c559.
Each with the smallest decision and my recommendation:

1. [MAC-R3-001, critical] One touch list cannot govern both first
   installation (which MUST seed app-owned files) and upgrades
   (which MUST NOT touch them). DECISION: two lists or one?
   RECOMMEND: two — an INSTALL list (may seed) and a strictly
   smaller UPGRADE list (may touch only vendored + enrollment
   lines); the rehearsal asserts against the upgrade list.
2. [MAC-R3-002, high] The write-tracer has no implementable
   observation contract (tree-diff misses write-then-restore;
   OS-level tracing is platform-split). DECISION: which observation
   standard? RECOMMEND: portable before/after tree comparison
   including mtimes, DECLARED as the contract's limit —
   write-then-restore is out of scope by declaration, not by
   accident; no OS tracer.
3. [MAC-R3-003, critical] stateRoot() scoped to registers/receipts
   is too narrow: vendored entrypoints pass their own root to
   serving-goal, open-work, and steward operations — ALL app state
   needs the two-root mapping or adopted repos keep goals under the
   vendored tree. DECISION: widen stateRoot() to all app-state
   kinds (goals, open-work, steward, evidence pointers) in the
   engine slice, raising its appetite ~3h→5h? RECOMMEND: yes —
   partial routing would rebuild the mixing this design exists to
   end.
4. [MAC-R3-004, critical] metasystem.conf is app-tailored but
   lives beside vendored scripts — no ownership-safe home.
   DECISION: where does tailored conf live? RECOMMEND: app root
   (app-owned, upgrade-safe); vendored scripts already resolve
   their conf via one config reader, so the move is one resolution
   change plus the conf.local convention beside it; the vendored
   tree may keep a pristine metasystem.conf.template.

Design state: CONVERGED-WITH-DISPUTES. No file moves, no slices
scheduled, until Wido rules on 1-4 and the open questions above.
