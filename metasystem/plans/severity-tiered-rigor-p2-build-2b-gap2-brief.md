Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-03

# Goal

Answer the three gaps your round 2 stopped on and BUILD slice 2b per
metasystem/plans/severity-tiered-rigor-p2-build-brief-2b.md with the
contracts of metasystem/plans/severity-tiered-rigor-p2-build-2b-gap-brief.md
and the three below. Two rounds have now ended with nothing built; this
round builds. The wall clock of the 2b brief restarts with this round.

# Gap: who writes `out-of-scope`

`validate critique-closed` (metasystem/internal/validate/critiqueclosed.go)
stays a join and gains two optional flags, `--repo <root> --root-job
<root>`. Without them it validates only, as today. With them, after
the join passes, it writes `resolution=out-of-scope` on the register
entries the dispositions table marks out of scope, in ONE write under
the register lock of finding_register.go (the same lock and write path
`CritiqueRegisterClose` uses; expose one function
`CritiqueRegisterResolveOutOfScope(repoRoot, rootJob string, findingIDs
[]string) error` in metasystem/internal/dispatch and call it from the
validator), refusing before any write when any named entry is severe
or unproven, naming it. Persistence order: join, then the register
write; nothing else is written. Fixture: STR3-GAP-OOS-WRITE, bounded
entry written once and idempotent on rerun; severe entry refused with
no write.

# Gap: discharge selection

`DischargeReviewObligation --finding <id> --chain <root> --by <actor>
--test "<citation>"`: both selectors required; exactly one obligation
must match, else refuse ("no such obligation" or "ambiguous
obligation" naming the matches). Fixture: STR3-GAP-DISCHARGE-SELECT,
two obligations sharing a finding id on two chains; the one named by
chain is discharged, the other stays open; a missing chain flag is a
usage refusal.

# Gap: the accepted-risk register line

Field mapping, fixed: `title` = the register entry's title, or when
absent the first line of the finding's claim trimmed to 120 bytes;
`acceptanceStatus` = `accepted`; `acceptanceReason` = the `--why` text
of `goal accept-risk`; `class` = the entry's rigor class; each
`specimenFacts` element is one evidence line of the finding with one
citation `{kind: "job-record", target: "artifacts/agents/jobs/<root>.json",
detail: "<finding id>"}`; `reviewLinks` holds one link `{kind: "goal",
target: "plans/goals/<goal>.md", detail: "opid=<opid>"}`; `id`
`ar-<root>-<findingId>`, `kind` `accepted-risk`, `schemaVersion` 1,
`recordedAt` RFC3339 UTC. The misclassification line of the 2b brief
maps the same way with `title` = "tier raised <from> to <to>",
`acceptanceStatus` = `recorded`, `acceptanceReason` = the evidence
reference, and the strict reader admits `recorded` for that kind only.
Fixture: STR3-GAP-REGISTER-LINE, one accepted-risk line round-trips
through the strict reader with every field populated.

# The rule for what remains

The gap rule stops you for a contract that would change law: a new
authority, a new refusal, a new landing bar, a schema the fleet reads.
It does not stop you for a mechanical choice of the grain above (a
field mapping, a selector, a flag name, a lock to reuse, an order of
two writes). For those, choose what the tree already does nearest to
the seam, record the choice in your return under `decisions` (one line
each: the choice, the alternative, the reason), and continue. A choice
recorded in the return is not silent. Stop only for a law-changing gap
that neither design nor the two gap briefs answer, and then stop with
the tree gate-green and the built items listed.

# Everything else

Unchanged from the 2b brief and the first gap brief: items 1 to 7 in
that order, every named fixture plus the three above, the gate, the
diffBoundary, the `goalReviewRoundLimit` seam, no commit.
