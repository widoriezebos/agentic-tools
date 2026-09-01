# Spend-cap retirement design

Goal: native-spend-cap-retirement (goal file read at commit 9aa4dae4; the goal
lineage landed after this worktree's branch point, so it is cited by commit).
Wido's conditional mandate, verbatim from the goal: "KILL THIS STUPID CAP.
But... backlog, design, critique. build, critique build. NO EXCEPTION. Make
sure we do not inflict self-harm using this. The assumption is that we have
enough protection in the machinery already. The assumption is that this one is
a stupid one that actually harms us. If these are true (enough) then proceed
and kill this stupid idea."

Revision 2 (2026-09-01): folds the four material findings of the round-1
design critique (`records/misc/spend-cap-critique-r1.md`, chain
design-critic-20a8f1bdf17c3df866738e14, which affirmed the backstop
direction): SCR-R1-LEVEL-001 (the level, now derived from elapsed-time
evidence — the number changes), SCR-R1-PROTECTION-002 (the backstop's honest
accounting basis and overshoot; the pricing-anomaly justification is
withdrawn), SCR-R1-OWNER-003 (correct wall-clock owners and the backstop's
real purpose), and SCR-R1-ENVELOPE-004 (the no-fan-out claim is withdrawn;
one OPEN unowned spend scenario is recorded).

## Verdict up front

Assumption 2 (the cap harms us) is VERIFIED in full: the record shows at least
fourteen cap firings, every one killing legitimate work — nine of them
finished-but-unreported workers requiring paid hand recovery — and ZERO
recorded firings that stopped an actual runaway.

Assumption 1 (the machinery already bounds runaway spend) is VERIFIED while
the time enforcers are alive but REFUTED in the liveness-failure residual
(SCR-R1-OWNER-003): a live-but-wedged custodian whose adapter supervisor is
dead and whose dispatcher is unavailable has NO hard bound — accepted
explicitly as a residual in `records/misc/f4-orphan-window-design.md`
(lines 108–114) — and a mission host turn's cap is enforced by the mission
runner (`internal/missionrunner/host.go` `superviseHostToExit`, lines
421–450), so it too depends on another process's liveness. In those states
the native Claude limit is the ONLY bound left on the primary process.
Revision 1's pricing-anomaly refutation is withdrawn: the native counter
measures list cost, so it cannot own a provider billing anomaly at all
(SCR-R1-PROTECTION-002). A second, genuinely OPEN scenario — a
Bash-launched fresh paid session — is recorded below with no owner
(SCR-R1-ENVELOPE-004).

Per the mandate's own escape clause ("if a real unowned runaway scenario
emerges, the design says so and proposes the minimal owner instead"), this
design specifies the minimal owner: **the flag stays, the default rises from
$5.00 to a never-hit backstop of $150.00**, the level derived from measured
elapsed-time burn rates and the 120-minute wall-clock horizon the time
enforcers permit (SCR-R1-LEVEL-001; derivation in the specification). This
converts the cap from a thing that fires on finished work (all fourteen
recorded firings) into a last-line liveness backstop on the one primary
process, sized never to fire on legitimate work within the permitted
horizon. The clean-kill delta is specified as the named alternative should
the human overrule the residual-scenario reading.

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
   explicit `capDeadline`). The owners, corrected per SCR-R1-OWNER-003: the
   INNER owner is the detached adapter supervisor itself, which reads the
   record's own `capDeadline` and, on expiry, kills its whole process group
   from inside its wait loop (`scripts/agents/adapters/runtime-common.sh`
   `check_record_deadlines` / `enforce_expired_deadline` /
   `sweep_kill_domain`, lines ~206–305). The OUTER owner is the kill-capable
   dispatch reap ladder (`scripts/agents/dispatch.sh`, the `budget_expired`
   branch around lines 1046–1095), which runs `wind_down_group` on the
   recorded process group and stamps `timeout`/`budget-cap`. The standing
   supervision reaper (`internal/supervise/reaper.go`, lines 126–156) has NO
   kill authority over a live custodian: it lands the verdict only for a
   provably dead one and emits `REAP-DECLINED` for a live over-cap job. For
   mission host turns, the turn cap is enforced by the mission runner's
   `superviseHostToExit` loop (`internal/missionrunner/host.go` lines
   421–450), not by the host process itself. Each of these is a hard kill —
   WHILE its owner is alive. The accepted liveness residuals (wedged
   custodian with the dispatcher unavailable; dead runner during a host
   turn) are enumerated in the scenario table below; they are the
   backstop's real purpose.
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
   tool, so a Claude delegate has no NATIVE subagent tool. Revision 1's
   categorical claim that it "cannot fan out native subagents at all" is
   WITHDRAWN (SCR-R1-ENVELOPE-004): Bash is in the write envelope, and a
   Bash-capable delegate can launch a fresh paid `claude` session directly —
   including `claude --bg`, which the installed CLI (Claude Code 2.1.252)
   documents as starting a background session and returning immediately,
   with `claude agents` as its management surface. That path is the OPEN
   scenario in its own section below, not a covered one.

### The attack: runaway scenarios and their surviving owners

Burn-rate calibration from ELAPSED-TIME evidence (SCR-R1-LEVEL-001;
revision 1 wrongly divided round cost by ~40 *reserved* minutes to get
$0.13–$0.25/minute — reservation is not runtime, and
`plans/goals/dispatch-cap-necessity.md` records a 120-minute reservation for
a 10-minute run): the seven raw local results
(`artifacts/agents/*/rounds/*/claude-result.json`, `total_cost_usd` over
`duration_ms`) measure **$0.528–$0.974 per elapsed minute**, and the three
uncapped legitimate completions are the three HIGHEST rates ($0.729, $0.738,
and $0.974/min, on short rounds of 1.9–6.1 minutes; the four budget deaths
ran 8.1–9.9 minutes at $0.528–$0.641/min). Call $0.974/min the worst
observed legitimate burn.

| Scenario | Surviving owner | Rough dollar bound |
| --- | --- | --- |
| Expensive re-read loop in few turns | Wall-clock kill (inner adapter-supervisor `capDeadline`; outer dispatch reap ladder) | burn × capMin at measured rates: ≈$39 at a 40-min cap, ≈$117 at the built-in 120-min cap and the worst observed $0.974/min |
| Stuck stream (process alive, no output) | Wall-clock kill; watcher CAPPED inactivity signal surfaces it earlier | Near zero — a hung stream is not sampling; at most one in-flight turn |
| Native subagent fan-out | Task tool absent from the envelope (claude.go:217) | Zero — the tool does not exist in the delegate envelope |
| Bash-launched fresh paid session (incl. `claude --bg`) | **OPEN — no owner.** See the open-scenario section below (SCR-R1-ENVELOPE-004) | Unbounded by anything this repository controls; a foreground child in the recorded process group dies at capMin, but a background/detached session may escape wind-down and inherits no `--max-budget-usd` |
| Time enforcers unavailable: live-but-wedged custodian whose adapter supervisor is gone AND whose dispatcher is unavailable (f4-orphan-window-design.md:108–114 accepts "NO hard bound"); mission runner dead during a host turn (host.go:421–450 puts the turn cap in the runner) | **The native cap — this is its real purpose** (SCR-R1-OWNER-003) | Native default per process: $150.00 measured list cost, plus one indivisible call of overshoot (next section) |
| Provider billing anomaly invisible to list cost | **No owner in this repository** — the native counter measures list cost and never moves (SCR-R1-PROTECTION-002); provider-side account limits are not verifiable from here and this design does not lean on them | Unbounded here; explicitly NOT owned by the backstop |

The time-enforcers-unavailable row is the refutation of assumption 1 and the
backstop's justification: when every outer time owner is dead, the native
limit is the last fence on the primary process. Revision 1 put the
refutation on a pricing anomaly instead; that justification is dropped —
the last row shows the backstop cannot own it.

## What the backstop honestly is (SCR-R1-PROTECTION-002)

`--max-budget-usd` is a measured-cost THRESHOLD, not a hard bound. The CLI
checks a running cost counter between calls and refuses the NEXT call, so a
job always completes the indivisible call that crosses the line. Observed:
the four recorded budget deaths terminated at $5.047, $5.150, $5.218, and
$5.343 against a $5.00 threshold — up to ~$0.35 of overshoot on ordinary
calls. The worst overshoot is the list cost of ONE maximal indivisible call
past the threshold: on the priciest rostered model with a full context
window, order single dollars at list prices — and it scales with any
anomaly that list cost DOES reflect, because that one call cannot be
stopped mid-flight.

The counter's accounting basis is LIST cost, not authoritative provider
billing: every raw result records `costBasis: "list"` in its `modelUsage`
block. Therefore:

- **What the backstop can own:** bounding the CLI-measured list-cost spend
  of the ONE process the flag is passed to, to threshold-plus-one-call,
  even when every outer time enforcer is dead. That is the liveness-residual
  row above, and it is the whole justification.
- **What it cannot own:** billed dollars that diverge from list cost (an
  anomaly invisible to the counter never moves it), and any process it was
  not passed to (a Bash-spawned child never inherits the flag).

Revision 1's claim that the flag "caps a 10× pricing anomaly at $50 per
job" is withdrawn as false on both counts.

## Open unowned spend scenario (SCR-R1-ENVELOPE-004)

A Bash-capable delegate can launch a fresh paid Claude session with plain
`claude`, or with `claude --bg`, which Claude Code 2.1.252 documents as
starting a background session and returning immediately. Such a child
inherits no `--max-budget-usd`, is invisible to goal admission (which sees
metasystem dispatch records, not raw provider sessions), and — in
background or otherwise detached form — may escape the recorded process
group that capMin wind-down kills. The census surfaces such a process as
UNTRACKED, but census visibility is observation, not a spend bound.

**Honest exposure:** an uncapped paid session bounded in dollars by nothing
this repository controls, until a human acts on the census signal or a
provider-side account limit (unverifiable from here) intervenes. This
exposure predates this design — the $5.00 flag never bound a Bash-spawned
child either — so retiring the low cap neither widens nor narrows it; but
revision 1 wrongly listed it as covered, and it is recorded here as OPEN
and unowned. **Candidate owners:** (a) the delegate permission preset,
refusing `claude` invocation from delegate Bash; (b) extending census /
kill-guard reach over signature-matched agent processes in repository
scope; (c) a follow-on goal deciding the containment rule. Whether this
scenario must gain its owner before or after the cap change ships is a
disposition above this design; this design records it and claims no part
of it for the backstop.

The critic's unproven-residual gaps, recorded verbatim and NOT resolved
here:

> - The raw 8.19-dollar and 10.01-dollar ledger-attention results are not
>   present in this worktree; only their durable goal record is available.
>   I did not silently treat their unknown elapsed times as 40 minutes.
> - The repository contains no authoritative provider billing source or
>   proof that Claude Code's list-cost counter tracks a provider pricing
>   anomaly. Local CLI help and result artifacts establish only the native
>   threshold and reported list-cost behavior.
> - I did not execute `claude --bg` from inside another paid Claude session
>   because doing so would cross an external spend boundary. Whether
>   nested-launch protection refuses that exact command, and whether a
>   permitted background session remains in the recorded process group,
>   remains unproved and must not be assumed.

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
  one theoretically unique protection (the time-enforcers-unavailable row
  above) has never fired for that reason.

The harm mechanism is structural, not just a low number: the return protocol
is the LAST tokens of a session, so a cap sized near the typical round cost
preferentially executes exactly the workers that finished. A never-hit
backstop removes that failure mode; the write-early-return / cap-death
recollection fix remains goal budget-death-on-return's separate obligation
and is unaffected by this design.

## The specification

One code change, one test change, one comment.

1. **`internal/adapter/claude.go`, `ClaudeBudget` (line 228):** the default
   becomes `budget = "150.00"`. Nothing else in the function changes: the
   `METASYSTEM_CLAUDE_MAX_BUDGET_USD` override survives unchanged as the
   explicit operator tool (both directions — m0's recorded $15 design-round
   sizing keeps working, and an operator can still tighten), the validation
   regexp and the zero refusal stay, and both protocol errors stay:
   `invalid_native_budget` still guards a malformed override,
   `invalid_native_turn_limit` is untouched. Update the doc comment (lines
   220–226) to name the backstop's purpose: a last-line liveness backstop
   bounding the one primary Claude process's measured list-cost spend when
   the time enforcers are unavailable, sized from elapsed-time burn
   evidence never to fire on legitimate work within the 120-minute shipped
   horizon, with the wall-clock cap as the real per-job bound, citing this
   design.
2. **Why $150.00 (the derivation, SCR-R1-LEVEL-001):** the never-hit
   condition is level > worst-observed-legitimate-burn × the wall-clock
   horizon the time enforcers permit. Worst observed legitimate burn:
   $0.974 per elapsed minute — the highest of the seven measured raw
   rounds, itself a legitimate completion. Permitted horizon: 120 minutes,
   the shipped `dispatch.cap-min` default and the largest cap
   `metasystem.conf` grants without an explicit per-dispatch override.
   Product: ≈$117. Headroom for the sample's thinness (seven rounds, one
   workload class, the three legitimate rates all from rounds under seven
   minutes): round the rate up to $1.25/minute, giving 1.25 × 120 =
   **$150.00**. Revision 1's $50 was derived from reservation allowances
   rather than elapsed time and is withdrawn as dishonest arithmetic: at
   the observed $0.974/min, a legitimate round would hit $50 after about
   51 minutes — well inside the permitted horizon, reproducing exactly the
   fires-on-finished-work failure this design exists to kill. Operator
   rule: a dispatch explicitly capped above 120 minutes needs a matching
   `METASYSTEM_CLAUDE_MAX_BUDGET_USD` override sized at ≥ $1.25 per capped
   minute.
3. **Flag shape:** the flag continues to be passed always;
   `BuildClaudeCommand` (line 300) is untouched, so the argv byte order
   `claudecommand_test.go` pins is untouched. No "omit when unset" branch, no
   unlimited sentinel (the Claude CLI has no documented unlimited value and
   our own validator rejects 0 — omission would be the only kill shape; see
   the alternative below).
4. **Tests:** exactly one assertion changes —
   `internal/adapter/claudecommand_test.go` `TestClaudeBudgetPolicy` line 12,
   `budget != "5.00"` becomes `budget != "150.00"`. Every other occurrence of
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
   contract now gives its host the $150 backstop instead of a $5 tripwire,
   which is the fix, not a regression — the sealed time fences remain the
   mission's real exposure bound.
6. **Codex (and Devin):** no equivalent cap exists to judge.
   `scripts/agents/adapters/codex.sh` declares `"nativeBudget": false` (line
   78) and `internal/adapter/codex.go` builds no budget flag; Devin likewise
   (`devin.sh` line 88). Nothing is scoped in and nothing is deferred —
   there is no object.

**The named alternative (clean kill), if the human overrules the
liveness-residual row:** `ClaudeBudget` returns empty `budget` when the
environment variable is unset; `BuildClaudeCommand` omits the
`--max-budget-usd <value>` pair on empty budget (the CLI's contract treats
the flag as optional — omitted means no native spend limit); one new argv
test case pins the flag's absence; the override and both protocol errors
survive identically. This design recommends against it only because it
leaves the primary process with no bound at all in the liveness-failure
residual (the time-enforcers-unavailable row: wedged custodian with an
unavailable dispatcher, or a dead mission runner during a host turn),
where today the native limit is the last surviving fence.

## Self-grade

- **Confidence:** 0.85 that the backstop is the right call — the direction
  is now critic-affirmed ("directionally safer than the clean kill") — and
  0.7 that $150 is the right level: the derivation is honest arithmetic on
  measured elapsed-time rates, but its inputs are thin.
- **Weakest claims:** (a) the headroom step. The $1.25/minute rounding
  (≈28% above the worst observed legitimate rate) is judgment atop seven
  measured rounds from one workload class, with all three legitimate rates
  coming from rounds under seven minutes — a long legitimate round could
  sustain a rate no specimen has shown. (b) The three unproven residuals
  recorded verbatim in the open-scenario section, which this design leaves
  deliberately unresolved rather than settling by assertion.
- **Reject this design if:** (a) the human rules the liveness residual
  (wedged custodian with an unavailable dispatcher; dead runner during a
  host turn) acceptable unbounded, or owned by provider-side account
  limits (unverifiable from the repo) — then assumption 1 holds in full
  and the clean-kill alternative above is the right specification; (b)
  anyone produces a recorded firing where the $5 cap stopped a genuine
  runaway — none was found, but that evidence would re-price the whole
  trade; or (c) the human rules that the OPEN Bash-launch scenario must
  gain its owner before any cap change ships — then this design waits on
  that owner.
