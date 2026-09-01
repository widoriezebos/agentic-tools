# Alert channel design critique — revision 10, round 4 (Sol)

Chain: revision 10 (implementer-54471173c527d3d8d8e1f769) -> critic
design-critic-0868feb318b6458c6637b500 (codex gpt-5.6-sol, xhigh,
fresh context), 2026-09-01. CONVERGENCE STALL: four material findings
of which three are re-opens (the critical identifier-reuse hole — the
birth token is neither mandatory nor immutable in the shipped job
record contract; the read-set bound enumerates but does not bound;
the remedy table ignores chain-level preconditions) plus one new
stop-lifecycle regression. The critic's standing gap all four rounds:
no implementation of the disputed mechanisms exists to execute. The
loop pauses for the human fork per the design-critique stop
discipline; an evidence spike runs meanwhile.

## AC9-JOB-ID-ABA-001 — critical, material=True

CLAIM: AC9-JOB-ID-ABA-001 remains open: the proposed birth token is neither mandatory nor immutable in the shipped job-record contract, so the new identifier-reuse row does not prove that the two incarnations receive different tokens. RecordCreate accepts a source without createdAt or with an arbitrary repeated value, and RecordCAS can replace createdAt because that field is not protected as immutable. A lawful ordering therefore remains: old job J with token T is alerted and collected; a new job J is created or patched with the same T; its digest equals the old digest, so the old episode suppresses its alert and satisfies its collection pin. The disclosed monotone-clock assumption does not close these shipped non-clock paths. An implementer must choose between strengthening the record schema, minting a separate generation under the record lock, or retaining the unsafe identity.

EVIDENCE: metasystem/internal/dispatch/record.go lines 60–75 omit createdAt from immutableFields; lines 222–272 validate only jobId and pending-setup before persisting the caller-supplied object; lines 475–544 accept any patch field not in immutableFields, including createdAt, even on a transition into a terminal state. metasystem/plans/alert-channel-design.md lines 1477–1483 and 1583–1586 also map an absent token to the empty string, while line 1601 proves inequality only from wall-clock separation.

## AC9-SCAN-BOUNDEDNESS-001 — high, material=True

CLAIM: AC9-SCAN-BOUNDEDNESS-001 remains open: the new per-tick open contract enumerates work but does not bound it. The alert listing is explicitly allowed to grow without a numeric cap; outage-pinned jobs grow for the unbounded duration of an outage; each listed job requires a whole JSON record read with no byte limit; and the same tick's retained health path still opens and decodes every episode under the alert lock. Naming a size owner and promising one operation per entry makes the cost proportional, not bounded. An implementer following this design will build unbounded tick time, memory, and lock hold instead of choosing a cap, cursor, or bounded index.

EVIDENCE: metasystem/plans/alert-channel-design.md lines 1166–1200 require one directory listing and one record read per listed job, admit capless health and unacknowledged episode accumulation, and explicitly preserve the health path's full-episode load. Lines 1603–1609 admit that pinned volume is duration-proportional with no prior outage bound. metasystem/internal/steward/alert_episode.go lines 126–149 list and decode every episode, and lines 231–240 do so while the exclusive alert lock is held.

## AC10-STOP-CLEAR-READSET-001 — high, material=True

CLAIM: Revision 10 introduces a stop-lifecycle regression: the filename-only stop scan cannot perform its required clear transition after a successfully submitted stop alert is followed by goal resume. The scan forbids episode-file opens and history reads, but the SHA-256 filename cannot reveal the episode's stored goal identifier and revision, which are required to prove its fence gone. The pre-send suppression operation does not cover this case because a successfully submitted attempt is no longer due and therefore receives no later pre-send recheck. An implementer must violate the read-set contract, add a reversible bounded index, or leave submitted stop episodes uncleared and uncollectable.

EVIDENCE: metasystem/plans/alert-channel-design.md lines 1320–1327 limit the stop scan to the current goal projection, live fenced batches, and a digest-only filename index with zero episode opens. Lines 1371–1388 nevertheless assign the scan responsibility for clearing an existing episode after its fence disappears. Lines 1394–1407 apply the suppression operation only between stamping and transport, while lines 501–504 define due attempts to exclude already submitted episodes. The digest encoding at lines 1483–1485 is one-way and the filename carries no goal or revision.

## AC9-ANSWER-FOLLOWUP-ACTION-001 — high, material=True

CLAIM: AC9-ANSWER-FOLLOWUP-ACTION-001 remains open: the four-row table decides from one failed record, but the shipped commands have chain-level and role-specific preconditions that the episode does not preserve. Follow-up checks the root record, chainClosed, and the newest chain member; the retention handshake may collect those records immediately after the episode becomes durable; and the proposed suffix-stripping rule is not the shipped parentJob ancestry rule. Fresh code-critic and warden dispatches additionally require a --reviews target that the episode neither stores nor renders. The advertised action can therefore already be invalid when journaled or become unusable through the design's own collection rule, beyond the disclosed later chainClosed race.

EVIDENCE: metasystem/scripts/agents/dispatch.sh lines 1687–1716 require the root record, reject a closed chain, and select the newest record before checking status; lines 1744–1752 read the resumable session and role from that selected record. Lines 1222–1227 require --reviews for fresh code-critic and warden dispatches. metasystem/internal/usage/usage.go lines 43–68 derives a chain root by walking parentJob, not by stripping every -rN suffix. metasystem/plans/alert-channel-design.md lines 1252–1297 reads only the failed record, omits reviews and chain state, and renders the incompatible suffix-derived root; lines 1597–1605 release the record pin as soon as the episode is durable.

## Critic-declared gaps (verbatim)

- The task describes semantic critique round 4, but the generated runtime notice identifies this job as round 1. The prior critic chain was rooted at design-critic-a27506cb4736a12e5dcfc31c, while this job is design-critic-0868feb318b6458c6637b500; no evidence proves the required same-chain follow-up or critique-exhaustion transition. The return therefore preserves the harness-observed round number instead of inventing continuity.
- The critique brief at metasystem/plans/alert-channel-r10-critique-brief.md does not declare the failsafe round, threat model, risk appetite, or chain round budget required by the design-critique skill. I did not infer those missing constraints.
- The runtime exposed no session identifier, so sessionId is reported as unobserved and no alternative session is claimed.
- No implementation of the new retention pin, collectors, scans, or suppression operation exists to execute. Live proof was not required; conclusions are grounded in the complete written contract and shipped code surfaces.
