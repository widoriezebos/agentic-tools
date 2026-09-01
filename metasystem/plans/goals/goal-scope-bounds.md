# goal-scope-bounds

- State: queued
- Intent: Goals get mechanical size bounds AND a split mechanism (Wido 2026-08-30): a big goal is welcome at intake, but before slicing it evolves into an arc of small goals - the scope norm triggers, the split verb is the remedy; concurrency is designed at two axes, scope (this goal: planning parallelism via arcs) and load (machine-concurrency-governor: execution parallelism via slots)
- Origin: human
- Next step: SLICE 1 COMPLETE, SLICE 2 TWICE GAP-STOPPED, ONE ATTEMPT LEFT - RAISED TO WIDO. Design landed 4783e7f3 (two Sol rounds, 14->7 material, failsafe declared at loop start; SS10 residue binding with finding-over-text precedence; SS8 carries three decisions for Wido incl. the arc-uniformity framing Sol twice judged too narrow). Slice 2 (Sol) gap-stopped correctly on four undesigned mechanisms; Fable addendum SS11 landed eb8526ed; the rebuild gap-stopped AGAIN on five new gaps, all at implementer-private seams (dispatch ClaimLaunch has no goal-op ULID source; recovery completes dead-owner entries with absent postconditions so slice-start replay would falsely mark sliced; the human path for main-origin splits cannot construct an accepted token under the tier=main rule; MAIN holder classification can lack a claim epoch; over-norm steal recovery would replay a human actor recovery deliberately strips). PATTERN: the design lane cannot see these seams from the design; each ping-pong costs an attempt and a day. Attempts 7/8 used. OPTIONS FOR WIDO: (a) raise budget +3 attempts/+400min and authorize ONE joint round where the Sol implementer designs the five boundary mechanisms in-line and builds them, Fable critiquing the result after (R-25 lane exception, his word only); (b) raise and run a third Fable addendum with the five gaps verbatim (the shape that just failed twice); (c) park slice 2-3 and take the landed design as the deliverable for now. m2 recommends (a): every gap is a statement about machinery only the implementing lane touches, and the after-the-fact Fable critique preserves the two-lane check. SLICE 2 LANDED 84f847aa (2026-09-01, the joint round under Wido's lane exception: norm + split verb + decomposition registry + five in-line boundary mechanisms mapped in design SS12; Fable safeguard critique 6 material -> zero across three rounds). REMAINING: slice 3, claim-side dependency gating - members claimable within recorded dependency edges, unmet dependency refuses with the dependency named; note the whole-arc frontier suppression removal was already pulled forward lawfully (GJ-R1-008). Budget exhausted at 11/11 attempts - slice 3 needs Wido's raise (~+3 attempts/+400 job-minutes).
- OpenedAt: 2026-08-30T16:08:07Z
- Revision: 11
- Budget: elapsedLimit=6d attemptLimit=11 reservedJobMinutesLimit=1360 activeJobLimit=2

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
- 2026-09-01T02:42:54Z 5JNV9Y12GBJ45AQQ54WQ1FZN8G-m2-bc1be9cb edit actor=m2+mac-coordinator targets=goal-scope-bounds
- 2026-09-01T02:43:16Z G8C6S158CEV2T4Z3DG9T1TTNNJ-m2-bc1be9cb release actor=m2+mac-coordinator targets=goal-scope-bounds
Integrity: sha256=f1b95e8bcef968bac2fbde28c6d824591be92bbb729aa3a87d1a3cbbe943f0e9
