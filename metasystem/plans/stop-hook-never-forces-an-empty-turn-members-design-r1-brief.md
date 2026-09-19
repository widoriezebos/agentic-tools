# Design brief: stop-hook-never-forces-an-empty-turn members design

## Revision

Revision: first draft of the page stop-hook-never-forces-an-empty-turn-members-design.md

Reason: Wido accepted the seat's recommendation of 2026-09-12 to design and build the umbrella per member instead of polishing the umbrella page. Members 3 to 7 have no design of their own yet; this page makes each of them buildable by one Codex job from its section alone.

## Context pack

Read this pack first. Open another file only to check one of the cited lines
below; do not widen the read beyond that check.

Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn.

Diagnosis or prior revision:

This is the first draft of the members page. The prior revision is the umbrella page, plans/stop-hook-never-forces-an-empty-turn-design.md at commit 4e4e46de7, copied whole further below in this section. Its section 7 names eight members in landing order; its sections 2, 5 and 6 hold the mechanism, the fixtures and the landing rules the members inherit.

What this round produces: one page with one section per member for members 3 to 7, in landing order, plus one short section for member 8 and one paragraph on member 2. Each member section is the build brief for that member (the page is the brief): a builder receives the section, the workspace and the return shape, nothing else, so the section must be complete on its own. Every member section has these seven parts, in this order:

1. DONE. The member's DONE from umbrella section 7, restated as numbered rules; each rule names the fixture that fails without it.
2. Mechanism. The functions, records and seams the member changes or adds, by symbol and file; what the member reads and writes; what it does not change. New symbols are named. Line numbers only where they come from the excerpts below.
3. File set. Every file the build touches: existing files by path as they are at 4e4e46de7, new files by full path, test files included.
4. Fixtures. Every test by name and package. The section 5 names that belong to the member are kept (four of them exist on main today, listed below; the rest are new). Each fixture states the assertion that fails without the mechanism and the seams it injects (clock, sleep, file stat, process prober, fake hook log).
5. Mutations. For each fixture, one source mutation that makes it red: file, function, what to break. The seat runs these on the host after the build.
6. Estimate. Changed lines including tests. The expected size of a member is 600 to 1,500 changed lines; that is an estimate, not a cap. A member that cannot be built inside one section says why and proposes the split.
7. Landing. What lands with the member (records, docs, the testing.json group of each new test), the proof commands (whole packages, plain and with the batchtest tag), and what the member leaves for the next one.

Members in scope, in landing order: 3 stop-incidents-reach-the-steward; 4 stop-frontier-joins-owner-and-revision; 5 stop-refusals-name-seat-actions; 6 stop-fences-surface-once; 7 stop-hosts-enforce-the-common-gate.

Member 8, stop-refusals-under-ten-a-day, is an observation, not a build. Give it one short section: what is counted (refusals per seat per day), from which lines and records (the hook-log verdict line and stop-condition lines of excerpts 16 and 17, the refusal record of excerpt 9), the command or script by which a seat produces the seven-day count, and what member 7 must leave behind so that the count is readable per seat per day. No fixtures.

Member 2, stop-decisions-record-deadline-evidence, is parked and was decoupled from the umbrella by Wido on 2026-09-19. No member in this page may depend on member 2 landing. Where the umbrella or the example page assumes the version-2 incident record, the member reads what exists today: the refusal record written by StopRefusal (schemaVersion 1, excerpt 9), the hook-log lines (excerpts 16 and 17) and the per-session verdict state. Each member section says in one sentence what changes if member 2 lands later.

Member 1, stop-infrastructure-allows-the-seat-to-stop, is concluded (commit c8074077). Its typed classification is in place; the members build on it and do not redesign it.

Facts about the tree at 4e4e46de7, checked by the seat:

- Of the fixture names in umbrella section 5 only four exist today: TestArmingDetailSurvivesStop in cmd/metasystem/up_test.go, and TestIdleBacklogBlocksTwiceThenDefersClaimAndPreparesStewardContinuation, TestIdleRefusalSurvivesALostCounter and TestSessionStopInfrastructurePreservesAuthority in internal/goal/turnverdict_idle_test.go. Every other section 5 name is new. Keep the names.
- There is no host stop-gate verb, no DrainStopIncidents, no version-2 incident record, and the Codex adapter (excerpt 19) has no stop gate at its turn boundary. The Claude Stop hook runs supervision-hook.sh claude stop (excerpt 18); the hook calls report turn-verdict (excerpt 15) and appends stop-condition lines and the verdict line to artifacts/agents/supervision/hooks.log (excerpts 16 and 17). The stop block and the refusal record are rendered by internal/report/stopblock.go (excerpt 9), called from the report stop-block verb (excerpt 13).
- The idle refusal, its digest, the escalation event and the continuation command are composed in internal/goal/turnverdict.go (excerpts 1 to 3); the fenced-claim lines and the queued-frontier digest are in the same file (excerpts 4 to 7); the board is Project in internal/goal/project.go (excerpt 8); goal next prints the fenced lines (excerpt 14).
- The steward tick (excerpts 10 and 11) queues notifications through QueueNotification (excerpt 12) and writes the narrator digest after its decision. A drain of stop incidents sits before NarrateDigest and delivers through the same queue.
- Stop-related source on main, by path under metasystem: cmd/metasystem/session_stop.go, internal/adapter/stopoutput.go, internal/audit/stopsurface.go, internal/dispatch/stop.go, internal/dispatch/stopfence.go, internal/goal/sessionstop.go, internal/goal/stop.go, internal/goal/stopsurface.go, internal/report/stopblock.go, internal/report/stopcompletion.go, internal/report/stoppresentation.go, internal/run/stop.go, internal/stopfence/fence.go, internal/stopreport (response.go, storage.go, stopreporttest/fixture.go), internal/stoptransition (families.go, transition.go), scripts/agents/fixture-stop-report.sh, scripts/agents/stop-degraded-forms.sh, and 32 test files with stop in their name. A member that changes behaviour one of them asserts adapts the body and keeps the name.

Rules that bind every section:

- A test never depends on wall-clock time: injected clocks and fakes only; never t.Skip, never a raised bound, never a retry. A fixture that needs a process uses the process fixtures of internal/testutil with readiness lines, never a sleep.
- One mechanism per member; a member has its own DONE and lands alone. This page does not restate or improve the umbrella; it only makes each member buildable.
- The engine never names a runtime. Claude and Codex specifics stay in scripts/agents/supervision-hook.sh and scripts/agents/adapters; the common gate is one Go verb both call.
- Read rules on append-only records (registries, ledgers, logs, hooks.log, mains and cursor files) are never tightened; records written by older code still read.
- Listed test names are protected; a member renames or drops nothing that testing.json lists.
- Source comments say what the code does and why, in plain English; never a round, a finding, a seat or a goal id in code or comments.
- Plain English, short sentences, things named by symbol and path. Tables only for fixture lists.

The umbrella page, plans/stop-hook-never-forces-an-empty-turn-design.md at 4e4e46de7, complete:

    # Stop hooks must not force empty turns
    
    Design for goal 4 of `metasystem/plans/delivery-efficiency-plan.md`, dated 2026-09-12.
    The accepted scope and absorbed clauses are in `metasystem/plans/goals/stop-hook-never-forces-an-empty-turn.md:8` and line 10.
    Drafted in three Codex rounds (chain stop-hook-design-190340, gpt-6-astra), read twice by Codex gpt-5.6-sol, settled by the m1b seat under Wido's lanes of 2026-09-12 (the seat designs, Codex reads and builds, Opus reads the build).
    The second read's six findings are folded below: the idle refusal survives a lost counter, the incident record has an independent second channel, the idle digest is not amended, the launcher block moves into the first member, the members are reordered so each lands on its own, and the seven-day observation has an owner.
    
    ## 1. What refuses today and why
    
    The audit records 331 refusals and 292 forced coordinator turns over five days.
    Its 184 narrator reads, 117 deadlines, and about 95 arming failures are separate recorded counts; they are not a partition of the 331 refusals.
    The census below counts trigger sites and catchall routes, not distinct messages or calls to a renderer.
    There are 45 infrastructure rows, five seat-actionable branches, and two idle branches.
    The infrastructure count includes 27 hook or launcher routes and 18 engine producers reached through the unavailable-verdict route. These are overlapping routes, not 45 distinct incidents.
    
    | Class | Trigger and evidence |
    | --- | --- |
    | Steward-owned infrastructure | Invalid runtime argument: `metasystem/scripts/agents/supervision-hook.sh:15`. |
    | Steward-owned infrastructure | Invalid event argument: `metasystem/scripts/agents/supervision-hook.sh:16`. |
    | Steward-owned infrastructure | Claude's launcher converts any nonzero hook exit, including either argument error, into an unpersisted raw block: `metasystem/scripts/enforcement/claude-code-hooks.json:25`. |
    | Steward-owned infrastructure | Invalid or empty worker output, or nonzero worker exit: `metasystem/scripts/agents/supervision-hook.sh:218`. |
    | Steward-owned infrastructure | Stop deadline expired: `metasystem/scripts/agents/supervision-hook.sh:329`. |
    | Steward-owned infrastructure | Engine unavailable: `metasystem/scripts/agents/supervision-hook.sh:399`. Repair of the enrolled installation belongs to the steward; its present rebuild advice does not prove seat authority. |
    | Steward-owned infrastructure | Runtime name unreadable: `metasystem/scripts/agents/supervision-hook.sh:514`. |
    | Steward-owned infrastructure | Runtime process identity unreadable: `metasystem/scripts/agents/supervision-hook.sh:523`. |
    | Steward-owned infrastructure | Fallback identity classification failed: `metasystem/scripts/agents/supervision-hook.sh:537`. |
    | Steward-owned infrastructure | Fallback runtime name unreadable: `metasystem/scripts/agents/supervision-hook.sh:541`. |
    | Steward-owned infrastructure | Fallback process identity unreadable: `metasystem/scripts/agents/supervision-hook.sh:549`. |
    | Steward-owned infrastructure | Turn evidence preparation failed: `metasystem/scripts/agents/supervision-hook.sh:745`. |
    | Steward-owned infrastructure | Attempt evidence write failed: `metasystem/scripts/agents/supervision-hook.sh:751`. |
    | Steward-owned infrastructure | Attempt evidence unreadable: `metasystem/scripts/agents/supervision-hook.sh:757`. |
    | Steward-owned infrastructure | Checkout holder classification failed: `metasystem/scripts/agents/supervision-hook.sh:766`. |
    | Steward-owned infrastructure | Checkout holder classification unreadable: `metasystem/scripts/agents/supervision-hook.sh:772`. |
    | Steward-owned infrastructure | Arming failed: `metasystem/scripts/agents/supervision-hook.sh:836`; line 835 retains only the aggregate, losing component detail. |
    | Steward-owned infrastructure | Health returned no verdict: `metasystem/scripts/agents/supervision-hook.sh:842`. |
    | Steward-owned infrastructure | Narrator response fields unreadable: `metasystem/scripts/agents/supervision-hook.sh:855`. |
    | Steward-owned infrastructure | Narrator read failed: `metasystem/scripts/agents/supervision-hook.sh:859`. |
    | Steward-owned infrastructure | Holder protocol read failed: `metasystem/scripts/agents/supervision-hook.sh:1037`. |
    | Steward-owned infrastructure | Holder protocol response unreadable: `metasystem/scripts/agents/supervision-hook.sh:1044`. |
    | Steward-owned infrastructure | Holder lease renewal failed: `metasystem/scripts/agents/supervision-hook.sh:1072`. |
    | Steward-owned infrastructure | Watchdog read failed: `metasystem/scripts/agents/supervision-hook.sh:1082`. |
    | Steward-owned infrastructure | Watchdog evidence preparation failed: `metasystem/scripts/agents/supervision-hook.sh:1087`. |
    | Steward-owned infrastructure | Hook evidence maintenance failed: `metasystem/scripts/agents/supervision-hook.sh:1098`. |
    | Steward-owned infrastructure | Turn verdict unavailable: `metasystem/scripts/agents/supervision-hook.sh:1197`. This includes malformed fields at line 1126, engine uncertainty at line 1133, and command failure at line 1138. |
    | Steward-owned infrastructure | State-root resolution: `metasystem/internal/goal/turnverdict.go:229`. |
    | Steward-owned infrastructure | Checkout fence read: `metasystem/internal/goal/turnverdict.go:241`. |
    | Steward-owned infrastructure | Closed-fence description: `metasystem/internal/goal/turnverdict.go:244`. |
    | Steward-owned infrastructure | Closed-fence command: `metasystem/internal/goal/turnverdict.go:245`. |
    | Steward-owned infrastructure | Session-stop marker read: `metasystem/internal/goal/sessionstop.go:435`. |
    | Steward-owned infrastructure | Session-stop marker decode: `metasystem/internal/goal/sessionstop.go:438`. |
    | Steward-owned infrastructure | Session-stop inspection registry read: `metasystem/internal/goal/sessionstop.go:442`. |
    | Steward-owned infrastructure | Session-stop holder lease proof: `metasystem/internal/goal/sessionstop.go:452`. |
    | Steward-owned infrastructure | Verdict-state read: `metasystem/internal/goal/turnverdict.go:270`. |
    | Steward-owned infrastructure | Brain status write: `metasystem/internal/goal/turnverdict.go:291`. |
    | Steward-owned infrastructure | Verdict-state write: `metasystem/internal/goal/turnverdict.go:296`. |
    | Steward-owned infrastructure | Authorization revalidation during consume: `metasystem/internal/goal/sessionstop.go:485`. |
    | Steward-owned infrastructure | Registry reread during consume: `metasystem/internal/goal/sessionstop.go:489`. |
    | Steward-owned infrastructure | Consumed-authorization registry write: `metasystem/internal/goal/sessionstop.go:500`. |
    | Steward-owned infrastructure | Authorization changed or was spent before consume completed: `metasystem/internal/goal/turnverdict.go:304`; the second spent check is `metasystem/internal/goal/sessionstop.go:493`. |
    | Steward-owned infrastructure | Goal-lock directory creation: `metasystem/internal/goal/goalverbs.go:106`. |
    | Steward-owned infrastructure | Goal-lock open: `metasystem/internal/goal/goalverbs.go:109`. |
    | Steward-owned infrastructure | Goal-lock acquisition deadline: `metasystem/internal/goal/goalverbs.go:119`. |
    | Seat-actionable | Owned work has no live waiter: `metasystem/internal/goal/turnverdict.go:675`. |
    | Seat-actionable | An unblocked plan line has not been surfaced: `metasystem/internal/goal/turnverdict.go:872`. |
    | Seat-actionable | The current goal revision names the next step: `metasystem/internal/goal/turnverdict.go:907`. |
    | Seat-actionable | The queue changed while a goal remained held: `metasystem/internal/goal/turnverdict.go:916`. |
    | Seat-actionable | The goal-free declaration predates new work: `metasystem/internal/goal/turnverdict.go:948`. |
    | Idle-with-backlog | Fresh ledger unavailable, counted under its existing sentinel: `metasystem/internal/goal/turnverdict.go:428`. |
    | Idle-with-backlog | Approved, claimable backlog without a live delegate: `metasystem/internal/goal/turnverdict.go:465`. |
    
    The invalid-worker route includes incomplete delegate hints at `metasystem/scripts/agents/supervision-hook.sh:384`, failed ancestry authentication at line 394, registry read failure at line 407, and unregistered runtime at line 411.
    It also includes unreadable payload at `metasystem/scripts/agents/supervision-hook.sh:421`, environment lookup failure at line 440, repository resolution failure at line 443, installation mismatch at line 454, and unreadable custody at line 494.
    Other unexpected shell exits enter this same route or Claude's outer launcher block. Argument validation is therefore part of the Stop census.
    The engine producers above reach `failClosedTurnVerdict` directly or through the error return at `metasystem/internal/goal/turnverdict.go:337`; its generic block is at line 412.
    An invalid or expired authorization without a read error grants no exemption; it is not a separate refusal. Successful-consume cleanup failure at `metasystem/internal/goal/sessionstop.go:503` and full-display write failure at `metasystem/internal/goal/turnverdict.go:324` are already notices.
    
    Twenty infrastructure triggers use the first-error slot at `metasystem/scripts/agents/supervision-hook.sh:481` and reach the external refusal at line 943.
    The deadline uses the same recorder at `metasystem/scripts/agents/supervision-hook.sh:334`.
    `metasystem/internal/report/stopblock.go:121` hashes cause text alone; line 137 writes before returning the first block at line 141.
    It remembers a cause for the session, without the condition, deadline, turn generation, or arming result.
    The raw refusals and unavailable-verdict route bypass that record.
    The verdict renderer at `metasystem/scripts/agents/supervision-hook.sh:1178` serves both seat and idle branches.
    Its fallbacks at `metasystem/scripts/agents/supervision-hook.sh:954`, line 981, and line 1016 preserve an independent verdict block; they add no new cause.
    Those fallbacks lose the bounded-idle renderer choice. The generic renderer appends unrelated plan advice at `metasystem/internal/report/stopblock.go:26`.
    Hook completion follows emission at `metasystem/scripts/agents/supervision-hook.sh:921`; it cannot prove that every refusal and its arming result were saved first.
    
    ## 2. The mechanism
    
    Put the classification and stop decision in `metasystem/internal/goal/turnverdict.go`.
    A typed condition carries its code, class, subject, observed state, detail, remedy owner, and clearing command.
    Use explicit codes from the producing owner. Never classify rendered strings, exit status alone, or the runtime name.
    Collect all failures; replace the first-error slot. An unknown collection failure belongs to infrastructure.
    Migrate each of the 18 engine producers in the census at its source, with a distinct code for the operation that failed.
    State-root, fence read, fence description, fence command, verdict-state read, status write and verdict-state write return typed infrastructure conditions.
    Session-stop inspection and consume retain their separate marker, registry, lease, revalidation and write codes. Losing authorization during consume grants no exemption.
    The lock owner distinguishes directory creation, open and acquisition failures. The outer error route preserves that code instead of calling `failClosedTurnVerdict`.
    Remove the generic blocking uncertainty result at `metasystem/internal/goal/turnverdict.go:412`; unexpected errors become infrastructure notices. Keep the fresh-ledger sentinel as the separate idle branch.
    The command composition in `metasystem/cmd/metasystem/goal.go` gathers these inputs and calls the decision owner.
    `metasystem/internal/report/stopblock.go` renders the result and maintains infrastructure reporting history. It does not invent a blocking reason.
    
    A seat-actionable refusal needs a positively observed condition and one complete command the seat may run.
    The command must change that condition or start its required work. A status read, a retry, and advice to ask a human do not qualify.
    For unwatched runs, use the command renderer at `metasystem/cmd/metasystem/run.go:80`. Add a job-watch renderer in that file for the existing verb parsed at line 433; line 240 is also a parser, not a renderer.
    Both renderers include the exact root and identifier, with shell-safe argument quoting. The job form is `bin/metasystem job watch --root <root> --job <job>`.
    For stale goal-free state, supply the current digest and the complete declaration command, after checking its existing admission rules in `metasystem/internal/goal/verbs.go:2281`.
    For plan, current-goal, and changed-queue prods, use a complete clearing command already named by the selected work; do not turn arbitrary next-step prose into shell.
    When such a command is absent or requires a human, show the work as a notice. Do not issue a text-only prod.
    This eligibility check applies to the five seat-actionable branches. Their existing seen-state limits still apply when they qualify.
    The idle path remains mandatory and outside that eligibility check. Its refusal names approved backlog with no job, the selected ready goal, its next-step text and one claim command.
    Render `bin/metasystem goal claim --root . --id <ready goal> --lineage <seat lineage>` with the real goal and authenticated lineage, quoted as arguments, for execution from the checkout's metasystem directory.
    Add this renderer beside the idle decision at `metasystem/internal/goal/turnverdict.go:490`; the existing claim verb is `metasystem/cmd/metasystem/goalsync_mutations.go:1934`.
    For this path, DONE clause 1 means the first lawful act, not a whole workflow or an immediate exemption. Authoring the brief and dispatching remain the seat's work under that quoted next step.
    Claiming alone does not manufacture a job or clear the idle rule. The counter and three-refusals handoff remain mandatory; the steward launcher is never printed as the seat's command.
    
    Infrastructure produces an allowed stop with degraded health, including on its first occurrence, and records a steward incident through the delivery path below.
    There is no initial infrastructure refusal to spend. Thus the once-per-deadline refusal ceiling is zero for this class.
    Do not convert that allowance into ALL CLEAR, a valid human authorization, a lease, a claim, or permission to dispatch.
    A separately established seat or idle refusal still wins. Its reason names only that condition; infrastructure stays in the notice channel.
    Keep invalid human-stop markers ineligible and preserve consumption checks in `metasystem/internal/goal/sessionstop.go:428` and line 484.
    If inspection or consume fails, evaluate ordinary work without that exemption when its inputs remain available; do not bypass idle evaluation by returning early. Never claim an uncommitted consumption.
    Fresh-ledger uncertainty stays in the existing bounded idle branch; it is not the narrator or hook-evidence failure class.
    
    Extend the existing per-session verdict state in `metasystem/internal/goal/turnverdict.go:150` with the actual stop deadline coordinates and prepared decision evidence.
    Persist the decision, classification, clearing command, arming result, and generation in the same commit that spends its seen marker or idle count.
    The hook generation is also carried into the checkout's existing component evidence through `metasystem/internal/steward/component_evidence.go:188`.
    Component evidence remains observation evidence; failure to read or mirror it cannot reverse a durable work decision.
    Every emitted refusal names a committed decision: in the verdict state, or, when that write failed, in the hook log's appended line (the second channel below), which is written before emission. No raw block or post-emission-only evidence remains.
    If the authoritative decision cannot be persisted, the observed work decision still stands: an observed idle-with-backlog condition still refuses the stop, uncounted, with its reason and command, and the refusal says the count could not be spent. The refusal is written to the hook log as a decision line first (see the log protocol below); if even that append fails, the refusal is still emitted and says it is unrecorded. Infrastructure alone never manufactures a refusal from a lost record.
    Never claim a counted idle refusal or a successful authorization when the record did not commit. An auxiliary incident-record failure cannot erase an already committed idle decision. The two records have different duties.
    
    Extend the external record owned by `metasystem/internal/report/stopblock.go:155` to version 2 for infrastructure incidents and delivery state.
    Resolve it from the same canonical checkout state root as the verdict; remove the hook's separate repository-root path calculation.
    At the outer gate's entry, allocate the hook turn generation and record that invocation's start and actual deadline end, before optional collection.
    The current deadline starts at `metasystem/scripts/agents/supervision-hook.sh:38` and is passed to the worker at line 61; carry its end explicitly through the new Go boundary.
    Key the episode by canonical checkout, holder session and lease epoch, hook turn generation, deadline end and condition fingerprint. A first-observation time is evidence only; it never starts a rolling window.
    The fingerprint contains cause code, component or resource, stable error kind and relevant engine or fence identity. Exclude elapsed time, incidental wording and observation timestamps from this inner fingerprint only.
    Record attempt sequence, first and last observation, count, full arming result and delivery state. All retries within that actual deadline carry the same generation and end, including timeout completion.
    Concurrent updates use the bounded record lock. Repeated observations reuse one incident identity and increase its count; they neither refuse nor enqueue a new identity.
    A different generation or deadline end creates a new episode even within 60 seconds of the previous report. Recovery marks the old condition resolved without merging future deadlines.
    If deadline coordinates cannot be established, report infrastructure uncertainty without borrowing a previous episode or suppressing evidence as already seen.
    Read version-1 entries as history, never as evidence that a new decision or deadline was already processed.
    Retain undelivered incidents until the steward acknowledges them; prune delivered history using the existing session retention horizon.
    
    Preserve `up.Result` and use its renderer in `metasystem/internal/up/up.go:80` for both the command and the stop report.
    Carry every failed component's outcome, detail and remedy unchanged, followed by the exact aggregate. ENROLLMENT_DRIFT means the engine differs from its enrollment; retain its agent-free-terminal restart remedy.
    An interrupted arming call records its partial component results and an explicit incomplete aggregate from the arming owner. It never fabricates an armed result.
    The persisted record holds the full text. Bound unrelated display text first; keep the failed component's outcome, detail and remedy exact and together. A decision reference supplements those fields rather than replacing them.
    
    Move Stop collection and deadline composition out of the shell into the shared command path in `metasystem/cmd/metasystem/goal.go`.
    Read the work decision before optional health, narrator, or arming waits. Prepare its durable response with arming marked incomplete, then attach the completed arming result before final emission.
    Use the existing 60-second outer deadline and reserve its final three seconds for completion. All dependency waits and lock waits share that deadline.
    A deadline relays an already committed work refusal if one exists; otherwise it allows with the infrastructure incident. It never manufactures uncertainty as a new block.
    Compare generation, deadline end and attempt sequence when completing the prepared decision. Worker and timeout completion are mutually exclusive; a late worker cannot overwrite the result.
    Replaying a completed decision within that deadline never spends another seen marker or idle refusal. A genuine later stop remains a new idle-counter opportunity under the existing rules.
    The shell keeps provider payload transport and owned-child cleanup. Without a readable committed work decision, missing engine, malformed output and deadline fallback emit the fixed degraded allowance.
    Replace Claude's raw launcher block at `metasystem/scripts/enforcement/claude-code-hooks.json:25` with a nonblocking degraded notice on bootstrap failure, including invalid arguments.
    Buffer and validate the gate response before relaying it; never append a second JSON response to partial output. Relay an available committed work refusal; a transport exit code alone cannot create one.
    If no engine can record the failure, the fallback states delivery unconfirmed. The shell performs no policy or JSON-state repair.
    
    Add `host stop-gate` in `metasystem/cmd/metasystem/host_verbs.go`, registered in `metasystem/cmd/metasystem/main.go`. It calls the same Go collection and decision service as `report turn-verdict`.
    Its request carries checkout, authenticated session and holder, provider turn identity, repeat evidence, generation, attempt and deadline end. Its response carries a decision identity when committed, block or allow, display, health and delivery state.
    Native Stop hooks call this boundary and translate its response.
    A boundary never counts the same native Stop twice: the decision record is keyed to the turn generation and deadline end it was made for.
    `metasystem/scripts/agents/adapters/runtime-common.sh:3` owns delegate plumbing only; the fake adapter does not source it (`metasystem/scripts/agents/adapters/fake.sh:134`). Neither fact supplies a coordinator gate.
    A Codex delegate round has no native Stop event, so its adapter (`metasystem/scripts/agents/adapters/codex.sh`) calls the gate at the turn boundary, after the provider returned and before the job is made terminal (`metasystem/scripts/agents/adapters/runtime-common.sh` terminalizes only from running). The gate call names the round's own job, and the decision excludes that job from the live-work exemption: a round asking whether it may stop is not its own reason to stop. On an allowance the adapter terminalizes as today. On a committed refusal the adapter does not terminalize; it re-invokes the provider once, in the same job and round, with the refusal's reason and command as the message, under the round's remaining cap (the dispatcher owns that transition and records it on the round as a gate continuation; no new job, no new budget, no hidden resume). A second committed refusal in the same round terminalizes the job with the refusal recorded as the round's gap, and the seat sees it as a normal return. This is the dispatcher lifecycle change member stop-hosts-enforce-the-common-gate builds and its Codex fixture reproduces.
    Hosts without native Stop support are a lifecycle of their own (a managed seat: launch, permission envelope, ordinary reply, gate at every turn boundary, activation owner) and this goal does not build it; member stop-hosts-enforce-the-common-gate proves the common gate at the two boundaries production runtimes have today, the Claude Code native Stop hook of a seat and the adapter turn boundary of a Codex delegate round (`metasystem/scripts/agents/adapters/codex.sh`), and the managed-seat lifecycle (`metasystem/scripts/agents/hosts/host-common.sh`, `host finish` in `metasystem/cmd/metasystem/host_verbs.go`) is named as the follow-on goal in section 7. An unmanaged provider is not certified by an instruction to read `goal next`.
    Replace the informational fallback at `metasystem/docs/design/turn-verdict-delivery-contract.md:26` with this enforcing contract for every registered host, including future runtimes. Runtime adapters own provider flags and delivery only, as `metasystem/docs/orchestration.md` requires.
    
    Add a `DrainStopIncidents` reader in `metasystem/internal/steward/tick.go`, before narrator work. The current loop at line 237 reads only ledger-attention events; it does not consume stop incidents.
    The reader loads undelivered version-2 incidents through the record owner and calls `QueueNotification` at `metasystem/internal/steward/intervene.go:317` with the incident identifier as its nonce.
    Report seat, checkout, generation, deadline end, first and last observation, count, cause, component outcome, detail, remedy and aggregate. The stop path never waits for channel delivery or wakes the failed seat to retry it.
    `DeliverPending` in `metasystem/internal/steward/notify.go:112` reads the durable queue; the runner invokes it at `metasystem/internal/steward/runner.go:203` even after a failed tick.
    Persist the incident's delivery acknowledgement before deleting its queued notification. Retry the same identity after failure; a crash around channel delivery may redeliver it, but cannot create another stop refusal.
    The second, independent channel is the hook log the hook already writes at `metasystem/scripts/agents/supervision-hook.sh:878`, with a line protocol this goal defines: every line carries the canonical checkout, the holder session and lease epoch, the hook turn generation, the deadline end, the line kind (decision or incident), and for a decision its class, reason, clearing command, count state and the arming aggregate; for an incident its cause code, component, count and the arming aggregate. The gate appends from the shell with the append's exit checked, before emission; a failed append is reported in the emitted text. The steward's drain reads the log's incident lines when the version-2 record is unreadable, and records each delivery as an acknowledgement line in the same log (kind acknowledged, the incident identity), so a delivered line is never queued again on a later tick even though the log line itself is permanent. The per-session verdict file under artifacts/agents/supervision/stop-verdicts is replaced at every verdict (`metasystem/internal/goal/turnverdict.go:322`) and is display, never record.
    Only when both channels fail does the gate say "delivery unconfirmed" in the provider notice and standard error; stopping stays allowed for infrastructure alone. That residual case is the honest limit of DONE clause 2, and the seven-day observation counts it.
    
    Use one fresh projected board for current goal, READY, INFLIGHT and idle selection in `metasystem/internal/goal/project.go`.
    READY means eligible for the authenticated machine and lineage pair, using the existing claim admission rules. It is not machine-only ownership.
    Retain accepted tip, successful fetch completion, projection time and owner pair as its freshness proof; recheck the local tip and lease epoch before committing the decision.
    The existing fresh projection is at `metasystem/internal/goal/project.go:312`; retire the separate offline Stop reads at `metasystem/internal/goal/turnverdict.go:994` and line 1055.
    For the activity join, include ready goals and the same actor's working claims as goal/revision pairs; exclude fenced claims.
    INFLIGHT requires a nonterminal job or governed attempt matching that pair and owner, with live recorded process birth identity. A live seat claim alone never qualifies.
    Read the job coordinates in `metasystem/internal/dispatch/jobrecord.go:64` and line 98, and attempt coordinates in `metasystem/internal/run/run.go:126` and line 189.
    Resolve a job's lineage through its recorded main identity and announcement; require its claim epoch to match the selected claim. Missing owner or revision evidence grants no exemption.
    Unrelated, old-revision, other-lineage, terminal, and unproven-live activity stays visible as activity but cannot exempt this frontier. Keep generic Busy display separate from this exemption.
    The idle digest is not amended by this goal: `metasystem/internal/goal/turnverdict.go:509` keeps hashing every nonterminal job, so another seat's job churn resets the counter as it does today. DONE clause 4 says the idle path is untouched, and the absorbed INFLIGHT clause governs the WORK IN FLIGHT exemption, not the digest. TestIdleDigestKeepsEveryNonterminalJob pins that nothing moved; narrowing the digest is a proposal for memory/backlog-notes.md with the churn count as its evidence.
    Observe goal fences before the Busy and WORK IN FLIGHT exits. Persist a seen fingerprint per session from goal, stop identifier, fence revision, epoch, fence generation and reason; hook generation is not a fence change.
    Emit FENCED once for each unchanged fence, including beside live work. Clearing and recreating a fence is a new observation. Reporting never resumes it or creates current work from it.
    
    ## 3. What does not change
    
    `metasystem/records/goals/idle-with-backlog-alarm.md:7` remains the idle refusal and handoff contract, digest included.
    Approved backlog with no joined job still blocks the first and second stops. The third unchanged refusal prepares steward continuation and records the incident.
    The steward claims if needed on its tick and rechecks dispatch guards. Its human alarm is used when continuation cannot be prepared or carried out.
    The unreadable-ledger sentinel and preservation of an independent block remain intact; which work changes reset the counter is not changed by this goal.
    Infrastructure observations never increment, reset, or consume the idle counter. The generation mirror never serves as its repeat counter.
    Human session-stop authorization remains holder-bound, expiring and single-use. No error path creates, renews, or consumes permission by inference.
    StopFence authority, approval, budgets, lease custody, foreign-runtime skips, proven-delegate skips, and steward continuation admission remain unchanged.
    
    ## 4. Risks
    
    The main risk is treating missing telemetry as proof that backlog is empty. Keep the fresh idle read and its bounded failure path separate from auxiliary reads.
    The opposite risk is letting a telemetry failure hide a real refusal. Persist the work decision first and preserve it through arming, rendering and incident-delivery failures.
    Text-only goal prods become notices unless they already name a lawful clearing command. This is a deliberate consequence of the actionable-refusal requirement; the mandatory idle rule still applies.
    A global job count or a machine-only claim read can silently grant the wrong seat an exemption. Test both owner coordinates and revision joins.
    Coalescing by elapsed time would suppress a new deadline's report. Freeze the actual generation and deadline end at entry; retries reuse them, while later stops have distinct coordinates.
    The explicit delivery limit leaves some infrastructure failures without a durable steward report. Such stops must say delivery unconfirmed; they must never erase authorization records or claim healthy supervision.
    
    ## 5. Fixtures
    
    These are implementation obligations, not tests claimed as run by this design round. New test logic is Go; shell beds only launch it.
    
    | Test or bed leg | Setup and required observation |
    | --- | --- |
    | TestStopConditionClassification in `metasystem/internal/goal/turnverdict_test.go` | Cover all 52 census rows, including the 18 engine producers. All 45 infrastructure routes preserve their distinct codes and never independently block; the five action branches require lawful commands; both idle branches retain their decisions. |
    | TestStopClearingCommand in `metasystem/internal/goal/turnverdict_test.go` | For each actionable branch, run its emitted command in an isolated fixture and observe the condition clear or its work start. Missing commands, human-only remedies and text-only next steps become notices. Keep idle decisions mandatory. |
    | TestIdleClaimCommandIsFirstAct in `metasystem/cmd/metasystem/goal_test.go` | An approved ready goal has no brief or job. Assert the real goal, lineage, quoted next step and executable claim command. Run it as the seat and observe its lawful claim, no fabricated dispatch, no human-stop command substitution and no exemption merely for claiming. |
    | TestInfrastructureDeadlineIdentity in `metasystem/internal/report/stopblock_test.go` | Use a fake clock for identical retries, concurrent writers, changed condition, recovery and recurrence. Assert one identity per condition and actual deadline. Different generations or deadline ends report separately even within 60 seconds; retries cannot extend the deadline. Cover missing coordinates and version-1 history import. |
    | TestStopDecisionPersistsBeforeEmission in `metasystem/cmd/metasystem/goal_test.go` | Capture output while faulting arming, incident writes, component reads and payload completion. Every block has prior committed decision evidence, full available up result and checkout generation. A lost auxiliary record preserves a real idle block; a lost authoritative commit cannot claim one. |
    | TestStopDeadlineCompletionIsSingleUse in `metasystem/internal/goal/turnverdict_test.go` | Race worker completion with timeout and replay their shared generation, deadline end and attempt. One committed decision wins; late completion cannot overwrite it or spend another idle count. A new stop remains countable. |
    | TestSessionStopInfrastructurePreservesAuthority in `metasystem/internal/goal/turnverdict_idle_test.go` | Fault every inspection and consume producer, then replay marker bytes. No error creates or consumes permission by inference. Available ordinary work is still evaluated; a committed consumed marker cannot be reused after cleanup failure. |
    | TestArmingDetailSurvivesStop in `metasystem/cmd/metasystem/up_test.go` | Replay ENROLLMENT_DRIFT and a component failure. Compare persisted and rendered component outcome, detail, remedy and aggregate byte-for-byte with up's own output. FakeReplayArming separately proves the stop decision after classification lands. |
    | TestStopIncidentDrainBeforeNarrator in `metasystem/internal/steward/tick_test.go` | Persist an incident, fail narrator work, then tick and deliver. Assert the named reader still queues the incident, delivery is acknowledged, and later ticks do not create a new identity. |
    | TestStopIncidentDeliveryUnconfirmed in `metasystem/internal/steward/notify_test.go` | Fault incident reads, writes, queue writes and channel delivery separately. Storage loss or absent bootstrap engine yields an explicit unconfirmed report, never an infrastructure block. Queued delivery failure retains its identity for retry; no hook-log delivery is claimed. |
    | TestStopFrontierOwnerRevisionAndFreshness in `metasystem/internal/goal/turnverdict_world_test.go` | Two machines, two lineages on one machine, current and old goal revisions, jobs and governed attempts. Only matching live work exempts the actor's frontier. Failed fetch uses the existing sentinel; a changed tip or lease rejects the stale projection. |
    | TestIdleDigestKeepsEveryNonterminalJob in `metasystem/internal/goal/turnverdict_idle_test.go` | Another seat's job appears and ends between two idle stops; the digest changes and the counter resets, exactly as today. Pins that this goal does not amend the idle digest. |
    | TestIdleRefusalSurvivesALostCounter in `metasystem/internal/goal/turnverdict_idle_test.go` | Fault the verdict-state write while approved backlog has no job. The stop is refused with the idle reason and the claim command, the refusal says the count could not be spent, and no count is recorded. |
    | TestStopIncidentDrainFromHookLog in `metasystem/internal/steward/tick_test.go` | Make the version-2 record unreadable after the hook log carried two incident lines. The drain reads the log's unacknowledged lines, queues one notification per incident identity and acknowledges them; with both channels unreadable the provider notice says delivery unconfirmed. A delivered incident is acknowledged by a line in the log and is not queued again on the next tick. |
    | TestDailyRefusalCountReadsTheLogStream in `metasystem/internal/report/stopblock_test.go` | A synthetic week of decision lines on two seats counts refusals per seat and day; replacing the per-session verdict file changes no count. |
    | TestFencedClaimSurfacedOnceBesideFlight in `metasystem/internal/goal/turnverdict_stopfence_test.go` | Hold a fenced goal beside a working goal, with and without joined live work. Observe FENCED once despite new hook generations, a new line after fence change, and no continuation of the fenced goal. |
    | FakeReplayNarrator, FakeReplayDeadline, FakeReplayArming in `metasystem/cmd/metasystem/runtime_conformance_test.go` | Add an explicit stop-replay action to `metasystem/scripts/agents/adapters/fake.sh` that drives the common gate for a bed-owned seat, not the fake delegate's custody exemption. Replay the three recorded causes, including the outer deadline. Same-deadline repeats cause no forced turn or new notification identity; records retain the cause and arming evidence. |
    | FakeReplayIdleWithInfrastructure in `metasystem/cmd/metasystem/runtime_conformance_test.go` | An approved goal, no job, and each infrastructure failure in turn. First and second stops still block for backlog. Third stages continuation, records the incident and allows absent another block. A steward tick starts the guarded continuation; a failed dispatch raises the existing alarm. |
    | FakeReplayUnreadableHookState in `metasystem/cmd/metasystem/runtime_conformance_test.go` | Break hook evidence and incident state independently, then together. Assert degraded allowance without work, retained independent idle refusal when its decision is durable, and truthful unconfirmed-delivery text. |
    | TestLauncherFailureNeverRawBlocks in `metasystem/cmd/metasystem/runtime_conformance_test.go` | Invoke the installed Claude launcher with invalid runtime and event arguments, missing engine, partial output and nonzero exit. Assert one valid response, no unpersisted raw block, preserved readable committed work decisions, and truthful degraded or unconfirmed notices. |
    | TestEveryRuntimeUsesStopDecision in `metasystem/cmd/metasystem/runtime_conformance_test.go` | Enumerate registered runtimes. The native Stop hook boundary and the delegate adapter boundary both call the one gate and honor a committed refusal; infrastructure never requests another turn; a runtime with neither boundary is reported unproven, not certified. |
    | RealClaudeNativeStopReplay and RealCodexAdapterStopReplay in `metasystem/cmd/metasystem/runtime_conformance_test.go` | The orchestrator launches actual Claude Code with its installed Stop hook, and an actual Codex delegate round through its adapter, in isolated enrolled beds. Replay narrator read failure, deadline expiry and arming failure, then approved backlog with no job. Observe the provider's real behavior: no infrastructure reprompt; the idle refusal, then the third-call handoff. Retain transcripts, decision records and the steward receipt. |
    
    IdleHandoffRegression retains TestIdleBacklogBlocksTwiceThenDefersClaimAndPreparesStewardContinuation, TestIdleEscalationPreservesAnIndependentOpenWorkBlock, and TestThreeUnreadableLedgerStopsRecordIncidentRaiseAlarmAndEnd in `metasystem/internal/goal/turnverdict_idle_test.go`.
    Retain that file's single-use permission assertions, including consume failure; update superseded infrastructure-block expectations without weakening authorization checks.
    All process beds own their registry, checkout and recorded child identities. Use bounded fixture waits and clean up only those children.
    The direct shell calls at `metasystem/scripts/agents/supervision-hook-fixtures.sh:672`, line 909 and line 1162 remain plumbing tests. They cannot stand in for either live production-runtime leg; the fake is auxiliary.
    The Claude and Codex legs supply the two-production-runtime proof. The seven-day observation is member stop-refusals-under-ten-a-day: its data source is the retained stream this goal creates (the hook log's decision lines, one per stop, appended and never replaced, plus the version-2 incident record; the per-session stop-verdict file is replaced at every verdict, `metasystem/internal/goal/turnverdict.go:322`, and cannot count a day) and the version-2 incident record, its acceptance is Wido's on the count the member records on this page.
    
    ## 6. Landing
    
    Implementation changes the owners and test files named above, plus `metasystem/cmd/metasystem/report.go` for rendering plumbing and `metasystem/cmd/metasystem/steward_verbs.go` for generation-bound completion.
    `metasystem/scripts/agents/supervision-hook.sh` becomes the native transport, and the Claude launcher loses its raw block. The managed-seat host scripts are untouched by this goal.
    Wire the gate call into `metasystem/scripts/agents/adapters/codex.sh` at the round's turn boundary (the claude adapter's rounds keep their native hook). Use `metasystem/scripts/agents/hosts/fake.sh` and the fake adapter for auxiliary replay only.
    Update `metasystem/docs/design/turn-verdict-delivery-contract.md` and the affected groups in `metasystem/testing.json`; retire superseded first-infrastructure-block expectations in `metasystem/scripts/agents/supervision-hook-fixtures.sh`.
    Land the members below in order. Add record readers before switching writers and consumers; keep each member usable on its own. Do not combine the program into one landing or claim universal enforcement before the final member.
    Every seat must receive each landed engine or runtime change and re-arm its enrolled supervision set so running components use the new generation. A documentation-only landing needs neither a rebuild nor re-arming.
    Use `metasystem/scripts/agents/go-build.sh` for the build and the existing up/re-arm rules. Enrollment drift requiring an agent-free terminal remains the steward's task.
    Check installation and actual turn-boundary observation for each seat. Installed hook files alone are not proof that a provider loaded them.
    This round revises only this page. Document checks cover its boundary, citations and length; implementation and runtime proof belong to the members and the orchestrator's committed shared testing contract.
    
    ## 7. Slices
    
    Keep stop-hook-never-forces-an-empty-turn as the context umbrella. The 2026-09-10 rule requires one mechanism per member, each with its own DONE; this page names them, and the seat opens them as blockers of the umbrella (R-93-m1e) in this order, which is the landing order. Each lands green on its own: no member's DONE needs a later member.
    
    | Member goal | DONE, umbrella clauses and fixtures |
    | --- | --- |
    | stop-infrastructure-allows-the-seat-to-stop | DONE: every censused producer uses the typed classification, infrastructure alone never blocks a stop (including the three recorded causes), the launcher's raw block on a nonzero hook exit is replaced by the degraded notice, and an observed idle-with-backlog condition still refuses even when its count cannot be spent. Carries clauses 1 (for infrastructure), 2 and 3, preserves clause 4. Fixtures: TestStopConditionClassification, TestSessionStopInfrastructurePreservesAuthority, TestLauncherFailureNeverRawBlocks, TestIdleRefusalSurvivesALostCounter, IdleHandoffRegression, and the fake replays in their first form: each recorded cause allows the stop and writes its hook-log line; the once-per-deadline and notification-identity assertions of those replays join members two and three when they land. Lands first: it is the member that answers the three recorded causes (184 narrator reads, 117 deadlines, about 95 arming failures). |
    | stop-decisions-record-deadline-evidence | DONE: each emitted refusal has a prior durable decision with its full up result and the checkout generation, and each incident episode is keyed to the hook's turn generation and deadline end. Carries clause 3's deadline scope and the absorbed persistence and arming-detail clauses. Fixtures: TestInfrastructureDeadlineIdentity, TestStopDecisionPersistsBeforeEmission, TestStopDeadlineCompletionIsSingleUse, TestArmingDetailSurvivesStop. |
    | stop-incidents-reach-the-steward | DONE: the steward drains retained stop incidents before narrator work from the version-2 record or, when it is unreadable, from the hook log's unacknowledged lines, acknowledges each delivery as a line in that log so no incident is queued twice, and only a failure of both channels is reported as unconfirmed. Carries clause 2's reporting duty. Fixtures: TestStopIncidentDrainBeforeNarrator, TestStopIncidentDrainFromHookLog, TestStopIncidentDeliveryUnconfirmed, FakeReplayUnreadableHookState. |
    | stop-frontier-joins-owner-and-revision | DONE: one fresh board scopes ready work and the WORK IN FLIGHT exemption to the machine, lineage, goal and revision, with a freshness proof, and the ready goal's next-step text is on that board; the idle digest is untouched. Carries the absorbed READY and INFLIGHT clauses. Fixtures: TestStopFrontierOwnerRevisionAndFreshness, TestIdleDigestKeepsEveryNonterminalJob, IdleHandoffRegression. |
    | stop-refusals-name-seat-actions | DONE: an eligible seat refusal prints one executable clearing command, the mandatory idle refusal prints the real claim command and the ready goal's next step from the board, and a refusal with no lawful command becomes a notice. Carries clause 1 for the seat and idle paths, preserves clause 4. Fixtures: TestStopClearingCommand, TestIdleClaimCommandIsFirstAct, IdleHandoffRegression. |
    | stop-fences-surface-once | DONE: a held fenced goal prints FENCED once per unchanged fence, including beside live work. Carries the absorbed FENCED clause, preserves clause 4. Fixture: TestFencedClaimSurfacedOnceBesideFlight. |
    | stop-hosts-enforce-the-common-gate | DONE: the Claude native Stop hook of a seat and the adapter turn boundary of a Codex delegate round both call the one Go gate and enforce its committed decision (the Codex round re-invokes its provider once in the same job under its remaining cap on a refusal, excluding its own job from the live-work exemption), proven live on both. Carries clauses 1 to 4 across the two production boundaries. Fixtures: TestEveryRuntimeUsesStopDecision, the five fake replay legs, RealClaudeNativeStopReplay, RealCodexAdapterStopReplay. The managed-seat lifecycle for hosts without a native Stop is the follow-on goal managed-seats-run-under-the-common-gate, opened when a runtime needs it. |
    | stop-refusals-under-ten-a-day | DONE: seven days of hook-log decision lines (one per stop, per seat, never replaced) and version-2 incident records on every seat show fewer than ten refusals per seat per day, and the count with its residual unconfirmed deliveries is recorded on this page for Wido's acceptance. Carries the umbrella's operational clause. Fixture: TestDailyRefusalCountReadsTheLogStream (a synthetic week of lines on two seats counts per seat and day, and a replaced verdict file changes nothing). |
    
    The umbrella closes when its members are done and Wido accepts the seven-day count. No fixture or design return substitutes for those receipts.

Critique findings being answered:

1. none

Cited code excerpts:

1. `internal/goal/turnverdict.go:1174-1242`

   ```text
   func (s *Store) enforceIdleBacklogWithWaits(verdict *Verdict, work *ClaimableBudgetedWork, workErr error, session *sessionState, sessionID, mainID string, options TurnVerdictOptions, waits registeredWaits) {
   	blockedBeforeIdle := verdict.ShouldBlock
   	if workErr != nil {
   		verdict.Class = "idle-with-backlog"
   		verdict.IdleRefusal = true
   		verdict.CountSpent = true
   		if session.IdleBlockDigest != unreadableIdleBacklogDigest {
   			session.IdleBlockDigest = unreadableIdleBacklogDigest
   			if options.StopHookActive && session.IdleBlocks > 0 {
   				session.IdleBlocks++
   			} else {
   				session.IdleBlocks = 1
   			}
   		} else if options.StopHookActive {
   			session.IdleBlocks++
   		}
   		if session.IdleBlocks >= 3 {
   			s.escalateIdleBacklog(verdict, session, sessionID, mainID, "", false, blockedBeforeIdle, options,
   				"the fresh canonical ledger read failed, so no continuation goal could be identified: "+workErr.Error())
   			return
   		}
   		verdict.ShouldBlock = true
   		source := "uncertainty"
   		verdict.BlockSource = &source
   		detail := "IDLE WITH BACKLOG cannot be ruled out: the fresh canonical ledger read failed: " + workErr.Error()
   		detail += fmt.Sprintf("; refusal %d of 3 for the unreadable ledger; stop_hook_active=%t; at 3 the refusal is recorded, the human idle alarm is raised, and the turn ends", session.IdleBlocks, options.StopHookActive)
   		verdict.Diagnostics = append(verdict.Diagnostics, detail)
   		verdict.Display = strings.TrimSpace(verdict.Display + "\n" + detail)
   		return
   	}
   	if work == nil {
   		return
   	}
   	if len(work.Refused) > 0 {
   		detail := fmt.Sprintf("CLAIM WOULD REFUSE: %s: %s", work.Refused[0].GoalID, work.Refused[0].Cause)
   		if remaining := len(work.Refused) - 1; remaining > 0 {
   			detail += fmt.Sprintf(" (and %d more)", remaining)
   		}
   		verdict.Diagnostics = append(verdict.Diagnostics, detail)
   		verdict.Display = strings.TrimSpace(verdict.Display + "\n" + detail)
   	}
   	digest := idleBacklogDigest(*work)
   	if len(work.Claimable) == 0 || work.HasDelegateJobInFlight() || waits.hasWorkInFlight() {
   		session.IdleBlockDigest = digest
   		session.IdleBlocks = 0
   		return
   	}
   	verdict.IdleRefusal = true
   	verdict.Class = "idle-with-backlog"
   	verdict.CountSpent = true
   	if digest == session.IdleBlockDigest {
   		session.IdleBlocks++
   	} else {
   		session.IdleBlockDigest = digest
   		session.IdleBlocks = 1
   	}
   	if session.IdleBlocks >= 3 {
   		goalID, claimNeeded, unavailable := idleBacklogContinuation(*work)
   		s.escalateIdleBacklog(verdict, session, sessionID, mainID, goalID, claimNeeded, blockedBeforeIdle, options, unavailable)
   		return
   	}
   	verdict.ShouldBlock = true
   	source := "idle-backlog"
   	verdict.BlockSource = &source
   	countText := fmt.Sprintf("refusal %d of 3 for this unchanged backlog; at 3 request steward continuation and allow Stop unless another branch blocks", session.IdleBlocks)
   	verdict.Display = strings.TrimSpace(verdict.Display + "\n" + fmt.Sprintf(
   		"IDLE WITH BACKLOG: %d claimable goals await a live claim or job: %s; %s; stop_hook_active=%t; an attended human may run `metasystem session stop --by <name>`",
   		len(work.Claimable), idleBacklogNames(*work), countText, options.StopHookActive))
   }
   ```

2. `internal/goal/turnverdict.go:1244-1283`

   ```text
   func idleBacklogContinuation(work ClaimableBudgetedWork) (string, bool, string) {
   	if len(work.Claimed) > 0 {
   		id := work.Claimed[0]
   		if NextStepNamesAPendingHumanWord(work.GoalFacts[id].NextStep) {
   			return "", false, "this machine's held goal waits on a human word"
   		}
   		return id, false, ""
   	}
   	for _, id := range work.Claimable {
   		if !NextStepNamesAPendingHumanWord(work.GoalFacts[id].NextStep) {
   			return id, true, ""
   		}
   	}
   	return "", false, "every ready goal waits on a human word"
   }
   
   func idleBacklogNames(work ClaimableBudgetedWork) string {
   	limit := min(5, len(work.Claimable))
   	names := append([]string(nil), work.Claimable[:limit]...)
   	display := strings.Join(names, ", ")
   	if remaining := len(work.Claimable) - len(names); remaining > 0 {
   		display += fmt.Sprintf(" and %d more (metasystem goal list names them all)", remaining)
   	}
   	return display
   }
   
   func idleBacklogDigest(work ClaimableBudgetedWork) string {
   	claimable := append([]string(nil), work.Claimable...)
   	claimed := append([]string(nil), work.Claimed...)
   	jobs := append([]string(nil), work.NonTerminalJobs...)
   	sort.Strings(claimable)
   	sort.Strings(claimed)
   	sort.Strings(jobs)
   	parts := []string{
   		"claimable\n" + strings.Join(claimable, "\n"),
   		"claimed\n" + strings.Join(claimed, "\n"),
   		"non-terminal-jobs\n" + strings.Join(jobs, "\n"),
   	}
   	return sha256Hex([]byte(strings.Join(parts, "\n--\n")))
   }
   ```

3. `internal/goal/turnverdict.go:1285-1395`

   ```text
   func (s *Store) escalateIdleBacklog(verdict *Verdict, session *sessionState, sessionID, mainID, goalID string, claimNeeded, blockedBeforeIdle bool, options TurnVerdictOptions, unavailable string) {
   	escalationRepair := unavailable != ""
   	if unavailable == "" && s.ResolveIdleSeat != nil {
   		actor, epoch, err := s.ResolveIdleSeat()
   		if err != nil {
   			options.SeatActorProblem = err.Error()
   			escalationRepair = true
   		} else {
   			options.SeatActor = actor
   			options.SeatClaimEpoch = epoch
   		}
   	}
   	event := IdleEscalationEvent{
   		SessionID: sessionID, MainID: mainID, GoalID: goalID,
   		BacklogDigest: session.IdleBlockDigest, Refusal: session.IdleBlocks,
   		StopHookActive: options.StopHookActive, ClaimActor: options.SeatActor,
   		ClaimNeeded: claimNeeded, SeatClaimEpoch: options.SeatClaimEpoch,
   	}
   	if unavailable != "" {
   		event.ClaimDetail = unavailable
   	} else if options.SeatActorProblem != "" {
   		event.ClaimDetail = options.SeatActorProblem
   	} else if options.SeatActor.Machine == "" || options.SeatActor.Lineage == "" || options.SeatClaimEpoch < 1 {
   		event.ClaimDetail = "the announced seat actor or its positive checkout lease epoch was unavailable"
   	} else if claimNeeded {
   		event.ClaimDetail = "claim deferred to the steward tick"
   	} else {
   		event.ClaimDetail = "goal is already claimed by this machine; no steward claim is needed"
   	}
   
   	if unavailable == "" && goalID != "" && options.SeatActorProblem == "" &&
   		options.SeatActor.Machine != "" && options.SeatActor.Lineage != "" && options.SeatClaimEpoch > 0 {
   		if s.PrepareIdleContinuation == nil {
   			event.IntentDetail = "the steward continuation preparation seam is unavailable"
   			escalationRepair = true
   		} else if nonce, err := s.PrepareIdleContinuation(event); err != nil {
   			event.IntentDetail = err.Error()
   			escalationRepair = true
   		} else {
   			event.IntentID = nonce
   			event.IntentPrepared = true
   			event.IntentDetail = "prepared steward continuation intent " + nonce
   		}
   	} else {
   		event.IntentDetail = "no steward continuation intent was prepared because its goal and seat actor could not be established"
   		escalationRepair = true
   	}
   
   	incidentDetail := ""
   	incidentID := ""
   	if s.RecordIdleIncident == nil {
   		incidentDetail = "the steward incident recorder is unavailable"
   		escalationRepair = true
   	} else if id, err := s.RecordIdleIncident(event); err != nil {
   		incidentDetail = "the steward incident could not be recorded: " + err.Error()
   		escalationRepair = true
   	} else {
   		incidentID = id
   		incidentDetail = "recorded steward alert episode " + id
   	}
   	alarmDetail := "the human idle alarm was not raised because the steward intent was prepared"
   	if !event.IntentPrepared {
   		if s.RaiseIdleAlarm == nil {
   			alarmDetail = "the human idle alarm could not be queued because its steward seam is unavailable"
   			escalationRepair = true
   		} else if err := s.RaiseIdleAlarm(event); err != nil {
   			alarmDetail = "the human idle alarm could not be queued: " + err.Error()
   			escalationRepair = true
   		} else {
   			alarmDetail = "queued the steward's existing human idle alarm because a steward intent could not be prepared"
   		}
   	}
   
   	if !blockedBeforeIdle {
   		verdict.ShouldBlock = false
   		verdict.BlockSource = nil
   	} else {
   		verdict.IdleRefusal = false
   	}
   	detail := fmt.Sprintf("IDLE WITH BACKLOG: refusal %d reached the bound of 3 for this unchanged backlog; ", session.IdleBlocks)
   	if event.IntentPrepared && claimNeeded {
   		detail += fmt.Sprintf("selected goal %s and deferred its claim as %s to the steward tick; ", goalID, options.SeatActor.historyActor())
   	} else if event.IntentPrepared {
   		detail += fmt.Sprintf("selected this machine's held goal %s as %s without another claim attempt; ", goalID, options.SeatActor.historyActor())
   	} else if goalID == "" {
   		detail += "could not identify a continuation goal: " + event.ClaimDetail + "; "
   	} else {
   		detail += fmt.Sprintf("could not hand goal %s to the steward: %s; ", goalID, event.ClaimDetail)
   	}
   	if event.IntentPrepared {
   		detail += "prepared steward continuation intent " + event.IntentID + "; "
   	} else {
   		detail += "could not prepare a steward continuation: " + event.IntentDetail + "; "
   	}
   	detail += incidentDetail + "; " + alarmDetail
   	if blockedBeforeIdle {
   		detail += "; another turn-verdict branch remains blocking this stop"
   	} else {
   		detail += "; the turn will end"
   	}
   	detail += fmt.Sprintf("; stop_hook_active=%t", options.StopHookActive)
   	verdict.escalation = &TurnEscalationFacts{
   		IntentId:          event.IntentID,
   		IncidentId:        incidentID,
   		AlarmDetail:       alarmDetail,
   		Detail:            detail,
   		IntentPrepared:    event.IntentPrepared,
   		SupervisionRepair: escalationRepair,
   	}
   	verdict.Display = strings.TrimSpace(verdict.Display + "\n" + detail)
   }
   ```

4. `internal/goal/turnverdict.go:1549-1563`

   ```text
   func FencedClaimLines(files []*GoalFile) []string {
   	lines := make([]string, 0, len(files))
   	for _, file := range files {
   		// A breach-stopped goal is waiting on a human and must not keep the
   		// machine from taking the next item.
   		if !file.IsFencedClaim() {
   			continue
   		}
   		lines = append(lines, fmt.Sprintf(
   			"FENCED %s: breach-stopped by %s (%s); only goal resume, a human act, clears it; the queue is open",
   			file.Id, file.StopFence.StopID, file.StopFence.Reason,
   		))
   	}
   	return lines
   }
   ```

5. `internal/goal/turnverdict.go:1609-1614`

   ```text
   func (w ClaimableBudgetedWork) OnlyFencedClaim() (*GoalFile, bool) {
   	if len(w.Claimed) != 0 || len(w.fencedClaims) != 1 || !w.fencedClaims[0].IsFencedClaim() {
   		return nil, false
   	}
   	return w.fencedClaims[0], true
   }
   ```

6. `internal/goal/turnverdict.go:1700-1750`

   ```text
   			if work != nil && len(work.Claimable) > 0 && waits.hasWorkInFlight() {
   				detail := "WORK IN FLIGHT: a registered wait is joined to the claimed goal"
   				if work != nil && len(work.Claimable) > 0 {
   					detail += "; claimable shared backlog also includes " + strings.Join(work.Claimable, ", ")
   				}
   				display = append(display, detail)
   				break
   			}
   			if work != nil && work.HasDelegateJobInFlight() && len(work.Claimable) > 0 {
   				display = append(display, "WORK IN FLIGHT: a non-terminal delegate job is joined to a live process; claimable shared backlog also includes "+strings.Join(work.Claimable, ", "))
   				break
   			}
   			if !contains(session.BlockedGoalRevisions, facts.Revision) {
   				session.BlockedGoalRevisions = appendCapped(session.BlockedGoalRevisions, facts.Revision, maxGoalRevisions)
   				blockGoal("open work is done; the goal file names the next step: " + facts.NextStep)
   			} else {
   				display = append(display, "NOTHING LEFT TO WORK ON; the current goal is "+facts.Id+" ("+facts.NextStep+")")
   			}
   			if first, digest := s.queuedFrontier(); digest != "" {
   				if session.ObservedQueueDigest == "" {
   					session.ObservedQueueDigest = digest
   				} else if digest != session.ObservedQueueDigest {
   					session.ObservedQueueDigest = digest
   					queueNow := "is now empty"
   					if first != "" {
   						queueNow = "now starts with " + first
   					}
   					blockGoal(fmt.Sprintf("the shared goal queue changed while %s remains claimed here; it %s", facts.Id, queueNow))
   				}
   			}
   			if work != nil {
   				display = append(display, FencedClaimLines(work.fencedClaims)...)
   				display = append(display, LandingClaimLines(work.landingClaims, s.now())...)
   			}
   		case "queued-only":
   			first, _ := s.queuedFrontier()
   			if first == "" {
   				display = append(display, "no goal is claimed here and the queue is empty; a person opens the next goal (`goal open --origin human`); a seat opens only the blocker of its claimed goal (`--blocks`, R-93-m1e)")
   				if work != nil {
   					display = append(display, FencedClaimLines(work.fencedClaims)...)
   					display = append(display, LandingClaimLines(work.landingClaims, s.now())...)
   				}
   				break
   			}
   			display = append(display, "no current goal; the queue holds "+first)
   			if work != nil {
   				display = append(display, FencedClaimLines(work.fencedClaims)...)
   				display = append(display, LandingClaimLines(work.landingClaims, s.now())...)
   			}
   		case "goal-free":
   			fresh, digest, declared := s.freeState()
   ```

7. `internal/goal/turnverdict.go:1855-1893`

   ```text
   func (s *Store) queuedFrontier() (first, digest string) {
   	if NewWorld(s.Root) {
   		endpoint, err := ResolveEndpoint(s.Root)
   		if err != nil {
   			return "", ""
   		}
   		proj, err := Project(endpoint, false, s.now())
   		if err != nil || proj.Tree == nil {
   			return "", ""
   		}
   		type row struct {
   			id  string
   			rev uint64
   		}
   		var rows []row
   		for _, id := range OrderedOpenGoalIDs(proj.Tree.Live) {
   			f := proj.Tree.Live[id]
   			if f.State == StateQueued || f.State == StateApproved {
   				rows = append(rows, row{id, f.Revision})
   			}
   		}
   		if len(rows) == 0 {
   			return "", sha256Hex(nil)
   		}
   		var lines []string
   		for _, r := range rows {
   			lines = append(lines, fmt.Sprintf("%s@%d", r.id, r.rev))
   		}
   		return rows[0].id, sha256Hex([]byte(strings.Join(lines, "\n")))
   	}
   	ledger, _, _ := s.ReadLedger()
   	if ledger == nil {
   		return "", ""
   	}
   	if len(ledger.Queued) == 0 {
   		return "", sha256Hex(nil)
   	}
   	return ledger.Queued[0].Id, ledger.QueuedDigest()
   }
   ```

8. `internal/goal/project.go:72-125`

   ```text
   func Project(e Endpoint, fetchFirst bool, now time.Time) (Projection, error) {
   	return project(e, fetchFirst, now, projectionDependencies{})
   }
   
   func project(e Endpoint, fetchFirst bool, now time.Time, dependencies projectionDependencies) (Projection, error) {
   	dependencies = dependencies.withDefaults()
   	if fetchFirst {
   		if err := fetchProjectionWithinDeadline(e, dependencies); err != nil {
   			return Projection{}, err
   		}
   	}
   	tipOut, err := goalGit(e.Root, nil, "rev-parse", "--verify", "--quiet", AcceptedRef)
   	if err != nil {
   		return Projection{}, fmt.Errorf("no accepted tree; the first fetch or the migration bootstraps it")
   	}
   	tip := strings.TrimSpace(tipOut)
   	tree, err := loadTree(e.Root, tip)
   	if err != nil {
   		return Projection{}, err
   	}
   	p := Projection{Root: e.Root, Tip: tip, Tree: tree, Horizon: approvalHorizon(tree, now)}
   
   	// The durable sync-mode identity: the root record's word against
   	// the clone's config. A local-mode ledger with a remote config is
   	// the forbidden promotion; a remote-mode ledger pointed at local
   	// is a split-brain risk. Both refuse by name.
   	if tree.Root != nil {
   		recordMode := tree.Root.SyncMode
   		configLocal := e.LocalMode()
   		if recordMode == SyncLocal && !configLocal {
   			return Projection{}, fmt.Errorf("sync-mode mismatch refused: the ledger is committed local, the config says remote %q — promotion is the backlog-local-promotion goal, not a config flip", e.Remote)
   		}
   		if recordMode == SyncRemote && configLocal {
   			return Projection{}, fmt.Errorf("sync-mode mismatch refused: the ledger is committed remote, the config says local — a split brain is not a mode")
   		}
   		if recordMode == SyncLocal {
   			p.Banners = append(p.Banners, "single-machine mode: multi-machine guarantees are void here; joining a fleet is the backlog-local-promotion goal")
   		}
   	}
   
   	// Staleness: the accepted COMMIT's age is the tree's age.
   	if ageOut, err := goalGit(e.Root, nil, "log", "-1", "--format=%ct", tip); err == nil {
   		if seconds := strings.TrimSpace(ageOut); seconds != "" {
   			var epoch int64
   			if _, scanErr := fmt.Sscanf(seconds, "%d", &epoch); scanErr == nil {
   				age := now.Sub(time.Unix(epoch, 0))
   				if age > StaleThreshold {
   					p.Banners = append(p.Banners, fmt.Sprintf("the accepted tree is %s old; goal list --fetch validates and advances it", age.Round(time.Minute)))
   				}
   			}
   		}
   	}
   	return p, nil
   }
   ```

9. `internal/report/stopblock.go:95-213`

   ```text
   func StopBlock(detail string) map[string]any {
   	reason := stopBlockReason
   	if detail != "" {
   		reason = detail + "\n\n" + stopBlockReason
   	}
   	return map[string]any{
   		"decision": "block",
   		"reason":   reason,
   	}
   }
   
   // BoundedIdleStopBlock renders the idle backlog verdict without the open-work
   // block-once preface. Its detail already carries the current refusal count,
   // bound, and steward escalation.
   func BoundedIdleStopBlock(detail string) map[string]any {
   	return map[string]any{"decision": "block", "reason": detail}
   }
   
   // StopRefusal records one external stop failure and returns the provider
   // response for that occurrence. The first occurrence blocks; later
   // occurrences remain visible without keeping the turn open.
   // stopRefusalLockWait bounds the wait for an overlapping writer of the
   // refusal record. stopRefusalLockSleep is replaceable so tests can drive a
   // lock retry from the writer's release instead of wall time.
   var stopRefusalLockWait = 100 * time.Millisecond
   var stopRefusalLockSleep = time.Sleep
   
   func StopRefusal(path, session, cause, remedy, detail, systemMessage string, class StopClass, now time.Time) (map[string]any, error) {
   	if path == "" || session == "" || cause == "" || remedy == "" {
   		return nil, fmt.Errorf("stop refusal requires a record path, session, cause, and remedy")
   	}
   	if class != StopClassInfrastructure && class != StopClassSeatActionable {
   		return nil, fmt.Errorf("stop refusal class must be infrastructure or seat-actionable")
   	}
   	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
   		return nil, fmt.Errorf("prepare stop-refusal directory: %w", err)
   	}
   	lockFile, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
   	if err != nil {
   		return nil, fmt.Errorf("open stop-refusal lock: %w", err)
   	}
   	defer lockFile.Close()
   	if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
   		deadline := time.Now().Add(stopRefusalLockWait)
   		for err != nil && time.Now().Before(deadline) {
   			stopRefusalLockSleep(10 * time.Millisecond)
   			err = unix.Flock(int(lockFile.Fd()), unix.LOCK_EX|unix.LOCK_NB)
   		}
   		if err != nil {
   			return nil, fmt.Errorf("lock stop-refusal record: busy after %s: %w", stopRefusalLockWait, err)
   		}
   	}
   	defer func() { _ = unix.Flock(int(lockFile.Fd()), unix.LOCK_UN) }()
   
   	record := stopRefusalRecord{SchemaVersion: 1, SessionID: session, Causes: map[string]stopRefusalCause{}}
   	if data, readErr := os.ReadFile(path); readErr == nil {
   		if err := json.Unmarshal(data, &record); err != nil {
   			return nil, fmt.Errorf("read stop-refusal record: %w", err)
   		}
   		if record.SchemaVersion != 1 || record.SessionID != session || record.Causes == nil {
   			return nil, fmt.Errorf("read stop-refusal record: unexpected schema or session")
   		}
   	} else if !os.IsNotExist(readErr) {
   		return nil, fmt.Errorf("read stop-refusal record: %w", readErr)
   	}
   
   	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(cause)))
   	stamp := now.UTC().Format(time.RFC3339)
   	entry, repeated := record.Causes[digest]
   	if repeated && entry.Cause != cause {
   		return nil, fmt.Errorf("read stop-refusal record: cause digest does not match its cause")
   	}
   	if !repeated {
   		entry = stopRefusalCause{Cause: cause, FirstAt: stamp}
   	}
   	entry.Count++
   	entry.LastAt = stamp
   	record.Causes[digest] = entry
   	encoded, err := json.MarshalIndent(record, "", "  ")
   	if err != nil {
   		return nil, fmt.Errorf("render stop-refusal record: %w", err)
   	}
   	if _, err := atomicfile.WriteText(path, string(encoded)+"\n", ""); err != nil {
   		return nil, fmt.Errorf("write stop-refusal record: %w", err)
   	}
   	if class == StopClassInfrastructure {
   		message := fmt.Sprintf("Metasystem allowed stopping with degraded infrastructure (occurrence %d).\nCause: %s\nRemedy: %s", entry.Count, cause, remedy)
   		if detail != "" {
   			message += "\n" + BoundSystemMessage(detail)
   		}
   		return map[string]any{"systemMessage": boundSystemMessageWithTail(message, systemMessage)}, nil
   	}
   
   	if !repeated {
   		response := StopBlock(detail)
   		if systemMessage != "" {
   			response["systemMessage"] = BoundSystemMessage(systemMessage)
   		}
   		return response, nil
   	}
   	message := fmt.Sprintf("Metasystem allowed this repeated external stop failure to surface without blocking (occurrence %d).\nCause: %s\nRemedy: %s", entry.Count, cause, remedy)
   	if systemMessage != "" {
   		message += "\n" + systemMessage
   	}
   	return map[string]any{"systemMessage": BoundSystemMessage(message)}, nil
   }
   
   type stopRefusalRecord struct {
   	SchemaVersion int                         `json:"schemaVersion"`
   	SessionID     string                      `json:"sessionId"`
   	Causes        map[string]stopRefusalCause `json:"causes"`
   }
   
   type stopRefusalCause struct {
   	Cause   string `json:"cause"`
   	Count   int    `json:"count"`
   	FirstAt string `json:"firstAt"`
   	LastAt  string `json:"lastAt"`
   }
   ```

10. `internal/steward/tick.go:116-165`

   ```text
   func RunTick(repoRoot string, cfg TickConfig, census WorkerCensus) (result TickResult, returnErr error) {
   	cfg = cfg.withDefaults()
   
   	// One tick at a time per repository: the CLI seam and the
   	// resident runner share the evidence store and the pending
   	// queue, and neither may age or drain it under the other.
   	tickLock, err := AcquireArbitration(repoRoot)
   	if err != nil {
   		return TickResult{}, err
   	}
   	defer tickLock.Release()
   
   	selfExact, selfState, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
   	if err != nil || selfState != identity.Alive {
   		return TickResult{}, fmt.Errorf("the steward tick cannot read its own process identity")
   	}
   	generation, generationErr := installedGeneration(repoRoot)
   	if generationErr != nil {
   		// An unarmed manual tick remains useful for diagnosis, but generation
   		// zero can never satisfy the armed-runner health check.
   		generation = 0
   	}
   	tickAttempt, err := beginComponentAttempt(repoRoot, "steward-tick", generation, selfExact.Ref(), cfg.now())
   	if err != nil {
   		return TickResult{}, fmt.Errorf("record tick attempt: %w", err)
   	}
   	tickCompleted := false
   	defer func() {
   		if result.Health.Schema == 0 {
   			if healthErr := completeTickHealth(repoRoot, &result, generation, selfExact.Ref(), cfg.Now); healthErr != nil && returnErr == nil {
   				returnErr = healthErr
   			}
   		}
   		if tickCompleted {
   			return
   		}
   		evidence := "tick did not complete"
   		if returnErr != nil {
   			evidence = returnErr.Error()
   		}
   		if _, completeErr := completeComponentAttempt(repoRoot, "steward-tick", generation, tickAttempt.AttemptSeq,
   			ComponentError, "TICK_FAILED", evidence, nil, cfg.now()); completeErr != nil && returnErr == nil {
   			returnErr = fmt.Errorf("record failed tick completion: %w", completeErr)
   		}
   	}()
   
   	// Budget healing runs before health and notification. A successful stop is
   	// machinery history only; a failure remains visible to the ordinary health
   	// breaker, which is the sole escalation owner.
   	goalStops := runBreachStopCustodian(repoRoot, cfg.now())
   ```

11. `internal/steward/tick.go:225-250`

   ```text
   		return TickResult{}, err
   	}
   	if d.Action == ActNotify {
   		// A notify verdict IS the visibility the invariant promises:
   		// it goes to the queue, keyed by its verdict so the standing
   		// condition holds one pending message (redelivered after each
   		// successful delivery, held durably through an outage).
   		if err := QueueNotification(repoRoot, PendingNotification{
   			Nonce:   "verdict-" + string(d.Verdict),
   			Message: fmt.Sprintf("steward: %s — %s", d.Verdict, d.Reason),
   		}); err != nil {
   			return TickResult{}, err
   		}
   	}
   	result = TickResult{Decision: d, Evidence: ev, OpenWork: workReason,
   		Reaped: reaped, ProviderOutage: providerOutage, Outage: outageMark, GoalStops: goalStops,
   		LedgerAttention: ledgerReport}
   	if err := NarrateDigest(repoRoot, prev, result, cfg.now()); err != nil {
   		return result, fmt.Errorf("write narrator digest: %w", err)
   	}
   	machine := "this machine"
   	if enrolled, err := goal.ResolveMachine(repoRoot); err == nil {
   		machine = enrolled
   	}
   	var surfaced []string
   	for _, event := range ledgerReport.Pending {
   ```

12. `internal/steward/intervene.go:336-351`

   ```text
   func QueueNotification(repoRoot string, n PendingNotification) error {
   	dir := pendingDir(repoRoot)
   	data, err := json.Marshal(n)
   	if err != nil {
   		return err
   	}
   	path := filepath.Join(dir, n.Nonce+".json")
   	durable, err := atomicfile.WriteText(path, string(append(data, '\n')), repoRoot)
   	if err != nil {
   		return err
   	}
   	if !durable {
   		return fmt.Errorf("pending notification %s was published with durability unknown", n.Nonce)
   	}
   	return nil
   }
   ```

13. `cmd/metasystem/report.go:203-225`

   ```text
   	if flags.NArg() > 0 {
   		detail = flags.Arg(0)
   	}
   	var block map[string]any
   	if *refusalRecord != "" || *session != "" || *cause != "" || *remedy != "" {
   		if *refusalRecord == "" || *session == "" || *cause == "" || *remedy == "" {
   			fmt.Fprintln(os.Stderr, "report stop-block: --refusal-record, --session, --cause, and --remedy must be provided together")
   			return 2
   		}
   		var err error
   		block, err = report.StopRefusal(*refusalRecord, *session, *cause, *remedy, detail, *systemMessage, report.StopClass(*class), time.Now())
   		if err != nil {
   			fmt.Fprintf(os.Stderr, "report stop-block: %v\n", err)
   			return 1
   		}
   	} else {
   		if *boundedIdle {
   			block = report.BoundedIdleStopBlock(detail)
   		} else {
   			block = report.StopBlock(detail)
   		}
   		if *systemMessage != "" {
   			block["systemMessage"] = *systemMessage
   ```

14. `cmd/metasystem/goal.go:528-545`

   ```text
   	for _, entry := range frontier.TrunkRedOwned {
   		fmt.Println(trunkRedOwnedLine(entry))
   	}
   	fenced := make([]*goal.GoalFile, 0, len(frontier.Fenced))
   	for _, id := range frontier.Fenced {
   		fenced = append(fenced, p.Tree.Live[id])
   	}
   	for _, line := range goal.FencedClaimLines(fenced) {
   		fmt.Println(line)
   	}
   	landing := make([]*goal.GoalFile, 0, len(frontier.Landing))
   	for _, id := range frontier.Landing {
   		landing = append(landing, p.Tree.Live[id])
   	}
   	for _, line := range goal.LandingClaimLines(landing, now) {
   		fmt.Println(line)
   	}
   	selection := goal.SelectNext(frontier)
   ```

15. `scripts/agents/supervision-hook.sh:1720-1727`

   ```text
   
   report_turn_verdict() {
     if [[ "$session_absent" == true ]]; then
       "$ms" report turn-verdict --session-absent "$@"
     else
       "$ms" report turn-verdict "$@"
     fi
   }
   ```

16. `scripts/agents/supervision-hook.sh:2436-2447`

   ```text
     # The evidence trail sits beside the rest of the supervision state. One
     # hook-log line per infrastructure condition, in every outcome: the
     # advisor's allowance below appends its lines too.
     supervision_dir="$repo/artifacts/agents/supervision"
     mkdir -p "$supervision_dir" 2>/dev/null || true
     hook_log_failure=
     append_stop_condition() { # class, cause code, component, outcome
       local generation=${hook_generation:--} deadline_end=$((stop_started_epoch + 60))
       if ! printf 'stop-condition %s %s %s %s %s %s\n' "$1" "$2" "$3" \
           "$generation" "$deadline_end" "$4" >>"$supervision_dir/hooks.log" 2>/dev/null; then
         hook_log_failure="the infrastructure stop condition could not be appended to the hook log"
       fi
   ```

17. `scripts/agents/supervision-hook.sh:2567-2592`

   ```text
     if (( ${#stop_conditions[@]} > 0 )); then
       for stop_condition in "${stop_conditions[@]}"; do
         append_stop_condition infrastructure "${stop_condition%%|*}" "${stop_condition#*|}" degraded-allow
       done
     fi
     if [[ "$verdict_readable" == true && "$verdict_class" == infrastructure ]]; then
       verdict_cause=$("$ms" json get --value "$verdict" --field causeCode 2>/dev/null || true)
       verdict_component=$("$ms" json get --value "$verdict" --field component 2>/dev/null || true)
       append_stop_condition infrastructure "${verdict_cause:-turn-verdict-unavailable}" "${verdict_component:-verdict-state}" degraded-allow
       # The verdict's own state could not be read or written: the notice says
       # so in fixed words, names the owner, and carries the detail; it never
       # reads as an all-clear.
       display="turn-verdict degraded: stopping is allowed on degraded infrastructure; the steward owns repair. Cause: ${verdict_cause:-turn-verdict-unavailable}. Component: ${verdict_component:-verdict-state}.
   $display"
     fi
     if [[ "$verdict_readable" == true && "$verdict_class" == idle-with-backlog && "$should_block" == true && "$count_spent" == false ]]; then
       append_stop_condition idle-with-backlog idle-refusal-count-not-spent verdict-state refused-uncounted
     fi
     if [[ "$verdict_readable" != true ]]; then
       append_stop_condition infrastructure turn-verdict-unavailable verdict-state degraded-allow
     fi
   
     if [[ "$verdict_readable" == true ]]; then
       printf '%s stop verdict block=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
         "$should_block" >>"$supervision_dir/hooks.log" 2>/dev/null || true
   
   ```

18. `scripts/enforcement/claude-code-hooks.json:28-38`

   ```text
       "Stop": [
         {
           "hooks": [
             {
               "type": "command",
               "command": "(bash scripts/agents/supervision-hook.sh claude stop) || printf '%s\\n' '{\"systemMessage\":\"Task unknown; Stop allowed; needs supervision repair; hook-bootstrap-failed. The steward must restore supervision. Status unavailable.\"}'",
               "timeout": 60
             }
           ]
         }
       ],
   ```

19. `scripts/agents/adapters/codex.sh:122-160`

   ```text
   supervise() { # dispatch|follow-up and supervisor args
     local verb=$1
     shift
     prepare_supervision "$verb" "$@" || { usage; return 2; }
     record_build_cache_path "$workspace" "$round_dir"
     local usage_file="$round_dir/usage.json"
     local cli_pid event_session event_turn reasoning_effort
     local -a command
   
     record_actual_workspace_write_scope
     fail_if_effective_wider_before_launch || return 1
     : >"$events"
     : >"$raw"
     # The envelope decides sandbox and network — in the engine, from the
     # record itself (KI-12: a hard-coded value made the recorded field
     # decorative).
     reasoning_effort=$(field "$record" reasoningEffort 2>/dev/null || true)
     [[ "$reasoning_effort" != null ]] || reasoning_effort=
     build_codex_command "$verb" "$requested_model" "$workspace" "$schema" "$raw" \
       --record "$record" "$requested_session" "$reasoning_effort"
     command=("${codex_cli_command[@]}")
   
     # The write boundary is the CLI's cwd. `codex exec` takes -C, but
     # `codex exec resume` has no such flag, so a resumed turn would otherwise
     # inherit the adapter's cwd (the metasystem root) and be free to write the whole
     # repository while the record still claimed the job worktree. Entering the
     # workspace makes the recorded boundary true on both paths. `exec` keeps the
     # pid, which custody registration depends on.
     local -a job_git_env=()
     while IFS= read -r assignment; do job_git_env+=("$assignment"); done < <(job_git_quarantine_env "$workspace")
     verify_references_before_launch || return 1
     mark_cli_prefork || { fail_pending prefork_marker handshake; return 1; }
     # The chain's build cache: the sandbox cannot write the user's Go cache,
     # and a delegate left to itself sets a cold one per round (the deep dive's
     # slowest gates all did).
     while IFS= read -r assignment; do job_git_env+=("$assignment"); done < <(job_build_cache_env "$workspace")
     ( cd "$workspace" && exec env ${job_git_env[@]+"${job_git_env[@]}"} "${command[@]}" ) <"$prompt" >"$events" 2>>"$log" &
     cli_pid=$!
     register_cli_custody "$cli_pid" || { terminate_cli_child "$cli_pid"; fail_pending custody_registration handshake; return 1; }
   ```

Example page:

The member-2 page below (plans/stop-decisions-record-deadline-evidence-design.md, revision 4, at 4e4e46de7) shows the level of detail expected of one member: grounding, decisions by symbol, fixtures as the builder's proof obligations, landing, moved effects. The members page uses the seven-part section shape given above, one section per member, at this depth.

    # stop-decisions-record-deadline-evidence: build design
    
    Revision: 4 (2026-09-14), following revision 3 at 78f40168.
    This revision decides the two hook-bed fixtures that reached the retired shell parent. Other decisions stand.
    
    Member two of `metasystem/plans/stop-hook-never-forces-an-empty-turn-design.md`.
    Two reads are spent. This page decides the eight remaining findings
    from `metasystem/plans/stop-decisions-record-deadline-evidence-design-read2-findings.md`.
    Wido's direction ends prose review here. Codex builds these obligations;
    its fixtures and an Opus read of the build judge them. No third design read.
    
    ## 1. Grounding and boundary
    
    The Stop parent captures worker stdout; that file is not provider delivery.
    `metasystem/scripts/agents/supervision-hook.sh` waits for cleanup and hook-expire
    before its deadline fallback can speak.
    `runReportTurnVerdict` in `metasystem/cmd/metasystem/goal.go` calls
    `MarkOpenWorkSeen` before `TurnVerdict`.
    `metasystem/internal/report/openwork.go` writes that marker under an
    unbounded flock. `metasystem/internal/goal/turnverdict.go` also changes
    seen state and the idle counter before returning its verdict.
    The two seconds in `metasystem/internal/goal/goalverbs.go` bound acquisition,
    not the work inside the lock. A longer parent lock wait cannot close this race.
    
    Classification, idle digest, the three-observation handoff and human-stop
    eligibility stay with their existing goal owners. This member changes when
    refusal effects become real. Steward incident delivery and the Codex adapter
    boundary remain later members. The per-session verdict file stays display.
    
    ## 2. Episode identity: SDE-02 accepted
    
    Keep the canonical checkout state root and the per-session refusal record.
    Version 2 has `episodes`, an `invocations` map from token to episode key,
    and `upgradedAt` when it imports version-1 history.
    The ordinary key is `<generation>/<attempt>/<deadlineEnd>`.
    Generation and attempt come from `BeginHookAttempt` in
    `metasystem/internal/steward/component_evidence.go`; deadlineEnd is the
    outer invocation's fixed UTC epoch-second deadline, never a retry's start.
    This member implements the shorter key instead of the umbrella's longer key
    because checkout and session scope the record, lease is evidence, and a
    condition fingerprint identifies an incident within that invocation.
    
    Replace the uncertain PID/start suffix with `0/0/<deadlineEnd>/<token>`.
    The parent generates 32 random bytes and reserves their lowercase hexadecimal
    token by exclusive mkdir beneath `artifacts/agents/supervision/stop-invocations`
    in that root. Retry collisions; never reuse a reservation directory.
    Keep the empty directory permanently after its evidence is pruned.
    Failure to reserve yields an unrecorded degraded response, never a borrowed key.
    The parent retains the token in memory and gives it to every child explicitly.
    
    An episode stores canonical `checkout`, normalized `session`, `leaseEpoch`,
    `enrollmentGeneration`, generation, attempt, deadlineEnd, token and preparedAt.
    Unknown lease and enrollment values are zero. They grant no authority.
    Prepare binds a token once under the record lock. Repeating it returns that
    binding, including an uncertain binding chosen before a late worker arrived.
    A later generation report never creates a second episode for the same token.
    Different tokens claiming an occupied ordinary key fail with identity conflict;
    they do not merge. Incident fingerprints never change the episode key.
    
    ## 3. Identity handoff: SDE-03 accepted
    
    The parent publishes an invocation descriptor before starting the worker.
    It contains schemaVersion 1, token, checkout, session, deadlineEnd and the
    absolute deadline directory. Failure leaves the Stop explicitly unrecorded.
    The worker publishes `episode.json` there after hook-attempt and before prepare.
    It contains the descriptor's token and deadline plus generation, attempt,
    leaseEpoch and enrollmentGeneration. Failed hook-attempt uses zero coordinates.
    Both publications use `atomicfile.WriteText` from
    `metasystem/internal/atomicfile/atomicfile.go`, with a known existing anchor.
    No direct redirection, empty placeholder or in-place replacement is allowed.
    
    The parent strictly decodes one JSON object and validates its token and deadline.
    Missing, empty, truncated, malformed or mismatched handoff files are all an
    unavailable handoff. First look up the parent's token in `invocations`.
    An existing binding wins. Otherwise prepare the uncertain key with that token.
    A valid handoff with no binding prepares its ordinary key. A pending binding
    is completed by the parent. Thus death on either side of rename or prepare
    leaves at most one episode. Temporary files never establish an identity.
    
    ## 4. Arming and interfaces: SDE-06 accepted with a factual correction
    
    The finding overlooks `StopArmingResult` and its parser in
    `metasystem/internal/report/stoppresentation.go`; they already preserve typed
    components and streams, but neither proves that arming finished.
    Reuse that presentation owner. Add explicit completeness through an arming
    envelope rendered by `up.Result` in `metasystem/internal/up/up.go`.
    The envelope has schemaVersion 1, complete, componentLines, otherLines,
    aggregate and nullable exitCode. The lines are exact renderer output.
    `up --result-file FILE` atomically publishes an initial incomplete envelope,
    then a snapshot after each component result, then the final complete envelope.
    Incomplete aggregate is exactly `up outcome=incomplete`; exitCode is null.
    Complete means the arming call returned, even when its outcome is failure.
    Final lines and aggregate equal `Result.Lines()` byte for byte.
    Stop presentation reads this envelope instead of guessing from the last line.
    Missing or invalid evidence uses the last valid snapshot, or the empty
    incomplete envelope. It never changes a separately observed work refusal.
    
    Register `report stop-decision` in `metasystem/cmd/metasystem/main.go`.
    Its parser lives in `metasystem/cmd/metasystem/report.go`. The interfaces are:
    
    - `run --installation ROOT --runtime NAME --worker-script FILE`: the Go
      deadline parent; also requires `--deadline-end E`, reads stdin, alone writes stdout.
    - `prepare --invocation FILE --generation G --attempt A --lease-epoch L
      --enrollment-generation N`: resolves the descriptor's root, returns its
      record path and bound episode key. Zero generation and attempt mean uncertain.
    - `complete --invocation FILE --selection FILE`: parent-only composition.
      Selection is schemaVersion 1 with token, nullable episode, proposal or null, parent cause
      or null, and an arming envelope. Its token must match the descriptor.
    - `incident --invocation FILE --episode ID --observation ID --cause CODE
      --component NAME [--kind KIND] [--identity ID]`: records one observation.
    - `repair --root ROOT`: repairs retained decision, log and emission intents.
    
    `report turn-verdict` adds separate `--invocation FILE --episode ID
    --proposal-file FILE --arming-file FILE` flags to its existing inputs.
    All four are required together. It reads only this explicit descriptor;
    no environment variable or colon-split path supplies record coordinates.
    Absent arming uses the incomplete rule above.
    `report stop-block --class infrastructure` uses the same invocation, episode
    and observation flags. A legacy call cannot overwrite a version-2 record.
    The shell passes the descriptor path to child commands; it owns no JSON state.
    Run starts `bash FILE NAME stop --invocation FILE` with the existing parent-PID
    guard. That guard prevents recursion; it supplies no decision coordinates.
    The proposal contains schemaVersion 1, token, episode, verdict, frozen facts,
    deferred effects and a ready provider response. Hash its stored bytes with SHA-256.
    Cache a valid fallback rendering of each proposal before accepting it, so a
    stalled later formatter cannot erase a known block. Cache arming separately.
    
    ### Optional health and narrator evidence: excluded from the record
    
    The decision record carries the arming envelope alone as optional evidence.
    Its accepted DONE requires the full up result. Separate readers retain the other facts.
    Do not collect health or narrator attachments for the record or frozen selection.
    Health stays absent from the provider response. Section 5 preserves narrator report delivery.
    Absent, late or unreadable attachments cannot change the decision, add an incident or delay emission.
    They receive no acknowledgement. Omission proves no health delivery; hook-complete keeps its proof checks.
    The reader loses the attached health and narrator snapshots for this decision.
    Current health is read by `health --repo ROOT` in `metasystem/cmd/metasystem/steward_verbs.go`.
    That fresh reading cannot reconstruct health at the earlier Stop.
    Narrator history stays in `metasystem/records/narrator-digest.log`; `steward digest-pending --repo ROOT` reads
    pending text without moving its cursor, through `metasystem/internal/narratordigest/digest.go`.
    No later member or goal adds these attachments. That needs a separate accepted design.
    
    ## 5. Delivery and the last seconds: SDE-01 and SDE-11 accepted
    
    Replace worker completion with a proposal. The goal owner evaluates on a copy
    of refusal state and returns the verdict plus deferred refusal effects.
    The command's early `MarkOpenWorkSeen` becomes a read-only seen-state lookup.
    Proposal publication changes no open-work marker, goal/free/queue/unwatched
    seen marker, watchdog marker, green cursor or idle count. It invokes no idle
    continuation or alarm callback. The worker's stdout is only private transport.
    Existing human authorization inspection and single-use consumption retain
    their checks; they cannot be inferred from a proposal or a failed state write.
    
    The Go parent owns selection, completion and provider emission. Its controller
    lives in new `internal/report/stopdeadline.go`; command composition supplies
    goal callbacks so the goal package acquires no dependency on report.
    The shell replaces its existing deadline block with this command, retaining
    the fixed engine-missing allowance when no parent engine can start.
    If the engine lacks `report stop-decision`, emit the fixed degraded allowance:
    It says "Metasystem engine and hook are out of step" and names rebuilding `metasystem/bin/metasystem`.
    No worker starts. Supported attempts complete in the parent process, without the elapsed-flag retry.
    Replace that retry fixture with StopEngineSkewAllowance in `metasystem/scripts/agents/supervision-hook-fixtures.sh`.
    It refuses the new verb and proves one allowance names the skew and rebuild, with no worker, retry or claimed completion.
    
    T is outer hook entry. Freeze E = T + 60 seconds there, before payload, Git or
    resolver work. Pass its UTC epoch-second value explicitly to the Go parent.
    Map the remaining time to a monotonic timer; engine startup never resets E.
    All filesystem work, command execution and response preparation run off the
    timer/output loop. Cancellation does not require their acknowledgement.
    The fixed degraded JSON is already in memory before starting those tasks.
    Proven foreign-runtime and delegate skips still exit silently without a decision.
    
    | Absolute cutoff | Allowed work and required parent action |
    | --- | --- |
    | Before E minus 3 seconds (T + 57) | Worker publishes a work proposal before optional arming collection. It may attach newer arming snapshots. Parent validates and caches complete proposals as they arrive. No refusal effects are spent. |
    | At E minus 3 seconds (T + 57) | Parent cancels optional work without waiting and stops accepting worker results. In-flight evidence writes may finish; they spend nothing without a matching receipt. Parent freezes the latest validated proposal, or an infrastructure allowance if none exists. An observed block stays a block through recording failures. Expiry or unreadable output adds its parent incident. |
    | By E minus 2 seconds (T + 58) | Parent stops waiting for every read, lock, completion write, append and formatter. Record locks get at most 100 milliseconds within this interval, not a fresh relative allowance. A held worker lock cannot extend it. Unfinished persistence is labelled unproven; a known failure is labelled unrecorded. |
    | At E minus 1 second (T + 59), latest | Parent starts writing its single, fully buffered response. It uses the frozen decision and cached evidence. No cleanup, resolver join, hook-expire or new engine call precedes that write. |
    | By E minus 0.5 seconds (T + 59.5) | The complete response has been written or the bounded output operation has failed. Only full successful output can produce an emission receipt. A partial write, closed pipe or timeout produces none and never starts another JSON response. |
    | Until E (T + 60) | Close provider stdout. Attempt the receipt and bounded bookkeeping; then exit. Signal only recorded owned children. Never wait past E for cleanup or hook-expire. Retained intents are recovered on the next Stop. |
    
    Early success follows the same selection, persistence, emission and receipt
    order without waiting for T + 57. Invalid worker output starts fallback work
    immediately; it gets no new budget. Late results cannot replace a selection.
    The selected decision is immutable even if its persistence task finishes late.
    Move hook-complete and narrator cursor advancement from worker output to parent
    delivery bookkeeping. Captured child stdout must not claim an emitted generation.
    Only narrator attachments are excluded. The Stop report keeps delivering the pending digest.
    After its matching emission receipt, the parent calls `narratordigest.Advance` for the installation's
    human cursor with the cursor and prefix hash frozen with that report. No delivered digest means no advance.
    The template-layout report and cursor assertions in `metasystem/scripts/agents/supervision-hook-fixtures.sh` stand.
    Remove the `deadline-emitted` marker protocol entirely.
    
    ### The record write that fails, and the worker after the cutoff
    
    Two landed hook-bed fixtures reached the shell parent's verbs and file layout.
    Both guarantees stand. Their faults and evidence follow the Go parent.
    
    A record write failure at the deadline still allows, says so in one line, and
    logs its own outcome. The parent completes the decision in its own process, so
    a stub engine refusing `report stop-block` injects nothing; that stub retires.
    The fault is a record path the parent cannot write. The bed makes
    `artifacts/agents/supervision/stop-refusals` under the fixture's state root
    read-only for the one Stop and restores it after. Prepare then cannot create
    that session's lock or record, completion returns recorded false with the error,
    the allowance ends `; stop deadline expired; record update failed.`, the log line
    reads `stop response outcome=deadline-expired-record-failure-allow`, and no record
    exists for the session. The hook log and the invocation directory are siblings and
    stay writable, so the outcome line and the retained selection still land.
    This proof stays in the shell bed as StopDeadlineRecordWriteRefused. The Go tests
    of the notice bytes and the outcome name stand; only the bed reaches the real
    record owner through the real shell.
    
    The parent starts the worker as its own direct child and holds its handle, so
    ownership is never in doubt and `ps` is never consulted. At the freeze cutoff,
    when the worker has not returned, the parent ends that child through the handle.
    That is the one signal the cutoff table allows. The worker's own children are not
    signalled: an engine call still running inside the worker finishes on its own, and
    its late result cannot change the selection. The reader is told once, on the
    parent's stderr, in the fixed form
    `stop deadline: worker <pid> ended at the freeze cutoff; its children were not signalled`.
    Nothing the worker wrote to its own stderr before the cutoff is forwarded, because
    children it left behind may still hold its pipes. The other evidence is what the
    parent already leaves: the outcome line, the retained `stop-deadline-expired`
    incident, and the expired hook attempt. No worker directory is left under TMPDIR;
    worker output lives in the parent's memory. StopDeadlineEndsOnlyItsDirectWorker proves this.
    
    Completion records class, outcome, reason, command, countState, completedBy
    and completedAt. Class remains infrastructure, seat-actionable or idle-with-backlog.
    Outcome is block or allow. Command stays empty until clearing commands land.
    The parent is the sole completer; completedBy names the selected source,
    worker or deadline. CountState is not-spent when effects await delivery,
    otherwise none. A proposal is never a completed or counted refusal.
    
    The parent writes an atomic `emission.json` in its reserved invocation directory
    only after the complete provider write succeeds. It names token, episode,
    selected-decision hash, exact response hash and emittedAt. No child output,
    exit code, saved proposal, lock release or marker may substitute for that event.
    This proves delivery to the provider pipe; it does not claim a human read it.
    Death before that receipt may repeat a refusal. It can never suppress one unseen.
    An unresolved binding leaves episode null in selection, receipt and log; token
    still joins them. Repair resolves the token once, without inventing a second key.
    
    The goal and open-work owners apply deferred effects only with a matching
    receipt. Each effect carries its owner's prior-state digest and invocation token.
    Under that owner's lock, apply once and save the applied token in the same
    atomic write. An already applied token is success; a changed prior digest is
    not-spent, without overwriting newer state. Report per-owner results separately.
    The idle count's result alone determines its spent status. Lost writes leave
    the next stop eligible. A settlement event records results; it does not rewrite
    the pre-emission decision line or claim that it was already counted then.
    The display calls a proposed idle occurrence pending until delivery is recorded.
    Settlement runs the existing third-observation callbacks before its goal-state
    commit, preserving their admission checks and any independent block.
    Save the continuation nonce in the proposal; retries reuse it, never mint another.
    As today, callback failure records its incident/alarm; it does not invent success.
    The command recovers outstanding receipts before evaluating the next Stop.
    Never replay effects older than the existing thirty-day verdict-state horizon.
    Keep applied tokens outside session/revision caps for that horizon; never count twice.
    
    ## 6. Incidents and the permanent stream: SDE-05 and SDE-07 accepted
    
    Complete returns `{episode, decision, alreadyCompletedBy, recorded, recordError}`.
    `recorded` is true for durable publication, `unproven` for publication with
    unknown durability, and false for a failed publication. Keep SDE-04 exactly.
    An unfinished write at the cutoff is also unproven, never claimed durable.
    False includes recordError. Notices distinguish record failure from log failure.
    A repeat returns the same decision and source. It still upserts its parent
    incident in a separate record update, even when the decision already exists.
    Failure of that update never erases or changes the decision.
    
    Expiry uses cause `stop-deadline-expired`, component `stop-deadline`.
    Unreadable output uses `stop-hook-output-was-unreadable`, component `stop-worker`.
    Parent observation IDs are `<token>/deadline` and `<token>/unreadable`.
    Retries with the same observation ID do not increase count. Distinct observations
    of one fingerprint do. Fingerprints contain cause, component, kind and identity;
    they exclude timestamps, elapsed time and wording. Keep firstAt, lastAt, count,
    observation IDs and delivery. Incidents remain undelivered until acknowledged.
    
    Before emission, complete saves the frozen selection and parent incidents in
    the invocation directory. It then updates the session record, freezes its write
    result and exact log bytes in an append intent, and ensures the log entries.
    These intents use atomicfile independently of the record lock. If recovery finds
    a published record without a saved write result, that result is unproven.
    If both stores fail, retain the observed decision and say delivery unconfirmed.
    The next prepare and explicit repair recover intents, including a parent that
    timed out while the worker held the record lock. They finish the token's one binding.
    
    `stopblock.go` owns the append protocol and a separate checkout-wide log lock.
    Replace the hook's direct decision/condition appends with this owner's events.
    Send other diagnostics to stderr; only this owner appends to `hooks.log`.
    Persist each event's ID and exact bytes in its intent before appending.
    Decision ID is `<token>/decision`. Incident ID is token plus fingerprint hash;
    its log event ID also includes observation ID. Use SHA-256 over encoded fields.
    Under the log lock, scan for that ID. Identical complete bytes mean already
    appended. Conflicting bytes fail visibly. Append only when the ID is absent.
    Save the starting offset before append. Truncate only a final partial prefix
    matching that intent; other corruption fails visibly. Never remove complete lines.
    Append the full newline-terminated line and sync the file.
    Death before append leaves a recoverable intent. Death after append needs no
    second line: recovery scans the log rather than trusting an append-done flag.
    Never nest the record and log locks. A failed append is retained for retry and
    reported in the provider notice; a sync doubt is reported as unproven.
    
    Each JSON line has kind, eventId, token, episode, checkout, session, leaseEpoch,
    generation, attempt, deadlineEnd, aggregate, recorded and recordError.
    Decision lines also have class, outcome, reason, command, countState,
    completedBy and completedAt. Incident lines carry their full incident fields.
    Emission and settlement lines reference the decision ID. Count confirmed
    refusals by distinct decision ID with a receipt. Report selected blocks without
    receipts separately as unconfirmed; seven-day acceptance cannot count them as zero.
    A repeated completion repairs missing lines; it appends no extra relay or decision.
    
    ## 7. Retention and the later drain: SDE-08 accepted
    
    Never prune an episode with an undelivered incident, pending append or unsettled
    in-horizon emission receipt. Prune other completed episodes seven days after
    the later of completedAt and last delivery acknowledgement. Recover selections first.
    A pending episode without a selection, past deadlineEnd, completes as an
    infrastructure allowance with cause `stop-episode-abandoned`; retain it until acknowledged.
    There is no unconditional thirty-day episode deletion. Imported version-1 causes
    are history only. Set upgradedAt once during the atomic upgrade and drop only
    that history thirty days later. Reopening the record never resets upgradedAt.
    Reservation tombstones and complete hook-log lines are never pruned.
    This member requires the later drain to union record incidents, retained intents
    and unacknowledged log events by incident ID, unlike the umbrella's unreadable-only
    log fallback, because a readable record can still lack a failed incident update.
    Acknowledgements use the same incident ID across all three sources.
    No infrastructure delivery or steward wake-up runs on the Stop path.
    
    ## 8. Fixtures: the builder's proof obligations
    
    These tests are build obligations. Use Go fake clocks, injected I/O and barriers.
    Crash fixtures restart a fresh owner against the same temporary files after
    the named injection point. They must not use sleeps to guess the interleaving.
    Hook beds launch the shell and Go parent with owned, bounded fake children.
    
    | Finding and named test | File | Forced fault or interleaving; required observation |
    | --- | --- | --- |
    | SDE-01: TestStopDeadlineCompletionIsSingleUse | New `cmd/metasystem/stopdecision_test.go` | Barrier holds the worker's record lock before publishing its proposal. Advance parent to T + 58, emit its allowance, then release worker. No receipt matches the worker proposal; every seen marker and idle count is unchanged. Next invocation refuses the same work. Repeat with a cached block and receipt before/after effect save; count once. |
    | SDE-01: TestOpenWorkWaitsForProviderEmission | `metasystem/cmd/metasystem/goal_test.go` | Publish a real open-plan proposal, then fault provider output with a short write and closed pipe. The open-work marker remains absent. Full output followed by receipt creates it once. Worker stdout alone never creates it. |
    | SDE-11: TestStopParentAbsoluteCutoffs | New `internal/report/stopdeadline_test.go` | Independently stall payload staging, resolver, record read/lock/write, log append, formatter, child wait and hook-expire. Fake time reaches each table cutoff. Exactly one frozen response starts by T + 59; no receipt follows output failure. Late I/O cannot change that response. |
    | SDE-02: TestInfrastructureDeadlineIdentity | `metasystem/internal/report/stopblock_test.go` | Same-second retries have different attempts. Two uncertain calls reuse PID and start time; force the random allocator's first collision and observe exclusive retry. Token retries bind once; occupied ordinary keys never merge. Concurrent condition observations count within their own episode. |
    | SDE-03: TestEpisodeHandoffCrashWindows | New `internal/report/stopdeadline_test.go` | Crash before handoff rename, after rename before prepare, and after prepare. Inject empty, truncated and wrong-token files separately. Recover by the parent's token; exactly one episode survives each case. Also race uncertain parent prepare with late ordinary worker prepare. |
    | SDE-05: TestParentIncidentSurvivesCompletedDecision | `metasystem/internal/report/stopblock_test.go` | Complete a worker-sourced block, then retry completion with each parent cause twice. Decision bytes stay unchanged; each parent incident has count one. Fail the incident update and recover its retained intent without changing the block. |
    | SDE-06: TestStopDecisionInterfaceInputs | New `cmd/metasystem/stopdecision_test.go` | Exercise every declared verb. Missing descriptor, mismatched token/root, incomplete flag groups and trailing JSON fail explicitly. Absent arming is accepted as incomplete. No test supplies hidden deadline environment variables. |
    | SDE-06: TestArmingDetailSurvivesStop | `metasystem/cmd/metasystem/up_test.go` | Capture ENROLLMENT_DRIFT and failed component output. Compare sidecar, stored episode and Stop presentation lines byte for byte. Interrupt between components; only published components survive and complete is false. |
    | Optional evidence: TestStopDecisionCarriesArmingOnly | New `cmd/metasystem/stopdecision_test.go` | Keep a pending narrator entry and health evidence in their existing stores. Try absent, late and unreadable optional attachments. The record and frozen selection carry only arming evidence beside the decision. No extra incident, delay or attachment acknowledgement appears. Separate readers still expose their facts. Stop report delivery and its cursor follow section 5. |
    | SDE-07: TestDecisionAppendCrashWindows | `metasystem/internal/report/stopblock_test.go` | Inject death after intent save, after record completion before append, midway through append, and after full append before acknowledgement. Restart repair twice. One complete decision line remains; a relay repairs a missing line. Concurrent emitters cannot interleave bytes. |
    | SDE-08: TestUndeliveredIncidentRetention | `metasystem/internal/report/stopblock_test.go` | Advance fake time past thirty days with an undelivered incident, a pending append and an unacknowledged log-only incident. Recover their union once by ID. Acknowledged history prunes after seven days; v1 history uses the original upgradedAt. Retention removes no log bytes. |
    | SDE-04 retained: TestStopDecisionPersistsBeforeEmission | New `cmd/metasystem/stopdecision_test.go` | Fault before rename, after rename during directory sync, and in append. Observe false with error, unproven, and true as appropriate before provider output; preserve a known block through persistence failures. |
    | Hook bed: StopDeadlineRecordRecovery | `metasystem/scripts/agents/supervision-hook-fixtures.sh` | Launch the Go tests above as plumbing. Drive missing handoff, unreadable worker output, frozen block plus expiry, stalled lock and append failure through the actual shell. Observe one provider JSON response and recoverable parent incidents. |
    | Hook bed: StopDeadlineRecordWriteRefused | `metasystem/scripts/agents/supervision-hook-fixtures.sh` | Hang the worker in hook-attempt with the deadline engine stub; make the state root's `stop-refusals` directory read-only for this Stop and restore it after. Observe the record-failure allowance, its outcome line and no record for the session. Replaces the fixture that stubbed `report stop-block`; that stub branch and `METASYSTEM_DEADLINE_RECORD_FAILURE` retire. |
    | Hook bed: StopDeadlineEndsOnlyItsDirectWorker | `metasystem/scripts/agents/supervision-hook-fixtures.sh` | Hang the worker in hook-attempt with a stub that records its own pid and waits for a fixture release file, not for its parent. After the parent returns: the plain deadline allowance and outcome line, the fixed stderr line with the worker pid, that pid gone, the stub pid alive; then release the stub and end it by that pid on every exit path. Replaces the empty-ps fixture; its `ps` shim, `TMPDIR` override, `metasystem-stop-deadline` worker directory and `left running, command line unverifiable` line retire. |
    
    Add TestDeferredRefusalEffectsRequireReceipt in
    `metasystem/internal/goal/turnverdict_test.go` for missing, mismatched and replayed receipts, including session eviction.
    Retain regressions in `metasystem/internal/goal/turnverdict_idle_test.go`.
    Add deferred-mode coverage
    there: two delivered refusals, then the existing third-observation handoff;
    a lost receipt must not advance the counter or consume authorization by inference.
    Extend `metasystem/internal/report/stoppresentation_test.go` for incomplete
    arming and pending-count wording. Extend `metasystem/internal/up/up_test.go`
    for atomic partial and complete arming envelopes.
    
    ## 9. Landing
    
    The build changes these existing production files:
    `metasystem/internal/report/stopblock.go`, `metasystem/internal/report/openwork.go`,
    `metasystem/internal/report/stoppresentation.go`, `metasystem/internal/up/up.go`,
    `metasystem/internal/goal/turnverdict.go`, `metasystem/cmd/metasystem/goal.go`,
    `metasystem/cmd/metasystem/report.go`, `metasystem/cmd/metasystem/up.go`,
    `metasystem/cmd/metasystem/main.go`, and
    `metasystem/scripts/agents/supervision-hook.sh`.
    Add `internal/report/stopdeadline.go`. Change these tests:
    `metasystem/internal/report/stopblock_test.go`,
    `metasystem/internal/report/stoppresentation_test.go`, `metasystem/internal/up/up_test.go`,
    `metasystem/internal/goal/turnverdict_test.go`, `metasystem/internal/goal/turnverdict_idle_test.go`,
    `metasystem/cmd/metasystem/goal_test.go`, `metasystem/cmd/metasystem/up_test.go`,
    and `metasystem/scripts/agents/supervision-hook-fixtures.sh`.
    Add new `internal/report/stopdeadline_test.go` and `cmd/metasystem/stopdecision_test.go`.
    Reuse atomicfile and component evidence; keep new logic in Go.
    The orchestrator runs full-width shared testing and the Opus build read, then
    rebuilds and re-arms seats through landing. This round changes only this page.
    
    ## 10. Moved effects
    
    Recorded 2026-09-15 as the first inventory under goal moved-effects-are-inventoried-in-the-design. It is the sweep build round 8 returned (every effect the Stop worker performed after composing its response, and its owner now) with round 25's per-route hooks-log lines. Code names where the effect is performed before this member lands.
    
    | Effect | From | To | Code |
    |---|---|---|---|
    | The response line `stop response decision=<decision> elapsed=<n>s` in the supervision hooks log | Stop worker | parent emission recorder, after a successful delivery | `metasystem/scripts/agents/supervision-hook.sh` |
    | The elapsed-seconds measurement for that line | Stop worker | parent emission recorder, from the actual post-write time | `metasystem/scripts/agents/supervision-hook.sh` |
    | Staging the response bytes in a temporary file, the provider stdout write, and removing the file | Stop worker | deadline controller: holds the bytes in memory, performs the sole provider write, hashes them into the receipt | `metasystem/scripts/agents/supervision-hook.sh` |
    | Hook attempt completion on success or unavailable presentation | Stop worker | receipt-gated settlement | `metasystem/scripts/agents/supervision-hook.sh` |
    | Hook attempt completion when provider output fails | Stop worker | parent failed-delivery callback, closing the attempt as provider-delivery-failed with no receipt | `metasystem/scripts/agents/supervision-hook.sh` |
    | Narrator digest advancement and its failure warning | Stop worker | settlement result checked against the held report and its matching receipt | `metasystem/scripts/agents/supervision-hook.sh` |
    | Protocol-error cursor advance (main identity, caller process, counts) | Stop worker | settlement, after matching delivery, with the identity carried in the held report | `metasystem/internal/lease/verbs.go` |
    | The full turn-verdict artifact and turn-verdict state | Stop worker | receipt-gated goal settlement | `metasystem/internal/goal/turnverdict.go` |
    | The idle continuation or alarm and its occurrence count | Stop worker | receipt-gated goal settlement | `metasystem/internal/goal/turnverdict.go` |
    | One-use session Stop consumption | Stop worker | receipt-gated goal settlement | `metasystem/internal/goal/sessionstop.go` |
    | Open-work seen state | Stop worker | receipt-gated open-work settlement | `metasystem/internal/report/openwork.go` |
    | The decision or outcome trail line, and the condition line before it when the decision carries an infrastructure condition, on every delivered route | Stop worker (`append_stop_condition`) | parent, per delivered route | `metasystem/scripts/agents/supervision-hook.sh` |

## Recurring findings

Filled by the seat from the records of goal retro-flags-repeated-critique-findings for the goals this design touches, or the words `none recorded`.

none recorded

## Tool-call budget

Maximum delegate tool calls: 80

Stop when this number is reached. List anything the budget did not allow you
to check.

## Page-size ceiling

Maximum page size: 7500 words

Cut a draft that exceeds this ceiling. If cutting would make the page
incomplete, stop and propose a split instead.

## Fresh session

This revision runs in a new delegate session. Use the prior page and critique
only through the context pack above. Never resume a delegate from an earlier
revision.

## Page artifact and return shape

Write the page to: /Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/launchtip/metasystem/plans/stop-hook-never-forces-an-empty-turn-members-design.md

Return only these two lines:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/launchtip/metasystem/plans/stop-hook-never-forces-an-empty-turn-members-design.md
DESIGN: ready (NNNN words)
```

or:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/launchtip/metasystem/plans/stop-hook-never-forces-an-empty-turn-members-design.md
DESIGN: blocked (one line saying why)
```
