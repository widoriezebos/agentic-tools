Working Mode: design
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal seat-mutual-awareness)
Date: 2026-09-07

# Revision 4 of the seat-mutual-awareness design, under Wido's ruling A

Goal seat-mutual-awareness (tier 3, approved). Its record,
metasystem/plans/goals/seat-mutual-awareness.md, is the contract. The
design on main is revision 3,
metasystem/plans/seat-mutual-awareness-design.md (922 lines with its
two disposition tables). The third review is
metasystem/records/misc/seat-mutual-awareness-critique-r3.md: five
material findings, SMA-C-21 to SMA-C-25. Findings C-21, C-22 and C-23
are about the rollout fence (the enable marker and the human-only
repair verb) and do not converge. On 2026-09-06 Wido ruled option A:
the rollout is operational, the marker and the repair verb are removed.

# The ruling, verbatim in effect

- Every machine pulls, rebuilds and re-arms before any seat writer
  runs; the engine-rearm law already makes a pull a rebuild (the
  design's own trace of up.go:427-456 and the fleet order).
- The validator refuses a spoiled tip (a record an upgraded engine
  cannot read) exactly as revision 3 says; nothing new there.
- A spoiled tip is repaired by an existing human act at the enrolled
  terminal: a hand commit with the pre-commit guard acknowledged, or
  the existing `goal repair --accept-remote --by <human>` where its
  shape fits. No new verb.
- Residual accepted by Wido: an old checkout can only spoil the
  directory by a deliberate hand landing; nobody does that by accident.

# What revision 4 does

Produce metasystem/plans/seat-mutual-awareness-design.md revision 4 as
the deliverable (edit the file in place, bump the title's revision,
keep the two disposition tables and add the round-3 table). Small and
surgical; do not re-open what converged (presence, fleet view, asks
and answers as ledger transactions, deadlines, recovery, retirement,
transport, validator, human surfaces, proof matrix).

1. Remove the enable marker and the repair verb everywhere: section 2
   (the marker record, `seat enable`, `seat-not-enabled`), section 8
   (repair), the fixtures that exist only for them (SMA-F-ENABLE,
   SMA-F-FORGED-MARKER, TestSeatMarkerBindsToTransactionTrailer and
   their kin in section 9), and their reservations in section 10.
   Membership is the presence record itself: a machine is a member
   when its presence record exists and validates; nothing enables the
   directory.
2. Write the operational rollout rule in ONE paragraph in section 8,
   in plain English, as the thing that replaces the fence: the rearm
   law, the validator's refusal of a spoiled tip, the existing human
   repair acts, and the accepted residual, with Wido's ruling and date
   named. Say explicitly what an operator does when a spoiled tip
   appears (the exact existing command or hand act).
3. SMA-C-24: one strict boundary for the deadline second, in the live
   predicate, the validator and the malformed-tree fixture; state it
   once and reference it from the three places.
4. SMA-C-25: reorder the slices so the reader and notification path
   (RunSeatAttention, the seat-questions health role, digest and
   status clause) lands before or with the ask writer; a machine can
   never ask a member that cannot hear. Re-count the box in section 10
   for the new order, smaller than seventeen reservations now that the
   fence work is gone.
5. Dispositions table for round 3 at the end: C-21, C-22, C-23
   "resolved by ruling A (removed)", C-24 and C-25 "accepted" with the
   section that answers each.
6. Self-grade (section 12) updated honestly.

# Constraints

Wall-clock budget: 45 minutes. DESIGN-BEARING reach; the deliverable
is the one file. Cite only paths that exist under metasystem/. No
seat-authored words on any human surface (unchanged rule). Gap rule:
if ruling A cannot be written without a new verb or marker, stop and
say why with the smallest alternative written out.
