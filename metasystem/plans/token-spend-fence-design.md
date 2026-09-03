# Token Spend Fence, step 1 (alert mode) — design, revision 2 (goal token-spend-fence)

Author m1b (Fable lane), 2026-09-03. Tier 3 under R-54-m1 and R-60-m1 (a
finding is material only if it changes what gets built; a disputed point at
the budget becomes a named test obligation). Wido's words: R-58-m1 (tokens
are the expensive resource), R-60-m1 (alert first; enforce only on his word;
configurable in the root config file), R-61-m1 (approved). NOTHING IS
REFUSED IN STEP 1. Changelog r2 (the one fold; plans/token-spend-fence-
dispositions.md): folded TSF-R1-alert-crossing-identity, TSF-R1-shared-checkout-double-count,
TSF-R1-seat-omission-honesty and TSF-R1-job-record-read-honesty (§2, §3, §6-8).

## 1. What exists and is reused (traced)

1. Every dispatched round's adapter writes one typed usage record to
   artifacts/agents/<root>/rounds/<n>/usage.json (claude: internal/adapter/
   claude.go:84-103, cost from `total_cost_usd`; codex: codex.go:41-46 over
   internal/usage/usage.go:177-193, cost null). It rides the result patch
   (runtime-common.sh:171-172; internal/adapter/patch.go:47-55) onto
   artifacts/agents/jobs/<job>.json, which carries usage, goalId, machineId,
   canonicalModelKey, runtime, startedAt, endedAt, status, round, parentJob
   (internal/dispatch/build.go:159,378,425) and the runtime session id:
   `sessionId` patched at the handshake (handshake.go:117); `resumedSessionId`
   and the parent's `sessionId` on a follow-up (build.go:574,617;
   record.go:69). Launch mode is `worktree` or `shared-checkout`
   (claim_fingerprint.go:30,108). Each round is its own record.
2. The mission fence turns a record into measured usage: `usageTokenFields`
   (internal/mission/fence.go:598); `addReportedUsage` (734-763: availability
   "unavailable" adds no tokens; cost or provider units still measure);
   `deriveRoundUsage` (772-834: event-stream recovery only under proven
   whole-group death, else pending-death-proof or unavailable, never zero);
   `AggregateUsage` (632-729) applies them to one mission's terminal jobs
   (`terminalJobStatus`, 36-38), skips a content-equal rewrite (724-728) and
   skips a record that fails to parse (657-660).
3. Health: `HealthStatus` alive|dead|unknown (internal/steward/health.go:
   33-38); `HealthRole`, `healthRoleOrder` (40-76); `RoleVerdict` (78-89);
   `applyHealthObservation` (323-414) makes every dead role alert-eligible
   (377-401); the finding digest hashes role=status pairs (1116-1125); the
   tick joins the verdict to the episode store (tick.go:271-295), where
   episodes are keyed by the whole digest: a healthy aggregate clears EVERY
   episode (alert_episode.go:246-268); a non-healthy verdict resolves every
   other digest's episode (270-279), finds or opens its own (286-302) and
   submits through a crash-safe attempt journal and `Deliver` (324-361;
   notify.go:40-62). No admission path reads the aggregate (only
   alert_episode.go:246 and internal/watch/watch.go:559,603 do).
4. Claude Code transcripts ~/.claude/projects/<slug>/<sessionId>.jsonl (slug
   = working directory with `/` → `-`; the file name IS the runtime session
   id: 492a4417-….jsonl equals jobs/fence-design-1.json `sessionId`) carry
   per assistant message timestamp, cwd, sessionId, requestId, message.model
   and message.usage; streaming repeats a line per content block with the
   SAME requestId and usage (364 of 643 lines on 2026-09-02). Cross-check:
   the worktree cap-settle-design transcript, deduplicated, sums to input
   3,026 / cache_read 10,845,332 / output 195,857 — exactly its four claude
   job records' sums; the adapter drops cache_creation (1,290,309).
5. Specimen 2026-09-02 on this checkout (m1b), records started that UTC day:
   16 jobs; 14 measured, 2 unavailable (design-critic-049b1ce02dea946074cba4f6,
   two-bars-cc-crit-2: failed, usage null). Measured: input 19,177,239;
   cached 35,305,212; output 514,996; reasoning 100,384; sum 55,097,831;
   native cost USD 67.911555 on the 7 claude-fable-5-1 rounds, null on the 7
   gpt-5-6-sol rounds; dispatch-cap-necessity 33,922,917 tokens (USD 38.34),
   two-bars-for-changes 21,174,914 (USD 29.57). The seat's transcript that
   day (279 requests): input 29,343; cache_creation 1,299,328; cache_read
   116,761,073; output 336,181 ≈ 118.4M tokens. Fleet-wide: 126 dispatches
   across four seats (goal record line 4).

## 2. The truth source: the spend reader

New package internal/spend, one function `Measure(repoRoot, machine string,
now time.Time) (Ledger, error)`; import direction steward → spend → mission,
usage, config, goal (no cycle). The per-record rule is lifted out of
`AggregateUsage` into `mission.JobUsageAt(repo, recordPath string)
JobMeasurement`, which reads and parses the file, then applies
`addReportedUsage` and `deriveRoundUsage` unchanged, returning {Record (nil
when unreadable), Tokens over `usageTokenFields`, Cost, ProviderUnit,
Provenance reported|derived|pending-death-proof|unavailable|unreadable,
Detail ("<file>: <error>" when unreadable)}. `AggregateUsage` calls it and
keeps its observable behaviour:
`unreadable` is skipped with `continue` as at 657-660 today; the other
outcomes map to today's provenance and unavailable list. One function serves
both because only the handling of `unreadable` differs: the mission
aggregator skips it; the spend reader records it. Rules, over the jobs dir:
1. An absent directory means zero records; one that cannot be listed makes
   Measure return the error (health unknown, §6). An `unreadable` record is
   an explicit unmeasured entry {file, error}: counted, never skipped.
2. Only a terminal `status` (`terminalJobStatus`) is measured; a non-terminal
   record counts in `inflight`, never as spend. Day = UTC date of `startedAt`
   (none parsable → unmeasured "no startedAt"); goal = `goalId` or "none";
   machine = `machineId` or the local nickname; model = `canonicalModelKey`.
3. Reported or derived: the four classes sum into (goal, machine, day,
   runtime, model); a null class adds nothing. Pending-death-proof,
   unavailable or unreadable: the record joins the unmeasured list (jobId or
   file, detail); NOTHING enters any total. Provider-unit-only usage (devin
   ACU) measures zero tokens, always unpriced.
4. Scopes: `day` = every measured record on this machine with that UTC day;
   `goal` = every measured record with that goalId across all days; both add
   the seat rows of §3. The ledger artifacts/agents/steward/spend/<UTC
   day>.json is written each tick with every row, list and count this design
   names, skipping a content-equal rewrite as fence.go:724-728 does.

## 3. The seat's own spend

Measurable today: Claude Code transcripts (§1.4) on the machine that runs
the seat. The reader first builds the delegate session set: every non-empty
`sessionId` and `resumedSessionId` on every readable job record (§1.1). It
walks ~/.claude/projects/<slug>/*.jsonl for every slug whose name starts
with the slug of the Git toplevel of repoRoot and keeps a line only when ALL
hold: the file's session id (its name, and the line's `sessionId`) is NOT in
the delegate set — excluding a delegate in any launch mode, including
shared-checkout whose cwd is the repository root; `type` is "assistant"; the
line's `cwd` is at or below the Git toplevel and NOT below artifacts/agents/
worktrees/ (the second guard). Lines deduplicate by `requestId`, last wins.
A kept request is MEASURED only when `message.usage` is an object whose
input_tokens and output_tokens are non-negative numbers and whose cache
fields, when present, are numbers; otherwise (usage absent, wrong type, or a
line that is not JSON) it is a seat unmeasured request with its reason,
never dropped. Mapping: inputTokens = input_tokens + cache_creation_input_
tokens; cachedInputTokens = cache_read_input_tokens; outputTokens =
output_tokens; reasoningTokens = thinking_tokens when present; cost null.
Attribution: day = UTC date of `timestamp`; machine = local nickname; runtime
"claude"; model = CanonicalModel(message.model); goal = "seat". Age: for the
DAY scope only, files with mtime older than 48 hours are skipped and counted
as `aged=<n>`; the GOAL scope (the seat's lifetime row) reads every present
file; both counts print.
Stated gaps, printed as explicit ledger lines, never folded: (a) seat→goal
unattributed: the current claim is derivable (`Claimed: machine=<m>
lineage=<mainId>`) but no record joins a seat transcript session to a mainId
(artifacts/agents/mains/session-60696-60696.json carries no session id; the
SessionStart signal writer runs only for delegates, cmd/metasystem/
adapter_runtime_verbs.go:328-345); interval
overlap needs one main per machine and a release time: obligation O-1 (§8).
(b) seat runtime codex: no meter; line `seat codex: unmeasured`.
(c) a purged transcript is invisible (the file count prints); a delegate
whose job record is unreadable is missing from the session set, so its
transcript would count as seat spend — the unreadable entry (§2.1) warns.

## 4. Money

Keys: `spend.currency` (default USD, three uppercase letters) and
`spend.price.<runtime>.<canonical-model>.<input|cached|output|reasoning>`,
a non-negative decimal in currency per million tokens; runtime rostered,
model canonical as cap keys are checked (internal/config/validate.go:
156-182 via `CanonicalModel`, model.go:15-18). Per record: (1) native cost
wins when its currency equals spend.currency; a foreign currency counts
beside the totals as `foreign=<n>`, never converted. (2) Otherwise derived
cost = Σ over classes with a non-null count of tokens × price ÷ 1,000,000;
any such class without a price row makes the record unpriced: tokens count,
money does not exist. (3) Scope money = Σ priced records as `<CUR><amount>`
(two decimals), always beside `unpriced=<n>`: the sum is a floor; tokens are
the truth. (4) The shipped conf carries the keys commented and NO price rows
(Wido's or the seat's to enter); until then codex and the seat show unpriced.

## 5. Ceilings and validation

Keys, all committed-root law through `budgetLawValue` (internal/config/
budget.go:89-119: root only outside a fixture root; .local and environment
refused), so "who can raise it" is one place: `spend.mode` (only `alert`;
`enforce` refused with "spend.mode=enforce is refused until step 2 lands on
Wido's word (R-60-m1)"; any other value refused); `spend.ceiling.day.tokens`
and `spend.ceiling.goal.tokens` (positive integers); `spend.ceiling.day.money`
and `spend.ceiling.goal.money` (positive decimals in spend.currency); the
validator (validate.go:22, knobs 327-440) registers them and names the key
in every refusal. A ceiling compares the SUM of the four classes. Day scope
is PER MACHINE-DAY, not fleet-summed: job records are gitignored
(.gitignore:1) and the only git-shared ledger, plans/goals, carries no usage
in its history (plans/goals/token-spend-fence.md:13-16); a fleet roll-up is
later work; each line and alert names the machine. Goal scope is likewise
this machine's view (`scope=machine`). Defaults from 2026-09-02 (§1.5, the
day the account hit its limit, so the first signal fires at that rate):
55,097,831 tokens ÷ 14 dispatches ≈ 3.94M each; 126 ÷ 4 seats = 31.5 per
machine-day → 124M, plus the seat's ≈118.4M → ≈242M →
`spend.ceiling.day.tokens=250000000`; claude 67.91 ÷ 7 = USD 9.70 per round
and 67.91 ÷ 18,002,340 ≈ USD 3.77/M tokens → 31.5 × 9.70 + 118.4M × 3.77/M ≈
752 → `spend.ceiling.day.money=750`; the largest goal-day was 33.9M for 8
rounds (4.24M each) and this goal's tuple allows attemptLimit=30 → 127M →
`spend.ceiling.goal.tokens=125000000`; 30 × 9.70 = 291 → `…goal.money=300`.

## 6. The health line

New role `RoleSpendFence HealthRole = "spend-fence"` after
RoleClaimedGoalDelivery in `healthRoleOrder`; `checkSpendFence(repoRoot,
now)` calls spend.Measure, reads the ceilings and takes the claimed goals
from the ledger (State claimed, `Claimed: machine=` as at plans/goals/fleet-
slack-channel.md:12 equal to the nickname from `git config metasystem.goal.
machine`, internal/goal/actor.go:15-27). Exact reason bytes — day segment,
seat segment, one per claimed goal (`goal=none` when none); money two decimals:

```
spend-fence=alive (mode=alert day=2026-09-02 tokens=173523756/250000000 money=USD67.91/750.00 unpriced=8 unmeasured=2 unreadable=0 inflight=0; seat tokens=118425925 lifetime=118425925 files=1 aged=0 unmeasured requests=0; goal=dispatch-cap-necessity tokens=33922917/125000000 money=USD38.34/300.00 unpriced=4 unmeasured=0 unreadable=0)
```

`unmeasured` counts every unmeasured record of the scope; `unreadable`
repeats its unreadable part so a parse failure is visible alone. The status
judges the meter: alive whenever Measure ran. On a crossing the reason is
prefixed `CROSSED <scope-id>.<ceiling>x<multiple>[,…] ` (multiple = floor of
spend ÷ ceiling) and the verdict carries the remedy `raise spend.ceiling.
<scope>.<ceiling> in metasystem.conf on Wido's recorded word (R-60-m1); alert
mode refuses nothing; see artifacts/agents/steward/spend/<day>.json`. Alive,
not dead: a dead role is alert-eligible through the health digest
(health.go:377-401) and would deliver a second, digest-keyed alert beside
§7's per-crossing episode; in alert mode a crossing is a reading, not a
failure with a remedy. Unknown only when the jobs directory exists but cannot
be listed, the goal ledger cannot be read, or the conf is invalid, naming the
failing path; the aggregate never turns unhealthy for spend (§1.3).

## 7. The alert: one episode per crossing

Mechanism: per-key episodes INSIDE the existing store, not a register file
beside it: the store already owns identity (`nextEpisodeID`, 177-192), the
crash-safe attempt journal (324-339), delivery (341-355) and acknowledgment
(366-392); a register file would re-implement the journal or lose it.
Changes to alert_episode.go, exactly three: (1) `AlertEpisode`
gains `Owner string` (`json:"owner,omitempty"`; empty = health-digest
episodes as today, "spend-fence" = ours). (2) The `UpdateAlertEpisodes`
loops that clear on healthy (246-268) and resolve on digest mismatch
(270-279) skip episodes whose Owner is non-empty: the health path never
touches a crossing's episode. (3) The attempt-and-deliver block
(324-361) becomes `submitEpisode(repoRoot string, episode *AlertEpisode, now
time.Time) error`, called unchanged by `UpdateAlertEpisodes` and by the new
`UpdateSpendEpisodes(repoRoot string, crossings []SpendCrossing, now
time.Time) error`, which the tick calls right after `UpdateAlertEpisodes`
(tick.go:293) under the same alerts lock. Identity: one crossing =
(scope-id, ceiling, multiple); scope-id `day-<UTC day>` or `goal-<id>`;
ceiling `tokens` or `money`; digest = sha256("spend-fence\n<scope-id>.
<ceiling>x<multiple>"); episode id `alert-<digest16>-<n>`. Per tick: a
crossing whose digest has no uncleared spend-owned episode opens one and
submits once; an uncleared TransportSubmitted one is left alone;
TransportFailed or Pending is retried by `submitEpisode`. Each further
multiple is a new digest, so a new episode; two ceilings crossed in one tick
are two episodes, two submissions. Clearance (the re-arm): every uncleared
spend-owned episode whose (scope-id, ceiling) has no crossing this tick at
any multiple is marked Resolved and Cleared now, whatever the other roles'
status; a recurrence opens a fresh id. A new UTC day is a new scope-id, so
yesterday's day crossings clear on today's first tick; a raised ceiling
clears the higher multiples the same way. The Message per episode is one
line with R-45-m0b's Option A facts: `SPEND CROSSED <scope-id>.<ceiling>
x<multiple> machine=<m> spend=<n>
ceiling=<n> ledger=artifacts/agents/steward/spend/<day>.json raise:
spend.ceiling.<scope>.<ceiling> in metasystem.conf on Wido's recorded word
(R-60-m1); alert mode refuses nothing`. Transport is `Deliver` today, the
fleet channel once it lands. No admission code imports or reads spend (§8).

## 8. Proof plan (fixture-driven, tests by name)

Fixture bed `internal/spend/testdata/bed-20260902/`: a fake-runtime root
with fixture ceilings and price rows; job records and round usage files
replaying §1.5 plus 1 round on model `unpriced-model`, 2 usage-null records
without pgid, 1 running record, 1 invalid-JSON file and 1 completed
shared-checkout claude record whose sessionId names a transcript;
transcripts: the seat's (streamed duplicates, worktree-cwd and foreign-cwd
lines, a usage-less request, an unparsable line), the delegate's (cwd =
repository root), one seat file three days old by mtime.
- internal/mission: `TestJobUsageAtMatchesAggregateUsage` (same units as
  AggregateUsage on the existing fixture; an unreadable file still skipped).
- internal/spend: `TestMeasureReplays20260902Bed` (totals per goal, machine
  and day equal §1.5; unmeasured names both unavailable jobs; inflight=1);
  `TestUnreadableJobRecordCannotDisappear` (the invalid-JSON file is an
  unmeasured entry naming file and error, unreadable=1, no Measure error; an
  unlistable directory is an error);
  `TestSeatTranscriptExcludesSharedCheckoutDelegateSession` (passes the cwd
  filter yet adds nothing; its tokens appear once, from the job record);
  `TestSeatTranscriptShapeFailureIsUnmeasured` (`unmeasured requests=2` with
  reasons); `TestSeatGoalDoesNotSilentlyLoseAgedTranscriptSpend` (old file:
  out of the day row, aged=1, in the lifetime row);
  `TestDayIsUTCDateOfStartedAt`; `TestNativeCostWinsOverPriceTable`;
  `TestUnpricedModelIsNeverZero`; `TestForeignCurrencyIsCountedBeside`;
  `TestSeatTranscriptDedupesByRequestId`; `TestSeatTranscriptFiltersByCwd`;
  `TestSeatCodexRuntimeIsUnmeasured`; `TestLedgerSkipsContentEqualRewrite`;
  `TestAdmissionNeverConsultsSpend` (`go list -deps` on ./internal/dispatch,
  ./internal/goal, ./internal/goalbudget has no internal/spend).
- internal/config: `TestValidateSpendKeys` (accepts the keys and a price
  row; refuses enforce mode, a non-canonical price model, a .local or
  environment source, a malformed decimal); `TestSpendCeilingDefaults`.
- internal/steward: `TestSpendFenceHealthLineBytes` (exact §6 bytes, below
  and crossed; the aggregate stays healthy);
  `TestSpendFenceCrossingsHaveIndependentEpisodesAndRearmWhileOtherRoleDead`
  (day tokens and money crossed in one tick: two episodes, two submissions;
  next tick: none; 2×: a third; retro-debt dead throughout neither resolves
  nor clears them; day tokens clears while retro-debt stays dead: Resolved
  and Cleared; it recurs: a new id, one submission; each Message holds the
  five facts).
O-1 (§3a): if the closing review holds seat→goal attribution material,
`TestSeatAttributesToSingleClaimInterval` becomes a named obligation.

## 9. Non-goals

Step 2's refusal; the reserved-minute pool (goal dispatch-cap-necessity);
prices as numbers and rows for non-roster models; the adapters' usage writers
(the dropped cache_creation tokens included); `AggregateUsage`'s behaviour;
a fleet-summed ledger; currency conversion; a CLI verb.

## 10. Self-grade

Confidence: high that the reader, money rule, validation and per-crossing
episode rules are mechanically determined by this text and the traced code;
medium on the defaults (one day of one machine scaled by a fleet count).
Weakest claim: the seat's measured-shape rule (§3) fixes "recognised usage"
from one Claude Code version; a renamed field turns the seat row into
unmeasured requests — loud, not silent, but the mapping then needs an update.
Reject if the Owner skip leaves a spend episode the health path can still
clear or re-deliver, or an alive crossing hides from an aggregate-only reader.
