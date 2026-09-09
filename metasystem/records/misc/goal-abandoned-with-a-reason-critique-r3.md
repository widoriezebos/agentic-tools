# abandoned-goal design critique — round 3 (revision 3, the last prose read)

Chain: revision 3 (landed 81995968, sha256 3cae4b54103cefe3ac3c48ecd8d4fd4e230f41f5eb528bbfbb6cff924d48a505, 1510 lines) -> critic gawr-crit3 (design-critic, codex gpt-5.6-sol, xhigh, read-only). Brief: plans/goal-abandoned-with-a-reason-critique-r3-brief.md (landed ea48d490). Reviewed at checkout 93a210f0. 5 findings, 5 material. Carried verbatim by the coordinator; the critic's sandbox is read-only.

## GAW-19 — critical, material=True

CLAIM: The held invariant checks only the tip commit, but every named push route can introduce more than that one commit. Section 4a incorrectly states that a successful plain push requires origin's tip to equal the pushed commit's parent; Git accepts any fast-forward for which origin's tip is an ancestor. Consequently, a lower commit stamped for abandoned goal G can be rebased above the abandonment, followed by a tip commit stamped for still-held goal H. Held validates H in the tip's first parent and the push lands both commits without ever validating the lower G commit. The parked register-carriage repair explicitly creates and pushes this two-commit shape. This changes the safety mechanism the builder must implement. A real-route fixture named TestHeldChecksEveryCommitIntroducedByPush would settle it by pushing a stack whose lower commit has a dead goal binding and asserting that no remote ref advances.

EVIDENCE: metasystem/plans/goal-abandoned-with-a-reason-design.md:548-605 checks HEAD and its first parent and bases its proof on the false immediate-parent assertion at lines 590-594. metasystem/scripts/agents/commit.sh:583-600 performs an unrestricted branch push without fetching. metasystem/scripts/agents/land.sh:404-410 rebases and pushes the branch rather than proving it is exactly one commit ahead. metasystem/plans/landing-receipt-survives-records-drift-design.md:915-936 describes a failed unpushed commit, a second carriage commit and a recovery that carries both to origin.

## GAW-20 — high, material=True

CLAIM: The proof-bearing goal edit --carried submode has no deterministic update contract. Rule 4 says Carried binds the newest abandon or edit line, which implies replacement, but it also rejects any carried line whose successor differs from the current field, which makes a second edit from S1 to S2 invalidate the older S1 line. The shared edit command also accepts intent, risk, tier, next-step, label and blocker deltas, while the design never says whether combining any of them with carried is refused or mutates an archived record. Current Edit operates on Live only, so this is not resolved by existing behavior. An implementer must guess at both history semantics and the allowed mutation surface. A named fixture TestCarriedEditDefinesReplacementAndRejectsMixedDeltas should settle a first edit, a second edit to another live successor, repeated identical edits, and every carried-plus-ordinary-edit combination.

EVIDENCE: metasystem/plans/goal-abandoned-with-a-reason-design.md:799-812 specifies the new option and newest-line binding. Lines 882-887 simultaneously reject a carried line that differs from the current field. metasystem/cmd/metasystem/goalsync_mutations.go:1392-1465 constructs the existing multi-field edit delta, and metasystem/internal/goal/verbs.go:1447-1498 accepts only Live records. No section 13 fixture exercises a second carried edit or a mixed edit invocation.

## GAW-21 — high, material=True

CLAIM: The recertified-route shell leg cannot reach the held check it claims to prove. The recipe mints a recertification and then has the peer abandon the goal before invoking land.sh. That changes origin away from the recertification's frozen target, so land.sh parks at its initial check_recertification_target before staging, committing or held. The stated full-width-chain seed also lacks the required referenced closed critic job and critic return. A real critic process is not required—the existing recertification test synthesizes the complete records—but the design's recipe does not. Allowing the build to narrow the leg would discard proof of one of the three promised land.sh push routes. The existing abandonment-refuses-every-push-route recertified leg must instead be made reachable and must discriminate that land.sh invokes held before its single push.

EVIDENCE: metasystem/plans/goal-abandoned-with-a-reason-design.md:1276-1285 orders peer abandonment before land.sh and expects a later held refusal. metasystem/scripts/agents/land.sh:462-469 compares both local and fetched origin tips with targetCommit, and lines 497-505 run that check before staging. metasystem/internal/validate/recertification.go:333-429 requires independentCritiqueJobRef, a closed code-critic chain, a completed terminal member, a critic return with reviewedTree, and matching review artifacts. metasystem/cmd/metasystem/landing_verbs_test.go:131-180 and 584-627 demonstrate synthetic critic records sufficient to mint the proof.

## GAW-22 — high, material=True

CLAIM: History rule 11 makes the lost-checkout recovery impossible to reopen. The carried recovery appends an edit line while the record is abandoned, but ReopenAbandoned preserves every History line and changes State to queued. On the resulting live record, rule 11 rejects that historical edit line because it permits edit carried only when the record's current State is abandoned. ValidateCommit therefore rejects the reopen transaction. The two section 13 fixtures cannot both pass under the written rules. TestReopenFromAbandonedIsBoundToTheClaimantCheckoutAndCarriedRecovers must parse and validate the final reopened record while a separate forged-live case still proves that arbitrary live edit carried is rejected.

EVIDENCE: metasystem/plans/goal-abandoned-with-a-reason-design.md:806-824 requires the carried edit to remain in append-only History after reopen. Lines 914-921 allow edit carried only on a record whose State is abandoned. Lines 1189-1192 require both the current-state restriction and a later successful reopen. Reopen publications run ValidateCommit, so this is a transaction refusal rather than a cosmetic parse problem.

## GAW-23 — high, material=True

CLAIM: The commit.sh --push shell assertion is unreachable as written and therefore does not prove the wrapper's held call. For a non-human actor, when the local parent already carries the abandonment, the mandatory observation refuses before commit creation. When the abandonment exists only at origin, both observation and held read the stale claimed local parent; held passes and the raw push is rejected as non-fast-forward. commit.sh always creates a new commit and has no mode that merely pushes a previously standing commit. The route fixture must separately discriminate execution of held and the actual remote-race outcome instead of requiring an impossible held goal-item-not-held refusal.

EVIDENCE: metasystem/plans/goal-abandoned-with-a-reason-design.md:1292-1294 requires commit.sh --push to make a commit whose parent carries the abandonment and then refuse at held. metasystem/scripts/agents/commit.sh:448-478 performs and enforces observation before git commit, while lines 583-600 push without fetching or rebasing. Thus the same local abandoned parent cannot pass observation and later fail held for a non-human actor.

## Gaps the critic named

- No implementation of revision 3 exists, and the sandbox is read-only, so the proposed fixtures could not be executed. Discrimination judgments are grounded in the existing production control flow and the fixture recipes.
- The runtime did not expose a harness session identifier or prove context isolation; the supplied runtime notice classifies independence as advisory.
- During the read, the shared checkout acquired an unrelated modification at metasystem/records/narrator-digest.log. It was not inspected, changed or used as evidence, and the reviewed Git commit remained unchanged.

## What the critic verified

- (ran) git show -s --format=%H 81995968 && shasum -a 256 metasystem/plans/goal-abandoned-with-a-reason-design.md && wc -l metasystem/plans/goal-aba => The reviewed design commit resolves to 81995968092b69a769993041f341a06f276e23fa. The design is 1510 lines and its SHA-256 digest is 3cae4b54103cefe3ac3c48ecd8d4fd4e230f41f5eb528bbfbb6cff924d48a505, ex
- (ran) git rev-parse HEAD && git merge-base --is-ancestor 81995968 HEAD && git diff --name-status 81995968..HEAD => The synchronized checkout is at 93a210f02d5a02f7b45cd0fdc4e5b38d978f909e, which descends from the design commit. Since 81995968 only the critique brief was added and the goal record changed; no review
- (ran) git log -400 81995968 --format='%H%x1f%B%x1e' | awk 'BEGIN { RS="\036"; FS="\037" } { body=$2; if (body ~ /(^|\n)Machine:[[:space:]]*[^+\n]+ => At 81995968 the corresponding census is 60 non-human Machine commits, all 60 with Goal-Item and none without. This is outcome evidence for one 400-commit history window, not evidence that every live p
- (ran) git grep -nE 'git .*push|git -C .* push|git push' 81995968 -- 'metasystem/scripts/agents/*.sh' | rg -v 'fixtures|test|#' => Production landing transport is performed at metasystem/scripts/agents/commit.sh:588 and through metasystem/scripts/agents/land.sh:409, with metasystem/scripts/agents/sync-transport.sh:35 only mirrori
- (read) nl -ba metasystem/plans/goal-abandoned-with-a-reason-design.md | sed -n '357,713p'; nl -ba metasystem/scripts/agents/commit.sh | sed -n '430 => A null or absent chain-root goalId is a legitimate goal-free root. A non-human landing from it must either name and hold a current goal or observe Root.Free; a goal-bound root must name exactly its re
- (read) nl -ba metasystem/internal/goal/txn.go | sed -n '35,75p;343,373p'; nl -ba metasystem/scripts/agents/land.sh | sed -n '400,410p;495,596p' => The configured goal endpoint defaults to origin and refs/heads/main. The proposed land.sh calls pass origin and refs/heads/<checked-out-branch>. The --skip-transport option skips only the mirror after
- (read) nl -ba metasystem/internal/goalrevision/lock.go | sed -n '21,167p'; nl -ba metasystem/scripts/agents/dispatch.sh | sed -n '1497,1546p;1808,1 => The goal-revision lock validates only the goal identifier and a positive revision; it does not require a claim, so an unclaimed live goal can be locked at its record revision as specified. Dispatch ho
- (read) nl -ba metasystem/cmd/metasystem/goalsync_mutations.go | sed -n '1388,1465p'; nl -ba metasystem/internal/goal/verbs.go | sed -n '1430,1613p' => A proof-requiring option inside goal edit is structurally sound: existing risk-lowering edits already obtain proof in the command and revalidate it inside the goal package. A separate verb is not requ
- (ran) git show cab73164:metasystem/internal/goal/root.go | nl -ba | sed -n '90,125p'; git show cab73164:metasystem/internal/goal/file.go | nl -ba  => The engine at cab73164 accepts the proposed root History line: ParseRoot delegates every History line to ParseHistoryLine, which accepts an arbitrary verb word and the known actor and reason keys. The
- (ran) git grep -n '\.Done' 81995968 -- 'metasystem/**/*.go' ':!*_test.go' => The synchronized TreeGoals.Done readers occur in metasystem/cmd/metasystem/goal.go and metasystem/cmd/metasystem/goalsync_mutations.go; metasystem/internal/channel/report.go; metasystem/internal/couns
- (ran) git diff --unified=0 7ae27b7e 81995968 -- metasystem/plans/goal-abandoned-with-a-reason-design.md | rg '^\+.*`Test|^-.*`Test' => The twelve revision-3 new or rewritten fixtures were audited. TestAbandonLocksEveryGoalInTheSetAndRefusesAnyRecordChange fails today because Abandon is absent and should pass after the build, but it c
- (read) nl -ba metasystem/scripts/agents/land.sh | sed -n '430,487p;495,568p'; git show 81995968:metasystem/internal/validate/recertification.go | n => The normal and retry shell legs can drive land.sh itself and reach the proposed held step after rebasing over the peer abandonment. The recertified recipe cannot: an abandonment published after the re
- (read) nl -ba metasystem/scripts/agents/commit.sh | sed -n '30,60p;430,485p;580,602p'; nl -ba metasystem/plans/goal-abandoned-with-a-reason-design. => The proposed commit.sh --push fixture cannot reach its stated held refusal. If local HEAD already carries the abandonment, the non-human observation refuses before a commit exists. If only origin carr
- (read) nl -ba metasystem/plans/landing-receipt-survives-records-drift-design.md | sed -n '824,936p;938,967p' => The parked landing-receipt design makes the stack defect concrete: its contended-register recovery can leave one unpushed commit, create a carriage commit above it, advance the stack and push both com
- (inferred) nl -ba metasystem/plans/goal-abandoned-with-a-reason-design.md | sed -n '1378,1454p' => Fold status: GAW-08 is closed by section 4a mechanism 1's mandatory goal binding. GAW-09 is closed by section 4a's Machine uniqueness and unsoftened trailer check. GAW-10 is closed by section 4a's all

## Coordinator disposition (m1b, 2026-09-09) — after fold-read cycle 2

Trajectory of the three reads: 7, 11, 5 material findings. Falling, not
empty. The anchors of all five were checked before disposing: the page does
assert at lines 590-594 that a plain push lands exactly one commit above
origin's tip, and git accepts any fast-forward; commit.sh's --push has no
fetch; land.sh's check_recertification_target runs before staging; rule 11
at 914-921 keys off the record's current state; commit.sh observes before it
commits. All five accepted.

- GAW-19 (critical): held validates only HEAD and its first parent, but a
  fast-forward push carries every commit between origin's tip and HEAD. A
  commit stamped for an abandoned goal can ride below a valid tip commit.
  The closure must hold for every commit the push introduces: held walks the
  range from the fetched remote tip to HEAD and refuses on the first commit
  that fails, and every push route proves it pushes exactly that range.
- GAW-20 and GAW-22 (high): the lost-checkout recovery the designer chose
  on its own, goal edit --carried, contradicts rules 4 and 11 and cannot be
  reopened: after reopen the record is queued and rule 11 rejects the
  historical carried line. The shape is wrong: a proof-bearing submode of a
  proof-less multi-field verb. The coordinator's decision: a dedicated verb,
  and validation rules that judge each History line against the record's
  state at that line, not the current one.
- GAW-21 and GAW-23 (high): two of the twelve fixtures cannot reach what
  they claim (the recertified leg parks at the target check before held is
  reached; the commit.sh --push leg cannot both pass observation and refuse
  at held on the same parent). They must be rewritten to discriminate.

This was the last prose read. What happens next is the land-or-fold call
that the loop rule reserves for Wido before any third fold-read cycle; the
coordinator's recommendation is recorded in the goal record and the report.
