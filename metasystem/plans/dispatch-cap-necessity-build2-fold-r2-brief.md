Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal dispatch-cap-necessity)
Date: 2026-09-10

# Fold round two: proof-attempt reservations are a third, named component

Follow-up round on chain dispatch-cap-build2-20260910. Round one carried
the box onto today's main and stopped, correctly, at a gap: 1b12f534's
testing-risk proof attempts add reserved minutes to ReservedJobMinutes
but to neither of the box's two components, so a projection with a
proof attempt can break the sum invariant and its refusal line can
understate the total. The decision, which keeps both contracts whole:

## The rule

ReservedJobMinutes is the sum of THREE named components:
ObservedJobMinutes (ended delegated and governed work, settled as the
box says), OpenCapMinutes (open delegated and governed work, at its
cap), and ProofReservationMinutes (every proof attempt's reservation
exactly as 1b12f534 charges it today: a live proof attempt at its
reservation, a terminal proof attempt at its full reservation, the
penalty 1b12f534 intends). The box does not settle proof attempts; it
names them. The overflow guard covers the third addition.

The refusal line appends the proof component only when it is not zero,
so the box's exact lines (T10's two, the governed subtest) are
unchanged where no proof attempt exists:

    ...; reserved observed=<n> open-caps=<m> limit=<L>
    ...; reserved observed=<n> open-caps=<m> proof=<p> limit=<L>

The sum-invariant test gains a case with one live and one terminal
proof attempt beside a delegated job, asserting the three components
and their sum equal ReservedJobMinutes and that the line carries
proof=<p>. Nothing in 1b12f534's own accounting or tests changes.

## Mandate

1. The third component and its line, as above; the invariant test case.
2. Everything else from round one stands; no other file moves.

## Proof

`go test ./internal/dispatch/ -count=1` (coverage at or above 75.9),
`./internal/obligationstate/`, gofmt, vet, with your cache workaround.
Report the round as your own.

## Constraints

Wall-clock budget: 20 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
