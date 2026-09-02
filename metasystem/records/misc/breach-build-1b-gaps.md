# Sol build breach-build-1b: three Fix 3 gaps and the orchestrator's decisions (2026-09-02)

Job breach-build-1b (implementer, codex gpt-5.6-sol, cap 120) read design
revision 5 of plans/breach-clock-and-budget-honesty-design.md against the
tree and stopped at three mechanically unresolved Fix 3 conflicts. No bytes
written, no commit, the gate not run. Return:
artifacts/agents/breach-build-1b/rounds/1/return.json. Sol's riskiest-part
note: "Fix 3 would prevent the hard breach fence from closing on a member of
a claimed multi-goal arc, so implementing it as written could weaken the
breaker itself." That is the finding that matters; the ladder caught it
before a line of Go.

## Gap 1 — the state model and the package rule disagree on resume from the claimed shape

The state model says "either shape → queued by goal resume"; the package rule
(a) says a claimed-and-fenced goal follows today's path and stays claimed
with a fresh episode. Today's `resumeRequest` (internal/goal/stop.go:378-411)
installs the fresh budget and calls `bindClaim` on the same machine and
lineage: the goal stays claimed.

Decision (m0b, 19:00Z): the package rule is right, the state model sentence
is wrong. From the claimed shape, resume keeps the claim and starts a fresh
episode (State: claimed); from the parked shape, resume binds no claim
(State: queued). Revision 6 rewrites the transition sentence.

## Gap 2 — the only-claim tree invariant blocks CloseStop on an arc member

The quota (internal/goal/validate.go:250-283) lets one machine claim every
member of ONE arc; those claims count once. The revision-5 invariant ("a
fenced claim is the machine's only claim … same arc or not") is enforced by
`ValidateCommit` on every publication, so `CloseStop` writing a fence on one
member while the same pair holds a sibling would fail its own commit, and
the design supplies no atomic sibling transition.

Decision (m0b, 19:00Z): the invariant's unit is the QUOTA's unit. Restated:
for each machine, if any claimed file carries a `StopFence`, the machine
holds no claim outside that goal's arc — an unarced fenced goal admits no
other claim at all; an arced fenced goal admits claims only on members of
its own arc (which the quota already counts as one claim). Wording:
`"machine %s claims %s outside the arc of %s, which is breach-stopped by %s; a fenced claim admits no claim beyond its own arc until it is released or parked into parked-with-breach"`.
Consequences: `CloseStop` on an arc member never conflicts with its
siblings; a different-arc or unarced second claim beside a fence is still
refused everywhere (claim, open-claim, cascade, steal, arc move, reopen,
reconcile), which is the whole of BCD-R1-004's case (the quota never
admitted those); the orientation branch (goal.go:469-472) checks EVERY claim
of the machine for a fence, not only `Claimed[0]`, and prints the way out
naming the fenced goal; release of the fenced member is lawful and leaves
its siblings claimed and workable. Not chosen: an atomic sibling release in
`CloseStop` (it would punish goals that did not breach and orphan their
jobs) and exempting the breaker's publication from the invariant (a tree
invariant that some commits may violate is not an invariant).

## Gap 3 — internal/steward/delivery.go constructs the claimed set and the table omits it

`delivery.go:56-79` builds this machine's claimed set for the
claimed-goal-delivery health role (no automatic remedy, delivery.go:297-301)
and judges each claim's landing evidence against its elapsed limit.

Decision (m0b, 19:00Z): UNCHANGED, and the table says so. A claimed-and-
fenced goal stays in the set and its verdict reads as today (it has failed
to deliver; the role saying so is true, and the role acts on nothing); a
parked-with-breach goal leaves the set as any parked goal does, so the
machine's health no longer carries the stopped goal once it is released.
The role reads `Budget.ElapsedLimit` through the legacy reader
(`goal.ParseWorkingDuration`), which Fix 2 keeps for stored tokens; revision
6 adds the row to the consumer table and confirms the reader is in the
day-token inventory's legacy-reader class.

## Next

Revision 6 (Fable, brief plans/breach-clock-revision-r6-brief.md) folds the
three; Sol round 5 judges Gap 2's rule by id; then the build. Budget note:
the re-claim's attempt counter stands at 3 of 6 after this build; the four
remaining rungs need one more attempt than the tuple allows, so the code
critique waits on Wido's word to raise attemptLimit.
