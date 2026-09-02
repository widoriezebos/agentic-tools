# Dispositions, round 3 (the failsafe): two-bars caller-class design critique

Design revision 3 (job implementer-178d269e0852ac7a8e897657-r3). Critic
chain two-bars-cc-crit-3, round 3 (job two-bars-cc-crit-3-r3,
gpt-5.6-sol). Rounds 1 and 2: plans/two-bars-caller-class-dispositions.md
and plans/two-bars-caller-class-dispositions-r2.md. Fold verification:
the critic confirmed all three round-2 findings folded.

## Round 3 — 4 material findings, verdictMaterialCount=4 (2 invariant-grade, 2 mechanical-grain)

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| TBCC-R3-CANCELLATION-PHASE | accepted | Verified: dispatch.sh cancel_job records `phase: cancelling` on a running record before winding down its process group and only later records status `cancelled` (dispatch.sh:2201-2235); internal/dispatch/record.go:492-506 and :732-742 refuse authority-bearing operations on a record whose phase is cancelling even while status is running. The worker rule's step 4 reads status only, so a job an operator has revoked keeps worker authority during the wind-down. Invariant-grade. | Step 4 additionally requires the record's `phase` to be absent or not `cancelling` (the exact vocabulary cited from record.go); the reader gains `phase`; the refusal table gains running-plus-cancelling. |
| TBCC-R3-HOOK-VALUE-REWRITE | accepted | Verified against the design's own text: the post-commit closure (:379-386) counts each owned key once; a commit-msg hook may replace an owned trailer's VALUE (Machine: …+delegate:job to …+human, or a landing verdict) leaving exactly one key, and the count passes. Sections 6-7 make the values authoritative. Invariant-grade. | The post-commit check compares each owned trailer's value byte-exactly with the value the wrapper stamped (the lineage it holds, the provenance and verdict strings it computed); any difference rolls back with the monopoly sentence naming the key; the hook fixture gains a value-rewrite run. |
| TBCC-R3-READABLE-LONG-FORMS | accepted | Verified: `git commit -h` lists `--message <message>` and `--file <file>` as separated long forms; the design's readable set (:345-356) names only the `=` spellings, so a person's ordinary `--message text` would refuse or force invention. Mechanical-grain. | The readable set includes `--message <text>` and `--file <path>` (separated) beside the `=` forms; one leg per form. |
| TBCC-R3-HOOK-TOKEN-MARKER | accepted | Verified against the design's own ordering: the hook-injection rollback is detected after step 5 minted the token and the commit was made, so the marker IS present in that leg; section 7.2's blanket "marker absent on every refusal" (:820-821) contradicts it. Mechanical-grain. | The hook-rollback leg asserts the marker PRESENT (the token was lawfully minted before the commit) and HEAD unchanged; the blanket rule is narrowed to the pre-mint refusals. |

Trajectory 3 -> 3 -> 4 with two invariant-grade findings remaining after
the declared failsafe: the principled exit is NOT available (its second
condition fails). Under the design-critique skill the next focused
follow-up must enumerate every open finding id and opens one fresh
three-round budget on the same critic chain; the lane structure for
that fold (pure design lane, or the joint round Wido granted twice on
this goal's other chains under R-35-m0 and R-38-m0) is his word.
