# Design critique: seat-successor-continues-a-handoff-without-a-human, round 3

The review target is `metasystem/artifacts/reports/succ-design-r3.md`, checked against current `origin/main` at `7df4c50878dffe35f3f8e33826fb7909e5148efb`, the brief, the goal, the binding rulings, and the round-2 critique. No tests, fixture beds, or process-control commands were run.

## Round-2 material-finding closure

| Prior finding | Closed by this revision? | Assessment |
|---|---|---|
| SUCC-R2-01 — post-cap recovery contradicts the binding terminal boundary | **No.** | Sections 3.3i and 3.3k now specify the ruled slow-retry cadence, expiry, and injected clock, but the relaunch still calls the ordinary handoff decision without specifying how it passes that decision's active-intent and dry-revival-cap guards. See SUCC-R3-03. Section 3.12b also makes expiry-notice recovery non-replayable. See SUCC-R3-05. |
| SUCC-R2-02 — recovery claimant has no authorized announcer or crash-safe transfer | **Yes.** | Sections 3.7d–3.7f remove the synthetic recovery claimant, retain the dead real claim pair during recoverable custody, and key return/release to the chain. |
| SUCC-R2-03 — hook rewrites are not authorized | **Yes.** | Sections 3.4a–3.4b copy the canonical observe-only hook map byte-for-byte and use the existing adapter-supplied session id. |
| SUCC-R2-04 — later attempts have neither authorization nor distinct durable job identity | **No.** | Section 3.3b adds per-attempt job ids, but its terminal-record exception authorizes redispatch of the same terminal job id, which the dispatcher replays rather than launches. See SUCC-R3-04. Its relaunch admission is also still blocked. See SUCC-R3-03. |
| SUCC-R2-05 — continuation can end below the trigger without launching a successor | **Yes.** | Section 3.4c removes milestone allow paths, blocks while work remains, and terminates only after the bounded empty-backlog idle sequence required by the final seat ruling. The required notice still lacks its own mutation witness; see SUCC-R3-07. |
| SUCC-R2-06 — mutation fence is non-exhaustive and non-atomic | **No.** | Sections 3.8a–3.8d cover the common request constructor and every `Mutate` invocation, but the check still occurs before commit construction and the compare-and-swap publication. A successor can start in that interval. See SUCC-R3-02. |
| SUCC-R2-07 — registered-wait pin races target registration | **Yes.** | Section 3.6c makes `ExpectedTarget` part of the initial persisted observation and requires mismatch refusal. |
| SUCC-R2-08 — successor census weakens exact identity to seconds | **No.** | Section 3.2a carries microseconds, ticks, and boot id, but then puts mutually exclusive exact-identity representations into one `identity.Ref`; that ref is invalid on Linux. See SUCC-R3-06. |
| SUCC-R2-09 — nested arbitration-lock acquisition deadlocks tick recovery | **Yes.** | Section 3.3j gives tick and explicit recovery one lock owner and calls locked internals beneath it; its named witness covers competing callers. |
| SUCC-R2-10 — production timing is not fully injectable | **Yes.** | Section 3.3k threads `cfg.Now` through the changed decision, relaunch, and expiry paths and names a no-wall-clock mutation witness. |

## Current-code fact audit

- The design's source anchors have not drifted in the affected runtime paths: the range from its base `7298bbe91` to current `origin/main` changes spend work, not handoff, revival, dispatch, census, identity, goal transaction, or hook code.
- Handoff staging currently writes durable intent but does not launch: `metasystem/cmd/metasystem/context_verbs.go:139-208` and `metasystem/internal/steward/handoff_capture.go:926-995`. Revival is still a one-shot live-intent consume/launch/stamp path: `metasystem/internal/steward/revive.go:90-206`.
- The shared launch seam exists and preserves the inherited environment other than three internal keys: `metasystem/cmd/metasystem/steward_verbs.go:493-510` and `metasystem/cmd/metasystem/delegate.go:111-132`. The Claude adapter already supplies `sessionId`: `metasystem/internal/adapter/claude.go:268-299` and `metasystem/scripts/adapters/claude.sh:165-176`.
- Current handoff admission blocks when another active intent exists and returns notify at the dry-revival cap: `metasystem/internal/steward/handoff.go:116-125`. The current relaunch evidence counts active consumed intents: `metasystem/internal/steward/revive.go:269-300`.
- Dispatch replays a terminal record with the same job id instead of launching it again: `metasystem/internal/dispatch/claim.go:594-635`; terminal states are enumerated at `metasystem/internal/dispatch/record.go:41-58`.
- A goal transaction calls `Mutate`, then constructs and validates the commit, then performs publication compare-and-swap: `metasystem/internal/goal/txn.go:609-708`. There is no steward arbitration lock across that interval.
- `identity.Ref` treats microsecond time and ticks/boot id as mutually exclusive modes and returns invalid when both are present: `metasystem/internal/identity/identity.go:21-31` and `metasystem/internal/identity/identity.go:72-109`. Linux census production supplies both representations: `metasystem/internal/census/production.go:45-70`.
- Existing reaping deliberately queues the notice before terminally marking a record so a crash remains replayable: `metasystem/internal/steward/reap.go:31-80`. That is the inverse of design section 3.12b's cancellation order.
- Engine-created owner lineages use `main-<epoch>-<pid>-<suffix>`: `metasystem/internal/lease/verbs.go:181-205`. Goal mutation requests take `METASYSTEM_OWNER_LINEAGE` into the actor identity: `metasystem/cmd/metasystem/goalsync_mutations.go:470-476` and `metasystem/cmd/metasystem/goalsync_mutations.go:525-528`. Current live goal claims also use the `main-...` form, for example `metasystem/plans/goals/coordinator-context-stays-under-budget.md:17`.
- The current idle gate already implements the ruled bounded shape: it increments the empty-backlog refusal count and escalates at three, with escalation clearing the block: `metasystem/internal/goal/turnverdict.go:1023-1050` and `metasystem/internal/goal/turnverdict.go:1151-1163`.

## Cap-column audit

The claim that twelve units need no cap-group unit is false by one. Counting both direct and transitive prerequisites in section 6, **11 of 18** units can build without an unlanded cap-group unit:

`U3c-0`, `U3c-5a`, `U3c-5b`, `U3c-1a`, `U3c-1c`, `U3c-2a`, `U3c-2b1`, `U3c-3a1`, `U3c-3b1`, `U3c-4a`, and `U3c-4b`.

The seven that cannot are `U3c-1a2` (`U3b-1`), `U3c-2b2` (`U3e-2b`), `U3a-2` (`U3a-1b`), `U3c-3a2` (`U3b-1`), `U3c-3b2` (`U3b-1`), `U3c-1b` (transitively `U3b-1` and `U3a-1b`), and `U3c-6` (`U3e-1` and `U3e-2a2`). The table's individual dependency declarations are sufficient; the aggregate count is arithmetic and does not itself change what an implementer would build.

Material: no

## Material findings

### SUCC-R3-01 — The successor lineage is the expressly refused bare job id

Severity: critical  
Material: yes

Section 3.1c requires `METASYSTEM_OWNER_LINEAGE=<jobId>` and declares the job id itself to be the custody lineage. Sections 3.6a and 3.7 then reuse that value for registered waits and goal-claim ownership. This directly contradicts the final seat ruling: the successor must retain the engine grammar `main-<epoch>-<pid>-<suffix>`, carry the job id inside the suffix, and export that full value as `METASYSTEM_OWNER_LINEAGE`; a bare job id is refused.

This is an authority identity, not a display label. Goal requests copy the environment value into the actor (`metasystem/cmd/metasystem/goalsync_mutations.go:470-476`, `metasystem/cmd/metasystem/goalsync_mutations.go:525-528`), while engine-created lineages have the required form (`metasystem/internal/lease/verbs.go:181-205`). As written, U3c-1a would install the wrong identity, and U3c-5a's claim/return/release behavior would be proved against the wrong owner.

Required correction: replace bare-job lineage throughout sections 3.1c, 3.6, 3.7, the witnesses, and the affected build units with a `main-<epoch>-<pid>-<suffix-containing-job-id>` lineage exported by the shared launch seam. Add the seat-required mutation proof that a goal verb accepts this successor lineage and refuses a malformed lineage, including the bare job-id form.

### SUCC-R3-02 — The mutation fence remains a check-then-publish race

Severity: critical  
Material: yes

Sections 3.8c–3.8d move the fence into the common goal request and require it on every `Mutate` invocation. That makes coverage exhaustive, but not atomic. `goal.Publish` calls `Mutate` and only afterward constructs and validates the commit and attempts compare-and-swap publication (`metasystem/internal/goal/txn.go:609-708`). The successor can stamp its start marker after `SessionFence` checks the marker but before that publication. The stale predecessor mutation then publishes despite the stated invariant that none may publish after successor start.

The proposed witness pauses before `Publish`, flips the marker, and resumes. That only proves rejection when the marker already exists at the start of `Mutate`; it cannot expose a marker written after the fence check.

Required correction: define one atomic ordering mechanism shared by successor start-marker publication and goal publication—such as a common lock or a version checked by the publication compare-and-swap—and state its crash semantics. The witness must pause after the last fence check but before the goal CAS, publish the successor marker, resume, and prove that no predecessor commit lands.

### SUCC-R3-03 — The ruled slow retry cannot pass either of the decision path's admission guards

Severity: critical  
Material: yes

Section 3.3i requires consumed terminal attempts at or above the dry cap to call `relaunchAttemptLocked`, and section 3.3b says relaunch re-evaluates `decideForHandoff`. However, the current decision holds when `others > 0` and notifies when `DryRevivals >= MaxRevivals` (`metasystem/internal/steward/handoff.go:116-125`). Section 3.2b excludes a consumed intent only when its *current attempt job id* equals `PredecessorJob`; for the continuation being relaunched, those are different identities. Its own consumed intent therefore remains in `others`. Even if that were corrected, the unchanged dry-cap guard still returns notify on precisely the at-cap recovery that section 3.3i commands.

No rule defines a chain-local exception or distinct admission path for the seat-ruled slow retry, and U3c-1c does not own an explicit change to either guard. Consequently the retry unit can schedule forever but cannot authorize a launch.

Required correction: specify the exact relaunch admission contract: exclude the current intent by stable nonce/chain identity, not by predecessor-job equality, and define the narrow ruled path that may pass the ordinary dry-revival cap without weakening other dry-cap decisions. Add mutations independently restoring each old guard and prove that the eligible at-cap terminal attempt still relaunches while another chain does not.

### SUCC-R3-04 — A terminal attempt is authorized for redispatch under an id that can only replay

Severity: critical  
Material: yes

Section 3.3b permits dispatch when there is no record for the attempt job id **or when that record is terminal**. Current dispatch semantics do not relaunch a terminal record with the same id; they return the recorded terminal result (`metasystem/internal/dispatch/claim.go:594-635`). Thus a crash after dispatch but before `AttemptDispatchedAt` is stamped can leave a terminal job record which the design authorizes for “redispatch,” but the dispatcher will only replay. No successor starts, while the attempt may be treated as handled.

The named duplicate-dispatch witness covers only an existing running record, so it cannot catch this terminal replay case.

Required correction: make terminal recovery allocate and durably authorize a fresh per-attempt job id before dispatch, or define a supported dispatcher operation that truly creates a fresh attempt. Extend the witness through crash-after-dispatch-before-stamp with a terminal prior record and prove exactly one new launch under a distinct id.

### SUCC-R3-05 — Expiry marks the only recovery record terminal before its notice is durable

Severity: high  
Material: yes

Section 3.12b orders expiry as: return/release the claim, write `CancelledAt`, then queue the attention item. It says a crash between the latter two steps repeats the queue write, but the same section says cancelled records are excluded from active selection and ignored by the reaper and slow retry. After that crash, no specified selector can rediscover the record to perform the alleged repeat. The required eight-hour expiry notice can therefore be lost after authority has already been released.

Current reaping deliberately uses the recoverable order—queue first, terminal mark second—so a crash before the terminal mark remains eligible for replay (`metasystem/internal/steward/reap.go:31-80`).

Required correction: make the notice durable before `CancelledAt`, with an idempotency key, or add a durable notice-pending phase that a named reconciler selects after cancellation. The witness must crash at every boundary, restart, and prove one notice and one terminal cancellation.

### SUCC-R3-06 — The proposed Linux census identity is always invalid

Severity: high  
Material: yes

Section 3.2a constructs each `identity.Ref` with `StartedAtSec`, `StartedAtUnixMicro`, `StartTicks`, and `BootID` all populated. The identity package defines microsecond and ticks-plus-boot as mutually exclusive modes; a ref containing both returns `CompareInvalid` (`metasystem/internal/identity/identity.go:21-31`, `metasystem/internal/identity/identity.go:72-109`). Linux production census supplies both raw representations (`metasystem/internal/census/production.go:45-70`). Consequently every Linux successor census entry takes the non-excludable invalid path, so a sole predecessor cannot be excluded and can never satisfy the launch precondition.

Required correction: construct the platform's one canonical native-exact ref—ticks plus boot id on Linux, microseconds on macOS—rather than combining both representations. The witness must include a Linux-shaped record carrying both raw census fields and prove that canonicalization produces `CompareNativeExact` for the predecessor and still refuses genuinely invalid identity.

### SUCC-R3-07 — The mandatory idle-end notice has no witness that fails when the notice is removed

Severity: medium  
Material: yes

Section 3.4c correctly adopts the final seat ruling: after three unchanged empty-backlog Stops, allow termination and queue `seat-chain-idle`. But its witness, `TestContinuationVerdictHasNoMilestoneAllow`, only names blocking before the bound and allowing at the bound. Deleting the attention-item enqueue leaves that witness green. That violates the binding rule that every rule name the witness that fails without it, and leaves the final idle-end unit unable to prove the notice half of the seat ruling.

Required correction: add a named mutation witness that removes the third-refusal `seat-chain-idle` enqueue and must fail, while also proving no notice before the bound and one idempotent notice when the bound allows termination.

## Recommended answers to the design's open questions

1. Idle end: confirm the seat ruling—an unchanged empty backlog uses the existing “refusal 3 of 3, then allow” shape, emits the notice at the bound, and launches no successor.
2. Successor lineage: require `main-<epoch>-<pid>-<suffix-containing-job-id>`, export the full value as `METASYSTEM_OWNER_LINEAGE`, and mutation-test acceptance of it plus refusal of bare/malformed values.
3. Section 6 amendment: add the ruled one-line per-unit-cap-dependency amendment to `metasystem/plans/coordinator-context-stays-under-budget-design.md` when this design page lands.

## Evidence limits

- The permitted `bin/metasystem goal show --id seat-successor-continues-a-handoff-without-a-human` could not be run because this worktree has no `bin/metasystem`; the tracked goal file was inspected directly.
- Per the review constraint, no tests, fixture beds, or commands that start, signal, or kill processes were run.
- Test-harness feasibility claims that cannot be established by static inspection remain for the builder; none is used to dismiss a finding above.

Proposed receipt line: design critique round 3 completed against `origin/main` `7df4c508`; seven material build/proof defects found; verdict rework.

## Rigor classification

| findingId | rigorClass | facts | reopeningTrigger |
|---|---|---|---|
| SUCC-R3-01 | severe | `local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=true; externalSideEffectBoundaryCrossed=false` | Reopen when the design exports an engine-grammar successor lineage containing the job id and names the goal-verb accept/refuse mutation proof. |
| SUCC-R3-02 | severe | `local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=true; externalSideEffectBoundaryCrossed=false` | Reopen when successor start and goal publication share an atomic ordering rule and a post-check/pre-CAS race witness. |
| SUCC-R3-03 | severe | `local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | Reopen when the design defines current-intent exclusion and the narrow at-cap retry admission path with independent guard mutations. |
| SUCC-R3-04 | severe | `local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | Reopen when terminal recovery necessarily uses a fresh dispatch identity and the crash window is proved end to end. |
| SUCC-R3-05 | severe | `local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=true; externalSideEffectBoundaryCrossed=true` | Reopen when expiry has a replayable durable notice phase and crash-boundary witnesses prove one notice and one cancellation. |
| SUCC-R3-06 | severe | `local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | Reopen when census canonicalizes to one platform-native exact mode and the Linux-shaped witness proves predecessor exclusion. |
| SUCC-R3-07 | severe | `local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | Reopen when deleting the idle-bound notice makes a named mutation witness fail. |

Material finding count: 7

VERDICT: rework
