# Cleanup design critique, round 2 of 2

- Design: `metasystem/plans/designs/verb-cleanup.md`, id 01M3FG9P2H020EKJDZHEFNMRB0, status accepted (`project design-of --goal verbs-match-intent` at P, run via `go run ./cmd/metasystem`). Source read at 1f0f51e48 plus the uncommitted design delta (67 insertions, 8 deletions).
- Critic: Fable 5.1, independent, no subagents, read-only. 22 tool calls. Scope per brief: the exact capability mapping, `done job`, `revise run`/`review run`/`wait run`, authority and lost capability, and whether the retained private owners really carry what the deleted aliases did. Floor and administration parts were accepted in round 1 and not re-read.
- Materiality: would the change ship a defect, violate its brief, or damage what certifies it? Everything below is WORKS/SAFE without it.
- Verdict: **0 material findings. The loop exits; the mapped portion is ready to implement.** Every row of the mapping table names an owner that exists and is already the owner of the retained public form. `done job J` is a truthful completion, keeps every close check, and has no path to conclude, land or approve.

## Mapping verification (all read at 1f0f51e48)

| Row | Claim | Evidence | Holds |
| --- | --- | --- | --- |
| ready [G] → land G --queue-only | same landReady | intent_planning.go:1194 (ready) and :1204 (land --queue-only) both call `inv.landReady(id)` | yes; the no-arg inference is a shorthand, not an operation |
| decide → accept-risk | already calls runIntentDecide | runIntentAcceptRisk :1275 → runIntentDecide :1287. This also closes the "parity not traced" cell in root's caller-facts table | yes |
| resolve (both forms) → review G --finding F | same runIntentResolve, four advanced fields | runIntentReviewDischarge (intent_selection.go:588-605) refuses --work/--dispositions/--model, infers --review, calls runIntentResolve; review declares --test, --implementation-chain, --artifact, --result, --critic (intent_delivery.go:86-91) | yes; round-1's unverified row is now verified |
| recover [G] [--session S] → repair goals + repair waits | both owners remain | runIntentRecover :1851 recovers the whole journal then `waitContinuations`; G is only a target label. repair usage lists goals and waits [--session S] (:404-410) | yes; G loses nothing |
| red own/close → incidents claim/close | same runIntentRed, same options | :2001 calls runIntentRed; flag sets identical: goal, branch, to, reason/why, human-act flags (:458-473 vs :476-492) | yes |
| fleet [--refresh] → status --machines | same runIntentFleet | intent_process.go:823; --refresh declared on status (:113) and read at :824 | yes |
| doctor → check | same runIntentDoctor | intent_operations.go:406 | yes |
| ui verbs → start/stop/status/restart ui | serve/tools family retained | restart ui exists (:91); private Go consumers use only `ui serve` (lifecycle/launch.go:21) and `ui tools` (partner/runtime.go:103, walkthrough/partner.go:195, smoke.go:70) | yes, see clarification 1 |
| fold review R → revise job R | same foldReview | runIntentRevise (intent_selection.go:611-620) already routes `job` to foldReview | yes |
| fold unit RUN → revise run RUN | same Continue/FollowUp owner | runIntentFoldUnit (intent_work.go:1027-1035) calls `runner.Continue(launch.UnitRequest{Resume: run, FollowUp: brief})`; the runner holds the immutable-input, repetition and round-limit checks | yes |
| close J → done job J [--dispositions] [--evidence R] | same complete closeChain | closeChain (intent_delivery.go:1214-1292); see next section | yes |

## `done job J`: truthful, checks preserved, cannot conclude or land

- **Truthful intent.** closeChain writes exactly the chain's records: lock, evidence reconcile, register close, close check, closed stamp (dispatch.sh close_chain 2983-3044: `acquire_chain_lock`, `__review-reference-reconcile --root-job --evidence-job`, `mirror_record`, `job critique-register-close`, `__critique-close`, stamp). Nothing in either layer calls a goal mutation, landing or approval. "Completes the job's records" is what it does.
- **Original checks preserved.** Authority: `recordWriterPreflight` (person, lease holder or the chain's own job, :1230-1243), and `lease_run_held` in the shell owner. Critic chains: --dispositions required, `validate.CritiqueClosedWithRegister` (:1244-1256). Non-critic chains: --dispositions refused (:1257), non-terminal newest round reported in progress (:1260). Evidence: `validIntentJobID` (:1227) then `valid_id` in the shell; forwarded, never trusted.
- **Cannot conclude a goal by accident.** runIntentDone (intent_goals.go:803-831) needs a non-empty --reason and reaches `trySyncMutationWithCompletion("done")`. If the job branch is taken first on `len(args)==2 && args[0]=="job"` (the pattern runIntentRevise uses at intent_selection.go:611) and refuses the goal-only options, no argv reaches the goal mutation. Note for the builder: goal-only options are --reason, --by, --lineage and the advanced --id/--goal alias (intent.go:80); the zero-arg goal form keeps its claimed-goal inference (:811).
- **Separation.** `done G --reason TEXT` stays one positional plus --reason; `done job J` is two positionals. Evidence and dispositions refused on the goal form, as the brief requires.

## `revise run`, `review run`, `wait run`

- Owners exist today under the word `unit`: `review unit RUN` (intent_delivery.go:592, --model allowed at :556) and `wait unit RUN` (intent_work.go:1103, 1127-1128, waitUnit :1312). Neither has a usage string today: review's usage list has no unit form and intent_review_help.go does not mention unit; wait's usage lists job/resume/question/proof/file/goal only. "Expose" therefore means their first public catalogue entry, which the complete-catalogue test must include. Keep --model on `review run`.
- Returned words to migrate: intent_work.go:970 (`verb unit RUN` continuations), :1035, :1128, intent_delivery.go:1738; target-word lists at intent_delivery.go:75 (review goal G) and intent_work.go:1103 (wait goal G) gain `run`. `build --resume RUN` (intent_work.go:177, 352) stays as the design says.
- No manufactured IDs: every run form is reached from a continuation the owner printed.

## Authority and lost capability, and the private raw entry

- No shell script, hook, skill, instruction page or Go self-exec calls any of the ten public spellings (regex census over scripts, skills, optional-skills, internal, cmd, docs, wow.md, AGENTS.md and .claude at 1f0f51e48, non-test). The only hits are output strings, all inside owners the design already names: internal/ui/lifecycle/serve.go:48, result.go:68, internal/ui/act/act.go:45, intent_process.go:1338 (`next: metasystem ui start`), intent_delivery.go:971 (`close: metasystem close J --dispositions`), :1046, :1206, :1222 (`next: metasystem close PARENT`), intent_work.go:1030, internal/dispatch/read_admission.go:252-253 (`next: dispatch.sh close --job … --reconcile-evidence …`). That list is complete for the census and bounds the "changed message producers" test set.
- So the private owners preserve what the aliases did (every retained public form calls the same function), and nothing preserves the alias spellings, which is the stated intent. No authority condition moves: every human-act flag set is identical on the retained form.

## Clarifications, none material

1. **ui refusal level.** After the descriptor goes, `metasystem ui start` still falls to the `ui` family (main.go:813) unless the family's start/stop/status/restart verbs go too. Private Go consumers use only `ui serve` and `ui tools`, so deleting those four family verbs is safe and makes "all removed aliases refused" true for ui at execution. One sentence stating that choice keeps a code round from being spent on a red or a silently surviving spelling.
2. **Two completions for a critic chain.** `review job C --dispositions` and `done job C --dispositions` both end in closeChain. Not a defect; help should say review job is the route for reviews and done job the route for implementer chains and evidence.
3. **recover [G]** may read as `recover [--session S]` in the table; G was only a label, so nothing is lost.

## Disposition of round-1 findings

- VC-D1: resolved. All thirteen rows verified above, including the resolve form left unverified in round 1.
- VC-D2: amended (design lines 102-104 name both beds, retain the assertions, run both sections).
- VC-D3: amended (VC-1 owners now include internal/ui/lifecycle, internal/ui/act and read_admission.go; the "test the changed producers" choice is acceptable given the bounded producer list above).

## Limitations

- Static reads only; the CLI was not run and no test was executed. runIntentResolve's test-versus-fixture exclusivity and the body of `__review-reference-reconcile` were not read beyond their call sites.
- The caller census is a regex upper bound over the checked-in tree, not a parse; `review unit`/`wait unit` had no live callers outside returned argv.
- Floor and administration text was not re-read this round.
