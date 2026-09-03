# Token Spend Fence, step 1 (alert mode) — design (goal token-spend-fence)

Author m1b (Fable lane), 2026-09-03. Tier 3 under R-54-m1: this design, one
Sol review, one fold, one closing review, a Sol build, one Fable code review.
R-60-m1 binds the reviews: material only if it changes what gets built and
names the artifact; a disputed point at the budget becomes a named test
obligation. Wido's words: R-58-m1 (tokens are the expensive resource),
R-60-m1 (alert first; enforce only on his word; configurable in the root
config file), R-61-m1 (approved). NOTHING IS REFUSED IN STEP 1.

## 1. What exists and is reused (traced)

1. Every dispatched round's adapter writes one typed usage record to
   artifacts/agents/<root>/rounds/<n>/usage.json: claude via `claude-usage`
   (scripts/agents/adapters/claude.sh:93,173; writer internal/adapter/
   claude.go:84-103: inputTokens=input_tokens, cachedInputTokens=
   cache_read_input_tokens, outputTokens, reasoningTokens, cost from
   `total_cost_usd` as {amount, "USD"}); codex via `codex-usage`
   (codex.sh:89,168; internal/adapter/codex.go:41-46 over internal/usage/
   usage.go:177-193; cost always null). The usage rides the result patch
   onto the job record (runtime-common.sh:171-172; internal/adapter/
   patch.go:47-55), so artifacts/agents/jobs/<job>.json carries `usage` plus
   goalId, machineId, canonicalModelKey, runtime, startedAt, endedAt, status,
   round, parentJob (jobs/fence-design-1.json; internal/dispatch/build.go:
   159,378,425). Each round is its own record with its own usage
   (cap-settle-design-r2.json, parentJob cap-settle-design): summing records
   never double counts a chain.
2. The mission fence already turns a record into measured usage:
   `usageTokenFields` (internal/mission/fence.go:598), `addReportedUsage`
   (734-763: availability "unavailable" adds no tokens; a cost or provider
   unit still counts as measured), `deriveRoundUsage` (772-834: a terminal
   job without measured usage is recovered from its event stream only under
   proven whole-group death, else pending-death-proof or unavailable, never
   zero). `AggregateUsage` (632-729) applies them to one mission's terminal
   jobs (`terminalJobStatus`, 36-38), skipping a content-equal rewrite.
3. Config law shape: internal/config/budget.go:89-119 (`budgetLawValue`:
   committed root only outside a fixture root; .local and environment
   refused loudly); the validator (internal/config/validate.go:22) checks
   knobs at 327-440 and canonical model keys at 156-182 via `CanonicalModel`
   (internal/config/model.go:15-18).
4. Health: `HealthRole` constants and `healthRoleOrder` (internal/steward/
   health.go:40-76); `RoleVerdict` (78-89); `HealthVerdict.Line` (153-170)
   renders `role=status (reason; remedy: r)`; `applyHealthObservation`
   (323-414) sets ShouldAlert on the first dead observation without a
   lawful automatic remedy (387-389); the finding digest hashes only
   role=status pairs (1116-1125). The tick narrates the line and joins it to
   the episode store (tick.go:271-295; alert_episode.go:231-362: one open
   episode per digest; delivery via `Deliver`, notify.go:40-62). The
   aggregate is read by alert_episode.go:246 and shown by the watch snapshot
   (internal/watch/watch.go:559,603); no admission path reads it.
5. The machine's nickname is `git config metasystem.goal.machine`
   (internal/goal/actor.go:15-27; m1b here). A claimed goal names its
   machine: `- Claimed: machine=m0b lineage=main-… at=… revision=4`
   (plans/goals/fleet-slack-channel.md:12). The seat's enrollment (artifacts/
   agents/mains/session-60696-60696.json) carries pid, pgid, instanceTag,
   runtime — and NO runtime session id.
6. Claude Code transcripts ~/.claude/projects/<slug>/<session>.jsonl (slug =
   working directory with every `/` replaced by `-`, observed) carry per
   assistant message `timestamp`, `cwd`, `sessionId`, `requestId`,
   `message.model`, `message.usage` {input_tokens, cache_creation_input_tokens,
   cache_read_input_tokens, output_tokens, output_tokens_details.
   thinking_tokens}. Streaming repeats one line per content block with the
   SAME requestId and usage (364 of 643 assistant lines on 2026-09-02).
   Cross-check: the delegate transcript for worktree cap-settle-design,
   deduplicated by requestId, sums to input 3,026 / cache_read 10,845,332 /
   output 195,857 — exactly the four claude dispatch-cap-necessity records'
   sums. The mapping is faithful; it also shows the adapter drops
   cache_creation_input_tokens (1,290,309 there).
7. Job records live under artifacts/, gitignored (.gitignore:1). The only
   git-shared ledger, plans/goals, carries actor, op and targets in its
   history lines, never usage (plans/goals/token-spend-fence.md:13-16).
8. The 2026-09-02 specimen on this checkout (machine m1b), records with
   startedAt on that UTC day: 16 jobs; 14 measured, 2 unavailable
   (design-critic-049b1ce02dea946074cba4f6, two-bars-cc-crit-2: failed,
   usage null). Measured tokens: input 19,177,239; cached 35,305,212; output
   514,996; reasoning 100,384; sum 55,097,831. Native cost USD 67.911555 on
   the 7 claude-fable-5-1 rounds; the 7 gpt-5-6-sol rounds carry cost null.
   Per goal: dispatch-cap-necessity 33,922,917 tokens (claude 11,044,215,
   USD 38.34; codex 22,878,702); two-bars-for-changes 21,174,914 (claude
   6,958,125, USD 29.57; codex 14,216,789) plus the 2 unavailable. The
   seat's transcript that day (slug …-m1b-metasystem, 279 requests): input
   29,343; cache_creation 1,299,328; cache_read 116,761,073; output 336,181
   — about 118.4M tokens, over twice the machine's 16 dispatches together.
   Fleet-wide: 126 dispatches (goal record line 4) across four seats.

## 2. The truth source: the spend reader

New package internal/spend, one function `Measure(repoRoot, machine string,
now time.Time) (Ledger, error)`. Import direction steward → spend →
mission, usage, config, goal; mission imports neither steward nor spend
(verified from its import list), so no cycle. Reuse, not a second reader:
lift the per-record rule out of `AggregateUsage` into an exported
`mission.JobUsage(repo string, record map[string]any) JobMeasurement` —
`addReportedUsage` and `deriveRoundUsage` unchanged in semantics, returning
{Tokens map[field]float64 over `usageTokenFields`, Cost *{Amount,
Currency}, ProviderUnit *{Name, Value}, Provenance reported|derived|
pending-death-proof|unavailable, Detail}. `AggregateUsage` is rewritten to
call it; `TestAggregateUsageSumsTerminalJobs` stays green unchanged. Files
read: every artifacts/agents/jobs/*.json. Rules, in order:

1. Only records with a terminal `status` (`terminalJobStatus`) are measured.
   A non-terminal record is counted in `inflight`, never as spend.
2. Day = UTC date of `startedAt`; a terminal record without a parsable
   startedAt is unmeasured (detail "no startedAt"). Goal = `goalId`, or
   "none" when null. Machine = `machineId`, or the local nickname when null.
   Model = `canonicalModelKey`; runtime = `runtime`.
3. Provenance reported or derived: the four classes are summed into (goal,
   machine, day, runtime, model); a null class contributes nothing.
   Pending-death-proof or unavailable: the record joins the unmeasured list
   (jobId, detail); NOTHING is added to any total. Provider-unit-only usage
   (devin ACU) is measured with zero tokens and always unpriced.
4. Scopes: `day` = every measured record on this machine with that UTC day;
   `goal` = every measured record on this machine with that goalId across
   all days. Both add the seat rows of §3.
5. The ledger is written to artifacts/agents/steward/spend/<UTC day>.json
   each tick (machine, day, rows per goal×runtime×model, seat rows,
   unmeasured and inflight lists, transcript files read, prices, ceilings,
   mode), skipping a content-equal rewrite as fence.go:724-728 does. This
   file is the "where to look" every line and alert names.

## 3. The seat's own spend

Measurable today, on the machine that runs the seat: Claude Code transcripts
(§1.6). The reader walks ~/.claude/projects/<slug>/*.jsonl for every slug
whose name starts with the slug of the Git toplevel of repoRoot, skips files
whose mtime is older than 48 hours, and keeps a line only when ALL hold:
`type` is "assistant"; `message.usage` is present; the line's own `cwd` is
at or below the Git toplevel and NOT below artifacts/agents/worktrees/
(delegate sessions are jobs, already measured). Lines deduplicate by
`requestId`, last wins. Mapping: inputTokens = input_tokens +
cache_creation_input_tokens; cachedInputTokens = cache_read_input_tokens;
outputTokens = output_tokens; reasoningTokens = output_tokens_details.
thinking_tokens when present; cost null, so money is derived or unpriced.
Attribution: day = UTC date of `timestamp`; machine = local nickname;
runtime "claude"; model = CanonicalModel(message.model); goal = "seat".

Stated gaps, printed as explicit ledger lines and never folded:
(a) seat→goal: unattributed. The current claim is derivable (`Claimed:
machine=<m> lineage=<mainId>`), but no record joins a transcript `sessionId`
to a mainId: the enrollment carries pid and instanceTag only, and the
SessionStart signal writer runs only for delegates through
METASYSTEM_CLAUDE_SESSION_SIGNAL (cmd/metasystem/adapter_runtime_verbs.go:
328-345). Claim-interval overlap would need one enrolled main per machine
and the release time from history: obligation O-1 (§8), never assumed.
(b) seat runtime codex: no meter is known; line `seat codex: unmeasured`.
(c) a purged transcript is invisible; the ledger prints the file count. The
adapter's dropped cache_creation tokens (§1.6) are a recorded follow-up for
the adapters' owner; adapters are unchanged here.

## 4. Money

Keys in metasystem.conf: `spend.currency` (default USD, three uppercase
letters) and `spend.price.<runtime>.<canonical-model>.<input|cached|output|
reasoning>` — non-negative decimal, currency per million tokens, runtime in
`metasystem.runtimes`, model canonical exactly as cap keys are checked
(validate.go:156-182). Rules, per record then per scope:

1. Native cost wins: a record with Cost present is priced at its amount when
   its currency equals spend.currency; a foreign currency is counted beside
   the totals as `foreign=<n>`, never converted (no rate table is invented).
2. Otherwise derived cost = Σ over classes with a non-null count of tokens ×
   price ÷ 1,000,000. If any such class lacks a price row, the record is
   unpriced: its tokens still count, its money does not exist.
3. Scope money = Σ priced records as `<CUR><amount>` (two decimals), always
   beside `unpriced=<records>`: the sum is a floor. Tokens are the truth.
4. The shipped conf carries the keys commented and NO price rows: the
   roster's prices are Wido's or the seat's to enter from the providers'
   lists. Until rows land, codex rounds and the seat show unpriced.

## 5. Ceilings and validation

Keys, all committed-root law through `budgetLawValue`'s pattern (root only
outside a fixture root; .local and environment refused, so "who can raise
it" is one place): `spend.mode` (only `alert`; `enforce` refused with
"spend.mode=enforce is refused until step 2 lands on Wido's word
(R-60-m1)"; any other value refused); `spend.ceiling.day.tokens`,
`spend.ceiling.goal.tokens` (positive integers); `spend.ceiling.day.money`,
`spend.ceiling.goal.money` (positive decimals in spend.currency). The
validator registers the six keys and the price prefix in its knob block and
names the key in every refusal. A ceiling compares the SUM of the four
token classes; the breakdown stays in the ledger. Day scope is PER
MACHINE-DAY, not fleet-summed: the only git-shared ledger carries no usage
(§1.7), job records and transcripts are local, and a machine can only
measure what it holds; a fleet roll-up needs a shared spend ledger and is
later work, and each line and alert names the machine so four lines add up
by eye. Goal scope is likewise this machine's view; a goal worked on two
machines is under-counted and the ledger says `scope=machine`.

Defaults, derived from 2026-09-02 (§1.8), the day the account hit its limit,
so the first calibration signal fires at the recorded failure rate:
- tokens per measured dispatch = 55,097,831 ÷ 14 ≈ 3.94M;
- dispatches per machine-day = 126 ÷ 4 seats = 31.5 → 124M tokens; the seat
  added ≈118.4M → all-in ≈ 242M → `spend.ceiling.day.tokens=250000000`;
- claude money per round = 67.91 ÷ 7 = USD 9.70; per token 67.91 ÷
  18,002,340 ≈ USD 3.77 per million; day money = 31.5 × 9.70 + 118.4M ×
  3.77/M ≈ 306 + 446 = 752 → `spend.ceiling.day.money=750`;
- the largest specimen goal-day was 33.9M for 8 rounds (4.24M per round);
  this goal's tuple allows attemptLimit=30 (goal record line 10) → 127M →
  `spend.ceiling.goal.tokens=125000000`; 30 × 9.70 = 291 → `…goal.money=300`.

## 6. The health line

New role `RoleSpendFence HealthRole = "spend-fence"`, in `healthRoleOrder`
after RoleClaimedGoalDelivery, evaluated by `checkSpendFence(repoRoot, now)`
from `evaluateHealthRoles`: it calls spend.Measure and reads the ceilings;
the claimed goals are the ledger's goals with State claimed and
Claimed.machine equal to the local nickname. Exact reason bytes (one
segment per claimed goal, `goal=none` when none; money two decimals):

```
spend-fence=alive (mode=alert day=2026-09-02 tokens=173523756/250000000 money=USD67.91/750.00 unpriced=8 unmeasured=2 inflight=0; goal=dispatch-cap-necessity tokens=33922917/125000000 money=USD38.34/300.00 unpriced=4 unmeasured=0)
```

Status: alive below every ceiling; dead at or above any, the reason prefixed
`CROSSED <scope>.<ceiling>x<multiple>[,…] ` (multiple = floor of spend ÷
ceiling), `NoAutomaticRemedy=true`, remedy `raise
spend.ceiling.<scope>.<ceiling> in metasystem.conf on Wido's recorded word
(R-60-m1); alert mode refuses nothing; see artifacts/agents/steward/spend/
<day>.json`; unknown when the jobs directory, ledger or conf is unreadable,
naming the failing path. A dead spend role makes the aggregate "unhealthy":
the narration line, exit code 1 of `metasystem health` and the watch label
follow; nothing admits or refuses on it (§1.4).

## 7. The alert

One change to shared machinery: `RoleVerdict` gains `EpisodeKey string`
(`json:"episodeKey,omitempty"`) and `healthFindingDigest` hashes
`role=status:episodeKey` when set (health.go:1116-1125). The spend role sets
EpisodeKey to the sorted list `<scope-id>.<ceiling>x<multiple>` (scope-id
`day-2026-09-02` or `goal-<id>`). Through the existing store, no new
machinery: the same crossing keeps the same digest, so `UpdateAlertEpisodes`
finds the open episode and submits nothing (alert_episode.go:286-291,
315-322); the next multiple changes the digest, so a new episode opens and
delivers once (292-302,341-355); a new UTC day is a new scope-id. Another
role changing status also changes the digest — today's behaviour. The
episode Message is the health line (tick.go:283,293) and carries R-45-m0b's
Option A facts: what crossed (the CROSSED prefix), the spend and the
ceiling, where to look (the ledger path), who can raise it (Wido's word,
the key). Transport is `Deliver` today; when fleet-slack-channel lands, its
channel posts the same Message (its design §9 names episodes as riding that
package). Nothing refuses: no code in internal/dispatch, internal/goal or
internal/goalbudget imports or reads spend (proof in §8).

## 8. Proof plan (fixture-driven, tests by name)

Fixture bed `internal/spend/testdata/bed-20260902/`: a fake-runtime root
(`metasystem.runtimes=fake`, fixture ceilings and price rows), job records
and round usage files replaying §1.8's shape — 7 claude rounds with native
cost, 7 codex rounds priced by fixture rows, 1 round on model
`unpriced-model`, 2 records with usage null and no pgid (unavailable by
fence.go:773-776), 1 running record — and one transcript fixture with
streamed duplicates, one worktree-cwd line and one foreign-cwd line.
- internal/mission: `TestJobUsageMatchesAggregateUsage` (the lifted rule
  returns exactly the units AggregateUsage sums for the existing fixture).
- internal/spend: `TestMeasureReplays20260902Bed` (per-goal, per-machine,
  per-day totals equal the §1.8 integers; unmeasured=2 names both jobs;
  inflight=1; nothing from an unavailable record enters any total);
  `TestDayIsUTCDateOfStartedAt` (a round starting 23:59Z, ending next day,
  counts on the start day); `TestNativeCostWinsOverPriceTable`;
  `TestUnpricedModelIsNeverZero`; `TestForeignCurrencyIsCountedBeside`;
  `TestSeatTranscriptDedupesByRequestId` (cache_creation folds into input);
  `TestSeatTranscriptFiltersByCwd`; `TestSeatCodexRuntimeIsUnmeasured`;
  `TestLedgerSkipsContentEqualRewrite`; `TestAdmissionNeverConsultsSpend`
  (`go list -deps` on ./internal/dispatch, ./internal/goal, ./internal/
  goalbudget has no internal/spend; admission.go and budget.go grep clean).
- internal/config: `TestValidateSpendKeys` (six keys and a price row
  accepted; `spend.mode=enforce`, a non-canonical price model, a .local or
  environment source, a malformed decimal each refused by name);
  `TestSpendCeilingDefaults` (§5 numbers).
- internal/steward: `TestSpendFenceHealthLineBytes` (the exact §6 bytes with
  ceilings above and below the totals); `TestSpendFenceCrossingOpensOneEpisode`
  (first crossing: one episode, one submission; same multiple next tick:
  none; 2× spend: a second; the Message holds the five facts);
  `TestSpendFenceDeadHasNoAutomaticRemedy` (no revive, no repair).
- Shell: `scripts/agents/spend-fence-fixtures.sh` runs `metasystem health`
  on a clone of the bed and diffs the role item (orchestrator-run, KI-15).
Obligation O-1 (§3a): if the Sol review holds seat→goal attribution material,
the fold adds `TestSeatAttributesToSingleClaimInterval`; else it stays an
explicit unmeasured line.

## 9. Non-goals

Step 2's refusal and anything the dispatcher would do with the ledger; the
reserved-minute pool (settled by goal dispatch-cap-necessity); price rows
for models outside the roster, and the roster's own prices as numbers; any
change to the adapters' usage writers, including the dropped cache_creation
tokens; a fleet-summed ledger; currency conversion; a new CLI verb.

## 10. Self-grade

Confidence: high that the reader, the money rule and the validation are
mechanically determined by this text and the traced code; medium on the
defaults (one day of one machine scaled by a fleet count). Weakest claim:
the seat mapping (§3) rests on one delegate transcript matching its usage
records exactly; a Claude Code release that changes the transcript's usage
shape breaks the meter silently, hence the file count and the shape test.
Reject if a reviewer shows that `EpisodeKey` in the finding digest re-alerts
an unchanged crossing (then a crossing register must own its own dedupe
file), or that any admission path already reads health's aggregate.
