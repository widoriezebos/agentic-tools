# Alert channel design critique — revision 13, round 7 (Sol)

Chain: revision 13 -> critic design-critic-5e2ff3b90c478e8ecc526648
(codex gpt-5.6-sol, xhigh, fresh context), 2026-09-02. THE REMEDY
CORNER DIVERGES: three material findings (one new critical), all in
the advertised-command eligibility mirror — rounds 5, 6, and 7 went
one, two, three findings in this one corner while the rest of the
design has been stable for three rounds. The seat's read: the
advertisement machinery is chasing an unwinnable mirror of the
dispatcher's mutable preconditions; the fork (simplify to stale-proof
facts, or keep folding) is Wido's, put to him with the seat's
recommendation to simplify.

## AC13-ANSWER-REFERENCE-ABA-001 — critical, material=True

CLAIM: AC13-ANSWER-REFERENCE-ABA-001 — The read-time safety claim is false because the advertised chain-root and reviews-target references carry only reusable job identifiers. Revision 13 promises that every stale command is loudly refused and never performs a silent wrong action. However, after journaling, evidence garbage collection may remove the completed chain root or reviewed implementer record; a lawful fresh dispatch may then reuse the same identifier for a different job incarnation. The stale follow-up command will resolve the replacement chain, while a stale code-critic or warden command will accept the replacement record whenever its role is implementer. Either command can therefore be accepted against unrelated work rather than refused. An implementer must bind these references to an incarnation, retain their records until the alert closes, or explicitly accept possible retargeting; those choices produce materially different systems.

EVIDENCE: metasystem/plans/alert-channel-design.md lines 307–337 establish that collected job identifiers are reusable; lines 1670–1675 retain only the reviews identifier; lines 1703–1715 admit that completed chain roots are not pinned while claiming every stale reference refuses rather than acts wrongly. metasystem/internal/dispatch/record.go lines 245–271 refuses an identifier only while its record exists. metasystem/scripts/agents/dispatch.sh lines 1226–1231 validates a reviews target only by current path existence and role, while lines 1698–1726 resolves follow-up state from the record currently occupying the supplied root identifier.

## AC13-ANSWER-JOURNAL-SNAPSHOT-001 — high, material=True

CLAIM: AC13-ANSWER-JOURNAL-SNAPSHOT-001 — The round-six atomicity finding is not folded: the alert lock does not establish the source-state snapshot that revision 13 advertises. The design itself says none of the gated facts' writers use that lock and that a fact may change between its read and the durable save. Reading each fact once merely freezes a possibly fractured in-memory view; it does not verify that the command is valid when the episode is journaled. The instruction not to take a source lock or perform a final validation makes an implementer preserve the exact pre-save race reported in round six. The contract must either say the command is best-effort during journaling as well as at read time, or specify a source serialization or version-validation mechanism.

EVIDENCE: metasystem/plans/alert-channel-design.md lines 1598–1623 call the operation one snapshot but explicitly admit that source facts can change between their reads and the save because their writers do not share the alert lock. Lines 1689–1701 nevertheless call the command verified at journal time. metasystem/records/misc/alert-channel-critique-r12.md lines 7–14 identified precisely this check-to-save window as the material atomicity defect.

## AC13-ANSWER-GOAL-BINDING-INCOMPLETE-001 — high, material=True

CLAIM: AC13-ANSWER-GOAL-BINDING-INCOMPLETE-001 — The new current-goal gate mirrors only part of the dispatcher's goal-binding precondition. It checks that the goal is live, claimed, and has a claim record, but the real goal-binding operation also refuses a claimed goal whose breach-stop capability is absent. That capability is present in the same goal-tree projection, so this is not an unknowable future-caller condition. A legacy claimed goal that predates breach-stop authority passes revision 13's check and receives a concrete command that the dispatcher immediately refuses. An implementer following the design would therefore build a systematically dead-on-arrival Answer for a supported goal state.

EVIDENCE: metasystem/plans/alert-channel-design.md lines 1581–1597 defines check (e) solely as presence in the live set with claimed state and a claim record, then says no other readable dispatcher gate exists. metasystem/internal/dispatch/stop.go lines 53–59 first checks those fields and then separately rejects a nil StopCapability with the message that the goal predates breach-stop authority. metasystem/scripts/agents/dispatch.sh lines 1327–1335 and 1773–1780 invoke that full goal-binding operation for both advertised commands.

## Critic-declared gaps (verbatim)

- The task describes semantic critique round seven, but the generated runtime notice identifies this job as round one and supplies no resumed critic session. The returned round therefore preserves the harness-observed value of one.
- The seven critique briefs do not declare the failsafe round, threat model, risk appetite, or critic-chain budget required by metasystem/skills/design-critique/SKILL.md. Semantic round six exhausted the second three-round budget with material findings, and no human reopening ruling was found; lawful continuation and design-phase closure therefore cannot be confirmed.
- The launcher classifies this broad-read job as advisory and says its provider tool catalog is unobserved. No finding depends on the unavailable catalog.
- An unrelated tracked modification appeared at metasystem/records/narrator-digest.log during review. The reviewed design remained at its landed revision and was not modified.
