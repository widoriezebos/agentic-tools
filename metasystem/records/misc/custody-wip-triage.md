# The custody wip triage (L2 of the lean program, 2026-08-28)

Working Mode: design

Subject: branch wip/custody-launch-machine at 7fcd73e (the L1
isolation snapshot; supersedes 3fec78a). Verdicts feed landings
L10-L12; Wido's review gates those landings, not L3-L9. Every
verdict carries its reason; the coordinator briefed and verified
every piece named here during the overnight build, and the
code-critique chain (custody-launch-machine-code, rounds 1-2 plus
seven correction passes) certified the KEEPs cited below.

## KEEP — apply as-is (L10)
- internal/identity: platform-exact identity (darwin microseconds,
  linux ticks+bootID), the ordered verification sandwich with the
  first-read-GONE row, exact-token tag matching. Reason: certified
  across three critique rounds; it IS Ruling C's foundation.
- internal/janitor killproof shapes incl. adapter/supervisor/hold
  forms and the Go tri-state group predicate. Reason: the
  substring-kill hazard is real and the fix was proven by fixture.
- internal/dispatch record provenance (goalId through the
  lifecycle, exact ownership fields, ownership-patch Go verb) and
  the receipt goal=/built_by= keys. Reason: forward-only
  provenance ruled by Wido; already exercised by the landed
  metrics receipt.
- internal/progress evaluator (watermark never-resets, containment
  re-resolution, per-root demotion, events-file-only progress).
  Reason: the honest liveness signal law; health L3-L4 consumes
  its concepts and L10 its code.

## KEEP — apply as-is (L11)
- custody_death + adoption + prefork marker (census-universe
  narrowing with foreign-EPERM and age-slack exclusions, marker
  identity-bounded expiry, nonce-global adoption, cross-group
  custody closure). Reason: four correction cycles of real-machine
  evidence; the recycled-pgid and pre-registration holes are
  genuinely closed.
- claim.go + claim_fingerprint.go + occupancy: the typed outcome
  machine (Ruling-affirmed in operator-surface round 1: the
  outcomes STAY typed), v1 fingerprint with golden vectors,
  per-session occupancy with crash-order healing. Reason: this is
  delegate's engine; rebuilding it would be the 158-hour mistake.

## ADAPT (L12-L13)
- The claim-launch CLI surface and dispatch.sh call-site wiring →
  become delegate's INTERNALS per the operator-surface design
  (typed enum in JSON out, headline grouping for humans; the
  four-field budget tuple joins the admission check per Ruling H
  and the v4 lock order chain → goal-revision → cap → lifecycle →
  occupancy → record).
- critique-round custodial routing → the delegate --role path;
  the standalone driver then deletes per the removal ledger.
- Adapter tag propagation (claude --name, devin config-path
  nonce) → kept, re-verified when delegate lands.

## DELETE (with reasons and reopen-observations)
- The wip copy of docs/orchestration.md's doctrine edit — landed
  separately at 5bdc102; applying it again would conflict.
- Any residue of the two-file chain-attribution experiment —
  already stripped under Wido's exhaustion ruling in the metrics
  arc; reopen only if a critique chain is ever observed without a
  lawful attribution home after delegate lands.
- The stage-4-era fixture beds' scattered hold-leak workarounds
  superseded by the suite's own cleanup fixes — reopen if
  dispatch-fixtures ever leaks holds again.

## The application discipline (L10-L12)
Apply by CHERRY-PICKING content from the worktree per area, not by
merging the branch wholesale (the snapshot mixes areas). Each
landing re-runs the area's own fast tests plus the dispatch fixture
suite once; batteries never run (lean mandate). Conflicts against
the moved main surface in the landing report, never silently
resolved.
