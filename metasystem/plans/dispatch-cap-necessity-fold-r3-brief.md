Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal dispatch-cap-necessity)
Date: 2026-09-09

# Fold round three: the breach fields keep their comma

Follow-up round on chain dispatch-cap-build1-20260909; your worktree
carries round two. The critic (dispatch-cap-crit1-20260909) confirmed
the box is built and found one material defect and four notes; two
fold here.

## DCN-01 (material): breach fields joined with "; " instead of ", "

internal/dispatch/admission.go's new shared helper formatRefusalDetail
joins the breach fields with a semicolon and a space where the renderer
joined them with a comma and a space before this chain. Design section
4.1 authorised one change only: appending the reserved segment after
the joined breach fields, separated by a semicolon and a space, with
the breach objects keeping their fields and today's values. A refusal
that trips two limits at once now prints differently for no reason any
page asked for, and no test catches it because every assertion checks
one field. Fix: breach fields joined with ", " as before; then "; " and
the reserved segment; then "; " and the setup-refusal rule clause where
it attaches today. Add one assertion (in the new admission test) for a
refusal that breaches two limits at once, spelling the whole line.

## DCN-04 (note, folded because it is one line): the sum-invariant
test's governed case

The design's row for the sum-invariant test also asks that a live
governed run counts as one active job; the test checks the settled
minutes, the open ceilings, their total and the attempt count but not
the active-job count. Add that assertion to the governed case.

## Noted, no change

DCN-02 (two existing tests' fixtures gained a pid and timestamps whose
runtime equals the cap, unreported), DCN-03 (three specimen start
stamps moved one second earlier so the design's instant serves as the
proof time; totals unchanged), DCN-05 (two failure reasons reworded to
name the ownership-proof instant the scope cut chose). All three are
recorded as accepted deviations in the dispositions; nothing moves.

## Mandate

1. DCN-01 and DCN-04 as above.
2. Nothing else changes.

## Proof

`go test ./internal/dispatch/ -count=1` and
`./internal/obligationstate/`, gofmt, vet, with the cache workaround
you used in round two. Report the round as your own.

## Constraints

Wall-clock budget: 15 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
