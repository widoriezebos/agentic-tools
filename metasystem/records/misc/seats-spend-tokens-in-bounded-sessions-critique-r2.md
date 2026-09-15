# Design critique, round 2

Reviewed commit: `a1d23839fb2b1b550a72d0c7d6c07f954a68c80d`, plus the uncommitted revision 2 design and diagnosis named in the brief.

Round-one closure result: SSTB-102, SSTB-103, SSTB-105, SSTB-108, SSTB-110, SSTB-111, SSTB-112, SSTB-114, SSTB-115, SSTB-116 and SSTB-117 reopen below. SSTB-101 is closed by the unchanged member list at `plans/seats-spend-tokens-in-bounded-sessions-design.md:144`. SSTB-104 is closed by the separate policies and outcomes at `plans/seats-spend-tokens-in-bounded-sessions-design.md:73-75`. SSTB-106 is closed at `plans/seats-spend-tokens-in-bounded-sessions-design.md:145-147`. SSTB-107 is closed by the three liveness outcomes at `plans/seats-spend-tokens-in-bounded-sessions-design.md:66-73`. SSTB-109 is closed by the delegate-path launcher and judgement pause at `plans/seats-spend-tokens-in-bounded-sessions-design.md:79`. SSTB-113 is closed at `plans/seats-spend-tokens-in-bounded-sessions-design.md:83`. SSTB-118 and SSTB-119 are closed at `plans/seats-spend-tokens-in-bounded-sessions-design.md:242-243`.

## Material findings

### SSTB-201

Severity: critical  
Material: yes

Claim: The cap is still observed after an unbounded turn. Revision 2 therefore does not keep every call under the cap by construction.

Evidence:

- Design sentence: "A turn that runs past the cap before its Stop is outside the trigger's reach; the proof's maximum rule measures it, and S2 and S5 bound turn length." `plans/seats-spend-tokens-in-bounded-sessions-design.md:40`. The first enforcement point is the following Stop. `plans/seats-spend-tokens-in-bounded-sessions-design.md:52-54`.
- Requirement fact: DONE requires the main session to stay under the stated size by construction. `plans/goals/seats-spend-tokens-in-bounded-sessions.md:8`.
- Data fact: the diagnosis reports means, not maxima, of 6.1 notification calls and 4.3 refusal calls. It also contains turns averaging 41.3 calls after compaction and 11.3 after a usage-limit continuation. `records/misc/token-diagnosis-2026-09-15.md:165-174`.
- Control fact: S2 limits background tasks and S5 limits one tool result. Neither limits foreground calls or total context growth before the next Stop. `plans/seats-spend-tokens-in-bounded-sessions-design.md:131,134`. The existing proof rejects any maximum over the ceiling, but that detects the failure after it happened. `internal/steward/contextreport.go:560-569`.

### SSTB-202

Severity: critical  
Material: yes

Claim: The handoff record does not preserve the complete wait predicate, and neither successor path owns fresh registration. A handoff can lose a landing wait or a channel-backed human wait.

Evidence:

- Design sentence: `openWork` captures "the kind, the target (job id, run id, attempt id, goal event and question), the deadline that was left, and the fresh registration command." The examples cover a job, a local process and one `human-act` wait. `plans/seats-spend-tokens-in-bounded-sessions-design.md:56`. The headless answer says only that the continuation reads `openWork` and registers fresh. `plans/seats-spend-tokens-in-bounded-sessions-design.md:60,165`.
- Code fact: `WaitSelector` is the complete durable predicate. It also contains `After`, `Verb`, `Chain` and `Poll`. `internal/run/waiter.go:55-68`. The command parser uses those fields for goal landing, human action and channel waits. `cmd/metasystem/wait_verb.go:75-85,110-123`. A reduced target cannot reconstruct every legal row required by the registered-wait design.
- Owner fact: the current staged continuation brief says to read and verify the handoff, but never says to execute fresh wait registrations. `internal/steward/stage.go:153-164`. The continuation role also has no such rule. `scripts/agents/roles/steward-continuation.md:10-24`. U3b does not own either file. U3c1 also omits `stage.go`, although its critique record says that it names that owner. `plans/seats-spend-tokens-in-bounded-sessions-design.md:151-153,240`.

### SSTB-203

Severity: high  
Material: yes

Claim: The expected-day table measures sessions to the ceiling, not to the trigger. Its handoff counts, cap comparison and main-share thresholds are not the R6 recomputation.

Evidence:

- Design sentence: the model sets floor `F = 110K`, growth `4K` per call, and a trigger `50K` below cap `C`. It then reports 22, 35 and 47 calls per session for caps 200K, 250K and 300K. `plans/seats-spend-tokens-in-bounded-sessions-design.md:40-48`. The expected day assumes one handoff per 22 calls. `plans/seats-spend-tokens-in-bounded-sessions-design.md:93-106`.
- Arithmetic fact: the stated model gives `(C - 50K - 110K) / 4K`, which is 10, 22.5 and 35 growth calls. The displayed 22, 35 and 47 use the ceiling instead. The diagnosis also has 669 m1e main calls, 40 Fable plus 629 Opus, not 653. `records/misc/token-diagnosis-2026-09-15.md:220-231`. The 653 figure is one largest session. `records/misc/token-diagnosis-2026-09-15.md:24`.
- Recalculation: before handoffs, the no-U7 main work is about 454.2 calls and the U7 form is 308 calls, using the design's diagnosis-derived turn counts. At ten growth calls per session that is about 45.4 and 30.8 handoffs, not 18. Keeping the table's 0.51M per handoff gives about 185.7M and 155.6M for the half day. The corresponding main shares are about 50.4 percent and 40.8 percent. Both exceed the stated 50 and 40 percent thresholds. Applying 1.86 gives about 345M and 289M for the full day, still under the 400M fleet threshold. `plans/seats-spend-tokens-in-bounded-sessions-design.md:95-117`.
- Cost fact: the prose defines a handoff as two 150K trigger calls plus one 110K boot call, which is 0.41M, while the table charges 0.51M. `plans/seats-spend-tokens-in-bounded-sessions-design.md:42,46,102`. Using 0.41M changes the forecasts to about 181.1M and 152.5M and moves both main shares back below their lines. The design must settle this input before the threshold assertions can be built. The per-seat 150M and ten-refusal thresholds also have no per-seat forecast. `plans/seats-spend-tokens-in-bounded-sessions-design.md:115,120`.

### SSTB-204

Severity: high  
Material: yes

Claim: The measurement has no single runtime scope, and its m1e baseline omits a source that the proposed fence continues to count.

Evidence:

- Design sentence: the three-seat window sums per-call usage and counts engine jobs once by job id. The fence then widens to subagent files. `plans/seats-spend-tokens-in-bounded-sessions-design.md:89-91`. The m1e baseline is stated as 316.3M, or 264.4M plus 51.9M. `plans/seats-spend-tokens-in-bounded-sessions-design.md:116`.
- Data fact: the 871.7M diagnosis explicitly excludes Codex. `records/misc/token-diagnosis-2026-09-15.md:24-28`. The current m1e fence is 322.5M because it includes 264.4M of main transcripts plus 58.1M of engine-job records. `records/misc/token-diagnosis-2026-09-15.md:28`. Widening that same fence to the reported 51.9M of m1e subagents gives 374.4M, not 316.3M.
- Code fact: the current spend measurement reads every job record, retains its runtime, and appends those tokens before the seat transcripts. It does not select Claude-only jobs. `internal/spend/measure.go:138-223,238-247`. An implementer must choose between filtering the fleet metric to Claude, contrary to this existing fence, or counting Codex job usage against a Claude-only baseline. The design does not make that choice.
- Window fact: an engine job is currently one aggregate measurement assigned from its `startedAt`. `internal/spend/measure.go:192-223`. U5c promises an arbitrary half-open per-call window, but its boundary changes only `window.go` and the verb. It does not add per-call engine timestamps or a boundary attribution rule. `plans/seats-spend-tokens-in-bounded-sessions-design.md:91,159,215`.

### SSTB-205

Severity: high  
Material: yes

Claim: The named diagnosis script cannot cross-check the proof day or even reproduce the baseline's stated 12:45 endpoint.

Evidence:

- Design sentence: the baseline is exactly 00:00 to 12:45 CEST, and the proof reruns the diagnosis script under the same window rules. `plans/seats-spend-tokens-in-bounded-sessions-design.md:89,110,161,247`.
- Data fact: the source report says its 871.7M window ends at generation time, 10:51Z or 12:51 CEST, not 12:45. `records/misc/token-diagnosis-2026-09-15.md:1-12`.
- Script fact: the script hardcodes the 2026-09-15 lower bound and takes the current clock as its end. `records/misc/token-diagnosis-2026-09-15.py:6-9,181`. Its day-so-far window has no upper bound. `records/misc/token-diagnosis-2026-09-15.py:386-394`. It also writes the fixed 2026-09-15 report path. `records/misc/token-diagnosis-2026-09-15.py:29,443-445`. The proof boundary lists only a new record, so no unit owns the script changes needed for a selectable proof day, exact endpoints and non-destructive output. `plans/seats-spend-tokens-in-bounded-sessions-design.md:161`.

### SSTB-206

Severity: high  
Material: yes

Claim: `outcome=shipped` receipt rows are not a one-to-one landing identity. Adding only `seat` cannot measure tokens per landed unit.

Evidence:

- Design sentence: "each receipt is one landing, because a receipt is appended in the same commit as the work." `plans/seats-spend-tokens-in-bounded-sessions-design.md:89`. R39 makes one qualifying receipt the count key. `plans/seats-spend-tokens-in-bounded-sessions-design.md:213`.
- Contract fact: the project rule requires a receipt to be appended in the same commit as its work. It does not say a commit or landing has exactly one receipt. `../development/project-rules-local.md:11`.
- Ledger fact: `outcome=shipped` includes steward revival receipts that are not code landings. `memory/receipts.log:342-343`. Four implementation receipts were appended by the same commit, `9328fb7ec3`, and have the same timestamp. `memory/receipts.log:359-362`, checked with `git blame`.
- Schema fact: the receipt has no landing commit or unit identity field. `internal/receipt/receipt.go:34-45,210-225`. A `seat` field identifies attribution, not uniqueness. The proposed historical fallback through a goal claimant is also not the same rule as the future explicit field. The implementation needs a durable unit or landing identity and a deduplication rule.

### SSTB-207

Severity: high  
Material: yes

Claim: Native delegate subtype is still runtime plumbing, not the required semantic split of design, build read and critique.

Evidence:

- Design sentence: native delegate subtype comes from `meta.json` type and is only `code-critique`, `general-purpose`, or other. Engine jobs use semantic roles. `plans/seats-spend-tokens-in-bounded-sessions-design.md:89,210`.
- Requirement fact: the owned spend goal requires delegate kinds `design`, `build read`, and `critique`. `plans/goals/spend-fence-reports-tokens-per-model-and-cause.md:8`.
- Data fact: both named design delegates are `general-purpose`, while critique delegates are `code-critique`. `records/misc/token-diagnosis-2026-09-15.md:290-305`. The proposed rule cannot distinguish a native design from a native build or read. An implementer needs semantic launch metadata or a grounded classifier, which changes the cache schema and R36.

### SSTB-208

Severity: high  
Material: yes

Claim: Five unit boundaries omit the command router required to expose their verbs. The U7 answer also leaves the inbox's durable owner undefined.

Evidence:

- Design sentence: U3c2 and U3d add context verbs; U4, U5c and U7 add the new `unit`, `spend` and `message` command families. Their boundaries omit `cmd/metasystem/main.go`. `plans/seats-spend-tokens-in-bounded-sessions-design.md:153-160`.
- Code fact: `main.go` currently lists the five existing context verbs only. `cmd/metasystem/main.go:430-439`. Only `wait` and `delegate` have special top-level routes, and every other command must be present in `families()`. `cmd/metasystem/main.go:718-744`. Implementing only the stated boundaries leaves the new commands unreachable.
- Shape fact: U7 promises append, unread count and mark-read behavior but names only a new CLI file, `turnverdict.go` and tests. It states no inbox path, schema, receiver identity, cursor, lock or crash rule. `plans/seats-spend-tokens-in-bounded-sessions-design.md:83,160,219`. The answer to open question 3 therefore requires guessed storage and concurrency behavior. Boundaries and changed-line ceilings must be redrawn.

### SSTB-209

Severity: high  
Material: yes

Claim: U1 has neither the required changed-line allocations nor a focused Go witness runnable without its fixture bed.

Evidence:

- Design sentence: every unit is at most 300 changed lines, but U1's ceiling is "that page's" and its witnesses are simply the registered-wait page's members A, B1, B2, B3 and C. `plans/seats-spend-tokens-in-bounded-sessions-design.md:139,144,171`.
- Design-source fact: the registered-wait page gives member A seven production files, member B2 five production files, and separate member boundaries, but no changed-line ceiling for any member. `plans/registered-wait-matches-the-runtime-session-design.md:25-30,75-80,101-105,140-145`. Member C explicitly owns an installed-binary fixture-bed leg. `plans/registered-wait-matches-the-runtime-session-design.md:173-189`.
- Code fact: the current Go witness returns before its installed path when `METASYSTEM_WAIT_BINARY` is absent. `cmd/metasystem/wait_verb_test.go:668-683`. The fixture bed supplies that variable and runs the test. `scripts/agents/supervision-fixtures.sh:695-698`. The builder cannot run the load-bearing path using only `go test -run` as revision 2 promises. U1 needs per-member allocations and a direct focused witness without redrawing the binding A, B1, B2, B3 and C contracts.

### SSTB-210

Severity: medium  
Material: yes

Claim: The launcher has no correction-round path that can obey the one-chain critique rule.

Evidence:

- Design sentence: U4 always dispatches the reader as a fresh delegate, and it "never lands, retries or follows up." `plans/seats-spend-tokens-in-bounded-sessions-design.md:79`. R31 again requires a fresh reader and never a follow-up. `plans/seats-spend-tokens-in-bounded-sessions-design.md:205`. S3 separately requires critique rounds to be follow-ups on one critic chain. `plans/seats-spend-tokens-in-bounded-sessions-design.md:132`.
- Code fact: the delegate verb has distinct fresh and follow-up dispatch modes. `cmd/metasystem/delegate.go:62-67`. A follow-up requires the existing job id and a new brief. `cmd/metasystem/delegate.go:252-290`.
- Control-flow fact: after a red review and a correction, rerunning U4 starts a fresh critic and violates S3. Continuing by hand preserves S3 but abandons the one-launcher pipeline and has no plan-state contract for re-entry. This changes U4's plan schema and control flow, so SSTB-110 is not closed.

## Non-material findings

### SSTB-211

Severity: low  
Material: no

Claim: Three new source facts are inaccurate, but their nearby owners remain findable.

Evidence:

- Design sentence: U2c places the idle bound at `turnverdict.go:996-1010`. `plans/seats-spend-tokens-in-bounded-sessions-design.md:148`. The threshold is at `internal/goal/turnverdict.go:991`; the cited range starts inside its outcome branch.
- Design sentence: the launcher rationale cites `development/project-rules-local.md:10` as a path relative to the metasystem module. `plans/seats-spend-tokens-in-bounded-sessions-design.md:79`. The file is one level above the module at `../development/project-rules-local.md:10`.
- Design sentence: the rejected background-task path says 61 background completions cost 204.9M. `plans/seats-spend-tokens-in-bounded-sessions-design.md:79`. The diagnosis's complete background Bash total is 211.6M. `records/misc/token-diagnosis-2026-09-15.md:14-18`. This does not change U4's owner or control flow.

## Rigor

| findingId | rigorClass | facts | reopeningTrigger |
|---|---|---|---|
| SSTB-201 | severe | `local=true, recoverable=true, proofBoundaryCrossed=true, authorityBoundaryCrossed=false, secretsBoundaryCrossed=false, irreversibleDataBoundaryCrossed=false, externalSideEffectBoundaryCrossed=false` | A focused proof establishes a hard maximum on every pre-Stop turn and all handoff calls. |
| SSTB-202 | unproven | `local=true, recoverable=true, proofBoundaryCrossed=false, authorityBoundaryCrossed=false, secretsBoundaryCrossed=false, irreversibleDataBoundaryCrossed=false, externalSideEffectBoundaryCrossed=false` | Every legal selector round-trips through `openWork`, and each successor path proves fresh registration. |
| SSTB-203 | severe | `local=true, recoverable=true, proofBoundaryCrossed=true, authorityBoundaryCrossed=false, secretsBoundaryCrossed=false, irreversibleDataBoundaryCrossed=false, externalSideEffectBoundaryCrossed=false` | The trigger-based forecast has one settled handoff cost and every threshold is recomputed from it. |
| SSTB-204 | severe | `local=true, recoverable=true, proofBoundaryCrossed=true, authorityBoundaryCrossed=false, secretsBoundaryCrossed=false, irreversibleDataBoundaryCrossed=false, externalSideEffectBoundaryCrossed=false` | One runtime scope and one engine-window attribution rule reproduce a same-scope baseline. |
| SSTB-205 | severe | `local=true, recoverable=true, proofBoundaryCrossed=true, authorityBoundaryCrossed=false, secretsBoundaryCrossed=false, irreversibleDataBoundaryCrossed=false, externalSideEffectBoundaryCrossed=false` | The script accepts exact endpoints and proof day and writes only the requested proof artifact. |
| SSTB-206 | severe | `local=true, recoverable=true, proofBoundaryCrossed=true, authorityBoundaryCrossed=false, secretsBoundaryCrossed=false, irreversibleDataBoundaryCrossed=false, externalSideEffectBoundaryCrossed=false` | A durable landing or unit identity proves one denominator count per landed unit. |
| SSTB-207 | severe | `local=true, recoverable=true, proofBoundaryCrossed=true, authorityBoundaryCrossed=false, secretsBoundaryCrossed=false, irreversibleDataBoundaryCrossed=false, externalSideEffectBoundaryCrossed=false` | Native delegates are classified into the semantic kinds required by DONE. |
| SSTB-208 | unproven | `local=true, recoverable=true, proofBoundaryCrossed=false, authorityBoundaryCrossed=false, secretsBoundaryCrossed=false, irreversibleDataBoundaryCrossed=false, externalSideEffectBoundaryCrossed=false` | Every new verb has a router owner, and U7 has a complete durable storage contract. |
| SSTB-209 | unproven | `local=true, recoverable=true, proofBoundaryCrossed=false, authorityBoundaryCrossed=false, secretsBoundaryCrossed=false, irreversibleDataBoundaryCrossed=false, externalSideEffectBoundaryCrossed=false` | Every U1 member has an allocation at or under 300 and a direct focused Go witness. |
| SSTB-210 | unproven | `local=true, recoverable=true, proofBoundaryCrossed=false, authorityBoundaryCrossed=false, secretsBoundaryCrossed=false, irreversibleDataBoundaryCrossed=false, externalSideEffectBoundaryCrossed=false` | A corrected unit re-enters the launcher through the same critic chain. |

Verdict: rework.
