# Critique r3 of chain bsws-build1b-20260909 (goal breach-stop-wedges-seat)

Critic bsws-crit4b-20260910 (code-critic, claude claude-opus-5, xhigh,
read-only) on work round bsws-build1b-20260909-r6, reviewed tree
75763b94f1b57ec2bc44ca904b69f4c619e11c5f, 2026-09-10 07:52 to 08:03Z. Three
findings, one material. Dispositions are the orchestrator's (m1d).

| id | severity | finding, in one line | disposition |
|---|---|---|---|
| BSW-12 | high | The chain lets a machine claim goal B beside a breach-stopped A and points every surface at B, but EvaluateGoalAdmission in internal/dispatch/admission.go scans every claim the machine and lineage hold, adds a refusal with a live stop reason for the fenced A, and exits 10; dispatch.sh then runs the breach-stop routes, which skip a fenced goal whose batch is complete, and the dispatch dies with "goal admission required breach-stop but supplied no stoppable route". The seat can claim the next item but cannot delegate on it until a human resumes A: the wedge moved from claim to dispatch, and the planned live proof (claim while fenced) would not have exercised dispatch | accept; fold in round 7: the machine-wide scan skips fenced claims when the dispatch is for another goal; dispatch against the fenced goal itself stays refused by the per-goal revision admission; fixture for the fenced-sibling case; the live proof becomes claim AND dispatch while fenced |
| BSW-13 | low, not material | Two stopped claims and no live claim is now reachable; uniqueActiveProofGoal and the steward's OnlyFencedClaim then fall back to their generic sentences (both require exactly one fenced claim); outcomes still right | note; the generic sentences are true, only less exact |
| BSW-14 | low, not material | With only a stopped claim, the delegate prompt omits its Serving goal block silently and dispatch --serving-goal refuses without mentioning the fence; behaviour correct, wording only | note |

The critic's gaps: it did not run conformance review itself (it used the
round-6 review record); it ran the three command-package chain fixtures on a
scratch copy, not the full packages or beds (the orchestrator ran those green
outside the sandbox); BSW-12 is proven by a scratch Go test against
EvaluateGoalAdmission, with dispatch.sh's path read, not run; BSW-12 assumes
the same owner lineage holds both claims, which is the uninterrupted-seat
case this goal exists for; trunk had moved eighteen commits past the chain
base, none under cmd/ or internal/.
