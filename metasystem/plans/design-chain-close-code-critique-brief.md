Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal design-chain-has-no-lawful-close)
Date: 2026-09-06

# Review brief: the design-critic binding at chain close (chain design-close-build1)

FINDING IDS: chain-unique, DCC-01, DCC-02, ... never F-n.

Round budget: 1 focused round, then at most one correction and its
re-review (the goal record carries reviewRoundLimit 2). R-60-m1's rule:
a finding is material only if it changes what gets built and names the
artifact it would change.

Threat model: the reconcile in
metasystem/internal/dispatch/review_reference.go admitting a
design-critic whose register still holds an open or disputed finding,
or pairing a critic with a chain whose final design file it did not
review when the record carries a design path; the reconcile writing
`reviews` onto a critic record that is not a fresh chain root, or onto
the wrong record; the closure law in
metasystem/internal/dispatch/hazard.go weakened or bypassed (its
staleness rule, its fresh-chain and effort rules must hold unchanged);
the code-critic, warden and verifier bindings changing behavior; the
dispatcher recording `design` on the wrong role or in a form the
reconcile cannot compare; a fixture that passes without the new code;
a weakened or deleted test. Out of scope: closing the design-critic
chain itself (goal merge-stage-critic-close); a fold made after the
last critique staying open (decided in the build brief, D5); taste.

Scope: the computed diff of implementer job design-close-build1.
Contract: metasystem/plans/design-chain-close-build-brief.md (decisions
D1 to D7 and the facts) and the goal record
metasystem/plans/goals/design-chain-has-no-lawful-close.md. The
orchestrator persisted the computed diff and the reviewed tree hash at
metasystem/artifacts/agents/design-close-build1/rounds/1/review.json
with the diff beside it; take reviewedTree from that record. Your tool
surface is read-only; the facts below stand in for runs you cannot make.

Facts the orchestrator established (evidence level: ran): recorded
here after the orchestrator's own gate on the reviewed tree, in the
"Orchestrator runs" section at the end.

# Mandate

1. Trace ReconcileReviewReference for a design-critic end to end: the
   derivation of the reviewed job, the register-closed check, the
   pairing check, the stamp on the reviewed root, and the `reviews`
   write on the critic root; name what each refusal says and confirm
   nothing is written before every check passes.
2. Confirm the closure law in hazard.go is untouched and that, after
   the reconcile, its existing checks (reviews equals the final work
   round, fresh chain, distinct session, effort, model, ended-after)
   are what closes the chain; nothing in the change special-cases
   design-critic there.
3. The dispatcher records `design` only for design-critic dispatches,
   as the repo-root-relative path it validated, and the reconcile
   compares it with the final work round's return diffBoundary exactly.
4. The pins in metasystem/internal/dispatch/review_reference_test.go
   cover the positive path through CloseCheck and each named refusal,
   and would fail against the old code.
5. Nothing outside the declared boundary changed; no test weakened.

If nothing material remains, say so; that closes the chain and the fix
lands.

# Constraints

Wall-clock budget: 40 minutes. Return per the code-critic schema with
the reviewedTree from the persisted review record. Gap rule: stop and
report a gap; never fill it silently.

# Orchestrator runs

On the reviewed tree (402bacb8b810845d105da6b103b29c44e48a9a4e), from
this Mac, evidence level ran: `gofmt -l .` printed nothing; `go vet
./internal/dispatch/` passed; `bash -n scripts/agents/dispatch.sh`
passed; `go test ./internal/dispatch/ -count=1` passed (29 seconds);
`GOOS=linux GOARCH=arm64 go build ./...` passed. The implementer's own
gate additionally ran the whole dispatch fixture bed and the goal
command-line fixture bed green inside its sandbox; the orchestrator
reruns the dispatch fixture bed before landing.
