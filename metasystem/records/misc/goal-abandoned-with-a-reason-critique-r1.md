# abandoned-goal design critique — round 1 (revision 1)

Chain: revision 1 (landed 03f94dfc, sha256 1d6245c80dbae6d1ec8dbc95ae4c52dfed7143ad0c4e8fddd6021ad1682cdbb5) -> critic gawr-crit1 (design-critic, codex gpt-5.6-sol, xhigh, read-only; the harness observed gpt-5.6-sol and the return claimed nothing different). Reviewed at checkout 7f1c1c53. 7 findings, 7 material. The coordinator carried the return here verbatim because the critic's sandbox is read-only.

## GAW-01 — medium, material=True

CLAIM: The third-map census is incomplete, so an implementer following it will leave split readers that report an abandoned goal as nonexistent. runGoalSplit in metasystem/cmd/metasystem/goalsync_mutations.go and the split parent precheck in metasystem/internal/goal/split.go directly inspect Done after failing to find Live. Neither is given an Abandoned or Archived path by the table. This also disproves the design's claim that the shape fails closed: forgotten readers degrade to absence rather than refusing specifically because the goal is archived.

EVIDENCE: The complete non-test direct-reader search found the two split prechecks in addition to the correctly covered sites. The Done-first Archived helper itself is not defective on a validated tree; duplicate identifiers across Done and Abandoned are rejected before consumers receive the projection.

## GAW-02 — critical, material=True

CLAIM: An already-running foreign job is not inert after abandonment and can still land. Landing observation checks the stale checkout before land.sh fetches and rebases, does not bind the job's GoalRevision, and is not repeated after rebase. A clean rebase can therefore place that job's commit above the abandonment and push it. The same preflight-versus-dispatch interval is not specified as an atomic transition. Meanwhile the job drops out of live budget accounting, can complete and return normally, and eventually becomes terminal-record garbage-collection material. This invalidates the transition's safety premise and also makes the stated reason for skipping stop-batch verification on reopen unsound.

EVIDENCE: Dispatch admission in metasystem/internal/dispatch requires Live, claimed state and an exact revision for new work. In contrast, metasystem/internal/landing/observe.go heldGoal checks state and actor in the current base tree. metasystem/scripts/agents/land.sh commits through commit.sh before fetching and rebasing, then pushes without repeating landing observation. No landing goal-revision comparison was found.

## GAW-03 — high, material=True

CLAIM: Reopen is described as a human operation but its guard is only a nonempty caller-supplied human name. In a lineage-bearing non-brain checkout, an agent can supply --by without enrolled-terminal proof, remove the Abandoned record, and clear the frozen StopFence and StopCapability. Retaining the stop identifier in history preserves provenance but not authority. Because old execution is not actually made inert, skipping VerifyStopBatchComplete lacks an alternative quiescence proof.

EVIDENCE: The proposed transition requires only r.Actor.Human != empty. metasystem/cmd/metasystem/sync.go accepts --by into Actor.Human without terminal enrollment proof when lineage is already available. Section 6 explicitly clears both active stop fields, while the existing verifier is invoked only through resume of a Live claimed goal.

## GAW-04 — high, material=True

CLAIM: The warning-only rollout permits a known fleet-wide compatibility failure. The design states that every older binary will fail all ledger reads after the first Abandoned record, but the transaction merely prints a warning and continues without a machine-checkable fleet precondition or a recorded human fleet-wide compatibility ruling. A missed procedural upgrade can therefore wedge every older seat at once.

EVIDENCE: Section 10 acknowledges that the binary cannot know whether every relevant checkout has been upgraded, specifies no shim or feature flag, and nevertheless makes the warning nonbinding. Engine builds can be inspected per checkout, but the repository has no central authoritative binary registry that would make this continuation safe.

## GAW-05 — medium, material=True

CLAIM: The waiver grammar does not enforce the brief's requirement for a recorded human reason. The exhaustive ordered refusal list rejects an unknown or ineligible dependent but has no refusal for --waive dependent= with an empty reason, duplicate waiver entries with conflicting reasons, or equivalent blank input. Implementers can therefore differ on whether a blank reason removes the dependency, and one choice violates the settled contract.

EVIDENCE: Section 4 declares --waive dependent=reason and later writes that text into history, but refusals 1 through 8 contain no reason validation. Once accepted, the edge is removed, so validation rule 5 cannot recover or detect the missing rationale.

## GAW-06 — medium, material=True

CLAIM: The dependency-reader canary does not exercise the proposed tree shape and can pass with an incomplete implementation. It places a StateAbandoned record in the Done map and checks only that depState is not StateDone. An implementation that returns the stored state from Done but never reads the new Abandoned map passes, and an empty or missing result also satisfies the assertion.

EVIDENCE: Section 13's TestDependencyReaderDoesNotTreatAbandonedAsDone constructs TreeGoals{Done: ... StateAbandoned} even though section 3 places such records in TreeGoals.Abandoned. Its inequality assertion does not require StateAbandoned.

## GAW-07 — medium, material=True

CLAIM: The in-process specimen cannot reach the invariant it claims to test because it calls Abandon with --carried successor without first creating that live successor. After the verb exists, refusal 6 will reject the request before the fence, quota, dependency, and reopen assertions run. Its untouched-tree compile failure is an acceptable fail-before signal, but its implemented-tree setup fails for the wrong reason.

EVIDENCE: The Go specimen seeds only the stopped goal and one dependent, then names successor. Section 4 requires every carried target to name a Live successor. The adjacent shell specimen explicitly opens S before using --carried S, confirming the missing Go setup.

## Gaps the critic named

- The requested Markdown register was not created because the design-critic role forbids edits and the workspace was read-only. This JSON is the complete register payload for the orchestrator to persist at metasystem/records/misc/goal-abandoned-with-a-reason-critique-r1.md.
- No fixture bed or runtime mutation was run, as required by the brief; transition and landing conclusions are grounded in static control-flow reads rather than live proof.
- The runtime did not expose a session identifier or prove independent context isolation; sessionId is therefore reported as unobserved and the critique remains advisory.

## What the critic verified

- (ran) git rev-parse HEAD && git status --short --branch && shasum -a 256 metasystem/plans/goal-abandoned-with-a-reason-design.md => The synchronized checkout was clean at commit 7f1c1c537fd0dcf62f902fee6f8a51370014bfc4. The reviewed design's SHA-256 digest was 1d6245c80dbae6d1ec8dbc95ae4c52dfed7143ad0c4e8fddd6021ad1682cdbb5, matching the brief.
- (read) Read metasystem/plans/goal-abandoned-with-a-reason-design.md, metasystem/plans/goal-abandoned-with-a-reason-design-brief.md, and metasystem/ => Revision 1 specifies a third Abandoned archive map, a human abandonment transaction, frozen stop authority, reopening, pruning, validation, rollout, and fail-before canaries.
- (ran) rg -n --glob '*.go' --glob '!**/*_test.go' '\.(Done|DonePaths)\b' metasystem => The twenty-two-row reader table does not cover metasystem/cmd/metasystem/goalsync_mutations.go line 1130, where runGoalSplit treats only Done as archived, or clearly cover metasystem/internal/goal/split.go line 229, where the split parent precheck does the same. Of the additional sites named by the brief: metasystem/internal/counselor/sources.go line 542 is covered and correctly classified by row 11; metasystem/cmd/metasystem/goal.go line 316 is legacy-ledger handling correctly left unchanged by row 21; line 361 is covered and correctly classified by row 8; line 450 is covered and correctly classified by row 9; and metasystem/internal/goal/migrate.go line 366 is legacy input correctly left unchanged by row 21. Archived's Done-first order is safe for validated projections because validation rejects duplicate archive identifiers, but omitted direct readers silently treat Abandoned as absent, so the third-map shape does not fail closed.
- (read) Read metasystem/internal/dispatch/stop.go, metasystem/internal/dispatch/admission.go, metasystem/internal/dispatch/servinggoal.go, metasyste => Fresh dispatch and reservation admission require a Live, claimed goal at the exact goal revision. Landing's heldGoal check also requires a claimed goal, but it reads the candidate's current base tree and checks only goal identifier, state, and actor. land.sh commits and runs observation before fetching and rebasing onto origin; it does not rerun heldGoal after the rebase, and landing contains no goal-revision check. Consequently, a foreign checkout that began while the goal was claimed can commit against stale state, rebase over the abandonment, and push.
- (read) Read metasystem/internal/goal/budget.go and metasystem/internal/evidence/gc.go around live budget projection and archived-job retention. => Once the goal leaves Live, the old job is no longer charged through the live goal's budget projection. It can still finish and return normally. Under the proposed Archived lookup, its terminal record eventually ceases to count as spending and can become garbage-collection eligible after the existing terminal retention conditions.
- (read) Read every non-validator reader found by rg -n 'StopFence|StopCapability|VerifyStopBatchComplete|FindBreachStops' metasystem/internal metasy => Record-level stop readers first select Tree.Live or require StateClaimed and a current claim before acting; none treats an arbitrary archived record containing a fence as a claimed stopped goal. VerifyStopBatchComplete is reached through Resume only after those checks. Stop-batch commands and watch inventory inspect batches independently by stop identifier, so an existing batch can remain visible or be explicitly advanced, but the frozen fields on an Abandoned record do not themselves trigger action.
- (read) Read metasystem/cmd/metasystem/sync.go, metasystem/internal/goal/sync.go, and the reopen and resume implementations. => The proposed reopen guard is only Actor.Human being nonempty. syncReqClassified accepts a caller-supplied --by as Actor.Human when lineage is already present and does not require enrolled-terminal proof in that path. The design explicitly clears StopFence and StopCapability, retains the stop identifier only in history, and skips stop-batch verification.
- (read) Read the dependency transition, prune, and validation sections against metasystem/internal/goal/validate.go and metasystem/internal/goal/pru => After --waive, each affected dependent loses every edge to the abandoned set and is touched with history, so validation rule 5 no longer sees that edge. After --carried, each dependent's edges are repointed to the live successor; the abandoned record merely records Carried and is not itself repointed. The proposed prune seeding covers every abandoned record's BlockedBy edges and recursively walks retained Done prerequisites. Updating stateOf to return StateAbandoned preserves the existing prohibition on a Done goal depending on an abandoned goal.
- (read) Read sections 10, 13, and 15 of metasystem/plans/goal-abandoned-with-a-reason-design.md against the command and test surfaces they name. => The rollout knowingly permits the first incompatible write after only a warning. Parser- and symbol-boundary failures are acceptable fail-before signals for most canaries because later assertions name the intended reader behavior. Two proposed tests do not prove their intended behavior after implementation: the dependency canary uses an Abandoned record in Done and asserts only not-Done, while the Go specimen never creates the required carried successor. Keeping Done separate otherwise preserves Done counts and frontier meaning, and abandonment appends rather than rewrites history.

## Coordinator disposition (m1b, 2026-09-09) — fold-read cycle 1

All seven accepted; every one changes what gets built. The design loop's
stop criterion is not met by revision 1. Fold to revision 2 on the design
lane.

- GAW-02 (critical): the page's safety premise, that dispatch admission and
  landing both requiring a claimed goal make a foreign running job inert,
  is false on the landing side. heldGoal in internal/landing/observe.go
  checks state and actor in the candidate's base tree at observation time,
  binds no goal revision, and land.sh commits, fetches, rebases and pushes
  without observing again; a job that started before the abandonment can
  land above it. Revision 2 must make the landing bind the goal's revision
  and re-observe after the transport rebase, or refuse the push when the
  goal left the claimed state, and must state the preflight-to-dispatch
  window as an atomic transition. The reopen argument that rested on
  inertness falls with it.
- GAW-03 (high): reopen's guard is only a non-empty --by; in the
  lineage-bearing path an agent can supply it without enrolled-terminal
  proof and clear the frozen fence. Reopen must take the same enrolled
  proof as set-priority (proveGoalHumanAuthority), and a reopen from a
  record with frozen stop authority must either verify the stop batch as
  resume does or keep the fence frozen until a resume-like act clears it;
  provenance in history is not authority.
- GAW-04 (high): a warning-only rollout can wedge every older seat at once.
  The first abandon must refuse without a machine-checkable precondition or
  a recorded fleet-wide ruling; the design lane decides which, and what the
  precondition can read (the machine-wide supervision registry names each
  enrolled seat's engine build on this machine; other machines are not
  visible, so a recorded ruling may be the only honest gate).
- GAW-01 (medium): the census missed runGoalSplit
  (cmd/metasystem/goalsync_mutations.go:1130) and the split parent precheck
  (internal/goal/split.go:229), both of which treat only Done as archived;
  the fails-closed claim is withdrawn as stated and restated as what the
  shape actually guarantees (a forgotten reader degrades to absence, never
  to completion).
- GAW-05 (medium): --waive needs a refusal for a blank or conflicting
  reason before the edge is removed; once removed, rule 5 cannot detect it.
- GAW-06 (medium): the dependency-reader canary must place the record in
  the Abandoned map and assert StateAbandoned, not merely not-done.
- GAW-07 (medium): the Go specimen must open its --carried successor first,
  as the shell specimen does.

Next: fold brief to the Fable lane for revision 2 in place; then a fresh read
of revision 2. At cycle 2 the coordinator says aloud that this loop has no
natural exit; before cycle 3 the land-or-fold call is Wido's.
