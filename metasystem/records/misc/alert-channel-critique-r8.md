# Alert channel design critique — revision 8, round 2 (Sol)

Chain: design round implementer-4401253ea4b95f3958b61c71 (Fable lane,
recovery-certified) -> critic design-critic-82a0663b42cbce77c0ffc515
(codex gpt-5.6-sol, xhigh, fresh context), 2026-09-01. Reviewed bytes
identified by the critic as SHA-256
ae0d3fb7ec76bb3688cdbf8b4ea76dd93aabc912c71ae19f9d8d058ec2f5e93f —
verified by the seat as byte-identical to the landed revision 8
(commit 3b6a5a7f, origin/main), closing the critic's declared
provenance gap: the seat's local branch was one commit behind its own
pushed landing at worktree spawn (a raw-git repair mistake, disclosed
in the landing that carries this record). Six material findings — one
critical, five high. Revision 9 must fold or refute each by id.

## AC8-JOB-SOURCE-RETENTION-001 — critical, material=True

CLAIM: The delegate-job-failed scan does not eliminate the original pre-journal loss window because its sole enumerated source is actively garbage-collected. A runner outage can therefore outlive local record retention and permanently suppress an alert that the design still owes.

EVIDENCE: Metasystem plan section 11a.8 says “terminal records are never deleted” and that the scan re-derives “on EVERY tick”; section 1 similarly says “no shipped path deletes them.” In contrast, metasystem/internal/evidence/gc.go lines 81–100 and 369–449 explicitly prune mirrored terminal job records after the grace window, and metasystem/scripts/agents/evidence-gc.sh sets that window to 5,400 seconds by default. Once the bound goal revision is no longer the current claimed revision, lines 477–512 permit pruning. Interleaving: a job fails, the goal later moves out of that claimed revision, the runner remains down while the chain is mirrored and pruned, and the next scan finds neither the local record nor its durable mirror because section 11a.8 scans only artifacts/agents/jobs/*.json. This recreates AC7-PRODUCER-ATOMICITY-001, the producer-atomicity defect, rather than folding it.

## AC8-STOP-BATCH-BINDING-001 — high, material=True

CLAIM: The stop scan accepts less evidence than the prescribed goal-resume command, so it can publish an alert claiming resume is available when that command will refuse.

EVIDENCE: Metasystem plan section 11a.9 defines the source as a fence whose ReadStopBatch result “reads COMPLETE” and claims “resume's own precondition VerifyStopBatchComplete guarantees the prescribed verb will not refuse.” Metasystem/internal/goal/stop.go lines 181–206 show that ReadStopBatch checks only the batch document itself. Lines 248–262 show that VerifyStopBatchComplete additionally compares goal identifier, goal revision, fence epoch, capability generation, machine, claim epoch, and reason against the accepted fence. A readable COMPLETE batch with contradictory coordinates passes the specified scan but fails resume. An implementer following section 11a.9 would therefore build a weaker predicate than the command whose availability the alert asserts.

## AC8-STOP-RESUME-RACE-001 — high, material=True

CLAIM: A human resume can race the stop scan and cause a stale stop-awaiting-resume message to be sent after the goal has already resumed.

EVIDENCE: Metasystem plan section 11a.9 says “after resume no alert is owed — the human has acted,” while section 5 sends only after RunTick returns. Metasystem/internal/steward/tick.go lines 107–114 serialize ticks with the steward arbitration flock. Metasystem/cmd/metasystem/goalsync_mutations.go lines 394–413 shows goal resume instead takes a goal-revision lock, with no steward arbitration lock. The lawful interleaving is: the tick projects a COMPLETE fenced goal and journals a pending episode; goal resume then publishes the fresh revision; RunTick returns; DeliverDueAlerts sends the pending message saying “the goal waits for resume.” The next scan can clear the episode, but cannot retract the already submitted false message. The design must choose serialization, a pre-send source recheck, or explicitly weaker stale-message semantics.

## AC8-SCAN-BOUNDEDNESS-001 — high, material=True

CLAIM: The new periodic scans and their deduplication are not bounded: they repeatedly read growing job and episode history while holding locks on the core tick path.

EVIDENCE: Metasystem plan section 11a.8 specifies a full artifacts/agents/jobs/*.json pass per tick and offers only the analogy “the same order as the watcher's existing per-interval census.” It supplies no count, byte, duration, cursor, or retention bound. Section 11a.10 says delegate-job-failed episodes are “NEVER auto-cleared.” Metasystem/internal/evidence/gc.go lines 477–512 retains every terminal record that still contributes to the current claimed revision, while metasystem/internal/steward/alert_episode.go lines 126–149 loads every retained episode to perform digest lookup. Thus deduplication is stable only because history is retained indefinitely, and both source scanning and episode lookup grow with history. A fresh implementer has no specified lawful bound or checkpoint and will build increasing I/O and alert-lock hold time on the repeating tick.

## AC8-STOP-INDETERMINATE-LIFECYCLE-001 — high, material=True

CLAIM: The stop episode lifecycle is contradictory when a previously COMPLETE batch becomes temporarily unreadable, forcing an implementer to guess whether to suppress an already pending alert.

EVIDENCE: Metasystem plan section 11a.9 says “Closed StopFence + batch INDETERMINATE or unreadable → NO ALERT yet,” but section 11a.10 says stop-awaiting-resume episodes “are resolved and cleared by their own scan when the 11a.9 condition no longer holds.” Because the exact condition is that ReadStopBatch reads COMPLETE, a transient read error makes it false even though the fence remains closed and no human resumed. Clearing cancels delivery because section 5 excludes cleared episodes from due sends; retaining the episode contradicts the literal clear-when-false rule. These builds differ in whether Wido receives an alert, so this is an implementation gap under the design's own third-gap-stop reject condition.

## AC8-ANSWER-BYTES-AND-ACTION-001 — high, material=True

CLAIM: The composer repair is still not byte-exact or mechanically actionable because both new producer classes leave Answer bytes and required command arguments unresolved.

EVIDENCE: Metasystem plan section 6 says the message is byte-exact and the Answer is the “exact ANSWERING ACT.” Section 11a.8 supplies “metasystem delegate --follow-up <job-id> --brief <corrective-brief-file>” but specifies substitution only for the job identifier, leaving the brief token's literal-versus-path treatment undefined. Section 11a.9 says only “metasystem goal resume with the goal id appended.” Metasystem/cmd/metasystem/goalsync_mutations.go lines 104–180 and 347–417 require the flag form --id, a --by value, and all four budget flags. An implementer must invent whether the identifier is positional or flagged, what placeholders remain literal, and which mandatory arguments appear. Those choices change both message bytes and whether the advertised action can run, so AC7-COMPOSER-BYTES-001 is only partially folded.

## Critic-declared gaps

- Revision-identification gap: revision 8 is present only as modified working-tree bytes. Commit 0deec6c3863bab72bd7b6bfa01ee3cfd93db4d79 does not contain those bytes, and metasystem/plans/alert-channel-r8-critique-brief.md is untracked. This review identifies the design by SHA-256 ae0d3fb7ec76bb3688cdbf8b4ea76dd93aabc912c71ae19f9d8d058ec2f5e93f rather than silently claiming it was landed.
- The generated runtime notice classifies context isolation and tool-catalog observation as advisory. I independently read the present repository sources, but the harness cannot prove fresh-context isolation.
