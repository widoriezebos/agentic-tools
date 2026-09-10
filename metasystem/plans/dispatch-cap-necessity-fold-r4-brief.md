Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal dispatch-cap-necessity)
Date: 2026-09-10

# Fold round four: rebase onto today's main, nothing else

Follow-up round on chain dispatch-cap-build1-20260909; your worktree
carries round three, which the second critic closed with no material
finding. The chain cannot land as it stands: main gained
1b12f534 ("Make testing risk-selected and retain proof for landing")
after this chain's base, and it touches internal/dispatch/budget.go, so
the certified round-three diff no longer applies ("patch failed:
budget.go:492"). The dispatcher rebases your worktree onto today's main
for this round.

## Mandate

1. Resolve the rebase so that the box built in rounds two and three is
   preserved exactly (the charge rule, settlement, the two projection
   fields, the refusal line with ", " between breach fields then "; "
   and the reserved segment then "; " and the setup-refusal clause,
   endedAt transition-owned, tests T1-T12 and the component
   assertions) on top of what 1b12f534 changed in the same files. Take
   main's side for everything that is not this chain's; take the
   chain's side for everything that is. Where both changed the same
   lines, keep both behaviours and say so in your return.
2. No new behaviour, no new file, no test weakened.

## Proof

`go build ./...`, `go test ./internal/dispatch/ -count=1`,
`./internal/obligationstate/`, gofmt, vet, with your cache workaround.
Report the round as your own and list every conflict you resolved.

## Constraints

Wall-clock budget: 15 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
