# Design revision 7: two useful Stop lines and an exact short report command

Goal: [stop-refusal-fits-on-one-screen](goals/stop-refusal-fits-on-one-screen.md).
Design only. **Revision 7, 2026-09-14; one consolidated build brief.**
Revision 6 was unbuilt. It is superseded and folded into this revision.
Revisions 4 and 5 landed. Trunk `b643f423` also landed the always-loaded
report-read instruction. Preserve it and build the Stop imperative here.
Both carriers are required. Neither may be removed as redundant.

Wido's restatement this morning supersedes the earlier one-line wording:

> "I care about: a stop message that provides a useful status that tells me what just completed and what is being worked on next. In max two lines. I also care about a reliable loop. I kind of agree that we should not mix stop messages of agents; so that makes sense too. The insanely long hex dump in [the line] however I don't like. That can be much shorter with the same guarantee"

The supplied failure was an allowance preceded by a separate retro notice:

```text
Stop says: Metasystem retro due: run scripts/receipt.sh check for details, then skills/retro.
Stop says: Task: stop refusal fits on one screen; Stop allowed; needs your decision; status: metasystem report stop-status --id 76a61b004914c84c6463a4d5efc0d17cd1c42f3be0c9e215e2d637b1d524e7eb-15fd2d40fae99afd9756ffd62c6903a7
```

The first line will say what completed. The second will name current or
selected next work, retain the Stop outcome, and instruct the seat to read
and act. The short command is an exact reserved alias, not an unchecked
abbreviation. Details and ancillary notices go into the same report.
The two-line count covers everything these installed Stop hooks show the
human. It is not a count of fields in one JSON object.

**No Stop decision may change.** Preserve `(shouldBlock, blockSource)`,
including the third-refusal release, failed preparation and delegate
exemption. Wido's September 13 loop ruling and the goal's printing-only
scope still bind. The separate goal
**seat-work-continues-past-the-runtime-stop-cap**, opened in Wido's name on
2026-09-13, owns continuation beyond the provider's Stop cap. It must land
and prove its outer mechanism before closing the two allowances as its own
final item. This page grants no claim, rearm, repair or human-stop override.
No third goal or human decision is needed. No third design critique round.

## Grounding at HEAD and current trunk

The worktree HEAD is `b64a485e0212f3152da07dbb659a56af33660cb1`.
Current local trunk is `refs/heads/main` at
`b643f4237379a12f402de0b463f57a7e7a08ffc0`. Before editing, this lane fetched
both requested trunk blobs with read-only `git show refs/heads/main:<path>`:
this page and `metasystem/AGENTS.md`. It read the entire trunk page and its
diff from HEAD. No ref, index or checkout was moved; no remote freshness
claim is made beyond that current trunk ref. All source paths below are
relative to `metasystem/`. The grounding table cites re-read HEAD lines
unless explicitly labelled trunk. Other citations in the retained clauses
are historical anchors from their earlier revision, unless listed here. The cited Stop implementation is unchanged
between these two commits. The supplied console observation is evidence
from the brief, not a locally reproduced host run.

| Owner and lines re-read | What the design uses or preserves |
| --- | --- |
| Trunk `AGENTS.md:34-36`; trunk `plans/goals/stop-refusal-fits-on-one-screen.md:8-10` | The standing rule already requires reading on both outcomes and following lawful seat actions. The goal permits only a printing change. |
| `internal/report/stoppresentation.go:847-905,921-968,984-1036` | Job, run, held and selected-work precedence; goal-slug naming; stopped, empty and unknown states. The current formatter still emits `status:` with the full identifier. |
| `internal/goal/turnfacts.go:31-59,88-112,174-263`; `internal/goal/turnverdict.go:176-201,296-324` | Frozen actions and main-ID ownership exist. The verdict state has a last-touch time and a green cursor, but neither is a portable runtime turn-start record. Do not mutate either to format completion. |
| `internal/report/scan.go:182-190,362-440`; `internal/dispatch/jobrecord.go:51-65,98-114`; `internal/dispatch/record.go:36-65`; `internal/run/run.go:173-220` | Existing job and run records identify their owner and work. Jobs have `endedAt`; successful jobs are `completed`. Runs have generation, nonce and terminal sequence; success is `green`. A run's `endedAt` marks draining entry, not successful terminalization. Read these as presentation evidence without changing scanner membership or Stop policy. |
| `internal/receipt/receipt.go:30-43,149-158,201-211`; `internal/goal/file.go:378-410` | Task receipts have optional goal/builder fields, but no required seat/session/turn identity. Goal history can attribute a human conclusion. Neither a global latest receipt nor a conclusion proves this seat completed work this turn. |
| `internal/report/stoppresentation.go:574-675,1263-1368` | Immutable report publication, per-session lock, rotation and strict full-ID lookup already exist. The report marker binds full identity. Decision 5 extracts shared storage for aliases; no global latest-report lookup. |
| `internal/adapter/stopoutput.go:45-80`; `internal/steward/component_evidence.go:450-489`; `scripts/agents/fixture-stop-report.sh:24-61` | Mapper, completion verifier and fixture reader currently assume one line, `status:` and a full ID. All must move together while retaining identity, digest and sole-field checks. |
| `scripts/agents/supervision-hook.sh:722-731,916-925,999-1002,1148-1250,1291-1312,1440-1476,1479-1542` | Complete `surface_json` inventory and reachability. Receipt already has a collected Stop path. Some calls only construct captured JSON; direct later notices follow Stop's unconditional exit. Decision 7 assigns every site. |
| `scripts/enforcement/claude-code-hooks.json:16-25`; `scripts/agents/supervision-hook.sh:35-40,108-123,276-305,1015-1023` | Trunk's Claude template has one Stop registration. A stale installed receipt registration can still explain the supplied extra notice. Engine-free and retained-decision failures are fixed one-line forms. |
| `internal/goal/turnverdict.go:577-607,686-738` | The delegate exemption and third-refusal allowance remain. Failed preparation still releases when no earlier branch blocks. Presentation cannot change that source or any side effect. |
| `internal/report/stoppresentation_test.go:169-216,501-551,767-834,930-977`; `scripts/agents/supervision-hook-fixtures.sh:790-828` | Landed presenter/name/allowance/storage tests and hook pair are the extension seams. This revision names additional proofs; it does not claim they ran. |

The older revisions' provider-document observations are historical only.
They do not certify this installation. The real-host gate below remains
necessary. The existing body carries the landed contracts whole except
where revision 7 explicitly changes display, completion evidence or lookup.
Statements about pre-build status in the revision record are historical.

## Decisions

### 1. Preserve every trunk Stop decision

For every input, the pair **(blocks or allows, block source)** must be exactly
what trunk at the grounded HEAD produces. Only the text differs. This covers
the goal judgment, hook composition, native emission and degraded exits.
STOP-PRESERVATION below is the headline acceptance obligation.

Keep these branches exactly as trunk has them:

- `internal/goal/turnverdict.go:577-581`: the
  `HasDelegateJobInFlight()` early return in `enforceIdleBacklog`, including
  its digest update and counter reset. It exempts the parent seat's backlog;
  it does not clear an independent block. The live and pending-setup cases
  remain as tested. `internal/goal/project.go:212-222` owns the predicate.
- `internal/goal/turnverdict.go:592-607,711-738`: at attempt three and later,
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
| Attempt 3 or later | `refusal N reached the bound of 3 for this unchanged backlog`. Retain the actual intent/incident/alarm result. End with `Stop is allowed; read the report and continue lawful work before stopping` on an allowance; otherwise `another turn-verdict branch remains blocking this stop`. |
| Delegate exemption | `delegate work is in flight; claimable backlog remains`, when both facts exist. The report still names the selected work and delegate state. |

Keep "of 3" and the conditional Stop outcome. Revision 7 replaces the
prediction "the turn will end": an allowance permits stopping, but the
seat must first read and act on lawful remaining work. This is a wording
correction only. Do not substitute revision 3's unbounded refusal wording.
A prepared intent proves a request, not a completed handoff or an actual
continuation. Replace the old promise that the steward will continue the goal with the request wording
above. Keep the existing unreadable-ledger wording and uncertainty evidence.
"This refusal does not repeat for the same work" remains confined to its
existing open-work case; it does not describe counted idle refusals.

### 2. At most two logical lines, for either outcome

A report-bearing Stop emits exactly two logical lines in one native text
field, separated by one LF. Line 1 is at most **144 UTF-8 bytes**. Line 2 is
at most **256 UTF-8 bytes**. The pair is at most **401 bytes including the
LF**, after JSON decoding and excluding the transport's final newline.
No other newlines, carriage returns, Unicode line separators or terminal
controls. The adapter must not duplicate the text or show raw JSON.
The retained one-line degraded forms have their existing 256-byte bound.
Authenticated delegate skips remain silent. Narrow terminals may wrap;
this is a logical-line contract, not a promise about every terminal width.

These are the exact forms. Angle brackets denote substitutions.

Block:

```text
Just completed: <completion>.
Task: <name>; Stop blocked; Read: metasystem report stop-status --id <short-id>
```

Allowance:

```text
Just completed: <completion>.
Task: <name>; Stop allowed; Read, then continue lawful work before stopping: metasystem report stop-status --id <short-id>
```

`Read` means run that complete command and read the returned report.
For a block the report supplies the action. For an allowance the line
explicitly requires reading and continuing lawful work before stopping.
The word `lawful` preserves human stops, authority, true waits and action
restrictions. It makes no claim that all allowances have work. If there is
no lawful action, the seat may end its turn after the read. A held goal is
continued without another claim. A free seat fetches and claims only the
ready goal returned by `goal next`. A delegate exemption retains its owned
work; the seat follows its watch/wait action. A prepared intent is not a
completed handoff. No new continuation field or extra turn is forced.

Line 2 retains the exact task prefix and naming rule from Decision 4:
`Task: <name>` for in-flight work; `No task in flight; next: <name>` for
selected unclaimed work; `No task in flight` for proven empty work or the
existing stopped-ledger state; `Task unknown` for incomplete evidence.
The `next` form names what is selected to work on, not a claim already made.
Current work's exact next step leads the report. Other seats cannot supply
either line's named work. A known task with a missing name uses
`task name unavailable`, never `Task unknown` merely because its name failed.

Insert any typed intervention immediately after the Stop outcome on line 2:
`needs your decision`, `needs supervision repair`, or
`needs your decision and supervision repair`. No separate warning accompanies
it. Neither retained allowance alone adds a flag. Failed intent preparation
or failed supervision evidence adds repair; human-required repair adds both.
Ordinary retro debt and a spend suggestion remain report-only.

Reserve the entire line-2 prefix, outcome, intervention, imperative and
command before shortening only its chosen name at a UTF-8 boundary.
The short identifier is never clipped. With a maximum 32-byte alias and
both flags, the name budgets are 117 bytes for a held block, 98 for a next
block, 74 for a held allowance and 55 for a next allowance. The source name
still has its 100-byte limit. With a one-byte alias these budgets are 148,
129, 105 and 86 bytes respectively. The report heading keeps the full name.
No ellipsis, model-written summary or intent fragment replaces it.

Line 1 uses one of these exact substitutions for `<completion>`:

| Evidence | Completion text |
| --- | --- |
| Newly observed owned job with status `completed` | `<name> (delegate returned)` |
| Newly observed owned run with status `green` | `<name> (run passed)` |
| Complete comparison, with neither event | `none recorded this turn` |
| No usable previous observation, missing ownership or unreadable evidence | `unknown for this turn` |

The name follows Decision 4's job/run source rule, with the same fixed
`task name unavailable` fallback. Even a 100-byte name in the longer job
form makes line 1 only 137 bytes. The full record and all other completions
stay in the report. `delegate returned` does not certify its answer, a
landing or the goal's completion. `run passed` does not conclude the goal.

### 2a. What supplies “just completed”

Use **newly observed successful terminal job/run records owned by this
seat**. `internal/report` owns this read-only completion projection and its
comparison. The existing dispatch and run owners supply the records.
This catches finished design/build/review delegates and passed runs. It
avoids attributing a repository-wide receipt, someone else's landing or a
human's goal conclusion to this seat. No current record proves arbitrary
unrecorded manual work finished. In that case say `none recorded this turn`,
not that nothing happened. Do not guess from prose, a disappearing claim,
a green process exit alone, receipt-check success, or an allowance.

For this presentation, “this turn” is the work interval since this seat's
previous frozen Stop observation. The hook has no portable runtime
turn-start fact. State that interval in the report. On first use or a lost
baseline say `unknown for this turn`; list any older known completion as
background, never as newly completed. This honest limit is preferable to
calling an old success “just completed”. It needs no transcript parser or
new runtime lifecycle hook.

Capture the existing job records before their live-status filter and the
existing run records at their current read seam. Make a separate typed
projection; do not add terminal jobs to `judgment.scan.jobs`, `Busy`, the
green cursor or any predicate the judgment consumes. Retain source bytes
and digests. Match a nonempty `mainId` to the current resolved main. Use any source-defined parent-seat lineage or claim-epoch binding too.
Reject conflicts. A delegate's own runtime/session is not its parent seat's
runtime/session; never compare those as though they were the same identity.
Sharing a goal or machine name is insufficient. Unknown ownership supplies
neither a completion nor evidence that no completion exists.

Under the existing session presentation lock, compare against the newest
valid earlier immutable report for this exact installation, runtime,
normalized session, session key and main ID, with compatible holder
identity. Use its frozen completion observation, not a global latest file,
file mtime or mutable verdict text. Order reports by observation time, then
full attempt ID for equal times. Reject a future or invalid baseline.
An old report without these new facts is not a baseline. If a newer
observation is unreadable or invalid, do not silently skip it and call an
older comparison current; use `unknown for this turn`. No extra
cursor or ledger field is added. Rotation may remove a baseline; that
returns `unknown`, never a stale or other-seat completion.

A job completion identity is its immutable reservation identity (operation
ID when present, otherwise job ID plus startedAt), owner and endedAt.
The new event is status `completed` in this observation and absent as a
completed event in the previous one. Validate its timestamps. A newly
appearing job whose endedAt predates the baseline is historical background,
not a new completion. A known active-to-completed transition at the same
coarse timestamp still counts; the observed state change is evidence. A run event
is run ID, generation, launch nonce and positive terminal sequence with
status `green`; compare that whole identity. Do not time green completion
from run `endedAt`: it timestamps draining entry. A record created and
completed between observations is still new. Repeated Stops with unchanged
records say `none recorded this turn`. Failed, cancelled, timed-out, red,
launch-failed and ended-unknown records remain diagnostics, not success.

Choose the most recently ended new job, with ID as a deterministic tie
break. If there is no new job, choose the new green run with greatest
terminal sequence, then ID. Jobs take this display precedence because they
name a returned piece of seat work. Do not claim this orders job returns
against run terminalization by time. The report lists all events and the
chosen rule. A missing or malformed potentially relevant record makes the
comparison unknown; keep known events in report detail without asserting a
complete latest result. Source-read failures add the existing typed repair
flag, with cause and recovery. A first-use baseline absence alone adds none.

The goal CLI captures this separate observation during the existing scan
and writes an optional `--completion-file <absolute-path>` sidecar alongside
`--facts-file`. Validate fresh paths before judging. The sidecar contains
schema version 1, installation/session/main identity, collection time and
the typed job/run observation. It never changes the Verdict JSON, frozen
judgment fields or scan membership. A sidecar write failure after completed
judgment keeps exit 0 and the same stdout; capture its stderr and leave no
partial file. `report stop-input` consumes the sidecar into presentation v2.
The report presenter compares the observation to the baseline and freezes
the resulting completion. A missing sidecar produces `unknown` plus a typed
repair entry; the otherwise valid report still publishes. This differs
from losing the whole judgment-facts file. No second scan or judgment.

The immutable input carries `completionObservation`, an object with
`schemaVersion:1`, the sidecar's identity and collection timestamp,
`records` (typed job/run record array) and `unavailable` (string array).
Use null plus a typed outer unavailable entry on failed capture. Successful
empty enumeration is an empty array, not null. Missing baseline is a
comparison result, not a failed capture. The presenter does not rewrite
the input file. It freezes both that observation and the derived
`completion` object into the report under the presentation lock.

In `completion`, `state` is `observed`, `none` or `unknown`;
`baselineReportId` is the full canonical ID or empty; `intervalStart` and
`observedAt` retain observation times; `selected` is an event or null;
`events` contains every new successful event; `unavailable` contains every
reason a comparison cannot be complete. The recorded observation retains
all compared owned identities and states, not just the selected success.
Each event retains kind, record/goal IDs, main ID, source path/digest, full
name sources and terminal identity. Arrays are present even when empty.
The report body preserves these frozen facts and the exact two console
lines. Extend the internal presentation wire as v2; keep the frozen judgment
wire and the `metasystem-stop-report-v1` full identity marker unchanged.
Old immutable reports remain readable. Unsupported wire versions degrade
with the retained decision, never by judging again.

Publication is an observation boundary, not proof the human read it. A
published report whose emission later fails can be the next baseline; the
next report must disclose that interval and cannot claim earlier delivery.
Concurrent Stops serialize publication and use frozen inputs; if observation
time goes backwards or ownership changes, report completion as unknown.
These display comparisons never change Stop control, refusal counts,
protocol advancement or the existing delivery-evidence conditions.

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

- Block: `{"decision":"block","reason":"<two lines joined by \n>"}`.
  Omit `systemMessage` and additional context.
- Allowance: `{"systemMessage":"<two lines joined by \n>"}`.
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
learns that supervision needs repair from the second line's own
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

| Runtime | Envelope/control and human fields | Duplication and report-read route | Installation/trust and continuation limit | Evidence limit for revision 7 |
| --- | --- | --- | --- | --- |
| Claude | Candidate shapes above; block reason and allowance systemMessage are public | Two populated visible fields produce two notices; landed standing rule plus Stop imperative required | Inspect effective Stop registrations, generated launcher and loaded CLAUDE.md; fire block and allow; documented cap 8 | Existing hook emission only; revision-7 two-line/read behavior unobserved; absolute no-stop claim contradicted by cap |
| Codex | Documented decision/block plus reason continuation; systemMessage public; same compact shapes proposed | Treat reason as shared; inspect actual warning/prompt display for duplicates; landed AGENTS.md rule plus Stop imperative required | Project and exact hook definition must be trusted; inspect effective `/hooks` definition/hash and fire both outcomes; continuation limit unknown | Declared only; new mapping, trusted firing, report read and cap behavior unobserved |
| Devin | Accepted Stop envelope, blocking field and public fields unknown | Duplication and native continuation/read route unknown | Inspect effective installation and provider trust mechanism, then observe; limit unknown | Declared only; no conforming mapping may be claimed |

**Two required carriers, folded from revision 6.** The always-loaded rule
already exists at trunk `b643f423`, `AGENTS.md:36`. Its exact text is:

> On every governed Stop, blocked or allowed, run the exact printed `metasystem report stop-status --id ...` command and read the report before ending the turn or taking the next work action; follow its seat action within existing authority. If the report is unavailable, keep any established block, report the read failure and recover current work through `goal next`; never infer that no work remains.

Keep that canonical instruction. Generated CLAUDE.md and other runtime
entrypoints must load it. Decision 2's Stop imperatives are the second
carrier and remain to be built. Both carriers exist in this contract;
the standing carrier is landed and the Stop carrier is specified here.
Neither may be removed as redundant. Compaction or different always-loaded
instructions can lose the rule. The delivered Stop text must itself
instruct the seat. Its small budget cannot carry every authority and
recovery rule, so the standing carrier is still needed too. Neither
source-file presence nor intended hook delivery proves actual host loading.

> On every governed Stop, blocked or allowed, run the exact printed
> `metasystem report stop-status --id ...` command and read the report before
> ending the turn or taking the next work action; follow its seat action
> within existing authority. If the report is unavailable, keep any
> established block, report the read failure and recover current work through
> `goal next`; never infer that no work remains.

This supersedes the old blocked-only trigger and deliberate allowance-read
deferral. It requires no new adapter continuation, acknowledgment turn or
Stop decision. If the host closes the turn before the seat can act on an
allowance, record that delivery limit and read at the next ordinary
opportunity. Do not describe that as a successful before-stop read. Actual
continuation beyond the host's behavior remains the separate goal's duty.

Instructions request a read; they do not mechanically prove it. Before a
runtime may claim this delivery contract, pin host version and effective
configuration, observe its human transcript with all registered hooks,
prove the locator works from the seat's actual cwd, and observe a blocked
seat read that exact report and perform its named action. For an allowance
with a lawful next step, observe the report read and action when the host
permits it, or record the host's inability to deliver that opportunity.
The allowance envelope still creates no forced continuation. A stub adapter
harness proves only serialization. The gate also records trust/disabled-hook behavior, read
failure, control precedence with other hooks, and the continuation cap.
Source tests cannot replace this gate.

**Implementable now:** compact Claude/Codex candidate envelopes, complete
reports and lookup, and both instruction carriers, with every goal-engine
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

The v1 interfaces below have landed. Revision 7 extends only the internal
presentation input/result to v2 for completion and the two-line text; the
frozen judgment remains v1. Preserve these existing field and failure contracts.
`cmd/metasystem/goal.go` already supports `--facts-file <judgment.json>`
on the existing `report turn-verdict` invocation. It performs its scan and locked judgment once, captures the
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
`StopPresentationInput` v2 file. `internal/report` owns its schema and renderer;
the command boundary composes it. The goal engine owns the frozen judgment
projection and imports neither steward nor adapter. Invoke exactly:

`metasystem report stop-present --root <resolved-installation> --input-file
<input.json> --output-file <presentation.json>` (one shell command).

All flags are required, paths are absolute, and inputs travel through files.
On success the command publishes the Markdown, alias binding and
presentation result in Decision 5's durable order. Stdout/stderr are empty
and exit is 0. Exit 2 denotes
invalid flags/schema/identity; exit 1 denotes publication failure. Diagnostics
go to captured stderr, never directly to the human. Neither failure changes
the retained control verdict. Reject unknown schema versions, trailing JSON,
wrong types and identity mismatches before publishing anything. Arrays are
present even when empty; an unavailable observation is null with a named
entry in `unavailable`, never an invented empty successful result.

| Input field | Retained field type and content; owning producer |
| --- | --- |
| `schemaVersion` | Integer, exactly 2 for presentation input/result; nested judgment and health remain 1. |
| `identity` | Object with string fields `installation`, `runtime`, `session`, `sessionKey`, `attempt`, `mainId`, `machine`, `lineage`, `observedAt`; integer `claimEpoch`. Timestamp is UTC RFC3339Nano; paths/root and holder fields come from resolved installation/seat evidence. Unavailable holder strings are empty, epoch 0, and ownership is `unknown`. |
| `judgment` | Null on unavailable judgment or facts-file failure (distinguished by `control.judgmentAvailable` and `unavailable`), otherwise the immutable facts-file object: `schemaVersion:1`, `identity` (strings `installation`, `session`, `mainId`, `observedAt` from the existing command arguments/resolution and evaluation clock), `verdict` (the full existing Verdict JSON), `fullDisplay` (uncapped composed text), `scan`, `work`, `ownership`, `actions`, `refusal`. Root/session/main identity must match the outer input; evaluation time is preserved separately from collection time. This is a record of one evaluation, not permission to evaluate again. |
| `judgment.scan` | Object with arrays `open`, `templateUnfilled`, `waitingOnHuman`, `stalePlans`, `busy`, `questions`, `drafts` of `ScanItem`; `openWorkWarnings`, `unreadable`, `runUnreadable` of strings; `jobs` and `runs` of the extended facts below. Preserve scanner order, membership and seen evidence. |
| `ScanItem` | Object: strings `kind`, `id`, `detail`, `fullDetail`, `lineDigest`, `sourcePath`, `ownerMainId`, `requestedAction`; booleans `previouslyRefused`, `humanRequired`. `fullDetail` and `requestedAction` are uncut. Unknown ownership is empty; the item remains in the report. |
| Job/run facts | All current `JobFact`/`RunFact` fields, lower-camel JSON keys, plus strings `title`, `role`, `goalId`, `startedAt`, `sourcePath`, `sourceDigest`, `ownership` (`owned`, `other`, `unknown`). Job title uses the joined goal intent when available, otherwise its existing role label with separator hyphens/underscores replaced by spaces; run title uses `Record.Display`. These are uncut source facts, not approved display names. The naming rule below consumes them without rewriting them. Empty values mean missing source data and are recorded as such. No record schema or ledger edit is needed. |
| `judgment.work` | Object: `readSucceeded` boolean; arrays `claimed`, `landing`, `claimable` of `GoalFacts` with `id`, `intent`, `nextStep`, `revision` strings; `selected` is one such object or null; `selection` is `held`, `claimable`, `none`, or `unknown`; `refused` retains goal ID/cause objects; `inFlight`, `nonTerminalJobs` string arrays; `queued` integer and `goalFree` boolean. Capture IDs **and intents** from the goal records already read under the judgment lock. Retain goal-engine ordering; never select by report scan order. |
| `judgment.ownership` | Object: `state` (`owned`, `none`, `unknown`), `goalId` string, `evidence` string. Only the resolved holder and joined own work can establish ownership; merely sharing a machine name is insufficient. |
| `judgment.actions` | Ordered array of objects with strings `kind`, `targetId`, `instruction`, `command`, `owner`, `restriction`; booleans `humanRequired`, `supervisionRepair`. Empty command means a named manual step, not a made-up verb. Exact instructions/commands precede background detail in the report. |
| `judgment.refusal` | Object with `class`, `causeCode`, `component`, `detail`, `remedy`, `blockSource` strings (empty absent), `occurrence` integer, `countSpent`, `idleRefusal`, `humanStopConsumed`, `humanRequired`, `supervisionRepair` booleans; `escalation` object containing `intentId`, `incidentId`, `alarmDetail`, `detail` strings and `intentPrepared`, `humanRequired`, `supervisionRepair` booleans. No clipped notice is parsed to recover these facts. |
| `control` | Object: `shouldBlock` boolean, `blockSource` string or null, `class` and `causeCode` strings, `judgmentAvailable` boolean. Match the retained verdict exactly; unavailable judgment uses the existing infrastructure result. This is the only mapping input that controls block/allow. |
| `completionObservation` | Null on failed capture with a typed unavailable entry, or Decision 2a's versioned, identity-bound observation. Presentation only; no new judgment predicate. |
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

**Task naming, revision 5.** Select the work before naming it. Keep the
precedence: an owned nonterminal job, then an owned active run, then the
seat's held goal, then the engine-selected claimable goal. For multiple
jobs/runs in the same tier, choose the earliest nonempty `startedAt`, then
ID; missing dates sort last. Include all others and their count in the
report. Other seats' work never supplies the name. Sharing a machine name
does not establish ownership. Keep the frozen holder/main-ID joins and
engine selection; do not infer ownership from a goal ID or a useful title.

| Selected tier | Name source, in order |
| --- | --- |
| Owned nonterminal job | Its `goalId`, rendered as words. If unusable, its `role`, rendered as a whole short label. Never use `JobFact.title`: it can be the whole joined intent. A role alone names the activity, such as `design review`; the report records that the goal name was unavailable. |
| Owned active run | Its `goalId`, rendered as words. If unusable, its whole `title` (`Record.Display`), only if it passes the short-label rule below. A prose display contributes report detail only. The kind (`suite`, `cohort`, `custom`) and run ID remain report context; neither alone names the task. |
| Seat's held goal | The frozen selected goal's `id`, rendered as words. Intent is report detail even when short. |
| Engine-selected claimable goal | The same ID rule, labelled `next`. It is selected work to claim, not work in flight. |

Render a goal ID by replacing each hyphen with a space and collapsing
whitespace. Keep its words and case; do not invent a paraphrase. Thus
`stop-refusal-fits-on-one-screen` names `stop refusal fits on one screen`.
Use the existing goal-ID grammar and 100-byte bound before conversion.
Require at least one letter. Reject an opaque hexadecimal key: after
removing hyphens, it consists entirely of hexadecimal digits and has
32, 40 or 64 characters. This also excludes a UUID. Empty, numeric-only,
punctuation-only and malformed sources are unusable. These are display
checks, not new ledger validation or reasons to block Stop.

For a fallback role or run display, accept a **whole short label**, never
an extracted sentence or clause. Reject embedded line breaks and terminal
controls before normalization. Apply the opaque-key exclusion to the
trimmed source before replacing any separators. Replace role hyphens and
underscores with spaces; collapse whitespace in either label. The result
must be at most 100 UTF-8 bytes, contain a letter, and contain only Unicode
letters, combining marks,
digits, spaces, hyphens, `/`, `&`, `+` and parentheses. Reject the whole
candidate if it contains any other character. Do not strip quotes or
sentence punctuation to make prose pass. The 100-byte bound matches the
maximum goal name and bounds the heading too. It is checked **before** line shortening.
A long display cannot qualify by first being cut to size.

This is a conservative display rule, not language understanding. It may
decline a useful punctuated display. A vague but lawful slug or short label
can still be vague; no deterministic rule can recover missing meaning.
The report keeps the exact source so that can be corrected at its owner.
Do not add an English word list, model call, ledger field or live reread.

If all name sources for the selected record fail, use
`task name unavailable` and keep its ID and original sources in the report.
Do not skip that record or drop to a lower work tier to find a prettier
name. Empty intent does not invalidate a usable goal ID. A selected but
unclaimed goal still says `No task in flight; next: task name unavailable`
when its ID is unusable. Proven empty owned work and no selection produce
`No task in flight`; incomplete ownership/scan evidence produces
`Task unknown`. A known task with a missing name is neither of those states.
Preserve the existing stopped-ledger handling too.

The goal ID is preferable to the intent's first sentence or first clause:
both can still start `Wido, 2026-09-06` and quote the complaint. Combining
either with the slug spends the scarce title space on that same history.
Appending role, run kind or record ID to a usable goal name has the same
cost. Keep those details in the report. The goal's existing words already
identify the work without a summary operation.

Keep Decision 2's shortening rule whole. Reserve the outcome, intervention
words, read imperative, fixed separators and **whole read command** first.
Shorten only the chosen name at a UTF-8 boundary to use the remaining bytes.
A valid long name may be cut, including within its last word; that is display shortening
of an established name, never a way to turn intent into a name. No ellipsis,
wording change or shortening of the command is introduced here. The report
keeps the chosen name before this cut. An over-100-byte goal ID is invalid
source data and takes the fallback above, not a larger naming allowance.

The report heading uses the **same chosen name before console shortening**:
`# Task: stop refusal fits on one screen; Stop blocked`, for this held task
and outcome. It carries at most 100 name bytes, or the fixed unavailable
label. Keep `# No task in flight; next: <name>; Stop allowed/blocked`,
`# No task in flight; Stop allowed/blocked` and `# Task unknown; Stop
allowed/blocked` for their existing states. The heading never carries
intent, a prose job title or a rejected run display. In the report body,
retain the complete intent, original job/run title, role, goal/record IDs
and source path from the frozen facts. The summary identifies the chosen
tier and source, including a missing or rejected name source. Keep actions
ahead of background as Decision 5 requires. Naming changes no source fact,
control decision, intervention flag, report identity or delivery rule.

Derive intervention phrases **only from explicit typed classifications**.
`needsYourDecision` is the OR of `humanRequired`; `needsSupervisionRepair`
is the OR of `supervisionRepair` across the frozen scan items/actions/refusal/
escalation, `health.interventions`, arming components, notices and unavailable
entries that declare the respective field. Producers own classification at
the branch that establishes the condition; the renderer only combines the
booleans. A missing declared classification is invalid presentation input, not an
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

The presentation result is a v2 JSON object with `schemaVersion:2`, the
unchanged `identity` and `control`, `humanLine` string containing the
complete two-line text, the two derived booleans, and `report` object (`id`, `alias`, `path`, `readCommand`, `sha256`
strings). `id` stays the full canonical ID; `alias` is the reserved short
token; `readCommand` uses `alias`; `path` uses `id`.
The report digest is SHA-256 of the published Markdown bytes. It contains
no runtime envelope. The mapper accepts this exact version and validates
both line bounds, the pair bound and report identity before producing a
host payload.

### 5. An immutable report with a cwd-independent read command

Keep each canonical report at its existing immutable path:
`artifacts/agents/supervision/stop-verdicts/<session-key>-<attempt>.md`.
The session key remains the full 64 lowercase hex digits of SHA-256 over
the compact two-string JSON array `[runtime,session]`. The attempt remains
32 lowercase hex digits. Exclusive reservation still prevents overwrite.
The report's full identity is not shortened, truncated or replaced.

**Print the shortest free prefix of the attempt, reserved as an exact
alias in this installation.** Start with one lowercase hex digit, then two,
up to all 32. For example, the supplied attempt starts `15fd2d40...`.
With no prior reservation, its command becomes:

```text
metasystem report stop-status --id 1
```

If `1` is already reserved, try `15`, then `15f`, and so on. Never change
an issued alias. The command is 35 fixed ASCII bytes plus the alias:
**36 to 67 bytes**, instead of 132. One digit is the smallest nonempty
identifier. No fixed minimum of 8 or 12 digits is needed for this guarantee.
Length grows only when actual installation-local reservations require it.

This is an **exact alias, not live prefix matching**. `1` and `15` may name
different reports. `--id 1` always reads the exact reservation named `1`.
A future attempt sharing that prefix cannot make the old command ambiguous.
Do not enumerate matching reports and choose the latest, the same-session
one or the first match. Do not reinterpret an expired alias as a prefix.

`internal/stopreport` owns alias reservation and lookup in the local directory
`stop-verdicts/aliases/`. Its one file per token is `<short-id>.json`.
A published entry has `schemaVersion:1`, `state:published`, `alias`, the
full canonical `reportId`, and the report's complete SHA-256 digest.
Reserve with exclusive create, trying prefixes in length order. Sync the
reservation and directory before publishing a binding; failure is publication
failure, never permission to emit an unreserved token. A reservation
is occupied even if another publisher has not finished or has crashed.
Under the owning reservation, publish the canonical report durably first,
then atomically publish and sync the alias entry, then the presentation
result. Verify lookup before emitting the command. Keep the existing
per-session publication lock; exclusive alias creation arbitrates concurrent
sessions in the same installation. There is no new judgment lock.

Never recycle an alias, including after expiry, a failed publication or a
crash. Keep its small reservation as a tombstone after report rotation.
Thus an old command either reads its original report or fails as expired;
it can never silently read a newer report. Do not reclaim orphan reservations
on a timeout. If every prefix through 32 is occupied, or a full canonical
ID collides, return publication failure and use the retained-decision
unavailable form. Do not mint retries inside the Stop deadline or print
the long ID as a human fallback. Reservation, write and lock failures earn
no extra budget and change no Stop result. This bounded collision path is
presentation failure, not an authorization or policy refusal.

This local alias map deliberately replaces the earlier “no report index”
clause. It is the small durable state needed to keep old short commands
stable across concurrency and pruning. It is not a global report registry,
a mutable latest pointer or a new task source. Alias storage grows with
issued reports; canonical Markdown still follows the existing retention.
That cost buys deterministic non-reassignment, rather than a probability
that short hashes happen not to collide.

**Guarantee:** every successfully published and emitted short command
resolves to exactly one immutable report in its bound installation while
that report is retained. An expired, incomplete, unreadable or damaged
binding fails with no report output. It never returns another report.
The guarantee is installation-local and depends on the verified executable
binding below; a token alone does not identify an installation globally.

The CLI retains required `--id` and optional `--root`. It accepts either an
exact alias of 1 through 32 lowercase hex digits or the legacy exact
`64-hex-32-hex` canonical ID. No other prefix syntax is accepted. The
legacy full-ID command stays available for old reports and recovery.
For an alias, load one regular, nonsymlink reservation file. Reject unknown
versions, duplicate JSON keys, trailing data, multiple targets, alias/name
mismatch, a non-prefix alias for the full attempt, or an invalid canonical
ID. Resolve that single canonical file and run all existing checks.
If a prefix would match several canonical reports, the reservation is the
sole disambiguator. If the reservation itself is ambiguous or corrupt,
fail closed with exit 1 and a recovery diagnostic; do not auto-extend or
retarget a command already printed. Grammar errors remain exit 2.
The report and setup diagnostics give the full-ID recovery command.

Root resolution is unchanged: follow the actual executable's symlinks,
require `<installation>/bin/metasystem`, and use `stateroot.RootForCandidate`.
Never derive the installation from cwd, payload text, another checkout or
a parent repository. An explicit root uses the same validator. Reject
symlinked alias/report directories and any path escape. The report's one
`metasystem-stop-report-v1` marker must still bind the resolved installation,
runtime, normalized session, recomputed full session key and full attempt
to the canonical filename. Compare the full marker to the presentation's
identity and verify the complete report digest at mapping/completion.
Shortening the command removes none of these verification checks.

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
36-to-67-byte bare command is guaranteed only in a verified provider command
environment or a human shell already passing the same probe. A fresh or
unbound human shell is not covered. Setup diagnostics and the report give
the shell-quoted absolute recovery command
`<installation>/bin/metasystem report stop-status --id <id> --root <installation>`
and the full report path; adding `--root` to an unavailable bare executable
is insufficient. This recovery text is outside the bounded Stop notice.
Retain Decision 2's bounds; do not insert arbitrarily deep absolute paths
into it or claim automatic human-shell installation. No profile mutation
or further human authorization is part of this build.

Successful `stop-status` prints the exact Markdown to stdout, no stderr,
exit 0; invalid ID/flags exit 2, missing/expired/unreadable/root mismatch exit
1 with a diagnostic on stderr and no report output. It is read-only: no
judgment, refusal, lease, cursor, scan or claim. All lookup and binding work
is part of the implementation/conformance obligation, not proof of a read.

Write atomically before emitting the pointer. The report starts with Decision
4's task-name heading and **Stop blocked/allowed**, observation time, and any
action needed from the human. Next come seat actions with their commands,
then background counts, then complete diagnostic sections: original turn verdict, uncut open-plan
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
and canonical filename; completion additionally checks the externally retained digest.
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

**Every separate `surface_json` site, re-read at HEAD.** No site may
add a third visible Stop line. The final native envelope is emitted once.
Keep the helper's non-Stop behavior; change its Stop callers locally.

| Hook site | Reachability and disposition |
| --- | --- |
| `722-731`, helper definition | On start it collects; otherwise it makes JSON. Do not change this helper globally. |
| `920`, retro due | Direct output for `event=receipt`. Stop already captures the receipt check at `999-1002`. Keep the due text and exact retro actions only in the report. Ordinary debt adds neither intervention flag and never displaces the task name. No registered Stop may also invoke `receipt`. |
| `922`, receipt-check error | The other direct receipt-event notice. Move its diagnostic/remedy into the collected report and fold `needs supervision repair` into line 2. No second receipt registration. |
| `1184`, failed refusal-record write | Captured in `response=$(...)`, not separately emitted. Keep cause, owner, remedy and record/log evidence in the report; fold typed repair into line 2. Preserve the retained allowance or independent block. Remove this obsolete display construction when the composer returns facts. |
| `1212`, unreadable repeated-failure record | Same captured-response disposition; preserve its exact failure evidence and existing control precedence. |
| `1249`, repeated infrastructure allowance | Same captured-response disposition. Keep repeated-failure detail in the report, repair on line 2, and the unchanged allowance. |
| `1468-1469`, unavailable turn verdict | Captured and then replaced by `fallback_stop_payload false` at `1470`. Keep the original diagnosis in attempt/log evidence. Emit only the existing one-line unavailable allowance when no trustworthy report can be published. Do not add a report pointer or manufacture completion. |
| `1484`, pending steward incidents | Runs after Stop's unconditional exit at `1476`. Keep for start/end, where it is an undelivered-incident recovery route. It is not an extra Stop notice. |
| `1490`, unused stop-authorization retirement failure | SessionEnd only. Keep: it warns that future authorization is unsafe. It cannot be reached by Stop. |
| `1493`, unidentified agent at end | SessionEnd only. Keep its arming refusal. |
| `1505`, unidentified agent at start | SessionStart only. Keep its collected startup warning. |
| `1520`, rebuilt-engine rearm | SessionStart only. Keep. Stop's separate `up_notice` at `945-954` is already captured and belongs only in the report. |
| `1541`, startup arming failure, possibly rearm detail | SessionStart only. Keep. Stop arming failures go to its report plus the classified line-2 flag. |

The source template already removed Claude's separate receipt Stop hook
(`scripts/enforcement/claude-code-hooks.json:16-25`). The supplied observation
therefore requires an **effective installed-hook** check, including retained
or duplicate user/project registrations. The implementation cutover removes
obsolete metasystem receipt registrations in generated installations and
proves exactly one governed Stop producer. Do not disable unrelated hooks
or call a template-only check proof of the human's total count. If unrelated
host hooks add lines, record that installation as nonconforming; no silent
exception to the two-line promise. Direct manual `receipt` invocation may
retain its output outside a Stop event.

Also account for output outside `surface_json`: advisor `1307-1312` and
ordinary `1472-1476` Stop exits emit only the shared pair; engine-free,
launcher and deadline paths emit only their existing one-line fallback.
Health, digest, watchdog, holder protocol, brain-post failures, arming,
condition-log and refusal-record failures are collected report detail with
only typed intervention flags on line 2. No trim notice is added. Preserve
all existing logs, failure records and cursor conditions. Diagnostic stderr
must remain captured; it cannot become an uncounted human warning line.

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
intervention phrase and title-only shortening as Decision 2. The landed
standing instruction supplies the `goal next` recovery path. If no detailed
channel or readable report exists, full seat guidance is **not delivered**;
mark delivery degraded, never successful. The hook cannot promise both full
detail and one line under that failure, nor can another report read
defeat the host's continuation cap. This is the milestone limit accepted in
Decision 3 and carried by the separate continuation goal; it never clears
an established block or permits a second human notice.

Revision 7 keeps every unavailable form above byte-for-byte unchanged.
A one-line degraded status is within “at most two lines”. It carries no
completion assertion and no invented report command. The report-bearing
forms have the imperative; a missing report cannot have one with a bogus
locator. Alias failure uses this same retained-decision fallback. Alias publication or lookup failure leaves delivery cursors unchanged.
Completion collection/comparison never writes a cursor. If a valid report
explicitly records unknown completion, its successful delivery uses the
existing digest/protocol conditions; unknown completion adds no new gate.

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

**Revision 7 cutover, including revision 6's unbuilt work:** the presenter,
mapper, steward completion verifier and shared fixture reader change
together. Validate the complete decoded pair, the outcome-specific
imperative, the exact alias command, the canonical full identity and digest,
and byte equality with the two console lines frozen in the report.
The internal result remains one `humanLine` string; it contains one LF.
Update the report's console-text representation to a two-line literal block
so both lines can be compared exactly. Never accept arbitrary text before
an ID, a clipped command, `status:` as a newly emitted form, or an allowance
whose imperative does not require continuing lawful work. Legacy reports
stay readable by their old full IDs; no immutable report is rewritten.
A stale consumer's rejection takes the retained-decision fallback and is
recorded as degraded delivery. It cannot clear a block or claim success.

`HookDeliveryReference` keeps the full canonical ID, path and expected
digest and additionally carries the exact short alias used in the command.
Mapper and completion resolve the alias, then compare that canonical target
and full marker against their externally retained evidence. Do not compare
a short alias to `sessionKey + "-" + attempt` or make a canonical filename
from it. The setup probe and fixture reader use the same lookup contract.
Put the existing identity/lookup code and alias reservation in the small
`internal/stopreport` storage package. `internal/report` already imports
steward, so steward cannot import report to reuse its lookup. Both import
this lower-level owner. It imports neither. Report still owns presentation
and calls storage to publish/resolve. Adapter and completion share exact
resolution while independently comparing against their retained identity
and digest. Do not duplicate prefix matching in shell. The named consumers
are why this extraction is needed; it adds no registry framework.

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

Revision 7 adds or extends these focused fixtures. Revision 6's six
instruction obligations are folded here, not pending as a separate build.
Expected strings are literal examples from this page; do not generate
expectations with production formatting or lookup. No whole fixture bed
is requested by this design.

| Proof row and fixture | Required observation |
| --- | --- |
| STOP-TWO-LINES — `TestStopTwoLineStatusBounds` in `internal/report/stoppresentation_test.go` | Exact first and second lines for block/allow and all task states and flags. One LF; line bounds 144/256; pair bound 401 including LF. Exercise maximum 32-byte alias and actual shortening budgets 117/98/74/55, plus a one-byte alias, long valid names and a Unicode cut. Assert complete outcome, imperative and command and uncut heading. Reject third lines, blank extra lines, CR, controls, U+2028/U+2029 and invalid UTF-8 at presenter/mapper boundaries. Existing one-line unavailable forms stay byte-identical and within 256. |
| STOP-COMPLETED — `TestStopCompletionUsesNewOwnedTerminalRecords` in `internal/report/stoppresentation_test.go` | Literal delegate-return and run-passed forms. An active record becomes completed/green, and another is created and finishes between observations. Choose latest ended job, then highest-sequence run only if no new job; pin equal-time/ID ties. A repeated Stop says `none recorded this turn`. Report retains all completions, original sources and baseline identity. A green run that entered draining before the baseline is still new by terminal identity. No landing or goal-done claim. |
| STOP-COMPLETED-UNKNOWN — `TestStopCompletionRejectsUnprovedAttribution` in the same file | First use, legacy baseline, removed baseline, changed main/session/runtime/installation, conflicting parent-seat identity, corrupt/future baseline, missing/ill-typed record or timestamp produce `unknown for this turn`. Other-seat jobs on the same goal/machine never supply either line. Child runtime/session differences alone do not reject an owned delegate. Receipt/check success, a human conclusion, disappearance, failed/cancelled/timed-out jobs and red/unknown runs never prove success. Concurrent/stale observation tests pin honest intervals without touching control/cursors. |
| STOP-COMPLETED-CAPTURE — `TestReportTurnVerdictCompletionCapturePreservesVerdict` in `cmd/metasystem/session_stop_test.go` | The existing scan captures all job/run presentation evidence once without adding terminal jobs to judgment arrays. Sidecar writer failure preserves completed stdout/exit/control and leaves no partial/stale sidecar. Hook still publishes its usable report with completion unknown and typed repair. Compare policy inputs and every state write to the baseline; no new judgment, refusal spend or cursor. |
| STOP-SHORT-ID — `TestStopAliasReservesShortestFreePrefix` in `internal/stopreport` tests | With attempt `15fd2d40fae99afd9756ffd62c6903a7`, reserve `1`; with `1` occupied reserve `15`; with both occupied reserve `15f`. Assert the complete 36/37/38-byte printed commands. Reserve every shorter prefix to exercise a 32-byte alias and its 67-byte command. All occupied through 32 and full-ID collision use publication failure with retained control. |
| STOP-SHORT-ID-UNIQUE — `TestStopAliasIsExactAndNeverReassigned` in `internal/stopreport` tests, plus command tests | Deterministic concurrent sessions/attempts sharing prefixes get distinct exclusive reservations. Both `1` and `15` resolve only their original canonical reports. Add later reports that make the raw prefix ambiguous: old commands remain exact. Prune the first report, leave its reservation and create another matching attempt: the old command fails expired and never selects the new report. Crash before/after each publication step leaves no alias that resolves to wrong/partial bytes. Orphan reservations are not reused. |
| STOP-SHORT-ID-BINDING — `TestStopAliasRetainsFullIdentityVerification` in storage, adapter and completion tests | Alias substitution, duplicate keys/targets, wrong alias/name, invalid canonical ID, symlink/escape, wrong installation/runtime/session/session key/attempt and modified digest fail with no report output or successful delivery claim. A damaged ambiguous reservation fails; it never guesses or auto-extends a printed token. Identical sessions/tokens across installations stay isolated. Legacy exact full IDs still read unchanged old reports. Printed alias command, canonical path, full marker and retained digest must all agree. |
| STOP-NOTICES — `stop-notices-two-line-total` focused row in `scripts/agents/supervision-hook-fixtures.sh` | Invoke the actual effective Stop registrations with retro due and with receipt-check error. Count all decoded visible text across every emitted object and hook registration, not just the final payload. Exactly one pair, no third notice; retro stays in report, error adds repair to line 2. Check stale/duplicate receipt registration detection and generated removal. Exercise each captured composer branch and unavailable verdict; separately prove start/end notices still run and cannot be reached from Stop. |
| STOP-READ-IMPERATIVE — `TestStopReadImperativeByOutcome` in presenter tests | Require `; Read: ` on block and `; Read, then continue lawful work before stopping: ` on allowance, no `status:`. Compare the complete decoded pair and every flag combination. Full report actions, control and identity remain equal. |
| STOP-READ-ALLOWANCE — extend `TestStopAllowanceLineRetainsThirdRefusalAndDelegateWork` and `TestPresentStopStoppedJudgmentIsAnOrdinaryAllowance` | Third-refusal held/next work, failed preparation, delegate exemption, empty work, unknown work and stopped ledger retain their actual control and task prefix. Every report-bearing allowance gets the same read/continue imperative. It grants no claim, repair, rearm or human-stop override and forces no new turn. Retain exact report steps and restrictions. |
| STOP-READ-BOUNDS — `TestStopReadImperativeBounds` | Companion to STOP-TWO-LINES: reserve the complete longer allowance imperative and 32-byte alias before cutting the task name. Pin a next-name allowance with both flags at 256 bytes and a maximum first line within 144; assert the pair's 401-byte bound. A shorter alias frees title bytes; it cannot change the chosen name or report identity. |
| STOP-READ-BINDING — affected `internal/adapter/stopoutput_test.go` and `internal/steward/component_evidence_test.go` cases | Accept only the correct pair/imperative/alias for the retained outcome. Reject a bare label, old allowance imperative, wrong outcome's imperative, truncated command, substituted alias or altered console-text block in the report. Preserve full-ID, marker/digest checks and block/allow fallback with no cursor advance. |
| STOP-READ-HOOK — extend STOP-TASK-NAME-HOOK at `scripts/agents/supervision-hook-fixtures.sh:790-828` | Drive both outcomes through the invoked presenter and mapper. The shared `fixture_stop_status_report` decodes two lines, validates bounds, runs the exact alias command, and compares full canonical identity/digest and all report bytes. Keep literal slug names, dated intent, mapper count, block source and delivery evidence. Repeat missing-report/alias/mapper failures with unchanged degraded output. |
| STOP-READ-TWO-CARRIERS — installed-instruction checks and STOP-HOST | Assert trunk's canonical rule and actual loaded entrypoint/hash cover both outcomes, reading and lawful action. Run line fixtures without loading any instruction file. Real-host evidence must show both the rule and the imperative delivered, the exact report read, and lawful action on block and allowance. Record no-opportunity allowances as a delivery limit, not a pass. Neither carrier alone satisfies this row. |

Revision 5 added the following naming rows in
`internal/report/stoppresentation_test.go`, plus the named hook row. The
existing shape/byte assertions stay. They are necessary but did not catch
the naming defect: `TestPresentStopPublishesOneBoundedLineAndCompleteImmutableReport`
at lines 93-129 accepts any text after `Task: `, while
`fixture-stop-report.sh:24-43` checks the envelope, bound and locator only.
These are historical anchors from the revision 5 grounding. Those rows
have landed. Preserve their naming contracts; only their literal imperative
suffixes and resulting line-2 name budgets change in revision 7; add the
completion line independently.

Each naming row must assert the **exact task text before the outcome and
exact first heading line**, chosen from its input and this decision. Do not
derive the expectation by calling the production name picker or truncator.
Require equality of decoded line 2 to that literal task text
plus the full expected outcome/intervention/command suffix. A prefix match
alone could allow prose after a correct name. For the reported defect,
use the complete intent at
`plans/goals/stop-refusal-fits-on-one-screen.md:8`, including its date and
quotation, with its real slug. Require `stop refusal fits on one screen`
as the name and exclude `Wido, 2026-09-06` and the quotation from both title
and heading. Assert the complete intent survives in the decoded frozen
facts in the report body. A bound, an arbitrary nonempty title, absence of
newlines or the word `Task` alone cannot satisfy a naming row.

| Revision 5 proof row | Required observation |
| --- | --- |
| STOP-TASK-NAME-GOAL | Held and selected-but-unclaimed subcases use the real slug and full dated intent. Pin `Task: stop refusal fits on one screen` and `No task in flight; next: stop refusal fits on one screen`, with their exact headings and supplied outcome. Changing only intent, including to empty or a short sentence, leaves the name unchanged. A different slug changes the name. Test names: `TestStopTaskNamesUseGoalIDsInsteadOfIntent`. |
| STOP-TASK-NAME-JOB-RUN | An owned job and an owned active run each carry the real goal ID, a conflicting role/display and a prose title. Both must use `stop refusal fits on one screen`. With no usable goal ID, role `design-review` gives `design review`; run display `Stop report checks` gives exactly `Stop report checks`. Long, quoted or multi-sentence titles cannot supply names. Preserve every original title and source in the body. Test name: `TestStopTaskNamesForOwnedJobsAndRuns`. |
| STOP-TASK-NAME-PRECEDENCE | Distinct literal names prove job over run over held goal over selected claimable goal. Cover other-seat and unknown ownership, terminal records, reversed scan order, equal dates, missing dates and ID tie-breaking. An unnamed winning job/run says `task name unavailable` despite a named lower tier. Verify counts and other records in the report. Proven empty work says `No task in flight`; incomplete evidence says `Task unknown`. Keep the stopped-ledger case. Test name: `TestStopTaskNamePrecedence`. |
| STOP-TASK-NAME-FALLBACK | Empty, whitespace-only, numeric-only, punctuation-only, malformed and overlong goal IDs, a UUID and 32/40/64-digit hexadecimal keys use the specified fallback. A job role that fails the short-label rule and a run with only a prose display yield `task name unavailable`. Include prose shorter than 100 bytes with a quote/date and prose longer than 100 bytes with no punctuation: neither may be clipped into eligibility. A missing name never changes work selection, control or flags. Valid Unicode labels and benign role separators remain usable. Test name: `TestStopTaskNameFallbacks`. |
| STOP-TASK-NAME-BOUNDS | A valid goal slug near 100 bytes and an accepted Unicode run label put the cut inside a multibyte character. Pin literal shortened names for held and `next` forms, both outcomes and all four intervention combinations. The line stays valid UTF-8 and at most 256 bytes. The exact complete command, outcome and intervention words survive. The first report line retains the full accepted name, at most 100 name bytes, even when the console cuts it. A fallback label over 100 bytes is rejected whole. Test name: `TestStopTaskNameBoundsAndReportHeading`. |
| STOP-TASK-NAME-HOOK | Add a focused hook row beside the existing bounded-line case at `supervision-hook-fixtures.sh:620-665`. Drive a frozen held goal with the real slug and dated intent through the actual presenter and invoked mapper. Decode the sole native field and require the literal name; run its printed command and require the exact heading plus complete body intent. Include a block with both intervention flags, matching the reported failure, and an allowance. Keep the same control/source, one visible pair and report identity. This row must fail against revision 4's intent-based title even though its old shape assertions pass. |

The table below carries revision 4's remaining proof obligations whole.

| Proof | Required observation |
| --- | --- |
| STOP-REPORT-INPUT: huge input, block and allow | Combine 200 stale red runs, an owned unwatched job with an exact watch command, eight long plan steps, long questions/human waits, a health line over 2,000 bytes and a 40-line digest. Report contains every uncut fact, title, command and collected digest byte; one judgment/count only. Unknown schema, wrong types and mismatched identity fail before publication. Independent allowed input creates no continuation. |
| STOP-RUNTIME-MAP: one notice | Decode every emitted object: block has compact reason and no systemMessage; allowance has only systemMessage. Total human Stop text is the single pair within 144/256/401 bytes, without an ancillary notice; the retained degraded exception is one line within 256. Pin the actual invoked mapper and generated registration; prove obsolete installed receipt Stop registrations are removed. Unknown mapping does not gain conformance. |
| STOP-INFRA-DISCLOSURE: superseded public fields | Combine a real seat-actionable block with an arming infrastructure failure: the only text field is reason, the line contains `needs supervision repair`, and the report leads with the full infrastructure cause/owner/remedy. Assert unchanged block source, hook log, stop-condition line and refusal record. Infrastructure alone still allows with the flag solely in systemMessage. Missing report retains block plus repair/unavailable line. Update the landed root design's 2968-2973 reject clause and both pages' disclosure assertions in the same build. |
| STOP-PRESERVATION-IDLE: unchanged decisions | Pair attempts 1, 2, 3, 4 and later against trunk, including successful/failed preparation, independent blocks, live/pending-setup delegates and lost counter. Preserve each source, count/reset, intent/incident/alarm call and authorization result. Do not add an escalation attempt/result slot or assert once-only escalation across later Stops. |
| STOP-PRESERVATION / TestMismatchedSessionStopMarkerDoesNotGateIdleEscalation | Keep `internal/goal/turnverdict_idle_test.go:338-370` unchanged in outcome: attempt 3 allows and `prepared == 1` across its three calls. Retain the mismatched main's display-only SESSION STOP authorization marker and prepared intent detail in the report. The marker neither authorizes the allowance nor gates escalation; the existing bound releases it. |
| STOP-PRESERVATION / TestReportTurnVerdictHeldClaimWritesRealSeatIdleIntent | Keep `cmd/metasystem/session_stop_test.go:258-371` unchanged in outcome: exit 0 throughout, attempts 1/2 block and attempt 3 allows. Retain `--stop-hook-active` and exactly one real persisted intent across those three calls: reason `seatIdle`, goal `held`, `claimNeeded=false`, machine `bed-m1`, lineage `main-1` and current holder's claim epoch. |
| STOP-ALLOWANCE-LINE: retained work | Third-refusal release shows the held task or `No task in flight; next: <title>` with `Stop allowed`. A delegate exemption shows the owned delegate task with `Stop allowed`. Both require revision 7's read/continue imperative; neither adds a blanket work claim. Neither alone adds an intervention phrase; failed escalation adds the typed repair phrase and full report remedy, with the decision phrase only for required human repair. Decode both sole fields, check the pair bounds and retained work in the report; neither creates a forced continuation. |
| STOP-DELIVERY-SNAPSHOT: expectation versus observation | `internal/adapter/snapshot_test.go` and runtime/installation tests assert that Stop-specific registry data is only ExpectedStopDelivery and reuses existing InstructionFile, with no installation evidence. A dated snapshot retains the outer host version/configuration/time and capabilities.stopDelivery's instruction hash, launch/seat binary, artifact and achieved level. Missing observations remain unknown; changed version/config/instruction invalidates observed reuse. A declaration alone cannot pass the conformance gate. |
| STOP-FACTS-WRITER-FAILURE: completed judgment survives | Add `TestReportTurnVerdictFactsWriteFailurePreservesCompletedVerdict` in `cmd/metasystem/session_stop_test.go` and a hook integration row with this identifier. Inject the auxiliary writer failure after a real judgment: one scan/judgment/refusal spend, exit 0, byte-identical existing Verdict JSON plus newline on stdout, diagnostic only on stderr, facts path absent with no partial/stale publication. Hook consumes stdout once, keeps shouldBlock/source, emits only reason with repair/Status unavailable, no bogus pointer, no delivery-cursor advancement. Repeat for a completed allowance without creating a continuation. |
| STOP-HEALTH-JSON: CLI forms | Unflagged and text previews remain byte-identical; JSON decodes to v1 with all role reasons/remedies and matching line/exitCode. Assert one evaluation, no observation/alert writes, stopped/spend/unknown cases and invalid format handling. |
| STOP-INTERVENTION-RETRO-ONLY: negative | In the health preview and presenter, supply only ordinary agent-actionable retro debt: status dead, NoAutomaticRemedy true, full retro/receipt remedy; both classifications and both derived flags are false. Neither intervention phrase appears; the report retains the debt and exact action. Control stays at the supplied verdict. |
| STOP-INTERVENTION-SPEND-ONLY: negative | Supply an otherwise healthy preview with an alive spend crossing and nonempty ceiling-change remedy; both classifications and both derived flags are false. Neither intervention phrase appears, and the crossing/remedy remain report attention. An independent typed human-required fact in a paired case turns on only the decision phrase; spend itself remains report-only. |
| STOP-INTERVENTION-TYPED: positive and rejection | Exercise neither flag, decision only, repair only and both; assert exact phrases and order. Reject missing/wrong-type v1 booleans and missing/duplicate health intervention roles. Changing only remedy text, NoAutomaticRemedy or generic health status cannot change a producer-supplied classification. Required human-only repair remains classified as both with its restrictions in the report. |
| Summary correctness | Alive/dead/unknown and stopped health, alive spend crossing, other-seat/unattributed pages, 11 claimable goals, explicit green continuation and no-continuation history. Counts derive from facts, and every action retains its complete command or honest manual step. |
| STOP-LOCATOR: bounds and lookup | Newlines, controls, Unicode at byte 200 and long titles; deep installation, nested/external cwd, symlink launch, wrong/missing executable binding, identical sessions under different installations. Run the **printed command** and match exact Markdown and identity before emission; reject invalid IDs/escapes. Assert the command is complete, line 2 stays within 256 bytes, and the whole pair satisfies Decision 2. |
| STOP-LAUNCH-BINDING: read-only setup probe | In `internal/hostsetup/setup_test.go`, `cmd/metasystem/runtime_setup_test.go` and host launch tests, prove each claimed start/resume route gives the child the validated installation/bin prefix and keeps provider argv intact. Run `--check --stop-status-id` under that actual command environment with a known report from nested/external cwd and a symlinked executable. Assert correct identity/exit 0; wrong/missing binary and missing report exit 1; invalid flags/ID exit 2. Compare profile, rc, configuration, report, snapshot and control-state bytes before/after the probe: no writes. A separate unbound human shell is explicitly unsupported and receives the absolute recovery command in setup diagnostics, never an over-bound Stop line. |
| STOP-HOST: conformance prerequisite | For each claimed runtime/version/config/instruction hash, observe effective trusted hooks, total human transcript, block-to-exact-report-read-to-action in the provider's actual command environment, and an allowance with its imperative and no forced continuation. Revision 7 carries allowance-read/action evidence or an explicit host opportunity limit, and both instruction carriers. Keep missing-report and continuation-cap observations. A real host is required; an adapter harness cannot discharge this row. Store the evidence in the dated snapshot. Report the finite range honestly; continuation beyond it belongs to seat-work-continues-past-the-runtime-stop-cap. |
| Storage and races | Concurrent Stops in one session, same session ID under different runtimes/installations, hostile session text, report write/rename/durability failure, failed stdout, retention boundary, stalled completion after published block. Verify exact report-to-payload identity, no cross-session replacement, unchanged control, no false `EMITTED`, no cursor advancement after lost delivery. |
| Degraded fallback | Engine missing/skew, launcher failure, invalid/partial worker output, unresolved root, timeout, record failure, log failure and combined failures. One bounded line, truthful unavailable status where needed, original allowance and recorded causes unchanged. Pair an arming/report failure with a real seat-actionable block and prove it remains blocking. |

Historical revision-4 fixture anchors are retained below. Their behaviors
remain obligations; line numbers in this historical inventory are not
revision-7 HEAD citations. Revision 7's current seams are cited above:

| Bed and anchors | Assertions retained or relocated |
| --- | --- |
| `supervision-hook-fixtures.sh:20-36,179-200,778-801,873-957` | Launcher and missing-engine allowances; runtime-list, hook-attempt, verdict, malformed-verdict and partial-output failures; arming occurrences 1/2, exact component remedy, different health/narrator cause, fresh session, broken refusal record. Full notices move to report content; engine-free cases pin compact cause/remedy literals. |
| Same bed, `395-419,529-572,611-670,839-870,1082-1113` | Installation digest/root, current health and cursor; all open-work first/second/reset/deadline-marker rows; 200-run evidence; start/Stop re-arm success/failure; killed worker and following attempt. Keep the old verdict-file assertions, add Markdown report assertions, and move the console bound to the human field. |
| Same bed, `1150-1267,1323-1357,1514-1581` | Preserve deadline outcomes, records, missing-engine/log failures and consumed human authorization. The template-backlog fixture at 1535-1581 keeps its first/second blocks and third-Stop allowance at 1564-1574. Retain intent bytes at 1575-1581. Keep escalation detail in the report; apply Decision 1's allowance wording instead of predicting that the turn will end. Only display assertions move. |
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

The six policy tests keep their names and trunk Stop assertions. The line
numbers below retain the historical inventory, not a fresh HEAD claim.
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

The first four rows below assess revision 7's unbuilt delta. The remaining
rows retain the landed obligations as preservation requirements. Their
historical pre-build statuses are not a recertification or a request to
rebuild the whole milestone. All named proofs must be satisfied before the
corresponding implementation claim; none is claimed run by this design.

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| STOP-READ-INSTRUCTION | HIGH | Decisions 2-3, 7, revision 7; revision 6 folded | Exact two-outcome imperatives and landed standing rule both required; allowance reads and continues lawful work | internal/report; adapter; steward verifier; canonical AGENTS.md/runtime loading | Formatter and exact-command consumers | All six STOP-READ rows | Observe both carriers and exact report read/action on both outcomes; state host limits | PARTIAL | Build imperative in the same cutover; preserve landed AGENTS.md |
| STOP-COMPLETION | HIGH | Decision 2a | Attribute only new owned terminal successes; honest none/unknown; no changed judgment inputs | report collection/presentation; goal CLI sidecar boundary | Completion observation, baseline comparison and frozen report | STOP-COMPLETED, UNKNOWN, CAPTURE | Inspect same-seat report sequence and literal first lines | PARTIAL | Build display projection and prove no decision/state change |
| STOP-ALIAS | HIGH | Decision 5 | Shortest free exact alias; one immutable target; never reuse; full identity and digest retained | internal/stopreport; report publisher; adapter/steward consumers | Exclusive reservation, atomic binding, strict lookup and canonical verification | STOP-SHORT-ID, UNIQUE, BINDING | Run exact commands before/after concurrent publication and expiry | PARTIAL | Build alias publication/lookup in the shared storage owner |
| STOP-TWO-LINE-TOTAL | HIGH | Decisions 2, 7 | Normal first/second/pair bounds 144/256/401; no third notice in actual Stop display | report, adapter, hook and installed registrations | Pair validation and complete emission inventory | STOP-TWO-LINES, STOP-NOTICES, STOP-READ-HOOK | Count actual host transcript across all registrations | PARTIAL | Build pair and remove obsolete installed receipt producer |
| STOP-TASK-NAME | HIGH | Decision 4, revision 5 | Name the selected work from its goal ID or whole fallback label; prose stays in the report body; preserve precedence, shortening and control | internal/report | stoppresentation.go: stopTitle, availableTitle, compactStopLine, renderStopReport | STOP-TASK-NAME-GOAL, JOB-RUN, PRECEDENCE, FALLBACK, BOUNDS and HOOK rows above | Read the decoded hook line and the exact report reached by its complete command with the dated-intent specimen | PARTIAL | Preserve the landed name picker and literal names/headings; update only line-2 formatting |
| STOP-LINE | HIGH | Decisions 2-3 | Two bounded lines in the sole visible field, no extra Stop notice | report and adapter | stop-present, MapStopOutput, runtime capability, all registrations | STOP-RUNTIME-MAP, STOP-LOCATOR, STOP-ALLOWANCE-LINE | Observe total actual host display | MISSING | Extend the landed mapping to the pair; prove actual host behavior |
| STOP-PRESERVATION | CRITICAL | Decision 1; headline proof | Every Stop decision and source equal trunk's for the same input; presentation failure creates or clears no block | goal, goal CLI, report, adapter and hook | Retained enforceIdleBacklog/escalateIdleBacklog; facts writer, mapper/fallback and both parent validators | Six unchanged Go control tests and template-backlog decisions; STOP-PRESERVATION-IDLE, STOP-FACTS-WRITER-FAILURE, STOP-INFRA-DISCLOSURE and degraded cases | Reproducible paired trunk/candidate table with raw verdict, source and native payload for every matrix case | MISSING | Preserve policy, build presentation and prove equality before acceptance |
| STOP-FACTS | HIGH | Decisions 4, 6 | One uncut frozen judgment and explicit health schema; auxiliary write failure preserves completed control | goal, report scanner and health CLI | goal.go facts-file projection/writer; scan.go; steward_verbs.go | STOP-REPORT-INPUT, STOP-FACTS-WRITER-FAILURE, STOP-HEALTH-JSON | Inspect report tails and actions; observe preserved block after failed auxiliary write | MISSING | Preserve uncut facts and the exact stdout/stderr/exit contract |
| STOP-INTERVENTION | HIGH | Decisions 2, 4, 6 | Only typed required action/repair facts add the respective phrase; spend and ordinary retro debt remain report-only | goal/report fact producers, steward health and hook boundary; report aggregates | Classified fact projection, HookHealthPreview.interventions, stop-present | STOP-INTERVENTION-RETRO-ONLY, STOP-INTERVENTION-SPEND-ONLY, STOP-INTERVENTION-TYPED | Inspect actual line-2 flags and matching report actions | MISSING | Preserve producer classifications and combination without text/status inference |
| STOP-DETAIL | HIGH | Decisions 4-5, 7 | Matching readable report and truthful delivery evidence within verified command environments | report, hostsetup, host launch plumbing and steward | stop-status, host-common.sh child PATH, runtime setup --check probe, CompleteHookAttempt | STOP-LOCATOR, STOP-LAUNCH-BINDING, storage/race/cursor cases | Read printed command from the actual provider command tool; separately classify human-shell binding | MISSING | Publish and verify lookup before emission; never mutate shell profiles |
| STOP-HOST | CRITICAL | Decision 3 | Runtime may claim only observed finite control/display/read behavior | adapter conformance; runtimes owns static expectation only | ExpectedStopDelivery, adapter mapping, snapshot.go capabilities.stopDelivery, canonical read instruction and runtime entrypoints | STOP-RUNTIME-MAP, STOP-DELIVERY-SNAPSHOT, STOP-LAUNCH-BINDING; installed-config checks | Per-runtime trusted firing and report-read/action observation tied to version/config/instruction | MISSING | Record dated installation evidence before upgrading a runtime's claim |
| STOP-FAILURE | HIGH | Decisions 3-4, 7; STOP-PRESERVATION | Degraded causes survive; presentation failure neither clears nor creates a block; sole line flags repair | hook and report | Parent, composer and launcher fallbacks; preserved condition/refusal writers | STOP-INFRA-DISCLOSURE, STOP-FACTS-WRITER-FAILURE and all degraded rows | Controlled combined block/arming failure, missing-engine and deadline firings | MISSING | Preserve precedence, records and degraded literals during the pair cutover |

## Build list

Only revision 7 is the next build. Revision 6 has no separate pending item.
The earlier landed implementation is the baseline, not a new whole-bed job.

1. Preserve STOP-PRESERVATION first: retain every outcome, source, count,
   authorization, escalation side effect, engine/deadline fallback and
   subordinate skip. Keep the six named tests and template-backlog decisions.
   Use focused paired baseline/candidate observations for changed paths.
2. Build STOP-COMPLETION from the existing record reads and a separate
   presentation sidecar. Do not change live scan membership, frozen
   judgment schema or state writes. Compare frozen observations in reports;
   prove success attribution, first use, repetition, failures and ownership.
3. Build STOP-ALIAS in the shared storage owner. Keep canonical filenames,
   full identity and legacy reads. Prove shortest-prefix reservation,
   concurrent uniqueness, permanent non-reuse, corruption and expiry.
   No extra retries or timeout allowance. Keep launch binding, read-only
   setup probing and the narrow human-shell guarantee.
4. Build STOP-TWO-LINE-TOTAL and folded STOP-READ-INSTRUCTION together.
   Cut over presentation v2, mapper, completion verifier, fixture reader,
   deadline validators and generated installations together. Keep the
   canonical AGENTS.md:36 instruction landed by b643f423. Change the report's
   console-text block and full-reference metadata consumers together.
5. Apply the entire Decision 7 emission inventory. Inspect effective hook
   registrations, not only source templates. Move receipt notices and
   captured infrastructure detail into the report and fold only typed flags
   into line 2. Retain non-Stop notices, degraded literals, logs, refusal
   evidence and delivery-dependent digest/protocol advancement. Update the
   previously named public-rendering documentation owners in the build;
   this design lane edits none of them.
6. Run the named focused Go cases and affected hook rows. Read the computed
   code diff for preserved decisions; perform required implementation
   code-critique and end-to-end verification. Do not rerun whole fixture
   beds solely for this design correction. STOP-HOST remains a separate
   real observation for each claimed installation/runtime/version: both
   carriers, total human lines, exact report read and action on an allowance
   with remaining work. If the host affords no action opportunity, record
   that limit; no claim of a reliable observed allowance loop. Continuation
   beyond the provider cap retains its existing separate goal and ordering.

## Revision record

**Revision 7, 2026-09-14:** Wido's morning restatement changes the target
from one task/status line to at most two useful lines: what just completed
and current or selected next work. It also makes the human's total visible
count authoritative and rejects the 97-character hex identifier. The
supplied case was an allowance, so the imperative must explicitly require
continuing lawful work before stopping, not merely label a report.

Choose newly observed owned terminal job/run success for completion, with
literal none-recorded and unknown forms. Do not borrow another seat's
receipt, landing or human conclusion. The report states the observed work
interval and the first-use limit. Keep goal-slug task naming and precedence.
Normal bounds are 144 bytes, 256 bytes and 401 bytes for the pair with LF.
Keep degraded single-line forms and every Stop decision unchanged.

Choose the shortest free attempt prefix as a permanently reserved exact
alias, from one to 32 hex digits. Exclusive reservation and non-reuse give
one immutable target in this installation while retained, then an explicit
read failure. A later prefix collision cannot retarget an issued command.
Full canonical identity and SHA-256 verification stay inside the report and
its delivery checks. This intentionally replaces the old no-index clause.

Every `surface_json` site has a reachability and disposition row. Retro
and receipt-check failures go into the report; error flags fit line 2.
Captured Stop responses add no separate notice. Later start/end notices
remain at their own events. Effective installed receipt registrations are
part of the proof because trunk's template already removed the extra hook.

Revision 6's unbuilt imperative, strict consumer cutover, allowance read,
both carriers, degraded behavior and proof obligations are folded here.
`b643f423` has landed the canonical AGENTS.md carrier. It is not still
pending in a companion lane. The Stop carrier is still required and neither
may be deleted as redundant. No new human choice, ledger write or third
design critique round is requested. The outer-continuation goal retains
its full obligations and lands its mechanism before closing allowances.

Completion check for this design: the page specifies the observable forms,
owners, failure behavior and named proof targets. This lane re-read HEAD
sources, both current trunk blobs and the computed page diff, and calculated
the byte bounds. It ran no code test, fixture, hook, host or whole bed.
Only this page was edited, and it remains uncommitted. Implementation,
focused fixtures and host observations are still unverified; this is not
a runtime-conformance claim. Nothing remains for Wido to decide.

**Revision 6, 2026-09-14 — superseded; unbuilt decisions folded into revision 7.**
The allowed seat had listed next steps and stopped. Revision 6 replaced
`status:` with a read imperative, required both the Stop and standing
instruction carriers, and removed deliberate allowance-read deferral.
It reserved the imperative before cutting the title and named six strict
text/binding/delivery proofs. Revision 7 carries those duties, strengthens
the allowance to read and continue lawful work, shortens the command and
adds completion as line 1. Revision 6 is historical, not a second open build.

**Revision 5, 2026-09-14 — landed; naming retained.** The intent-based title
printed Wido's dated complaint and cut it mid-word. Use the goal slug as
words. Owned jobs fall back to a whole role; runs to a whole short display.
Prose stays in the body. Six literal name/heading/precedence fixtures replaced
the inadequate shape-only proof. No naming or ownership decision is reopened.

**Revision 4, 2026-09-13 — landed; preservation retained.** Wido required
that changing Stop text must not break loops. Restore every trunk Stop
decision. Keep third-refusal release, failed intent and delegate exemption.
Transfer persistent blocking and its escalation-once state to the already
opened continuation goal, whose mechanism must land before allowance closure.
Retain immutable full reports, full identity, typed interventions, exact
health projection, launch binding, degraded forms and real-host proof.
Revision 7 supersedes only the display/lookup portions stated above.

**Revisions 1 to 3 — superseded.** Their control inversions and unbounded
continuation claims are not build authority. Revision 3's six accepted
findings remain covered: full preservation-test inventory; public-field
supersession with logs/refusal evidence kept; dated adapter observations;
completed-judgment writer-failure behavior; launch binding without shell
profile edits; and typed intervention classification with negative retro
and spend fixtures. The whole transferred continuation obligation remains
above. No old pre-build status authorizes rebuilding or weakening it.
