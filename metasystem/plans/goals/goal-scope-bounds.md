# goal-scope-bounds

- State: claimed
- Intent: Goals get mechanical size bounds AND a split mechanism (Wido 2026-08-30): a big goal is welcome at intake, but before slicing it evolves into an arc of small goals - the scope norm triggers, the split verb is the remedy; concurrency is designed at two axes, scope (this goal: planning parallelism via arcs) and load (machine-concurrency-governor: execution parallelism via slots)
- Origin: human
- Next step: SLICE 1 COMPLETE, SLICE 2 TWICE GAP-STOPPED, ONE ATTEMPT LEFT - RAISED TO WIDO. Design landed 4783e7f3 (two Sol rounds, 14->7 material, failsafe declared at loop start; SS10 residue binding with finding-over-text precedence; SS8 carries three decisions for Wido incl. the arc-uniformity framing Sol twice judged too narrow). Slice 2 (Sol) gap-stopped correctly on four undesigned mechanisms; Fable addendum SS11 landed eb8526ed; the rebuild gap-stopped AGAIN on five new gaps, all at implementer-private seams (dispatch ClaimLaunch has no goal-op ULID source; recovery completes dead-owner entries with absent postconditions so slice-start replay would falsely mark sliced; the human path for main-origin splits cannot construct an accepted token under the tier=main rule; MAIN holder classification can lack a claim epoch; over-norm steal recovery would replay a human actor recovery deliberately strips). PATTERN: the design lane cannot see these seams from the design; each ping-pong costs an attempt and a day. Attempts 7/8 used. OPTIONS FOR WIDO: (a) raise budget +3 attempts/+400min and authorize ONE joint round where the Sol implementer designs the five boundary mechanisms in-line and builds them, Fable critiquing the result after (R-25 lane exception, his word only); (b) raise and run a third Fable addendum with the five gaps verbatim (the shape that just failed twice); (c) park slice 2-3 and take the landed design as the deliverable for now. m2 recommends (a): every gap is a statement about machinery only the implementing lane touches, and the after-the-fact Fable critique preserves the two-lane check.
- OpenedAt: 2026-08-30T16:08:07Z
- Revision: 9
- Budget: elapsedLimit=6d attemptLimit=11 reservedJobMinutesLimit=1360 activeJobLimit=2
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-31T22:43:45Z revision=9
- StopCapability: generation=9 revision=9 machine=m2 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-30T16:08:07Z ZBBC230KPCRGNH7WXR6CE9FNXM-m1-bf243850 open actor=m1+coordinator targets=goal-scope-bounds
- 2026-08-30T16:09:26Z B8VEFZ7EP7TPFG2V8MCTWJWJ0E-m1-bf243850 edit actor=m1+coordinator targets=goal-scope-bounds
- 2026-08-30T16:11:19Z 3WMXJV4RP1NPDZDDBFGFH8NB88-m1-bf243850 edit actor=m1+coordinator targets=goal-scope-bounds
- 2026-08-30T16:52:44Z 40PZFCFMKGDQB3F0FW8JQA06CF-m2-bc1be9cb set-budget actor=human:wido targets=goal-scope-bounds
- 2026-08-30T21:36:13Z NQAJMVGEZAKP0XWNJJX6BE9G21-m2-bc1be9cb claim actor=m2+mac-coordinator targets=goal-scope-bounds
- 2026-08-30T23:03:39Z PGMVCKRKW4QNGR18AG82GWBKJ2-m2-bc1be9cb edit actor=m2+mac-coordinator targets=goal-scope-bounds
- 2026-08-31T06:17:27Z HW2GNQRGRDKWJSMMVM5GSYGHKZ-m2-bc1be9cb release actor=m2+mac-coordinator targets=goal-scope-bounds
- 2026-08-31T10:13:44Z 2HMBN0P9ZES9Z70BJAMK7283PJ-m2-bc1be9cb set-budget actor=human:wido targets=goal-scope-bounds
- 2026-08-31T22:43:45Z QK4P636CB51139XJXF9DK80AXX-m2-bc1be9cb claim actor=m2+mac-coordinator targets=goal-scope-bounds
Integrity: sha256=3fb1509603fccc3ca71a9aa1273fb9ea0162aef753303b776cc35ddf07ddc4ba
