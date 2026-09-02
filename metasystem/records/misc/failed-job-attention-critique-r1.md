# Failed-job-attention design critique — round 1 (Sol)

Chain: design implementer-d06bfd5d011adc3e41571c39 (Fable lane) ->
critic design-critic-aa06e6deb1d16f17a6116145 (codex gpt-5.6-sol,
xhigh, fresh context), 2026-09-02. Seven material findings, zero
gaps. Revision 2 folds each by id; the box fits fold, closing
critique, build, and code review exactly.

## FJA-R1-BIRTH-ABA — high, material=True

CLAIM: The delegate-job deduplication key is unsafe until the job-record-birth-token sibling lands. The design calls the fallback digest one episode per job incarnation, but createdAt is mutable and optional, startedAt can repeat or be absent, old record files are garbage-collected, and RecordCreate then lawfully reuses the identifier. Because these episodes never auto-clear, a reused identifier with the same fallback bytes permanently matches the old episode and suppresses the new failure—the exact attention loss this goal is meant to prevent. The design must either depend on the minted birth-token

EVIDENCE: metasystem/internal/dispatch/record.go permits reuse after record removal and does not protect createdAt as immutable. metasystem/internal/evidence/gc.go removes eligible terminal records. metasystem/plans/alert-channel-design.md records an executed identifier-reuse reproduction against these fallback fields and orders its producer after the birth-

## FJA-R1-STOP-PREDICATE — high, material=True

CLAIM: The stop firing predicate is not the alert-channel predicate whose identity this design claims to adopt. This design creates an episode as soon as StopFence exists, but the fence is durable before the cancellation batch exists or becomes COMPLETE, and goal resume refuses until VerifyStopBatchComplete proves the complete, exactly bound batch. The result can be an externally delivered alert prescribing a command that necessarily refuses. Because the early episode uses the final channel digest and is write-once, the later channel cannot mint a corrected episode when the batch actually completes.

EVIDENCE: metasystem/internal/dispatch/stop.go closes the fence before creating and advancing the stop batch. metasystem/internal/goal/stop.go makes complete batch verification a resume precondition. metasystem/plans/alert-channel-design.md defines successful VerifyStopBatchComplete as the sole stop alerting condition, while metasystem/plans/failed-job-atten

## FJA-R1-CHANNEL-PARTIAL-FACTS — high, material=True

CLAIM: The channel seam leaves an incompatible partial record under the channel's final identifier and schema. The channel contract requires answerAction and answerReason, derives them from journal-time chain and goal eligibility, and skips any digest whose episode already exists. This design omits both fields and leaves the future channel to guess between backfilling and treating absence as no answer. Backfilling can be impossible after evidence garbage collection removes the source or ancestor records, while treating absence as no answer weakens the channel's actionable-message contract. A versione

EVIDENCE: metasystem/plans/alert-channel-design.md defines the two fields as exact delegate-job facts, makes episode writes immutable after digest discovery, and allows an existing episode to satisfy the source-retention pin. metasystem/plans/failed-job-attention-design.md explicitly omits the fields and delegates the incompatible alternatives to the future

## FJA-R1-PENDING-LIFECYCLE — high, material=True

CLAIM: The stated notification off-switches do not control notifications already in the durable queue. Acknowledgment, chain closure, goal release, record disappearance, and stop-fence removal merely prevent a later requeue; none removes the existing episode-nonce file. The runner delivers that file after the tick without rechecking the episode or source, so an acknowledged or resolved condition can still send externally. Conversely, after one successful delivery, record disappearance or goal release prevents further nags while leaving an unacknowledged, uncleared delegate episode retained. The promi

EVIDENCE: metasystem/internal/steward/intervene.go says pending notifications remain until MarkDelivered removes them. metasystem/internal/steward/alert_episode.go acknowledgment does not touch that queue. metasystem/internal/steward/runner.go delivers pending notifications after RunTick without a source recheck. The lifecycle and fixtures in metasystem/plan

## FJA-R1-DIGEST-TRANSITION-LOSS — high, material=True

CLAIM: Raise and clear transitions can be lost permanently from the narrator digest. The sweep durably writes the episode or removes the stop marker before several later RunTick operations that can fail. Its Raised and Cleared flags exist only in the returned in-memory report. If the tick fails before NarrateDigest, the next sweep sees the episode already present or the marker already drained and emits neither transition, so the digest can never recover the promised entry. The fixture plan covers only a successful end-to-end tick and does not exercise this crash boundary.

EVIDENCE: The proposed insertion point in metasystem/plans/failed-job-attention-design.md is immediately after ReapContinuations. metasystem/internal/steward/tick.go performs ledger-attention, evidence, decision, and other fallible work before NarrateDigest. The design makes digest emission transition-only and derives it solely from the sweep report.

## FJA-R1-READ-BOUND — medium, material=True

CLAIM: The bounded read contract cannot implement the NAG rule as written. A stat of alert-<digest>.json proves only existence; an existing episode must be opened to learn whether it is acknowledged and to retrieve its write-once message. Section 4 enumerates only one stat per candidate and does not count these opens. Using AlertEpisodes would instead open every retained producer episode, an accumulating history, while per-source episode opens are a different algorithm that the design does not specify or measure. The implementer is therefore left to guess at the boundedness-critical seam, and no fixt

EVIDENCE: metasystem/internal/steward/alert_episode.go shows acknowledgment and Message live only inside each episode file and AlertEpisodes opens every retained file. metasystem/plans/failed-job-attention-design.md requires the NAG to inspect acknowledgment but lists only candidate stats in its bounded read set and states that delegate episodes never auto-c

## FJA-R1-PROTOCOL-WRITER-PROOF — medium, material=True

CLAIM: The fixture plan does not prove coverage of the direct RecordProtocolError terminal writer named in the attack. Its failed-job fixture starts from a generic failed record, so an implementation can raise ordinary failures while mishandling the direct writer's error value protocol_error or its nested protocolError.violation and still pass every listed test. A deterministic fixture must invoke RecordProtocolError and assert both episode creation and the rendered nested violation.

EVIDENCE: metasystem/internal/dispatch/record.go shows RecordProtocolError bypasses RecordCAS and writes its own terminal shape. metasystem/plans/failed-job-attention-design.md specifies nested violation rendering but none of its ten fixtures invokes that writer or asserts that rendering.
