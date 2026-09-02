# Missionrunner patience design critique — round 2 (Sol)

Chain: revision 2 (spike-folded) -> critic mr-crit2-fresh-1788310542
(codex gpt-5.6-sol, xhigh, fresh context), 2026-09-02. Converged nine
to three, all one corner: unreadable argv can still yield a false
death verdict; one census row is not group custody; scan-then-walk
ordering does not prove pid continuity. The six spike gaps are
restated verbatim by the critic. Box exhausted (the cancelled
pre-landing critique spent an attempt, cause-blind); parked
cold-resumable.

## DC-PAT-R2-001-ARGV-UNKNOWN-FALSE-DOWN — high, material=True

CLAIM: DC-PAT-R2-001, unreadable argument vectors still permit a false death verdict: the DC-PAT-001 fold does not implement its promised completeness rule. A live group member whose start identity is readable but whose argument vector is not readable produces a successful Probe call with ArgvKnown false. The specified observer neither appends that member nor marks the walk incomplete, so a live group can still satisfy walkComplete with zero substantive process identifiers and be classified as down. An implementer following this design would preserve the original false-green path.

EVIDENCE: metasystem/plans/missionrunner-patience-design.md lines 188-210 mark incompleteness only for a Probe error and append only live processes with ArgvKnown true. metasystem/internal/identity/identity_linux.go lines 62-84 return Alive with nil error when ReadArgv fails, while metasystem/internal/identity/identity.go lines 40-45 explicitly state that Ar

## DC-PAT-R2-002-ONE-ROW-IS-NOT-GROUP-CUSTODY — high, material=True

CLAIM: DC-PAT-R2-002, one arbitrary census row is not custody of the process group: narrowing DC-PAT-006 from every current substantive member to at least one materially weakens the outcome rule. If the tagged shell leader is gone and only its untagged sleep child remains, one transient indeterminate census row for that child satisfies the proposed helper and immediately returns abandoned-to-census. Once the child's argument vector is readable, the census classifies it as not ours and emits no row, leaving the still-live group outside census evidence. The design would therefore turn a leak into a pas

EVIDENCE: metasystem/plans/missionrunner-patience-design.md lines 372-397 accept any one current member appearing in either a tagged or indeterminate signalable row. metasystem/internal/census/tagged.go lines 219-225 emit an indeterminate row while identity is unreadable but emit no row after a readable tag mismatch. The design itself establishes at metasyst

## DC-PAT-R2-003-ORDERING-DOES-NOT-PROVE-PID-CONTINUITY — high, material=True

CLAIM: DC-PAT-R2-003, scanning before walking the group does not prove process-identifier continuity: the DC-PAT-006 fold still joins a census process identifier with a later kernel observation without a start identity. A process represented by the scan can exit and its identifier can be reused by a process that is a target-group member before the later walk. Observation order establishes two sightings, not that both sightings name the same process. An implementer would therefore build the identity-unsafe join that DC-PAT-006 required the revision to remove.

EVIDENCE: metasystem/plans/missionrunner-patience-design.md lines 372-381 claim that scan-before-observation means the member was alive continuously through the scan, but no stated invariant supports that continuity. metasystem/internal/census/tagged.go lines 56-61 show that an indeterminate row contains no start identity. The spike's third declared gap says

## Critic-declared gaps (verbatim)

- Spike gap 1: the spike brief named metasystem/internal/proc, which does not exist. It exercised metasystem/internal/missionrunner/proc.go, metasystem/internal/identity, and metasystem/internal/census; any intended different package remains uncovered.
- Spike gap 2: no executable run forced a live group member's argument-vector read to fail. Code reading shows that this boundary is not safely folded because KernelProber reports that state as Alive with no error and ArgvKnown false.
- Spike gap 3: no physical process exit-and-identifier-reuse race was forced. The available structural evidence does not prove the design's continuity claim because indeterminate census rows carry no start identity.
- Spike gap 4: DC-PAT-008 was a specification contradiction and was judged only by reading the old and revised contracts; no executable behavior exists for the pre-revision contradiction.
- Spike gap 5: the natural-exit laundering spike used a three-second fixture against a 30-second stall. It did not execute the former exact 30-second versus 30-second race; the revised 600-second fixture was judged from the specified duration separation.
- This read-only critique did not rerun the spike's seven executable tests. It assessed the landed transcript, the complete revision-2 design, and the current source contracts.
