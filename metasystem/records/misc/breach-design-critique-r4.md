# Sol design-critique of the breach-machinery design — round 4 (2026-09-02)

Job breach-design-crit4 (design-critic, codex gpt-5.6-sol), reviewed commit
d380e22b, design revision 5 (plans/breach-clock-and-budget-honesty-design.md),
brief plans/breach-clock-critique-r4-brief.md. ZERO material findings; the
design is certified for build at revision 5. Full return:
artifacts/agents/breach-design-crit4/rounds/1/return.json.

## What Sol checked

- BCD-R3-001 closed by id: the five stated sequences traced against
  verbs.go and budget.go — discharge→raise stays T0+3h;
  discharge→set-obligation returns to T0 without a raise;
  discharge→set-obligation→raise stays T0; discharge→raise→set-obligation→raise
  stays T0; discharge→raise→raise stays T0+3h because the second raise
  inherits obligation revision 5. No raise moves the start in either
  direction.
- A sixth sequence Sol constructed (set-budget, set-obligation, discharge,
  release, reclaim, set-budget): release clears Claimed and Obligation,
  reclaim starts a fresh episode with the third key at zero, the old proof
  is excluded by the episode revision, and a later raise inherits zero and
  leaves the start at the reclaim time. Release-and-reclaim intentionally
  starts a new clock; neither raise moves it.
- `bindClaim` and `clearClaimBinding` (verbs.go:108-142) and every call
  site: only `SetBudget`'s rebind goes through `rebindClaimKeepEpisode`;
  every genuine claim start goes through `bindClaim`, so a fresh episode
  starts the key at zero and a raise never overwrites a non-zero key with
  zero.
- The Claimed grammar (file.go:76-84, 378-394, 526-543, 767-772): the key
  renders only with the episode binding, refuses a missing binding with the
  named wording, refuses zero or malformed present values, and
  `ValidateClaimRevision` gains the non-zero-implies-episode-binding rule;
  legacy absence stays distinguishable from a present key.
- reconcilemap.go:220-255 compares the whole ClaimRecord, so the new key
  adds no field-level hand-edit surface.
- Proof plan: one named projection test per sequence including the second
  raise, plus legacy anchoring, fresh release-and-reclaim episodes, grammar
  round trips, the third-key parse refusal and the validation
  contradictions.
- No regression: revision 5 changes only the inheritance decision and its
  consequences; Fix 3 whole; Fix 2's refusal-of-new-day-tokens decision,
  legacy reader, unfenced-only migration, day-token inventory and writer
  rollout table intact.

Gap reported (not material): the catalogued design-critique skill file was
absent in the sandbox; the brief carried the binding text, applied directly.

## Orchestrator decision (m0b, 2026-09-02 18:45Z)

Certified. The build proceeds from plans/breach-clock-build-brief.md against
revision 5. The one point that stays OPEN FOR WIDO and is NOT built: whether
a later human set-obligation should inherit a discharge consumed inside the
same claim episode (today's meaning, kept by the design: it does not).
