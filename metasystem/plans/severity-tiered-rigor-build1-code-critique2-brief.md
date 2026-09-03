Working Mode: implement
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal severity-tiered-rigor)
Date: 2026-09-04

# Review brief: re-review of part one after its one correction (chain str-build1c, round 3)

Round budget: 1 focused round; this is the re-review the ceremony
allows after the one correction. A second correction goes to Wido
with evidence (R-73-m3). R-60-m1's rule: material only if it changes
what gets built and names the artifact.

Threat model and scope: as in
metasystem/plans/severity-tiered-rigor-build1-code-critique-brief.md;
the computed diff of the implementer job under review (58 files
against main) is the authority. Your round-1 findings and their
dispositions: metasystem/records/misc/severity-tiered-rigor-build1-critique-cc1.md;
the correction brief: metasystem/plans/severity-tiered-rigor-build1-fix-brief.md;
its gap answer (legacy approval records):
metasystem/plans/severity-tiered-rigor-build1-fix-gap-brief.md.

# Mandate

1. F-1 closed: every `goal open` caller in the module passes a tier
   and the touched fixtures run.
2. F-2 closed: `classify-sweep --confirm` installs the TierLaw marker
   on an empty listing as its own root change; the fixture proves a
   hand-written tierless goal is refused afterwards.
3. F-4, F-7, F-8 folded as the correction brief says; the legacy
   NormApproval rule of the gap answer has its two tests.
4. ADJUDICATE the four fixture failures the implementer reports as
   pre-existing: dispatch-fixtures.sh (an obsolete `config tailor`
   invocation reached after the corrected approval), channel-fixtures.sh
   (a fixed 15-second poll context expiring while posting a receipt),
   adopt-fixtures.sh (receipt.sh writing memory/receipts.log under the
   vendored prefix), supervision-fixtures.sh (the stop-hook-monitor
   payload). For each: caused by this change (material, name the fix)
   or pre-existing on main (not material; say what on main causes it,
   so it becomes its own backlog item). The implementer's module tests
   and the goal CLI fixtures are green.

If nothing material remains, say so; that closes the chain and part
one lands.

# Constraints

Wall-clock budget: 40 minutes. Return per the code-critic schema, with
the reviewedTree from validate conformance --stage review for job
str-build1c-r3. Do not run path-class-fixtures.sh (ripgrep is absent).

# Gap Rule

stop and report a gap; never fill it silently.
