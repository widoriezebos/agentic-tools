# Design critique, round 1

## Material findings

### SSTB-101

Severity: critical  
Material: yes

Claim: U1 repeats the session value that already fails. It does not implement the assigned goal's settled design.

Evidence:

- Design sentence: "the waiter resolves the runtime session at registration from the engine's own records" and writes the holder's `SessionId`; U1 assigns that change to `internal/run/waiter.go`. `plans/seats-spend-tokens-in-bounded-sessions-design.md:60,128`.
- Code fact: registration already classifies the child process and copies `view.Announcement.SessionId` to both session fields. `cmd/metasystem/wait_verb.go:144-158`. `lease.CurrentHolder` returns that same announcement field. `internal/lease/verbs.go:266-289`.
- Contract fact: the approved revision for this goal says the announcement's first `sessionId` is the stale value and fixes the fault by adding and associating `runtimeSession`. `plans/registered-wait-matches-the-runtime-session-design.md:12-18,25-43`. It then requires members A, B1, B2, B3, and C in order. `plans/registered-wait-matches-the-runtime-session-design.md:195-200`. U1 omits those owners and would preserve the mismatch.

### SSTB-102

Severity: critical  
Material: yes

Claim: the proposed Stop rule cannot enforce "every call at most 200K" by construction. It observes the crossing after the call and then creates more calls above the cap.

Evidence:

- Design sentence: "a seat's main session stays under 200 thousand tokens of context by construction" and the first block happens "at or over 200K." `plans/seats-spend-tokens-in-bounded-sessions-design.md:5,42`. R11 also leaves an unknown reading unchanged. `plans/seats-spend-tokens-in-bounded-sessions-design.md:161`.
- Data fact: the largest main session reached a 966K peak over 653 calls despite two compactions. `records/misc/token-diagnosis-2026-09-15.md:24`. A Stop refusal then creates 4.3 calls on average. `records/misc/token-diagnosis-2026-09-15.md:165-174`. A call can cross from below the cap before Stop runs, and the refusal turn grows it further. The existing proof actually requires a maximum at or below the ceiling. `internal/steward/contextreport.go:565-570`.
- Wiring fact: the current hook passes session and main identity to `report turn-verdict`, but not the payload's `transcript_path` or a runtime. `scripts/agents/supervision-hook.sh:2336-2339`. The verb has no transcript or runtime flag. `cmd/metasystem/goal.go:653-660`. U3a does not include the hook in its boundary. The existing health read is nonblocking. `internal/steward/context.go:95-99,121-126`. The design does not say whether U3a reuses that sample, rereads the cursor, or fails closed when the sample is unavailable. These choices produce different cap behavior and cost.

### SSTB-103

Severity: critical  
Material: yes

Claim: the forced handoff path still refuses a legal in-flight waiter state, and the design gives no bounded recovery after that refusal.

Evidence:

- Design sentence: a "pending registered wait is recorded" and R12 tests only a pending row. `plans/seats-spend-tokens-in-bounded-sessions-design.md:44,129,162`. The block tells the seat to run the handoff verb alone. `plans/seats-spend-tokens-in-bounded-sessions-design.md:42`.
- Trunk fact: commit `02677d248` classifies both `registering` and `pending` as in flight. `02677d248:internal/run/waiter_states.go:16-20`. Its handoff reader refuses either one with `HANDOFF_WAIT_IN_FLIGHT`. `02677d248:internal/steward/handoff_capture.go:374-426`.
- Failure fact: the command returns a handoff refusal as exit 9 and records no handoff. `cmd/metasystem/context_verbs.go:196-203,335-340`. The next Stop therefore sees no live handoff. The design only says that a third Stop is allowed. It does not say how the seat recovers from the failed command before ending. The counter is keyed by an undefined context "size band," so growth during each refusal may also change the digest and restart the count. `plans/seats-spend-tokens-in-bounded-sessions-design.md:64,159-170`.

### SSTB-104

Severity: critical  
Material: yes

Claim: one generic two-refusal rule cannot safely replace the different existing block policies.

Evidence:

- Design sentence: "the same two-per-unchanged-digest bound applies to every block source" and the third Stop is allowed with an attention item. `plans/seats-spend-tokens-in-bounded-sessions-design.md:64,170`.
- Code fact: unreadable ledger state is an uncertainty refusal. `internal/goal/turnverdict.go:935-963`. Idle backlog at its bound selects a goal, prepares a steward continuation, records an incident, and may raise an alarm. `internal/goal/turnverdict.go:1037-1146`. This is not a generic attention-item template.
- Code fact: unwatched work has its own block-once digest. `internal/goal/turnverdict.go:1184-1218`. Open work blocks once per durable marker or session signature. `internal/goal/turnverdict.go:1404-1416`. Goal revisions have another block-once set. `internal/goal/turnverdict.go:1451-1455`. The design does not define the third-Stop outcome or safety invariant for any of these sources. An implementer must guess whether unreadable, unwatched, or open work may be stranded.

### SSTB-105

Severity: high  
Material: yes

Claim: all three decisions reserved for Wido are already embedded in the design and its units.

Evidence:

- Design sentence: "Until he answers, U3c builds the interactive path." `plans/seats-spend-tokens-in-bounded-sessions-design.md:141`. Q1 also states that the successor "is a fresh interactive `claude` in the same pane." `plans/seats-spend-tokens-in-bounded-sessions-design.md:46`.
- Design fact: choosing headless removes or replaces U3c and U3d. Those units only build the interactive resume and tmux driver. `plans/seats-spend-tokens-in-bounded-sessions-design.md:130-131`.
- Design fact: 200K is hardcoded in S1, U3a, R8, R9, R11, and the DONE threshold while line 142 still offers 300K. `plans/seats-spend-tokens-in-bounded-sessions-design.md:90,114,132,142,158-161`. A 300K answer changes those units, tests, constants, and the expected-day arithmetic.
- Design fact: line 143 leaves rule versus inbox open, but the unit map contains only the rule. No unit owns the inbox record, reader, stop-report change, migration, or tests. `plans/seats-spend-tokens-in-bounded-sessions-design.md:127-136,143`. The design is not buildable under that answer.

### SSTB-106

Severity: high  
Material: yes

Claim: U2 names the wrong command owner and implements only half of its assigned goal.

Evidence:

- Design sentence: U2 puts `wait --local` in `cmd/metasystem/run.go` and defines only local-process rules. `plans/seats-spend-tokens-in-bounded-sessions-design.md:57,133`.
- Code fact: the top-level `wait` family routes to `runWait`. `cmd/metasystem/main.go:718-723`. Its parser is in `cmd/metasystem/wait_verb.go`, rejects positional commands, and accepts only job, run, attempt, goal, or resume. `cmd/metasystem/wait_verb.go:72-88,96-138`. The selector validator also rejects `local`. `internal/run/waiter.go:397-438`. U2 omits the parser owner and part of the selector schema while already sitting at its 300-line ceiling.
- Goal fact: DONE also requires a pending human question, with a live human wait allowing Stop and a dead one not allowing it. `plans/goals/stop-gate-sees-harness-tracked-work.md:8-10`. No U2 rule, boundary, or witness implements that half.

### SSTB-107

Severity: high  
Material: yes

Claim: registry-only liveness has unowned gaps that can allow the seat to end while result handling is still owed.

Evidence:

- Design sentence: the verdict reads "the wait registry only"; a local wait is eligible only while both waiter and child are alive; native Agent work is not covered; and an eligible wait allows Stop with no new message. `plans/seats-spend-tokens-in-bounded-sessions-design.md:50,57-58,62`.
- Code fact: the gate silently omits every row that is not pending, does not have a live waiter, or lacks a fresh heartbeat. `internal/goal/turnverdict.go:561-596,668-707`. A waiter crash while its child lives therefore removes the only liveness signal. A child exit before terminal result delivery also removes the proposed local eligibility. The design has no acknowledgment or recovery state for either window.
- Goal fact: the assigned goal names Codex builds, subagent reads, local runs, and a human answer. `plans/goals/stop-gate-sees-harness-tracked-work.md:8`. A forgotten registration or the permitted short Agent fork creates no row at all. The behavioral sentence "a seat does not stop" is not a gate invariant and names no wake after the session ends.

### SSTB-108

Severity: high  
Material: yes

Claim: U3b cannot add `openWaits` within its boundary, and the resume command it records conflicts with the session-successor contract that U1 is meant to land first.

Evidence:

- Design sentence: the state file gains `openWaits`, including `metasystem wait --resume WAIT-ID`, while U3b changes only `handoff_capture.go`, tests, and a design note. `plans/seats-spend-tokens-in-bounded-sessions-design.md:44,129,162`.
- Code fact: `HandoffState` and its overflow `HandoffManifest` live in `internal/steward/handoff_state.go:112-137`. Decoding rejects unknown fields. `internal/steward/handoff_state.go:139-151`. Validation, list limits, and overflow handling also live outside U3b's boundary. `internal/steward/handoff_state.go:289-365`; `internal/steward/handoff_capture.go:729-789`.
- Contract fact: the final registered-wait design says a successor must not adopt a predecessor row that is not authenticated for the successor session. It must register fresh. `plans/registered-wait-matches-the-runtime-session-design.md:157-169`. The snapshot therefore needs a fresh-registration contract or an explicit compatible transfer rule. Printing the old resume command is not enough.

### SSTB-109

Severity: high  
Material: yes

Claim: U4 puts a policy and certification state machine in Bash and bypasses the repository's delegate path.

Evidence:

- Design sentence: `unit-launcher.sh` parses a plan, dispatches builds, runs direct `claude -p` reads, decides first red, verifies, and runs the landing script. `plans/seats-spend-tokens-in-bounded-sessions-design.md:68,134`.
- Rule fact: committed implementation and test logic belongs in Go. Bash is plumbing. `../development/project-rules-local.md:10`.
- Orchestration fact: rostered roles dispatch through `metasystem delegate`, which owns the record, prompt, permissions, and adapter. `docs/orchestration.md:164-178`. Delegate results and transcripts are untrusted. Only the orchestrator adjudicates and certifies completion. `docs/orchestration.md:253-257`. The design gives no plan schema, retry and resume rules, judgment pause, or certification check before its automatic landing step.

### SSTB-110

Severity: high  
Material: yes

Claim: U6 cannot be the promised "practice from tomorrow." It codifies conflicting rules and depends on later units.

Evidence:

- Design sentence: S3 says "a new round is a new delegate; no `SendMessage` to a delegate." `plans/seats-spend-tokens-in-bounded-sessions-design.md:116`.
- Rule fact: design critique rounds must be follow-ups on one critic chain. A fresh job evades the round budget. `skills/design-critique/SKILL.md:64-67`. Normal corrections also use a recorded follow-up. `docs/orchestration.md:181-183`.
- Design contradiction: S6 requires `context status` before the first act, but the handoff contract says the fresh session runs `context resume` first, and R17 prints that same first-act instruction. `plans/seats-spend-tokens-in-bounded-sessions-design.md:44,119,167`.
- Dependency fact: U6 writes the steward open-waits rule before U3b creates open waits, tells seats to use local waits before U2 creates them, and asks receipts to name daily tokens and landed units before U5 measures them. `plans/seats-spend-tokens-in-bounded-sessions-design.md:115,119,123,127-136`. U6 therefore needs later units to be truthful and usable.

### SSTB-111

Severity: high  
Material: yes

Claim: U5's machine-local UTC ledger cannot measure the stated CEST and three-seat thresholds.

Evidence:

- Design sentence: every threshold is measured by U5, including three seats from 00:00 to 12:45 CEST, a full working day, and m1e at 13:00 CEST. It also says adding subagents makes the fence and diagnosis figures agree. `plans/seats-spend-tokens-in-bounded-sessions-design.md:80,82-92`.
- Code fact: `Measure` normalizes `now` to UTC, produces one machine's UTC day, and writes a UTC-dated ledger. `internal/spend/measure.go:114-135`. Its transcript reader reads one local home and currently enumerates only top-level JSONL files. `internal/spend/transcript.go:43-95`.
- Data fact: the 264.4M fence figure excludes both subagents and the 22:00 to 00:00 UTC slice. `records/misc/token-diagnosis-2026-09-15.md:28`. Adding m1e's 51.9M delegates to 264.4M gives 316.3M, not the 389.2M CEST seat total. No U5 unit owns timezone windows, fleet aggregation, or a same-scope baseline. Line 88 also compares a widened future scope with "today's scope."

### SSTB-112

Severity: high  
Material: yes

Claim: the 179.6M expected day double-counts 20M of work, and that double count is what makes the main-share threshold pass.

Evidence:

- Design sentence: delegates remain 79.9M while engine jobs rise from 12.2M to 32.2M because headless reads "move 20M here." `plans/seats-spend-tokens-in-bounded-sessions-design.md:104-106`.
- Data fact: the diagnosis classifies delegates and engine jobs as disjoint rows of 79.9M and 12.2M. `records/misc/token-diagnosis-2026-09-15.md:77-88`. A move makes delegates 59.9M and engine jobs 32.2M. The corrected total is 159.6M, not 179.6M.
- Threshold fact: the expected main classes total 67.5M: notification 15.5, Stop 13.3, peer 15.8, human 8.4, resume 5.3, and handoff 9.2. `plans/seats-spend-tokens-in-bounded-sessions-design.md:98-105`. That is 42.3 percent of 159.6M, above the stated 40 percent maximum. `plans/seats-spend-tokens-in-bounded-sessions-design.md:89`. If the 20M is additional instead of moved, the total is arithmetically 179.6M, but U4 has not saved that work.

### SSTB-113

Severity: high  
Material: yes

Claim: the rule-only peer option has no mechanism or evidence that cuts wake count from 53 to 30.

Evidence:

- Design sentence: a 400-character seat rule is expected to halve peer wakes, while an inbox is deferred. `plans/seats-spend-tokens-in-bounded-sessions-design.md:72,117,143`.
- Data fact: each peer message wakes the seat, and the diagnosis counted 53 such turn starters. `artifacts/reports/tokens-design-brief-r1.md:27-29`; `records/misc/token-diagnosis-2026-09-15.md:165-172`. Shortening a message does not remove its wake.
- Data fact: the diagnosis classifier puts all peer messages in one class and records no acknowledgement or boundary subtype from which a removable count can be derived. `records/misc/token-diagnosis-2026-09-15.py:48-75`. With no inbox unit and no send-side batching owner, an implementer who builds the rule-only answer cannot claim the 15.8M peer row.

### SSTB-114

Severity: high  
Material: yes

Claim: the landed-unit denominator counts work that did not land and can count one unit twice.

Evidence:

- Design sentence: landed units are History rows whose verb is `land-ready` or `release`. `plans/seats-spend-tokens-in-bounded-sessions-design.md:80,180`.
- Code fact: `release` returns a claim to its resting state. It is not a landing. `internal/goal/verbs.go:1546-1590`. `land-ready` is the distinct act that says work is built and verified and waiting to land. `internal/goal/verbs.go:1596-1648`. A released attempt followed later by a land-ready attempt makes one landing count as two units.
- Data fact: the diagnosis script never reads goal History and is fixed to the 2026-09-15 window. `records/misc/token-diagnosis-2026-09-15.py:1-29,96-115`. The claimed baseline of about six landed units is therefore not produced by the cited diagnosis and cannot validate this denominator.

### SSTB-115

Severity: high  
Material: yes

Claim: U5 does not complete the DONE clause of the goal it claims to land.

Evidence:

- Design sentence: U5 records a single `delegate` cause and R26 only proves internal cause and model sums. The listed proof rows stop at ledger shape and health display. `plans/seats-spend-tokens-in-bounded-sessions-design.md:80,135-136,174-180`.
- Goal fact: the assigned goal requires delegates split by design, build read, and critique, plus one day's report matching provider usage within ten percent. `plans/goals/spend-fence-reports-tokens-per-model-and-cause.md:8-10`.
- Boundary fact: no unit names a provider-usage reader, comparison artifact, tolerance rule, or witness. `plans/seats-spend-tokens-in-bounded-sessions-design.md:135-137`. `Kind` is specified only as main, delegate, or engine, so the required delegate subtypes are also absent from the aggregation contract.

### SSTB-116

Severity: high  
Material: yes

Claim: several unit boundaries omit required state owners, so their changed-line ceilings are not credible.

Evidence:

- Design sentence: U3c puts `successor` "on the intent binding" but lists `handoff_capture.go`, the context verb, the hook, and tests. U5a lists only `transcript.go`, `measure.go`, and tests. `plans/seats-spend-tokens-in-bounded-sessions-design.md:130,135`.
- Code fact: `HandoffBinding` is owned by `internal/steward/handoff.go:23-35`; the durable `Intent` is owned by `internal/steward/intervene.go:25-60`; staging writes it in `internal/steward/stage.go:127-179`; and the launch race is decided in `internal/steward/handoff.go:67-125` and `internal/steward/revive.go:90-206`. U3c omits these owners while allocating 280 lines.
- Code fact: spend parsing is incremental. The cursor persists each cached request and all continuation state in `internal/spend/cache.go:28-51`. `transcript.go` reuses that cache on unchanged and grown files. `internal/spend/transcript.go:189-240`. Turn-starter cause, queued arrivals, recursive subagent discovery, and meta changes need cache schema and invalidation work that U5a omits while allocating exactly 300 lines.

### SSTB-117

Severity: medium  
Material: yes

Claim: several rules have no witness that a builder can run without a fixture bed.

Evidence:

- Design sentence: "bash legs run through the engine on the seat." `plans/seats-spend-tokens-in-bounded-sessions-design.md:147`.
- Design fact: R17, R19, R21, R22, R23, and R29 name only shell fixture legs. R18 names a live proof record instead of a test. `plans/seats-spend-tokens-in-bounded-sessions-design.md:167-173,179`.
- Project fact: the existing wakes design separates focused Go assertions from optional bed legs and says Go owns assertions. `plans/coordinator-wakes-on-events-not-polls-design.md:193-209`. The listed rules need focused, process-free witnesses in their owning units before those units meet the requested acceptance shape.

## Non-material findings

### SSTB-118

Severity: low  
Material: no

Claim: several source anchors are inaccurate, although their nearby symbols are findable.

Evidence:

- Design sentence: the page cites `contextVerdict :292-330`, `OpenWorkSignature` at line 88, and the idle refusal at line 996. `plans/seats-spend-tokens-in-bounded-sessions-design.md:26,64`.
- Code fact: `contextVerdict` begins at `internal/steward/context.go:293`; `OpenWorkSignature` begins at `internal/goal/turnverdict.go:110`; and the refusal text is at `internal/goal/turnverdict.go:1006`. Also, `metasystem.conf:32` is a commented default, not an active setting. `metasystem.conf:27-35`. These anchor errors do not decide implementation after the symbols are located.

### SSTB-119

Severity: low  
Material: no

Claim: the displayed baseline and all-unit columns each add up, but the full-day extrapolation uses a comparison window of a different length.

Evidence:

- Design sentence: the expected table is for the 00:00 to 12:45 CEST window, then scales 179.6M by the prior full-day to first-14-hours ratio. `plans/seats-spend-tokens-in-bounded-sessions-design.md:94-108`.
- Data fact: the source comparison is 1,443.0M for the full day and 776.7M for 00:00 to 14:00, a ratio of about 1.86. `records/misc/token-diagnosis-2026-09-15.md:328-337`. Applying that factor to a 12.75-hour forecast is not a same-window extrapolation. The actual proof day would settle the threshold, so this does not change what is built.

Verdict: rework.
