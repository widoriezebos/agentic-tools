Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal idle-with-backlog-alarm)
Date: 2026-09-06

# Review brief: fresh independent critique of the final tree (chain idle-escalate-build1, work round idle-escalate-build1-r3)

FINDING IDS: chain-unique, IGF-01, IGF-02, ... never F-n.

Why this review exists: chain completion under DESIGN-BEARING reach
requires a fresh-context critic chain whose reviewed job is the final
work round. The earlier critic chain (idle-escalate-crit1) found three
material items in round 1, folded in the correction round, and its own
follow-up round resolves its register. This chain is the independent
examination of the FINAL tree in a fresh session, and it closes the
build chain.

Round budget: 1 focused round. R-60-m1's rule: a finding is material
only if it changes what gets built and names the artifact it would
change.

Threat model and scope: as in
metasystem/plans/idle-alarm-steward-escalation-code-critique-brief.md,
with the corrections of
metasystem/plans/idle-alarm-steward-escalation-fold3-brief.md
(decisions D7 to D12) as part of the contract, and the reconciliation
of the concurrent landing c1525b90 in the hook script. The orchestrator
persisted the diff and the reviewed tree hash at
metasystem/artifacts/agents/idle-escalate-build1/rounds/3/review.json
(the diff sits beside it); take reviewedTree from that record. Contract:
metasystem/plans/idle-alarm-steward-escalation-build-brief.md (D1 to
D6), the fold brief (D7 to D12), the goal record
metasystem/plans/goals/idle-with-backlog-alarm.md, and the round-1
dispositions metasystem/plans/dispositions/idle-alarm-code-critique-r1b.md.
The orchestrator's runs on the reviewed tree are listed at the end.

# Mandate

1. The bounded idle gate: stops one and two block with the counted
   text; the third on an unchanged world writes only local records and
   releases; a changed world resets; the unreadable-ledger branch
   counts; no branch's block is swallowed; no marker gates the
   escalation.
2. The steward's seatIdle consumption: continues a held claim without
   claiming; claims first when marked, as the recorded seat actor;
   raises the human alarm and launches nothing when the claim refuses;
   the one-active-continuation guard holds.
3. The hook script carries both c1525b90's refusal-carries-verdict
   contract and the bounded-idle rendering, and reads the harness flag
   tolerantly.
4. The pins named in D6 and D12 exist and would fail against the old
   code; no test weakened; nothing outside the declared boundary.

If nothing material remains, say so; that closes the chain and the fix
lands.

# Constraints

Wall-clock budget: 40 minutes. Return per the code-critic schema with
the reviewedTree from the persisted round-3 review record. Gap rule:
stop and report a gap; never fill it silently.

# Orchestrator runs

On the reviewed tree (3ce8eb9a5095d037d8b7476ec62cf6ec58706579), from
this Mac, evidence level ran: `gofmt -l .` printed nothing; `go vet
./...` passed; `bash -n scripts/agents/supervision-hook.sh` passed; `go
test` passed for the packages goal, steward, report, lease and the whole
command package; `bash scripts/agents/supervision-hook-fixtures.sh`
passed every scenario, including the bounded backlog escalation
scenario.
