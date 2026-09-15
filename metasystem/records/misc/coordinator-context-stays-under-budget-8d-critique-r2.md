# Amendment 8d design critique, round 2

Reviewed at `b70fe4f1a998ff1767e59899cee71b44ef195212`.

## Material findings

### CC8D-201

Severity: critical  
Material: yes

Claim: The delegate-class successor cannot perform the fresh wait registrations that `context resume` requires. There is a window in which the predecessor is fenced and the successor cannot restore the recorded waits.

Evidence: The design says, "It registers every `openWork` row as a fresh wait through the wait verb ... under U1's authenticated registration" (`metasystem/artifacts/reports/ctx-8d-design-r2.md:98`). It also says the successor "stays in the delegate class throughout" (`metasystem/artifacts/reports/ctx-8d-design-r2.md:91`). The current wait verb refuses unless the caller is both class `MAIN` and the live lease holder (`metasystem/cmd/metasystem/wait_verb.go:144-158`). U1 keeps that rule. Its successor is explicitly a "successor main" with its own authenticated session (`metasystem/plans/registered-wait-matches-the-runtime-session-design.md:157-167`). U1 does not add a delegate handoff path. This reopens CC8D-111 and defeats SR2's claim that the successor uses paths that exist.

Required correction: Define an authenticated delegate-class registration operation, or choose a successor identity that can lawfully use U1. Name the authority rule and its race witness. Do not call the current wait verb directly from `context resume`.

Rigor: severe. Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false. Reopening trigger: a focused test classifies the real continuation as a delegate and proves that it can register the exact saved target without becoming a second lease holder.

### CC8D-202

Severity: critical  
Material: yes

Claim: The runtime-session fence does not cover all goal mutations. A handed-off predecessor can still mutate the ledger while its successor runs.

Evidence: The design calls `runSyncOnly` "the one assembly point for claim, release, steal, land-ready, edit and the other mutations" and says it fills `RuntimeSession` (`metasystem/artifacts/reports/ctx-8d-design-r2.md:115`). `runSyncOnly` covers only the verbs declared at `metasystem/cmd/metasystem/goalsync_mutations.go:2260-2374`. Other mutating handlers assemble requests directly through `syncReq` or `syncReqClassified`. Examples are `discharge-review-obligation` (`metasystem/cmd/metasystem/goalsync_mutations.go:719-745`) and `accept-risk` (`metasystem/cmd/metasystem/goalsync_mutations.go:748-788`). Approve, revoke, resume, split, budget, grant, and priority handlers have the same separate shape. The actual common assembly point is `syncReqClassifiedWithTerminalGrade` (`metasystem/cmd/metasystem/goalsync_mutations.go:432-534`). This reopens CC8D-110. It also creates the two-live-session mutation window the design says does not exist.

Required correction: Enumerate every mutating command edge and put the token at a truly common edge, or prove an exhaustive smaller set. The mutation proof must delete the token propagation from each distinct edge in turn.

Rigor: severe. Facts: local=true; recoverable=true; proofBoundaryCrossed=false; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false. Reopening trigger: command-edge tests prove every goal mutation from the predecessor session refuses while the successor and unrelated sessions retain their present outcomes.

### CC8D-203

Severity: critical  
Material: yes

Claim: The 135K trigger proof uses an average as a maximum, misreads the cited boot record, and does not prove either p95 under 150K or maximum under 200K. Open question 1 is therefore posed on false arithmetic and offers an answer that SR4 forbids.

Evidence: The design says, "boot near 110K, growth near 4K a call," then derives 139K, 143K, 147K, and 159K upper bounds (`metasystem/artifacts/reports/ctx-8d-design-r2.md:53`). Its witness also hard-codes "two calls at 4K" (`metasystem/artifacts/reports/ctx-8d-design-r2.md:149`). The diagnosis table establishes only 16 calls and a 122K peak for session 9292cf37 (`metasystem/records/misc/token-diagnosis-2026-09-15.md:237-265`). In the cited transcript, the first distinct call is 40,504 prompt tokens, not 110K, and the sixteenth is 121,869 (`/Users/wido/.claude/projects/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/9292cf37-9f88-4700-a7f9-1fe2e45bf000.jsonl:28`, `/Users/wido/.claude/projects/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/9292cf37-9f88-4700-a7f9-1fe2e45bf000.jsonl:214`). Deduplicating by `message.id`, as the diagnosis does, gives mean context 89,490, mean growth 5,424, and a largest observed increment of 14,454. That increment occurs between transcript lines 51 and 71. The program page already warns that 4K is an observed average, not a maximum (`metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:40-44`). Five allowed calls at the observed 14,454 increment can cross 200K. The design also gives no frequency bound for its admitted calls over 150K, so it cannot derive a p95.

Open question 1 says the 110K boot leaves 25K and offers "restate p95 as 200K" (`metasystem/artifacts/reports/ctx-8d-design-r2.md:209`). The observed first-call room is about 94.5K, not 25K. SR4 withdrew this question unless evidence showed the DONE numbers could not be reached, and R-115 item 6 forbids lowering that proof floor (`metasystem/artifacts/reports/ctx-8d-brief-r2.md:418`, `metasystem/memory/rulings.md:174`). The design does not hold under the alternative answer.

Required correction: Withdraw question 1. Replace the average-based witness with a construction that covers a stated maximum increment and the frequency needed for p95. If the data cannot support that construction, return the conflict to Wido without selecting weaker proof numbers.

Rigor: severe. Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false. Reopening trigger: the design derives both DONE thresholds from checked call-level data without treating an average as an upper bound or changing either threshold.

### CC8D-204

Severity: critical  
Material: yes

Claim: The headless failure path can still end without a successor. A Stop block is not an infinite continuation mechanism.

Evidence: The design says each exhausted-launch block waits ten minutes and "no session ends without a successor started" (`metasystem/artifacts/reports/ctx-8d-design-r2.md:60`). It then states, "No other path ends it" because every other Stop blocks (`metasystem/artifacts/reports/ctx-8d-design-r2.md:92`). The Claude adapter always supplies a finite `--max-turns`; the default is 150 (`metasystem/internal/adapter/claude.go:318-339`, `metasystem/internal/adapter/claude.go:391-407`). The design itself counts each blocked Stop as another call. At exhausted launch attempts, there is no successor, so repeated blocks eventually consume the finite limit. The local host contract also says actual conformance needs dated evidence for block behavior and the finite continuation limit (`metasystem/docs/design/turn-verdict-delivery-contract.md:43-48`). The only current live `claude -p` fixture checks an allowed Stop, not a blocked one (`metasystem/scripts/agents/supervision-fixtures.sh:817-837`). The design admits that headless Stop blocking was assumed (`metasystem/artifacts/reports/ctx-8d-design-r2.md:217`). This reopens CC8D-109 and violates P2.

Required correction: Give failed launch a durable continuation owner that survives provider exit and the turn limit. Add dated host evidence that a headless blocked Stop behaves as required, but do not make that behavior the only survival mechanism.

Rigor: severe. Facts: local=false; recoverable=true; proofBoundaryCrossed=false; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true. Reopening trigger: an exhausted-launch proof crosses the provider turn limit and still shows one live recovery owner with no session-ending gap.

### CC8D-205

Severity: critical  
Material: yes

Claim: The continuation claim transfer names fields that do not exist, omits fields that must be preserved, and has no crash rule for its two durable stores.

Evidence: The design says the leg "rewrites the claimant's machine, lineage and main id ... keeps epoch and revision, appends ... `claimChain`" (`metasystem/artifacts/reports/ctx-8d-design-r2.md:113`). A `ClaimRecord` has no main id and no claim epoch. It has machine, lineage, claim time, revision, accounting revision, episode fields, and idle seconds (`metasystem/internal/goal/file.go:279-303`). The design does not say whether the accounting, episode, idle, and claim-time fields survive. The claim is in the goal ledger, while `claimChain` is added to a separate steward intent record (`metasystem/internal/steward/intervene.go:25-60`). The design supplies no ordering or reconciliation for a crash between those writes. Its return leg depends on the chain ending in the successor (`metasystem/artifacts/reports/ctx-8d-design-r2.md:116`). A ledger-first crash can therefore leave a successor-owned claim that the reaper will not return. This reopens CC8D-101 and CC8D-106.

Required correction: Define the exact before and after `ClaimRecord`, preserving every field not intentionally changed. Define an idempotent transaction order and reconciliation for the ledger and intent chain. Test both crash cuts.

Rigor: severe. Facts: local=true; recoverable=false; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=true; externalSideEffectBoundaryCrossed=false. Reopening trigger: crash-cut tests prove that each durable-write ordering converges to either predecessor ownership or successor ownership with a matching claim chain.

### CC8D-206

Severity: high  
Material: yes

Claim: The task state machine still targets the wrong current Claude record shape and treats every status as terminal.

Evidence: The design defines terminal as "a user record holding `<task-notification>`" and says "any `<status>` word ends the flight" (`metasystem/artifacts/reports/ctx-8d-design-r2.md:75`). In the current transcript, Agent launch is an assistant `tool_use`, followed by an immediate user `tool_result` carrying `agentId` in text (`/Users/wido/.claude/projects/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/9292cf37-9f88-4700-a7f9-1fe2e45bf000.jsonl:126-127`). Background Bash returns only the text `Command running in background with ID: bwuqu02qi` (`/Users/wido/.claude/projects/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/9292cf37-9f88-4700-a7f9-1fe2e45bf000.jsonl:155-156`). Both completion notifications are top-level `queue-operation` records. They are not user records (`/Users/wido/.claude/projects/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/9292cf37-9f88-4700-a7f9-1fe2e45bf000.jsonl:220`, `/Users/wido/.claude/projects/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/9292cf37-9f88-4700-a7f9-1fe2e45bf000.jsonl:270`). Their fields are tags inside the `content` string: `task-id`, `tool-use-id`, `output-file`, `status`, and `summary`. SR5 requires terminal states, not any status value (`metasystem/artifacts/reports/ctx-8d-brief-r2.md:419`). An implementation following the design will miss completions or end a task on a future nonterminal status. This reopens CC8D-104.

Required correction: Specify the top-level record types, text grammars, identity joins, and the exact terminal status set. Preserve the repeated enqueue and remove events for one task id in the fixture set.

Rigor: severe. Facts: local=false; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=true; externalSideEffectBoundaryCrossed=false. Reopening trigger: sanitized fixtures copied from the current record shapes prove Agent and Bash launch, completion, failure, repeated notification, resume, and nonterminal status behavior.

### CC8D-207

Severity: high  
Material: yes

Claim: Successor-death recovery cannot resume a replacement under the same nonce as designed.

Evidence: The design says `context resume` writes `resume.json` "once, exclusively" with one `successorJob` and one `successorSession` (`metasystem/artifacts/reports/ctx-8d-design-r2.md:99`). It later says the reaper relaunches a dead successor "under the same nonce and state" (`metasystem/artifacts/reports/ctx-8d-design-r2.md:108`). S6 requires the replacement's first act to be `context resume` (`metasystem/docs/orchestration.md:369`). The second successor therefore encounters the first successor's exclusive record. The design gives no idempotent same-successor rule, replacement-successor rule, or per-attempt record. Its own `TestClaimContinueHandoffAdmitsARelaunchedSuccessor` cannot repair the earlier resume failure (`metasystem/artifacts/reports/ctx-8d-design-r2.md:179`). This reopens CC8D-106.

Required correction: Make resume evidence attempt-scoped or define an atomic replacement history. Record dead time and wait-registration results for every launched successor, not only the first.

Rigor: severe. Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false. Reopening trigger: a focused death and relaunch test runs the replacement's required first act under the same handoff and records both attempts without overwriting evidence.

### CC8D-208

Severity: high  
Material: yes

Claim: The tool gate can force an immediate handoff loop before the successor takes its claim or reaches `goal next`.

Evidence: The design requires the successor's first three acts to be `context resume`, `goal claim --continue-handoff`, and `goal next` (`metasystem/artifacts/reports/ctx-8d-design-r2.md:101`). Its allowlist includes `context resume`, but it does not include the continuation claim or `goal next` (`metasystem/artifacts/reports/ctx-8d-design-r2.md:67-68`). A successor that boots near the trigger may run resume, cross the trigger, and then be denied its mandatory second act. The observed single-call increase reached 14,454 tokens, not 4K, as shown by the cited transcript at lines 51 and 71. The design has no minimum trigger relative to measured boot, no first-act reserve, and no exemption for the required claim sequence. Its response is another allowed `context handoff`, so a successor can hand off before its first goal act. The predecessor is already fenced at the running signal (`metasystem/artifacts/reports/ctx-8d-design-r2.md:85`, `metasystem/artifacts/reports/ctx-8d-design-r2.md:115`). This can leave no actor able to advance the claim.

Required correction: Reserve measured room for the complete mandatory startup sequence and allow that exact sequence. Add a witness where boot plus the largest admitted call growth crosses the trigger after resume.

Rigor: severe. Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false. Reopening trigger: the successor completes resume, claim, and `goal next` before any new handoff under the worst admitted boot and growth values.

### CC8D-209

Severity: high  
Material: yes

Claim: The 100 ms rule still starts its clock too late and relies on unproved harness timeout behavior.

Evidence: The design says the shell starts an engine and "the verb starts its clock at entry" (`metasystem/artifacts/reports/ctx-8d-design-r2.md:66`). SR6 binds the whole hook invocation, including shell startup and classification, to about 100 ms (`metasystem/artifacts/reports/ctx-8d-brief-r2.md:420`). The current common path performs engine subprocess work after the shell has started (`metasystem/scripts/agents/supervision-hook.sh:1461-1478`, `metasystem/scripts/agents/supervision-hook.sh:1577-1589`). Starting the clock in the Go verb excludes shell parsing, engine discovery, and process startup. The named tests cover the reader clock and a stub-engine script route, but no witness measures the whole invocation with the same injected deadline (`metasystem/artifacts/reports/ctx-8d-design-r2.md:155`). The design also assumes that a provider hook timeout or missing JSON allows the call (`metasystem/artifacts/reports/ctx-8d-design-r2.md:66`, `metasystem/artifacts/reports/ctx-8d-design-r2.md:217`). The local host contract says static fixtures do not prove actual host behavior (`metasystem/docs/design/turn-verdict-delivery-contract.md:43-48`). If the assumption is wrong, the hook can block the landing and wait paths that R-114 says must never block. This reopens CC8D-112.

Required correction: Carry one deadline from shell entry through the engine and make timeout output explicitly allow. Add a focused whole-hook artificial-clock witness and dated host conformance for provider timeout handling.

Rigor: unproven. Facts: local=true; recoverable=true; proofBoundaryCrossed=false; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false. Reopening trigger: the full shell-to-decision path proves its deadline under cold engine and partial-line cases, and current host evidence proves timeout cannot deny a landing or wait.

### CC8D-210

Severity: high  
Material: yes

Claim: The landing order activates enforcement before an automatic successor exists, and it permits U2c before U3c is complete.

Evidence: The design orders U3e before U1 and every U3c member, then says that until U3c-1b lands, `context handoff` only records and does not launch (`metasystem/artifacts/reports/ctx-8d-design-r2.md:205`). U3e denies delegate launches at the 250K ceiling while leaving that non-launching handoff as the only continuation command (`metasystem/artifacts/reports/ctx-8d-design-r2.md:68`). A seat that reaches the ceiling in this landed interval can neither launch a delegate nor start a successor. That violates R-114 item 6 (`metasystem/memory/rulings.md:173`). The graph also says "U2c after U3c-4a" while U3c-4b remains later (`metasystem/artifacts/reports/ctx-8d-design-r2.md:205`). P7 orders U2c after all of U3c (`metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:128-142`). The remaining listed dependencies are acyclic, and all U3c work correctly waits for U1, but these two activation edges are wrong. This reopens CC8D-102.

Required correction: Separate building U3e from activating its deny policy, with activation after U3c-1b and its failure owner. Put U2c after U3c-4b.

Rigor: severe. Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false. Reopening trigger: every landed prefix of the unit graph leaves a lawful continuation path, and the graph places U2c after all U3c members.

### CC8D-211

Severity: medium  
Material: yes

Claim: Open questions 2 and 3 do not have implementable branches under every answer.

Evidence: Question 2 recommends raising the ceiling by one configuration line (`metasystem/artifacts/reports/ctx-8d-design-r2.md:210`). D1 rejects any configuration whose trigger exceeds 135K (`metasystem/artifacts/reports/ctx-8d-design-r2.md:50`). Raising only the 250K ceiling while keeping the 115K margin raises the trigger above 135K, so the recommended change is rejected. Its alternative adds an attention branch, but no unit row or mutation witness owns that branch (`metasystem/artifacts/reports/ctx-8d-design-r2.md:142-182`). Question 3 says a successor receipt line costs "no new source" (`metasystem/artifacts/reports/ctx-8d-design-r2.md:211`). D8.3 names intent, job, resume, goal, and context budget as its sources, not the receipt ledger (`metasystem/artifacts/reports/ctx-8d-design-r2.md:107`). The current typed job record has no receipt field (`metasystem/internal/dispatch/jobrecord.go:17-126`). A receipt written after handoff must be read from another source. These are Wido's choices, but both branches must first be made internally consistent.

Required correction: For question 2, specify the coupled ceiling and margin change required to keep the trigger valid, and assign the alternative a unit and witness. For question 3, name the receipt source, freshness rule, allocation, and witness. Do not choose either answer.

Rigor: severe. Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false. Reopening trigger: every answer to questions 2 and 3 maps to a valid configuration and a named, sized, falsifiable implementation branch.

### CC8D-212

Severity: medium  
Material: yes

Claim: The S7 note-location rule rests on a nonexistent resolver, so CC8D-113 is not fully closed.

Evidence: The design says the seat memory directory is "the `memory/` sibling of the transcript's project directory that internal/usage already resolves" (`metasystem/artifacts/reports/ctx-8d-design-r2.md:76`). `internal/usage` resolves a Claude transcript from home, top-level, installation, and the Claude project slug (`metasystem/internal/usage/calls_claude.go:14-38`). Its `ReadOptions` has no memory-directory field or resolver (`metasystem/internal/usage/calls.go:64-72`). The unit is already allocated 300 changed lines and assumes this path owner exists (`metasystem/artifacts/reports/ctx-8d-design-r2.md:192`). An implementer must invent whether memory is derived from the explicit transcript, the registered project, or the installation, and how symlinks are confined.

Required correction: Define one canonical memory-directory resolver, its trust inputs, and its symlink rule. Add it to U3b-2's file list and allocation.

Rigor: severe. Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=true; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false. Reopening trigger: path tests prove the accepted project memory directory and reject an explicit transcript, slug, or symlink that escapes it.

### CC8D-213

Severity: medium  
Material: yes

Claim: The rule-to-witness table does not make every rule falsifiable without a fixture bed, and one saturated unit contains an unowned reporting surface.

Evidence: The design says every rule names a focused test that fails if it is removed (`metasystem/artifacts/reports/ctx-8d-design-r2.md:142`). D2.1's focused test only accepts command flags; only its hook-bed leg observes whether the shell passes them (`metasystem/artifacts/reports/ctx-8d-design-r2.md:150`). D6.3, D7.5, and D8.5 have no rows in the table at all (`metasystem/artifacts/reports/ctx-8d-design-r2.md:169-181`). Deleting the installation-root wiring, the required first-act order, or the predecessor relay rule can leave every named focused test green. P7 requires a focused witness without a fixture bed or live session (`metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:128-130`). This reopens CC8D-116.

The same unit map adds a weekly `context report` endpoint in U3c-3 (`metasystem/artifacts/reports/ctx-8d-design-r2.md:99`, `metasystem/artifacts/reports/ctx-8d-design-r2.md:197`). R-115 requires dead time to be recorded and later read by U5a, not a new weekly report verb (`metasystem/memory/rulings.md:174`). No accepted finding or DONE clause owns that endpoint. U3c-3 is already allocated the full 300 lines. This is avoidable scope in a design that is 2,420 lines over P7's estimate.

Required correction: Add focused script-to-command and sequence witnesses for the missing rules. Remove the weekly report surface unless a binding requirement owns it. Reallocate any retained surface before briefing the unit.

Rigor: severe. Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false. Reopening trigger: every D rule has a non-bed mutation witness, and every surface in each saturated unit cites a finding or DONE owner.

## Non-material finding

### CC8D-214

Severity: low  
Material: no

Claim: The 3,740-line total is not by itself a reason to reject the design.

Evidence: The design gives every unit a numeric allocation at or below 300 and lists tests and production files (`metasystem/artifacts/reports/ctx-8d-design-r2.md:186-203`). That conforms to the per-unit rule (`metasystem/docs/design/design-principles.md:118-130`). Most of the increase over P7's 1,320-line estimate is attached to accepted round-1 findings in the per-unit rationale column. The material size defects are the unowned endpoint and missing witnesses in CC8D-213, not the aggregate number alone.

Round-1 closure: CC8D-101, 102, 103, 104, 106, 109, 110, 111, 112, 113, and 116 are reopened by CC8D-201 through CC8D-213. CC8D-105 is folded by the durable retry lifecycle at `metasystem/artifacts/reports/ctx-8d-design-r2.md:82-85`. CC8D-107 is folded for the actual landing and wait spellings at `metasystem/artifacts/reports/ctx-8d-design-r2.md:67-68`. CC8D-108 is folded by the shipped-hook owner at `metasystem/artifacts/reports/ctx-8d-design-r2.md:65`. CC8D-114 is folded by the validator and checked-in configuration surfaces at `metasystem/artifacts/reports/ctx-8d-design-r2.md:50`. CC8D-115 is folded by schema 2 and the dual reader at `metasystem/artifacts/reports/ctx-8d-design-r2.md:77`. The B1 dependency is also correctly placed before every U3c unit at `metasystem/artifacts/reports/ctx-8d-design-r2.md:195-205`, and the checked advisor branch remains read-only at `metasystem/internal/up/up.go:691-724`.

Material findings: 13.

Verdict: rework.
