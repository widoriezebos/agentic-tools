Working Mode: implement
Orchestrator Identity: m1b+main-1788333346-60696-6a3256 (dispatch delegate under goal dispatch-cap-necessity)
Date: 2026-09-02

# Goal

Goal dispatch-cap-necessity, highest priority by Wido's word
(R-49-m1b, R-51-m1b in metasystem/memory/rulings.md): the reservation
accounting bug. Implement the ACCEPTED box of the design
metasystem/plans/dispatch-cap-settlement-design.md (revision 4) as
delimited by metasystem/plans/dispatch-cap-settlement-scope-cut.md,
which is part of this brief: critique chain cap-settle-crit closed at
round 4 with the loop's stop recorded
(metasystem/plans/dispatch-cap-settlement-dispositions-r4.md; earlier
rounds in metasystem/plans/dispatch-cap-settlement-dispositions.md,
metasystem/plans/dispatch-cap-settlement-dispositions-r2.md,
metasystem/plans/dispatch-cap-settlement-dispositions-r3.md). A job
that has ended charges the minutes it actually ran, measured from the
launcher's ownership proof time to its end stamp, rounded up and
clamped to its cap; a job still open charges its cap; a job that never
launched charges nothing; `endedAt` becomes transition-owned; every
budget refusal prints the reserved line with both numbers.

# Workspace

The dispatch-created job worktree, branched from main. The design's
in-box sections bind exactly as written — ruling R-25b-m1: the design
is carried whole within the box; any deviation, simplification, or
scope cut you find necessary is a GAP to report, never a silent
choice. The scope cut file names exactly what is OUT: design section
4.3 (the conclusion-time re-projection, `ProjectSpend`,
`NewConcludingRunStore`, `ErrNoSpendProjection`) with tests T13 and
T15, and section 1.9 (the lease sweep's death ladder) with test T14.
Do not build them; they are goals governed-exhaustion-reprojection and
lease-sweep-death-evidence.

Expected touched paths (declare every touched path in diffBoundary WITH
the metasystem/ prefix): metasystem/internal/dispatch/budget.go (the
charge rule at the one charging site, `settledJobMinutes`,
`recordHasProcessIdentity` extracted and shared, the amended
post-discharge filter, the two projection fields,
`reservedMinutesEvidence`; sections 1-4.2),
metasystem/internal/dispatch/reapfacts.go (calls the extracted
predicate, no behaviour change), metasystem/internal/dispatch/record.go
(the `endedAt` patch refusal, section 1.5),
metasystem/internal/dispatch/admission.go (`Reserved` on the refusal,
`formatRefusalDetail`, the renderer; section 4.1),
metasystem/internal/dispatch/governed.go (the refusal text through the
shared helper; section 4.1; `ReservedBefore` untouched),
metasystem/internal/dispatch/budget_test.go (the fixture extension and
tests T1-T9 and T12, plus the exact changes to existing tests listed
in section 5), metasystem/internal/dispatch/admission_test.go (T10),
metasystem/internal/dispatch/record_test.go (T11),
metasystem/internal/dispatch/governed_budget_coverage_test.go and
metasystem/internal/obligationstate/state_test.go (the component
assertions section 5 names). Nothing in section 7 (unchanged) and
nothing in the cut sections moves: not run/conclude.go, not
run/run.go, not lease/sweep.go, not supervise/arming.go, not
cmd/metasystem.

# What binds (by design section)

- §1 (1.1-1.8): open records charge the cap; no process identity
  charges 0; a launched terminal record charges observed minutes from
  `ownershipProof.provenAt` (revision 3's rule 1.3, restored by the
  scope cut: fallback `startedAt` only when `ownershipProof` is absent
  or its `provenAt` empty; a present, non-empty, unparseable `provenAt`
  is unknownBudget) to `endedAt`, whole seconds rounded up to the
  minute, floor 1, clamped to `capMin`; the post-discharge filter reads
  `startedAt` else `createdAt`; unreadable or reversed timestamps on a
  launched terminal record are unknownBudget with the named reasons;
  RecordCAS refuses any patch carrying `endedAt` with the exact
  message; no new configuration key.
- §2: no new field written anywhere; the figure is computed from the
  record on every projection.
- §3: `ObservedJobMinutes` and `OpenCapMinutes` with
  `ReservedJobMinutes` their sum, governed components included; the
  overflow guards on each addition.
- §4.1-4.2: the per-limit breach objects and their texts unchanged;
  `Reserved` evidence on every refusal with a known projection; the
  renderer appends `; reserved observed=<n> open-caps=<m> limit=<L>`;
  the governed refusal carries the same line through the shared
  helper; the consumer table row by row except the 4.3 row, which is
  out.
- §5: the fixture extension's exact shape; tests T1-T12 by name and
  assertion (T12 as the provenAt-versus-startedAt case: the charge
  follows the proof time); the listed existing tests changed exactly
  as stated and no other assertion touched; internal/dispatch at or
  above its coverage floor (metasystem/scripts/agents/coverage-ratchet.json).
- §8: the reject condition is a stop condition — a lawful writer
  patching `endedAt` on an open record, a terminal writer that stamps
  nothing, a writer clearing `pid`, or a bed asserting the bare refusal
  line: STOP and report the gap.

# Constraints

- KNOWN SANDBOX LIMIT: the full validation suite needs real process
  visibility your sandbox denies; run the focused proofs named below and
  report anything environment-limited as such, never faked.
- No test weakened; gofmt and go vet clean.
- Wall-clock budget: 60 minutes.

# Expected Return

Version-2 implementer JSON; complete diffBoundary; evidence commands
replayable from the worktree root including:
- `go build ./...`
- `go test ./internal/dispatch/ -count=1`
- `go test ./internal/obligationstate/ -count=1`
- `gofmt -l internal/dispatch internal/obligationstate`, `go vet ./internal/dispatch/ ./internal/obligationstate/`

# Gap Rule

stop and report a gap; never fill it silently — especially anywhere
the design underdetermines an implementation choice.
