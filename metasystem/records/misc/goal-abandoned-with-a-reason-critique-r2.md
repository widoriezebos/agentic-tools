# abandoned-goal design critique — round 2 (revision 2)

Chain: revision 2 (landed 7ae27b7e, sha256 55b979c7b06a8e08afadf0b066af85edc38140eaf318911d518ed7b4dc7e1d83, 1014 lines) -> critic gawr-crit2 (design-critic, codex gpt-5.6-sol, xhigh, read-only). Brief: plans/goal-abandoned-with-a-reason-critique-r2-brief.md (landed 2c81e1d6). Reviewed at checkout 2c81e1d6. 11 findings, 11 material. The coordinator carried the return here verbatim because the critic's sandbox is read-only.

## GAW-08 — critical, material=True

CLAIM: The claim-revision landing closure is optional because revision 2 never binds a goal-bearing chain record to the caller's --goal declaration. An agent can land an ordinary closed implementation chain while omitting --goal: no held-goal observation runs, no Goal-Item or Goal-Revision trailer is stamped, and landing held exits successfully on the unbound commit. A goal-bound straggler can also carry a new register record through register carriage without --goal. Thus the design's statement that a straggler cannot land above abandonment is false for existing command forms.

EVIDENCE: metasystem/scripts/agents/land.sh makes --goal optional. metasystem/internal/landing/observe.go lines 159-167 read the chain record but do not validate goalId; lines 276-279 call heldGoal only when ObserveParams.Goal is nonempty. Lines 647-676 repeat that conditional for register carriage, and lines 831-836 accept a new metasystem/records/ file without a held goal. Revision 2 section 4a adds goalRevision handling but never requires or derives --goal from a goal-bound chain. Its held command explicitly passes commits with no Goal-Item.

## GAW-09 — high, material=True

CLAIM: The human-commit exemption is selected from an underspecified and non-unique Machine trailer. An agent-controlled commit message may already contain Machine: forged+human; commit.sh appends the legitimate non-human Machine trailer and retains both. Revision 2 says only that held 'reads the Machine trailer', so implementations may select the first, last, or joined value. Selecting the forged value converts a stale agent landing refusal into the human warning and exit-zero path.

EVIDENCE: metasystem/scripts/agents/commit.sh lines 159-235 scan only for caller-typed Goal-Item, while lines 552-558 append Machine and do not prove its uniqueness. The executed git interpret-trailers specimen retained both Machine lines. Revision 2 section 4a requires exact uniqueness for Goal-Revision but states no duplicate or missing-Machine rule. The ordinary Machine value comes from METASYSTEM_OWNER_LINEAGE, while agent-versus-human commit enforcement is separately determined from the numeric lease epoch; the two are not authenticated as one fact at held time.

## GAW-10 — high, material=True

CLAIM: The transition is not atomic against dispatch on the same checkout when the human projected an unclaimed goal. Abandon takes no goal-revision lock for that goal, and its transaction permits it to have become claimed. A concurrent claim can land, dispatch can bind and lock that new revision, and Abandon can then publish while dispatch reserves and launches under its lock. The job is admitted against a claim that the transition removes, contrary to section 4a's same-checkout atomicity claim.

EVIDENCE: Revision 2 lines 405-418 acquire locks only for goals the projection saw claimed. Refusal 6 at lines 246-250 compares only those projected-claimed goals and explicitly tolerates an unclaimed tip. metasystem/scripts/agents/dispatch.sh lines 1497-1508 resolve the binding before lines 1544-1546 acquire the lock, then lines 1812-1824 hold it through process identity publication. Because Abandon never acquired that new revision's lock, the intervals overlap. The fixture tests only a projected claimed revision moving, not an unclaimed goal becoming claimed.

## GAW-11 — high, material=True

CLAIM: A fenced abandoned goal is not generally 'reopenable by a human' as the governing contract promises. Reopen requires both enrolled-terminal proof for the exact claimant checkout root and a stop-batch file under that root. A human on another machine cannot use that machine's proof or batch location; if the claimant checkout or machine is lost, even a completed but inaccessible batch cannot authorize reopening. Section 6 records only the never-completing-batch cost and leaves permanent loss of the checkout unstated.

EVIDENCE: metasystem/internal/humanauthority/authority.go lines 141-146 bind Proof.ValidFor to one absolute root; lines 508-530 and 608 onward read that root's enrollment and local process ancestry. metasystem/internal/goal/stop.go lines 82-86 locate the batch below the same root and lines 249-262 verify it. Revision 2 lines 547-555 therefore require the command to run from the original claimant checkout under its enrolled terminal. The rejection of keeping a fence on queued remains supported by metasystem/internal/goal/verbs.go line 269, but that does not make the selected route portable or recoverable.

## GAW-12 — high, material=True

CLAIM: The stated integration with the parked landing-advance chain preserves a documented raw-push repair route that bypasses the new held check. After landing advance repairs an unpushed commit, that design instructs the seat to invoke git push directly. A straggler can therefore rebase above an abandonment and follow the parked chain's own repair instructions without any parent-state recheck.

EVIDENCE: Revision 2 section 4a lines 465-475 says held runs after either rebase implementation and treats the integration as an adjacent land.sh edit. However metasystem/plans/landing-receipt-survives-records-drift-design.md lines 503-515 prescribe manual fetch, landing advance, raw git push, and transport sync for a refused unpushed commit; landing held is absent. The parked design also shares several files beyond land.sh, so revision 2's merge-surface premise is factually incomplete.

## GAW-13 — high, material=True

CLAIM: The landing check and the abandonment transaction can read different canonical remotes. Goal publication supports an arbitrary configured goal.sync-remote and goal.sync-branch, while land.sh and held are specified only against origin and the checked-out branch. If transport or another remote is the configured goal endpoint, abandonment can be present there but absent from origin; the stale straggler then passes held and pushes origin, and --skip-transport reports success. The design does not restrict the supported endpoint or bind held to it.

EVIDENCE: metasystem/internal/goal/txn.go lines 46-60 resolve configurable remote and branch values, and lines 365-367 publish to that remote. metasystem/scripts/agents/land.sh lines 400-410 hardcode origin. metasystem/scripts/agents/sync-transport.sh lines 31-35 safely mirror origin and cannot itself carry a commit origin rejected, but it cannot reconcile a different canonical goal remote; the optional --skip-transport path omits it entirely.

## GAW-14 — medium, material=True

CLAIM: The renewed Done-reader census still misses the reconcile edit-row archive decision. When a hand edit was captured against a live goal but the fetched tip now contains that goal in Abandoned, this site will report the generic 'not live' conflict rather than the design's archived-record conflict. An implementer following the table can therefore leave the wrong outcome mapping.

EVIDENCE: metasystem/internal/goal/reconcilepub.go lines 368-379 consult t.Done directly to distinguish an archive race from absence. Section 5 row 18 names only the open-row lookup at line 239, and row 14 names the render lookup at line 150. Neither names the edit-row lookup at line 371. The complete non-test .Done and .DonePaths search found it.

## GAW-15 — high, material=True

CLAIM: The at-rest validation rules do not prove that the Abandoned field and its referenced History event describe the same act. A tree can validate while Abandoned names a different human, reason, displacement, stop identifier, or successor than the event it points to, or while that event does not target the goal. Reopen then deletes the field and preserves only the contradictory event. The design also says stopId and carried appear only on abandon lines but specifies no validation for that restriction.

EVIDENCE: Revision 2 section 8 rule 4 checks only the human prefix, timestamp shape, revision range, History verb, matching timestamp, and nonblank reason. By contrast, metasystem/internal/goal/file.go lines 577-632 show the existing approval binding pattern comparing actor, timestamp, operation identifier, verb, and authority facts. Section 2 says the abandon event retains reason, stopId, and carried after reopen, yet section 13 has no mismatch or wrong-verb fixture.

## GAW-16 — high, material=True

CLAIM: The fixtures do not prove that land.sh invokes held on any push route. The only abandonment-race shell scenario manually runs landing held after a manual rebase, then runs an unrelated existing successful land.sh scenario. It can pass if held_check is omitted from the normal route, retry loop, or recertified branch, and the required landing gate does not run this fixture bed at all.

EVIDENCE: Revision 2 section 13 lines 908-920 performs git fetch, git rebase, and metasystem landing held directly; its later land.sh assertion proves only that an ordinary --goal landing still succeeds. Lines 922-923 exclude metasystem/scripts/agents/land-fixtures.sh from the required gate. Unit tests exercise Held and observation separately, so none discriminates the three shell call sites promised at lines 381-386.

## GAW-17 — medium, material=True

CLAIM: The ordered refusal contract contradicts its waiver fixture. Fleet-floor refusal 0 must read root History and the local registry before waiver refusal 3, while refusal 3 and its test require malformed waiver input to be rejected before any tree is read, using an endpoint whose root does not exist. Both cannot be implemented as specified, so the builder must choose between the declared order and the fixture.

EVIDENCE: Revision 2 section 4 lines 210-239 labels the list ordered and places fleet-floor processing at 0 and waiver grammar at 3. Section 13's TestAbandonWaiveRefusesBlankAndDuplicateReasons says every case reads no tree and supplies a nonexistent root. Fleet-floor check 0 cannot determine the newest root History line without trying to read that root.

## GAW-18 — high, material=True

CLAIM: The rollout does not define which registry slot classes must be checked against the engine floor. 'Slots, the live ones' could mean only LiveVerified, all unclosed armed claims, or every consumed slot. In particular, UnknownLiveness may be a still-running old engine, yet the required refusal and fixture do not say whether it is inspected. Different implementations therefore provide different safety guarantees for the human fleet-floor ruling.

EVIDENCE: metasystem/internal/registry/slots.go lines 17-35 defines six classes and lines 84-93 treats five as consuming capacity. Revision 2 section 10 lines 713-724 names Slots and 'the live ones' without a class predicate; it separately says every armed checkout visible on the machine must not contradict the floor. The sole fixture covers an ordinary armed checkout below the floor and does not exercise unknown liveness, dead owner, open reservation, or sweepable-closed records.

## Gaps the critic named

- No runtime fixture bed or mutation was run; the read-only critic role permitted static evidence only, so the race and shell-integration findings are control-flow proofs rather than executed scenarios.
- The parked landing-advance implementation is not present at the reviewed commit. Its interaction was assessed against metasystem/plans/landing-receipt-survives-records-drift-design.md and current landing code, so the eventual composed implementation remains unexecuted.
- The runtime exposed no session identifier and did not prove independent context isolation; the session is reported as unobserved and the critique remains advisory.

## What the critic verified

- (ran) shasum -a 256 metasystem/plans/goal-abandoned-with-a-reason-design.md && wc -l metasystem/plans/goal-abandoned-with-a-reason-design.md && gi => The design is exactly 1014 lines and its SHA-256 is 55b979c7b06a8e08afadf0b066af85edc38140eaf318911d518ed7b4dc7e1d83, matching the review brief. The synchronized checkout is 1809e732f7e4b2e5e80594ffb7
- (ran) git diff --name-status 7ae27b7e..HEAD -- metasystem/internal/landing/observe.go metasystem/scripts/agents/land.sh metasystem/scripts/agents/ => No named implementation file changed between the design commit and the reviewed commit, so the cited control flow remains the code under review.
- (read) Read metasystem/plans/goal-abandoned-with-a-reason-design.md lines 1 through 1014, metasystem/records/misc/goal-abandoned-with-a-reason-crit => Revision 2 specifies the new archive map and state, abandonment transaction, landing revision binding, dispatch locking, proven reopen, pruning, validation, recovery, rollout, readers, specimens, and
- (read) nl -ba metasystem/internal/landing/observe.go; nl -ba metasystem/scripts/agents/land.sh; nl -ba metasystem/scripts/agents/commit.sh => A chain record is read without validating its goalId against the optional --goal argument. Every current held-goal check is conditional on a nonempty caller-supplied goal. Goal-Item is stamped only wi
- (ran) printf a message containing 'Machine: forged+human' | git interpret-trailers --trailer 'Machine: m1b+main-lineage' => Git retained both Machine trailers. The current message scanner rejects caller-typed Goal-Item only, and the current postcondition proves Goal-Item uniqueness only; revision 2 adds the analogous check
- (read) nl -ba metasystem/scripts/agents/dispatch.sh; nl -ba metasystem/internal/dispatch/admission.go; nl -ba metasystem/internal/goalrevision/lock => Dispatch resolves the accepted goal binding before acquiring the goal-revision lock, then holds that lock through reservation, spawn, and process-identity publication. Revision 2 would acquire abandon
- (read) nl -ba metasystem/internal/humanauthority/authority.go; nl -ba metasystem/internal/goal/stop.go; nl -ba metasystem/internal/goal/verbs.go => Proof.ValidFor binds authority to one exact checkout root, whose enrollment file and live process ancestry are local. VerifyStopBatchComplete reads the batch from that same root and binds all frozen s
- (read) git show cab73164:metasystem/internal/goal/root.go; git show cab73164:metasystem/internal/goal/file.go => The cab73164 root parser sends every History entry to ParseHistoryLine. That parser accepts the third word as an unrestricted verb, recognizes actor, removes reason as a free tail before checking keys
- (ran) rg -n --glob '*.go' --glob '!**/*_test.go' '\.(Done|DonePaths)\b' metasystem => The renewed census found the table's named readers plus a still-omitted archive decision at metasystem/internal/goal/reconcilepub.go line 371. That edit-row race distinguishes an already archived reco
- (read) Read metasystem/internal/goal/file.go validation and History parsing, metasystem/internal/goal/validate.go, and sections 2, 8, and 13 of met => The proposed at-rest rule binds Abandoned only to the event's verb and timestamp. It does not bind the human actor, reason, displacement, stop identifier, carried successor, or target identifier to th
- (read) Read section 4a of metasystem/plans/goal-abandoned-with-a-reason-design.md and sections 3b and 3e plus the implementation map of metasystem/ => The parked landing-advance design explicitly tells a seat repairing an unpushed commit to run fetch, landing advance, raw git push, and transport sync by hand, omitting landing held. It also changes s
- (read) nl -ba metasystem/internal/goal/txn.go; nl -ba metasystem/scripts/agents/sync-transport.sh; nl -ba metasystem/scripts/agents/land.sh => Goal publication honors configurable goal.sync-remote and goal.sync-branch, defaulting to origin. Landing always checks and pushes origin. The transport script safely mirrors the exact origin tracking
- (read) nl -ba metasystem/internal/registry/slots.go; nl -ba metasystem/internal/registry/reduce.go; read section 10 of metasystem/plans/goal-abando => Registry Slots returns LiveVerified, OpenReservation, UnknownLiveness, DeadOwner, SweepableClosed, and Free. Revision 2 says to inspect 'the live ones' but never maps that phrase to slot classes, alth
- (inferred) Compare GAW-01 through GAW-07 with revision 2's revision record and the implementation evidence above. => GAW-01 — narrowed by sections 3 and 5: the two split sites were added and the fails-closed claim withdrawn, but metasystem/internal/goal/reconcilepub.go line 371 remains outside the census. GAW-02 — n

## Coordinator disposition (m1b, 2026-09-09) — fold-read cycle 2

The coordinator checked the code anchors of GAW-08, -09, -10, -13 and -16
before disposing: land.sh takes --goal as an option (line 71); observe.go
runs heldGoal only when a goal was named (276-279) and accepts a new
records/ file without one (831-836); commit.sh appends the Machine trailer
without checking for one already typed (552-558); the goal endpoint's
remote and branch are git config (txn.go 46-60) while land.sh pushes
origin; and the page's required gate omits land-fixtures.sh. All hold.

All eleven accepted. Every one changes what gets built, and GAW-08 voids
the closure revision 2 was written for: a landing that names no goal runs
no held check, and nothing on the page makes a goal-bound chain name its
goal. Eight of the eleven are in material revision 2 added (sections 4a,
6, 10, 13), which is the shape of a growing page, not a converging one.

This fold is CYCLE 2. Said aloud, as the rule requires: this loop has no
natural exit. Each fold invalidates the read that caused it and a fresh
reader of a thousand-line page will find something. The coordinator folds
once more because these findings are structural (a voluntary closure, a
lock that guards only what the projection saw, a fixture that cannot see
the shell routes it claims), then treats revision 3's read as the last
prose read: under D81 the standing residue after it (lock orderings,
refusal orders, trailer rules, slot predicates) becomes named fixtures for
the build, and the land-or-fold call before any cycle 3 is Wido's.

Directions the fold brief carries, one per finding:
- GAW-08: a goal-bound chain root binds the landing's goal; a landing that
  names none or another refuses; a non-human actor's direct fix names a
  goal or refuses. A human commit stays sovereign.
- GAW-09: commit.sh refuses a caller-typed Machine trailer the way it
  already scans for Goal-Item; held refuses a missing or duplicate Machine.
- GAW-10: refusal 6 compares the record's revision, not the claim's, for
  every goal in the set; a goal that became claimed under the human's read
  refuses, and the lock is taken for the revision the projection read.
- GAW-11: stated as the limitation it is, with the recovery named: a
  fenced abandoned goal whose claimant checkout is gone is not reopened; a
  human opens a successor and the abandoned record carries it.
- GAW-12: every push route runs held, including the parked chain's repair
  route; the coordinator has noted that requirement on the parked goal.
- GAW-13: held resolves the goal endpoint and refuses when the landing's
  remote and branch are not it; the supported configuration is stated.
- GAW-14: reconcilepub.go's edit-row archive decision joins the table.
- GAW-15: rule 4 binds every Abandoned field to its History event, with a
  mismatch fixture, after the approval-binding pattern in file.go.
- GAW-16: the race fixture goes through land.sh on all three push routes
  and land-fixtures.sh is named as part of the chain's proof.
- GAW-17: input-only refusals precede tree-reading ones; renumber.
- GAW-18: a slot-class predicate for the floor check, decided against the
  six classes in registry/slots.go.

Correction (m1b, later the same day): the note on the parked goal
landing-receipt-survives-records-drift was refused; editing a parked goal is
a human act. The GAW-12 requirement (every route that pushes a landing
commit, including that chain's repair route, runs the held check at the
rebased parent) is carried here and in the fold brief, and goes onto that
goal when it is unparked.
