# Exceptional-delivery design adjudication

Root read Fable round1 in full (4 material ids, plus IC-C5 minor), 26 September2026.
The report's IC-C3 and IC-C4 material labels exceed its own first-use criterion,
but their concrete simplification/remedy suggestions were considered, not dropped.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| IC-C1 | accepted | ReadCarryStatus requires the accepted ledger tip and uses local code refs; real land.sh fetches both first. Stale consumption could leave an unnecessary staged candidate. | Explicit owner fetches before recording/staging and fetched-origin-consumption fixture. |
| IC-C2 | accepted | LockPath resolves only the supplied root. Primary and goal-worktree paths are different mutexes; existing Advance locks primary. | PRIMARY named for lock, artifacts, drift, posture and carry status; canonical state remains endpoint-owned. |
| IC-C3 | accepted | Delete stored patch/digest because Diff is deterministic, but complete identity deletion is unsafe as proposed. Exact search `type CarryWord` at verbs.go4581 returned Goal, History, Workspace, Past, Expires, Supersedes; `type HistoryLine` at file.go446 has no code endpoint. Diff(new main B, old approved C) would undo newer product after dropping the expected old base. Retain only immutable base/workspace tuple and opid association, no patch or job registry. | Reduced subject to minimal tree identity, explicit moved-main counterexample and ambiguity refusal. |
| IC-C4 | accepted | Recomposition on current endpoint plus explicit carry replacement is already the requested exceptional-land act. It does not require a new goal endpoint verb or silent risk judgment. | Show public replacement-exception command and assert it on endpoint refusal. |
| IC-C5 | accepted | Plain Git application is cwd-sensitive; nested metasystem is the actual repository shape. Root independently found this same first-use failure from Apply source. | Persistent application explicitly runs at Git top level; linked/nested outside-prefix fixture. |
