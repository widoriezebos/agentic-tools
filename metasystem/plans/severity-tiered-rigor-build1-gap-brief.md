Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal severity-tiered-rigor)
Date: 2026-09-03

# Goal

Answer the gap you reported on chain str-build1 and build part one per
metasystem/plans/severity-tiered-rigor-build1-brief.md. The gap: the
review-round ceiling had no configuration key. Here is the contract;
nothing else in the brief changes.

# The two configuration keys (both in metasystem/metasystem.conf, both
validated by `metasystem config validate`)

1. `metasystem.budget.review-round-max=3` — the ceiling on
   `ReviewRoundLimit`. `Budget.Validate` refuses a `ReviewRoundLimit`
   above it. The literal value `0` means NO ceiling: the token raise of
   design point 03 applies and `Validate` accepts any non-negative
   value. Default 3, which is the design's recommended option (A) for
   decision 06; Wido's word "raise" becomes the one-line change to 0.
   Tier boxes stay 0, 2, 3 rounds for tiers 1, 2, 3 and must each be
   at or below the ceiling when it is non-zero (validate refuses a conf
   whose box exceeds the ceiling).
2. `dispatch.cap-max=120` — the enforced maximum reservation cap in
   minutes, applied by `ResolveCap` (internal/dispatch/cap.go) to every
   source including the explicit argument; a cap above it refuses,
   naming the key. Tier box minutes are the tier's attempts times this
   value (360, 720, 1200 for tiers 1, 2, 3). This is the design's
   recommended option (A) for decision 14. The mission fence is out of
   scope for part one (obligation STR3-MISSION-CAP-BYPASS-07 names the
   test; implement the test as skipped-with-reason if the fence path
   cannot be enforced in part one, and say so in the return).

Both keys carry a one-line comment in metasystem.conf naming the
decision and the ruling row that set it (R-42-m0 for the ceiling;
R-58-m1 for the pool as runaway guard). No other new keys.

# Everything else

As in metasystem/plans/severity-tiered-rigor-build1-brief.md: the
change list, the four binding test obligations, the gate, the
boundary rule, the wall-clock budget (90 minutes from now), and the
gap rule.
