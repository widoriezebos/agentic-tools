# Design: the breach machinery stops lying

Goal: breach-clock-and-budget-honesty (plans/goals/breach-clock-and-budget-honesty.md,
revision 14). Design revision 2, 2026-09-02, Fable-lane designer. Revision 1
(2026-09-01) drew eight material findings from Sol's design critique
(records/misc/breach-design-critique-r1.md); this revision closes each one by
changing the design or refutes it against the tree, and carries the second
specimen the goal record added. Standard (Wido, verbatim): "hard deterministic
machinery. This is Go territory enforcing your behaviour" — every mechanism
below is engine-enforced; nothing here is conduct guidance. No finding is
closed by softening a requirement, weakening a refusal, or narrowing a
guarantee; where revision 1 promised something the tree could not deliver, the
promise is replaced by a stronger mechanism, and the replacement is named.

Evidence base: the goal record's Intent (five lawful clock resets in one
night), records/misc/idle-loss-2026-09-01.md (m3 frozen whole by one breach),
the goal record's Next step (the second duration specimen, read against the
parser below), the critique record, and the file-and-line traces cited inline.
Every cited seam was read in this worktree on 2026-09-02; line numbers are the
tree's on that date.

## The two duration specimens, read against the parser

The parser is `goalbudget.ParseWorkingDuration` (internal/goalbudget/budget.go:14-49):
`m` is a minute, `h` is one hour, `d` is EIGHT hours (budget.go:38). The
constructor `New` (budget.go:80-90) parses the typed token and then REWRITES it
through `FormatWorkingDuration` (budget.go:51-71), which re-renders any
multiple of eight hours as days.

- **First specimen (m2, morning of 2026-09-01).** A human typed `9d` meaning
  nine calendar days. The parser read 72 hours; the record shows `9d`; the
  fence fired at 72 clock hours plus grace. Enforcement harm: the human's
  fence was cut to one third, silently.
- **Second specimen (Wido's relayed resume of alert-escalation-channel,
  2026-09-01 12:39:02Z, history line 33 of that goal file).** The command was
  typed `--elapsed-limit 8h`. `New` parsed 8 hours and re-rendered them as
  `1d`, so the record reads `elapsedLimit=1d`. Read against budget.go:38, that
  record ENFORCES 8 clock hours, which is what was typed; the goal record's
  Next step says it enforces 24 clock hours, and that reading is not what the
  code does. The harm is real but sits one layer up: the ledger's word does not
  mean what it says. Every human and every agent reads `1d` as a day, which is
  exactly what happened when the specimen was written up. A record that is
  enforced as typed but displays as something else is the favorable-direction
  lie the goal record warns about: nobody notices, and the next relay carries
  the wrong number.

Both specimens have one root: a stored token whose meaning depends on a
grammar the reader has to know. Fix 2 removes that root rather than labelling
it.

## Scope and non-goals

Three proven defects, three engine fixes:

1. **Raise-reset clock**: every budget raise restarts the breach elapsed clock.
2. **Dishonest durations**: the stored elapsed-limit token does not mean what
   a reader thinks it means (`d` is eight hours), and the constructor rewrites
   the human's word into that token.
3. **Breach wedges the machine**: a breach-stopped claim refuses release (and
   park, and done), so nothing but a human resume can ever clear it, and while
   it stands it is the machine's one claim.

Non-goals, with the seam boundaries stated so they cannot collide:

- **The steward tripwire (m0's goal, burn-without-delivery-tripwire).** This
  design changes ledger state machinery in internal/goal, the elapsed
  projection and the stop custodian's resolvers and routes in
  internal/dispatch, the duration constructor in internal/goalbudget, and the
  one orientation line in cmd/metasystem/goal.go. It adds NO alerting,
  notification, or observation of a breach. The parked-with-breach state is a
  ledger fact m0's tripwire may later observe; this design neither consumes
  nor emits a tripwire signal.
- **Failed-job-attention (m3's goal).** Job records, the reaper, delivery of
  job-failure facts, and the wait primitive are untouched. The stop batch and
  its cancellation loop (scripts/agents/dispatch.sh:2267-2287) are untouched;
  this design only guarantees they keep being reached.
- **Attempt and reserved-minute accounting across raises.** `ProjectBudget`
  filters reservations by claim revision (internal/dispatch/budget.go:350
  `recordRevision != revision → continue`), so a raise also restarts attempt
  and job-minute counting. That is a sibling honesty defect; the goal scopes
  Fix 1 to the elapsed breach clock. Named as residue, not silently filled.
- **Lineage succession.** A new lineage on the same machine cannot release
  its predecessor's claim (verbs.go:656-658, 447-449) whether or not a fence
  stands. Fix 3 makes release lawful on a fenced claim under release's
  existing ownership rule; who may release is unchanged.

## Traced facts (the seams as they are today)

- `bindClaim` (internal/goal/verbs.go:113-126) writes a fresh `ClaimRecord`
  (`Machine, Lineage, At, Revision` — internal/goal/file.go:76-84), a fresh
  `StopCapability`, `StopFence = nil`, `Obligation = nil`. Nine call sites:
  claim (verbs.go:475), set-budget (verbs.go:540), reopen into a claimed arc
  (verbs.go:1016), steal (verbs.go:1256), open-claim (verbs.go:1323), arc
  claim cascade (verbs.go:1595), arc move into a claimed arc (verbs.go:2063),
  the reconcile equivalent (internal/goal/reconcilepub.go:515), and resume
  (internal/goal/stop.go:405).
- `ProjectBudget` (internal/dispatch/budget.go:237-490) parses `Claimed.At`
  (budget.go:258) as the elapsed origin; `obligationBudgetStart`
  (budget.go:77-164) may move the origin later through a consumed discharge
  proof, but only a proof whose `goalRevision` equals the CURRENT
  `Claimed.Revision` and whose `obligationRevision` equals the CURRENT
  `Obligation.Revision` (budget.go:133-135), and only when `file.Obligation`
  is non-nil at all (budget.go:78-80). `CLOCK_REGRESSED` fires at
  budget.go:266-268. `ValidateClaimRevision` (file.go:378-394) pins
  `Claimed.At` to `History[Claimed.Revision-1].At`. Governed admission
  snapshots `projection.Limits`, `projection.StartedAt`, and the weight epoch
  into the run record (internal/dispatch/governed.go:150; internal/run/run.go:123-125).
- Duration grammar: parser, formatter, constructor, and `Validate` as cited in
  the specimens section. Non-test consumers of the parser, verified by
  repo-wide grep: the goalbudget package; the re-export wrapper
  internal/goal/budget.go:11-22; `parseBudgetRecord` (internal/goal/budget.go:35-70,
  stores the token VERBATIM and validates); journal replay
  `budgetFromIntentArgs` (internal/goal/budget.go:81-99, rebuilds through
  `NewBudget`, re-normalizing); the CLI tuple (cmd/metasystem/goalsync_mutations.go:166-181;
  the flag help at :121 says "for example 4h"); stop firing evidence
  `validateStopFiringEvidence` (internal/goal/stop.go:145-157); admission
  (internal/dispatch/admission.go:176); the projection (budget.go:517);
  governed-run exhaustion (internal/run/conclude.go:315-318, reading the
  Budget snapshot the run record embeds and decodes permissively at
  run.go:383-389); and metrics spend (internal/metrics/compute.go:294).
  `FormatWorkingDuration` has exactly ONE non-test caller: `New` itself
  (budget.go:88), plus the goal re-export (internal/goal/budget.go:20-22).
  Job caps and norms are integer minutes everywhere (internal/goal/norm.go)
  and never touch this grammar.
- `clearClaimBinding` (verbs.go:128-137) refuses whenever `StopFence != nil`
  with "goal %s is breach-stopped by %s; only goal resume may clear its launch
  fence". Thirteen call sites: release (verbs.go:664), done (verbs.go:770),
  park (verbs.go:862), arc release cascade (verbs.go:1652), arc park cascade
  (verbs.go:1727), detach (verbs.go:1851), arc move (verbs.go:2006), split
  parent decomposition (internal/goal/split.go:292), reconcile hand-park
  (reconcilepub.go:270), reconcile hand-done (reconcilepub.go:319), and two
  reconcile arc rows (reconcilepub.go:428, 468). Steal refuses a fenced
  member by name before binding (verbs.go:1229-1230).
- The quota is enforced tree-wide at internal/goal/validate.go:250-283; its
  verbatim rejection is "machine %s claims %s: the quota is one claim per
  machine (one arc counts once)". The parse invariant at file.go:289-291
  refuses stop authority on any non-claimed goal; file.go:334-341 requires a
  capability to match the claim binding and reports "StopCapability
  contradicts the claim binding" when `Claimed` is nil; file.go:342-355 binds
  fence to capability. The hand-edit mapper tolerates exactly two of those
  diagnostics for a claimed-to-parked edit (reconcilemap.go:141-145) and
  admits stop retention or clearing only as a side effect of a claim cleared
  by park (reconcilemap.go:229-248 `claimClearedByPark`, `stopRetainedByPark`,
  `stopClearedByPark`). The existing test at reconcilepub_test.go:295-326
  requires an ordinary claimed hand-park to stay lawful.
- The stop custodian. `ResolveGoalBinding` (internal/dispatch/stop.go:41-64)
  refuses anything that is not `State == claimed` with a `Claimed` record and a
  `StopCapability`. `EnsureBreachStop` (stop.go:102-192) resolves, locks the
  goal revision, re-resolves, closes the fence through `goal.CloseStop`
  (stop.go:153), re-resolves a THIRD time (stop.go:160), and only then reads
  or creates the batch (stop.go:168-191); every batch coordinate comes from
  the capability and the fence (`StopBatch` fields, internal/goal/stop.go:31-38),
  none from `Claimed`. `FindBreachStops` (stop.go:270-336) skips every
  non-claimed file at stop.go:290-292 before it looks for a fence. The steward
  tick (internal/steward/tick.go:69-99) consumes only those routes and runs
  `dispatch.sh __breach-stop-goal --goal --revision` (dispatch.sh:2289-2305),
  which calls `job breach-stop` (cmd/metasystem/dispatch_verbs.go:970-1001)
  and then the cancellation loop (dispatch.sh:2267-2287); the loop reconciles
  and cancels by batch coordinates only (stop.go:341-481, 485-521).
  `GoalRecoveryPolicy.BreachStop` (stop.go:201-251) re-establishes a fence
  from a live budget projection of a claimed goal. The firing evidence is
  built at stop.go:134-139 from the projection's `Limits.ElapsedLimit` token.
- Resume (internal/goal/stop.go:355-412) is human-only with a live authority
  proof, demands the batch COMPLETE (`VerifyStopBatchComplete`, stop.go:249-263),
  installs a fresh complete budget, and re-binds the claim to the same pair
  (stop.go:405). The shipped command (cmd/metasystem/goalsync_mutations.go:354-418)
  calls `ResolveGoalBinding` at :394, refuses without a fence at :399-402,
  locks the CLAIMED revision at :403, records the proof, then calls
  `goal.Resume`. Journal recovery never replays a resume
  (internal/goal/recover.go:146-152).
- Consumers of "this machine's claimed set", by repo-wide grep on
  `Claimed.Machine == machine` and `State == StateClaimed`: the quota
  (validate.go:250-283); `Next` (internal/goal/project.go:90-99) feeding the
  orientation line (cmd/metasystem/goal.go:469-472); the mission prompt's
  serving projection (internal/goal/goalverbs.go:820-823); the turn verdict
  (internal/goal/turnverdict.go:483-490); the steward's open-work judgment
  (internal/steward/openwork.go:49); machine-wide dispatch admission
  (internal/dispatch/admission.go:57-105, which lists refusals per fenced or
  exhausted claim of the dispatching lineage); the custodian's routes
  (stop.go:288-334); and the arc claim cascade's own-claim scan
  (verbs.go:1501). The serving projection and the turn verdict iterate a map
  and return the first hit; the orientation prints the first of a sorted list.

## Fix 1 — the raise-proof breach anchor

### Decision

The claim EPISODE is the span from the moment a pair takes ownership until
that ownership genuinely ends. A budget raise re-binds the claim revision (the
reservation boundary stays exactly as the comment at verbs.go:484-486 intends)
but the episode origin survives it, and so does every lawful clock movement
that happened inside the episode. A new episode starts only when ownership
genuinely restarts: claim, open-claim, arc claim cascade, steal, an arc move
that creates a claim, reopen into an arc — and resume. Resume is the explicit
human re-time the goal record names ("only release-and-reclaim or an explicit
human re-time restarts it"): human-only, proof-gated, batch-complete-gated,
recorded as its own history verb. A resume that preserved the episode would
force the human to do elapsed arithmetic at the console for no integrity gain;
the breach record and history already carry the true span. Designer decision,
recorded here.

### Record and schema changes

`ClaimRecord` (file.go:76-84) gains two fields:

```go
// EpisodeAt and EpisodeRevision pin the moment this ownership episode
// began; a budget raise re-binds At and Revision but never these.
EpisodeAt       string
EpisodeRevision uint64
```

- **Grammar.** The `Claimed:` line's closed key set (file.go:530) gains the
  optional keys `episodeAt` and `episodeRevision`, both-or-neither. Render
  (file.go:767-772) appends ` episodeAt=%s episodeRevision=%d` when
  `EpisodeRevision > 0`. Absent keys mean a legacy record: the episode is the
  claim binding itself (anchor = `Claimed.At`, episode revision =
  `Claimed.Revision`). Discrimination is key presence, never a heuristic.
- **Validation.** `ParseFile` adds the problem
  `"Claimed episode binding is incomplete (episodeAt and episodeRevision travel together)"`
  when exactly one key is present. `ValidateClaimRevision` (file.go:378-394)
  is extended: when `EpisodeRevision > 0` it additionally requires
  `EpisodeRevision <= Claimed.Revision`, `EpisodeRevision <= uint64(len(History))`,
  `History[EpisodeRevision-1].At == EpisodeAt`, and parsed
  `EpisodeAt <= Claimed.At`. Failure wordings, exact:
  - `"claimed episodeRevision=%d is later than claim revision=%d"`
  - `"claimed episodeAt=%s contradicts History revision=%d at=%s"`
  - `"claimed episodeAt=%s is later than claimed at=%s"`
  These surface through the existing `BUDGET_UNKNOWN %v` prefix (file.go:304),
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
  happened before deployment are unrecoverable; the engine never mines history
  for an older one). Everything else (`StopCapability` minting,
  `StopFence = nil`, `Obligation = nil`) matches `bindClaim`; clearing the
  obligation on a raise is the existing governance rule at verbs.go:122-124
  and stays. Call-site rule, mechanical: verbs.go:540 (set-budget) is the ONLY
  site that switches to `rebindClaimKeepEpisode`; the other eight `bindClaim`
  sites are unchanged (fresh episode).
- **Projection origin.** In `ProjectBudget` (budget.go:237-282): derive
  `episodeAt, episodeRevision` from `Claimed.EpisodeAt, Claimed.EpisodeRevision`
  when `EpisodeRevision > 0`, else from `Claimed.At, Claimed.Revision`
  (malformed stamp → the existing unknownBudget path with
  `"the claim episode timestamp is malformed"`). Every elapsed-origin use of
  `claimedAt` becomes `episodeAt`: the `obligationBudgetStart` base and its
  `!consumedAt.Before(...)` filter (budget.go:77, 108, 134), the
  `now.Before(...)` regression check (budget.go:266), `StartedAt`/`Elapsed`
  (budget.go:280), and the three post-discharge comparisons
  `budgetStartedAt.After(claimedAt)` (budget.go:353, 442, 480). The
  reservation revision filters (`recordRevision != revision`, budget.go:350,
  392, 422) are untouched — see the residue non-goal.
- **Discharge proofs bind to the episode, not the revision (closes
  BCD-R1-003).** Revision 1 left the proof filters at budget.go:133-135 and
  146-157 revision-bound; the critic showed that a raise then drops the
  consumed proof (the rebind clears `Obligation`, verbs.go:124, and the
  filter demands `goalRevision == Claimed.Revision`), so
  `obligationBudgetStart` falls back to the episode origin and the clock
  REWINDS to before the discharge — a false breach in the unfavorable
  direction. The rule becomes:
  - `obligationBudgetStart(repoRoot, file, episodeAt, episodeRevision)`.
  - Short-circuit (replaces budget.go:78-80): return `episodeAt` when
    `file.Obligation == nil && episodeRevision == file.Claimed.Revision`. This
    is sound because `Obligation` is cleared only by `bindClaim`,
    `rebindClaimKeepEpisode`, and `clearClaimBinding` (verbs.go:124, 135); if
    no rebind happened inside the episode and no obligation stands, none was
    ever set in it, so no proof can belong to it. When a raise happened
    (`episodeRevision < Claimed.Revision`) the ledger is read even with a nil
    obligation.
  - Eligibility (replaces budget.go:133-135): a proof counts when
    `goalId == file.Id`, `episodeRevision <= goalRevision <= Claimed.Revision`,
    `!consumedAt.Before(episodeAt)`, and it is later than the latest so far
    (ties broken by the higher weight generation, as today).
  - Durable green match (replaces budget.go:146-157): the matched obligation
    state is the one whose `(GoalRevision, ObligationRevision)` equals the
    LATEST PROOF'S OWN pair, not the live file's; the attempt match
    (run green, breaker closed, not exhausted, same weight generation) is
    unchanged. Every other refusal in the function (malformed ledger,
    unauthorized entry, duplicate proof, applied discharge without a ledger)
    is unchanged.
  - Legacy claims (no episode keys) have `episodeRevision == Claimed.Revision`
    and `episodeAt == Claimed.At`, so the rule is byte-for-byte today's for
    them.
  - Consequence, stated because it changes a behavior: inside one episode, a
    discharge consumed under obligation revision 5 still holds the moved start
    after a human `set-obligation` installs revision 7 (no rebind happens:
    verbs.go:568-626). Today's code drops that proof and rewinds the clock to
    the claim. Under Fix 1's thesis only an ownership restart or the human's
    re-time moves the clock, and set-obligation is neither; the alternative
    (keep revision-exactness before a raise, episode scope after it) would let
    a raise move the start LATER than it was the moment before, which is the
    favorable-direction dishonesty this goal exists to remove. Designer
    decision, named for the critic.
  - Governed dispatch snapshots `StartedAt` and the weight epoch from this
    projection (governed.go:150), so post-raise governed runs carry the same
    epoch the projection finds and internal/run/conclude.go:315-318 stays
    correct for new runs with no change of its own.
- **CLOCK_REGRESSED** now means: the observation precedes the episode origin
  (or the consumed discharge proof that lawfully moved the start later). Exact
  wording, replacing budget.go:267:
  `"CLOCK_REGRESSED: the claim episode origin is later than the observation"`.
  It remains a typed refusal to project, never a zeroed clock.
- **How recovery reads it.** Recovery and reconcile replay through the real
  verb builders (recover.go:236-264, reconcilepub.go), so the episode is
  reproduced by the same mutation code: a replayed claim writes a fresh
  episode, a replayed set-budget inherits through `rebindClaimKeepEpisode`
  against the tree at its tip. The hand-edit surface already refuses any
  altered `Claimed` line as a generated field (reconcilemap.go:232-237), which
  makes the episode keys tamper-proof there with zero new code.

## Fix 2 — the stored token means what it says

### Decision

For budget elapsed limits the engine accepts `m` (one minute) and `h` (one
clock hour) and stores the human's token VERBATIM. The engine REFUSES `d` at
the moment it is set, naming the ambiguity and the two readings, because a `d`
in a stored record has meant eight working hours since the grammar existed
and every human reads it as twenty-four. Nothing in the engine writes a `d`
any more, so a `d` in the tree is a legacy record by construction and keeps
its working-hours meaning forever; `m` and `h` mean the same under the old
parser and the new, so a new record cannot be misread by any binary in either
direction. The marker is the letter itself. Revision 1 carried a separate
`elapsedGrammar` field with clock semantics for `d`; that field had to be
wired into every producer and honored by every reader, and the critic found
one producer (BCD-R1-005) and three readers (BCD-R1-006) where it was not.
This revision removes the field: there is nothing to wire and nothing to
miss.

Per input shape, which of the goal record's two permitted outcomes applies:

| Typed | Outcome | Stored | Enforced | Displays as | An old binary reads |
| --- | --- | --- | --- | --- | --- |
| `8h`, `90m`, `4h30m`, `216h` (m and h only) | enforced exactly as typed | verbatim | as typed | as typed | the same hours (budget.go:33-36 unchanged) |
| `9d`, `1d`, `1d2h` (any `d`) | refused at set time, with both readings named | nothing | nothing | nothing | nothing (never written) |
| legacy record `1d` already in the tree | untouched | `1d` | 8 clock hours (its meaning since it was written) | `1d`, until the seat re-sets it with the human's recorded word | 8 clock hours |

Why refuse rather than redefine `d` as 24 hours: a redefinition needs a marker
to tell a new `9d` (216h) from an old `9d` (72h), and every reader that misses
the marker misreads by a factor of three. A refusal needs no marker, changes
no parser, and the human types the hours they mean. The second specimen shows
the human already types hours.

### Record and schema changes

- `goalbudget.New` (budget.go:80-90):
  1. If `strings.ContainsRune(elapsedLimit, 'd')`, refuse. Exact wording:
     `"elapsedLimit %q is refused: d is ambiguous (older records read it as 8 working hours, humans read it as 24); type the clock hours you mean in h and m, for example %s"`,
     where the example is the token re-read with `d` as 24 hours and rendered
     as `<hours>h[<minutes>m]` by a small pure helper `calendarHoursExample`
     (same token walk as the parser, `d = 24 * time.Hour`); for `9d` the
     example is `216h`, for `1d2h` it is `26h`. If the token does not parse
     even with that reading, the example is omitted and the sentence ends at
     "in h and m".
  2. Else parse with `ParseWorkingDuration`; refusal wording, exact:
     `"elapsedLimit %q is not a positive duration in minutes and hours (for example 4h, 90m, or 4h30m)"`.
  3. Store `ElapsedLimit: elapsedLimit` VERBATIM. The `FormatWorkingDuration`
     call at budget.go:88 is deleted.
- `FormatWorkingDuration` (budget.go:51-71) and its re-export
  (internal/goal/budget.go:18-22) are DELETED: `New` was their only non-test
  caller, and the engine must have no writer of `d` left. Their tests go with
  them (below).
- `ParseWorkingDuration`, `Validate`, `ElapsedDuration`, `ElapsedBreachDuration`
  (budget.go:14-49, 92-128) are UNCHANGED. `Validate` keeps accepting `d` so
  legacy records stay readable and enforceable at their original meaning.
  The `Budget` struct (budget.go:73-78) is unchanged and stays comparable.
- **Journal replay.** `budgetFromIntentArgs` (internal/goal/budget.go:81-99)
  STOPS calling `NewBudget`: it constructs the `Budget` verbatim from the four
  stored args and calls `Validate`. Two reasons, both mechanical: a legacy
  journal entry carrying `1d` would now be refused by `New`, and a recovering
  binary must never rewrite a stored token. `budgetIntentArgs` (budget.go:72-79)
  is unchanged.
- `parseBudgetRecord` (internal/goal/budget.go:35-70) is already verbatim plus
  `Validate`: unchanged. The `Budget:` line grammar (file.go:450-456,
  722-723) is unchanged; no new key.
- **Stop firing evidence.** `StopFiringEvidence` (internal/goal/stop.go:55-60)
  and `validateStopFiringEvidence` (stop.go:145-157) are UNCHANGED, and so is
  the producer `EnsureBreachStop` (dispatch/stop.go:134-139): it copies the
  stored token `budget.Limits.ElapsedLimit` into `AdmissionLimit`, and the
  validator recomputes the boundary with the same unchanged parser, so the
  two agree for a legacy `9d` (72h plus grace) and for a new `216h` (216h plus
  grace) alike. This closes BCD-R1-005 by removing the field the producer had
  failed to carry; the proof plan still puts the test at the producer seam.
- **Run snapshot.** `GovernedAttempt.Budget` (run.go:123) and the permissive
  decode at run.go:383-389 are unchanged; `Validate` (run.go:469-474) still
  checks the tuple; conclude.go:318 still reads `ElapsedDuration`. A new
  record's token means the same to every binary, so there is nothing to
  guard.
- The CLI help at cmd/metasystem/goalsync_mutations.go:121 becomes
  `"positive elapsed duration in minutes and hours, for example 4h or 90m (d is refused as ambiguous)"`.
  docs/backlog-mechanism.md:17-18 ("a positive working duration such as 4h or
  1d; one working day is eight hours") becomes: minutes and hours only; `d`
  is refused when set; a `d` in an existing record is a legacy token meaning
  eight hours per day and is re-set by hand with the human's recorded word.

### Rollout and rollback (closes BCD-R1-006)

The claim is now: **no binary, old or new, can misread a record written by
the other**, because the only byte whose meaning differs between readers is
`d`, and the new binary never writes it. Reader by reader, with an old binary
reading a NEW record (say `216h`):

| Reader | Old binary's behavior on `216h` | Verdict |
| --- | --- | --- |
| Goal file `Budget:` line, `parseBudgetRecord` (goal/budget.go:35-70) | parses 216 hours | correct |
| Governed run snapshot, `Store.Read` permissive decode (run.go:383-389) then `conclude.go:318` | decodes the tuple, enforces 216 hours | correct |
| Stop batch `validateStopFiringEvidence` (goal/stop.go:145-157) | parses `216h`, recomputes the same boundary | correct |
| Journal replay `budgetFromIntentArgs` (goal/budget.go:98) recovering a NEW binary's dead set-budget entry | `NewBudget("216h")` parses 216 hours and RE-RENDERS the token as `27d` into the goal file | enforcement identical (27 × 8 = 216 hours under both binaries); the record's display regresses to a legacy `d` token for that one goal until re-set |

And a new binary reading an OLD record (`9d`, `1d`): the unchanged parser
reads eight hours per `d`, which is that record's meaning since it was
written; correct by definition.

So the one mixed-deployment effect is a display regression written by an OLD
binary's normalizing paths (its journal recovery above, and its own
`goal set-budget`), never a misread by any reader. It cannot change what any
fence enforces. The design names it and asks Wido to accept it as the
rollout residue, and the deployment rule stays: deploy the new binary on both
machines before the first new-grammar write. Revision 1's claim that the
closed record grammar would refuse loudly (file.go:616) applied only to the
Markdown record and is withdrawn; it is no longer needed.

### Migration (closes BCD-R1-008)

- **Engine part: none.** No rewrite pass exists. Legacy tokens stay readable
  and enforceable at their original meaning for their whole lifetime. There is
  no rewrite to crash and no window without defined semantics.
- **Operational part (the seat's, not the implementer's), restricted to
  UNFENCED live budgeted goals.** After both machines run the new binary, the
  seat re-sets each unfenced live budgeted goal with `goal set-budget`, using
  the human's recorded verbatim word in `h`/`m`, verified against that goal's
  history and rulings (the normalized journal args are NOT the human's word;
  where no verbatim word is recoverable the seat asks Wido rather than
  converting arithmetically). Under Fix 1 this raise keeps the episode
  (`rebindClaimKeepEpisode`), so no breach clock resets.
- **Fenced goals are not re-set, and need not be.** `SetBudget` refuses on a
  fence (verbs.go:514-515, unchanged), on a claimed-and-fenced goal and on a
  parked-with-breach one alike. Revision 1 asked the seat to re-set "every
  live budgeted goal", which the critic showed is impossible for a fenced
  one; the instruction was wrong, not the refusal. A fenced goal's legacy
  budget is replaced by the human's fresh budget at `goal resume`, which is
  the explicit human re-time and a fresh episode by the goal record's own
  rule; that fresh budget is typed in the honest grammar because `New`
  refuses anything else. A grammar-only rebind on a fenced goal is therefore
  neither performed nor needed: the fence's own exit installs the honest
  word.
- **The specimen, before and after.** The critic's tree held
  plans/goals/alert-escalation-channel.md claimed and fenced with
  `elapsedLimit=1d`. In this worktree the same goal is `State: queued` with
  `Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=240 activeJobLimit=1`
  and no fence (its history lines 33-34: resume at 2026-09-01T12:39:02Z, then
  release). Under this design:
  - Before deployment: those bytes; enforced 8 clock hours; displays a day.
  - After deployment, no migration: identical bytes; identical enforcement
    (8 hours); still displays a day.
  - After the seat's re-set with Wido's recorded word from the resume command
    (`--elapsed-limit 8h`): `Budget: elapsedLimit=8h attemptLimit=10 reservedJobMinutesLimit=240 activeJobLimit=1`;
    enforced 8 hours; displays 8 hours. The goal is queued, so no episode
    exists to preserve; had it been claimed, the raise would have kept it.
  - Had it still been claimed and fenced: untouched by the seat; Wido's
    `goal resume --elapsed-limit 8h ...` installs the honest word and starts
    the fresh episode the goal record prescribes for a human re-time.

## Fix 3 — a breach parks the goal, never the machine

### Decision and state model

Two shapes carry a breach record:

- **claimed-and-fenced** (today's shape): `State: claimed`, `Claimed`,
  `StopCapability`, `StopFence`. Launches are refused (admission.go:133-142),
  the budget is still projectable for the record, and the claim is the
  machine's one claim under the quota.
- **parked-with-breach** (new): `State: parked`, a `Parked` record, `Claimed`
  and `Obligation` cleared, `StopCapability` and `StopFence` retained
  byte-identical. Not claimable (parked), not launchable (`ResolveGoalBinding`
  refuses at stop.go:54, unchanged), not budget-projectable, and it costs its
  former claimant nothing.

Transitions: claimed-and-fenced → parked-with-breach by `release` or `park`
(the owner pair, or a human; foreign park stays a human act); either shape →
`queued` by `goal resume` (fresh budget; from the claimed shape it re-binds
the claim and starts a fresh episode, from the parked shape it binds no
claim); either shape → `done` by a HUMAN `goal done` once the stop batch is
COMPLETE. `unpark`, `set-budget`, `set-obligation`, steal, arc cascades,
detach, arc move, and split keep refusing on a fence.

### The cancellation-duty invariant (closes BCD-R1-002)

Name: **a fence is never off the route.** For every live goal file with
`StopFence != nil`, in either shape, `FindBreachStops` emits a route until
`ReadStopBatch(fence.StopID)` reports COMPLETE; and a fence leaves the live
tree only through `resume` or a human `done`, both gated by
`VerifyStopBatchComplete` (stop.go:249-263). Together: every job a fence
obliges the custodian to cancel stays on the automatic route until the batch
that cancels it is COMPLETE, whatever release, park, or crash happens in
between. Revision 1 claimed "the custodian resumes the batch by stopId
regardless of the goal's state"; the critic showed the custodian's resolver
and route both skip parked goals, so that sentence was false. The mechanics
below make it true.

- **New resolver** in internal/dispatch/stop.go, beside `ResolveGoalBinding`
  (which is UNCHANGED and stays the launch-side gate):

  ```go
  // StopAuthority is the breach record in either lawful shape: a claimed
  // goal with its stop capability, or a goal parked with its complete
  // capability+fence pair. Revision and Machine come from the capability.
  type StopAuthority struct {
      GoalID     string
      Revision   uint64
      Machine    string
      Capability goal.StopCapability
      Fence      *goal.StopFence
      Parked     bool
      File       *goal.GoalFile
  }
  func ResolveStopAuthority(root, id string, now time.Time) (StopAuthority, error)
  ```

  It accepts `State == claimed && Claimed != nil && StopCapability != nil`
  (Fence optional) and `State == parked && StopCapability != nil && StopFence != nil`.
  Refusal wording, exact:
  `"goal %s carries no stop authority: it is neither a claimed goal with a stop capability nor a goal parked with its breach record"`.
- **`EnsureBreachStop`** (stop.go:102-192): the three resolves at :103, :117,
  and :160 switch to `ResolveStopAuthority`; the revision comparisons use
  `authority.Revision` (equal to `Claimed.Revision` in the claimed shape by
  the parse invariant at file.go:338). For the parked shape the fence is
  present, so the close branch (:125-164) is skipped and the batch is
  rediscovered or created from `Capability` and `Fence` exactly as at
  :168-191; the custodian actor's machine is `Capability.Machine`. The race
  the critic named (release landing between `CloseStop` at :153 and the
  re-resolve at :160) now resolves: the third resolve finds the
  parked-with-breach record and proceeds to the batch. If the process dies
  there, the next tick's route rediscovers the fence with no batch and creates
  it.
- **`FindBreachStops`** (stop.go:288-314): the `continue` at :290-292 becomes
  a two-branch predicate. A file with `StopFence != nil` that is claimed OR
  parked-with-breach produces the fenced route as at :293-314 with
  `Revision: file.StopCapability.Revision`. A claimed file without a fence
  takes the budget scan at :315-334 unchanged. An unfenced parked file is
  skipped as today. Route conditions (BREACH; INDETERMINATE for an unreadable
  or INDETERMINATE batch) are unchanged.
- **Steward and shell are unchanged.** tick.go:69-99 consumes routes;
  dispatch.sh:2289-2305 passes the route's revision to `job breach-stop`,
  whose `EnsureBreachStop` call checks it against `authority.Revision`; the
  cancellation loop at dispatch.sh:2267-2287 works from batch coordinates
  only. `GoalRecoveryPolicy.BreachStop` (stop.go:201-251) stays claimed-only:
  it re-establishes a fence from a live budget projection, which a parked goal
  does not have; a breach-stop whose fence never landed before a release has
  nothing to recover, exactly as today.
- **Release does not wait for the batch, and does not need to.** The route
  carries the duty. Requiring COMPLETE before release would re-create the
  wedge whenever the custodian is dead or the batch INDETERMINATE, which is
  precisely when a human most needs the machine free.
- **Human done requires COMPLETE.** Revision 1 let a human `done` clear the
  pair unconditionally; that would move an OPEN batch's fence into the archive
  where no route sees it. The rule below fixes it.

### Verb-by-verb mechanics

One shared helper owns the demotion:

```go
// demoteToBreachPark releases ownership of a fenced claim into
// parked-with-breach: the goal waits for the human, the machine goes free.
func demoteToBreachPark(f *GoalFile, by, at, because, displaced string)
```

It sets `State: parked`, writes `Parked{By, At, Because, Displaced}`, clears
`Claimed` and `Obligation`, and leaves `StopCapability` and `StopFence`
untouched.

- **Release (verbs.go:636-672).** Guards at :653-658 unchanged. At :663-666:
  when `f.StopFence != nil`, call `demoteToBreachPark` with
  `because = "breach-stopped by <stopId>; awaiting the human's word (goal resume with a fresh complete budget, or a human goal done)"`
  and the displacement as computed at :659-662; otherwise `State: queued` and
  `clearClaimBinding` as today. History verb stays `release`.
- **Park (verbs.go:820-875).** Guards at :839-855 unchanged. At :857-864:
  when fenced, `demoteToBreachPark` with the caller's `because`; otherwise as
  today.
- **Done (verbs.go:725-784).** After the guards at :750-758, when
  `f.StopFence != nil`: if `r.Actor.Human == ""` refuse
  `"goal %s is breach-stopped by %s; concluding over a standing breach is a human act"`;
  then `VerifyStopBatchComplete(r.Endpoint.Root, id, *f.StopCapability, *f.StopFence)`
  and on error refuse
  `"goal %s cannot conclude over an incomplete stop batch: %v"`; then clear
  `StopCapability`, `StopFence`, `Claimed`, and `Obligation` before the
  existing `clearClaimBinding` call at :770 runs on the now fence-free file.
  The stop batch file under artifacts/agents/goal-stops/ remains as local
  history. Done on a parked goal is already human-only (:753-755).
- **Unpark (verbs.go:886-).** New refusal after :903-905 when
  `f.StopFence != nil`:
  `"goal %s is parked with breach %s; unpark cannot clear a breach fence; only goal resume with a fresh complete budget reopens it"`.
  Without it, unpark would launder a fence into an illegal queued-with-fence
  state.
- **Resume, package (stop.go:365-412).** The guard at :378 splits into two
  legal shapes: (a) claimed + capability + fence → today's path, and per Fix 1
  `bindClaim` gives it a fresh episode; (b)
  `State == parked && StopCapability != nil && StopFence != nil` → verify the
  batch COMPLETE exactly as today (:390), install the fresh budget and norm
  approval, clear `StopCapability`, `StopFence`, and `Parked`, set
  `State: queued`, touch `resume`, bind no claim. Mismatch wording:
  `"goal %s has no breach-stopped revision (neither a fenced claim nor a goal parked with its breach record)"`.
- **Resume, command (closes BCD-R1-001; goalsync_mutations.go:394-413).**
  `ResolveGoalBinding` at :394 becomes `ResolveStopAuthority`; the
  `Fence == nil` refusal at :399-402 keeps its wording; the lock at :403 is
  taken on `authority.Revision` (the capability's revision, which in the
  claimed shape equals today's `binding.Revision`); `RecordResumeProof` and
  `goal.Resume` follow unchanged. The proof plan puts a test at this command
  seam, not only in the goal package.
- **`clearClaimBinding` (verbs.go:128-137)** keeps refusing on a fence; its
  wording is updated because the old one is now false:
  `"goal %s is breach-stopped by %s; release or park it into parked-with-breach, resume it, or (a human) conclude it before this operation"`.
  The remaining callers — arc release cascade (:1652), arc park cascade
  (:1727), detach (:1851), arc move (:2006), split (split.go:292), and the
  reconcile arc rows (reconcilepub.go:428, 468) — refuse on a fence; the
  refusal is no longer absorbing because release and park succeed, so no arc
  operation can wedge a machine. Steal's own fence refusal (verbs.go:1229-1230)
  is unchanged and still true.
- **Reconcile replay** routes through the same rules: the hand-park row
  (reconcilepub.go:257-282) takes the Release/Park branch (demote when
  fenced, `clearClaimBinding` otherwise); the hand-done row
  (reconcilepub.go:300-331) takes the Done rule (reconcile rows are human by
  construction, reconcilemap.go:14-15, so the COMPLETE check is what gates
  it). Replay and live execution stay byte-identical.
- **SetBudget and SetObligation** keep their fence refusals (verbs.go:514-515,
  591-593); resume owns reopening admission.

### One claim per machine, fenced or not (closes BCD-R1-004)

Revision 1 excluded fenced claims from the quota so a machine could claim its
next goal before releasing the fenced one. The critic showed that this leaves
two simultaneous claims on one machine, which the orientation line, the
serving projection, and the turn verdict then pick between nondeterministically
(goal.go:469-472; goalverbs.go:820-823; turnverdict.go:483-490). Revision 2
takes the other branch the brief allows: **a fenced claim is the machine's
only claim until it is released or parked**, and release is always lawful on
it. The machine becomes workable in one deterministic step,
`goal release --id <fenced>`, which has no batch precondition (the route
carries the duty) and follows release's existing ownership rule.

- **Quota (validate.go:250-283) is UNCHANGED**, wording included.
- **New tree invariant**, in the same block: for each machine, if any claimed
  file carries a `StopFence` and the machine holds any other claim (same arc
  or not), add
  `"machine %s claims %s while %s is breach-stopped by %s; a fenced claim is the machine's only claim until it is released or parked into parked-with-breach"`.
  It is enforced by `ValidateCommit` on every publication, so it covers claim,
  open-claim, the arc claim cascade (where the quota alone would admit a
  same-arc second claim), steal, arc move, reopen into an arc, and reconcile,
  with no per-verb code.
- **Orientation (goal.go:469-472)** gains one deterministic branch: when
  `p.Tree.Live[v.Claimed[0]].StopFence != nil`, print
  `"your claimed goal %s is breach-stopped by %s; release it to park it with its breach record (goal release --id %s), then claim your next goal"`;
  otherwise today's line. `Next` (project.go:90-99) is unchanged. This is the
  only consumer that changes, and it changes so the machine is never pointed
  back at stopped work without being told the way out.
- **Every consumer of the claimed set, and its rule under this design:**

  | Consumer | Rule |
  | --- | --- |
  | Quota, validate.go:250-283 | unchanged; a fenced claim counts |
  | Only-claim invariant, validate.go (new) | a machine with a fenced claim has no second claim |
  | `Next` → orientation, project.go:90-99, goal.go:469-472 | at most one claim exists; the fenced case prints the way out |
  | Serving projection, goalverbs.go:820-823 | unchanged; at most one claim, so the first map hit is the only hit; launches on it are refused by admission.go:133-142 as today |
  | Turn verdict, turnverdict.go:483-490 | unchanged; same reason |
  | Steward open-work, openwork.go:49 | unchanged; `WorkOwned` while the fenced claim stands, `WorkNone`/queued after release, because parked-with-breach is not this machine's work |
  | Machine-wide admission, admission.go:57-105 | unchanged; the fenced claim is listed as a refusal with its stop id, as today |
  | Custodian routes, stop.go:288-334 | extended to parked-with-breach (above) |
  | Arc claim cascade own-claim scan, verbs.go:1501 | unchanged; the tree invariant refuses the cascade's second claim beside a fenced one |

### Parse invariants and the hand-edit mapper (closes BCD-R1-007)

Parse rules (file.go):

- file.go:289-291 splits by state. Queued or done with any stop authority:
  `"stop authority on a %s goal"`, unchanged wording. Parked: legal only as
  the complete pair; a lone capability or lone fence adds
  `"parked stop authority must be the complete capability+fence pair"`.
- file.go:334-341: the capability-versus-claim cross-check runs only when
  `f.Claimed != nil`. On parked-with-breach the fence-versus-capability checks
  at file.go:342-355 carry the binding (revision, epoch, generation, reason).
- file.go:283-287 (claimed needs a claim record; a claim record only on a
  claimed goal) is untouched.

Mapper contract (reconcilemap.go), stated as the three operations the critic
named plus the one refusal that protects the invariant:

1. **Ordinary hand-park of a claimed goal** (base claimed, no fence; the
   human sets `State: parked`, removes the `Claimed` line, writes a `Parked`
   because, and either leaves the materialized `StopCapability` line or
   deletes it). Diagnostics: the lone-capability wording above when the line
   is left; none when deleted. The tolerated set at reconcilemap.go:141-145
   for `base.State == claimed && edited.State == parked` becomes exactly
   `"parked stop authority must be the complete capability+fence pair"` (the
   two old wordings no longer fire for this edit and are removed from the
   list). `mapOneChange` is unchanged for this case (`stopRetainedByPark` or
   `stopClearedByPark`, :238-242) and yields a park row; replay
   (reconcilepub.go:257-282) has no fence, so `clearClaimBinding` clears the
   capability. The existing test at reconcilepub_test.go:295-326 stays green.
2. **Breach hand-park** (base claimed and fenced; the human sets parked,
   removes `Claimed`, writes the because, leaves the pair). No diagnostics
   (complete pair on parked is legal; the claim cross-check is skipped).
   `stopRetainedByPark` holds → park row → replay takes the fenced branch →
   `demoteToBreachPark` → bytes identical to the verb's.
3. **Breach hand-done** (base parked-with-breach; the human sets `State: done`,
   writes `Concluded`, deletes the `Parked` line as today's grammar already
   requires, and DELETES both stop lines). New mapper clause:
   `stopClearedByDone := base.State == StateParked && base.StopFence != nil && edited.State == StateDone && edited.StopCapability == nil && edited.StopFence == nil`,
   admitted alongside the existing park clauses at :243-248 → done row →
   replay (reconcilepub.go:300-331) applies the Done rule: human actor by
   construction, `VerifyStopBatchComplete`, then the pair is cleared and the
   goal archived. Leaving the stop lines in place yields the untolerated
   `"stop authority on a done goal"` diagnostic and refuses, so the shape is
   single-valued, exactly like the `Parked` line today. A hand-done from the
   CLAIMED shape stays unmappable, as it is today for any claimed goal
   (:232-237 refuses the cleared `Claimed` line): the verb is the path.
4. **A fence cannot be hand-deleted.** `stopClearedByPark` (:242) gains
   `&& base.StopFence == nil`; a claimed-and-fenced base whose edit removes
   the pair while parking refuses with
   `"%s: a standing breach fence is generated; it leaves only through goal resume or a human goal done"`.
   This keeps the cancellation-duty invariant true on the hand path.

## Proof plan (tests named per behavior)

Fix 1 — internal/dispatch/budget_test.go unless noted:

- `TestRaiseDoesNotResetBreachClock`: claim at T0 with a 4h limit, set-budget
  raise at T0+3h, projection at T0+5h reports ELAPSED_BREACH anchored at T0.
- `TestFiveRaisesCannotOutrunTheBreaker`: five sequential raises (the night's
  exact pattern); elapsed still measures from T0.
- `TestRaiseAfterDischargeKeepsThePostDischargeStart` (closes BCD-R1-003):
  claim at T0 with a 4h limit and an obligation; a consumed discharge proof
  at T0+3h for the claim's (goalRevision, obligationRevision) with its exact
  durable green attempt; projection at T0+3h30 reports `StartedAt == T0+3h`;
  raise at T0+3h40 (new claim revision, obligation cleared); projection at
  T0+4h still reports `StartedAt == T0+3h`, `Elapsed == 1h`, no breach.
- `TestSetObligationDoesNotRewindADischargedClock`: same discharge, then a
  human set-obligation to a new obligation revision with no raise; the start
  stays at the discharge. Pins the stated consequence.
- `TestReleaseReclaimStartsNewEpisode` and `TestStealStartsNewEpisode`
  (internal/goal/verbs_test.go): a genuine ownership restart writes fresh
  episode keys.
- `TestResumeStartsNewEpisode` (internal/goal/stop_test.go): the human re-time.
- `TestSetBudgetPinsLegacyAnchor` (verbs_test.go): a raise over a legacy claim
  (no episode keys) writes episode keys equal to the pre-raise claim binding.
- `TestClaimedEpisodeRoundTrip` (internal/goal/grammar_test.go): render→parse
  identity for the new keys.
- `TestEpisodeBindingContradictionsRefuse` (file_test.go): each of the three
  new validation wordings, plus the lone-key incompleteness problem, surfaces
  as BUDGET_UNKNOWN.
- `TestClockRegressedNamesEpisodeOrigin` (budget_test.go): observation before
  episodeAt yields the new CLOCK_REGRESSED wording.

Fix 2 — internal/goalbudget/budget_test.go unless noted:

- `TestNewStoresTheTypedTokenVerbatim`: `8h`→`8h`, `24h`→`24h`, `90m`→`90m`,
  `4h30m`→`4h30m`; the second specimen's command shape, `New("8h", 10, 240, 1)`,
  stores `8h`.
- `TestNewRefusesDayTokensByName`: `9d`, `1d`, `1d2h` refuse with the exact
  wording and the computed examples `216h`, `24h`, `26h`.
- `TestLegacyDayRecordsKeepWorkingHours`: `Validate` accepts `1d` and `9d`;
  `ElapsedDuration` reads 8h and 72h.
- The existing `TestFormatWorkingDurationUsesEightHourDaysAndRoundTrips`
  (budget_test.go:35-51) is DELETED with the function it pins, and
  `TestNewNormalizesAndRefusesIncompleteTuples` (budget_test.go:53-69) is
  rewritten to assert verbatim storage instead of `3d`. Both pin the defect
  itself; recorded here so the removal is a design act, not a weakened test.
- `TestJournalReplayPreservesTheStoredToken` (internal/goal/budget_test.go):
  `budgetFromIntentArgs` rebuilds a legacy `1d` and a new `216h`
  bit-identical, never re-rendered and never refused.
- `TestBreachStopEvidenceCarriesTheStoredToken` (internal/dispatch/stop_test.go,
  the PRODUCER seam; closes BCD-R1-005): a goal with `216h` breached at the
  injected clock → `EnsureBreachStop` writes a batch whose
  `FiringEvidence.AdmissionLimit == "216h"` and whose `BreachBoundary` equals
  216h plus grace, and `ReadStopBatch` validates it; the same for a legacy
  `9d` fixture with a 72h-plus-grace boundary.

Fix 3 — internal/goal unless noted:

- `TestBreachWedgeDies` (verbs_test.go) — m3's demanded fixture, reshaped to
  the mechanism that now kills the wedge: machine claims goal A, breach-stop
  closes A's fence, the machine runs `goal release` on A. On today's code that
  release fails with the verbatim "only goal resume may clear its launch
  fence", and a claim of goal B fails with "the quota is one claim per
  machine (one arc counts once)"; the test asserts both refusals are DEAD in
  that order: release lands as parked-with-breach, then claim B lands.
- `TestFencedClaimIsTheMachinesOnlyClaim` (validate_test.go or verbs_test.go):
  with A fenced and unreleased, claim B (different arc) refuses with the
  unchanged quota wording; an arc claim cascade beside A (same arc) refuses
  with the new invariant wording.
- `TestOrientationNamesTheWayOutOfAFence` (cmd/metasystem/goal_test.go): the
  orientation line for a fenced claim is the exact new sentence.
- `TestReleaseSucceedsOnBreachStopped` (verbs_test.go): release yields
  parked-with-breach — parked state, Because names the stopId, capability and
  fence byte-identical, Claimed and Obligation cleared. Release does not read
  the stop batch.
- `TestParkPreservesBreachRecord` (verbs_test.go).
- `TestUnparkRefusesBreachParked` (verbs_test.go): the exact new wording.
- `TestResumeReopensBreachParkedToQueued` (stop_test.go): batch COMPLETE
  required; queued afterward with no stop authority and no claim.
- `TestGoalResumeCommandReopensAParkedWithBreachGoal`
  (cmd/metasystem/goalsync_mutations_test.go; closes BCD-R1-001): build on the
  fenced fixture at :447-505 with a COMPLETE batch, release the goal to
  parked-with-breach, run `runGoalResumeWithAuthority` with the full budget
  tuple and a granted proof; exit 0, the tree shows queued with no stop
  authority, no claim, and the fresh budget. Twin:
  `TestGoalResumeCommandRefusesAnIncompleteBatchOnAParkedWithBreachGoal`
  (batch OPEN → exit 1, stderr names "not COMPLETE").
- `TestReleasedFenceStaysOnTheRouteUntilItsBatchCompletes`
  (internal/dispatch/stop_test.go; closes BCD-R1-002): claim with a 1m limit
  and a running job record bound to the claim's machine, claim epoch, and
  revision; advance past the breach; close the fence through `goal.CloseStop`
  directly (the crash window: fence, no batch); release → parked-with-breach;
  `FindBreachStops` returns one BREACH route naming the stopId and the
  capability revision; `EnsureBreachStop(root, id, thatRevision, now)` creates
  the batch OPEN with the job pending; mark the job cancelled;
  `ReconcileStopBatch` → COMPLETE; `FindBreachStops` returns no route.
- `TestEveryLiveFenceIsOnTheRoute` (internal/dispatch/stop_test.go): table
  over shapes {claimed-and-fenced, parked-with-breach} × batch {absent, OPEN,
  INDETERMINATE, COMPLETE}: a route exists iff the batch is not COMPLETE, with
  Condition BREACH for absent/OPEN and INDETERMINATE for INDETERMINATE, as
  today's table for the claimed shape.
- `TestAgentDoneRefusesOverBreach`, `TestHumanDoneRequiresACompleteBatch`,
  `TestHumanDoneClearsBreach` (verbs_test.go): the agent refusal wording; a
  human done over an OPEN batch refuses naming the batch; over a COMPLETE
  batch the archive file carries no stop authority.
- `TestParkedLoneStopAuthorityRefused` (file_test.go): lone fence or lone
  capability on a parked goal is a parse problem with the exact wording.
- `TestHandParkOfAClaimedGoalDisplacesThePair` (reconcilepub_test.go:295,
  existing) stays green under the new tolerated set.
- `TestHandParkOfABreachStoppedGoalKeepsTheBreachRecord`
  (reconcilepub_test.go): the hand path produces the same parked-with-breach
  bytes as the verb.
- `TestHandDoneOfABreachParkedGoalRequiresACompleteBatch`
  (reconcilepub_test.go): OPEN → the replay refuses naming the batch;
  COMPLETE → archived without stop authority.
- `TestHandEditCannotDeleteAStandingFence` (reconcilemap_test.go): the exact
  refusal wording.
- `TestArcOperationsNameTheFencedMember` (verbs_test.go): an arc cascade over
  a fenced member refuses with the new `clearClaimBinding` wording.

Gate: the full Go test suite plus `metasystem validate` fixtures run by the
orchestrator outside the sandbox (KI-15); delegate briefs must not demand the
validation suite from inside a sandbox.

## Failure modes

- **Crash mid-migration.** There is no engine rewrite pass. The operational
  re-set is one atomic ledger commit per unfenced goal; a crash between goals
  leaves a tree in which every record is readable at its own meaning. No
  window exists where a budget has no defined semantics.
- **Mixed-era records.** Legacy claim + new raise: the raise pins the episode
  (legacy-inheritance rule). Legacy `d` budget under the new binary: eight
  hours per `d`, forever. New `h`/`m` budget under an old binary: identical
  hours; the old binary's normalizing writers may re-render it as a `d` token
  with identical enforcement (named in Fix 2's rollout table). Legacy stop
  batch: validates unchanged.
- **Concurrent raises.** Publication serializes on the ledger tip; each
  set-budget's Mutate reads the claim (and episode) from its own tip and the
  loser returns LostToCompetitor. The episode is inherited, so ANY
  serialization order preserves the same origin. A raise racing a release:
  whichever lands second sees the other's state — set-budget after release
  finds no claim to re-bind; release after raise demotes or queues exactly as
  if sequential.
- **Concurrent raise versus breach-stop.** Unchanged from today: set-budget
  refuses on a standing fence (verbs.go:514-515); breach-stop after a raise
  binds the raise's capability. The episode makes the ordering irrelevant to
  the clock.
- **Release racing the custodian.** Release lands between fence closure and
  batch creation: the custodian's third resolve finds parked-with-breach and
  creates the batch. The custodian dies before the batch exists: the next
  tick's route finds a live fence with no batch and creates it. The batch is
  OPEN or INDETERMINATE when release lands: the route persists, the steward
  keeps healing or reporting it, and resume and human done keep refusing
  until COMPLETE. In no ordering is a fenced revision's job left with no route
  to its cancellation.
- **Old governed-run snapshots.** Run records carry their dispatch-time
  `BudgetStartedAt` and Budget snapshot; runs dispatched before Fix 1 keep
  their old snapshot semantics, which is correct for their own terminal
  accounting; their tokens mean what they always meant.

## Deliberately unchanged

The reservation revision boundary and attempt/minutes filtering
(budget.go:339-360, 389-394, 419-424), the SetBudget fence refusal
(verbs.go:514-515), the quota and its wording (validate.go:250-283), the
duration parser and `Validate` (goalbudget/budget.go:14-49, 92-128), the stop
batch file format and its evidence validator, `ResolveGoalBinding` as the
launch-side gate, `GoalRecoveryPolicy.BreachStop`, the steward tick and the
shell cancellation loop, norm law (norm.go — minutes, not this grammar), and
everything in the steward and failed-job-attention seams named in the
non-goals.

## Critique round 1: disposition of every finding

| Finding | Disposition | Where | Tree seams verified |
| --- | --- | --- | --- |
| BCD-R1-001 (high): parked resume unreachable through the command | Closed by change: `ResolveStopAuthority`; the command resolves and locks on the capability revision; test at the command seam | Fix 3, "Resume, command"; proof plan | goalsync_mutations.go:394-413; dispatch/stop.go:41-64 |
| BCD-R1-002 (critical): release strands cancellation | Closed by change: the invariant "a fence is never off the route"; custodian resolves both shapes; routes include parked-with-breach; human done gated on COMPLETE; two named tests | Fix 3, cancellation-duty invariant | dispatch/stop.go:102-192, 288-314; tick.go:69-99; dispatch.sh:2267-2305 |
| BCD-R1-003 (high): raise after discharge rewinds | Closed by change: proofs bind to the episode; durable match on the proof's own pair; consequence for set-obligation stated; two tests | Fix 1, "Discharge proofs bind to the episode" | budget.go:77-164, 133-135, 146-157; verbs.go:122-124 |
| BCD-R1-004 (high): two claims, nondeterministic projections | Closed by change: quota unchanged; new only-claim tree invariant; release is the one step to workability; orientation names it; every consumer enumerated | Fix 3, "One claim per machine" | validate.go:250-283; goal.go:469-472; goalverbs.go:820-823; turnverdict.go:483-490; openwork.go:49; admission.go:57-105; verbs.go:1501 |
| BCD-R1-005 (high): marker not wired into the producer | Closed by change: the marker field is removed; the producer copies the stored token and the validator uses the same parser; test moved to the producer seam | Fix 2, "Stop firing evidence"; proof plan | dispatch/stop.go:134-139; goal/stop.go:145-157 |
| BCD-R1-006 (high): fail-closed claim false for run snapshots and journal | Closed by change: no ambiguous byte enters a new record; reader-by-reader table; the one old-binary display regression named for Wido | Fix 2, "Rollout and rollback" | run.go:123, 383-389; conclude.go:315-318; goal/budget.go:81-99 |
| BCD-R1-007 (medium): parked invariant breaks ordinary hand-park, no hand-done | Closed by change: split parse rule; tolerated set; `stopClearedByDone`; fence not hand-deletable; three operations and four tests named | Fix 3, "Parse invariants and the hand-edit mapper" | reconcilemap.go:131-147, 229-248; reconcilepub_test.go:295-326; file.go:289-291, 334-355 |
| BCD-R1-008 (high): migration cannot re-set a fenced budget | Closed by change: no engine migration; the seat re-sets unfenced goals only; a fenced goal's honest word arrives with the human's resume; specimen before/after shown; revision 1's instruction withdrawn, the refusal kept | Fix 2, "Migration" | verbs.go:514-515; stop.go:355-412; plans/goals/alert-escalation-channel.md:9, 33-34 |

## Self-grade

- **Confidence:** medium-high that every finding is closed against this
  tree, and high that no closure weakens a refusal or narrows a guarantee:
  two revision-1 promises were replaced by stronger mechanisms (release
  without a quota exclusion; a refused `d` instead of a marked one) and each
  replacement is named as such.
- **Weakest claim:** the episode-scoped discharge rule (Fix 1, BCD-R1-003)
  changes what a set-obligation does to a discharged clock inside one episode.
  I read it as a consequence of the goal's own thesis, and the alternative
  provably produces a favorable-direction clock move on a raise, but it is a
  behavior change on the governance seam that the goal's three defects did
  not name. The second-weakest is the reading of the second specimen: the
  goal record says `1d` enforces 24 hours; the parser at budget.go:38 says 8.
  I carried the code's reading and flagged the record's.
- **Reject condition:** reject this revision if the critic finds any
  publication path that can put a `StopFence` into the live tree in a state
  `FindBreachStops` does not route (that would falsify "a fence is never off
  the route"), or any writer left in the engine that emits a `d` token into a
  record (that would falsify "the marker is the letter itself"), or any
  consumer of the claimed set that can observe two claims on one machine
  after `ValidateCommit` has accepted the tree.
