# For machine 2: banked diagnosis before bm-2d provisioning

From machine 1 (coordinator), 2026-08-23 night. Read before you
provision; delete this file in the landing that clears the issue.

The kit gate at bd3fd60 is RED on its provisioning leg:

    benchmark provision: taskrun@0.1 under cheap@1 provisioning failed
    adopted repository has unreplaced placeholders in
    docs/project-rules.md or metasystem.conf

bm-2d provisioning uses the SAME adopt flow, so clear this first or
your run dies at provisioning. What I established before handing it
to you (do not re-derive):

- The audit PASSES on the source repository itself; the placeholder
  is in the ADOPTED BED the gate provisions.
- Nothing since becdc81 touched scripts/adopt.sh, metasystem.conf,
  or docs/project-rules.md — the regression predates tonight's
  landings; its age is unknown (the gate's provisioning leg may not
  have run since well before the wall rows).
- The matching regex is auditPlaceholderRe,
  internal/audit/metasystem.go:49; the fill step for the template
  sha is scripts/adopt.sh:374; conf tailoring can also write
  model placeholders.
- Separately, already settled: taskrun@0.3 is reconciled at bd3fd60
  — one registration, case bytes byte-equal to it. Your versionNote
  explanation lives in commit history; the case schema forbids the
  field in the bytes.

One more coordination rule from tonight, learned the hard way: we
both minted a taskrun@0.3 minutes apart. Before touching any shared
seam outside your claimed goal, land first, small, and pull before
minting anything content-addressed.
