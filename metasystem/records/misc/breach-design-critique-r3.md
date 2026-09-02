# Sol design-critique of the breach-machinery design — round 3 (2026-09-02)

Job breach-design-crit3 (design-critic, codex gpt-5.6-sol), reviewed commit
2a072390, design revision 3 (plans/breach-clock-and-budget-honesty-design.md),
brief plans/breach-clock-critique-r3-brief.md. One material finding; the
day-token inventory was found complete, the rollout table complete against
every `budgetTuple` and `goal.NewBudget` caller, the two norm rows' intent
confirmed to survive in hours, and Fix 3, Fix 1's record changes and Fix 2's
decision and migration unchanged. Full return:
artifacts/agents/breach-design-crit3/rounds/1/return.json.

## BCD-R3-001 (high, material)

The held obligation rule still permits a favorable-direction clock movement
at a raise, so it does not fully close BCD-R1-003. Sequence: discharge at
T0+3h under obligation revision 5; a human set-obligation installs revision
7 (the revision-5 proof is now excluded, start = T0); then set-budget. The
raise clears the live obligation (verbs.go:122-124) and the design's
nil-obligation rule ("no obligation-revision filter applies") makes the old
proof eligible again, moving the start forward to T0+3h — negating the
human's revision-7 supersession and postponing a breach. The proof plan
tests discharge→raise→set-obligation but not discharge→set-obligation→raise.

Evidence: design lines 292-299 and 314-321 ("A raise leaves the start
exactly where it was the moment before"); verbs.go:594-621 installs the
fresh obligation revision; the test at design lines 808-816.

## Orchestrator decision (m0b, 2026-09-02 18:50Z), folded by revision 4

A raise must carry forward WHICH obligation was live the moment before, so
the nil-obligation rule can keep excluding what the human superseded.
`rebindClaimKeepEpisode` records the cleared obligation's revision on the
claim binding as a third episode key, `episodeObligationRevision` (0 when no
obligation was live, and then the key is absent); `bindClaim` and
`clearClaimBinding` reset it. With `file.Obligation == nil` a proof is
eligible only when `obligationRevision == Claimed.EpisodeObligationRevision`
and that value is non-zero; zero means no proof counts and the start is the
episode origin, which is today's no-obligation reading. A later
set-obligation installs a live revision and the live filter takes over; the
next raise records that revision. Under this rule a raise never moves the
start in either direction, and set-obligation keeps its shipped meaning.

## Orchestrator addendum (m0b, 2026-09-02 19:20Z), after revision 4

Revision 4 (job breach-design-r4) folded the decision above as worded and
reported one gap: discharge → raise → raise. With "0 when no obligation was
live" the second raise writes key 0 and rewinds the start to the episode
origin, the very shape BCD-R1-003 named. Decision: INHERIT. When no
obligation is live at the raise, `rebindClaimKeepEpisode` carries the prior
claim binding's `episodeObligationRevision` forward unchanged (0 stays 0 only
when it was 0). Then every raise reproduces the filter that governed the
moment before it, and the invariant "a raise never moves the start in either
direction" holds for all five sequences. Folded by revision 5.
