# Spend-cap retirement design

Goal: native-spend-cap-retirement (goal file read at commit 9aa4dae4; the goal
lineage landed after this worktree's branch point, so it is cited by commit).
Wido's conditional mandate, verbatim from the goal: "KILL THIS STUPID CAP.
But... backlog, design, critique. build, critique build. NO EXCEPTION. Make
sure we do not inflict self-harm using this. The assumption is that we have
enough protection in the machinery already. The assumption is that this one is
a stupid one that actually harms us. If these are true (enough) then proceed
and kill this stupid idea."

## Verdict up front

Assumption 2 (the cap harms us) is VERIFIED in full: the record shows at least
fourteen cap firings, every one killing legitimate work — nine of them
finished-but-unreported workers requiring paid hand recovery — and ZERO
recorded firings that stopped an actual runaway.

Assumption 1 (the machinery already bounds runaway spend) is VERIFIED for
every time-bounded scenario but REFUTED in one narrow case: a burn-rate
anomaly (a model-side pricing surprise, or a burn far above anything
observed). The wall-clock kill bounds *time*; its dollar equivalent is
burn-rate × minutes, and the burn-rate assumption is exactly what a pricing
anomaly breaks. No surviving mechanism is denominated in dollars.

Per the mandate's own escape clause ("if a real unowned runaway scenario
emerges, the design says so and proposes the minimal owner instead"), this
design specifies the minimal owner: **the flag stays, the default rises from
$5.00 to a never-hit backstop of $50.00.** This converts the cap from a thing
that fires on finished work (all fourteen recorded firings) into a thing that
fires only under a genuine anomaly, while keeping a dollar-denominated fence
in the machinery. The clean-kill delta is specified as the named alternative
should the human overrule the residual-scenario reading.

## The object under judgment

`internal/adapter/claude.go` `ClaudeBudget` (lines 227–243): hardcoded default
budget `"5.00"`, overridable only by `METASYSTEM_CLAUDE_MAX_BUDGET_USD`,
passed unconditionally by `BuildClaudeCommand` (line 300) as
`--max-budget-usd` to every Claude delegate and host turn. Landed 2026-08-14
(commit 24345044) inside an argv consolidation; never designed, never
critiqued. The sibling turn guard (150, same function) is out of scope — it
already had its recalibration (issue #6, comment at claude.go:222–224) and
guards infinite loops, a different failure; the inventory below confirms
nothing forces it back in scope.

## Assumption 1: the protection inventory

Mechanisms that survive the cap's death, with evidence:

1. **Per-job wall-clock cap and kill.** Every dispatch carries `capMin`,
   resolved by the chain in `internal/dispatch/cap.go` `ResolveCap` (explicit
   argument → `cap.min.<role>.<runtime>.<model>` → `cap.min.<runtime>.<model>`
   → `dispatch.cap-min` → built-in 120; `metasystem.conf` sets
   `dispatch.cap-min=120`). `internal/dispatch/reapfacts.go` `CapExpired`
   (lines 138–152) is THE budget verdict: `startedAt` + `capMin` (or an
   explicit `capDeadline`). The kill-capable dispatch reap ladder
   (`scripts/agents/dispatch.sh`, the `budget_expired` branch around lines
   1046–1095) runs `wind_down_group` on the recorded process group and stamps
   `timeout`/`budget-cap`; the supervision reaper
   (`internal/supervise/reaper.go`) lands the same verdict for a provably
   dead custodian and emits `REAP-DECLINED` for a live over-cap job so the
   dispatch path takes it. This is a hard kill, not a report.
2. **Native turn limit (150).** Stays. Bounds turn count, not dollars — a
   maximally context-heavy turn on the priciest rostered model costs on the
   order of a dollar or more, so 150 turns is a weak dollar bound (order of
   low hundreds worst case). It is the infinite-loop guard, and it fires
   before the wall clock only for genuinely turn-dense loops.
3. **Goal budget tuple at admission.** `internal/dispatch/admission.go`
   (lines 156–196) refuses a dispatch whose proposed cap would exceed the
   remaining `reservedJobMinutesLimit`, and breaches `attemptLimit`,
   `reservedJobMinutesLimit`, and `activeJobLimit`. `internal/run/conclude.go`
   (line 317) trips the governed exhaustion breaker on the same tuple. Total
   per-goal exposure is therefore reservedJobMinutesLimit × burn-rate, and
   the goal norm (`internal/goal/norm.go`) caps the tuple itself without a
   recorded human approval.
4. **Attempt limits and breach fences.** Same tuple (`dispatch/budget.go`,
   `admission.go` line 189): a goal cannot re-dispatch its way past
   `attemptLimit`; a budget-cap death inside a mission additionally files a
   mission fence refusal (`dispatch.sh` `mission_fence refuse
   --reason job-cap-min`, reaper.go line 205 `mission.RefuseBudgetCap`),
   which parks rather than retries.
5. **Mission fences.** `fence.wall-clock-hours`, `fence.jobs`,
   `fence.concurrency`, `fence.job-cap-min` — serialized reservations in
   `fences.json`; a sealed exposure the contract signs.
6. **Stop machinery and observation.** The cancel path (record-marked before
   kill, dispatch.sh), the watcher's CAPPED inactivity ceiling
   (`watch.cap-min=180` in `metasystem.conf`), the census's UNTRACKED
   surfacing, and stop-loss conduct for chains.
7. **Tool-envelope narrowing.** The delegate tool list (`claude.go:217`,
   `Bash,Edit,Write,Read,Glob,Grep,NotebookEdit`) does NOT include the Task
   tool: a Claude delegate cannot fan out native subagents at all.

### The attack: runaway scenarios and their surviving owners

Burn-rate calibration from the record: the specimens spent $5.07–$10.01
inside ~40-minute reservations, i.e. roughly $0.13–$0.25 per minute for
legitimate context-heavy design work; call a pathological re-read loop
$0.50/minute sustained.

| Scenario | Surviving owner | Rough dollar bound |
| --- | --- | --- |
| Expensive re-read loop in few turns | Wall-clock kill (`CapExpired` + `wind_down_group`) | burn × capMin: ~$10–20 at a 40-min cap, ~$30–60 at the built-in 120 |
| Stuck stream (process alive, no output) | Wall-clock kill; watcher CAPPED inactivity signal surfaces it earlier | Near zero — a hung stream is not sampling; at most one in-flight turn |
| Worker spawning subagents | Task tool absent from the envelope (claude.go:217); a `claude` child launched via Bash sits in the recorded process group and dies with it at capMin; a deliberately detached evader is the census's named completeness exclusion | Same wall-clock bound. Note: `--max-budget-usd` binds only the one CLI process — a Bash-spawned child never inherited it, so the dollar cap NEVER owned this scenario and its death loses nothing here |
| Model-side pricing surprise / burn-rate anomaly | **No surviving dollar-denominated owner.** Wall clock bounds minutes; the dollar equivalent scales linearly with the anomaly (a 10× price error turns a $20 job into $200, further multiplied across a goal's reserved pool before a human notices) | Unbounded in dollars per unit anomaly; bounded only in minutes |

The last row is the refutation. It is narrow — no such anomaly is on record —
but the brief's test is "a plausible dollar exposure beyond what a wall-clock
kill bounds," and a rate anomaly is exactly the case where a time bound is
not a dollar bound. Provider-side account limits may exist, but they are not
verifiable from this repository and this design does not lean on them.

## Assumption 2: the harm record

- **budget-death-on-return** (goal, m1/m2, 2026-08-30): three consecutive
  Fable delegates on the ledger-attention design chain completed their entire
  product and died on the native cap during the return protocol, at $5.07,
  $8.19, and $10.01. Every product recovered whole by hand from stream or
  worktree. The goal's root-cause note (2026-09-01) adds five more two-bars
  design sessions dead at $5.4–5.6 — the $5.00 default plus final-iteration
  overshoot.
- **alert-escalation-channel** (goal, m0b, 2026-09-01): six identical runtime
  deaths in one day — authoring rounds r7/r8/r9 and even the r9 RECOVERY
  round died at the $5 cap after finishing work and before returning. Each
  cost a paid recovery round and 40 reserved pool-minutes; the chain ended in
  a seat stop-loss HALT. The cost driver was legitimate: a ~1000-line design
  plus register is most of $5 before writing begins.
- **What the cap ever prevented:** nothing on record. Every recorded firing
  killed finished or legitimately mid-flight work. The cap fires only AFTER
  spend, cannot distinguish runaway from expensive legitimate work, and its
  one theoretically unique protection (the rate-anomaly row above) has never
  fired for that reason.

The harm mechanism is structural, not just a low number: the return protocol
is the LAST tokens of a session, so a cap sized near the typical round cost
preferentially executes exactly the workers that finished. A never-hit
backstop removes that failure mode; the write-early-return / cap-death
recollection fix remains goal budget-death-on-return's separate obligation
and is unaffected by this design.

## The specification

One code change, one test change, one comment.

1. **`internal/adapter/claude.go`, `ClaudeBudget` (line 228):** the default
   becomes `budget = "50.00"`. Nothing else in the function changes: the
   `METASYSTEM_CLAUDE_MAX_BUDGET_USD` override survives unchanged as the
   explicit operator tool (both directions — m0's recorded $15 design-round
   sizing keeps working, and an operator can still tighten), the validation
   regexp and the zero refusal stay, and both protocol errors stay:
   `invalid_native_budget` still guards a malformed override,
   `invalid_native_turn_limit` is untouched. Update the doc comment (lines
   220–226) to name the backstop's purpose: a burn-rate-anomaly fence sized
   never to fire on legitimate work, with the wall-clock cap as the real
   per-job bound, citing this design.
2. **Why $50.00:** more than 4× the worst legitimately observed finished
   round ($10.01); more than 3× the deliberate $15 design-round override m0
   recorded as a thought-through spend decision; at or above the
   normal-price worst case a wall-clock kill already tolerates (~$30–60 at
   the built-in 120-minute cap and pathological burn); and it caps a 10×
   pricing anomaly at $50 per job instead of unbounded. A delegate round
   that legitimately reaches $50 is outside every recorded norm and deserves
   the stop plus recollection.
3. **Flag shape:** the flag continues to be passed always;
   `BuildClaudeCommand` (line 300) is untouched, so the argv byte order
   `claudecommand_test.go` pins is untouched. No "omit when unset" branch, no
   unlimited sentinel (the Claude CLI has no documented unlimited value and
   our own validator rejects 0 — omission would be the only kill shape; see
   the alternative below).
4. **Tests:** exactly one assertion changes —
   `internal/adapter/claudecommand_test.go` `TestClaudeBudgetPolicy` line 12,
   `budget != "5.00"` becomes `budget != "50.00"`. Every other occurrence of
   `5.00` is an explicit argument, not the default: the argv-pinning tests
   pass `"5.00"` as the `budget` parameter (claudecommand_test.go lines 52,
   77, 99, 115; events_test.go line 221) and pin wire byte order, which this
   design does not touch. `internal/contract/contract_test.go` line 174 uses
   `host.max-budget-usd=5.00` as a sealed-contract fixture value — contract
   validation, not adapter policy — untouched.
5. **Mission interaction:** unchanged. A contract that seals
   `host.max-budget-usd` still exports `METASYSTEM_CLAUDE_MAX_BUDGET_USD`
   for the host turn (`internal/missionrunner/host.go` lines 320–322) and
   overrides the new default exactly as it overrode the old one; an unsealed
   contract now gives its host the $50 backstop instead of a $5 tripwire,
   which is the fix, not a regression — the sealed time fences remain the
   mission's real exposure bound.
6. **Codex (and Devin):** no equivalent cap exists to judge.
   `scripts/agents/adapters/codex.sh` declares `"nativeBudget": false` (line
   78) and `internal/adapter/codex.go` builds no budget flag; Devin likewise
   (`devin.sh` line 88). Nothing is scoped in and nothing is deferred —
   there is no object.

**The named alternative (clean kill), if the human overrules the
rate-anomaly row:** `ClaudeBudget` returns empty `budget` when the
environment variable is unset; `BuildClaudeCommand` omits the
`--max-budget-usd <value>` pair on empty budget (the CLI's contract treats
the flag as optional — omitted means no native spend limit); one new argv
test case pins the flag's absence; the override and both protocol errors
survive identically. This design recommends against it only because it
leaves the rate-anomaly row with no owner.

## Self-grade

- **Confidence:** 0.8 that the backstop is the right call and the level is
  right to within a factor of two.
- **Weakest claim:** the dollar equivalents. They are extrapolated from
  fourteen specimen firings on one workload class (design rounds); no
  per-minute burn telemetry exists in this repository, and the
  rate-anomaly scenario that justifies keeping the flag at all has never
  actually occurred. The $50 level is judgment sized off three numbers
  ($10.01 worst legitimate, $15 recorded override, ~$60 wall-clock worst
  case), not a distribution.
- **Reject this design if:** (a) the human rules that provider-side account
  limits (unverifiable from the repo) are the true dollar owner — then
  assumption 1 holds in full and the clean-kill alternative above is the
  right specification; or (b) anyone produces a recorded firing where the
  $5 cap stopped a genuine runaway — none was found, but that evidence
  would re-price the whole trade.
