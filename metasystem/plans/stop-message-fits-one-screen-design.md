# Design: one task line at Stop, detail in the supervision report

Goal: [stop-refusal-fits-on-one-screen](goals/stop-refusal-fits-on-one-screen.md).
Design only. **Revision 4, 2026-09-13; ready for the build round.**
Wido's ruling this evening, verbatim:

> "make 100% absolutely sure we DO NOT BREAK LOOPS AGAIN. changing stop message is what we need to do; but WITHOUT introducing unwanted stopping again."

This build changes no Stop decision. Both the third-refusal release and
the delegate-job exemption stay as trunk has them. The goal's DONE clause
already says: **"Not a change to what is judged - only to what is printed."**
(`plans/goals/stop-refusal-fits-on-one-screen.md:8`).
Revision 3 exceeded that scope; revision 4 returns to it.

The one human line, full report and delivery work proceed now. The invariant
that a seat keeps working while work can be claimed, including beyond the
runtime's Stop cap, moves whole to
**seat-work-continues-past-the-runtime-stop-cap**, opened in Wido's name on
**2026-09-13**. That goal must land and prove its outer continuation before
closing the two allowances as its own final build item. No third goal is
needed. This lane edits only this page, with no goal or ledger write.

Revisions 1 to 3 remain superseded. Decisions 2 to 7 and their presentation
proofs carry forward except where preserved Stop behavior requires truthful
wording. Under R-97-m1e, there is no third critique round. No decision
remains for Wido. Implementation, code-critique and the named proofs remain
outstanding.

## Grounding

Read at this worktree's HEAD, `9106b52819de28b6de2f0a986325edf4da0935ce`.
Paths below are relative to `metasystem/`. Revision 4 re-read the cited
source and assertion lines at this HEAD, including both preserved allowances.
No implementation is inferred from this page, and no runtime observation
was performed.

| Owner and lines read | What exists, and the seam this design uses |
| --- | --- |
| `plans/goals/stop-refusal-fits-on-one-screen.md:8-10`; `plans/goals/seat-work-continues-past-the-runtime-stop-cap.md:6-15` | The console goal judges nothing new. The continuation goal was opened by human:Wido on September 13. Its references to this build closing the allowances predate the evening ruling and are superseded by this page's ordering; this lane makes no ledger edit. |
| `scripts/agents/supervision-hook.sh:26-41,105-125,269-312,328-365,426-468,519-558` | Engine-free notices, two deadline-parent validators, missing/skewed engine allowances, timeout records and fallbacks. These bypass ordinary emission. |
| Same hook, `893-918,927-985,998-1046,1049-1147,1188-1209,1248-1369` | `stop_block_json`, separate receipt output, `up`, health and digest forming `checkin_tail`, completion/cursor evidence, `compose_failed_stop`, advisor exit, and final block/allow composition. |
| `internal/report/stopblock.go:23-110,121-211`; `cmd/metasystem/report.go:15-75` | Block guidance, 4,000-rune trimming with the reported trim notice, and locked refusal records containing cause/count/timestamps. Trimming already happens before final emission. |
| `internal/goal/turnverdict.go:24-143,264-384,496-693,700-795,901-990`; `internal/goal/project.go:193-222` | One judgment and authorization precedence. `Claimable` contains IDs; `Verdict` loses scan categories, job/run names and the complete claimable projection. A live delegate exempts backlog at 543-546; escalation clears ShouldBlock and BlockSource at 665-667 only if no earlier branch blocked. |
| `internal/goal/verdictrender.go:9-171` | The 4,000-rune display, class/count run summaries, first three green continuations, and `stop-verdicts/<session>.txt`. |
| `internal/report/openwork.go:269-323,425-463`; `internal/report/scan.go:188-205,247-280,298-420` | Plan enumeration and durable seen markers. `clipDetail` slices at byte 200 and can break UTF-8. Open plans carry `FullDetail`; questions and human waits do not. Job role and run display exist at the record readers but are dropped from typed facts. |
| `internal/steward/health.go:31-214,268-295,338-367,380-463,984-1141`; `cmd/metasystem/steward_verbs.go:45-80` | Seventeen role results already have status, reason and remedy. `Line()` prints every sentence; hook preview is read-only. Spend crossings can have status alive **and** a remedy. |
| `internal/narratordigest/digest.go:74-101,261-381` | First check-in shows up to 40 lines; a rewritten story shows its last line. Ordinary pending delivery has no such limit. Cursor advancement checks the exact prefix. The companion change is present and stays untouched. |
| `internal/steward/component_evidence.go:401-433` | `OK/EMITTED` requires the health text in the payload, with decoded matching specifically against `systemMessage`; emission cannot claim client display. |
| `docs/design/turn-verdict-delivery-contract.md:12-45`; `internal/runtimes/runtimes.go:62-89,180-207`; `scripts/agents/adapters/runtime-common.sh:518-539`; `AGENTS.md:34` | Only SessionStart context is declared. There is no Stop capability or invoked Stop mapper; the standing instruction says `goal next`, not read a Stop report. Codex and Devin Stop delivery are declared only. |
| `scripts/enforcement/claude-code-hooks.json:16-27`, `codex-hooks.json:17-24`, `devin-hooks.json:15-22` | All wire the shared Stop hook; Claude additionally wires the receipt hook and an engine-free launcher allowance. |
| `internal/goal/turnverdict_idle_test.go:214-290,292-370,430-467,1241-1283`; `cmd/metasystem/session_stop_test.go:258-371`; `scripts/agents/supervision-hook-fixtures.sh:1535-1581`; `plans/stop-infrastructure-allows-the-seat-to-stop-design.md:55-60` | Six Go tests and the template-backlog row require the allowances this revision preserves. Keep their outcomes, mismatched-marker evidence and the single real intent's held goal, actor and claim epoch. |
| `cmd/metasystem/goal.go:641-703,726-785`; `internal/steward/alert_episode.go:234-262` | Scan and judgment are called once. Idle intent preparation mints a fresh random nonce; the incident recorder reuses an uncleared episode. Repeated intent preparation is not already deduplicated. |
| `plans/supervision-hook-root-design.md:262-285,2968-2973`; `plans/stop-infrastructure-allows-the-seat-to-stop-design.md:43-60,64-80` | The landed public-field clauses demand infrastructure detail in a block's systemMessage, including a reject condition. Decision 3 supersedes only their public rendering and keeps the control, log and refusal-record duties. |
| `internal/runtimes/runtimes.go:1-12,42-103`; `internal/adapter/snapshot.go:17-82` | Distribution expectations are static. Installation/version/configuration observations belong to the existing dated capability snapshot, whose envelope already carries runtime, cliVersion, configHash and capturedAt. InstructionFile already exists. |
| `internal/hostsetup/setup.go:25-84,111-170`; `cmd/metasystem/runtime_setup.go:15-55`; `scripts/agents/hosts/host-common.sh:13-60`, `claude.sh:41-45`, `codex.sh:41-47`, `devin.sh:94-123` | Setup registers repository files and --check is read-only; it owns no human shell profile. Existing provider launch boundaries supply the seam for an installation-bound child PATH. |
| `cmd/metasystem/goal.go:637-703`; `internal/steward/health.go:86-98,338-464` | Nonzero turn-verdict exit selects the hook's degraded path; a completed verdict is exit 0. Alive spend crossings carry a remedy; retro debt is dead/NoAutomaticRemedy yet names an agent action. Neither alone requires either intervention phrase. |

Provider references retained from revision 2's September 13 read: [Claude hooks](https://code.claude.com/docs/en/hooks)
documents human-visible warning/feedback, blocking `reason`, and the
eight-consecutive-continuation cap; [Codex hooks](https://learn.chatgpt.com/docs/hooks)
documents public `systemMessage`, `reason` as a continuation prompt, and
trust of the exact installed hook definition. Documentation is evidence of
provider semantics, not proof that this installation conforms.

## Decisions

### 1. Preserve every trunk Stop decision

For every input, the pair **(blocks or allows, block source)** must be exactly
what trunk at the grounded HEAD produces. Only the text differs. This covers
the goal judgment, hook composition, native emission and degraded exits.
STOP-PRESERVATION below is the headline acceptance obligation.

Keep these branches exactly as trunk has them:

- `internal/goal/turnverdict.go:543-546`: the
  `HasDelegateJobInFlight()` early return in `enforceIdleBacklog`, including
  its digest update and counter reset. It exempts the parent seat's backlog;
  it does not clear an independent block. The live and pending-setup cases
  remain as tested. `internal/goal/project.go:212-222` owns the predicate.
- `internal/goal/turnverdict.go:557-567,665-670`: at attempt three and later,
  `escalateIdleBacklog` clears `ShouldBlock` and `BlockSource` only when
  `blockedBeforeIdle` is false. Successful or failed intent preparation
  still reaches that release. An earlier block retains trunk's source.

Keep the whole existing decision sequence: count/reset and lost-counter
behavior, selection, human authorization and stop fences, independent-block
precedence, advisor/brain exits, authenticated delegate skips, unreadable-ledger
ladder, engine/deadline allowances, escalation, incident and alarm side effects.
The `idle-with-backlog` rule in
`plans/stop-infrastructure-allows-the-seat-to-stop-design.md:55-60` stays.
Unknown judgment still means unknown work, never an empty backlog.

The outer continuation does not exist yet. Revision 3 itself admitted that
a hook cannot enforce continued work once the runtime stops honouring it.
Its consecutive-block cap prevents an absolute claim. Closing these two
allowances first does not make a seat work longer. It makes the seat block
on every attempt until the runtime's own cap ends the session. That is the
failure Wido calls "breaking loops", which he has seen before.

Do not build revision 3's escalation-attempt/result slot in the locked
session Stop-state here. It supported persistent blocking and belongs with
that future change. Trunk calls escalation again at later attempts; do not
deduplicate those calls in this presentation landing or promise that intents
are already deduplicated (`cmd/metasystem/goal.go:726-755`).

The seat's short report summary must describe the preserved branch:

| Observed branch | Report wording |
| --- | --- |
| Unchanged readable backlog, attempts 1 and 2 | `refusal N of 3 for this unchanged backlog; at 3 request steward continuation and allow Stop unless another branch blocks`. Follow with the selected action and other-goal count. |
| Attempt 3 or later | `refusal N reached the bound of 3 for this unchanged backlog`. Retain the actual intent/incident/alarm result. End with `the turn will end` only on an allowance; otherwise `another turn-verdict branch remains blocking this stop`. |
| Delegate exemption | `delegate work is in flight; claimable backlog remains`, when both facts exist. The report still names the selected work and delegate state. |

Keep "of 3" and the conditional "the turn will end". Do not substitute
revision 3's unbounded "refusal N for this unchanged backlog" or
"escalation begins at 3; work remains claimable". A prepared intent proves
a request, not a completed handoff or an actual continuation. Replace the
old promise that the steward will continue the goal with the request wording
above. Keep the existing unreadable-ledger wording and uncertainty evidence.
"This refusal does not repeat for the same work" remains confined to its
existing open-work case; it does not describe counted idle refusals.

### 2. One line for either outcome

Every governed Stop emits one human status line, **at most 256 UTF-8 bytes and
one logical line**, measured after JSON decoding, excluding the transport's
final newline. No carriage returns, line separators, terminal controls or
embedded newlines. The adapter must not duplicate it or print its raw JSON.
Narrow terminals may wrap it; this is a byte/line contract, not a terminal-width
claim. Authenticated delegate skips remain silent.

The shape is `Task: <title>; Stop blocked; status: <read command>`, or the
same line with `Stop allowed`. With no task it starts `No task in flight`;
with only selected backlog it starts `No task in flight; next: <title>`.
That last form may block or allow, exactly as the retained verdict says. Unknown
ownership or unreadable task evidence says `Task unknown`, never "no task".
Waiting, completed and handed-off qualifiers require observed facts; a
prepared intent alone is not a handoff. An allowance is not an all-clear.
For a third-refusal release, use `No task in flight; next: <title>; Stop
allowed; status: <read command>` when only backlog is selected, or
`Task: <title>; Stop allowed; status: <read command>` for a held task.
For the delegate exemption, use that same `Task: <title>; Stop allowed`
form with the owned delegate title when available; otherwise apply Decision
4's title or unknown fallback. Both allow this turn to end while work
remains; neither says work is complete.
Neither allowance alone adds an intervention phrase. Failed intent
preparation, incident/alarm machinery or unavailable evidence adds
`needs supervision repair`; add `needs your decision` only when the existing
repair explicitly requires a human, as Decision 4 specifies. A successfully
prepared intent alone adds neither. Keep both forms within the same bound.

Decision 4 pins title precedence; Decision 5 pins the complete read command.
Reserve outcome, intervention words and the whole command before shortening
only the title at a UTF-8 boundary. No new ledger field or model call.

No health list, narration, retro announcement, plan list, backlog list or
trim notice reaches this line. A classified intervention adds only
`needs your decision`, `needs supervision repair`, or `needs your decision
and supervision repair`; Decision 4 defines their triggers. The full request
and remedy lead the report. Do not print another notice for each trigger.

Exact normal forms (angle brackets mark substitutions, never printed):

```text
Task: <title>; Stop blocked; status: metasystem report stop-status --id <session-key>-<attempt>
Task: <title>; Stop allowed; status: metasystem report stop-status --id <session-key>-<attempt>
```

When an intervention is classified, insert `; <intervention phrase>`
immediately after `Stop blocked` or `Stop allowed`, before `; status:`.
For example, an infrastructure failure accompanying a known block uses
`Task: <title>; Stop blocked; needs supervision repair; status: metasystem report stop-status --id <session-key>-<attempt>`.
Keep that flag in the one line even when all cause/owner/remedy detail moves
to the report. Decision 7 pins the unavailable variants.

### 3. Runtime mapping and the conformance prerequisite

`internal/report` owns compact status, detailed report and reference from
one judgment. `internal/adapter.MapStopOutput` owns host serialization,
invoked by the shared hook through the new `metasystem adapter stop-output
--runtime <runtime> --input-file <presentation.json> --output-file <payload.json>`.
The CLI writes no stdout; the hook validates and emits the file once.
The mapper never judges or spends a refusal. All three enforcement templates
and generated installations must call this path; merely adding an unused
capability or mapper cannot satisfy the design.

For Claude, **`reason` is shared with the human, not private**. Emit precisely:

- Block: `{"decision":"block","reason":"<single compact task/status line>"}`.
  Omit `systemMessage` and additional context.
- Allowance: `{"systemMessage":"<single compact task/status line>"}`.
  Omit `decision`, `reason` and additional context. Never force another turn
  solely to deliver background. Authenticated delegate skips emit nothing.

**Confirmed public-field supersession:** replace the verbatim-display rule
in `docs/design/turn-verdict-delivery-contract.md:12-45`; the infrastructure
`systemMessage` disclosure in `plans/supervision-hook-root-design.md:274-278`
**and its reject condition at 2968-2973**; and the public degraded-notice
clauses in `plans/stop-infrastructure-allows-the-seat-to-stop-design.md:43-49,64-77`.
Those pages no longer require infrastructure detail or a second human field
on a block. Their control precedence remains: infrastructure cannot create
or suppress a seat-actionable block; infrastructure alone allows under the
existing rules. Keep each condition's hook log, stop-condition line,
stop-refusal record, cause, owner, remedy and full report detail. The human
learns that supervision needs repair from the sole line's own
`needs supervision repair` flag (combined with `needs your decision` when
explicitly required), then reads the report's leading repair/action section.
If the report is unavailable, keep that flag and the fixed unavailable
guidance from Decision 7. The replacement reject condition is absence of
that classified flag, required report detail when publication succeeds,
log or refusal evidence; absence of a block's `systemMessage` is required.

Use these same shared-channel shapes for the documented Codex candidate.
No private Stop field is assumed for any runtime. Detailed seat actions and
diagnostics stay in the file. Do not classify an extra field as private
without actual host observation; Claude additional context also continues
the conversation and cannot solve allowance delivery.

`runtimes.Declaration` remains a **static distribution registry**. Add only
`ExpectedStopDelivery string`: `shared-reason-v1` for Claude/Codex candidates,
empty for an unknown contract such as Devin. Reuse its existing
`InstructionFile` to find the instruction entrypoint; do not duplicate it
in a Stop declaration. `internal/adapter` owns the actual envelope mapping
and validation. A declaration is an expectation, never installation evidence.

Put observed delivery in **`capabilities.stopDelivery` in the existing
dated adapter capability snapshot**, written by
`internal/adapter/snapshot.go:WriteCapabilitySnapshot`. Reuse the snapshot's
`runtime`, `cliVersion`, `configHash`, `configKeyHashes`, `capturedAt` and
exclusive dated sequence. No second registry or evidence file format.
The new object has `schemaVersion:1`; strings `envelope`
(`shared-reason-v1` or `unverified`), `blockField`, `blockValue`,
`blockTextField`, `allowTextField` (the four literals above for that envelope);
`humanVisibleFields` (string array); `duplicateBehavior` (`single`,
`duplicate`, `unknown`); `reportReadRoute` (`standing-instruction-and-command`
or `unknown`); `instructionHash`, `trustProbe`, `observationArtifact`
(SHA-256 of the loaded instruction bytes, trust-probe evidence reference,
and the real-host observation artifact respectively; empty if unobserved);
`launchBinary` and `seatCommandBinary` (resolved absolute paths, empty if
unobserved); `continuationLimit` (positive integer or null); and `level`
(`unobserved`, `emitted`, `observed`). A null limit means unknown, never
unlimited. Unknown mapping uses empty field strings and an empty visible
field array. The probe records actual host/config/instruction and locator
evidence; the static expectation must never prefill an observation.

`emitted` proves only the installed invocation and envelope. `observed`
requires every applicable STOP-HOST display/control/read/trust/cap observation
in the referenced artifact and matching installed hashes. It proves only
the finite contract, even with a known cap. A changed host version,
configuration or instruction hash invalidates reuse of the observation.
Missing or stale evidence leaves delivery unobserved/degraded, not silently
conforming. Unsupported mapping returns an explicit error to the existing
degraded path, not a guessed Devin envelope.

| Runtime | Envelope/control and human fields | Duplication and report-read route | Installation/trust and continuation limit | Evidence level at this HEAD |
| --- | --- | --- | --- | --- |
| Claude | Candidate shapes above; block reason and allowance systemMessage are public | Two populated visible fields produce two notices; new standing read instruction required | Inspect effective Stop registrations, generated launcher and loaded CLAUDE.md; fire block and allow; documented cap 8 | Existing hook emission only; new single-line/read behavior unobserved; absolute no-stop claim contradicted by cap |
| Codex | Documented decision/block plus reason continuation; systemMessage public; same compact shapes proposed | Treat reason as shared; inspect actual warning/prompt display for duplicates; new AGENTS.md read instruction required | Project and exact hook definition must be trusted; inspect effective `/hooks` definition/hash and fire both outcomes; continuation limit unknown | Declared only; new mapping, trusted firing, report read and cap behavior unobserved |
| Devin | Accepted Stop envelope, blocking field and public fields unknown | Duplication and native continuation/read route unknown | Inspect effective installation and provider trust mechanism, then observe; limit unknown | Declared only; no conforming mapping may be claimed |

The following is a **new instruction to install**, not a claim about today's
AGENTS.md. Add it to the canonical contract's turn-end instruction and make
the generated CLAUDE.md/other runtime instruction entrypoint load that owner:

> On a blocked Stop, run the exact `metasystem report stop-status --id ...`
> command in the notice and read its report before the next work action or
> another Stop. Follow its seat action within your existing authority. If
> it is unavailable, keep any established block, report the read failure,
> and use `goal next` to recover current work; never infer that no work
> remains. An allowed Stop adds no report-read turn; the file is available
> for the next ordinary turn. A free seat fetches and claims the ready goal;
> a held goal is continued without another claim.

Instructions request a read; they do not mechanically prove it. Before a
runtime may claim this delivery contract, pin host version and effective
configuration, observe its human transcript with all registered hooks,
prove the locator works from the seat's actual cwd, and observe a blocked
seat read that exact report and perform its named action. Observe an
allowance that creates no continuation. A stub adapter harness proves only
serialization. The gate also records trust/disabled-hook behavior, read
failure, control precedence with other hooks, and the continuation cap.
Source tests cannot replace this gate.

**Implementable now:** compact Claude/Codex candidate envelopes, complete
reports and lookup, and new standing instructions, with every goal-engine
and hook Stop decision preserved. None is shipped by this page.
**Not enforceable from inside the current Stop hook:** an absolute guarantee
that the runtime never stops. The host stops honouring the hook after a
fixed number of consecutive no-progress blocks (Claude's documented limit
is eight). A higher finite cap does not establish that guarantee. Unknown judgment,
untrusted hooks and report-read failures also preclude a full-contract claim.

**Ordering under Wido's evening ruling, 2026-09-13:** build the console and
delivery milestone now with both allowances preserved. The separate goal
**seat-work-continues-past-the-runtime-stop-cap**, opened in his name today,
owns the continued-work invariant and all its obligations, including
STOP-CONTROL and STOP-ABSOLUTE. Its outer mechanism must land and be proved
before that goal's own final build item closes the two allowances.
This is not a third goal or an unassigned follow-up.

That goal owns continuation outside the provider's turn loop, human-stop
authority, no duplicate claims, budgets and failure recovery. It must also
account for unknown judgment, disabled/untrusted hooks and unavailable
guidance without treating them as proof that work is absent. The full
transfer is recorded below the fixtures. This build neither implements nor
discharges it. No renewed human decision or third design read is needed.
Codex/Devin still need their named conformance evidence; silence never
upgrades them. All future claims must state the supported continuation range.

### 4. One versioned, uncut presentation input

These are proposed interfaces, absent at this HEAD. `cmd/metasystem/goal.go`
adds `--facts-file <judgment.json>` to the existing `report turn-verdict`
invocation. It performs its scan and locked judgment once, captures the
projection described below before any display bound, and atomically writes
that file. The ordinary verdict JSON remains on stdout with its existing
schema. **When judgment completes but the facts-file writer fails, exit 0
with the unchanged Verdict JSON plus newline on stdout.** Write the
facts-write diagnostic only to captured stderr; leave the facts file
absent, with no partial or stale file. Use a fresh per-attempt destination
that must not already exist; flag/path validation precedes judgment.
Remove failed temporary publication bytes. Do not insert the diagnostic in
the Verdict, return a nonzero presentation exit, or call `TurnVerdict` again.
Existing judgment/encoding failures retain their existing nonzero contract.

The hook parses the successful stdout verdict and retains its exact
`shouldBlock` and block source. A missing facts file marks the judgment
projection unavailable while retaining the completed control judgment:
`judgment=null`, `control.judgmentAvailable=true`, and a typed unavailable
entry with `section=judgment-facts`, captured diagnostic and
`supervisionRepair=true`. This is distinct from failed judgment, where
`judgmentAvailable=false`. Emit the unchanged block/allow envelope with
`Task unknown; Stop blocked; needs supervision repair; Status unavailable.`
(or `Stop allowed` for the completed allowance). Retain diagnostics in the
attempt/log evidence where writable; publish no full-guidance pointer and
advance no delivery cursor. STOP-FACTS-WRITER-FAILURE proves this wire and
one judgment/count even when the writer fails after a real blocking result.

The hook collects that file and its other command results into one
`StopPresentationInput` v1 file. `internal/report` owns its schema and renderer;
the command boundary composes it. The goal engine owns the frozen judgment
projection and imports neither steward nor adapter. Invoke exactly:

`metasystem report stop-present --root <resolved-installation> --input-file
<input.json> --output-file <presentation.json>` (one shell command).

All flags are required, paths are absolute, and inputs travel through files.
On success the command atomically publishes the Markdown first, then the
presentation result; stdout/stderr are empty and exit is 0. Exit 2 denotes
invalid flags/schema/identity; exit 1 denotes publication failure. Diagnostics
go to captured stderr, never directly to the human. Neither failure changes
the retained control verdict. Reject unknown schema versions, trailing JSON,
wrong types and identity mismatches before publishing anything. Arrays are
present even when empty; an unavailable observation is null with a named
entry in `unavailable`, never an invented empty successful result.

| Input field | Exact v1 type and content; owning producer |
| --- | --- |
| `schemaVersion` | Integer, exactly 1. |
| `identity` | Object with string fields `installation`, `runtime`, `session`, `sessionKey`, `attempt`, `mainId`, `machine`, `lineage`, `observedAt`; integer `claimEpoch`. Timestamp is UTC RFC3339Nano; paths/root and holder fields come from resolved installation/seat evidence. Unavailable holder strings are empty, epoch 0, and ownership is `unknown`. |
| `judgment` | Null on unavailable judgment or facts-file failure (distinguished by `control.judgmentAvailable` and `unavailable`), otherwise the immutable facts-file object: `schemaVersion:1`, `identity` (strings `installation`, `session`, `mainId`, `observedAt` from the existing command arguments/resolution and evaluation clock), `verdict` (the full existing Verdict JSON), `fullDisplay` (uncapped composed text), `scan`, `work`, `ownership`, `actions`, `refusal`. Root/session/main identity must match the outer input; evaluation time is preserved separately from collection time. This is a record of one evaluation, not permission to evaluate again. |
| `judgment.scan` | Object with arrays `open`, `templateUnfilled`, `waitingOnHuman`, `stalePlans`, `busy`, `questions`, `drafts` of `ScanItem`; `openWorkWarnings`, `unreadable`, `runUnreadable` of strings; `jobs` and `runs` of the extended facts below. Preserve scanner order, membership and seen evidence. |
| `ScanItem` | Object: strings `kind`, `id`, `detail`, `fullDetail`, `lineDigest`, `sourcePath`, `ownerMainId`, `requestedAction`; booleans `previouslyRefused`, `humanRequired`. `fullDetail` and `requestedAction` are uncut. Unknown ownership is empty; the item remains in the report. |
| Job/run facts | All current `JobFact`/`RunFact` fields, lower-camel JSON keys, plus strings `title`, `role`, `goalId`, `startedAt`, `sourcePath`, `sourceDigest`, `ownership` (`owned`, `other`, `unknown`). Job title uses the joined goal intent when available, otherwise its existing role label with separator hyphens/underscores replaced by spaces; run title uses `Record.Display`. Empty values mean missing source data and are recorded as such. No record schema or ledger edit is needed. |
| `judgment.work` | Object: `readSucceeded` boolean; arrays `claimed`, `landing`, `claimable` of `GoalFacts` with `id`, `intent`, `nextStep`, `revision` strings; `selected` is one such object or null; `selection` is `held`, `claimable`, `none`, or `unknown`; `refused` retains goal ID/cause objects; `inFlight`, `nonTerminalJobs` string arrays; `queued` integer and `goalFree` boolean. Capture IDs **and intents** from the goal records already read under the judgment lock. Retain goal-engine ordering; never select by report scan order. |
| `judgment.ownership` | Object: `state` (`owned`, `none`, `unknown`), `goalId` string, `evidence` string. Only the resolved holder and joined own work can establish ownership; merely sharing a machine name is insufficient. |
| `judgment.actions` | Ordered array of objects with strings `kind`, `targetId`, `instruction`, `command`, `owner`, `restriction`; booleans `humanRequired`, `supervisionRepair`. Empty command means a named manual step, not a made-up verb. Exact instructions/commands precede background detail in the report. |
| `judgment.refusal` | Object with `class`, `causeCode`, `component`, `detail`, `remedy`, `blockSource` strings (empty absent), `occurrence` integer, `countSpent`, `idleRefusal`, `humanStopConsumed`, `humanRequired`, `supervisionRepair` booleans; `escalation` object containing `intentId`, `incidentId`, `alarmDetail`, `detail` strings and `intentPrepared`, `humanRequired`, `supervisionRepair` booleans. No clipped notice is parsed to recover these facts. |
| `control` | Object: `shouldBlock` boolean, `blockSource` string or null, `class` and `causeCode` strings, `judgmentAvailable` boolean. Match the retained verdict exactly; unavailable judgment uses the existing infrastructure result. This is the only mapping input that controls block/allow. |
| `health` | Null or the `HookHealthPreview` v1 object in Decision 6, captured from one preview call. |
| `digest` | Null on failed read, otherwise object with string `mode`, `text`, `sourcePath`, `cursorPrefix`, `sha256`. Preserve exactly the collected pending bytes and prefix; hash UTF-8 bytes. No new narrator policy. |
| `receipt`, `arming` | Each null or object with integer `exitCode` and string `stdout`, `stderr`. Arming also carries `components`: array of string `name`, `outcome`, `detail`, `remedy`, `owner`, `restriction`; booleans `noAutomaticRemedy`, `humanRequired`, `supervisionRepair`; and string `aggregate`. Keep original bytes and typed results from the same invocation. Receipt classifications travel in `notices`, never by parsing its stdout. |
| `notices` | Ordered array of objects: strings `source`, `class`, `causeCode`, `component`, `detail`, `remedy`, `owner`, `restriction`; booleans `humanRequired`, `supervisionRepair`, `noAutomaticRemedy`. Covers watchdog, holder protocol, brain post, hook evidence/log failures and durable wait recovery lines. |
| `unavailable` | Array of objects with strings `section`, `cause`, `remedy`, `owner`, `restriction`; booleans `humanRequired`, `supervisionRepair`. Required for every null observation or failed auxiliary capture. An empty successful digest remains distinguishable from a failed digest read. |

The producer must extend the existing facts, not serialize today's lossy
`Verdict` as if it were enough. In `internal/report/scan.go`, populate
`FullDetail` before clipping **for every category**, including questions,
human waits, drafts, busy jobs/runs and templates; retain question Wants as
the requested action. Extend job/run facts at their current record reads
with role, goal join and display text. Fix `clipDetail` to cut at a valid
UTF-8 boundary within 200 bytes, retaining the legacy detail bound. ASCII
legacy output stays byte-identical. The new report consumes `fullDetail`;
fixing the clip alone cannot recover missing tails. Keep seen signatures,
readiness, budgets and scan membership unchanged. Capture command-added
durable wait lines before display trimming too.

Title precedence is deterministic: an owned nonterminal job, then an owned
active run, then the seat's held goal intent, then the engine-selected
claimable goal intent. For multiple jobs/runs in the same tier, choose the
earliest nonempty `startedAt`, then ID; include all others and their count in
the report. Prefer a job's joined goal intent, falling back to its role; a
run uses its display. A missing title says `task name unavailable` and keeps
the record ID in the report. Other seats' work never supplies this title.
A selected but unclaimed goal is labelled `next`, not in flight. Proven
empty owned work and no selection produce `No task in flight`; incomplete
ownership/scan evidence produces `Task unknown`. Normalize whitespace and
controls only in the compact title; preserve source text in the report.

Derive intervention phrases **only from explicit typed classifications**.
`needsYourDecision` is the OR of `humanRequired`; `needsSupervisionRepair`
is the OR of `supervisionRepair` across the frozen scan items/actions/refusal/
escalation, `health.interventions`, arming components, notices and unavailable
entries that declare the respective field. Producers own classification at
the branch that establishes the condition; the renderer only combines the
booleans. A missing declared classification is invalid v1 input, not an
invitation to parse prose. Never infer either flag from `NoAutomaticRemedy`,
a nonempty remedy, a generic dead/unknown status, an owner string, or words
such as "Wido" in a suggested optional action.

| Producer and established condition | `humanRequired` | `supervisionRepair` |
| --- | --- | --- |
| Scanner: an actual open human question or `WaitingOnHuman` item | true | Absent on ScanItem; false for aggregation. Preserve complete Wants/wait and answer route. |
| Goal/action producer: an existing rule requires a human decision/authorization before the named action, including an actual human-only or agent-free-terminal recovery | true | true only when that required action repairs failed supervision; ordinary approval/wait is false. |
| Hook/arming producer: infrastructure failure, failed arming, unavailable health/facts/report/locator, hook log/record failure or escalation outcome unknown | false unless the existing repair requires the human | true; keep exact cause, repair owner and restrictions. |
| Health producer: failure of supervision machinery or its evidence, established at its failing check | false unless its required repair is human-only | true. Mark actual failed machinery/evidence branches; do not classify all non-alive role results this way. |
| Retro debt only, with an agent-run retro and receipt action (`health.go:448-463`) | false | false, even though status is dead and `NoAutomaticRemedy=true`. |
| Alive spend crossing only (`health.go:421-445`), including a suggested ceiling change | false | false. Keep crossings/reason/remedy as report attention only. |
| Other ordinary seat work, completed actions and healthy observations | false | false. A distinct classified fact can independently require either intervention. |

Typed fields are presentation metadata only: no change to health status,
alert/remedy policy, automatic recovery eligibility or Stop control. Spend
crossings remain report-only unless a separate explicit fact requires a
human action; a suggestion to raise a ceiling is not such a requirement.
Each true flag retains its full requested action, source, owner and
human/agent-free-terminal restrictions at the start of the report. Both
negative fixtures and all four flag combinations are required below.

The presentation result is a v1 JSON object with `schemaVersion:1`, the
unchanged `identity` and `control`, `humanLine` string, the two derived
booleans, and `report` object (`id`, `path`, `readCommand`, `sha256` strings).
The report digest is SHA-256 of the published Markdown bytes. It contains
no runtime envelope. The mapper accepts this exact version and validates
the line bound and report identity before producing a host payload.

### 5. An immutable report with a cwd-independent read command

Use a new human-readable report in the existing directory:
`artifacts/agents/supervision/stop-verdicts/<session-key>-<attempt>.md`.
`session-key` is the 64 hexadecimal digits of SHA-256 over runtime plus the
normalized session encoded as the two-string JSON array `[runtime,session]`
without whitespace; `attempt` is a freshly reserved
32-hex identifier. Exclusive reservation prevents overwrite on a collision.
Storage remains relative to the engine-resolved installation. That relative
path is **not** a cwd-independent locator: no adapter currently resolves such
a link, and hosted payload cwd can be elsewhere. Print the complete read-only command
`metasystem report stop-status --id <session-key>-<attempt>` instead. The ID
is 97 ASCII bytes, a 64-hex key, hyphen and 32-hex attempt; the whole command
is 132 bytes regardless of installation depth. It names an exact immutable
snapshot, never a global `--last` or mutable session file.

The new CLI accepts required `--id` and optional `--root`. Without `--root`,
resolve the real executing binary (following symlinks), require its installed
`<installation>/bin/metasystem` layout, then apply the existing
`stateroot.RootForCandidate` validation to that installation. Do not derive
the root from cwd, walk parent repositories, consult another checkout, or
trust a root supplied in the hook payload. An explicit `--root` uses the same
validator. Lookup accepts only the exact ID grammar, forms the single path
under `stop-verdicts`, checks embedded identity, and refuses escapes/symlinks
outside that directory. No global registry or report index is introduced.

**Provider launch binding:** `scripts/agents/hosts/host-common.sh` owns the
shared child-environment plumbing used at the existing Claude, Codex and
Devin launch boundaries. Resolve and validate the installation before
launch, require its real `bin/metasystem`, and prepend that installation's
absolute `bin` directory to the provider child's inherited PATH:
`PATH="<resolved-installation>/bin:<inherited-PATH>"` (omit the colon when
the inherited PATH is empty). Pass it as an environment entry, never shell
evaluation of an interpolated string. Resolve the provider executable to
its absolute path before this prepend. Do not substitute `METASYSTEM_BIN`
or hook-payload roots for the validated installation. Every claimed launch
route must use this binding, including resumed turns; a direct interactive
provider launch may use the same per-process `env PATH=... <absolute-provider>`
form documented by setup. A route that does not establish it remains
unverified. Inherited PATH is only a launch prerequisite: if the provider's
actual command tool changes it, STOP-HOST must detect that lost binding.

**Read-only probe:** extend `runtime setup` with optional
`--stop-status-id <id>`, legal only with `--check`. The exact probe is
`<installation>/bin/metasystem runtime setup --repo <checkout> --runtimes
<runtime> --check --stop-status-id <id>` (one command with shell-quoted
absolute paths). `internal/hostsetup` adds `StopStatusID` to its options.
It checks registration as today, resolves `metasystem` from the caller's
actual PATH without fixing that PATH, follows symlinks, and requires the
resolved binary to equal the target installation's real binary before
executing it. Then run the printed `report stop-status --id <id>` argv and
validate the returned exact report identity. It must not use `--root` to
mask a wrong binding. Success is exit 0 with a `STOP_LOCATOR_READY` row
naming the resolved binary and ID; a missing/wrong executable or unreadable/
mismatched report is exit 1 with stderr diagnosis; invalid flags/ID or the
new flag without `--check` is exit 2 before checks. No probe writes files,
snapshots, reports, counters or configuration, fires a hook, launches a
provider, or changes the environment. Ordinary `--check` without an ID
retains registration-check semantics and makes no report-read claim.

Run that probe inside the provider's actual command environment, using an
already-published report, from nested and external cwd and through a
symlinked launch binary. Run the **printed command** too; match the exact
Markdown bytes. Store this observed binding with the dated adapter evidence,
not in the declaration or inside the read-only probe. A wrong binary or
missing command is degraded locator delivery, even when the report exists.

**Human-shell guarantee is explicitly narrowed:** setup never edits a
human's shell profile, rc file, aliases, functions or global PATH. The
132-byte bare command is guaranteed only in a verified provider command
environment or a human shell already passing the same probe. A fresh or
unbound human shell is not covered. Setup diagnostics and the report give
the shell-quoted absolute recovery command
`<installation>/bin/metasystem report stop-status --id <id> --root <installation>`
and the full report path; adding `--root` to an unavailable bare executable
is insufficient. This recovery text is outside the bounded Stop notice.
Retain the 256-byte bound; do not insert arbitrarily deep absolute paths
into it or claim automatic human-shell installation. No profile mutation
or further human authorization is part of this build.

Successful `stop-status` prints the exact Markdown to stdout, no stderr,
exit 0; invalid ID/flags exit 2, missing/expired/unreadable/root mismatch exit
1 with a diagnostic on stderr and no report output. It is read-only: no
judgment, refusal, lease, cursor, scan or claim. All lookup and binding work
is part of the implementation/conformance obligation, not proof of a read.

Write atomically before emitting the pointer. The report starts with the task
and **Stop blocked/allowed**, observation time, and any action needed from the
human. Next come seat actions with their commands, then background counts,
then complete diagnostic sections: original turn verdict, uncut open-plan
steps, complete claimable list from the judgment, every health role, the
pending digest, receipt result, `up` component output, watchdog/protocol and
other notices. Include causes, remedies, refusal occurrence, block source,
installation, runtime/session/attempt identity, and links to the existing
verdict and refusal records. Mark missing sections as unavailable. "Full"
means all facts collected for this Stop, not copying the entire historical
narrator log; link that log too.

After the heading, one `metasystem-stop-report-v1` HTML comment carries the
exact `identity` JSON from the presentation input. Lookup validates its
installation and recomputed session key/attempt against the resolved root
and filename; completion additionally checks the externally retained digest.
The comment is report metadata, not a second human notice or a self-hash.

Keep `stop-verdicts/<session>.txt` and `stop-refusals/<slug>.json` at their
current paths with their current writers and record schemas. Their decisions,
causes, counts and side effects stay intact; only presentation text moves.
The new report supplements them using Decision 4's frozen facts, not the
already-trimmed payload or a reread of a mutable per-session verdict file.
Keep all required seat commands intact, followed by summarized background
and the complete captured detail. Nothing long returns to a public field.

Rotate only these new Markdown snapshots: retain the newest 20 per
runtime/session **and** every snapshot younger than 24 hours. Prune older
ones under a per-session presentation lock after successful publication;
pruning failure is diagnostic and never affects Stop. No rotation touches
the existing verdict, refusal, source or control-state files. Parallel Stops
get different paths; a later Stop cannot overwrite the report just emitted.

### 6. Actionability and the exact health interface

| Class | Presentation and clearing action |
| --- | --- |
| Current seat's unwatched work | Action first: the existing exact `metasystem run watch --id <id> --root <installation>` or job watch command. Resolve parameters from owned records. Concluding a run remains a separate deliberate act. |
| Current approved goal or claimable backlog | Keep the goal's actual next step. A free seat uses `metasystem goal next --machine <seat> --fetch`, then the normal claim command for the returned ready goal. A held goal is continued without another claim. Do not treat any plan reminder as approval. |
| Open plan reminder | Include its full step and existing instruction to perform it or record the true wait in that plan. There is no universal clearing command: identify the file to edit, never invent a verb or mark work done for display purposes. Other seats' and unattributed pages are background for this reader; do not change their scanner membership, seen markers or judgment effect. |
| Open question or human wait | Name the question/plan and preserve its complete Wants/wait text. Set the decision flag; show who must answer and the existing response route or file to edit. Never turn a wait into approval or truncate its requested action. |
| Dead/unknown health, arming and record failures | Name the role/component, cause, owner and exact supplied remedy. Preserve "agent-free terminal", stop-fence and human-only restrictions. A diagnostic reread is labelled as a reread, not a repair. |
| Retro due/debt | Preserve `scripts/receipt.sh check`, the retro procedure and its completion receipt command. A receipt is owed after doing the retro, not a command to silence it without work. |
| Healthy roles, other plans, past runs and narration | Summarize by class/count: roles alive/dead/unknown; other/unattributed pages; runs red, ended unknown, hung, unsupervised, or green without a continuation; digest lines delivered. Keep explicit continuations actionable. Do not infer "oldest" dates from an ID. |

Add exactly `metasystem health --repo <checkout> --metasystem-root
<installation> --hook-preview --format=json`. `--format` defaults to `text`;
`json` requires `--hook-preview`, and unsupported values or `json` without
preview exit 2 before evaluation, with empty stdout and an error on stderr.
Unflagged preview and explicit `--format=text` remain byte-identical to
today's `verdict.Line()` plus newline. Ordinary health behavior is unchanged.

JSON stdout is exactly one `HookHealthPreview` object plus newline. Its
schema is `schemaVersion:1`, `exitCode` integer, `line` string equal to that
same verdict's unabridged `Line()`, `interventions` array as specified below,
and `verdict` object with the existing
`HealthVerdict` JSON fields: `schema`, `observedAt`, `observation`, `aggregate`,
`roles`, `shouldAlert`, `findingDigest`, and optional `stopped`, `stopPhase`,
`stopUnresolved`. Each role retains `role`, `status` (`alive`, `dead`,
`unknown`), `reason`, optional `remedy`, `durationMillis`,
`consecutiveUnknown`, `consecutiveFailures`, `failureEscalation`,
`noAutomaticRemedy`. Existing omitted optional values retain their zero
meaning. Do not export private observation `State` or `Spend`; any spend
attention needed by this report must be preserved in the role reason/remedy.

`interventions` contains exactly one object per role in verdict order:
`role` string, required booleans `humanRequired`, `supervisionRepair`, and
strings `owner`, `restriction`. The same health evaluation produces these
typed classifications at the condition's source under Decision 4. They
travel in the preview projection; keep the existing `HealthVerdict` JSON,
`Line()`, finding digest, status and recovery behavior unchanged. Join by
role, reject missing/duplicate/unknown roles, and take the exact action from
that role's remedy (empty only when no action is required). An unavailable
preview produces a typed unavailable entry classified as supervision repair.

Evaluate `PreviewHealthAt` once, then derive both `line` and `verdict` from
it. Do not call `ObserveHealth`, advance observations, or write alerts.
Return the same `ExitCode()` in either format: 0 healthy, 1 dead, 2 unknown
(with existing precedence/stopped semantics). A valid object at exit 1/2
is health data, not a parse failure. Normal diagnostic content stays in the
object; stderr is empty. Encoding/command failure returns no partial JSON,
stderr diagnostics, and exit 2; the caller marks health unavailable.

Health summaries are rendered in `internal/report` from this typed data;
keep `HealthVerdict.Line()` unchanged. The report summary says
`Health: all 17 roles alive` when applicable; otherwise name every dead or
unknown role with its remedy and count the healthy roles. An alive spend
role with a crossing/remedy remains an attention item; "alive" is not enough
to discard it. Preserve stopped-state qualifiers.

Project these health fields across the command boundary as data; do not add
a dependency from the goal engine to the steward or adapter packages.

Open-work and idle-backlog summaries likewise use typed facts in the report
renderer, not shell regular expressions. The goal engine provides the
claimable count/list and selected goal from its existing locked judgment.
It still owns `IDLE WITH BACKLOG`, refusal count, escalation and precedence.
The short report summary names the selected action plus the number of other
goals; the detail section retains every name. No scan-membership, readiness,
budget, ownership or narrator rewrite-policy redesign belongs in this build.

### 7. Cover all exits and keep evidence honest

Replace final Stop composition at `checkin_tail`, `compose_failed_stop`,
`stop_block_json` and `emit_stop_payload` with the shared presentation result.
Keep refusal recording separate from rendering so the composer no longer
extracts an infrastructure notice by splitting a bounded `systemMessage`
suffix. Preserve its cause, count and arming-result bytes before rendering.
Remove the old trim from this Stop path only; other callers retain their
existing contract, except the UTF-8 clipping correction in Decision 4.
Advisor and unavailable-verdict exits use the same renderer. Presentation
never changes an established Stop decision or its source.

Fold the receipt check into this collection and remove Claude's separate
Stop receipt registration, including its generated installed form. The
receipt event can remain callable directly, but cannot print a second line
for one registered Stop. Do not change `surface_json` globally: SessionStart,
the brain boot packet/notices, and SessionEnd retain their contracts.

Both deadline-parent validators must accept the new adapter envelope and
retain their strict single-object/type validation. Remove the special
requirement that a published block contain `HEALTH` in its human field.
A completely published valid block still wins the deadline race, including
when completion logging stalls. The parent never creates a new block.
Presentation stays inside the existing 57-second worker/60-second overall
budget; a report failure earns no extra deadline or retry loop.

The parent and launcher also use the verified compact runtime contract.
Their engine-free mapping must be generated from the same adapter-owned
envelope selected by the static expectation,
with no dependency on invoking a missing engine. An unknown runtime mapping
cannot be advertised as working through a guessed Claude fallback.
When the engine and resolved root are available, persist their exact original notices in the
report and retain existing condition logs/refusal writes. On staging,
root-resolution, rendering or storage failure, preserve trunk's control
result. Use the engine-free allowance below only on existing allowance
paths; after a completed judgment use the retained-decision form below it.
Do not publish a nonexistent or stale path. Each fallback has
the same 256-byte bound and names its cause and remedy; combined deadline,
log and record failures use one fixed combined form. No dynamic unescaped
diagnostic text enters engine-free JSON. Keep no-world/missing-engine paths
free of guessed state writes. This unavailable case replaces their current
only human signal; it cannot promise an external file when none can be made.

The engine-free form is `Task unknown; Stop allowed; needs supervision repair;
<fixed cause><qualifiers>. <fixed remedy>. Status unavailable.` with spaces,
never line breaks. `needs your decision and supervision repair` replaces
the repair-only phrase only on a branch whose existing recovery explicitly
requires a human. This is a fixed branch classification, never inferred from
its remedy string. Causes
are `engine missing`, `engine does not answer path state-root`,
`hook-bootstrap-failed`, `payload staging failed`,
`stop-hook-output-was-unreadable`, or `stop deadline expired`.
Missing/skewed engines use `Rebuild bin/metasystem`; the others use
`The steward must restore supervision`. Append the fixed cause qualifiers
`record update failed`, `condition log failed`, or `no resolved checkout`
when those failures occurred, counting them within the same 256 bytes.
Qualifiers append in the order listed, each prefixed by `; `, before the
cause's period. The longest combined form, including the combined human/
repair phrase, must be pinned by a fixture. Engine-backed report
failures retain diagnostics in the existing attempt/log evidence where
writable, with no claim that the seat received that missing detail.

Facts-write, report-write, locator or mapper failure never changes an
established control result. On mapper error, use the adapter-owned fixed
native fallback for the retained decision; never turn a completed block into
an unreadable-worker allowance. Keep its source in retained evidence.
Retain the native block with the exact form
`Task: <title>; Stop blocked; needs supervision repair; Status unavailable.`
or the allowance with `Stop allowed` in its sole visible field. Use
`Task unknown` if the task is not established; apply the same typed combined
intervention phrase and title-only shortening as Decision 2. The new
standing instruction supplies the `goal next` recovery path. If no detailed
channel or readable report exists, full seat guidance is **not delivered**;
mark delivery degraded, never successful. The hook cannot promise both full
detail and one line under that failure, nor can another report read
defeat the host's continuation cap. This is the milestone limit accepted in
Decision 3 and carried by the separate continuation goal; it never clears
an established block or permits a second human notice.

Change `CompleteHookAttempt` to verify delivery against the exact published
report referenced by the emitted payload's sole text field. Check report
identity and content digest, including the health snapshot; finding health
bytes somewhere in arbitrary JSON is insufficient.
The presenter returns the report path and SHA-256 as internal metadata;
completion receives these beside the actual emitted payload file. Adapters
emit only host-supported fields; validation parses the complete `stop-status`
command, resolves its exact ID through Decision 5's lookup under the bound
installation, and verifies that report's hash. A random matching path or
health bytes elsewhere in JSON are insufficient.
`OK/EMITTED` means emitted guidance/reference with retained evidence, never
"human saw it". Advance the Stop digest cursor only after that report is
durably published and its reference successfully emitted with the verified
lookup binding. It records availability, not reading.
On report/emission failure leave that cursor unchanged. Protocol advancement
has the same delivery condition. Do not add a turn to acknowledge an allowance.

## Headline obligation: STOP-PRESERVATION

The whole control risk of this build is accidental change. Required
observation: **for the same input, the Stop decision and its source equal
trunk's** across this matrix. No presentation-path failure may clear an
established block or create one.

| Paired case | Required comparison |
| --- | --- |
| Real seat-actionable block | Same block and source, including an arming failure beside the block. |
| Idle backlog, attempts 1, 2, 3, 4 and later | Same block/block/allow sequence without another blocking branch; same source at every attempt. Pair successful and failed escalation, changed digest, lost counter and a separate block at escalation too. |
| Ordinary allowance; delegate in flight | Same allowance and absent block source. Cover live and pending-setup delegates, plus a separate block that the exemption must leave intact. |
| Unreadable ledger | Same uncertainty ladder at attempts 1, 2, 3 and later, including count-write failure. No empty-backlog inference. |
| Missing engine; skewed engine; deadline expiry | Same degraded allowance and absent block source when trunk allows. Also expire after a completely published valid block: that block still wins. |
| Failed facts write; failed report write; failed mapper | Inject each after a real completed block and after a completed allowance. The result and source must survive unchanged through emission; no second judgment or refusal spend. Include locator failure and invalid mapper output. |

The builder supplies passing trunk control tests with their decision
assertions unchanged, plus a table of paired trunk/candidate runs. Record
the baseline HEAD, candidate identity, exact commands, input/state identity,
attempt, injected failure and phase, both decisions, both sources and links
to the raw verdict/payload/log evidence. Use the same initial ledger,
session state, actor, clock and controlled liveness for each pair. Compare
sequences too, so changed counters cannot hide behind a matching first Stop.
For new presentation fault sites, use trunk's ordinary output for that same
control input and inject the fault only in the candidate's presentation.
The native envelope proves block/allow; the retained judgment or existing
fallback branch proves its source. Record null where no source exists;
never invent one from the repair phrase.

`internal/goal` owns judgment equality; the goal CLI, report/adapter boundary,
shared hook and both deadline parents own its preservation through delivery.
STOP-FACTS-WRITER-FAILURE, STOP-INFRA-DISCLOSURE and the degraded-fallback row
below are parts of this obligation, not separate excuses to change control.
The critic checks the paired evidence and the test diff. A green presenter
test or a table without reproducible inputs and raw results is insufficient.

## Fixtures and proof obligations

Implement focused Go cases plus the affected existing hook rows. No fixture
bed was run for this design. **Build and critic rule:** a build that must
invert, rename with inverted assertions, weaken or delete any test or bed
row asserting a Stop decision has changed behavior and is rejected. The
six tests and template-backlog row below keep their trunk expectations,
unmodified. Only assertions on message text move to the named report or
human-line surface, with Decision 1's truthful wording. If a test's name
states an outcome that remains true while its display string changes, the
name stays and the string moves. All control, cause, remedy and evidence
assertions remain. No row may pass merely because it found any JSON.

| Proof | Required observation |
| --- | --- |
| STOP-REPORT-INPUT: huge input, block and allow | Combine 200 stale red runs, an owned unwatched job with an exact watch command, eight long plan steps, long questions/human waits, a health line over 2,000 bytes and a 40-line digest. Report contains every uncut fact, title, command and collected digest byte; one judgment/count only. Unknown schema, wrong types and mismatched identity fail before publication. Independent allowed input creates no continuation. |
| STOP-RUNTIME-MAP: one notice | Decode every emitted object: block has compact reason and no systemMessage; allowance has only systemMessage. Total intended human text is one logical line, at most 256 bytes, without trim notice. Pin the actual invoked mapper and generated registration; remove Claude's second receipt Stop. Unknown mapping does not gain conformance. |
| STOP-INFRA-DISCLOSURE: superseded public fields | Combine a real seat-actionable block with an arming infrastructure failure: the only text field is reason, the line contains `needs supervision repair`, and the report leads with the full infrastructure cause/owner/remedy. Assert unchanged block source, hook log, stop-condition line and refusal record. Infrastructure alone still allows with the flag solely in systemMessage. Missing report retains block plus repair/unavailable line. Update the landed root design's 2968-2973 reject clause and both pages' disclosure assertions in the same build. |
| STOP-PRESERVATION-IDLE: unchanged decisions | Pair attempts 1, 2, 3, 4 and later against trunk, including successful/failed preparation, independent blocks, live/pending-setup delegates and lost counter. Preserve each source, count/reset, intent/incident/alarm call and authorization result. Do not add an escalation attempt/result slot or assert once-only escalation across later Stops. |
| STOP-PRESERVATION / TestMismatchedSessionStopMarkerDoesNotGateIdleEscalation | Keep `internal/goal/turnverdict_idle_test.go:338-370` unchanged in outcome: attempt 3 allows and `prepared == 1` across its three calls. Retain the mismatched main's display-only SESSION STOP authorization marker and prepared intent detail in the report. The marker neither authorizes the allowance nor gates escalation; the existing bound releases it. |
| STOP-PRESERVATION / TestReportTurnVerdictHeldClaimWritesRealSeatIdleIntent | Keep `cmd/metasystem/session_stop_test.go:258-371` unchanged in outcome: exit 0 throughout, attempts 1/2 block and attempt 3 allows. Retain `--stop-hook-active` and exactly one real persisted intent across those three calls: reason `seatIdle`, goal `held`, `claimNeeded=false`, machine `bed-m1`, lineage `main-1` and current holder's claim epoch. |
| STOP-ALLOWANCE-LINE: retained work | Third-refusal release shows the held task or `No task in flight; next: <title>` with `Stop allowed`. A delegate exemption shows the owned delegate task with `Stop allowed`. Neither alone adds an intervention phrase; failed escalation adds the typed repair phrase and full report remedy, with the decision phrase only for required human repair. Decode both sole fields, check the 256-byte bound and retained work in the report; neither creates a continuation. |
| STOP-DELIVERY-SNAPSHOT: expectation versus observation | `internal/adapter/snapshot_test.go` and runtime/installation tests assert that Stop-specific registry data is only ExpectedStopDelivery and reuses existing InstructionFile, with no installation evidence. A dated snapshot retains the outer host version/configuration/time and capabilities.stopDelivery's instruction hash, launch/seat binary, artifact and achieved level. Missing observations remain unknown; changed version/config/instruction invalidates observed reuse. A declaration alone cannot pass the conformance gate. |
| STOP-FACTS-WRITER-FAILURE: completed judgment survives | Add `TestReportTurnVerdictFactsWriteFailurePreservesCompletedVerdict` in `cmd/metasystem/session_stop_test.go` and a hook integration row with this identifier. Inject the auxiliary writer failure after a real judgment: one scan/judgment/refusal spend, exit 0, byte-identical existing Verdict JSON plus newline on stdout, diagnostic only on stderr, facts path absent with no partial/stale publication. Hook consumes stdout once, keeps shouldBlock/source, emits only reason with repair/Status unavailable, no bogus pointer, no delivery-cursor advancement. Repeat for a completed allowance without creating a continuation. |
| STOP-HEALTH-JSON: CLI forms | Unflagged and text previews remain byte-identical; JSON decodes to v1 with all role reasons/remedies and matching line/exitCode. Assert one evaluation, no observation/alert writes, stopped/spend/unknown cases and invalid format handling. |
| STOP-INTERVENTION-RETRO-ONLY: negative | In the health preview and presenter, supply only ordinary agent-actionable retro debt: status dead, NoAutomaticRemedy true, full retro/receipt remedy; both classifications and both derived flags are false. Neither intervention phrase appears; the report retains the debt and exact action. Control stays at the supplied verdict. |
| STOP-INTERVENTION-SPEND-ONLY: negative | Supply an otherwise healthy preview with an alive spend crossing and nonempty ceiling-change remedy; both classifications and both derived flags are false. Neither intervention phrase appears, and the crossing/remedy remain report attention. An independent typed human-required fact in a paired case turns on only the decision phrase; spend itself remains report-only. |
| STOP-INTERVENTION-TYPED: positive and rejection | Exercise neither flag, decision only, repair only and both; assert exact phrases and order. Reject missing/wrong-type v1 booleans and missing/duplicate health intervention roles. Changing only remedy text, NoAutomaticRemedy or generic health status cannot change a producer-supplied classification. Required human-only repair remains classified as both with its restrictions in the report. |
| Summary correctness | Alive/dead/unknown and stopped health, alive spend crossing, other-seat/unattributed pages, 11 claimable goals, explicit green continuation and no-continuation history. Counts derive from facts, and every action retains its complete command or honest manual step. |
| STOP-LOCATOR: bounds and lookup | Newlines, controls, Unicode at byte 200 and long titles; deep installation, nested/external cwd, symlink launch, wrong/missing executable binding, identical sessions under different installations. Run the **printed command** and match exact Markdown and identity before emission; reject invalid IDs/escapes. Assert the command is complete and human line stays within 256 bytes. |
| STOP-LAUNCH-BINDING: read-only setup probe | In `internal/hostsetup/setup_test.go`, `cmd/metasystem/runtime_setup_test.go` and host launch tests, prove each claimed start/resume route gives the child the validated installation/bin prefix and keeps provider argv intact. Run `--check --stop-status-id` under that actual command environment with a known report from nested/external cwd and a symlinked executable. Assert correct identity/exit 0; wrong/missing binary and missing report exit 1; invalid flags/ID exit 2. Compare profile, rc, configuration, report, snapshot and control-state bytes before/after the probe: no writes. A separate unbound human shell is explicitly unsupported and receives the absolute recovery command in setup diagnostics, never an over-bound Stop line. |
| STOP-HOST: conformance prerequisite | For each claimed runtime/version/config/instruction hash, observe effective trusted hooks, total human transcript, block-to-exact-report-read-to-action in the provider's actual command environment, allowance without continuation, missing report and continuation-cap behavior. A real host is required; an adapter harness cannot discharge this row. Store the evidence in the dated snapshot. Report the finite range honestly; continuation beyond it belongs to seat-work-continues-past-the-runtime-stop-cap. |
| Storage and races | Concurrent Stops in one session, same session ID under different runtimes/installations, hostile session text, report write/rename/durability failure, failed stdout, retention boundary, stalled completion after published block. Verify exact report-to-payload identity, no cross-session replacement, unchanged control, no false `EMITTED`, no cursor advancement after lost delivery. |
| Degraded fallback | Engine missing/skew, launcher failure, invalid/partial worker output, unresolved root, timeout, record failure, log failure and combined failures. One bounded line, truthful unavailable status where needed, original allowance and recorded causes unchanged. Pair an arming/report failure with a real seat-actionable block and prove it remains blocking. |

Existing rows to carry forward, re-read at this worktree's HEAD for revision 4:

| Bed and anchors | Assertions retained or relocated |
| --- | --- |
| `supervision-hook-fixtures.sh:20-36,179-200,778-801,873-957` | Launcher and missing-engine allowances; runtime-list, hook-attempt, verdict, malformed-verdict and partial-output failures; arming occurrences 1/2, exact component remedy, different health/narrator cause, fresh session, broken refusal record. Full notices move to report content; engine-free cases pin compact cause/remedy literals. |
| Same bed, `395-419,529-572,611-670,839-870,1082-1113` | Installation digest/root, current health and cursor; all open-work first/second/reset/deadline-marker rows; 200-run evidence; start/Stop re-arm success/failure; killed worker and following attempt. Keep the old verdict-file assertions, add Markdown report assertions, and move the console bound to the human field. |
| Same bed, `1150-1267,1323-1357,1514-1581` | Preserve deadline outcomes, records, missing-engine/log failures and consumed human authorization. The template-backlog fixture at 1535-1581 keeps its first/second blocks and third-Stop allowance at 1564-1574. Retain intent bytes at 1575-1581. Move long escalation text, including the true "the turn will end", to the report. Only display assertions move. |
| Same bed, `304-323,422-437,447-526` | Brain start and compact packet, separate cursor, undeclared/corrupt/wrong-ledger cases, missing-engine rebuild notice and boot exit/sleep/invalid notices. These **start** assertions stay on their existing fields and bounds. |
| `supervision-fixtures.sh:1036-1043,1194-1246,1258-1316,1366-1407,1455-1484` | Every nested/linked/override block and sentinel; arming cause plus block and record; copied/symlinked missing-engine allowances; no-world, skew and malformed-root allowances; deadline root isolation and component evidence. Move only the human-field detail checks. |
| Same bed, `3313-3409` | Open-work first/repeat/deep/settled, goal first/repeat, session hygiene, degraded state and unwatched-work block/repeat. Retain the decision sequence and original guidance in agent/report content. |

Also retain the report package's StopRefusal class/count/lock tests, goal
turn-verdict precedence/stop-fence and full-verdict renderer tests,
narrator prefix/cursor tests, health-preview read-only tests, and generated
launcher tests in `internal/hostsetup/setup_test.go` and
`cmd/metasystem/runtime_setup_test.go`. Update installed-template expectations
together with the three enforcement files; a source template alone is not
proof of the registered Stop sequence.

The six policy tests keep their names and trunk Stop assertions.
In `internal/goal/turnverdict_idle_test.go`:

- `TestIdleBacklogBlocksTwiceThenDefersClaimAndPreparesStewardContinuation`
  (214-290): blocks twice, then allows with no block source. Keep
  intent/incident and no direct Claim assertions.
- `TestIdleBacklogFailedIntentStillEndsAndNamesTheFailure` (430-469):
  failed intent still allows with no block source. Keep failure/alarm proof.
- `TestIdleBacklogContinuesThisMachinesHeldClaimWithoutAnotherClaim`
  (292-336): keep the third allowance, held-goal selection and no second claim.
- `TestMismatchedSessionStopMarkerDoesNotGateIdleEscalation` (338-370):
  keep the third allowance, `prepared == 1` across its three calls, mismatched
  `SESSION STOP authorization` marker and prepared-intent detail. The marker
  grants no authorization and suppresses no escalation.
- `TestClaudeTurnExitRequiresDelegateWorkNotMerelyALiveSeat` (1241-1283):
  live and pending-setup delegate work still allow. Stale records and a live
  held seat alone still block. Keep the in-flight facts.

In `cmd/metasystem/session_stop_test.go`:

- `TestReportTurnVerdictHeldClaimWritesRealSeatIdleIntent` (258-371):
  keep exit 0, first/second blocks and the third allowance. Retain the real
  `steward.LiveIntents` read, exactly one `seatIdle` intent across these three
  calls, held goal, `ClaimNeeded=false`, actor machine/lineage, current
  holder's `ClaimEpoch` and `--stop-hook-active`. Do not replace persistence
  with a mock.

These are **six preserved Go tests plus the preserved template-backlog row**.
Later-attempt coverage belongs in paired preservation cases; it must not
silently extend these tests into revision 3's once-only escalation contract.

Keep unreadable-ledger tests (471-558) as degraded/unknown evidence under
Decision 1's preserved behavior; they do not prove an empty backlog. Retain the
independent-open-work-block, stop-fence and one-use human-authorization
tests. New red-before/pass-after obligations above name presenter, mapper,
health CLI, control preservation and cwd-independent read surfaces; none
was run by this design lane.

**Transferred obligations, owned by seat-work-continues-past-the-runtime-stop-cap:**
the continued-work invariant moves whole, with STOP-CONTROL, STOP-CLAIMABLE
and STOP-ABSOLUTE. That goal was opened in Wido's name on 2026-09-13
(`plans/goals/seat-work-continues-past-the-runtime-stop-cap.md:6-15`).
It first lands and proves the outer mechanism, bounded by spend and budgets,
visible in records, stopped by one human command, with no duplicate claims
or restarts into a loop. A seat with no lawful work records its wait and
goes quiet. Proof covers at least two runtimes and a real seat whose work
continues after reaching this host's cap. No finite hook-repeat fixture
discharges STOP-ABSOLUTE.

**Closing the allowances is that goal's own final build item**, after that
mechanism lands and its proof passes. This page leaves no closure work for
a third goal. References in that goal's current record to this console
build closing them are stale under the evening ruling; this lane does not
rewrite the record.

STOP-CONTROL remains CRITICAL in that goal, owned by the goal engine and
hook at `enforceIdleBacklog`, `escalateIdleBacklog`, the locked session
Stop-state and both parent validators. It is outside this build's matrix.
Its full contract follows: after a fresh successful judgment for a
claim-owning seat, nonempty `Claimable` keeps
`ShouldBlock=true` on every engine attempt unless applicable human
session-stop authorization is consumed. Keep human stop-fence authority.
Never infer authorization from a report, timeout, delegate, intent or alarm.
Remove the parent-seat delegate exemption there, without a new capacity
exemption. Authenticated subordinate delegate skips stay distinct. No
advisor/brain presentation exit overrides a successfully observed nonempty
claimable list for a claim-owning seat. Keep independent block precedence
and its exact source/action; otherwise use `idle-backlog` with the selected
goal and normal next/claim instruction. Keep count/reset and lost-counter
evidence. Unknown judgment, unreadable ledgers, engine/deadline allowances,
untrusted hooks and unavailable guidance still cannot prove work absent.

At attempt three, that later item retains intent preparation, incident
recording and the alarm on failed preparation; neither successful nor failed
handoff clears the known-backlog block. Later attempts keep the block and
report the first escalation result. It owns revision 3's once-per-session/
backlog-digest escalation-attempt/result slot in the existing locked Stop-state,
reset with the digest. Persist the attempt before the three escalation seams;
on failed persistence, keep the block, report escalation unavailable and call
none. A crash after that marker leaves an unknown outcome for steward repair;
later Stops mint no new intent. Persist the result for later reports.
Rendering never prepares an intent or makes a claim. Intent nonces are fresh
on each current call; incident deduplication does not deduplicate intents
(`cmd/metasystem/goal.go:726-785`;
`internal/steward/alert_episode.go:234-262`).

STOP-CLAIMABLE there covers all six named scenarios and the template-backlog
case under the later persistent-block contract, retaining their identity
and evidence assertions. It proves attempts 1, 2, 3, 4 and beyond the host cap,
successful/failed preparation, live/pending-setup delegates, lost counter,
marker interruption and failed result persistence, with no duplicate
claims/intents/alarms. Its mismatched-marker and real-held-intent cases keep
authorization evidence, actor/epoch and no-second-claim proof. Its engine
result and actual host continuation are separate observations. The later
item changes the bounded-refusal wording only when its new behavior is true.
All these are future obligations; none authorizes a Stop change in this build.

The following matrix is for this presentation milestone. All rows are
designed but unimplemented at the grounded HEAD. MISSING records absent
code/proof, not a missing human decision.

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| STOP-LINE | HIGH | Decisions 2-3 | One compact line in the sole visible field | report and adapter | stop-present, MapStopOutput, runtime capability, all registrations | STOP-RUNTIME-MAP, STOP-LOCATOR, STOP-ALLOWANCE-LINE | Observe total actual host display | MISSING | Build candidate mappings and prove host behavior |
| STOP-PRESERVATION | CRITICAL | Decision 1; headline proof | Every Stop decision and source equal trunk's for the same input; presentation failure creates or clears no block | goal, goal CLI, report, adapter and hook | Retained enforceIdleBacklog/escalateIdleBacklog; facts writer, mapper/fallback and both parent validators | Six unchanged Go control tests and template-backlog decisions; STOP-PRESERVATION-IDLE, STOP-FACTS-WRITER-FAILURE, STOP-INFRA-DISCLOSURE and degraded cases | Reproducible paired trunk/candidate table with raw verdict, source and native payload for every matrix case | MISSING | Preserve policy, build presentation and prove equality before acceptance |
| STOP-FACTS | HIGH | Decisions 4, 6 | One uncut frozen judgment and explicit health schema; auxiliary write failure preserves completed control | goal, report scanner and health CLI | goal.go facts-file projection/writer; scan.go; steward_verbs.go | STOP-REPORT-INPUT, STOP-FACTS-WRITER-FAILURE, STOP-HEALTH-JSON | Inspect report tails and actions; observe preserved block after failed auxiliary write | MISSING | Extend facts before clipping and implement the exact stdout/stderr/exit contract |
| STOP-INTERVENTION | HIGH | Decisions 2, 4, 6 | Only typed required action/repair facts add the respective phrase; spend and ordinary retro debt remain report-only | goal/report fact producers, steward health and hook boundary; report aggregates | Classified fact projection, HookHealthPreview.interventions, stop-present | STOP-INTERVENTION-RETRO-ONLY, STOP-INTERVENTION-SPEND-ONLY, STOP-INTERVENTION-TYPED | Inspect actual single-line flags and matching report actions | MISSING | Emit explicit classifications from the condition's owner and combine without text/status inference |
| STOP-DETAIL | HIGH | Decisions 4-5, 7 | Matching readable report and truthful delivery evidence within verified command environments | report, hostsetup, host launch plumbing and steward | stop-status, host-common.sh child PATH, runtime setup --check probe, CompleteHookAttempt | STOP-LOCATOR, STOP-LAUNCH-BINDING, storage/race/cursor cases | Read printed command from the actual provider command tool; separately classify human-shell binding | MISSING | Publish and verify lookup before emission; never mutate shell profiles |
| STOP-HOST | CRITICAL | Decision 3 | Runtime may claim only observed finite control/display/read behavior | adapter conformance; runtimes owns static expectation only | ExpectedStopDelivery, adapter mapping, snapshot.go capabilities.stopDelivery, canonical read instruction and runtime entrypoints | STOP-RUNTIME-MAP, STOP-DELIVERY-SNAPSHOT, STOP-LAUNCH-BINDING; installed-config checks | Per-runtime trusted firing and report-read/action observation tied to version/config/instruction | MISSING | Record dated installation evidence before upgrading a runtime's claim |
| STOP-FAILURE | HIGH | Decisions 3-4, 7; STOP-PRESERVATION | Degraded causes survive; presentation failure neither clears nor creates a block; sole line flags repair | hook and report | Parent, composer and launcher fallbacks; preserved condition/refusal writers | STOP-INFRA-DISCLOSURE, STOP-FACTS-WRITER-FAILURE and all degraded rows | Controlled combined block/arming failure, missing-engine and deadline firings | MISSING | Preserve precedence and records while replacing the landed public-field clauses |

## Build list

1. Carry **revision 4 and Wido's evening ruling** into the implementation
   brief. This is a presentation build. Preserve the third-refusal release,
   including failed intent, and the delegate exemption. Assign the whole
   continued-work invariant to seat-work-continues-past-the-runtime-stop-cap;
   its mechanism lands before its own final allowance-closure item.
   No renewed approval, third critique round or STOP-ABSOLUTE claim here.
2. Prove **STOP-PRESERVATION**. Keep the six named tests and template-backlog
   row's Stop expectations and names. Move only message assertions to their
   new surfaces. Add the paired matrix for current decisions and failures;
   retain control state, count/reset, escalation side effects, authorization
   and sources. Do not build the escalation-attempt/result slot.
   Apply Decision 1's truthful report wording and STOP-ALLOWANCE-LINE.
3. Build **STOP-FACTS and STOP-INTERVENTION**: frozen uncut projection in the
   goal command, scanner FullDetail/job/run joins and UTF-8 clipping fix in
   `internal/report/scan.go`, exact facts-writer failure wire in
   `cmd/metasystem/goal.go`, typed producer classifications and
   `HookHealthPreview` JSON in `internal/steward/health.go` and
   `cmd/metasystem/steward_verbs.go`. Preserve ordinary health output and
   policy. Run STOP-REPORT-INPUT, STOP-FACTS-WRITER-FAILURE, STOP-HEALTH-JSON,
   both negative intervention fixtures and all four flag combinations.
4. Build **STOP-DETAIL and STOP-LINE** in `internal/report` and the report
   CLI: v1 presenter/result, complete Markdown, atomic immutable publication,
   exact ID lookup, retention, byte-safe compact forms and unavailable forms.
   Implement `--stop-status-id` on `runtime setup --check` in
   `internal/hostsetup/setup.go` and `cmd/metasystem/runtime_setup.go`; wire
   the specified child PATH in `scripts/agents/hosts/host-common.sh` and all
   claimed start/resume launch routes. Keep profiles untouched and document
   the limited human-shell guarantee and absolute recovery command. Run
   STOP-LOCATOR, STOP-LAUNCH-BINDING and the storage/race/cursor proofs.
5. Build **STOP-HOST's delivery seam and STOP-FAILURE**: add only static
   ExpectedStopDelivery to `internal/runtimes/runtimes.go`, reuse
   InstructionFile, implement and invoke `internal/adapter.MapStopOutput`
   through `adapter stop-output`, and extend the existing dated
   `internal/adapter/snapshot.go` capability payload. Wire the presenter/
   mapper through `supervision-hook.sh`'s worker, composer, final emission,
   advisor exits and both deadline parents. Keep control and refusal
   recording separate from display; generate engine-free mappings from the
   same adapter table; consolidate the receipt check and remove Claude's
   second Stop registration. Update `CompleteHookAttempt` and digest/protocol
   cursor evidence to require the exact emitted report reference/hash and
   verified locator. Run STOP-RUNTIME-MAP, STOP-INFRA-DISCLOSURE,
   STOP-DELIVERY-SNAPSHOT, writer failure and all retained degraded rows.
6. In the **same implementation cutover**, update all three enforcement
   templates and their generated installations; install Decision 3's read
   instruction in the canonical AGENTS.md and the entrypoints resolved by
   Declaration.InstructionFile. Update these three documentation owners:
   `docs/design/turn-verdict-delivery-contract.md`,
   `plans/supervision-hook-root-design.md` (including the reject condition),
   and `plans/stop-infrastructure-allows-the-seat-to-stop-design.md`.
   Apply the confirmed public-field supersession only; retain the
   idle-backlog rule, delegate exemption, precedence, human authority,
   hook logs and refusal records.
7. Run the focused tests and affected fixture rows named above, then the
   mandatory implementation **code-critique** and applicable completion
   checks. The critic rejects any Stop-decision assertion inversion,
   weakening, deletion or concealed rename, and checks STOP-PRESERVATION's
   paired evidence before acceptance. Before claiming any runtime conforms,
   perform STOP-HOST's real
   observations and retain the dated version/config/instruction/binding
   evidence. Unsupported or unobserved runtimes remain explicitly limited;
   inability to prove a runtime is an evidence limit, not a new policy
   choice. No readiness, ledger schema, supervision lifecycle or narrator
   rewrite-policy redesign belongs in this build. Stop after discharging
   this milestone's applicable proofs; the outer continuation has its own goal.

## Revision record

**Revision 4, 2026-09-13:** Wido ruled this evening, verbatim:

> "make 100% absolutely sure we DO NOT BREAK LOOPS AGAIN. changing stop message is what we need to do; but WITHOUT introducing unwanted stopping again."

This supersedes revision 3's reading of "option 1 now, and option 2 opened
as its own goal rather than dropped" as permission to close the third-refusal
release and delegate-job exemption now, including failed handoff. It also
supersedes the six Go inversions, template-backlog inversion, persistent-block
display promises and escalation-attempt/result slot in this landing.
STOP-PRESERVATION replaces this build's STOP-CONTROL obligation. The whole
continued-work invariant and its obligations move to the already opened
continuation goal; that goal lands its mechanism first and closes the
allowances as its own final item. No third goal or human choice is left.

The reason follows revision 3's own admission: the hook cannot enforce
continued work once the runtime stops honouring it. Persistent blocking
without outer continuation runs the seat into the runtime's cap; it does
not prove more work. This revision restores the console goal's DONE clause.
Decisions 2 to 7 retain their report, locator, runtime mapping, health
interface and presentation proofs, adjusted only for preserved control
and truthful wording. The public-field supersession remains in force.

**Revision 3, 2026-09-13 (superseded):** accepted all six material items from
the closing `design-read-r2.md`, with finding and disposition sets both
`{1, 2, 3, 4, 5, 6}`. Its test inventory remains useful; its decision
inversions do not. The retained dispositions below reflect revision 4.
This is the human's correction, not a third critique. Code-critique of the
implementation and the proof matrix remain mandatory.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| 1 | Inventory retained; inversions superseded | `internal/goal/turnverdict_idle_test.go:338-370` and `cmd/metasystem/session_stop_test.go:258-371` require third-attempt allowances omitted from revision 2's inventory. | Both full names remain in the six-test preservation list and named STOP-PRESERVATION cases. Keep allowances, marker, exactly-one real intent across three calls, actor/epoch and no-second-claim assertions. Build step 2. |
| 2 | Accepted; folded | `plans/supervision-hook-root-design.md:274-278,2968-2973` and `plans/stop-infrastructure-allows-the-seat-to-stop-design.md:43-49,64-77` mandate superseded public disclosure fields. | Decision 3 explicitly supersedes those clauses and the reject condition; sole-line repair flag plus full report replace public detail, with precedence/logs/records preserved. STOP-INFRA-DISCLOSURE; build steps 5-6 name both pages. |
| 3 | Accepted; folded | `internal/runtimes/runtimes.go:1-12,76-103` is static; `internal/adapter/snapshot.go:17-82` already owns dated installation evidence. | Decision 3 keeps only ExpectedStopDelivery in Declaration, reuses InstructionFile and stores observations in capabilities.stopDelivery with existing snapshot version/config/time. STOP-DELIVERY-SNAPSHOT; build step 5. |
| 4 | Accepted; folded | `cmd/metasystem/goal.go:637-703` makes a nonzero exit control-significant even after a valid judgment. | Decision 4 pins exit 0, unchanged stdout Verdict JSON, stderr-only diagnostic, absent facts file, one judgment and retained hook control with Status unavailable. STOP-FACTS-WRITER-FAILURE; build steps 3 and 5. |
| 5 | Accepted; folded | `internal/hostsetup/setup.go:25-84` and `cmd/metasystem/runtime_setup.go:15-55` have no shell-profile owner; host scripts expose child launch boundaries. | Decision 5 pins provider child PATH and read-only --check --stop-status-id, explicitly forbids profile mutation and narrows the human-shell guarantee. Retain 132-byte command/256-byte line, absolute recovery outside the notice. STOP-LAUNCH-BINDING and STOP-LOCATOR; build step 4. |
| 6 | Accepted; folded | `internal/steward/health.go:421-445,448-463` shows alive spend with remedy and agent-actionable retro debt with NoAutomaticRemedy. | Decisions 4 and 6 require producer-owned humanRequired/supervisionRepair classifications, including HookHealthPreview.interventions. Spend stays report-only; STOP-INTERVENTION-RETRO-ONLY and STOP-INTERVENTION-SPEND-ONLY are negative fixtures, with positive typed combinations. STOP-INTERVENTION; build step 3. |

**Nothing remains for Wido to decide on this page.** The
**seat-work-continues-past-the-runtime-stop-cap** goal was opened in his name
on **2026-09-13**; this page records the revised ordering without a ledger
write. Revisions **1 to 3 remain superseded** and are not build authority.

**Revision 2, 2026-09-13 (superseded):** adjudicated revision 1's six findings
on persistent blocking, sole visible field, runtime delivery, uncut facts,
exact locator and health JSON. It left the human ruling pending, omitted
two tests, put installation evidence in the wrong owner, left facts-writer
exit/binding unspecified and overclassified intervention phrases. Revision
3 replaced those incomplete clauses; revision 4 now governs the build.

**Revision 1, 2026-09-13 (superseded):** captured the one-line target and
September 11 refinement but retained the Stop exceptions and assumed
unbuilt delivery mechanisms. Its superseded statements are not build authority.

The companion rewritten-story change is in this HEAD; retain what the
current digest reader returns without rebuilding that work. Revision 4
was checked by rereading this page and all cited source/assertion lines at
the grounded HEAD. Provider references remain historical evidence from
revision 2, not new host observations. The requested design outcome exists:
each current obligation has an owner and proof target; failure, timeout and
bad input preserve trunk control. The transferred invariant has an owner,
ordering and closure item. No code, fixture bed, host run, receipt, goal or
ledger write; only this design page changed and it remains uncommitted.
Implementation and named conformance proofs remain outstanding. The design
is ready for the presentation build under revision 4.
