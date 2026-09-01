# Design: the breach machinery stops lying

> HANDOVER NOTE (m2, 2026-09-01): this design is UNCERTIFIED. Sol's
> design-critique returned EIGHT MATERIAL findings against it (chain
> breach-design-crit, round 1, folded into the register); the revision
> round never dispatched because the goal's job-minute pool was spent.
> It lands here so the successor inherits the work rather than a
> worktree path. DO NOT BUILD FROM IT until the revision closes those
> findings and a re-critique confirms at zero material.



Goal: breach-clock-and-budget-honesty (plans/goals/breach-clock-and-budget-honesty.md,
revision 3). Author: Fable-lane designer, 2026-09-01. Standard (Wido, verbatim):
"hard deterministic machinery. This is Go territory enforcing your behaviour" —
every mechanism below is engine-enforced; nothing here is conduct guidance.

Evidence base: the goal record's Intent (five lawful clock resets in one night),
records/misc/idle-loss-2026-09-01.md (m3 frozen whole by one breach), and the
file-and-line traces cited inline. Every cited seam was read in this worktree.

## Scope and non-goals

Three proven defects, three engine fixes:

1. **Raise-reset clock**: every budget raise restarts the breach elapsed clock.
2. **Dishonest durations**: budget elapsed limits parse in working hours
   (d = 8h) and are normalized on input, silently shrinking human intent.
3. **Breach wedges the machine**: a breach-stopped claim refuses release (and
   park, and done) and still consumes the one-claim-per-machine quota.

Non-goals, with the seam boundaries stated so they cannot collide:

- **The steward tripwire (m0's goal, burn-without-delivery-tripwire).** This
  design changes only ledger state machinery in internal/goal, the elapsed
  projection in internal/dispatch/budget.go, and the duration grammar in
  internal/goalbudget. It adds NO alerting, notification, or observation of a
  breach — who gets told that a fence closed or a goal parked-with-breach is
  entirely m0's seam. The parked-with-breach state this design introduces is a
  ledger fact m0's tripwire may later observe; this design does not consume or
  emit any tripwire signal.
- **Failed-job-attention (m3's goal).** Job records, the reaper, delivery of
  job-failure facts, and the wait primitive are untouched. The one shared
  object is the quota invariant in internal/goal/validate.go, which this design
  changes only by excluding fenced claims (Fix 3); nothing about job lifecycle
  or attention changes.
- **Attempt and reserved-minute accounting across raises.** ProjectBudget
  filters reservations by claim revision (internal/dispatch/budget.go:350
  `recordRevision != revision → continue`), so a raise also restarts attempt
  and job-minute counting. That is a sibling honesty defect, but the goal and
  brief scope Fix 1 to the elapsed breach clock. Named here as residue, not
  silently filled; it needs its own goal.

## Traced facts (the seams as they are today)

- `bindClaim` (internal/goal/verbs.go:113) writes a fresh `ClaimRecord`
  (`Machine, Lineage, At, Revision` — internal/goal/file.go:76) and a fresh
  `StopCapability`, and is called from nine sites: claim (verbs.go:475),
  set-budget (verbs.go:540), reopen-into-claimed-arc (verbs.go:1002), steal
  (verbs.go:1242), open-claim (verbs.go:1309), arc claim cascade
  (verbs.go:1574), arc move into a claimed arc (verbs.go:2042), the reconcile
  equivalent (internal/goal/reconcilepub.go:515), and resume
  (internal/goal/stop.go:396).
- `ProjectBudget` (internal/dispatch/budget.go:237) parses `file.Claimed.At`
  (budget.go:258) as the elapsed origin; `obligationBudgetStart`
  (budget.go:77) may move the origin later via a consumed discharge proof;
  `CLOCK_REGRESSED` fires at budget.go:266-268 when `now` precedes the origin.
  `ValidateClaimRevision` (internal/goal/file.go:328) pins `Claimed.At` to
  `History[Claimed.Revision-1].At`.
- `goalbudget.ParseWorkingDuration` (internal/goalbudget/budget.go:14) reads
  `d` as `8 * time.Hour` (budget.go:38); `New` (budget.go:80) normalizes input
  through `FormatWorkingDuration` (budget.go:88), so "24h" is stored as "3d"
  and a human's "9d" is enforced at 72 clock hours. The grammar's complete
  non-test consumer set (verified by repo-wide grep for
  ParseWorkingDuration/FormatWorkingDuration/ElapsedDuration): the goalbudget
  package itself; the re-export wrapper internal/goal/budget.go; the stored
  Budget record parse `parseBudgetRecord` (internal/goal/budget.go:35, which
  stores ElapsedLimit VERBATIM and only validates); journal-replay
  `budgetFromIntentArgs` (internal/goal/budget.go:81, which re-normalizes via
  NewBudget — a replay hazard); the CLI entry
  cmd/metasystem/goalsync_mutations.go:172; stop firing evidence
  `validateStopFiringEvidence` (internal/goal/stop.go:145-157); admission
  (internal/dispatch/admission.go:176); the projection
  (internal/dispatch/budget.go:517); governed-run exhaustion
  (internal/run/conclude.go:315-318, via the Budget snapshot embedded in the
  run record); and metrics spend (internal/metrics/compute.go:294). Job caps
  (`capMin`) and slice/goal norms are integer MINUTES everywhere
  (internal/goal/norm.go, `reservedJobMinutesLimit`) and never touch this
  grammar — the change is scoped to budget elapsed limits by construction.
- `clearClaimBinding` (verbs.go:128) refuses whenever `StopFence != nil` with
  "goal %s is breach-stopped by %s; only goal resume may clear its launch
  fence", and is called from 13 sites: release (verbs.go:650), done
  (verbs.go:756), park (verbs.go:848), four arc cascade/detach sites
  (verbs.go:1631, 1706, 1830, 1985), split parent decomposition
  (internal/goal/split.go:292), and four reconcile-replay sites
  (reconcilepub.go:270 hand-park, :319 hand-done, :428 and :468 arc
  detach/join). The quota is enforced tree-wide at
  internal/goal/validate.go:250-283; its verbatim rejection is "machine %s
  claims %s: the quota is one claim per machine (one arc counts once)". The
  parse invariant at file.go:239 refuses stop authority on any non-claimed
  goal, while the hand-edit mapper already anticipates a park that retains
  stop authority (reconcilemap.go:141-144 tolerates exactly those two
  diagnostics; reconcilemap.go:238-241 `stopRetainedByPark`).
- Resume (stop.go:355-403) is human-only with a live authority proof, demands
  the stop batch COMPLETE (`VerifyStopBatchComplete`, stop.go:249), installs a
  fresh complete budget, and re-binds the claim to the same pair.

## Fix 1 — the raise-proof breach anchor

### Decision

The claim EPISODE is the span from the moment a pair takes ownership until
that ownership genuinely ends. A budget raise re-binds the claim revision (the
reservation boundary stays exactly as the comment at verbs.go:484-486 intends)
but the episode origin survives it. A new episode starts only when ownership
genuinely restarts: claim, open-claim, arc claim cascade, steal, an arc move
that creates a claim, reopen-into-arc — and resume. Resume is the explicit
human re-time the goal record names ("only release-and-reclaim or an explicit
human re-time restarts it"): it is human-only, proof-gated, requires the
completed stop batch and a fresh complete budget, and is recorded as its own
history verb. The task brief's phrase "only genuine release-and-reclaim starts
a new episode" is read together with that recorded human word; a resume that
preserved the episode would force the human to do elapsed arithmetic at the
console for no integrity gain, because the breach record and history already
carry the true span. This is a designer decision, recorded here.

### Record and schema changes

`ClaimRecord` (file.go:76) gains two fields:

```go
// EpisodeAt and EpisodeRevision pin the moment this ownership episode
// began; a budget raise re-binds At and Revision but never these.
EpisodeAt       string
EpisodeRevision uint64
```

- **Grammar.** The `Claimed:` line's closed key set (file.go:480) gains the
  optional keys `episodeAt` and `episodeRevision`, both-or-neither. Render
  (file.go:684-690) appends ` episodeAt=%s episodeRevision=%d` when
  `EpisodeRevision > 0`. Absent keys mean a legacy record: the episode is the
  claim binding itself (anchor = `Claimed.At`). Discrimination is the key's
  presence — a version marker, never a heuristic.
- **Validation.** `ParseFile` adds the problem
  `"Claimed episode binding is incomplete (episodeAt and episodeRevision travel together)"`
  when exactly one key is present. `ValidateClaimRevision` (file.go:328) is
  extended: when `EpisodeRevision > 0` it additionally requires
  `EpisodeRevision <= Claimed.Revision`,
  `EpisodeRevision <= uint64(len(History))`,
  `History[EpisodeRevision-1].At == EpisodeAt`, and parsed
  `EpisodeAt <= Claimed.At`. Failure wordings, exact:
  - `"claimed episodeRevision=%d is later than claim revision=%d"`
  - `"claimed episodeAt=%s contradicts History revision=%d at=%s"`
  - `"claimed episodeAt=%s is later than claimed at=%s"`
  These surface through the existing `BUDGET_UNKNOWN %v` prefix (file.go:254),
  so structured admission refuses rather than guesses.
- **Mechanics.** `bindClaim` keeps its signature and always writes a FRESH
  episode (`EpisodeAt = at`, `EpisodeRevision = revision`). One new function:

  ```go
  // rebindClaimKeepEpisode re-binds the claim to a new revision while the
  // ownership episode's origin survives. Only a raise uses it.
  func rebindClaimKeepEpisode(f *GoalFile, at string, revision uint64, claimEpoch int64) error
  ```

  It inherits machine and lineage from `f.Claimed`, and inherits
  `(EpisodeAt, EpisodeRevision)` from the prior record when set, else from the
  prior `(At, Revision)` — the legacy-inheritance rule: the first raise after
  deployment pins the anchor at the pre-raise claim moment (raises that
  already happened before deployment are unrecoverable; the engine never mines
  history heuristically for an older one). Everything else (`StopCapability`
  minting, `StopFence = nil`, `Obligation = nil`) matches `bindClaim`.
  Call-site rule, mechanical: verbs.go:540 (set-budget) is the ONLY site that
  switches to `rebindClaimKeepEpisode`; the other eight `bindClaim` sites are
  unchanged (fresh episode).
- **Projection.** In `ProjectBudget` (budget.go:237-282): parse `episodeAt`
  from `Claimed.EpisodeAt` when `EpisodeRevision > 0`, else `Claimed.At`
  (malformed stamp → the existing unknownBudget path with
  `"the claim episode timestamp is malformed"`). Every elapsed-origin use of
  `claimedAt` becomes `episodeAt`: the `obligationBudgetStart` base and its
  `!consumedAt.Before(...)` filter (budget.go:77, 108, 134), the
  `now.Before(...)` regression check (budget.go:266), `StartedAt`/`Elapsed`
  (budget.go:280), and the two post-discharge comparisons
  `budgetStartedAt.After(claimedAt)` (budget.go:353, 442, 480). The
  revision-binding filters (`recordRevision != revision`, proof
  `goalRevision == file.Claimed.Revision`) are untouched — see the residue
  non-goal above. Governed dispatch snapshots `BudgetStartedAt` from this
  projection, so internal/run/conclude.go:315-318 is correct for new runs with
  no change of its own.
- **CLOCK_REGRESSED** now means: the observation precedes the episode origin
  (or the consumed discharge proof that lawfully moved the start later). Exact
  wording, replacing budget.go:267:
  `"CLOCK_REGRESSED: the claim episode origin is later than the observation"`.
  It remains a typed refusal to project, never a zeroed clock.
- **How recovery reads it.** Recovery and reconcile replay through the real
  verb builders (recover.go, reconcilepub.go), so the episode is reproduced by
  the same mutation code, never re-derived: a replayed claim writes a fresh
  episode, a replayed set-budget inherits through `rebindClaimKeepEpisode`
  against the tree at its tip. The hand-edit surface already refuses any
  altered `Claimed` line as a generated field (reconcilemap.go:234-236), which
  makes the episode keys tamper-proof there with zero new code.

## Fix 2 — honest duration grammar for budget elapsed limits

### Decision

For budget elapsed limits: `m` = 1 minute, `h` = 1 clock hour, `d` = 24 clock
hours. Input is stored VERBATIM — the engine never rewrites a human's token.
Every stored budget carries a grammar version marker; a record without the
marker is a legacy record and keeps its original working-hours semantics
forever. Discrimination is the marker, never a heuristic.

### Record and schema changes

- `goalbudget.Budget` (internal/goalbudget/budget.go:73) gains
  `ElapsedGrammar string \`json:"elapsedGrammar,omitempty"\`` with exactly one
  legal non-empty value, `"clock"`. The struct stays comparable (the
  `*f.Budget == budget` check at verbs.go:525 keeps working; a legacy "3d"
  re-set as clock "24h" compares unequal, which is correct — it is a new
  record era).
- New parser in goalbudget:

  ```go
  // ParseClockDuration reads the honest budget grammar: m is a minute,
  // h a clock hour, d exactly 24 clock hours.
  func ParseClockDuration(value string) (time.Duration, bool)
  ```

  Same token walk and overflow guards as ParseWorkingDuration; only the `d`
  unit differs (`24 * time.Hour`).
- `New` (budget.go:80): validates with `ParseClockDuration`, stores
  `ElapsedLimit: elapsedLimit` VERBATIM, sets `ElapsedGrammar: "clock"`. The
  `FormatWorkingDuration(elapsed)` normalization at budget.go:88 is deleted.
  Refusal wording, exact:
  `"elapsedLimit %q is not a positive clock duration (for example 4h, 24h, or 2d; d is 24 clock hours)"`.
- `Validate` (budget.go:92) dispatches: `ElapsedGrammar == "clock"` →
  ParseClockDuration; `""` → ParseWorkingDuration (legacy); anything else →
  `"elapsedGrammar %q is not a recognized duration grammar (absent means legacy working-hours; \"clock\" means clock semantics)"`.
  `ElapsedDuration` and `ElapsedBreachDuration` dispatch identically, so
  admission.go:176, budget.go:517, conclude.go:318, and compute.go:294 are
  correct through the methods with no local change.
- **Goal-file grammar.** The `Budget:` record (parse file.go:400/budget.go:35,
  render file.go:641-644) gains the optional key `elapsedGrammar`; render
  writes it when non-empty. `parseBudgetRecord` accepts only `clock` or
  absence (Validate enforces).
- **Journal replay.** `budgetIntentArgs` (budget.go:72) adds `elapsedGrammar`
  when non-empty. `budgetFromIntentArgs` (budget.go:81) must STOP calling
  `NewBudget` — replaying a legacy journal entry through New would re-mark and
  re-parse it under clock semantics. It constructs the Budget verbatim from
  the stored args (grammar field included, absent = legacy) and calls
  `Validate`. This is the one replay-determinism trap in the package; the
  proof plan pins it.
- **Stop firing evidence.** `StopFiringEvidence` (stop.go:55) gains
  `ElapsedGrammar string \`json:"elapsedGrammar,omitempty"\``;
  `validateStopFiringEvidence` (stop.go:145) builds
  `Budget{ElapsedLimit: evidence.AdmissionLimit, ElapsedGrammar: evidence.ElapsedGrammar}`
  and uses its methods for both the admission parse and the boundary
  recomputation. Absent field = legacy batch = working-hours check, unchanged
  bytes, still valid.
- `FormatWorkingDuration` survives for legacy display only; nothing writes it
  into a record anymore. docs/backlog-mechanism.md:18 ("one working day is
  eight hours") is updated to state the clock grammar for budget elapsed
  limits and the legacy-marker rule.

### Migration

- **Engine part: none beyond the dual grammar.** Legacy stored strings (goal
  files, journal args, run-record Budget snapshots, stop batches) stay
  readable and ENFORCEABLE under their original working-hours semantics for
  their whole lifetime, discriminated by the absent marker. No rewrite pass
  exists, so there is no rewrite to crash.
- **Operational part (the seat's, not the implementer's):** after Fixes 1 and
  2 are deployed on every machine, the seat re-sets each live budgeted goal
  with `goal set-budget`, using the human's recorded verbatim word, verified
  against that goal's history and journal (the normalized journal args are NOT
  the human's word — the verbatim intent lives in the human's recorded
  rulings and goal history reasons; where no verbatim word is recoverable, the
  seat asks Wido rather than converting arithmetically). Ordering matters and
  is guaranteed by Fix 1: a grammar re-set is a raise/re-bind and must not —
  and with Fix 1 cannot — reset the breach episode.
- **Rollout ordering across machines.** `parseKVRecord`'s closed key set means
  an OLD binary reading a new `Claimed:` or `Budget:` line refuses loudly
  ("unknown key %q — the record grammar is closed", file.go:566) — it can
  never misread clock semantics as working hours. Deploy the new binary on
  both machines before the first new-grammar write; the fail-closed refusal is
  the guard if that ordering slips.

## Fix 3 — a breach parks the goal, never the machine

### Decision and state model

A breach-stopped-then-released goal IS **parked-with-breach**: `State: parked`
with a `Parked` record, `Claimed` cleared, and the complete
`StopCapability` + `StopFence` pair retained as the breach record awaiting the
human's word. It is not claimable (parked), not budget-projectable (no claim),
not launchable (fence), and costs its former claimant nothing. This is exactly
the shape the hand-edit mapper already anticipates
(reconcilemap.go:141-144, 238-241); the executable layer catches up to it.

Resume gains a second lawful shape. The human's word on a parked-with-breach
goal is either `goal resume` (fresh budget, back to queued) or a human
`goal done` (concluded; the breach record closes with the goal).

### Verb-by-verb mechanics

- **Release (verbs.go:622-658).** When `f.StopFence != nil`: instead of
  refusing, set `State: parked`, write
  `Parked{By: actor, At: stamp, Because: "breach-stopped by <stopId>; awaiting the human's word (goal resume or a human goal done)", Displaced: <as computed today>}`,
  clear `Claimed` and `Obligation`, KEEP `StopCapability` and `StopFence`.
  History verb stays `release`. One shared helper owns this demotion:

  ```go
  // demoteToBreachPark releases ownership of a fenced claim into
  // parked-with-breach: the goal waits for the human, the machine goes free.
  func demoteToBreachPark(f *GoalFile, by, at, because, displaced string)
  ```

- **Park (verbs.go:806-861).** When fenced: same demotion via the helper,
  keeping the caller's `because`. (Park of a foreign claim stays a human act,
  unchanged guard at verbs.go:836.)
- **Done (verbs.go:711-770).** When fenced and the actor is an agent, refuse:
  `"goal %s is breach-stopped by %s; concluding over a standing breach is a human act"`.
  When fenced and the actor is human, clear `StopCapability` and `StopFence`
  in the done transition (the human's conclusion IS the later word; the stop
  batch file under artifacts/agents/goal-stops/ remains as local history).
  Done on a parked-with-breach goal is human-only already (verbs.go:739) and
  takes the same human clearing path.
- **Unpark (verbs.go:872-914).** New refusal when `f.StopFence != nil`:
  `"goal %s is parked with breach %s; unpark cannot clear a breach fence — only goal resume with a fresh complete budget reopens it"`.
  Without this, unpark would launder a fence into an illegal queued+fence
  state.
- **Resume (stop.go:355-403).** The guard at stop.go:377 splits into two legal
  shapes: (a) claimed + capability + fence → today's path (and per Fix 1,
  `bindClaim` gives it a fresh episode — the human re-time); (b)
  `State == StateParked && StopCapability != nil && StopFence != nil` →
  verify the batch COMPLETE exactly as today, install the fresh budget and
  norm approval, clear `StopCapability`, `StopFence`, and `Parked`, set
  `State: queued`, bind no claim. The mismatch refusal rewords to
  `"goal %s has no breach-stopped revision"`.
- **The other ten `clearClaimBinding` sites** (arc cascades verbs.go:1631,
  1706, 1830, 1985; split.go:292; reconcilepub.go:428, 468; plus any future
  caller) keep refusing on a fence, with the wording updated because the old
  one is now false:
  `"goal %s is breach-stopped by %s; release it to parked-with-breach or resume it before this operation"`.
  The refusal is no longer absorbing — release now succeeds — so no arc
  operation can wedge a machine. The reconcile hand-park (reconcilepub.go:270)
  and hand-done (reconcilepub.go:319) rows route through the same
  `demoteToBreachPark` / human-done-clears rule as the verbs, keeping replay
  identical to live execution; `mapOneChange`'s `stopRetainedByPark`
  (reconcilemap.go:238-241) then describes real outputs.
- **SetBudget and SetObligation** keep their existing fence refusals
  (verbs.go:515, 587) — resume owns reopening admission.

### Quota and ledger validation invariants

- **Quota (validate.go:250-283).** The collection at validate.go:255 skips
  claims whose file carries a `StopFence`: the quota counts only WORKABLE
  claims. A machine holding one breach-stopped claim may claim its next goal
  immediately, before any release. The rejection wording for two workable
  claims is unchanged.
- **Parse invariants (file.go:239).** Replaced by: stop authority on a queued
  or done goal is a problem (as today); on a parked goal it is legal only as
  the complete pair — a lone capability or lone fence adds
  `"parked stop authority must be the complete capability+fence pair"`. The
  capability-vs-claim cross-check (file.go:288) applies only when
  `f.Claimed != nil`; on parked-with-breach the fence↔capability cross-checks
  (file.go:296-301) carry the binding. The claimed-state invariants
  (file.go:233-237) are untouched.
- **Batch custody.** Release does not require the stop batch COMPLETE — the
  fence survives release and resume still demands completeness
  (VerifyStopBatchComplete, stop.go:249), so cancellation duty is never lost
  by freeing the machine.

## Proof plan (tests named per behavior)

Fix 1 — internal/dispatch/budget_test.go unless noted:

- `TestRaiseDoesNotResetBreachClock`: claim at T0 with a 4h limit, set-budget
  raise at T0+3h, projection at T0+5h reports ELAPSED_BREACH anchored at T0.
- `TestFiveRaisesCannotOutrunTheBreaker`: five sequential raises (the night's
  exact pattern); elapsed still measures from T0.
- `TestReleaseReclaimStartsNewEpisode` and `TestStealStartsNewEpisode`
  (internal/goal/verbs_test.go): a genuine ownership restart writes fresh
  episode keys.
- `TestResumeStartsNewEpisode` (internal/goal/stop_test.go): the human re-time.
- `TestSetBudgetPinsLegacyAnchor` (verbs_test.go): a raise over a legacy claim
  (no episode keys) writes episode keys equal to the pre-raise claim binding.
- `TestClaimedEpisodeRoundTrip` (internal/goal/file_test.go or
  grammar_test.go): render→parse identity for the new keys.
- `TestEpisodeBindingContradictionsRefuse` (file_test.go): each of the three
  new validation wordings, plus the lone-key incompleteness problem, surfaces
  as BUDGET_UNKNOWN.
- `TestClockRegressedNamesEpisodeOrigin` (budget_test.go): observation before
  episodeAt yields the new CLOCK_REGRESSED wording.

Fix 2 — internal/goalbudget/budget_test.go unless noted:

- `TestClockGrammarDayIs24Hours`: "9d" enforces at 216 clock hours.
- `TestElapsedLimitStoredVerbatim`: New("24h") stores "24h", marker "clock".
- `TestLegacyRecordKeepsWorkingSemantics`: a markerless "3d" still enforces at
  24 clock hours (8h days).
- `TestUnknownGrammarRefuses`: any marker other than "clock"/absent.
- The existing normalization pin at budget_test.go:37 ("24 clock hours must
  render as 3 working days") is DELETED as part of this design — it pins the
  defect itself. Recorded here so its removal is a design act, not a weakened
  test.
- `TestJournalReplayPreservesLegacyBudget` (internal/goal/budget_test.go or
  journal_test.go): budgetFromIntentArgs without a grammar arg reconstructs a
  legacy budget bit-identical, never re-normalized or re-marked.
- `TestBudgetRecordGrammarRoundTrip` (internal/goal/grammar_test.go):
  `elapsedGrammar=clock` renders and parses; a legacy line without it
  round-trips unchanged.
- `TestStopFiringEvidenceCarriesGrammar` (internal/goal/stop_test.go): a
  clock-grammar fence's evidence validates; a legacy batch without the field
  still validates under working semantics.

Fix 3 — internal/goal unless noted:

- `TestBreachWedgeDies` (validate_test.go or verbs_test.go) — m3's demanded
  fixture, reproducing the exact wedge: machine claims goal A, breach-stop
  closes A's fence, the machine claims goal B. On today's code this commit
  fails validation with the verbatim "the quota is one claim per machine (one
  arc counts once)"; the test proves that rejection is DEAD — claim B lands
  while A stands fenced. The test first asserts the old refusal exists on the
  pre-fix behavior path only through the changed collection predicate (i.e.,
  it must fail on unpatched code), then asserts two WORKABLE claims still
  refuse with the unchanged wording (`TestQuotaStillOneWorkableClaim`).
- `TestReleaseSucceedsOnBreachStopped` (verbs_test.go): release yields
  parked-with-breach — parked state, Because names the stopId, capability and
  fence byte-identical, Claimed and Obligation cleared.
- `TestParkPreservesBreachRecord` (verbs_test.go).
- `TestUnparkRefusesBreachParked` (verbs_test.go): the exact new wording.
- `TestResumeReopensBreachParkedToQueued` (stop_test.go): batch COMPLETE
  required; queued afterward with no stop authority and no claim.
- `TestAgentDoneRefusesOverBreach` / `TestHumanDoneClearsBreach`
  (verbs_test.go): the archive file carries no stop authority.
- `TestParkedLoneStopAuthorityRefused` (file_test.go): lone fence or lone
  capability on a parked goal is a parse problem.
- `TestHandParkOfBreachStoppedReplays` (reconcilepub_test.go or
  reconcilemap_test.go): the hand-edit path produces the same
  parked-with-breach bytes as the verb.
- `TestArcOperationsNameTheFencedMember` (verbs_test.go): an arc cascade over
  a fenced member refuses with the new non-absorbing wording.

Gate: the full Go test suite plus `metasystem validate` fixtures run by the
orchestrator outside the sandbox (KI-15); delegate briefs must not demand the
validation suite from inside a sandbox.

## Failure modes

- **Crash mid-migration.** There is no engine rewrite pass. The operational
  re-set is one atomic ledger commit per goal; a crash between goals leaves a
  mixed tree in which every record is deterministically readable by its own
  marker. No window exists where a budget has no defined semantics.
- **Mixed-era records.** Legacy claim + new raise: the raise pins the episode
  (legacy-inheritance rule). Legacy budget under the new binary: working
  semantics forever. New record under an OLD binary: the closed record
  grammar refuses loudly by unknown key (file.go:566) — fail-closed, never a
  silent semantic flip. Legacy stop batch without the grammar field: validates
  under working semantics unchanged.
- **Concurrent raises.** Publication serializes on the ledger tip; each
  set-budget's Mutate reads the claim (and episode) from its own tip and the
  loser returns LostToCompetitor. The episode is inherited, so ANY
  serialization order preserves the same origin. A raise racing a release:
  whichever lands second sees the other's state — set-budget after release
  finds no claim to re-bind; release after raise demotes or queues exactly as
  if sequential.
- **Concurrent raise versus breach-stop.** Unchanged from today: set-budget
  refuses on a standing fence (verbs.go:515); breach-stop after a raise binds
  the raise's capability. The episode makes the ordering irrelevant to the
  clock.
- **Release with an OPEN stop batch** (custodian crashed mid-cancellation):
  the fence survives on the parked goal and resume still requires the batch
  COMPLETE; the custodian resumes the batch by stopId regardless of the
  goal's state. Nothing launches meanwhile (parked, and fenced).
- **Old governed-run snapshots.** Run records carry their dispatch-time
  `BudgetStartedAt` and Budget snapshot; runs dispatched before Fix 1 keep
  their old snapshot semantics, which is correct for their own terminal
  accounting and discriminated by the marker for durations.

## Deliberately unchanged

The reservation revision boundary and attempt/minutes filtering
(budget.go:339-360), the SetBudget fence refusal (verbs.go:515), the quota
wording for workable claims (validate.go:281), norm law (norm.go — minutes,
not this grammar), the stop batch file format except the one additive evidence
field, and everything in the steward and failed-job-attention seams named in
the non-goals.
