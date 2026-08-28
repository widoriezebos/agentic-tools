# operator-surface v3 — the round-2 resolutions (codex as co-designer, adjudicated by the coordinator)

Drafted by the critic-turned-co-designer against its own fifteen round-2 findings, under Rulings A-D; adjudicated ACCEPTED IN FULL by the coordinator (notable: the draft DELETES four requirements — post-push receipts, observe-only for already-authorized invariants, the one-week retirement condition, and the impossible bootstrap gate — and it caught prior ruling D121, which v2's Ring 3 would have violated). These sections are the authoritative design text for their findings; the main design document binds them.

The v3 fold text below closes all fifteen findings with the smallest mechanisms I can make honest. Evidence level: checked by reading the design, critiques, paper, current implementation, and prior human rulings; no executable suite ran and no repository files changed.

## OSD-R2-01

Delete the impossible requirement that S0a through S4 use acceptance machinery introduced by S4. The bootstrap and ordinary eras are separate typed epochs.

`BootstrapAuthorization` contains:

- `schema=operator-bootstrap/v1`
- `authorizationId=operator-surface-s0a-s4`
- `authoritySource=HUMAN_CHAT_RULING_D`
- `baseCommit=70b09ae0b7455f2566f4428ff39e991e6f91fca5`
- `authorizedScope=S0a_THROUGH_S4_FINAL`
- `sliceTableDigest=<v3-slice-table-digest>`
- `verificationOwner=COORDINATOR`
- `landingGate=EXISTING_LANDING_GATE`
- `scopeEnd=BOOTSTRAP_RETROSPECTIVE_PASS`

Its explicit assumptions are `CHAT_AUTHENTICATES_WIDO`, `BASE_GATE_AND_ORIGIN_NOT_HOSTILE`, `EACH_SLICE_IS_REVERSIBLE`, `NO_EXTERNAL_CONTRACT_CHANGE`, and `BASE_COMMIT_REMAINS_REACHABLE`.

This typed attribution is the sole pre-S4 exception to process-proven authority. It grants no reusable authority after S4.

Bootstrap landing outcomes are `BOOTSTRAP_LANDED`, `BOOTSTRAP_GATE_RED`, `BOOTSTRAP_BUDGET_STOP`, and `BOOTSTRAP_SCOPE_REFUSED`. No bootstrap landing claims an acceptance record or base-engine judgment that does not yet exist.

Remaining later-slice dependencies are removed:

- Hook freshness enters health in the same slice as the hook attempt/completion record.
- No scheduler calls full `up`; its restricted entry lands with its own implementation.
- BREACH-STOP activates only after goal/revision binding and its stop capability exist.
- Before the typed stop state exists, a four-hour stop uses the current explicit `parked` receipt and handoff record.
- A BREACH-STOP made before S4 is absorbing; human resume is unavailable until Ruling C enforcement lands.

Immediately after S4 and before S5, one battery judges the cumulative range `70b09ae..S4`. Its outcome is `BOOTSTRAP_RETRO_PASS`, `BOOTSTRAP_RETRO_FAIL`, or `BOOTSTRAP_RETRO_COULD_NOT_RUN`.

Only `BOOTSTRAP_RETRO_PASS` seals S4 as the first ordinary accepted basis. Either other outcome fences S5 and permits only an in-scope repair or recovery; wider repair requires another human authorization.

The retrospective result records fitness of the authorized bootstrap. It never rewrites the authority under which S0a through S4 landed.

## OSD-R2-02

All human-reserved acts use one `HumanAuthorityProof`. Delete per-verb typed actors, direct-parent exceptions, and coordinator announcements as authority evidence.

`up` records the exact human terminal root: controlling terminal identity, session leader birth identity, boot identity, and enrollment generation. Initial enrollment is part of Ruling D’s bootstrap.

For every later human-authority invocation, the engine walks from the verb’s real parent to that enrolled terminal. For each process it:

1. Reads platform-native birth identity, executable identity, argument vector, and parent.
2. Re-reads birth identity and parent to detect reuse or ancestry motion.
3. Applies all installed Claude, Codex, and Devin signatures.
4. Refuses immediately if any signature matches.
5. Continues through plain shells and wrappers until the exact enrolled terminal is reached.

The acceptance predicate is:

`terminalReached == true && agentNodes == 0 && uncertainNodes == 0`.

An announced coordinator, lease holder, intermediate shell, typed name, or agent-created pseudo-terminal cannot override it.

Typed outcomes are `HUMAN_AUTHORITY_PROVEN`, `AGENT_IN_AUTHORITY_CHAIN`, `TERMINAL_NOT_REACHED`, `ANCESTRY_UNREADABLE`, `ANCESTRY_CHANGED`, `ARGV_UNREADABLE`, `PROCESS_REUSED`, and `ANCESTRY_CYCLE`.

Unreadable identity, arguments, parentage, or terminal state fails closed. “Could not classify” never becomes “not an agent.”

The proof record contains `schema`, `checkedAt`, `invokerRef`, `terminalRef`, `terminalGeneration`, `signatureSetDigest`, `outcome`, and `nodes[]`. Each node contains birth identity, parent identity, executable/argument digest, nullable agent-runtime enum, and terminal-match result. Raw arguments are not retained.

Required cases are direct human shell success; multiple plain wrappers; direct agent invocation; `agent → shell → verb`; an announced agent ancestor; an agent-created terminal; unreadable arguments; parent motion; process reuse; and no path to the enrolled terminal.

No additional human ruling is needed. Ruling C fully chooses this boundary.

## OSD-R2-03

Two lawful shapes exist:

- Shape A: rebase before judgment and acceptance; any later origin movement requires re-judgment and reacceptance. Smallest and recommended.
- Shape B: reapply an accepted delta to each new base, but still re-judge and reaccept it. This preserves more automation while adding delta custody and another recovery path.

Adopt Shape A. Every landing still rebases; the rebase moves before acceptance.

`land` creates an `operationId`, prepares the required receipt, commits exactly the selected candidate bytes, fetches origin, and rebases that one candidate commit onto fetched base `B`.

The prepared state must satisfy `parent(C) == B`. Extra local commits, unresolved conflicts, residual bytes, or a mismatched parent return `REBASE_REFUSED`.

It then records `baseCommit`, `baseTree`, `candidateCommit`, `candidateTree`, and `deltaDigest=sha256(Diff(baseTree,candidateTree))`.

Only after this rebase does the engine built from `B` judge `C`. Green judgment produces `AWAITING_HUMAN_ACCEPTANCE`; it does not push.

`land accept --operation <id>` runs under Ruling C and creates an immutable acceptance record binding:

- Operation, branch, base commit/tree, candidate commit/tree, and delta digest.
- Goal identifier and revision.
- Judge source commit/tree, controller and binary digests.
- Rule-set, retained-case, toolchain, and evidence digests.
- `judgment=GREEN`, `decision=ACCEPT`, authority-proof digest, and acceptance time.

After acceptance, land performs no fetch and no rebase. It attempts one compare-and-swap origin update from `B` to `C`.

If origin moved, the result is `ORIGIN_MOVED_REACCEPT_REQUIRED`. Land fetches and rebases only as a new attempt, reruns the new base judge, and requests another human acceptance—even if the candidate tree is content-identical.

Acceptance lifecycle events are append-only: `STANDING`, `CONSUMED`, `SUPERSEDED`, or `REVOKED`. A record is never edited into another state.

An unknown push is reconciled by fetching the branch. `C` reachable means consumed; the branch still at `B` permits the same push retry; any other tip requires reacceptance.

Transport synchronization always uses origin’s accepted ref. Failure after origin succeeds is `TRANSPORT_PENDING`; retry never rebases or republishes candidate bytes.

Operation outcomes are `PREPARED`, `REBASE_REFUSED`, `JUDGMENT_RED`, `AWAITING_HUMAN_ACCEPTANCE`, `ACCEPTED`, `PUSH_UNKNOWN`, `ORIGIN_MOVED_REACCEPT_REQUIRED`, `TRANSPORT_PENDING`, `LANDED`, and `ALREADY_LANDED`.

## OSD-R2-04

An ambient `bin/metasystem` is never an engine of record. Delete that fallback.

For candidate `C`, the judge basis is the exact accepted base commit `B` named by the landing operation.

A fresh checkout fetches `B`, `C`, and the protected acceptance/control records. It creates independent, no-hardlink checkouts detached at `B` and `C`, then verifies both commit and tree identifiers.

The controller, governed rules, build procedure, and retained cases all come from clean checkout `B`. The program under test comes from checkout `C`.

No candidate file enters the controller checkout. Candidate tests may add evidence but never replace the retained base cases.

The base engine is built from the detached `B` source. The build refuses dirty input and embeds the full source commit; it may not stamp Git HEAD while compiling another worktree’s bytes.

`EngineProvenance` contains:

- `baseCommit`, `baseTree`, and engine projection digest.
- Controller, rule-set, and retained-case digests.
- Go version, Go executable digest, environment digest, platform, architecture, and CGO state.
- Build-command digest, build stamp, binary digest, and build time.
- Candidate commit/tree, judgment evidence digest, and typed outcome.

Outcomes are `ENGINE_READY`, `BASE_OBJECT_UNAVAILABLE`, `BASE_UNACCEPTED`, `BASE_CHECKOUT_DIRTY`, `BASE_BUILD_FAILED`, `BUILD_STAMP_MISMATCH`, `PROVENANCE_MISMATCH`, and `CANDIDATE_JUDGMENT_COMPLETE`.

Acceptance binds the complete provenance digest. A different toolchain or binary requires a new judgment; it cannot silently reuse evidence.

Two supply shapes are honest:

- Build from Git objects on demand. Portable and smallest; recommended.
- Retain a platform-specific accepted binary bundle. Faster, but it adds platform and retention state; it may be a verified cache, never the only provenance source.

The S4 retrospective seal makes the S4 tip the first accepted base. Later fresh checkouts derive their judge from the accepted chain, never from local binary state.

## OSD-R2-05

BREACH-STOP becomes enforceable only after every new delegate record carries `goalId`, `goalRevision`, `machineId`, `claimEpoch`, and `capMin`.

A live legacy record missing any binding blocks cutover as `LEGACY_JOB_UNBOUND`. Migration drains it or explicitly maps it; it never guesses.

The recurring tick receives no general lease, goal-write, or cancellation authority.

At claim time, the authenticated lease holder mints one stop capability bound to the exact `(goalId, goalRevision, machineId, claimEpoch)`.

That capability permits only:

- Closing that goal revision’s launch fence.
- Moving that exact claim to its typed stopped state.
- Cancelling matching local jobs.
- Completing the named stop batch.

It cannot edit intent, cancel foreign work, unpark, dispatch, accept, land, or act on another goal.

On BREACH-STOP, the tick consumes the capability to launch one ephemeral goal-stop custodian. The custodian is the lawful holder of this bounded authority; its authority ends when the batch becomes terminal.

Dispatch and stop share one lock keyed by `(goalId, goalRevision)`. Dispatch holds it from its final budget/stop check through reservation, spawn, and exact process-identity publication.

The stop custodian takes the same lock and persists the stop fence before scanning. A launch therefore either publishes before the fence and appears in the scan, or observes the fence and refuses.

The durable batch records `stopId`, bound tuple, capability generation, fence epoch, pass number, every observed job generation, every cancel outcome, remaining jobs, and `OPEN | COMPLETE | INDETERMINATE`.

The custodian scans to a fixed point. `already-terminal` and replay of an earlier cancel are successful idempotent outcomes.

Unreadable identity or unproven custody yields `INDETERMINATE`, keeps the fence closed, and alerts. Parking does not remove the batch from later ticks.

Foreign-machine jobs are recorded as `FOREIGN_REPORT_ONLY`; this machine never signals or mutates them.

`goal resume` is Ruling-C human-only. It requires a new goal revision and budget, verifies the stop batch terminal, and clears the stop fence in the same transaction.

## OSD-R2-06

Delete the claim that the WIP branch is recovery. It preserves source work only; it cannot restore rules, authority, or runtime data.

Before activation of an accepted safeguard candidate, the base controller creates a protected `RecoveryBundle`. Activation refuses unless the bundle is durable outside the candidate branch and worktree.

The bundle contains:

- Safe acceptance, commit, tree, engine, controller, rule-set, and retained-case identities.
- Durable authority-state and data-state digests.
- An explicit `stateEntries[]` manifest.
- Bundle digest, custody reference, authorized recovery actor, and `VERIFIED` status.

Each state entry contains `path`, `class`, expected kind, mode, content digest, archive member, and `restorePolicy=COPY | VERIFY_ONLY | INVALIDATE`.

Required classes are `RULES`, `GOALS`, `JOBS`, `MISSIONS`, `RUNS`, `SUPERVISION`, `STEWARD`, `ACCEPTANCE_STATE`, `DURABLE_AUTHORITY`, and `LOCAL_CONFIG`.

Secrets and local configuration are `VERIFY_ONLY`; their bytes never enter shared Git custody. Live leases, terminal births, and one-shot capabilities are `INVALIDATE`, not restored from stale process identity.

Safeguard activation first quiesces launches and requires no live mission, run, or delegate mutation. This is the smallest state boundary at which exact automatic recovery remains honest.

A candidate failure before the quiescence fence is released may recover automatically. Once candidate-owned authority or data may have changed, recovery requires Wido’s Ruling-C invocation.

Recovery is an idempotent journal with phases `PREPARED`, `FENCED`, `QUIESCED`, `FAILED_STATE_PRESERVED`, `RESTORING`, `RESTORED`, `VALIDATING`, and `RECOVERED`.

The base launcher verifies the bundle, fences new launches, stops only candidate-owned exact identities, preserves the failed state separately, restores each `COPY` entry, invalidates ephemeral authority, and verifies every digest.

If the bad candidate reached the shared branch, recovery creates a forward recovery commit; it never rewinds or force-pushes shared history. Incident and acceptance evidence remain intact.

Terminal outcomes are `RECOVERED`, `ALREADY_RECOVERED`, `RECOVERY_REFUSED`, `RECOVERY_PARTIAL`, and `RECOVERY_VALIDATION_FAILED`. Partial or failed validation retains the fence.

The rehearsal starts from a fresh checkout, removes the active binary, injects a false authority grant, alters one data record, and proves restoration of code, rules, durable authority, and data.

Two state shapes are honest:

- Explicit manifests over current state owners. Smallest migration; recommended.
- One versioned operational-state directory with an atomic active pointer. Cleaner eventually, but it adds a state-layout migration with no current consumer.

## OSD-R2-07

Delete the absolute claim “three rings, no common silent failure.”

An earlier binding Wido ruling, D121’s third addendum, forbids metasystem-installed launchd entries, crontab lines, or bytes outside the repository. V3 retains that ruling.

Rings 1 and 2 are repository-owned. They share the repository and accepted engine and therefore cannot report destruction of that entire domain.

Ring 3 is optional and operator-owned. The metasystem neither installs nor rewrites it.

When configured, the scheduler may invoke only:

`metasystem up --recover-only --if-down`

Recovery-only mode never runs ordinary `up`. It does not announce a human terminal, acquire or renew a write lease, open or mutate goals, dispatch or cancel jobs, accept candidates, or land.

Its sole effect is to inspect the enrolled repository and re-arm a missing supervision owner and steward runner from the enrolled accepted-engine generation.

The operator-owned enrollment record contains `platform`, `unitId`, canonical repository path, launcher path and digest, accepted-engine path and digest, expected cadence, enable state, and enrollment generation.

Invocation outcomes are `RECOVERY_NOT_NEEDED`, `RECOVERY_STARTED`, `RECOVERY_PARTIAL`, `ENROLLMENT_DRIFT`, `ENTRY_DISABLED`, `ENTRY_MISSING`, and `ENTRY_UNREADABLE`.

Each attempt records its generation, start/completion times, observed component generations, and restart results. Starting a process is not success; the new component must publish its first successful pass.

`NOT_CONFIGURED` is recorded as `ABSENT_BY_POLICY`, reflecting the accepted no-host-dependency limit. Configured-but-missing is unhealthy; unreadable is unknown.

Ring 1 or Ring 2 reports enrollment drift while either survives. When both repository rings are gone, only the operator’s scheduler service can report its own result.

Alternative: a mandatory metasystem-installed launcher provides stronger reboot recovery, but it directly reverses D121 and requires an explicit human ruling before design work begins.

## OSD-R2-08

A live process or recently written heartbeat proves liveness only. Health requires generation-bound evidence that the component’s required work completed.

Every runner, watcher, and hook record carries `component`, `generation`, exact process identity where applicable, `attemptSeq`, `lastAttempt`, `lastCompletion`, `lastSuccess`, `result`, and evidence digest.

An attempt is persisted before work. Completion is persisted only after every mandatory action has a typed result. Only `result=OK` advances `lastSuccess`.

A steward tick succeeds only after health computation, narration decision, alert-episode update, and any required stop routing are durable.

The watcher keeps process heartbeat separate from pass success. It records the attempt, performs census and required repair work, then records completion. Its current pre-work heartbeat cannot satisfy pass freshness.

The hook writes an attempt before resolving the engine and a completion only after it emits the health line. The outcome is `EMITTED`, not `DISPLAYED`; the hook cannot prove that a chat client rendered stdout.

Health rejects generation mismatch, an incomplete attempt past its deadline, stale success, repeated failed completions, and unreadable evidence even while the process remains alive.

Mutual repair uses two narrow capabilities:

- The supervision owner may restart only the enrolled steward generation.
- The steward may request restart only of the enrolled watcher generation.

Neither component may write the other’s success evidence.

Alert episodes are durable records containing `episodeId`, finding digest, opened time, attempt list, transport result, acknowledgment state, resolved time, and cleared time. Retry never deletes an unacknowledged episode.

A notifier’s zero exit means `TRANSPORT_SUBMITTED`; it does not mean phone delivery or human acknowledgment. Pending state is retained until a typed later event changes it.

Two honest phone shapes remain:

- A session adapter returns a phone-push message identifier, and the next authenticated human chat event acknowledges the episode. This preserves the stated interaction and is recommended if the runtime exposes authenticated user events.
- A notifier supplies a delivery receipt, followed by `health acknowledge-alert <episode>` from Wido’s agent-free terminal. This is smaller locally but changes the “next chat message” behavior.

Neither acknowledgment grants authority, resumes a goal, or clears a refusal.

## OSD-R2-09

Delete the requirement that a tracked receipt is created after push. A tracked file cannot be both post-push and part of the commit it describes.

`land` creates exactly one receipt before candidate-tree calculation, keyed by the landing operation ID.

The receipt bytes are part of the judged candidate and, for safeguard slices, part of its exact human acceptance.

A retry reuses the receipt bytes and digest recorded in the operation journal. It never appends a second receipt for the same operation.

`outcome` describes the completed task, not remote-ref state. A `shipped` receipt reaches shared history only if the commit containing it reaches origin.

All learning-significant fields are mandatory:

- Operation ID, task type, task outcome, and goal.
- Skills list and builder class.
- Verification outcome and evidence references.
- Correction count and stop-loss outcome.
- Delegate list and critique status.
- Derived waiver provenance.
- Explicit note presence; free-text notes remain non-authoritative.

There are no semantic defaults. Empty lists and `NOT_APPLICABLE` are explicit values. Missing, unknown, or contradictory important fields refuse landing.

Critique and waiver facts are derived from their records; the caller cannot assert “none.”

The journal phases are `PREPARED`, `COMMITTED`, `ORIGIN_PUSHED`, `TRANSPORT_PUSHED`, and `COMPLETE`.

The two-remote operation is explicitly idempotent, not globally atomic. Origin may succeed while transport is pending; retry completes only the missing phase.

Journal recovery finds the unique operation ID in commit metadata and the receipt, then compares both remote refs. Journal loss does not repeat the receipt, commit, or successful push.

The receipt family retains one owner. `correct`, `stats`, `check`, and `retro` remain append-only learning operations; `land` absorbs only creation of its own receipt.

## OSD-R2-10

A slice is one landing with one runnable behavior, focused fixtures, and a typed stop outcome. A phase label may contain several slices; the four-hour ceiling applies to each landing.

S0a is an isolation transaction, not “clean the tree.”

It freezes repository writers, takes the existing repository-operation lock, and records exact active-writer identities.

Its record contains operation ID, current main commit, WIP ref commit, dedicated WIP worktree path, initial and final path/mode/blob manifests, cleanup authorization, and outcome.

The dedicated WIP worktree is created and verified before any primary-tree cleanup. A moving manifest, unreadable path, unlisted writer, or missing cleanup authorization refuses without deleting anything.

A second identical manifest authorizes cleanup; a third comparison proves every preserved blob remains reachable. Rerun returns `ALREADY_ISOLATED`.

The WIP branch is never merged wholesale. S0b records `KEEP | ADAPT | DELETE` per path and behavior; later work reapplies only selected deltas to current main and surfaces conflicts.

The bootstrap phase is partitioned as:

- S1a: health evaluator over facts that exist at landing time.
- S1b: hook attempt/completion and chat-line emission.
- S1c: runner/watcher success evidence and narrow mutual repair.
- S1d: desktop alert episodes.
- S2a: structured budget schema and breach projection.
- S2b: mandatory goal/revision binding and legacy drain.
- S2c: stop capability, shared launch fence, and fixed-point cancellation.
- S2d: stopped state; resume remains unavailable.
- S3a: ordinary idempotent `up`, advisor outcome, and terminal enrollment.
- S3b: restricted recovery-only `up` seam; no host installation.
- S4a: full Ruling-C ancestry enforcement and migration of human acts.
- S4b: fresh-checkout engine-of-record and protected acceptance record.
- S4c: recovery bundle and rehearsal.
- S4d: landing acceptance enforcement.

The first retrospective battery runs after S4d and before S5.

Post-bootstrap work is fixed now:

- S5a–S5g: custody identity, claim, occupancy, process-group custody, progress signals, call-site migration, then public delegate/follow-up/cancel.
- S6: bounded watch and zombie outcomes.
- S7a: candidate selection and receipt.
- S7b: pre-acceptance rebase, base judgment, and human acceptance.
- S7c: origin/transport journal and reconstruction.
- S8a: empty governed-rule store and observe evidence.
- S8b: promotion, appeal, review, and withdrawal lifecycle.

Each custody slice inherits the corresponding failing fixtures from the six-unit custody brief. No slice imports a record, authority, check, or remedy first created later.

## OSD-R2-11

Replace the boolean “trial mode” with one typed lifecycle:

`DRAFT → OBSERVE → LIMITED → ENFORCED → WITHDRAWN`.

Each rule record contains `id`, `revision`, `stage`, `judgmentScope`, `activationScope`, `refusalEvidenceSchema`, `maintenanceOwner`, `reviewAt`, `appealRoute`, `evidenceGate`, `stageDecision`, and `withdrawalDecision`.

`evidenceGate` is fixed before observation and contains:

- Must-stop cases.
- Must-permit cases.
- Interaction cases against every active overlapping rule.
- Side-effect metrics with permitted bounds.
- Separate eligibility predicates for `LIMITED` and `ENFORCED`.

An empty interaction set is valid only when paired with the active-rule-set digest proving that no overlapping rule existed.

`OBSERVE` may emit only `WOULD_REFUSE`. `LIMITED` may refuse only inside its recorded activation scope. `ENFORCED` may refuse throughout judgment scope. `WITHDRAWN` preserves history and never refuses.

Every transition that grants, widens, narrows, or removes refusal authority is a Ruling-C human legislative act. A maintenance owner may revise implementation and propose evidence; it cannot promote itself.

The decision record contains operation ID, from/to stage, `PROMOTE | HOLD | REVISE | WITHDRAW`, authority-proof reference, rule revision, evidence digest, and risk-assessment reference.

Promotion refuses unless the engine recomputes the applicable evidence gate as satisfied. Changing the threshold creates a new revision; an unexplained override never becomes adoption.

A refusal names rule identifier/revision, candidate, scope, evidence, and typed appeal identifier. Appeals route through `goal rule appeal`.

At `reviewAt`, the rule never auto-promotes or silently disappears. Health reports `RULE_REVIEW_DUE`, and scope changes refuse until a human records `HOLD`, `REVISE`, or `WITHDRAW`.

Delete the requirement that the four-hour ceiling, claim-requires-appetite, and mandatory fields enter this store in observe-only mode.

Those are already-authorized schema and budget invariants, not newly learned refusal rules. S8 therefore deploys the governance mechanism empty, removing S2’s contradiction.

## OSD-R2-12

A claimed goal carries one typed budget bound to its revision:

- `elapsedLimit`: positive duration.
- `attemptLimit`: positive integer.
- `reservedJobMinutesLimit`: positive integer.
- `activeJobLimit`: positive integer.

All four fields are mandatory for claimed agent work. `goal open --claim` and `goal set-budget` accept the complete tuple; there are no implicit numeric defaults.

An attempt is consumed when a new external-execution reservation wins, including follow-ups. Replay of the same operation ID consumes nothing again.

The universal pre-launch spend signal is `capMin`, already stored in modern job records and enforced as their absolute runtime cap.

`reservedJobMinutes` is the sum of `capMin` for all reservations bound to the goal revision. This captures parallel work: two sixty-minute reservations consume 120 agent-minutes even when concurrent.

Reservations are conservative authorization, not an estimate of actual billing. They are not refunded because a job happened to finish early.

The budget journal is the sole counter owner, keyed by reservation operation ID. Before launch it atomically checks attempts, reserved job-minutes, active jobs, and elapsed claim time.

A proposal may bring attempts or reserved minutes exactly to its limit; that reservation is lawful and closes further admission. A later attempt triggers the typed stop path.

`activeJobLimit` is a concurrency bound: equality is lawful; an additional job returns `CAPACITY_REFUSED` without permanently consuming budget.

Elapsed time is lawful only while `elapsed < elapsedLimit`; equality is BREACH-STOP.

Attempt or reserved-spend exhaustion closes admission and lets already-authorized jobs reach their caps. Elapsed breach or corrupt over-limit state also initiates live-job wind-down.

Every job must carry `goalId`, `goalRevision`, and `capMin`. `goal none-explicit` cannot consume or advance a claimed goal.

Accounting is cross-checked against job records on every read. Missing or contradictory state produces `BUDGET_UNKNOWN`, refuses new reservations, and makes health unknown-present; it never assumes zero.

Job `usage` and root `chainUsage` remain actual-spend evidence: tokens, cost when available, and provider-native units. These signals exist today but are heterogeneous, sometimes absent, and post-run.

Therefore measured token or currency ceilings are optional only where every allowed runtime supplies enforceable `nativeBudget`. They never replace the universal reserved-job-minute fence.

## OSD-R2-13

The complete human-facing top-level set is exactly:

`up`, `health`, `goal`, `delegate`, `watch`, `land`, `mission`, `run`.

There is no ninth operator namespace. Top-level help prints exactly these eight names.

Required human acts route as follows:

| Human act | Surfaced route |
| --- | --- |
| Enroll the interactive terminal or optional recovery entry | `metasystem up …` |
| Inspect health or acknowledge an exact process/alert finding | `metasystem health …` |
| Resume a stopped goal or govern a rule | `metasystem goal …` |
| Launch, follow up, or cancel delegated work | `metasystem delegate …` |
| Follow one job to a typed terminal outcome | `metasystem watch …` |
| Prepare, accept, recover, or inspect a landing | `metasystem land …` |
| Start, inspect, answer, resume, or resolve a mission | `metasystem mission …` |
| Start, inspect, acknowledge, or clean up an arbitrary run | `metasystem run …` |

Exact safeguard acceptance is `land accept --operation <id>`.

Budget-stopped work resumes through `goal resume <id>`. Rule promotion, withdrawal, and appeal live under `goal rule`.

`proc acknowledge` leaves the operator surface. The birth-identity judgment remains engine-owned behind `health acknowledge --finding <id>`, and health prints that complete remedy.

The receipt engine remains, but top-level `receipt` leaves operator help. Landing receipts and corrections are reachable through `land receipt`; the retro workflow may invoke the same internal owner.

Every state-changing human subcommand uses the Ruling-C ancestry proof. Read-only help and status do not.

Every remedy emitted by health, watch, goal, or land must name one of these surfaced routes. Emitting an internal command is a fixture failure.

Old spellings may remain as migration-only internal aliases for scripts. They are absent from operator documentation and have a named retirement slice.

## OSD-R2-14

Delete “one week of live evidence” as a sufficient retirement condition. Seven days remains only a `notBefore` soak boundary; quiet time proves nothing.

Each overlapping protection receives its own transition record. There is no system-wide retirement switch.

The record contains `protectionId`, old and new owners, authority during dual-run, start time, `notBefore`, declared risk classes, comparison events, rollback evidence, computed eligibility, and retirement decision.

Each risk class names mandatory:

- Must-stop cases.
- Must-permit cases.
- Failure/recovery cases.
- Required evidence source: `FIXTURE` or `LIVE`.

Rare or dangerous failures may be fixture-covered. Ordinary operating behavior must include live evidence.

The old protection remains authoritative during coexistence. The new path records its decision independently before a join records agreement or disagreement.

Each comparison event contains candidate identity, risk class, old and new outcomes, both evidence references, both durations, and `OLD_CORRECT | NEW_CORRECT | BOTH_CORRECT | NEITHER_CORRECT | UNRESOLVED`.

Retirement eligibility becomes true only when every mandatory case passes, every risk class has its required evidence, no disagreement remains unresolved, the new path misses nothing the old path catches, new-only catches are adjudicated, rollback passes, and `notBefore` has elapsed.

Required operator-surface exercises include:

- Health: kill, stale-success, unreadable, restart, and recovery.
- Budget: within-budget permit, exact-boundary stop, launch/stop race, and foreign-job isolation.
- Authority: direct-human permit, agent-in-chain refusal, and unreadable ancestry refusal.
- Landing: moving-origin reacceptance, partial transport recovery, fresh-checkout judgment, and recovery rehearsal.

Agreement count alone is never evidence of eligibility.

Wido then records `RETAIN_OLD`, `EXTEND_DUAL_RUN`, or `RETIRE_OLD` under Ruling C.

`RETIRE_OLD` authorizes one cleanup landing that transfers authority and deletes the old execution path and switch together. If deletion does not land, authority does not transfer.

Where archaeology found no surviving protected condition, the record uses `DELETE_WITHOUT_REPLACEMENT`; replacement comparison is deliberately omitted, but human acceptance and retained deletion evidence remain mandatory.

## OSD-R2-15

Delete the separate vague “repeated within a window” rule. The existing five-observation supervision breaker is the sole failure-escalation owner; health projects it instead of adding another clock.

| Boundary | Default | Exact rule, including equality | Reset and restart |
| --- | --- | --- | --- |
| Health observation cadence | `watch.interval-sec=60` | One durable sequence advances per completed health pass; missed wall time creates no synthetic passes | Sequence and counters survive a valid restart |
| Periodic-role freshness | Two recorded producer intervals | Fresh iff UTC age `< 2 × interval`; equality is stale/service-dead | Only successful completion refreshes it |
| Hook freshness | Current turn generation | Healthy only when the current turn has a successful hook completion; attempt alone is service-dead | Next successful turn resets |
| Unknown grace | One completed observation | First consecutive unknown gives exit 2; second consecutive unknown alerts; equality is observation two | Alive or dead resets the unknown count |
| Failure escalation | Five consecutive failing observations | Dead or stale increments; unknown neither increments nor resets; failure five ends auto-healing and alerts | One full alive observation resets to zero |
| Aggregate health | No override | Any dead/service-dead gives exit 1; otherwise any unknown gives exit 2; otherwise exit 0 | Recomputed on every read |
| Watch polling | Two seconds | Inspect immediately, then every two monotonic seconds | A new invocation starts a new polling loop |
| Default watch deadline | `capDeadline`, else `startedAt + capMin` | Nonterminal at `now >= deadline` is timeout | Recomputed to the same absolute deadline after restart |
| Explicit watch timeout | Positive duration | Timeout when monotonic elapsed `>=` requested duration | Intentionally invocation-scoped |
| Pending launch | Recorded handshake deadline | Missing primary identity may remain pending only while `now < handshakeDeadline`; equality is zombie-suspected | Absolute deadline survives restart |
| Watch precedence | Fixed | Terminal record first; pending-within-handshake second; dead/indeterminate process third; deadline fourth; otherwise continue | Recomputed every poll |
| Clock regression | Zero guessed tolerance | Persisted time later than current UTC produces unknown `CLOCK_REGRESSED`, never alive | Clears only after a coherent later observation |
| Clock advance | No suppression | A forward jump may make persisted freshness or deadlines expire on the next observation | Result is explicit and self-verifying |

At an exact watch deadline, a terminal record wins. Otherwise a proven dead or indeterminate process wins over timeout and returns zombie-suspected.

A nonterminal legacy job without `capDeadline`, `startedAt + capMin`, or handshake deadline returns `ZOMBIE_SUSPECTED: DEADLINE_UNAVAILABLE`; it never waits indefinitely.

Watch exit codes are fixed: `0=terminal-ok`, `1=terminal-failed-or-cancelled`, `2=timeout`, `3=zombie-suspected`, and `4=missing-or-malformed-record`.

Fixtures cover every equality boundary, alternating unknown/alive observations, persistent unknown, healthy reset, process restart, hook attempt without completion, terminal-at-deadline precedence, backward clock movement, and forward clock movement.

Evidence note: `goal next` was attempted and hit the repository’s known macOS `confstr()` warning/object-name defect. Proposed receipt, not written:

`scripts/receipt.sh add --type design --outcome shipped --skills none --verify skipped --corrections 15 --stop-loss no --built-by mixed --note "operator-surface v3: concrete resolutions for all fifteen round-two findings; read-only"`

Deliberately unresolved for the human:

- Choose and provision protected custody: recommended shape is protected Git refs for acceptance/provenance plus the configured durable evidence store for recovery bundles. The current `evidence.root` remains a placeholder.
- Choose the concrete iPhone transport and acknowledgment shape. Recommendation: authenticated session push/next-user-message if the runtime exposes those events; otherwise use delivery receipt plus terminal acknowledgment.
- Decide whether to supersede D121 and permit a metasystem-owned Ring 3. This draft recommends retaining D121 and using only an optional operator-owned recovery entry.
# PART TWO — the round-3 resolutions (v4; codex as co-designer, adjudicated accepted in full)

Resolutions for R3-04..11 under Rulings A-K. Notable: the budget journal is DELETED (job reservations are the sole spending facts, projected under the goal-revision lock); the lock order becomes one ranked chain; the slice manifest becomes calendar-honest at 40 landings / ~158 clean hours / ≥14 soak days.

## R3-04

R3-04, the launch/stop lock-order defect, is accepted.

The global order is `chain → goal-revision admission/stop → cap authority → job lifecycle → session occupancy → job record`.

The existing `chain → cap → occupancy → record` relative order is unchanged; absent lock classes are skipped, never reordered.

Dispatch takes the goal-revision lock after any chain lock and holds it from the final fence and budget decision through reservation, spawn, and exact process-identity publication.

The cap lock still covers only cap adjudication and reservation publication; it is released before spawn or adapter work.

The stop custodian takes the goal-revision lock, atomically closes the launch fence, records its fence epoch, and releases the lock before scanning or cancelling anything.

Cancellation starts at the lifecycle rank and may then take occupancy and record locks. It never holds or reacquires the goal-revision lock.

`goal resume` takes the goal-revision lock alone to verify the terminal stop batch, create the new revision, and reopen admission.

Every acquisition is bounded. Failure returns `LOCK_BUSY` with the requested rank, key, holder evidence when readable, and a retry instruction.

A deterministic lock-order fixture pauses dispatch before reservation, after reservation, after spawn, and before identity publication while stop begins concurrently.

Every fixture outcome must be either “fence first, launch refused” or “launch first, published job included in the stop scan”; timeout or inversion fails the fixture.

A second fixture holds each lower-ranked lock and proves that an attempted higher-ranked acquisition refuses locally instead of waiting.

## R3-05

R3-05, the split ownership of budget and reservation state, is accepted.

Delete the budget journal. It is a second store for facts already durably represented by delegate job reservations.

The immutable reservation fields in each delegate job record are the sole spending facts: operation identifier, goal identifier, goal revision, and reserved runtime cap.

Under the goal-revision admission lock, the budget owner projects:

- attempts as the count of distinct reservation operation identifiers;
- reserved job-minutes as the sum of their runtime caps;
- active jobs as the count of non-terminal reservations;
- elapsed time from the goal revision’s claim record.

The goal record owns limits and the stop fence; it does not duplicate counters.

The session occupancy index remains only a rebuildable performance projection. It has no budget authority and cannot make a reservation exist.

A crash before job-record publication consumes nothing. A crash after publication consumes the attempt and reserved minutes even if spawn never occurs.

An abandoned setup is terminalized by custody reconciliation, releasing only its active-job slot; attempts and reserved minutes are never refunded.

A missing occupancy entry is rebuilt from job records. An orphaned occupancy entry is deleted. Neither condition produces permanent `BUDGET_UNKNOWN`.

An unreadable, duplicate, revisionless, or contradictory authoritative job record does produce `BUDGET_UNKNOWN`, closes admission, and names the exact record requiring repair.

The other honest shape is a prepare/commit budget journal with total crash reconciliation. It avoids projection scans but preserves two stores and several repair states.

Recommendation: delete the journal; job records are already the durable bounded reservation ledger.

The crash fixture interrupts before record publication, between record and occupancy publication, after publication before spawn, and during abandoned-setup reconciliation, then proves the projections above.

## R3-06

R3-06, the conflict between BREACH-STOP and dual-run authority, is accepted.

During coexistence, the old protection is authoritative exactly as the transition contract says. The new four-field budget path is read-only and emits `WOULD_BREACH_STOP`.

| Old decision | New decision | Authoritative behavior |
| --- | --- | --- |
| permit | permit | permit |
| stop | stop | old path stops; new path records agreement |
| stop | permit | old path stops; disagreement is retained |
| permit | stop | permit; new path must not close a fence or cancel work |

S2c therefore reaches `DUAL_RUN_READY`, not `ENFORCED`.

Every paired decision is joined by candidate, goal revision, input-state digest, and observation generation. Unjoinable decisions are `TRANSITION_UNKNOWN`, never agreement.

A new-only stop records the exact budget field, boundary, and projected stop batch without minting or consuming a stop capability.

T2, the budget authority-transfer slice, may begin only after every mandatory fixture and live case passes, every disagreement is adjudicated, `notBefore` has elapsed, and all current revisions are lawful under the new projection.

T2 closes launch admission, recomputes both decisions from the same quiescent state, consumes Wido’s recorded `RETIRE_OLD`, and lands one cleanup that deletes the old path and switch.

Authority transfers only when that cleanup is accepted and activated. Before then the old path remains authoritative; afterward no dual path remains.

A direct clean cutover is also honest and shortens the period in which new-only breaches are permitted, but it discards the ruled evidence-based transition.

Recommendation: retain old authority during dual-run and make T2 the single transfer point.

Fixtures exercise all four decision pairs, prove that only the old path mutates during coexistence, and prove that an incomplete cleanup cannot transfer authority.

## R3-07

R3-07, the incomplete quiescence lifecycle, is accepted.

Quiescence is part of the existing activation/recovery journal; delete any separate quiescence store.

Its ordered phases are `PREPARED → ADMISSION_CLOSED → DRAINING → QUIESCED → MUTATION_STARTED → VALIDATING → RESUMING → COMPLETE`.

`DEFERRED` is terminal before candidate mutation. `RECOVERY_REQUIRED` is a held state after mutation may have occurred.

The base controller closes one repository launch-admission epoch under a bounded admission lock before taking the drain census.

The gate is consulted only by process-creating entrances: mission turn launch or resume, run start, delegate, and follow-up.

Health, watch, status, design, critique, adjudication, and recovery work do not consult this gate. Quiescence pauses launches, never design or adjudication work.

Once admission is closed, a mission completes its current turn and adjudication but cannot launch another turn. It records `QUIESCENCE_PAUSED`.

Existing runs and delegates finish under their existing caps and lifecycle owners. Quiescence receives no authority to cancel base-owned work.

The journal records the exact drain set and a mandatory absolute drain deadline. Continuously renewed work cannot enter because every renewal is a new launch.

If the drain set becomes terminal, two identical censuses establish `QUIESCED`.

If the deadline arrives before candidate mutation, admission is reopened atomically and the operation returns `QUIESCENCE_DEFERRED`, naming every blocker.

A human may cancel named base work through its existing lifecycle verb and retry; quiescence itself never invents cancellation authority.

After successful activation or verified recovery, `RESUMING` reopens admission and wakes missions paused by that operation.

A crash before `MUTATION_STARTED` is automatically reconciled by reopening or continuing the drain. A crash afterward retains the fence and alerts.

From `RECOVERY_REQUIRED`, Wido’s enrolled-terminal invocation may idempotently continue recovery; admission opens only after exact validation passes.

Fixtures cover a launch racing fence closure, a mission cycle boundary, a long run reaching the drain deadline, pre-mutation crash reopening, post-mutation crash holding, and successful human recovery resumption.

## R3-08

R3-08, the false S0a bootstrap prerequisites, is accepted.

Delete both the literal slice-digest placeholder and the claim that an existing repository-operation lock protects S0a.

Before S0a, the coordinator records one concrete `BootstrapAuthorization` under Ruling D.

It binds:

- `genesisCommit=cbb1adb07712910df270c9c1335760054cd53160`;
- the exact landed commit containing the final v4 slice manifest;
- the manifest’s computed digest;
- the S0a-through-S4 scope and first-pass execution ceiling;
- the declared human maintenance window and starting writer-census digest.

The canonical digest input is the versioned slice manifest, sorted by slice identifier, with each row’s behavior owner, prerequisites, duration ceiling, fixtures, transition dependency, and human gate.

A template, missing value, mismatched design commit, or recomputation mismatch is not authorization.

No machine-enforced repository-wide writer exclusion is claimed during S0a. Ruling D permits one explicit human maintenance window because the ordinary admission gate does not yet exist.

S0a records visible writer identities and stable path/mode/blob manifests before preserving anything and immediately before cleanup.

An unexpected writer, a moving manifest, or an ended maintenance window refuses before deletion. The record truthfully names `HUMAN_MAINTENANCE_WINDOW` as the exclusion basis.

Ruling J’s genesis acceptance is not an S0a prerequisite: its enrolled-terminal recorder lands during S4.

It is instead mandatory before the bootstrap retrospective and S5. Its subject must be the exact genesis commit above, and every later acceptance must name its protected ref.

Building a repository-wide lock before S0a is the other honest shape, but it creates a new pre-bootstrap mechanism that itself needs authorization and proof.

Recommendation: delete that requirement and use the bounded Ruling-D maintenance window.

Fixtures refuse missing authorization, digest drift, base mismatch, an unexpected writer, and a moving manifest, proving no cleanup occurred in every refusal.

## R3-09

R3-09, the missing goal revision in retry identity, is accepted.

Goal revision joins both default operation identity and the custody fingerprint.

The default operation identity is derived from goal identifier, goal revision, dispatch mode, role, and brief digest.

Fingerprint version 2 adds goal identifier and revision to the complete process-creating request encoding.

An explicit operation identifier may replay only when its stored version-2 fingerprint matches, including the exact goal revision.

Reusing an operation from another revision returns `REFUSED_OPID_MISMATCH` before busy-gate or budget admission.

A legacy fingerprint without revision cannot replay claimed work and returns `REFUSED_UNPROVABLE_LEGACY`.

Implicitly repeating the same role and brief after a revision change mints a revision-distinct operation and reservation.

Named fixture `goal-revision-is-retry-identity` performs:

1. Open goal G at revision 7 with a two-attempt, thirty-minute reservation budget.
2. Dispatch a fifteen-minute job and record operation O7, one attempt, and fifteen reserved minutes.
3. Replay O7 at revision 7 and prove no second reservation or charge appears.
4. Create revision 8 with a fresh budget.
5. Retry with explicit O7 and require `REFUSED_OPID_MISMATCH`, with both revisions unchanged.
6. Repeat implicitly and require new operation O8, distinct from O7.
7. Prove O8’s job binds revision 8 and only revision 8 gains one attempt and fifteen reserved minutes.
8. Replay O8 after an interrupted launch and prove its reservation is not charged twice.

The same fixture leg runs for fresh dispatch and follow-up.

## R3-10

R3-10, the incomplete and calendar-false slice table, is accepted.

Delete every “25-slice” claim. The complete v4 manifest contains 40 landings:

- S0a isolation; S0b triage; T0 transition records and comparison join.
- S1a health evaluation; S1b hook evidence; S1c runner/watcher evidence; S1d alert episodes and desktop delivery; S1e phone delivery receipts.
- T1 health authority transfer and old-path deletion.
- S2a job-derived budget projection; S2b revision identity and legacy drain; S2c shadow stop capability/fence/cancellation; S2d stopped and resume states.
- T2 budget authority transfer and old-path deletion.
- S3a ambient `up` and advisor; S3b restricted recovery-only `up`.
- S4a terminal enrollment and `HumanAuthorityProof`; S4b migration of human acts, including alert acknowledgment.
- T3 human-authority transfer and old-path deletion; T3 must precede genesis acceptance.
- S4c engine-of-record, immutable acceptance refs, and genesis acceptance.
- S4d recovery-bundle construction and verification; S4e quiescence admission, drain, and resumption.
- S4f restore journal and exact restoration; S4g validation and forward recovery.
- S4h destructive rehearsal; S4i landing acceptance enforcement.
- S5a–S5g the seven custody landings; S6 bounded watch.
- S7a candidate and receipt; S7b rebase, judgment, and acceptance; S7c remote journal and reconstruction.
- T4 landing authority transfer and old-path deletion.
- S8a empty governed-rule store; S8b promotion, appeal, review, and withdrawal.

S0a retains its two-hour ceiling; every other landing retains the four-hour ceiling.

The clean first-pass execution ceiling is therefore 158 hours: 2 hours plus 39 four-hour landings, or 19.75 eight-hour working days before gates and interruption.

That number excludes failed gates, recovery, reslicing, reacceptance, human availability, transport provisioning, and operational rehearsal.

T1 through T4 each have their own seven-day `notBefore`. T3 and T4 cannot overlap under the dependency graph, so the program contains at least fourteen elapsed soak days.

T1 and T2 may soak while later design, adjudication, and non-dependent work continue.

The human is buying 40 independently reversible, fixture-owned behavior boundaries and four evidenced authority transfers—not a nominal 25-task quick build.

The manifest and these dependencies are the canonical input to R3-08’s bootstrap digest.

## R3-11

R3-11, the hook success-state ambiguity, is accepted.

Separate generic completion result from component-specific outcome.

Every completion record carries `result=OK | ERROR | INDETERMINATE` and a mandatory component-owned `outcome`.

Only `result=OK` advances `lastSuccess`. Every completed attempt advances `lastCompletion`.

For the hook, successful stdout emission records `result=OK` and `outcome=EMITTED`.

The hook never records `DISPLAYED`; client rendering remains unknowable.

A write, engine-resolution, or emission failure records `result=ERROR` with a typed outcome and leaves `lastSuccess` unchanged.

An unreadable or causally uncertain completion records `result=INDETERMINATE`, also without advancing success.

Named fixture `hook-emission-advances-success` performs:

1. Persist attempt N for turn generation G and prove attempt alone is unhealthy.
2. Complete N as `OK/EMITTED` and prove `lastCompletion` and `lastSuccess` both advance.
3. Prove health accepts generation G.
4. Start generation G+1, complete it as `ERROR/EMIT_FAILED`, and prove only `lastCompletion` advances.
5. Prove health rejects G+1 despite the prior successful timestamp.
6. Retry G+1, complete as `OK/EMITTED`, and prove success and health recover.
7. Assert that no record or output claims `DISPLAYED`.

Evidence level: checked by reading the design, resolutions, failsafe verdict, custody lock contract, and cited implementation. The 40-slice/158-hour arithmetic was run; no fixture suite ran and no files changed. `goal next` was attempted and hit the recorded macOS `confstr()` object-name defect.

Proposed receipt: `type=design`, `outcome=shipped`, `skills=design-critique`, `verify=skipped`, `corrections=8`, `stop-loss=no`, `built-by=mixed`, note “operator-surface v4 resolutions for R3-04 through R3-11 under Rulings A–K; read-only.”

## Left for the human

NEW HUMAN RULING REQUIRED: select the concrete iPhone transport that supplies authenticated delivery receipts; this design does not choose a provider, dependency, credentials, or spending tier.

Operational acts already reserved to Wido remain: explicit implementation approval, protected-ref and durable-evidence provisioning, the S0a maintenance window, terminal enrollment, genesis acceptance, and T1–T4 retirement decisions.