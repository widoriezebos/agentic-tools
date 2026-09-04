Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal severity-tiered-rigor)
Date: 2026-09-04

# Answer to the gap on chain str-build3

approve. Exactly your proposed contract: `--root-job <job-id>` on
land.sh, commit.sh, `landing observe` and `ObserveParams`, required
for the tier-1 direct-fix class; it must identify a root implementer
record whose goalId matches `--goal` and whose goalTier is 1; gateWidth
is read only from that record, absent means `area`, the only valid
present values are `area` and `full`; when `full`, the receipt's
command must be exactly
`scripts/agents/go-gate.sh --fast && scripts/agents/dispatch-fixtures.sh && scripts/agents/goal-cli-fixtures.sh`;
the receipt is bound to the supplied tree by requiring both the real
index projection and the working-tree projection to equal that tree
before and after the command runs; the binding test
`TestSTR3Tier1ReceiptProof06RefusesMismatchedIndex` changes the real
index after creating the candidate tree, expects receipt creation to
refuse, and proves no receipt was written.

Everything else as in metasystem/plans/severity-tiered-rigor-build3-brief.md.
Build now; the wall-clock budget is 90 minutes from this round's
start; return before it ends even if something is red, naming it.
Any further silent seam: a gap with the proposed contract written out.
