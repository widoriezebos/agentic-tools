# delegate-job-liveness — verified fact sheet (2026-08-27)

Gathered by a code-grounded fact pass at tip 3f57b04. The design
record cites these by number (F1..F83). A mechanism claim in the
design without an anchor here is a defect in review.

## 1. Job-record lifecycle machinery to reuse

- F1 Three lifecycle verbs relay one package: `job record-create/record-setup/record-cas` → cmd/metasystem/dispatch_verbs.go:37-88 → internal/dispatch/record.go (RecordCreate:166, RecordSetup:205, RecordCAS:285); registered cmd/metasystem/main.go:82-84.
- F2 Every write runs inside withRecordLock (record.go:111-146) — one flock over read-decide-write, 10s bound (:133-143), atomic publish with durability doubt witnessed (:423-450).
- F3 Reservation idempotency = REFUSAL, not binding: RecordCreate stats first and refuses "job id collision" (record.go:168-172). A second dispatcher on the same id loses hard; it does not attach to the standing job.
- F4 Two-phase reservation invariant kept in shell by ruling (dispatch.sh:1034-1039): record-create husk lands before any setup work; pending-setup feeds the cleanup trap.
- F5 RecordSetup (record.go:205-225) refuses unless status pending-setup AND jobId/claimEpoch/mainId/goalId match — the atomic handshake.
- F6 RecordCAS (record.go:285-351) is the primary status mover (RecordProtocolError at record.go:227 also moves pending/running to failed — round-1 correction): lost compare prints observed= and exits 3; illegal edge refuses by name; patch may not carry status; immutable identity refused; terminal records accept only mirror/chainClosed/chainUsage/runnerClosed/critiqueExhaustions (:71-74); terminal targets stamp endedAt (:345-347).
- F7 Lawful status graph closed: pending-setup→failed; pending→running|failed|cancelled; running→completed|failed|cancelled|timeout (record.go:37-46). TerminalStatus exported (record.go:48-51).
- F8 phase=="cancelling" defers every writer except the conclude — CAS (:313-317), custody registration (custody.go:22-24), repair claim (:538-541), protocol-error stamp (:260-262). New custodial writers must obey.
- F9 Ownership-proof template fields built in Go: pid/pidStartedAt/pgid null, instanceTag "metasystem-job-"+job, custodyProcesses [] (build.go:271-275; follow-ups :402-406).
- F10 ownershipProof is SHELL-authored: printf'd at launch with the only value "trusted-launcher" (dispatch.sh:611-617), read back by substring (dispatch.sh:262,290); standing review names this defect (docs/reviews/2026-08-12-full-system-review.md:1763-1765).
- F11 CustodyAdd (custody.go:8-48) appends {pid,pidStartedAt,instanceTag} under the lock, dedupes exact (pid,start), requires instanceTag, refuses unless pending/running.
- F12 Cap resolution is Go (cap.go:36-69: arg → cap.min.<role>.<runtime>.<model> → cap.min.<runtime>.<model> → dispatch.cap-min → "120"); fence-authority refusal for unsigned mission caps (cap.go:75-89).
- F13 Cap fields immutable after setup (record.go:63) — no mid-flight raise.
- F14 Cap verdict = dispatch.CapExpired (reapfacts.go:90-104): capDeadline, else startedAt+capMin. Exported so reapers and dispatchers cannot disagree.
- F15 Cap ENFORCEMENT (the kill) lives only in the kill-capable shell path (dispatch.sh:827-832); both Go reapers decline to kill (supervise/reaper.go:17-33,119-123).
- F16 Second ceiling: cap < attested watcher ceiling or dispatch refuses (dispatch.sh:1165-1167); ceiling derives from watch.cap-min.
- F17 `job reap-facts` (reapfacts.go:44-84) is the record-only seam: {status, setupAbandoned, handshakeWaiting, budgetExpired}; AbandonedSetupGrace=10m (:34), HandshakeBackstopGraceSec=2 (:40).
- F18 `job watch` blocks to terminal with pinned exit codes incl. ExitNoRecord (watch.go:18-51) and registers a waiter record.

## 2. Process identity — the proc family

- F19 Verbs (main.go:32-47): started-at, probe, exists, group-exists, group-members, census, alive, classify, signature-check, find-ancestor, acknowledge.
- F20 `proc classify --pid --tag` (census.go:66-83 → identity/tagstate.go:19-45) returns live|stale|dead|unknown. Inputs pid+tag ONLY — no start time; recycle-proof only by tag uniqueness. ESRCH→dead, EPERM→unknown, unreadable argv→unknown. Kill-capable callers DEFER on unknown (tagstate.go:14-17).
- F21 `proc alive --pid --start-time [--start-ticks --boot-id]` (census.go:154-181) is the start-time join; (ticks,bootID) both-or-neither.
- F22 `proc started-at` emits seconds or pair SECONDS TICKS BOOTID (identity.go:14-42).
- F23 `proc probe` returns full JSON identity, exits 1 on Unknown (identity.go:46-75).
- F24 `proc find-ancestor` walks to the first signature-matched agent ancestor (census.go:198-220); used by arm-supervision.sh:360, supervision-hook.sh:75.
- F25 Group-death proof = identity.TaggedSurvivors(tag, exclude, pgid) → (alive, certain) (survivors.go:25-67); indeterminacy scoped to the recorded pgid (:41-54).
- F26 groupDeathProvenAt stamped in THREE places: supervise/reaper.go:171, missionrunner/drain.go:262,269,287, dispatch.sh:820,830,1590. Three implementations of one ladder.
- F27 REAP-DEFERRED emitted in exactly ONE place (supervise/reaper.go:163-166); REAP-DECLINED at :145-148. The missionrunner drain reaches the same gate and defers SILENTLY (drain.go:252-255) — vocabulary gap.
- F28 Reaper verdict order on proven-dead custodian: cancelling > budget > loss (reaper.go:179-202; mirrored drain.go:256-288). No recorded pid + cancelling → cancelled, no death claimed (reaper.go:128-135).
- F29 ReaperConfig injects Custodian/Survivors/Apply/Emit (reaper.go:37-65) — natural mount for a companion-shaped probe.
- F30 Census classification CUSTODY|ANNOUNCED|UNTRACKED (census/run.go:299-326); liveCustody reads jobs/*.json + runner/turn records, expands custodyProcesses[] (run.go:538-590); join pid+(ticks,bootID) or pid+seconds (:289-297). ROUND-2 CORRECTION: on macOS started-at emits ticks 0 / bootID '-' so the join degrades to whole seconds; custody entries store seconds only (custody.go:42) and the census job loader discards ticks/bootID (run.go:580) — the current join is NOT recycle-proof on darwin; the kernel provides microseconds (identity_darwin.go:48, probe's startedAtUnixMicro).
- F31 In-census requires signature match AND cwd-or-argv below the repo (run.go:168,215-236); `codex` is a configured signature (runtimes.go:167-175).

## 3. Launch sites today

- F32 codex exec invoked in exactly two live places under scripts/: critique-round.sh:40-41 (raw, uncustodied; `command -v codex` at :32 the only precondition) and the adapter argv in adapters/codex.sh:142-146 (built by `metasystem adapter codex-command`, codex.sh:95-114).
- F33 The adapter's exec IS the custodial shape: (cd workspace && exec env … cmd) <prompt >events 2>>log & then register_cli_custody "$cli_pid"; failure kills the child and fails the record (codex.sh:142-146).
- F34 register_cli_custody retries `dispatch __register-custody` until the pid's start identity reads or 5s (runtime-common.sh:81-94) → proc started-at + job custody-add (dispatch.sh:1513-1523).
- F35 A per-job heartbeat file EXISTS: artifacts/agents/hb/<job> holding {"pid","pgid","instanceTag"}, written at supervisor start (runtime-common.sh:65,76), touched in the handshake poll (codex.sh:154). Nearest existing liveness-by-mtime signal.
- F36 dispatch launch_adapter (dispatch.sh:564-639): refuse pending; build argv --job --start-gate --instance-tag; optional checkout-execution-guard run-member wrap (:575-578); supervise launch-detached (:580-588); poll started-at under scaled cap; stamp pid/pidStartedAt/pgid/ownershipProof/handshakeDeadline via CAS pending→pending (:611-617); KILL by re-proven identity if CAS lost (:618-637); then release the gate.
- F37 supervise launch-detached (supervise_arming.go:112-176) is the reusable primitive: /dev/null stdin, appended log, Setsid, optional guard membership with kill-on-failure, prints pid.
- F38 MINIMAL custodial-exec subset (ROUND-2 AMENDMENT: the cap-authority lock is part of (b) — it is NEVER droppable; see F39): (a) build-setup + record-create reservation (dispatch.sh:1031-1039); (b) cap authorization + watcher-ceiling (:1049-1052,1150-1168); (c) permission expansion + snapshot (:1085-1108); (d) build-record + record-setup (:1128-1139,1189); (e) launch-detached + started-at + ownership CAS with kill-on-lost-compare (:580-637); (f) custody-add for the real CLI child (:1522); (g) terminal CAS (runtime-common.sh:145-171).
- F39 Full dispatch also carries chain lock, worktree quarantine, mission provenance, escalation, census freshness (:1021-1141) — droppable for a non-mission custodial exec. ROUND-2 AMENDMENT: the CAP-AUTHORITY LOCK IS NOT DROPPABLE — cap serialization and kill ownership stay singular (dispatch.sh:374,1046; arm-supervision.sh:402); an unsynchronized second cap path is prohibited.
- F40 Future-runtime extension point: delegate.Driver{Declaration();Open()} keyed (runtime,transport) (delegate.go:192-225); a companion custodian is naturally a TRANSPORT under the existing key.

## 4. The companion plugin's observable surface

- F41 State dir <slug>-<hash>/ with broker.json, state.json, jobs/ (~/.claude/plugins/data/codex-openai-codex/state/agentic-tools-107e72c675394e1a/).
- F42 Per task exactly TWO files: jobs/task-<id>.json (full record incl. prompt+result) and .log (append-only). No pid file, no lock, no heartbeat.
- F43 state.json {version, config, jobs[]} denormalized index; both written on every transition (tracked-jobs.mjs:141-200).
- F44 Record fields: id, kind, kindLabel, title, workspaceRoot, jobClass, summary, write, createdAt, sessionId, status, startedAt, phase, pid, logFile, threadId, turnId, updatedAt, completedAt, errorMessage, cancelledAt. Statuses queued|running|completed|failed|cancelled.
- F45 sessionId is the CLAUDE session (env CODEX_COMPANION_SESSION_ID; tracked-jobs.mjs:6, job-control.mjs:16); codex-side identities are threadId/turnId.
- F46 pid semantics: detached wrapper's pid at queued (codex-companion.mjs:684-711), overwritten with the worker's process.pid at running (tracked-jobs.mjs:141-149), NULL on every terminal path (:163,176,190,198). Never the codex CLI's pid; no start time.
- F47 NO liveness check anywhere: status filters on status∈{queued,running} only (job-control.mjs:213-239). THE ZOMBIE MECHANISM: kill the worker and the record stays running forever.
- F48 Subcommands (codex-companion.mjs:1024-1065): setup, review, adversarial-review, task, transfer, task-worker, status, result, task-resume-candidate, cancel. status/result read-only; cancel is the only external writer: broker interrupt on (threadId,turnId), terminateProcessTree(job.pid) with NO start-time re-proof (recycled-pid hazard), then terminal write.
- F49 Duplicate-launch guard exists only inside task and TRUSTS the unverified running status (observed live: "Task … is still running") — a zombie permanently blocks new launches.
- F50 External custodian can verify: log mtime; threadId/turnId (cancel handle); pid only while non-null (worker node, no start time); sessionId (Claude-session binding); broker.json.pid. It CANNOT verify the codex CLI process or detect a killed worker; the CLI is reachable only through the census signature scan (F31).

## 5. Steward sweep seam

- F51 ReadOpenWork (openwork.go:23-98) reads the GOAL LEDGER ONLY — never job records. The liveness plug-in point is internal/steward/census.go:25-41, specifically supplementWorkers (:37) — the existing "records the runtime census never reads" hook.
- F52 Census mapping (census.go:47-70): CUSTODY|ANNOUNCED→Live, UNTRACKED→Untracked, everything else→Unprovable.
- F53 Tick flow (tick.go:56-152): lock → ReapContinuations → evidence → Observe → outage pause → decideNow → notify/revive → appetite banners (:125-139) → save → Narrate + ReachTheHuman.
- F54 decideNow calls the census only when work==WorkOwned (tick.go:197-204) — a stalled delegate under a goal-free checkout is invisible today.
- F55 Verdict ladder (verdict.go:90-133): degraded → no-work → Live>0? (stale? stalled-idle : healthy) → !CensusComplete||Untracked>0||Unprovable>0 → unknown → proven death → stalled-dead.
- F56 A stalled companion delegate ALREADY trips the ladder as VerdictUnknown (untracked in-scope codex → Workers.Untracked>0 → verdict.go:110-114 notify). The gap is ATTRIBUTION and SUPPRESSION, not detection.
- F57 The appetite-banner precedent (tick.go:125-139; nonce key :156-165) is a genuine notification ride-along; the ResolveEndpoint guard at :125 makes it degrade silently. Suppression semantics require Workers/Snapshot surgery (the existing suppressor shape is ActiveContinuation, verdict.go:71-73,118-121).
- F58 The narrator noticings channel (narrate.go:137-178) names building anomalies with dedup keys, speaks only when Action==ActNone — a delegate stall-approaching fits its exact shape (:161-168).

## 6. Related goal states

- F59 acp-adapter-seam: DONE (done/acp-adapter-seam.md:3), all three slices landed (358f970, 782d7bc, 42ee165; one hash corrected in round 1), 12 rounds. THE LIVENESS DRAFT'S STATED BLOCKER IS CLEARED.
- F60 suite-dispatch-exclusion: DONE, landed 95e432d, concluded 2026-08-27T10:41 — execution guard on both entrypoints, queue-not-refuse. Custodial launches must ride checkout-execution-guard.sh run-member exactly as dispatch.sh:575-578 does.
- F61 session-coexistence: QUEUED, 4h — promises "codex-companion job ownership recorded mechanically … so an orphaned builder has one adopter": direct overlap with this arc's shadow-record requirement; decide seam ownership or two goals build the same ledger.
- F62 steward-owned-execution: QUEUED, 3h design — steward gains an execute surface; residue R3-001 reserves hard-kill escalation, i.e. the kill authority both Go reapers decline (F15).
- F63 delegate-exec-channel: queued; orthogonal (delegate build permissions).

## 7. Existing liveness / heartbeat signals

- F64 narration.log (narrate.go:27-29): dormant — 45 bytes, last written 2026-08-23.
- F65 events.jsonl writers: runner/lease/run/conformance + shell emit_event; best-effort, 4096-byte cap (events/emit.go:1-60).
- F66 THE PIPELINE IS STALE: events.jsonl last 2026-08-16; jobs/ last 2026-08-12 (matches the goal's claim exactly); steward runner.json 2026-08-20.
- F67 watch.stale-min/cap-min = 20/180 (metasystem.conf:11-12); consumers watch-background-jobs.sh:129-137, checkout-execution-guard.sh:35-39; validated config/validate.go:311.
- F68 Watcher ceiling republished as derivedWatcherCapMin (watch-background-jobs.sh:143-158) — the number dispatch.sh:1165-1167 checks.
- F69 Existing staleness measures: TicksSinceAdvance (tick.go:85, verdict.go:63-65); AbandonedSetupGrace 10m; handshake deadline + 2s; CapExpired; heartbeat file mtime (F35); continuation 10-min window (steward/reap.go:122-127). NOTHING measures WORK-PRODUCT MTIME — the triad leg that actually caught the zombies.
- F70 plans/goals-drafts/agent-liveness-contract.md is the absorbable draft: heartbeat-per-tool-round, steward coverage, probe-resume-before-kill, incremental-landing doctrine; its SEQUENCING wait on acp-adapter-seam is OVER (F59) — absorb on the intended trigger.

## 8. Idempotent-launch precedent

- F71 Goal transactions are the exactly-once model (txn.go:490-531): entry before act, journal keyed by opid, per-op refs, Goal-Transaction trailer.
- F72 Replay verification: re-Publish of a terminal-confirmed opid re-captures the tip and re-verifies TrailerPresent before returning idempotent (txn.go:504-523); "the opid is the truth, not the belief".
- F73 Cross-process exclusion: a pushed entry with unknown outcome blocks the clone until classified (txn.go:494-498).
- F74 Mutation classification vocabulary: AlreadyApplied (own-opid proof), LostToCompetitor{Winner}, NothingToDo{Reason} (txn.go:427-451).
- F75 Job-record reservation is the OTHER pattern: refuse-on-collision, no replay verify, no bind-to-standing (F3).
- F76 Steward revival is the closest idempotent LAUNCH: reserve intent by nonce → arbitration lock re-decide → ConsumeIntent → charge the dry cap BEFORE the irreversible act → launch → StampLaunch (revive.go:129-169); crash between consume and stamp reconciles next tick via continuationOutcome (reap.go:104-128).
- F77 Synthesis: "a retry binds to the standing task" is the TXN pattern, not record-create's. Shape: caller-supplied launch opid; record-create keyed on it; on collision re-read and VERIFY the standing job (pid+pidStartedAt live via proc alive, or instanceTag via classify) before returning idempotent success; refuse loudly only when the standing record cannot be proven.

## Corrections to initial assumptions (all folded above)

- F78 ReadOpenWork never reads job records; the plug point is census.go supplementWorkers (F51).
- F79 Group-death ladder exists in three copies (F26); observable deferral vocabulary exists in one (F27).
- F80 proc classify is not recycle-proof by start time (F20); start-time proof is proc alive (F21).
- F81 ownershipProof is shell-authored with a standing named defect (F10).
- F82 Caps: resolved in Go, judged in three readers, ENFORCED only by the shell kill path; a Go custodial reaper has no kill authority today (F15, F62).
- F83 The companion pid is never a custody handle (F46); log mtime + threadId are the durable handles; the codex CLI is visible only to the census signature scan.
