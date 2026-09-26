# Intent workflows: Fable design dispositions

Design: designs/intent-workflows.md. Root Codex authored and adjudicates; Fable
5.1 is the independent critic requested by Wido. This uses the existing explicit
machinery bypass, not a fabricated registered review closure.

Round 1: launch 20260926t081353-83a8177af2, provider session
d55cdac2-eac6-4f23-874c-e5e0bbd137d8, actual exit 0, model claude-fable-5-1,
28 observed tool calls/turns. Full report: intent-workflows-fable-review-r1.md.
Reviewed SHA256: 4382ffaea494dad8ae5c39eb76198bd922573a89946f017ac623facd47462bd9.
Draft retained at 98a632446d910b84308b983594322b370fe47346.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| IW-C1 | accepted | Build requires a claim at intent_work.go:301-310 and intent_worktree.go:123. The design omitted first-use acquisition. | Build acquires a ready unclaimed goal through the existing claim owner, never takeover/approval; claim/release/enroll stay public. |
| IW-C2 | accepted | Named records have no relevant-stage selection contract; a timestamp rule would guess after multiple work items. | Explicit per-command eligible sets, all-current-read unchanged result and named ambiguity. |
| IW-C3 | accepted | A terminal failed round still has its own number; forever joining its request loses the ability to retry. Automatically releasing identity would make a lost response authorize more paid work. | Refine Fable's remedy: stable after-N request, replay failure honestly, public --after FAILED-N deliberately creates exactly one new attempt. Identical old requests rejoin before current-version inference. |
| IW-C4 | accepted | CritiqueClosed permits accepted material decisions; that is not proof the fix exists or that a read is clean. | Review sends accepted unresolved material findings to revise before closing. Only actual owner-recorded resolutions/exceptions may complete. Preserve exact subject and disposition binding. |
| IW-C5 | accepted | Old ready is exactly goal.LandReady (intent_planning.go:1075), not landing proof preparation. | Use land --queue-only for that exact cheap act: no tests, read collection or push. Preserve ordinary land handover order. |
| IW-C6 | accepted | Whole close has distinct fence/holder/mirror/record-writer refusals; inheriting raw text violates the public recovery contract. | Map each class to public status/review/claim/repair or precise external dependency, preserving authority and actual incomplete state. Failed examination retry is explicitly bounded and idempotent. |

Notes N1-N7 were considered. N1 retained actors, N3 channel-answer wording, N4
validator vocabulary and N6 per-command help coverage are explicit in the fold.
N2 descriptor fields and N5 config ownership were already implementation choices
in the design. N7 is an estimate, not a new gate. No capabilities are silently
removed or waived.

Root also made two source-grounded contract details explicit: legacy fixture and
internal identity flags remain callable but not advertised; disposition files
carry the exact subject/return binding the command emits, so a repeated action
cannot judge a different current review. These implement the user's shielding
and retry requirements rather than adding a new authority or workflow system.

Round 2 will be the final design critique round on this thread and will check
this entire fold, especially revision replay, closure decisions and recovery.
No implementation has started at this entry.

## Final design round

Round 2: launch 20260926t082700-93f7b19165, same provider session, actual exit 0,
model claude-fable-5-1. Full report: intent-workflows-fable-review-r2.md.
Reviewed SHA256: 4cd91701097ea040dec208d87e21095dce6a10cd7712dcc38049ac22d917815c,
retained at b1c0f2ea8ed5c60a8b1aecc2766d5668ec3f3920. The report's earlier HEAD
reflects concurrent root record commits; the hashed design is authoritative and
incoming main changed no reviewed implementation owner.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| IW-C7 | accepted | The reviewed attempt and failed correction attempt are different identities; newest failed-round outputs may be empty. Refine the proposed word collected: unresolved findings must remain usable before clean closure. | Keep the newest authentic examination's immutable findings/decisions across failed corrections; after-N names the correction base. Reject a superseded review. Named fixture TestRevisionRetainsReviewedFindingsAfterFailure. |
| IW-C8 | accepted | Existing wait goal supports landing/human-act events and verb filtering; retaining only work/question waits would remove a public capability. | Advertise wait G --for landing|human-act --since TIP, preserve aliases, provide queued-landing hint. Named fixture TestIntentGoalEventWait. |

Notes N8-N10 are wording corrections. N11-N14 are named fixtures under IW-2,
IW-3 and IW-4. Administration choices not examined by Fable receive a source-
grounded exact-input appendix before that implementation stage, with authority
fixtures; none is asserted certified by this report.

Root judgment: the trajectory fell from six material findings to two bounded
binding/grammar corrections, with no remaining owner or workflow-shape dispute.
Close at round 2 on the two named fixture obligations under fixtures-as-arbiter.
Both are folded, neither waived. Independent code critique must check their actual
implementation and fixtures before release. User-authorized implementation begins;
the seven proof obligations remain MISSING until real evidence exists.
