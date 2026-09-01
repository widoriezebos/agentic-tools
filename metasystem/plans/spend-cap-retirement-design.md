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

Revision 3 (2026-09-01): folds the two material findings of the round-2
critique (`records/misc/spend-cap-critique-r2.md`, chain
design-critic-0671f76f743c4926c1a39e8f): SCR-R2-LEVEL-001 (the level is
rederived against the TRUE permitted horizon — the 150-minute derived
watcher ceiling, not the 120-minute dispatch cap — and now satisfies the
strictly-greater rule with stated headroom; the default becomes $200.00)
and SCR-R2-PROTECTION-002 (the worst-single-call overshoot is repriced at
low tens of dollars from the installed catalog, and the exposure bound is
restated). It also corrects the one impossible-scenario sentence of
non-material SCR-R2-OWNER-003 (the custodian IS the adapter supervisor)
and records Wido's disposition R-42-m0b (`memory/rulings.md`): the open
Bash-launched-session scenario does NOT block this change and is owned by
goal uncapped-delegate-fanout.

Revision 4 (2026-09-01): the final fold. Wido's recorded ruling R-43-m0b
(`memory/rulings.md`, verbatim "b it is, extra budget approved") chooses
the design's own fully-specified Option-B alternative — the CLEAN KILL —
over the $200.00 backstop. The specification below is now Option B; the
backstop's level derivation, horizon coupling, and overshoot sizing move
whole to the rejected-alternative record with the ruling's reasoning; and
the round-3 critique's finding SCR-R3-LEVEL-001 is DISSOLVED BY DECISION —
the number it calibrated no longer exists (R-43-m0b). The open scenario
(goal uncapped-delegate-fanout) and the enforcer-liveness residuals stay
recorded exactly as they were, with unchanged ownership.

## Verdict up front

Assumption 2 (the cap harms us) is VERIFIED in full: the record shows at least
fourteen cap firings, every one killing legitimate work — nine of them
finished-but-unreported workers requiring paid hand recovery — and ZERO
recorded firings that stopped an actual runaway.

Assumption 1 (the machinery already bounds runaway spend) is VERIFIED while
the time enforcers are alive but REFUTED in the liveness-failure residual
(SCR-R1-OWNER-003): a live-but-wedged custodian — the adapter supervisor
itself, its own deadline enforcement wedged (SCR-R2-OWNER-003: the
custodian IS the adapter supervisor, per f4-orphan-window-design.md) —
with the dispatcher unavailable has NO hard bound — accepted
explicitly as a residual in `records/misc/f4-orphan-window-design.md`
(lines 108–114) — and a mission host turn's cap is enforced by the mission
runner (`internal/missionrunner/host.go` `superviseHostToExit`, lines
421–450), so it too depends on another process's liveness. In those states
the native Claude budget limit WAS the only dollar bound left on the
primary process; under this revision the in-process turn limit is the
surviving in-process coarse dollar bound there (R-43-m0b).
Revision 1's pricing-anomaly refutation is withdrawn: the native counter
measures list cost, so it cannot own a provider billing anomaly at all
(SCR-R1-PROTECTION-002). A second, genuinely OPEN scenario — a
Bash-launched fresh paid session — is recorded below
(SCR-R1-ENVELOPE-004); per Wido's disposition R-42-m0b it does not block
this change and is owned by goal uncapped-delegate-fanout.

Revisions 2–3 answered the residual with a minimal owner — a never-hit
$200.00 backstop. Wido's recorded ruling R-43-m0b overrules that reading
and resolves the mandate to the CLEAN KILL: **`ClaudeBudget` passes NO
`--max-budget-usd` when `METASYSTEM_CLAUDE_MAX_BUDGET_USD` is unset**; a
set override is validated exactly as before (`invalid_native_budget`
survives for a malformed value); time (the wall-clock caps and the
watchdog machinery) and count (the 150-turn limit) are the law, with the
in-process turn limit explicitly the surviving in-process coarse dollar
bound. The backstop is recorded below as the rejected alternative; it lost
because it was a forever-recalibrated constant buying a tighter number on
a bound the turn count already provides, in a scenario never observed
(R-43-m0b). The round-3 finding SCR-R3-LEVEL-001, which calibrated the
backstop's level, is DISSOLVED BY DECISION — the number it calibrated no
longer exists (R-43-m0b).

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
   turn) are enumerated in the scenario table below; they were the
   rejected backstop's stated purpose, and per R-43-m0b the turn limit is
   their surviving in-process coarse dollar bound.
2. **Native turn limit (150).** Stays. Bounds turn count, not dollars — a
   maximally context-heavy turn on the priciest rostered model costs on the
   order of a dollar or more, so 150 turns is a weak dollar bound (order of
   low hundreds worst case). It is the infinite-loop guard, and it fires
   before the wall clock only for genuinely turn-dense loops. Per
   R-43-m0b it is explicitly the SURVIVING IN-PROCESS COARSE DOLLAR BOUND
   under this specification: when every outer time enforcer is dead, this
   count is what still limits the primary process's spend.
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
| Bash-launched fresh paid session (incl. `claude --bg`) | **OPEN — owned by goal uncapped-delegate-fanout** (R-42-m0b; does not block this change). See the open-scenario section below (SCR-R1-ENVELOPE-004) | Unbounded by anything this repository controls; a foreground child in the recorded process group dies at capMin, but a background/detached session may escape wind-down and inherits no `--max-budget-usd` |
| Time enforcers unavailable: live-but-wedged custodian — the adapter supervisor itself, its own deadline enforcement wedged (SCR-R2-OWNER-003) — with the dispatcher unavailable (f4-orphan-window-design.md:108–114 accepts "NO hard bound"); mission runner dead during a host turn (host.go:421–450 puts the turn cap in the runner) | **The in-process native turn limit (150)** — per R-43-m0b the surviving in-process coarse dollar bound; a native dollar flag exists here only when the operator sets `METASYSTEM_CLAUDE_MAX_BUDGET_USD` (rejected-alternative record below) | Coarse: order of LOW HUNDREDS of dollars worst case (150 turns at up to order-of-a-dollar-or-more per maximal turn, inventory item 2); an explicit operator override tightens it to threshold plus one indivisible call |
| Provider billing anomaly invisible to list cost | **No owner in this repository** — the native counter measures list cost and never moves (SCR-R1-PROTECTION-002); provider-side account limits are not verifiable from here and this design does not lean on them | Unbounded here; the rejected backstop could not have owned it either |

The time-enforcers-unavailable row is the refutation of assumption 1, and
it was the backstop's whole justification: when every outer time owner is
dead, a native dollar limit would be the last dollar fence on the primary
process. R-43-m0b accepts that residual with the turn limit as the
surviving in-process coarse dollar bound and kills the constant; the
backstop's full record lives in the rejected-alternative section below.
Revision 1 put the refutation on a pricing anomaly instead; that
justification stays dropped — the last row shows a native dollar flag
could not own it.

## Open spend scenario (SCR-R1-ENVELOPE-004) — owned by goal uncapped-delegate-fanout

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
revision 1 wrongly listed it as covered, and it is recorded here as OPEN.
**Disposition (R-42-m0b, Wido, 2026-09-01, `memory/rulings.md`):** the
scenario does NOT block the cap change shipping — "the retired $5 default
never covered spawned sessions either, so the change worsens nothing" —
and the hole is owned by its own backlog goal, uncapped-delegate-fanout,
awaiting his budget tuple. The containment mechanism (the delegate
permission preset refusing `claude` invocation from delegate Bash,
extended census / kill-guard reach over signature-matched agent processes,
or another rule) is that goal's decision; this design records the scenario
and claims no part of it for this change.

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
preferentially executes exactly the workers that finished. A flag that is
never passed removes that failure mode entirely; the write-early-return / cap-death
recollection fix remains goal budget-death-on-return's separate obligation
and is unaffected by this design.

## The specification (Option B, the clean kill — R-43-m0b)

One code file, one test file, one comment. Time — the per-job wall-clock
cap and kill, and the watchdog machinery (watcher, reaper, mission runner)
— and count — the native 150-turn limit — are the law. The turn limit is
explicitly the SURVIVING IN-PROCESS COARSE DOLLAR BOUND (order of low
hundreds of dollars worst case; inventory item 2).

1. **`internal/adapter/claude.go`, `ClaudeBudget` (lines 227–243): no
   default.** `budget` starts empty instead of `"5.00"`. When
   `METASYSTEM_CLAUDE_MAX_BUDGET_USD` is UNSET, `ClaudeBudget` returns the
   empty budget with no error and no validation. When the variable is SET,
   the value is validated exactly as before — the decimal regexp and the
   zero refusal — and a malformed value still returns
   `invalid_native_budget` (an explicitly SET empty string is a set value,
   fails the regexp today, and keeps failing). The turns half of the
   function — default `"150"`, override, `invalid_native_turn_limit` — is
   untouched. The doc comment (lines 220–226) updates to say: no default
   native dollar limit; time (wall-clock caps, watchdog) and count (the
   turn limit) are the law, the turn limit being the surviving in-process
   coarse dollar bound; `METASYSTEM_CLAUDE_MAX_BUDGET_USD` is the explicit
   operator opt-in, validated when set; citing this design and R-43-m0b.
2. **Flag shape: omitted when empty.** `BuildClaudeCommand` (line 300)
   appends the `--max-budget-usd <value>` pair only when `budget` is
   nonempty; `--max-turns` continues to be appended always. The CLI's
   contract treats the flag as optional — omitted means no native spend
   limit. No unlimited sentinel exists to pass instead: the CLI documents
   no unlimited value and our validator refuses 0, so omission is the only
   kill shape.
3. **Tests, exactly, in `internal/adapter/claudecommand_test.go`:**
   - `TestClaudeBudgetPolicy` line 12: the defaults assertion
     `budget != "5.00"` becomes `budget != ""` — an unset environment now
     yields the empty budget, turns `"150"`, and no error. The `$12.5`
     override case, the malformed-`"free"` refusal, the zero refusal, and
     the turns refusal stay byte-identical.
   - The argv byte-order pins stay untouched where they stand: every
     existing pin passes `"5.00"` as an EXPLICIT `budget` argument
     (claudecommand_test.go lines 52, 77, 99, 115; events_test.go line
     221), so each exercises the set-override branch, where the flag still
     appears — the pinned wire byte order is unchanged.
   - ONE NEW argv case pins the flag's absence: `BuildClaudeCommand` with
     empty `budget` yields an argv containing no `--max-budget-usd` token
     and no orphaned value slot, with `--max-turns <turns>` still present
     and every other token in the pinned order.
   - `internal/contract/contract_test.go` line 174 uses
     `host.max-budget-usd=5.00` as a sealed-contract fixture value —
     contract validation, not adapter policy — untouched.
4. **Mission interaction: unchanged code, already Option-B compatible.**
   `internal/missionrunner/host.go` (lines 319–322) exports
   `METASYSTEM_CLAUDE_MAX_BUDGET_USD` only when the sealed contract
   carries a nonempty `host.max-budget-usd`. A contract that seals the key
   gives its host turn a validated operator override exactly as before; a
   contract without it now gives the host turn NO native dollar flag — the
   sealed time fences and the runner-enforced `host.turn-cap-min` remain
   the mission's real exposure bound.
5. **Codex (and Devin):** no equivalent cap exists to judge.
   `scripts/agents/adapters/codex.sh` declares `"nativeBudget": false` (line
   78) and `internal/adapter/codex.go` builds no budget flag; Devin likewise
   (`devin.sh` line 88). Nothing is scoped in and nothing is deferred —
   there is no object.

## Rejected alternative: the $200.00 backstop (R-43-m0b)

Revisions 2–3 kept the flag and raised the default to a never-hit $200.00
backstop, justified by the time-enforcers-unavailable residual. **Why it
lost (R-43-m0b): a forever-recalibrated constant buying a tighter number
on a bound the turn count already provides, in a scenario never
observed.** The material below is the rejected alternative's full record,
preserved because its arithmetic still sizes an EXPLICIT operator
override, and because SCR-R1-LEVEL-001, SCR-R2-LEVEL-001,
SCR-R1-PROTECTION-002, and SCR-R2-PROTECTION-002 were folded into it.
SCR-R3-LEVEL-001, the round-3 finding against this level, is DISSOLVED BY
DECISION — the number it calibrated no longer exists (R-43-m0b).

**The level derivation ($200.00):** the never-hit condition was level
STRICTLY GREATER THAN worst-assumed-legitimate-burn × the true permitted
horizon — strictly, because the installed Claude Code predicate stops at
accumulated cost greater than or EQUAL to the threshold. Worst observed
legitimate burn: $0.974 per elapsed minute — the highest of the seven
measured raw rounds, itself a legitimate completion. Assumed burn, with
headroom for the sample's thinness (seven rounds, one workload class, the
three legitimate rates all from rounds under seven minutes): $1.25/minute,
≈28% above the worst observation. TRUE permitted horizon: 150 minutes —
NOT the 120-minute `dispatch.cap-min` default, but the derived watcher
ceiling that bounds every dispatched process: `internal/supervise/
ceiling.go` `DeriveCeiling` takes the maximum of the 120-minute floor,
`dispatch.cap-min` (shipped 120), `fence.job-cap-min`, and every
`cap.min.*` source, then adds the 30-minute allowance, giving 150 with
the shipped configuration (`ceiling_test.go:28` pins exactly this);
dispatch refuses caps at or above the attested ceiling
(`internal/dispatch/attest.go:136–138`). Product: 1.25 × 150 = $187.50;
the strictly-greater rule with stated headroom gave $200.00. Revision 2's
$150.00 violated the design's own rule by choosing equality on the wrong
horizon (SCR-R2-LEVEL-001); revision 1's $50 was derived from reservation
allowances rather than elapsed time — both stay withdrawn.

**The horizon coupling (sizing rule for an explicit override):** a
dispatch explicitly capped above 120 minutes raises the derived watcher
ceiling to capMin + 30 at the next re-arm, so an operator who chooses to
set `METASYSTEM_CLAUDE_MAX_BUDGET_USD` on such a job must set it STRICTLY
GREATER than $1.25 × (capMin + 30). Likewise for missions:
`host.turn-cap-min` is required (`internal/contract/contract.go:72–93`)
while `host.max-budget-usd` is optional, so a contract sealing a turn cap
above 120 minutes that ALSO chooses to seal `host.max-budget-usd` must
seal it strictly greater than $1.25 × (host.turn-cap-min + 30).

**What the flag honestly is (overshoot sizing, SCR-R1-PROTECTION-002,
SCR-R2-PROTECTION-002):** `--max-budget-usd` is a measured-cost THRESHOLD,
not a hard bound. The CLI checks a running cost counter between calls and
refuses the NEXT call, so a job always completes the indivisible call that
crosses the line. Observed: the four recorded budget deaths terminated at
$5.047, $5.150, $5.218, and $5.343 against a $5.00 threshold — ordinary
behavior, not the maximum. The worst overshoot is the list cost of ONE
maximal indivisible call past the threshold: Claude Code 2.1.252 lists a
one-million-token context window, a 64,000-token default maximum output,
and rates of $10 per million direct input tokens, $50 per million output
tokens, and $20 per million one-hour cache-write tokens, so a maximal call
(≈936,000 input tokens plus 64,000 output tokens) costs ≈$12.56 with
direct input and ≈$21.92 with one-hour cache creation — worst-case
exposure was threshold plus one maximal call, ≈$222 at the $200.00
default. The counter's accounting basis is LIST cost, not authoritative
provider billing (every raw result records `costBasis: "list"`): the flag
could bound only the CLI-measured list-cost spend of the ONE process it
was passed to; it could never own a billing anomaly invisible to list
cost, nor any process it was not passed to (a Bash-spawned child never
inherits the flag). Revision 1's claim that it "caps a 10× pricing anomaly
at $50 per job" stays withdrawn as false on both counts. These semantics
now describe the EXPLICIT override, the only form in which the flag
survives.

## Self-grade

- **The direction is no longer a graded claim:** R-43-m0b is Wido's
  recorded decision, and this revision is a promotion of the design's own
  fully-specified Option-B alternative, not new design. Confidence 0.9
  that the promotion is exact — the specification above is the alternative
  paragraph the ruling chose, expanded to name the precise test changes
  and the mission-host paragraph, with nothing new decided.
- **Weakest claims:** (a) the turn limit's dollar coarseness (order of low
  hundreds worst case) is priced from catalog rates, not observed — it is
  the accepted residual's only remaining in-process dollar bound, by
  decision; (b) the enforcer-liveness scenario has never been observed to
  fire, and the ruling explicitly prices its acceptance on that record;
  (c) the unproven residuals recorded verbatim in the open-scenario
  section remain deliberately unresolved rather than settled by assertion.
- **Reject this design if:** the shipped diff must touch anything beyond
  the ClaudeBudget function, its tests, and the doc comment.
