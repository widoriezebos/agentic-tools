# Build report: public error messages outside U9b

Branch `public-error-messages` in `/Users/wido/LocalStorage/GitHub/agentic-tools-emsg`,
built from the brief `metasystem/plans/public-error-messages-build-brief.md` and the audit
`/Users/wido/LocalStorage/agentic-tools-evidence/error-messages-20260928/public-audit.md`.

This work followed the brief and two later messages from the coordinator:

1. Merge origin/main (batch 11, 23ed66c98) into this branch. Done in 17e1df779.
   `internal/refusal/register.go` was taken from main, because main's register rows now
   anchor as `file.go#Symbol`. My one line re-pin (`RECEIPT_USAGE_INCOMPLETE`) is
   therefore gone. This branch adds and changes no register rows.
2. m1e-c4's scope update: drop every fix for a row U9b (batch 12) fixes. Keep EM-07, 08,
   18, 20, 25, 27, 28, 29, 32, 33, 38, 41, 44, 46 and 47.
   - Dropped in e6097c941 and 8564da0e1: EM-02, 03, 21, 36, 37 and 49. Those files, and
     their test pins, match main again.

U9b's file list came from `/Users/wido/LocalStorage/GitHub/verbs-u9a`, using
`git diff --name-only origin/main...u9b` plus `git status --short`. That clone has no
local `main`, so the brief's `main...u9b` does not resolve there.

## One line per EM id

- EM-00: skipped because of U9b (the audit excludes it).
- EM-01: fixed by U9b.
- EM-02: fixed by U9b. My receipt.go half is dropped.
- EM-03: fixed by U9b. My stoploss.go fix is dropped.
- EM-04, EM-05, EM-06: fixed by U9b.
- EM-07: fixed, `TestContextBudgetOfAMissingRootIsAnError` (internal/steward).
  - A root that does not exist is now unknown, with the error "ROOT does not exist, so there is no installation to read; nothing was read". The status command exits 1.
  - The coordinator said to keep it, though U9b also touches internal/steward/context.go.
- EM-08: not fixed; its whole fix is in a file U9b changes.
  - status.go already returns 7 for a missing mission, and its line is a format drivers parse.
  - The false exit 0 comes from cmd/metasystem/intent_process.go:1672-1673, which forces 0 on every status read.
  - U9b changes intent_process.go, so nothing here changes it.
- EM-09 to EM-17: fixed by U9b.
- EM-18: fixed, `TestUnknownProofAttemptSaysWhereItsIdComesFrom` (cmd/metasystem). Reproduced first: `work status --all` in the probe clone listed 2230 j1 jobs and not one `proof:` reference. `test wait` on an unknown attempt now says where a proof id comes from, instead of pointing there.
- EM-19: fixed by U9b.
- EM-20: fixed, `TestFrontierStatusOutsideARepositoryRefuses` (internal/report).
  - Outside a repository, experiment status now refuses with code 2 instead of "no frontier recorded at plans/frontier" and exit 0.
  - It asks Git through the existing gitRead seam. `TestFrontierUsageErrors` now uses that seam.
  - Resolving plans/frontier against the repository instead of cwd is report_frontier.go's (U9b).
- EM-21 to EM-24: fixed by U9b. My EM-21 texts in intent_planning.go and intent_goals.go are dropped.
- EM-25: fixed, `TestProjectOfAnUnfetchedLedgerNamesTheFetch` (internal/goal) and `TestUnfetchedLedgerNamesTheFetch` (cmd/metasystem).
  - The new sentinel `goal.ErrLedgerNotFetched` is used.
  - goal verbs say "this checkout has not fetched the goal ledger yet; nothing was read", then "run: metasystem goal list --fetch". The channel test pins were updated.
- EM-26: fixed by U9b (the registration part). The rest of EM-26 is in internal/config/validate.go, which U9b changes, so I left it.
- EM-27: fixed, `TestAddValidation` (internal/receipt). The `metasystem receipt add:` prefix is in receipt_verbs.go (U9b).
- EM-28: fixed, `TestBareGrantRevokeAndGoalOpenNameTheWayForward` (cmd/metasystem). It now says "run: metasystem grant list".
- EM-29: fixed, `TestBareGrantRevokeAndGoalOpenNameTheWayForward` (cmd/metasystem). G is listed first, and the full command shape is given.
- EM-30, EM-31: fixed by U9b.
- EM-32: fixed, `TestStatusResultStates` (internal/ui/lifecycle).
  - The status line of a stopped or stale interface now ends "start it with: metasystem ui start".
  - Exit 1 is unchanged. The audit calls it a dead end, not false, and scripts may test for it.
- EM-33: fixed, `TestFrontierScoreRefusalsSayWhatIsWrong` (internal/report).
- EM-34 to EM-37: fixed by U9b. My EM-36 and EM-37 stateroot.go fixes are dropped.
- EM-38: fixed, `TestHelmTakeRefusalSaysWhyInPlainTerms` (cmd/metasystem).
  - The refusal names the reason in plain words ("an agent started this shell" or "no terminal was found above this shell"), with no refusal code and no doubled prefix.
  - The helper that turns proof outcomes into those words, `actorProofReason`, now lives in intent_helm.go, which U9b does not change.
  - `TestHelmTakeRefusesAgentAncestry` used to require the leaked `AGENT_IN_AUTHORITY_CHAIN` code. It now requires the plain reason.
- EM-39, EM-40: fixed by U9b.
- EM-41: fixed, `TestResolveMachineWithoutANicknameNamesTheFix` (internal/goal) and the "no machine nickname" case in internal/landing/landpath/commit_test.go.
  - The text is now "no machine nickname is enrolled on this machine; name it once with: git config metasystem.goal.machine NAME".
  - It keeps the old prefix, so existing Contains pins still hold. Two fixtures that return the text were updated.
- EM-42, EM-43: fixed by U9b.
- EM-44: fixed, `TestStopReportDirectoryRefusalsNameTheCause` (internal/stopreport).
  - The audit's inferred cause, the scratch path, did not reproduce. The directory was missing, so the old message was false, not just jargon.
  - A missing report directory, alias directory or alias now says no Stop report is recorded. A real redirection names the directory and the symlink.
  - `stoppresentation_test.go` was updated.
  - The `report stop-status` prefix is report.go's (U9b).
- EM-45: fixed by U9b.
- EM-46: fixed, `TestGoalRefusalsDoNotRepeatThemselves` (cmd/metasystem). It now says "goals you can name", and the repeated needed-first line is gone. `goal reopen` still gives no list of goals.
- EM-47: fixed, `TestGoalRefusalsDoNotRepeatThemselves` (cmd/metasystem). The permission list is no longer repeated as a needed-first line.
- EM-48, EM-49, EM-50: fixed by U9b. My EM-49 intent_adopt.go change is dropped.

This branch still edits these files that U9b also changes, as the coordinator directed:
- cmd/metasystem/intent_planning.go (EM-28, EM-29)
- cmd/metasystem/intent_goals.go (EM-25, EM-46)
- internal/steward/context.go (EM-07)

internal/pathclass/pathclass.go still has the old "<installation>/bin/metasystem" and
"not inside a Git repository: fatal" texts. No audit row names it, so it is untouched.

## Proof

All runs used `METASYSTEM_TESTING_WORKERS=9`, from `metasystem/`, on the tree after the
merge and the drops (8564da0e1). The engine was first built into the worktree's ignored
`bin/metasystem`: three internal/steward tests need it (`engine binary not built`).

- `go run ./cmd/devgate static`: **passed** (exit 0). "go gate: fast mode passed
  (dependency ratchet, parallel ratchet, gofmt, shell parse, vet, staticcheck, refusal
  register, SessionStart exit audit, Stop decision surface audit, build)".
- `go test` on internal/goal, internal/steward, internal/channel, internal/landing/landpath,
  internal/stopreport, internal/report, internal/receipt, internal/ui/lifecycle and
  internal/refusal: **all ok** (exit 0).
- `go test -timeout 90m ./cmd/metasystem/`: **one failure**, `TestIntentCloseWholeOwner`
  (intent_delivery_test.go:389, "owner refusal: outcome confirmed, want refused").
  - The test assumes that with no evidence root the close owner refuses. The base
    branch, evidence-root-default, now gives an unset evidence.root a default, so the
    owner closes the chain.
  - That base is on main. This branch changes nothing on the work finish, close-owner
    or evidence path.
  - I infer that the test fails on main too. I did not run it there: that needs another
    checkout, outside this worktree. The test was left as it is.
  - Every other test in the package passed. An earlier run at 10m timed out (the package
    takes about 65 minutes here), so the run above used `-timeout 90m`.
- The fixed rows were run end to end through a private build, in a `git clone --shared`
  probe bed in the scratchpad (push URL blocked) and in a directory outside any
  repository.
  - Before each fix, that run reproduced the audit's text for the row.
  - After the fixes: EM-07, 18, 20, 25 (in a clone with no ledger refs, where `goal list
    --fetch` then worked), 27, 28, 29, 32, 33, 38, 41 (unit only), 44, 46 and 47 each
    printed the new text.
  - `metasystem grant list` and `metasystem receipt status --root metasystem` also run.

Incident: to stop my own stale test run I ran `pkill -f "metasystem.test"`. That pattern
may also have ended test processes of another builder on this host. The sibling
worktree was never touched, but any of their Go test processes running at about 01:06
may have been killed; I did not check whose processes matched.

## Sol's code critique and its dispositions (2026-09-29)

Sol: `metasystem/plans/public-error-messages-sol-critique.md`, 3 material.

| id | disposition | where |
|---|---|---|
| SOL-EM-01 | Not taken. With an explicit `--file`, printing a present file's contents outside a repository is true output, not a false success. EM-20 was an absent default path reported as "no frontier recorded" outside any project, and that stays refused. The existing test pins reading a present frontier without asking Git. | `internal/report/frontier.go:frontierStatusWithGit` |
| SOL-EM-02 | Fixed. Only a frontier that does not exist is "no frontier recorded"; any other read error refuses with exit 1 and names the path. | `frontierStatusWithGit`; `TestFrontierStatusUnreadableIsNotAbsent` |
| SOL-EM-03 | Fixed. The proof id is "in the output of metasystem test run", without claiming when it prints. | `cmd/metasystem/intent_references.go:noReference`; `public_error_messages_test.go` |
