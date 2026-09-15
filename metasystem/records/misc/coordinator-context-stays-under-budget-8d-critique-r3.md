# Design critique, amendment 8d, round 3

## Material findings

### CC8D-215

Severity: critical  
Material: yes

Claim: The arithmetic is correct, but neither proof threshold holds by construction. This reopens CC8D-203.

Evidence: The design says, "the p95 holds while those are under one in twenty calls of the week's cohort," and names a seventh admitted call, a larger step, repeated never-denied calls, and observe mode as residual findings (`metasystem/artifacts/reports/ctx-8d-design-r3.md:51`). That is a measured condition, not a construction. The trigger is 105,000. The line `150000 - 3 * 14454` is 106,638. At the default trigger, three steps give 148,362, six give 191,724, and seven give 206,178. The checked transcript supports the inputs: 40,504 at line 28, 48,990 at line 51, 63,444 at line 71, 121,869 at line 214, and 207,575 at line 702 (`/Users/wido/.claude/projects/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/9292cf37-9f88-4700-a7f9-1fe2e45bf000.jsonl:28`, `:51`, `:71`, `:214`, `:702`). Deduplication by `message.id` gives a largest step of 14,454. It also gives 5,424.3 mean growth over the first 16 calls and 2,456.9 over all 69. The 64,496-token room therefore supports 11.89 or 26.25 observed mean steps. It is enough for real work under these observations. It is not a hard step bound. The gate has no count or frequency bound on Agent, landing, wait, or other never-denied calls (`metasystem/artifacts/reports/ctx-8d-design-r3.md:59-60`). The DONE requires p95 below 150K and no call above 200K (`metasystem/plans/goals/coordinator-context-stays-under-budget.md:8`). R-114 requires the tool hook to make the stated size hold by construction (`metasystem/memory/rulings.md:173`). The proposed implementation can exceed 200K after the seventh admitted step and can put more than five percent of calls above 150K without violating any gate rule.

Build effect: This blocks the cap group as a whole. SR14's never-denied operations and R-114's construction requirement need a design or ruling resolution. It is not a safe build-time fold.

Rigor: severe. Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false.

### CC8D-216

Severity: high  
Material: yes

Claim: The task state machine classifies observed terminal statuses as unknown and still in flight. This reopens CC8D-206.

Evidence: The design says, "Terminal status set: `{completed}`" and treats every other status as non-terminal (`metasystem/artifacts/reports/ctx-8d-design-r3.md:68`). SR13 requires every status the harness can emit to be classified (`metasystem/artifacts/reports/ctx-8d-brief-r3.md:250`). The local transcript corpus contains a background Bash notification with `<status>failed</status>` (`/Users/wido/.claude/projects/-Users-wido-LocalStorage/c53cbef5-7be0-48fa-83f1-876f40a3c193.jsonl:839`). It also contains an Agent notification with `<status>killed</status>` whose note says the agent stopped and may later resume under the same task id (`/Users/wido/.claude/projects/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/2c27f5cc-245a-4735-a402-dafc351e4caf.jsonl:135`). Both end the current flight. A later successful resume starts another flight. The design would instead record both as running. Its next sentence, "A task is in flight iff its last event is a launch or a resume," also conflicts with its rule that an unknown-status notification leaves the task in flight (`metasystem/artifacts/reports/ctx-8d-design-r3.md:68`). An implementer has two incompatible transition rules.

Build effect: This blocks U3b-2a. The complete observed terminal table and one transition rule can be folded into its brief before build. A genuinely unknown word can still remain in flight and be reported.

Rigor: severe. Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false.

### CC8D-217

Severity: high  
Material: yes

Claim: The resume transition uses an unobserved result field and marks an attempted SendMessage as a resume before success is known.

Evidence: The design says a `SendMessage` whose `input.to` names a task, or a later result whose `toolUseResult.agentId` names it, puts the task back in flight. It admits that the resume fixture is unobserved (`metasystem/artifacts/reports/ctx-8d-design-r3.md:68`, `:149`). A checked resume uses `input.to` in the request, then returns `toolUseResult.success: true` and `toolUseResult.resumedAgentId`; it has no `toolUseResult.agentId` (`/Users/wido/.claude/projects/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/36c93128-ec16-4146-8def-3f706ba4ef10.jsonl:14039-14040`). The input alone cannot distinguish a successful resume from a refused or failed send. The specified result matcher misses the observed success field. A failed attempt can therefore create a false in-flight task, while an implementation that waits for the specified result field never observes a successful resume.

Build effect: This blocks U3b-2a. The observed success and failure result grammar can be folded into that unit before build.

Rigor: severe. Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false.

### CC8D-218

Severity: high  
Material: yes

Claim: The bounded reader can wait forever on either existing usage lock before it reaches a deadline check. This reopens CC8D-209.

Evidence: The design says `usage.LatestCall` gets `MaxBytes`, `Deadline`, and `Clock`, checks the clock between lines, and makes every slow path return no decision by the deadline (`metasystem/artifacts/reports/ctx-8d-design-r3.md:58`). It neither requires `NonBlocking: true` nor names a contended-lock witness (`metasystem/artifacts/reports/ctx-8d-design-r3.md:100-101`). Current `LatestCall` takes the maintenance lock before calling the reader (`metasystem/internal/usage/calls.go:124-131`). The lock is blocking unless `NonBlocking` is set (`metasystem/internal/usage/evidence.go:102-124`). The cursor reader then takes another blocking flock by default (`metasystem/internal/usage/cursor.go:55-67`, `:615-639`). A deadline check between transcript lines cannot bound time spent in either flock. The shipped hook can then delay a call past the promised 100 ms and up to host timeout, including observe mode.

Build effect: This blocks U3e-1. A nonblocking or deadline-aware lock contract and a contention witness can be folded before build.

Rigor: severe. Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false.

### CC8D-219

Severity: high  
Material: yes

Claim: The one-line observe-to-deny interface is not stable because the project PreToolUse hook also runs inside native Agent subagents.

Evidence: The design says job settings receive no gate, skips only `METASYSTEM_HOOK_DELEGATE_JOB`, and promises that the successor changes only `context.toolgate.mode=observe` to `deny` (`metasystem/artifacts/reports/ctx-8d-design-r3.md:57-58`, `:138`). The current job adapter does install only SessionStart (`metasystem/internal/adapter/claude.go:112-131`), but that covers adapter jobs, not native Agent subagents. Claude Code's current hook contract says settings hooks run in subagents and gives PreToolUse input an `agent_id` field (`https://code.claude.com/docs/en/hooks:400-403`). The proposed project hook has no matcher and the branch does not inspect that field (`metasystem/artifacts/reports/ctx-8d-design-r3.md:57-58`). In deny mode, a native Agent's Bash, Read, Edit, and other working tools therefore enter the coordinator gate. Allowing the outer `Agent` launch does not allow the launched agent's tools. S1 relies on delegates doing the remaining work (`metasystem/docs/orchestration.md:355-357`). The successor cannot safely flip one line without either changing the cap hook or accepting delegate denials.

Build effect: This blocks U3e-2b and the successor interface. The native-subagent scope and its focused witness can be folded into U3e-2b before build.

Rigor: severe. Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false.

### CC8D-220

Severity: medium  
Material: yes

Claim: A runtime without a transcript can record an incomplete delegate, contrary to the runtime-independent open-work contract.

Evidence: The design makes `asked` and `output` optional in `--delegate id=<a>[,asked=...][,output=...]`, fills omissions from the transcript, then says, "A runtime with no transcript takes the declaration as the record" (`metasystem/artifacts/reports/ctx-8d-design-r3.md:68`). It does not require the omitted fields when no transcript exists. R-115 requires every in-flight Agent delegate to carry id, what it was asked, and output path, or requires no delegate to be running (`metasystem/memory/rulings.md:174`). The goal also requires the non-accelerated path to give every runtime the same guarantee from records and the verb alone (`metasystem/plans/goals/coordinator-context-stays-under-budget.md:8`). An implementer following the grammar can accept `--delegate id=a` on Devin or another runtime and persist neither the assignment nor the output witness.

Build effect: This blocks U3b-2a. The no-transcript declaration requirements can be folded into that unit before build without deciding a Wido question.

Rigor: severe. Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false.

### CC8D-221

Severity: medium  
Material: yes

Claim: The U3e unit boundaries omit the shipped observe line from its owning unit and add a tracked generated file that SR3 excludes.

Evidence: D3.5 assigns the shipped `context.toolgate.mode=observe` line and its witness to U3e-2a (`metasystem/artifacts/reports/ctx-8d-design-r3.md:61`, `:104`). The U3e-2a file list names `internal/config` but not `metasystem.conf` (`metasystem/artifacts/reports/ctx-8d-design-r3.md:123`). U3a-1a names `metasystem.conf`, but its DONE owns D1.1 and D1.3 to D1.6, not D3.5 (`metasystem/artifacts/reports/ctx-8d-design-r3.md:117`). U3e-2b instead lists "the checkout's .claude/settings.json through `metasystem up`" (`metasystem/artifacts/reports/ctx-8d-design-r3.md:124`). Both files are tracked in this checkout. SR3 expressly says U3e edits the shipped hooks file and what `hooks check` reads, not `.claude/settings.json` (`metasystem/artifacts/reports/ctx-8d-brief-r3.md:235`). The current tracked installed file is generated hook output (`.claude/settings.json:1-38`). An implementer must either touch an unlisted file for U3e-2a or leave the required shipped mode absent, and U3e-2b directs a forbidden tracked edit.

Build effect: This blocks U3e-2a and U3e-2b. Correcting the two file boundaries and their allocations is a build-brief fold.

Rigor: unproven. Facts: local=true; recoverable=true; proofBoundaryCrossed=false; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false.

## Non-material findings

### CC8D-222

Severity: low  
Material: no

Claim: The design's claim that unknown `context.` keys are refused today is false, but its explicit new refusal still tells the implementer what to build.

Evidence: D1.1 says, "an unknown `context.` key is refused as validate.go refuses any unknown key today" (`metasystem/artifacts/reports/ctx-8d-design-r3.md:48`). Current validation parses every key into `values`, checks only selected domains, and has no general unknown-key rejection (`metasystem/internal/config/validate.go:37-68`, `:101-105`, `:496-512`). The factual rationale is wrong. The same design sentence and `TestContextConfKeysDocumented` still explicitly require the new `context.` rejection, so current implementation behavior is not ambiguous.

### CC8D-223

Severity: low  
Material: no

Claim: D4.1 names the two in-flight waiter states literally instead of naming the current shared classification owner.

Evidence: The design says the record refuses while any `registering` or `pending` row is the caller's (`metasystem/artifacts/reports/ctx-8d-design-r3.md:66`). Current main owns waiter state classification in `run.WaiterStates` (`metasystem/internal/run/waiter_states.go:3-20`), and the handoff reader consumes that table and refuses every state classed in flight (`metasystem/internal/steward/handoff_capture.go:415-426`). The current table's in-flight set is exactly `registering` and `pending`, so the literal wording does not change today's result. Using the shared owner during U3b-1 would avoid recreating the drift that the current code removed.

Verdict: rework.
