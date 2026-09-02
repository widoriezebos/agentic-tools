# Failed-job-attention design critique — round 2 (Sol)

Chain: revision 2 -> critic design-critic-637110922a03a91e852d2dcc
(codex gpt-5.6-sol, xhigh, fresh context), 2026-09-02. Four material
findings: the reused-identity fold documents rather than prevents; the
write-ahead journal can narrate uncommitted transitions; the queued-
notification snapshot races its own bound; the channel migration
window is unowned. Two are regressions the fold introduced. Box at
its edge; parked cold-resumable per the loop law's second-exhaustion
rule.

## FJA-R2-BIRTH-ABA-REMAINS — high, material=True

CLAIM: FJA-R2-BIRTH-ABA-REMAINS, the reused-job identity defect, remains material: the fold documents the missed-alert behavior instead of preventing it. An implementer following revision two ships before the queued birth-token producer, hashes createdAt, startedAt, or an empty value, and permanently suppresses a later failed incarnation when a lawful identifier reuse produces the same fallback bytes. The promised one-line upgrade is also incomplete for pre-contract records: replacing a previously used createdAt fallback with an absent minted token changes the digest and can create a second episode f

EVIDENCE: metasystem/plans/failed-job-attention-design.md:204 accepts the fallback until the sibling goal lands and calls the resulting suppression a bounded loss. metasystem/plans/goals/job-record-birth-token.md:3 says that producer is still queued. metasystem/internal/dispatch/record.go:60 omits createdAt from immutable fields, while its create path permit

## FJA-R2-TRANSITION-PHANTOM — high, material=True

CLAIM: FJA-R2-TRANSITION-PHANTOM, the write-ahead journal regression, can narrate a raise or clear that never committed. The design records the transition before writing the episode or clearing it, but it neither marks the journal entry committed nor validates the corresponding state before narration. A failed episode write followed by record removal therefore leaves a durable raised entry that a later successful tick emits even though no episode ever existed.

EVIDENCE: metasystem/plans/failed-job-attention-design.md:368-388 requires journal-before-episode and journal-before-clear ordering, then directs NarrateDigest to emit every journal entry. Fixture 12 starts only after the episode or clear has already committed, so it does not cover failure between the journal append and state write. The off-switch lifecycle

## FJA-R2-PENDING-SNAPSHOT-RACE — high, material=True

CLAIM: FJA-R2-PENDING-SNAPSHOT-RACE, the queued-notification fold's concurrency regression, invalidates the promised bound of exactly one stale delivery after acknowledgment or resolution. RunTick serialization ends before delivery begins, and delivery uses a previously decoded queue snapshot. Two overlapping resident-runner or command-line passes can therefore retain the same escalation in memory; a later sweep can acknowledge and remove the file while both earlier passes still perform the external send.

EVIDENCE: metasystem/internal/steward/tick.go:110-114 releases the arbitration lock when RunTick returns. metasystem/internal/steward/runner.go:131 and metasystem/cmd/metasystem/steward_verbs.go:270 call DeliverPending afterward. metasystem/internal/steward/notify.go:64-93 reads the queue once and sends without rechecking file existence or episode state. Rev

## FJA-R2-CHANNEL-MIGRATION-UNOWNED — medium, material=True

CLAIM: FJA-R2-CHANNEL-MIGRATION-UNOWNED, the private-namespace integration regression, leaves the promised bounded duplicate-alert window without an owner or bound. The current build retains private failed-job episodes indefinitely, while the already-written channel design independently mints alert-prefixed episodes for the same standing records and contains no rule that consumes, supersedes, acknowledges, or retires the private episodes. Following both designs can therefore produce two continuing nags rather than one temporary migration duplicate.

EVIDENCE: metasystem/plans/failed-job-attention-design.md:141-152 delegates the migration choice and calls duplicate visibility bounded, but supplies no end condition. Its failed-job lifecycle at lines 238-243 never automatically clears the private episode. Exact searches of metasystem/plans/alert-channel-design.md and metasystem/plans/goals/alert-escalation
