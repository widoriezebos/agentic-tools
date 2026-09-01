# Alert channel design critique — revision 9, round 3 (Sol)

Chain: revision 9 (authored by implementer-6df0467b2f1db45f5cecccdf,
completed by recovery implementer-ae1f3350f04e72277adc25b4) -> critic
design-critic-a27506cb4736a12e5dcfc31c (codex gpt-5.6-sol, xhigh,
fresh context), 2026-09-01. Five material findings, one critical
(job-id reuse ABA in the retention proof). Revision 10 folds each by
id.

## AC9-RETENTION-DIGEST-ADDRESSING-001 — high, material=True

CLAIM: The retention handshake lacks the episode-addressing operation on which all four interleaving rows depend. Section 11a.12 requires garbage collection to test existence with one stat of a digest-named episode file, while the shipped alert store names files by a distinct episode identifier containing only a digest prefix and sequence. Section 11a.8 instead relies on listing and fully loading the episode store. An implementer must therefore invent a new file layout or index, violate the one-stat rule, or stat a nonexistent path and pin records forever. The only shipped job-record collector is covered conceptually, and a stat would not create a lock-order deadlock, but the pin has a liveness deadlock as specified because successful journaling need not make its existence predicate true.

EVIDENCE: metasystem/plans/alert-channel-design.md section 11a.12 says episode existence is one stat of a digest-named file. In metasystem/internal/steward/alert_episode.go, alertPath accepts EpisodeID, nextEpisodeID produces alert-<digest-prefix>-<sequence>, and Digest remains only a JSON field. No full-digest filename or durable digest-to-episode-identifier index exists. This is also an internal inconsistency across the recorded recovery seam: section 11a.8 acknowledges full-store indexing, while the recovery-authored section 11a.12 assumes direct addressing.

## AC9-JOB-ID-ABA-001 — critical, material=True

CLAIM: The critical AC8-JOB-SOURCE-RETENTION-001 fold is unsafe under job-identifier reuse, an interleaving absent from its four-row proof. The alert digest contains the producer class and job identifier but no immutable job generation. After an old record is collected, the shipped dispatcher can create a new record with the same identifier. An old episode then both suppresses creation of an alert for the new failure and satisfies the new record's collection pin, permitting that record to disappear without any episode belonging to it. Conversely, collecting the old episode because its source is gone does not prove it can never be reminted: the reused identifier can produce the same digest later. This refutes both the pin safety premise and the converse bound.

EVIDENCE: metasystem/internal/dispatch/record.go RecordCreate checks only whether the current jobs/<identifier>.json path exists. Explicit job identifiers are supported, and no tombstone prevents reuse after metasystem/internal/evidence/gc.go removes the old record. Metasystem/plans/alert-channel-design.md sections 11a.8 and 11a.10 derive identity from class plus JobID, and section 11a.12 treats episode existence and source disappearance as generation-proof facts. The lawful omitted ordering is: old job J fails, its episode is journaled, the old record is collected, a new job J fails, the old digest suppresses its journal, and garbage collection treats that old episode as the new record's pin.

## AC9-SCAN-BOUNDEDNESS-001 — high, material=True

CLAIM: AC8-SCAN-BOUNDEDNESS-001 remains open. Using the durable episode store as a checkpoint makes deduplication durable, but it does not bound the read set: the shipped lookup opens every retained episode under the repeating tick path, the job scan still lists every retained job, and the proposed converse supplies no finite bound on either set. Unacknowledged delegate and stop episodes explicitly accumulate, health episodes have no stated collection case, and 'failures of one outage' is not a count because an outage can last indefinitely. A fresh implementer must guess a work cap, cursor, index, or acceptable unbounded lock hold.

EVIDENCE: metasystem/internal/steward/alert_episode.go loadAlertEpisodesUnlocked lists and decodes every episode. Metasystem/plans/alert-channel-design.md section 11a.8 expressly accepts that full load and specifies no durable cursor. Section 11a.12 says unacknowledged episodes accumulate and bounds pins only by failures during one outage; its delivery-closed cases enumerate delegate and stop episodes but not the existing health class. Therefore neither source retention nor episode collection establishes a numeric count, byte, or duration bound beneath the tick and alert locks.

## AC9-STOP-SUPPRESSION-MERGE-001 — high, material=True

CLAIM: The AC8-STOP-RESUME-RACE-001 suppression ordering is contradictory at the operation that must cancel a stale send. The new immediate pre-send VerifyStopBatchComplete recheck is correctly ordered, and the design explicitly accepts a resume occurring after that check. But when the recheck proves the fence gone, section 11a.9 says to clear the episode through the section 5a merge, while section 5a forbids that merge from changing Cleared. One implementation preserves Cleared and can send the stale alert; another clears it and violates the mandatory merge fixture. The exact binding and indeterminate-evidence hold rules are otherwise folded.

EVIDENCE: metasystem/plans/alert-channel-design.md section 11a.9 requires the fence-gone pre-send branch to clear through section 5a under the alert lock. Section 5a restricts completion merging to receipt fields and derived transport summary and requires every non-receipt field, including Cleared, to equal the reloaded episode. metasystem/internal/goal/stop.go confirms that resume does not share the tick arbitration lock, so this clear transition is not optional.

## AC9-ANSWER-FOLLOWUP-ACTION-001 — high, material=True

CLAIM: AC8-ANSWER-BYTES-AND-ACTION-001 is byte-exact now but still not mechanically actionable. The delegate producer selects every failed or timeout job and always advertises metasystem delegate --follow-up. The shipped command refuses timeout, process-lost, and ordinary failed records; even an otherwise eligible record is refused without a resumable session identifier. This directly contradicts the design's statement that the action remains total when a session cannot be resumed. An implementer must change the source predicate, advertise fresh dispatch for some outcomes, or change dispatcher semantics.

EVIDENCE: metasystem/plans/alert-channel-design.md section 11a.8 selects status failed or timeout and fixes the follow-up command as the Answer. metasystem/scripts/agents/dispatch.sh lines 1715-1746 permit a new follow-up only after completed or failed with protocol_error and require a resumable session identifier. metasystem/cmd/metasystem/delegate.go delegates the public follow-up verb to that shell implementation without adding the promised fallback.

## Critic-declared gaps (verbatim)

- The critique chain describes this as round 3, while the generated runtime notice labels this launched critic job Round 1. The return preserves the harness-observed round number 1 rather than silently substituting the semantic design round.
- The generated runtime notice did not expose a runtime session identifier and classified context isolation and tool-catalog observation as advisory. The sessionId is therefore reported as unobserved, without claiming an alternative identity.
- No implementation of the new pin or episode collector exists to execute. The retention verdict is based on the written contract and the complete shipped collection and episode-storage surfaces; live proof was not required.
- The final worktree status contained an unrelated modification at metasystem/records/narrator-digest.log. I neither inspected nor changed that file; the reviewed design itself is byte-identical to its landed revision-9 commit.
