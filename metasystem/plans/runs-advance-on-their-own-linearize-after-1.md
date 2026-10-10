# Brief: runs-advance-on-their-own, correction of the linear history (every commit builds)

Working Mode: Implement (git history surgery + one test)
This worktree is on branch goal/runs-advance-on-their-own at e99ea7553 (linear on origin/main 02f0957be: D3, D4, D3-fixes-test, D1, D1b, D2, D2b, D5, D5b, integration-fixes). Backup refs: backup/runs-advance-on-their-own-20261009 (the merge-laden original) and backup/runs-advance-on-their-own-linear-20261009 (= e99ea7553). The Opus read of the conversion found:

F-1 (must fix before publication): seven of the nine unit commits do not build. The hand fix that the dropped merge 219309577 carried (CheckContinuationInputs in internal/launch/unit_named.go and friends) was restored by amending the TIP commit D5b' (88a52522e, which gained 16 files, +483 lines, its message still quoting a read that never saw them), while D4' (a30ab47f9) already calls CheckContinuationInputs (cmd/metasystem/intent_selection.go:173). `go build ./...` fails at a30ab47f9, 02f1bee8b, 2bf0f04b1 and by construction at 45095b713, fb5cf28d7, 0f8199c94, b0491f824. The 16 files are listed in the file named below.
Do: rewrite the branch (plain git: a scripted rebase/cherry-pick sequence, never squash units) so that the restored changes sit where they belong in history: one new one-parent commit "goal runs-advance-on-their-own units D3-fixes" directly BEFORE D4' (or, if D4' itself needs them to build, fold only the production hunks it needs into D4' and keep the rest in D3-fixes), with the message: "The hand fix of 2026-10-09 (merge 219309577 on the original branch): CheckContinuationInputs and its callers, restored at its place in history after the plain-git linearisation." and the final paragraph `Goal-Unit: runs-advance-on-their-own/D3-fixes` + `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>`. D5b' must then carry only what the original D5b (bffbb8249) carried (its own files), with its original message. The final tree of the branch must be IDENTICAL to e99ea7553's tree (`git diff backup/runs-advance-on-their-own-linear-20261009 HEAD` empty). EVERY commit from origin/main to the tip must pass `go build ./...` and `go vet ./cmd/metasystem ./internal/launch` (check each with `git worktree add` of a temp dir or `git archive`; report the exit per commit). Trailers stay in the final paragraph of every commit (verify with `git log --format='%h %(trailers:key=Goal-Unit,valueonly)' origin/main..HEAD`).

F-2 (fix forward, in the integration-fixes commit, amended): internal/launch/unit_stop.go:302-309's early return (CollectExamination for a commit-less stopped-patch subject with a matching read step) also changes main's own stopped-patch review path and no test pins it. Add one launch-level test in internal/launch: a stopped-patch subject with Commit "" and a matching read step keeps its real stop (Reads not cleared, the stop not replaced by unavailable-stop-inputs); mutation: restore the pre-fix behaviour -> red. Run `go test -count=1 -timeout 30m ./internal/launch -run 'TestUnitStop|TestCollect'` (name the exact tests).

Report: the new tip, `git log --oneline origin/main..HEAD`, merge count (0), the per-commit build/vet exits, `git diff --stat backup/runs-advance-on-their-own-linear-20261009 HEAD` (must be empty apart from the new test), the new test and its mutation. Use `--no-verify` for the rewritten commits (the hook refuses agent commits here; say so). Do not push. Never open metasystem.conf.local; do not touch memory/, records/, plans/.

The 16 restored files (D5b' minus original D5b):
metasystem/cmd/metasystem/fleet_build_admission_test.go
metasystem/cmd/metasystem/intent_build_outcome_test.go
metasystem/cmd/metasystem/intent_driver_test.go
metasystem/cmd/metasystem/intent_process_roots_test.go
metasystem/cmd/metasystem/intent_selection_shared_test.go
metasystem/cmd/metasystem/intent_unit_driver.go
metasystem/cmd/metasystem/intent_unit_driver_test.go
metasystem/cmd/metasystem/intent_unit_review.go
metasystem/cmd/metasystem/intent_work_merge_test.go
metasystem/cmd/metasystem/intent_work_test.go
metasystem/internal/launch/codexsandbox_test.go
metasystem/internal/launch/landing_test.go
metasystem/internal/launch/seat_test.go
metasystem/internal/launch/settings_test.go
metasystem/internal/launch/unit_named.go
metasystem/internal/launch/unit_named_merge_test.go
