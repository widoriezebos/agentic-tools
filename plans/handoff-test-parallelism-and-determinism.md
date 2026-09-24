# Test parallelism and Git-free tests: final candidate handoff

Updated 24 September 2026, after the performance correction and Opus implementation roster change. Continue **finish-test-repairs-and-integrate**. Do not claim a new goal or stop unfinished work because of metadata. User authorizes commits, pushes, Main integration, tmux, and machinery workarounds. Main integration and final matching validation remain outstanding. The earlier full runs were halted at the user's request; do not restart them. The user subsequently authorized the three performance changes below.

## Authority and boundaries

Root Astra coordinates Claude Opus 5.5 (`claude-opus-5-5`) implementation and Sol6 reviews, as the user most recently requested. This supersedes the earlier Sol implementation authorization. Only root edits Main; delegates own isolated clones. Ordinary tests stub Git including setup. Necessary native Git adapters retain their actual integration assertions. No new inventory, audit system, emulator, dependency, or unrelated scope. User explicitly added bounded CLI help fixes; broad command restructuring already belongs to queued goal `verbs-match-intent`. Related `human-goal-verbs-forgiving` is done.

## Current performance slice

Implement only the three authorized changes: remove duplicate same-configuration execution, shrink heavyweight decision/failure fixtures using existing stubs, and eliminate measured repeated proof-reuse operations. The acceptance conditions are in `plans/test-suite-redesign.md`. No whole-suite speedup is proven; earlier 2x/8x claims do not describe the full suite.

Main base is `89d3674aa3a93929c6905e322f5cebfa4ea2ef36`; inherited staged work is backed up as candidate `58919d4e9dc51ac89b95f87beebdc3094115495f` on `backup/test-parallelism-repairs-20260924`. Preserve the live unstaged narrator append. Installed trusted engine is generation 177 at the Main baseline. Do not rebaseline again from older handoff sections below.

Four actual Opus children are supervised under `E/coordination/performance-opus-20260924/{dedup,fixtures,reuse,fixture-repair}`. Each directory holds `launch-input.json`, `process.json`, `events.jsonl`, and eventually `exit.json`; requested and observed model is `claude-opus-5-5`. The native launch build adapter currently hardcodes Codex, so these use directly supervised Claude CLI under the user's existing machinery-workaround authority. Their private source paths are in launch-input.json. Sol implementers stopped and handed over preserved edits. The small fourth assignment finishes already identified goal-layout/strict-stub fixture repairs; it is not another performance project. Do not start duplicate jobs.

At transfer: dedup has a traced design but no edits; fixtures has a two-file scenario migration, not yet tested; reuse has measured duplicate DEEP planning and temporary profiling that must be excluded; fixture-repair has two formatted, untested fixture edits. The separate one-file deadline-receipt fixture repair passed focused/Git-denied checks and has been applied to Main's index. No new full suite until focused correctness and actual mechanism observations justify it. Collect meaningful before/after measurements from existing runs; do not rerun only to repackage reports.

NEVER read, echo, copy, or diff actual `metasystem/metasystem.conf.local`: secrets. Transport only the tracked index tree. Preserve all inherited work and the four zero launch caps. Mac uses nine testing workers. Linux uses six workers on six CPUs and 16 GiB RAM. Preserve inherited `METASYSTEM_TEST_WORKERS`. Full Linux remains required for this runner/process goal; future Linux validation is selective.

## Locations

- Main: /Users/wido/LocalStorage/GitHub/agentic-tools-m1e
- E: /Users/wido/LocalStorage/agentic-tools-evidence/test-parallelism-20260921
- C: E/coordination/git-free-tests-20260923
- T: /Users/wido/LocalStorage/transport/test-parallelism-20260921
- UI clone: /Users/wido/LocalStorage/GitHub/agentic-tools-ui

Main HEAD is now `d76c71e3ba04e8edde533ee693330c1405bd0af0`, the incoming UI merge. Root reconciled it with all accepted test work before final validation. Accepted product edits are staged; the narrator may have legitimate unstaged append-only entries. Goal revision15 has 15-day elapsed,40-attempt,1800-minute,8-active budgets; no fence at last supported read.

## Accepted source

**330 accepted units; 1,583 original test parents converted.** `C/git-conversion-count-live-330.json` records zero remaining ordinary labels in the fixed mapped cohort and zero remaining identified mixed CMD policy claims. This is not a certified repository-wide denominator. The 38 mapped mixed parents overlap ordinary claims;34 physical parents remain justified adapters. Reclassifications and mixed-policy extraction receive no whole-parent conversion credit.

Every accepted patch and root disposition is in `C/accepted`. Root read each whole diff and independent read before applying. Latest units:

-325 governed/retry2: actual process custody, retry accounting and budget behavior with file fixtures; shared helper snapshot reconstructs accepted inputs without Git. Patch21ca9c61e03dd8850d8cc8f66ebe72f31f1d9dc234eb280ea8422f5d379d4bae. Required legacy explicit-tree contract preserved.
-326 receipt-clock3: budget refusal, five clock positions, delayed/cancel and shell receipt cases; exact five-parent Git-denied race/atomic proof passed20.019s. Patchf028edc7a6edf3afa8f2a5b442d00c4650dfc8be4175df29ddc1280c4bb9c915.
-327 branch policy2: through/later exclusion and lagging/endpoint exclusion exercise actual selection/fold/patch owners with four typed fact readers. Git-denied race proof passed. Native ancestry/tree witnesses unchanged. Patch8a1fe8d3256688e89db12bfc03831b7169b53d45f93b0c27af66da279a44140a.
-328 branch/chain verdict policy: existing read seam reaches actual commit wrapper and strict verdict owner; parallel LandSeries table proves branch refusal/pass and chain behavior with Git denied, race/atomic. Native trailer/push witnesses retained. Patch600c740c3636990ebc965410992aa85b9a57b56345c58030867d4aa90664363c.
-329 CLI help: user-scoped launch record guidance and correct --id examples, family-specific errors, sole goal help before mutation with shared-option caveat. Focused tests pass, four new tests fail on base,40 rebuilt CLI checks pass. Independent read20260924t181403-936d3693b8 returned land/zero findings. Patch6338cd62e20d76631f400f0e1d581f621bb53005a05f0666425a845001ee95c6. Broad help review at E/coordination/help-review.md covered42families/433verbs statically and sampled runtime behavior, not433 handler executions.

Final shadow E/units/after-by305-backup/source compiled CMD default/tagged and batch at329. Both static modes identified the same unused pre-snapshot assignment in proof_run_test.go. Mechanical correction330 removes it and declares root at the existing consumed read; patcha85d626dc27ba6977f6b2d13fe26e6ab7929fa8d1e2970bc3e031e6ca256c767. The delegate is repeating only affected CMD compilation/static. All product code is frozen; only evidenced final failures may change it. Actual per-unit proof paths and native read IDs are in acceptance records. Do not rerun focused tests for report packaging.

## Durable recovery

Remote branch `backup/test-parallelism-20260924` last root-verified through324 at `30cbe7d894b96fd74ff78a3a9dcae01964c67e6b`, parent ea0acfbc56129efb085c49a1bb0535c29ece873f, tree0b94da84f3bbf33370f1ada1b9bee73a2e5f06a2. Source is the shadow above. E/units/after-by305-backup/root-verification324.json and C/remote-backup-20260924-latest.json retain replay, append-only receipt, compilation/static and remote evidence.325–330 need the next normal fast-forward backup push.

Private shadow receipt history differs lawfully from Main after the common prefix; never overwrite it with Main's ledger. Use the actual frontend E/rollout/final-diagnostic-20260924candidate274-retry1/candidate-frontend receipt add with explicit private --root and --file; the shell wrapper pins Main. Root alone commits/pushes.

## Final validation

E/rollout/final-candidate-20260924prepared/prepare.sh creates a private commit from Main's exact index, parent47dbe0f18; it transports to a clean standalone clone and builds a candidate-stamped external frontend. It does not move Main. Its bindings.env supplies candidate repo/commit/tree/frontend. No local configuration is copied.

Mac: E/rollout/final-delivery-20260924prepared/run.sh, human tmux,9workers. Uses test plan/run/verify against Main control/index, exact final tree, deep delivery, --all-groups --force-groups,120-minute cap. Requires prior168 selected IDs remain covered and current required IDs are selected. All-groups means continue independent work after a failure. No diagnostic-only selection or artificial batch tip. Observer E/rollout/final-selected-delivery-20260924bx/watch.py takes the output directory; also directly inspect independent runner argv because helper processes can inflate counts.

Linux: E/coordination/linux-final-private-candidate-prepared/run-final-normal.sh with same clean candidate, candidate-stamped POLICY_ENGINE and fresh RUN_ID; sources safe explicit release-test-environment.sh, enrolls human tmux and uses ordinary proof-run launch with private execution root/Main control root. Guest standalone clone under /home/wido.guest, native Git2.55, six workers, ordinary full validator with race/coverage floors. Actual stage table, custody and native exit required; no synthetic receipt. Scripts were read and syntax/preflight checked; final runs have not yet begun at this handoff write.

Preserve warm cache E/rollout/final-selected-delivery-20260924bx/go-cache and pinned staticcheck C/steward-deadcode-build/go-cache/80/80346ab3a04b363533ecdef504967486f3f921df1a319f4bd3a6fa738215df85-d/staticcheck. Do not build duplicate cold caches. Use direct authorized host execution if sandbox cache/sysctl restrictions interfere.

## Previous measurement and remaining delivery

Latest full diagnostic was candidate274, not this candidate: E/rollout/final-diagnostic-20260924candidate274-retry2/result.json. All168groups executed:162pass,5fail,1invalid; native67m14.521s, wrapper68m56s. Nine actual Go runners observed. Three causes have focused passing repairs286(source guard),300(actor helper allow-list and nested adoption),312(restored exact protected manifest test). Matching full proof remains necessary. A prior55-minute run completed less work and is not a valid speed comparison. Targeted conversions are substantially faster; no overall full-suite speedup is yet proven.

After shadow checks: update remote backup, freeze exact candidate, launch Mac and Linux, report actual start, watch concurrency and structured outcomes. Fix only evidenced failures, reuse valid evidence. Then fetch/reconcile, append final receipt, consume matching test verify, commit and normal push/integrate. Never claim all tests parallel, full speedup, or completion before the observations support it. Latest ETA22:30–01:30CEST remains low confidence.

Historical detail is preserved in C/handoff-before-final329-20260924.md and earlier snapshots; use canonical accepted evidence rather than stale active-work sections there. Host had87GiB free at20:19; VM previously66GiB. Claude distilled old artifacts/logs to .gz and DISTILLED.txt; check those before declaring evidence missing. No active source, run or warm cache may be cleaned.

## Reconciliation with the UI merge, 24 September 20:46 CEST

All330 accepted units were pushed and verified on GitHub backup/test-parallelism-20260924 at4ad556a3c046eaec87e8155bd57474d326863440. A final remote check found the UI branch had moved Main to d76c71e3b. Before any new full run, root merged that upstream into an isolated copy of local candidate43b4f27. There were19 conflict paths. Three Sol delegates preserved upstream multi-block/human-proof semantics, typed notice history and UI inputs alongside the existing Git-free fixtures, per-call transport seams, parallel worker declarations and source guards. Incoming general roster/UI language rules are retained; explicit current-session Sol delegation still applies. Four zero window caps remain.

Root caught and fixed a semantic interaction in the automatically merged Tick caller: public Deliver would journal a second time under the typed notice owner. Both default transport slots now use raw deliver; the existing composed Tick test observes one typed row. The full goal package passed-race once in49.286s (1227 pass events,2 skips,zero failures). Three newly incoming multi-block tests also use fake endpoints and passed their exact Git-denied race check. Refusal source guards pass after51 source anchors moved; all293 codes remain. The37 actor sites match the merged brain allow-list. Combined CMD default/tagged, goal/steward compilation and pinned staticcheck across33 changed Go packages all passed in24.025s. No full suite has run on this merged candidate yet.

E/units/final-ui-reconcile contains the original conflict stages, resolution-frozen.patch, focused evidence and frozen-merge.json. Reviewed source freeze tree390c434984c0b4d6618a9bbb2546cef6d9430d5d; resolution patch SHA2934e48a4cffe80157cc1671c089fa051ad424d3584d6e37755fe5e38b0bfa49. The first independent reader20260924t184452-f76fc86ab2 refused an untrusted archive directory before any model calls; an empty private .git fixed that tooling precondition. Independent native Astra read20260924t184651-34a440fb40 is running against a separate frozen copy. Its material verdict is still required; no new redesign is permitted. The only excluded focus-patch body is the obsolete6322-row inventory deletion, with owner evidence retained.

Root applied the merged tree to Main's index and advanced only its base reference47dbe0f18→d76c71e3b. No Main product commit or push occurred. The live narrator file was untouched. main-rebaseline-result.json records the operation. New documentation/receipt packaging follows that source freeze and does not change tested code. The earlier final-candidate-20260924prepared source43b4f27 and engine are superseded: do not launch them. Prepare a fresh final-candidate-20260924reconciled from the current Main index and new parentd76c71e3b. Existing Mac/Linux scripts consume its new bindings; Mac output final-delivery-20260924prepared has never started.

Next: finish the bounded independent read, freeze/build the new candidate, push its private backup reference, launch Mac9 and Linux6 concurrently from human tmux, observe actual concurrency and structured outcomes, then consume matching proof and integrate Main. E/coordination/git-free-tests-20260923/final-integration-live.json supplements this page.
