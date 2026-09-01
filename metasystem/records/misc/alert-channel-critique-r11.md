# Alert channel design critique — revision 11, round 5 (Sol)

Chain: revision 11 (spike-evidence-backed) -> critic
design-critic-91af2db9aaa1803a17478ab9 (codex gpt-5.6-sol, xhigh,
fresh context), 2026-09-01. Convergence restored: ONE material finding
(the advertised remedy can be stale at journal time — the producer
derives it from the failed record alone, while the shipped dispatcher
gates on chain closure, newest member, and the reviews target's
continued existence). Revision 12 folds it; closure expected on the
re-critique.

## AC11-ANSWER-JOURNAL-ELIGIBILITY-001 — high, material=True

CLAIM: AC11-ANSWER-JOURNAL-ELIGIBILITY-001, the journal-time remedy eligibility defect: the AC9-ANSWER-FOLLOWUP-ACTION-001 fold still does not make its advertised command valid when the episode is journaled. The producer chooses follow-up from the failed record alone, so after an outage or delayed first scan it can process an older protocol-error record even though the chain is already closed or a newer ineligible round already exists. For fresh code-critic and warden commands, persisting the immutable reviews identifier does not preserve its referenced implementer record; that record may already have been collected, causing the shipped dispatcher to refuse. The spike tested an initially intact chain and checked only that reviews was nonempty, so it did not cover either already-stale condition. An implementer following the design would therefore build Answer lines that can already be refused at journal time, contradicting the design's explicit journal-time-validity claim.

EVIDENCE: metasystem/plans/alert-channel-design.md lines 1343-1358 derive answerAction only from the scanned record, while lines 1404-1407 claim the resulting follow-up is accepted when journaled. Lines 1395-1399 incorrectly equate reviews-field immutability with reference validity. metasystem/scripts/agents/dispatch.sh lines 1226-1231 require the reviews target to exist and be an implementer, and lines 1698-1757 gate follow-up on the root, chain closure, newest member, and session. The durable F4 source extracted from metasystem/artifacts/agents/implementer-142fd88a8c93640bc0f9969e/rounds/1/claude-stream.jsonl checks only a nonempty reviews string and begins its follow-up case with an intact chain.

## Critic-declared gaps (verbatim)

- The task describes semantic critique round five, but the generated runtime notice identifies this job as round one. No same-session chain evidence was supplied, so the return preserves the harness-observed round number.
- The revision-eleven critique brief does not declare the failsafe round, threat model, or risk appetite required by the design-critique skill. The review therefore covered the entire design and current cited code, but cannot classify hypothetical corruption or hostile-state cases as inside or outside an undeclared threat model.
- The spike could not execute the dispatcher script through its real process boundary because lease and census preconditions require live supervision. Its F4 result is a transcription, and that transcription omitted the reviews target-existence and target-role checks. The present finding is grounded in reading the shipped script rather than treating that replay as complete execution evidence.
- The throwaway spike package was not landed, as intended, so its tests could not be rerun from the current tree. The durable test source and recorded transcripts were read instead.
- The runtime exposed no session identifier, so sessionId is reported as an empty unobserved value and no alternative session is claimed.
