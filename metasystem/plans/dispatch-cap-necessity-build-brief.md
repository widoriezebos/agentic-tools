Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal dispatch-cap-necessity)
Date: 2026-09-09

# Goal

Goal dispatch-cap-necessity, highest priority by Wido's word (R-49-m1b,
R-51-m1b in metasystem/memory/rulings.md): reservation caps charge
budgets for time never run. Build the ACCEPTED box of the design
metasystem/plans/dispatch-cap-settlement-design.md (revision 4) as
delimited by metasystem/plans/dispatch-cap-settlement-scope-cut.md,
exactly as the landed build brief
metasystem/plans/dispatch-cap-settlement-build-brief.md specifies: that
brief is part of this one, word for word, in its sections "Workspace",
"What binds (by design section)", "Constraints", "Expected Return" and
"Gap Rule". Read it first; this page only names what has changed since
it was written on 2026-09-02.

# What has changed since the landed brief

- The seat is m1 (this page's identity), not m1b; the chain is the one
  this dispatch creates, branched from today's main. The design's line
  numbers for internal/dispatch/budget.go, record.go, admission.go and
  governed.go were read on 2026-09-02; find the same sites by their
  names (`settledJobMinutes`, `recordHasProcessIdentity`,
  `reservedMinutesEvidence`, `formatRefusalDetail`,
  `terminalStateContradiction`) rather than by line.
- Test T10 goes in a new file beside admission.go in
  internal/dispatch, named for admission, since none exists today;
  every other touched path the landed brief lists exists.
- Wido approved the goal at the enrolled terminal on 2026-09-06
  (revision 12), after the scope cut rose to him; the cut stands as
  written. Sections 4.3 and 1.9 stay out (goals
  governed-exhaustion-reprojection and lease-sweep-death-evidence).
- Sequence of proof after your return: the orchestrator runs
  conformance, the dispatch and obligationstate packages, the coverage
  ratchet and the hook suite on the seat; one Opus code review follows
  (the landed brief said Fable; the roster today puts Opus on code
  critique); fold rounds as needed; land with --chain.

# Constraints

As the landed brief, with one change: wall-clock budget 45 minutes.
Report the round as your own.
