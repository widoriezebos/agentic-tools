# Design critique, round 3

Reviewed commit: `a1d23839fb2b1b550a72d0c7d6c07f954a68c80d`, plus the uncommitted revision 3 design and diagnosis named in the brief.

All evidence below was read in this worktree. Arithmetic is marked as inferred. No test or process-driving command was run.

## Material findings

### SSTB-301

Severity: critical  
Material: yes

Claim: The chosen headless successor does not run under the supervision hooks. It therefore has no context trigger and cannot form the promised chain of successors.

Evidence:

- Read, design sentence: "The successor is a headless session under the same hooks, so the same trigger applies to it; when it reaches the trigger it hands off the same way." `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:62`.
- Read, launcher fact: the chosen launcher executes `metasystem delegate --revive <nonce>`. `metasystem/cmd/metasystem/steward_verbs.go:493-509`.
- Read, hook fact: a job-bound delegate that authenticates as a local delegate calls `intentional_hook_skip` and exits the supervision hook. The ancestry fallback does the same. `metasystem/scripts/agents/supervision-hook.sh:1451-1475,1581-1586`.
- Read, adapter fact: the generated Claude job settings contain only the `SessionStart` establishment signal. They do not install the supervision Stop hook. `metasystem/internal/adapter/claude.go:122-131`.
- An implementer following U3c can make the first launch work and still leave every continuation outside P1, P3 and the next handoff. The headless hook contract must be designed before this launcher is buildable.

### SSTB-302

Severity: critical  
Material: yes

Claim: P2 and P3 still allow a session to end before a successor is confirmed. Open question 1 also offers an answer that waits for predecessor death.

Evidence:

- Read, design sentence: "No path ends a session with no successor started." The same paragraph says the second over-trigger Stop "allows the Stop with `handoff recorded`", then says the gate keeps blocking while the successor is unconfirmed, and finally admits one infrastructure path that ends with no successor. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:60`.
- Read, design sentence: the idle-backlog policy says the second refusal launches a continuation and marks the session handed off, but it gives no confirmation gate or failed-launch outcome before the third silent Stop. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:66`.
- Read, code fact: the current idle escalation only prepares an intent. It then sets `ShouldBlock` false and says "the turn will end". It does not call the revival or observe the job. `metasystem/internal/goal/turnverdict.go:1067-1079,1110-1136`.
- Read, code fact: an unreadable handoff record produces an infrastructure verdict with `ShouldBlock: false`. `metasystem/internal/goal/turnverdict.go:900-906`.
- Read, open-question fact: option 1(b) says the pane relaunch "costs the predecessor's death before every successor". `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:150`. That reinstates the predecessor-death condition that currently holds the handoff. `metasystem/internal/steward/handoff.go:93-100`.
- These are different Stop outcomes. The program does not hold under option 1(b), and U3a and U2c have no single implementable confirmation rule.

### SSTB-303

Severity: critical  
Material: yes

Claim: The claim does not belong to the machine alone, and the selected launch path drops the goal binding. P2 does not specify how lineage authority reaches the successor.

Evidence:

- Read, design sentence: "The goal claim belongs to the machine, so the continuation on the same machine works under the same claim with no ledger verb." `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:54`.
- Read, authority fact: the goal store says the machine and lineage pair is the ownership key. A second lineage on the same machine is refused. `metasystem/internal/goal/verbs.go:502-506,964-970`.
- Read, state fact: the captured handoff state records the claimant machine and lineage. `metasystem/internal/steward/handoff_capture.go:835-844`. `HandoffBinding` does not carry that claimant, and a `seatHandoff` intent is staged without `SeatActor` or `SeatClaimEpoch`. `metasystem/internal/steward/handoff.go:23-35`; `metasystem/internal/steward/stage.go:172-179`.
- Read, dispatch fact: `steward authorize-dispatch` returns the goal. `metasystem/cmd/metasystem/steward_verbs.go:422-425`. The steward branch parses role, brief, job, runtime, model and permissions, but never parses `goal`. `metasystem/scripts/agents/dispatch.sh:1471-1482`. The later job claim and record receive that empty goal and cleared main and claim-epoch fields. `metasystem/scripts/agents/dispatch.sh:1552-1557,1842-1852,1874-1886,1931-1950`.
- U3c must settle whether authority is inherited, transferred, or explicitly projected. Its intent and design questions currently name none of those owners.

### SSTB-304

Severity: critical  
Material: yes

Claim: The stated 198K and 162K context bounds are calculated from averages, not maxima. Neither answer to open question 4 proves DONE by construction.

Evidence:

- Read, design sentence: "Maximum overshoot at 12 calls: 48K, so a Stop sample of at most 198K." The recommended tool hook similarly claims a 162K bound from three calls at 4K growth. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:42`.
- Read, data fact: the diagnosis reports average calls per turn. It gives 6.1 notification calls, 4.3 Stop-refusal calls, 41.3 compaction calls and 11.3 usage-limit calls. It does not report a maximum turn or a maximum context increase per call. `metasystem/records/misc/token-diagnosis-2026-09-15.md:165-174`.
- Read, data fact: the 4K number is inferred from one session's total growth over 217 calls. The same diagnosis observed a 966K peak. `metasystem/records/misc/token-diagnosis-2026-09-15.md:24`; `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:42`.
- Read, sampling fact: the existing read is explicitly nonblocking and can return an unknown reading. `metasystem/internal/steward/context.go:88-99,121-126`. The design says an unknown sample never blocks. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:40`.
- Read, requirement fact: DONE requires a stated size by construction. `metasystem/plans/goals/seats-spend-tokens-in-bounded-sessions.md:8`. Option 4(a) changes that to a transcript rule and admits that a 42-call turn breaks it. Option 4(b) still uses average growth as a hard bound. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:153`.
- The arithmetic `12 * 4K = 48K` and `42 * 4K = 168K` is correct. The inputs do not establish maxima. Open question 2 is otherwise sound under either configured cap, but it does not repair this bound.

### SSTB-305

Severity: high  
Material: yes

Claim: The promised zero-turn predecessor Stop cannot be produced by the named gate change. The current hook and adapter both require visible output, and U3c drops those owners.

Evidence:

- Read, design sentence: "every later Stop of that session is allowed with no display and no `systemMessage`". `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:56`.
- Read, hook fact: an empty `display` makes the turn verdict unreadable and degraded. `metasystem/scripts/agents/supervision-hook.sh:2368-2375`.
- Read, adapter fact: every allowed Stop output writes `systemMessage` with the human line. `metasystem/internal/adapter/stopoutput.go:91-97`.
- Read, routing fact: U3c names the handed-off allowance and session marker, but its design questions do not name the hook readability contract, Stop presentation, or adapter output. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:136`.
- An implementer who changes only the verdict will either get a degraded hook or still buy a system-message turn. This reopens the missing-owner part of SSTB-116.

### SSTB-306

Severity: high  
Material: yes

Claim: The measurement window violates binding R11. The 871.7M baseline is a 12:51 CEST result, not the required 12:45 CEST result.

Evidence:

- Read, design sentence: "The baseline is 2026-09-15 00:00 to 12:51 Europe/Amsterdam" and "revision 2's 12:45 was wrong". `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:70`.
- Read, binding fact: R11 requires the verb to own "the 12:45 CEST end". `metasystem/artifacts/reports/tokens-design-brief-r3.md:41`. The goal record also describes the source scope as ending at 12:45. `metasystem/plans/goals/seats-spend-tokens-in-bounded-sessions.md:10`.
- Read, data fact: the diagnosis was generated at 10:51Z, which is 12:51 CEST, and its window ends at generation time. `metasystem/records/misc/token-diagnosis-2026-09-15.md:1-3`. The script sets `NOW` to the current time. `metasystem/records/misc/token-diagnosis-2026-09-15.py:5-9`.
- The Claude-only rule is otherwise coherent. The 374.4M any-runtime arithmetic and the 316.3M Claude-only m1e fence arithmetic are also correct from the cited rows. The endpoint still changes the baseline, forecast window and proof assertion.

### SSTB-307

Severity: high  
Material: yes

Claim: The fleet forecast arithmetic closes, but the per-seat 150M forecast and the ten-refusal forecast do not follow from the diagnosis. Open question 3 therefore offers a proof day that does not meet P5.

Evidence:

- Read, design sentence: per-seat totals allocate forecast main cost by each seat's share of all baseline main calls. It reports m1e at 174M without U7 and 149M with U7. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:96`.
- Read, data fact: the causes are not distributed by total-call share. Peer calls are 119 for m1b, 2 for m1c and 60 for m1e. Human calls are 10, 4 and 40. Stop-hook calls are 123, 124 and 192. `metasystem/records/misc/token-diagnosis-2026-09-15.md:42-75`. Allocating the U7 peer cut by total-call share erases the very per-seat distribution that U7 changes.
- Read, design sentence: the ten-refusal row says "two per idle digest" and "at most 10 at 4 idle conditions". It gives no m1b, m1c and m1e forecast. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:107`. Inferred arithmetic gives eight, not ten, before any stated margin. The diagnosis's actual per-seat refusal-turn counts were 41, 7 and 55. `metasystem/records/misc/token-diagnosis-2026-09-15.md:121-145`.
- Read, denominator fact: the 20M line assumes eight m1e landings. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:106`. The diagnosis script scans Claude project transcripts and does not read landing receipts or Git history. `metasystem/records/misc/token-diagnosis-2026-09-15.py:96-125`.
- Read, open-question fact: option 3(a) explicitly says the two per-seat lines are not asserted on the first proof day. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:152`. That answer does not satisfy the P5 table or the goal's one-day proof.
- Inferred check: the aggregate 454 and 308 calls, 59.0M and 40.0M work costs, 26.3M and 17.9M handoff costs, and 48.1 and 38.6 percent main shares are arithmetically consistent with the stated rounded assumptions. SSTB-203 and SSTB-112 reopen only for the unsupported per-seat and denominator inputs.

### SSTB-308

Severity: high  
Material: yes

Claim: U3c is ordered before the goal that makes fresh wait registration valid for the successor session.

Evidence:

- Read, design sentence: U3c makes `context resume` freshly register every inherited `openWork` row. U1 is ordered after U3c. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:136-137`.
- Read, existing-goal fact: shell registration currently writes a pid-derived session that does not match the runtime Stop session. `metasystem/plans/goals/registered-wait-matches-the-runtime-session.md:8`.
- Read, approved-design fact: a successor must refuse adoption of a wrong-session row and register fresh under its own authenticated session. That fresh registration depends on the new association and supersession rules. `metasystem/plans/registered-wait-matches-the-runtime-session-design.md:157-169,195-200`.
- Building U3c first leaves its first-act wait restoration on the known broken registration path. U1 must precede the registration part of U3c, or U3c must own an equivalent authenticated-session dependency. This reopens SSTB-202 and SSTB-108.

### SSTB-310

Severity: high  
Material: yes

Claim: The exact predecessor exclusions cannot be implemented in `handoff.go` from the data it receives. The owning goal's design questions do not route the required census and active-intent shape changes.

Evidence:

- Read, design sentence: the census hold "counts seat mains other than the predecessor's pid", and the `others` hold excludes the consumed intent whose job id equals `binding.PredecessorJob`. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:48`.
- Read, code fact: `Workers` contains only integer counts. It has no process identities. `metasystem/internal/steward/verdict.go:47-64`. `workersFromVerdict` discards each inventory identity after incrementing those counts. `metasystem/internal/steward/census.go:52-81`.
- Read, code fact: revival reduces active consumed intents to the integer `others` before calling `decideForHandoff`. The callee cannot inspect their job ids. `metasystem/internal/steward/revive.go:276-299`; `metasystem/internal/steward/handoff.go:71,116-124`.
- Read, routing fact: U3c's design questions omit the census summary, the active-intent input, and the exact exclusion predicate. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:136`.
- A designer must choose whether to preserve identities, pre-filter the census, or pass explicit exclusion facts. Those choices change interfaces and tests. SSTB-116 was not fully moved.

### SSTB-309

Severity: medium  
Material: yes

Claim: Today's delegate rule tells seats to resume design corrections, while the owning goal requires every design revision to be fresh.

Evidence:

- Read, design sentence: S3 says "every design and revision is a fresh delegate", but later says "corrections use `delegate --follow-up`". Its transcript check permits resumed agents only in critique chains. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:121`.
- Read, governing-goal fact: the design-delegate goal says a design or revision runs in a fresh delegate and is never resumed. `metasystem/plans/goals/design-delegates-run-fresh-and-bounded.md:8`.
- The critique-chain exception is correctly retained, and S6 correctly puts `context resume` first. The correction sentence still makes a seat choose between two opposite dispatch modes. This reopens SSTB-110.

### SSTB-311

Severity: medium  
Material: yes

Claim: SSTB-117 is recorded as moved to every owning goal, but only U1 carries the focused-witness design question.

Evidence:

- Read, design sentence: the critique record says SSTB-117 moved "to U1 and to every owning goal: focused Go witnesses, bed legs as DONE proof only". `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:182`.
- Read, routing fact: U1 asks for focused witnesses. The design-question cells for U3a, U3e, U3b, U3c, U2a, U2c, U2b, U4, U5a, U5b, U5c and U7 do not. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:133-145`.
- Read, test fact: the present direct installed-binary wait test returns without exercising that route when `METASYSTEM_WAIT_BINARY` is absent, then relies on the installed fixture path when it is present. `metasystem/cmd/metasystem/wait_verb_test.go:668-683`. This is the proof-shape gap SSTB-117 required every owner to avoid.
- The move drops the requirement for all owners except U1. Their designers can build different proof surfaces because this program page does not ask the question.

## Non-material findings

### SSTB-312

Severity: low  
Material: no

Claim: Four goal coordinates are stale, but the slugs identify the owners unambiguously.

Evidence:

- Read, design sentence: P7 labels the owners as 1:10, 1:24, 1:23 and 1:5. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:133-144`.
- Read, ledger fact: the current sequences are 8 for `coordinator-context-stays-under-budget`, 22 for `registered-wait-matches-the-runtime-session`, 21 for `stop-gate-sees-harness-tracked-work`, and 3 for `spend-fence-reports-tokens-per-model-and-cause`. `metasystem/plans/goals/coordinator-context-stays-under-budget.md:5`; `metasystem/plans/goals/registered-wait-matches-the-runtime-session.md:5`; `metasystem/plans/goals/stop-gate-sees-harness-tracked-work.md:5`; `metasystem/plans/goals/spend-fence-reports-tokens-per-model-and-cause.md:5`.
- No implementer would choose a different goal because every row also gives the exact slug.

## Closure audit

| Prior finding | Revision 3 text | Result |
|---|---|---|
| SSTB-201 | P1 and the record claim numeric closure. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:42,161` | Reopened as SSTB-304. |
| SSTB-202 | P2 now carries every `WaitSelector` field and routes fresh registration to U3c. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:58,162` | Selector closure is correct. Registration order reopens as SSTB-308. |
| SSTB-203 | P5 uses ten growth calls and one 0.58M handoff cost. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:80-96,163` | Fleet arithmetic closes. Per-seat proof reopens as SSTB-307. |
| SSTB-204 | P4 is Claude-only and excludes non-Claude engine jobs by runtime. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:70,164` | Closed. |
| SSTB-205 | P4 gives the verb both endpoints but changes the binding endpoint. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:70-72,165` | Reopened as SSTB-306. |
| SSTB-206 | P4 defines one landing commit, deduplicates rows, and excludes revivals. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:74,166` | Closed at program level. Field detail is correctly moved to U5b. |
| SSTB-207 | The launch declaration is semantic, and its grammar and map go to U5a. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:76,142,167` | Correctly moved to `spend-fence-reports-tokens-per-model-and-cause`. |
| SSTB-208 | P7 states the shared router rule. U4 and U7 carry their router questions, and U7 carries storage. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:128,141,145,168` | Correctly moved. |
| SSTB-209 | U1 owns the member allocation and direct installed-binary witness. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:137,169` | Correctly moved to `registered-wait-matches-the-runtime-session`. |
| SSTB-210 | U4 owns rerun and critic re-entry through one chain. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:141,170` | Correctly moved to the proposed unique id `units-run-through-one-launcher`. |
| SSTB-211 | The three anchors are corrected in P3, P7 and the evidence table. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:22,66,128,171` | Closed. |
| SSTB-102 | P1 gives a trigger and two claimed bounds. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:40-42,172` | Reopened as SSTB-304. |
| SSTB-103 | P2 claims the gate launches before every end. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:60,173` | Reopened as SSTB-302. |
| SSTB-105 | The four choices are explicit. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:148-153,174` | Reopened by option 1(b) in SSTB-302, option 3(a) in SSTB-307, and both option 4 answers in SSTB-304. Option 2 holds under both answers. |
| SSTB-108 | P2 uses the complete selector and fresh registration. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:58,175` | Reopened only for dependency order as SSTB-308. |
| SSTB-110 | P6 retains critique follow-ups and `context resume` first. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:119-124,176` | Reopened as SSTB-309. The no-stop and no-wait checks are otherwise transcript-checkable. |
| SSTB-111 | P4 has one runtime scope, zone, roots and verb-owned endpoints. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:70-72,177` | Reopened only for the binding end time as SSTB-306. |
| SSTB-112 | The double count is removed and P5 recomputes fleet totals. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:84-96,178` | Aggregate closure is correct. Per-seat inputs reopen as SSTB-307. |
| SSTB-114 | P4 counts only deduplicated landing commits. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:74,179` | Closed. |
| SSTB-115 | P4 and P5 settle program scope. U5c owns the comparison artifact and tolerance. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:70-76,109,144,180` | Correctly moved to `spend-fence-reports-tokens-per-model-and-cause`. |
| SSTB-116 | The record says U3c and U5a name the real owners. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:181` | U5a's cache move closes. U3c's authority, output and census owners reopen as SSTB-303, SSTB-305 and SSTB-310. |
| SSTB-117 | The record says every owning goal gets focused witnesses. `metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:182` | Reopened as SSTB-311. |

The existing goal slugs in P7 all exist. The proposed `units-run-through-one-launcher` id does not duplicate a goal under `metasystem/plans/goals/`. Apart from SSTB-308, the program order is consistent with the stated dependencies.

## Rigor

| findingId | rigorClass | facts | reopeningTrigger |
|---|---|---|---|
| SSTB-301 | severe | `local=true, recoverable=true, proofBoundaryCrossed=true, authorityBoundaryCrossed=false, secretsBoundaryCrossed=false, irreversibleDataBoundaryCrossed=false, externalSideEffectBoundaryCrossed=false` | A named U3c contract and focused witness prove that a steward continuation receives the context trigger and can start its own successor. |
| SSTB-302 | severe | `local=true, recoverable=true, proofBoundaryCrossed=true, authorityBoundaryCrossed=false, secretsBoundaryCrossed=false, irreversibleDataBoundaryCrossed=false, externalSideEffectBoundaryCrossed=false` | Every context and idle Stop path blocks until the named running-successor signal exists, including capture and launch failures, and every launcher option preserves that invariant. |
| SSTB-303 | severe | `local=true, recoverable=true, proofBoundaryCrossed=false, authorityBoundaryCrossed=true, secretsBoundaryCrossed=false, irreversibleDataBoundaryCrossed=false, externalSideEffectBoundaryCrossed=false` | U3c states and routes one lineage and claim-epoch transfer or projection rule, and the authorized dispatch job is bound to that goal and authority. |
| SSTB-304 | severe | `local=true, recoverable=true, proofBoundaryCrossed=true, authorityBoundaryCrossed=false, secretsBoundaryCrossed=false, irreversibleDataBoundaryCrossed=false, externalSideEffectBoundaryCrossed=false` | P1 uses an observed or enforced maximum for in-turn growth and unknown samples, so every permitted answer satisfies the stated construction bound. |
| SSTB-305 | severe | `local=true, recoverable=true, proofBoundaryCrossed=true, authorityBoundaryCrossed=false, secretsBoundaryCrossed=false, irreversibleDataBoundaryCrossed=false, externalSideEffectBoundaryCrossed=true` | U3c owns a Stop output contract that the hook and adapter accept with no model-visible message, with a focused zero-turn witness. |
| SSTB-306 | severe | `local=true, recoverable=true, proofBoundaryCrossed=true, authorityBoundaryCrossed=false, secretsBoundaryCrossed=false, irreversibleDataBoundaryCrossed=false, externalSideEffectBoundaryCrossed=false` | The baseline and forecast are recomputed over the binding 12:45 CEST endpoint, or Wido changes that binding. |
| SSTB-307 | severe | `local=true, recoverable=true, proofBoundaryCrossed=true, authorityBoundaryCrossed=false, secretsBoundaryCrossed=false, irreversibleDataBoundaryCrossed=false, externalSideEffectBoundaryCrossed=false` | P5 derives each seat's token and refusal forecasts from per-seat cause counts, states the landing denominator source, and every option 3 answer meets the proof-day thresholds. |
| SSTB-308 | unproven | `local=true, recoverable=true, proofBoundaryCrossed=false, authorityBoundaryCrossed=false, secretsBoundaryCrossed=false, irreversibleDataBoundaryCrossed=false, externalSideEffectBoundaryCrossed=false` | The unit order makes authenticated successor-session registration available before U3c restores waits, with the dependency named in both owning designs. |
| SSTB-310 | unproven | `local=true, recoverable=true, proofBoundaryCrossed=false, authorityBoundaryCrossed=false, secretsBoundaryCrossed=false, irreversibleDataBoundaryCrossed=false, externalSideEffectBoundaryCrossed=false` | U3c routes the census and active-intent data shape needed to exclude the exact predecessor process and predecessor job. |
| SSTB-309 | unproven | `local=true, recoverable=true, proofBoundaryCrossed=false, authorityBoundaryCrossed=false, secretsBoundaryCrossed=false, irreversibleDataBoundaryCrossed=false, externalSideEffectBoundaryCrossed=false` | S3 distinguishes fresh design revisions from follow-up critique rounds and gives each one transcript-checkable dispatch evidence. |
| SSTB-311 | severe | `local=true, recoverable=true, proofBoundaryCrossed=true, authorityBoundaryCrossed=false, secretsBoundaryCrossed=false, irreversibleDataBoundaryCrossed=false, externalSideEffectBoundaryCrossed=false` | Every owning goal's design questions carry SSTB-117's focused, process-free witness requirement. |

Verdict: rework.
