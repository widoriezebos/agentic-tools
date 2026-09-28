# Dispositions: patience-satellite-4, round 1

Job: design-critic-20260811t175243z-c360 (codex gpt-5.6-sol, xhigh).
15 findings, 15 material, all accepted. The dispositions rewrite the
observable table, the configuration surface, the vocality path, the
booking order, and the exemptions — most of the document — so the
design is regenerated whole per the drift rule, not patched.

| id | disposition |
| --- | --- |
| P4-001 | accepted — return validation is struck from the observable table; it proves protocol, not value, and would reset the very drought the satellite measures. Value = accepted certification only. |
| P4-002 | accepted — the critique-closure claim is dropped entirely; no dispositions join exists to discover mechanically, and with P4-001 none is needed. F Q3.18's gap stays open and stays out of scope. |
| P4-003 | accepted — only certifications with verdict `accepted` count as value; rejected certifications do not. Chain close is not value either: a closed chain simply leaves the evaluation set (the drought ends by ending the spend), its annotation history remaining in the ledger. |
| P4-004 | accepted — patience derivation takes the in-flight TurnConclusion as an explicit input alongside the durable log, and its annotations ride the SAME AppendCycle call as the cycle line (the faulted-path pattern), so booking is atomic and no crash window or false-barren race exists. |
| P4-005 | accepted — vocality is made real: breach lines are appended by the runner to the `## This Turn` section (runner-authored free text, validator-neutral), and the ledger annotation covers the human audit trail. The ask-candidate claim is dropped — candidates belong to the host return. |
| P4-006 | accepted — the fact and the file name were wrong and the layer is bypassable (local reads any key, env outranks it). Resolved by eliminating the conf/local/env surface entirely: floors exist only as sealed mission-contract entries. |
| P4-007 | accepted — same resolution: contract-only floors, sealed and verifiable from the contract body at preflight. No conf fallback means no unsealed behavior change under a signed contract, and pre-feature contracts verify unchanged (no patience entries → none expected). |
| P4-008 | accepted — the faulted-conclusion exemption is removed. Patience evaluates at EVERY cycle booking (ordinary, faulted, healed), and the count is a pure function of terminal rounds versus the highest accepted-certified round, so faults cannot launder rounds; late certification retro-heals the streak. |
| P4-009 | accepted — defined outcomes: an unreadable or malformed round record counts barren (losing sight of a round never counts as value); a record that cannot be joined to a chain forms a single-round chain keyed by its jobId; chains never disappear. |
| P4-010 | accepted — dissolved by P4-001: the derivation no longer validates any return against any schema; it reads only turn-log certifications and job-record round facts, which are immutable history. |
| P4-011 | accepted — one definition: a floor of F tolerates exactly F barren rounds silently; the annotation books when the count strictly exceeds F. |
| P4-012 | accepted — the implicit-wall-bound claim is deleted; rounds-floors bound spend-shaped drought, wall fences bound time, and neither implies the other. |
| P4-013 | accepted — bounded nagging: at most the 20 most-breached chains annotated per booking plus one `Patience overflow` annotation carrying the remainder count, mirroring the landed-returns bound. |
| P4-014 | accepted — the floor for a chain is selected by the EFFECTIVE model of its most recent terminal round (patience is doctrine about who actually worked); requested model plays no part. |
| P4-015 | accepted — the sketch now names both enumeration surfaces (expectedSeal and the ordered emitter) and requires a seal-then-preflight round-trip unit test as part of the deliverable. |

Consequence recorded: docs/patience.md's placeholder sentence locating
floors in metasystem.conf roster keys is corrected to contract-only in
the same commit, with the P4-006/P4-007 evidence as the reason. The
human may veto; the placeholder was explicitly written to be finished
by this loop.
