# dispatch-bases-chains-on-the-remote-tip

- State: approved
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: a chain built on a stale base fails at landing with a rebase conflict after hours of builder and critic work, and a seat then resolves it by hand; novelty 1: a fetch and a base choice in a door that exists; exposure 3: every chain on every machine, four seats on one host landing all day; accumulation 2: the more seats land, the staler every other seat's base and the likelier the conflict"
- Tier: 3
- Intent: Wido, 2026-09-06: 'when the meta system starts a new goal, we are not pulling the remote, which means we might have a higher chance of running into a merge conflict ... Machinery should do this.' Confirmed: the dispatch door creates a chain's worktree from the local checkout's HEAD (scripts/agents/dispatch.sh, the worktree add) and nothing between goal claim and dispatch fetches the canonical branch - the goal verbs fetch only the ledger ref (internal/goal/txn.go, --refmap=). The landing wrapper fetches and rebases onto origin/main (scripts/agents/land.sh), so a stale base surfaces only there, as a conflict, after the builder and critic rounds were spent on it; the follow-up door merely warns WORKTREE-BEHIND and asks the seat to merge by hand. Seen all day on m1: local main 19 commits behind at one dispatch, 4 behind at this writing, manual pulls before every landing. DONE means machinery does it: the dispatch door fetches the canonical remote and bases every new chain worktree on the remote tip of the sync branch (never a stale local HEAD), refuses with a plain message when the fetch fails (no silent stale base), and fast-forwards the seat's own checkout when its tree is clean or says by how much it is behind when it is not; the follow-up door brings the chain worktree up to the remote tip itself when the chain has no conflicting product changes and otherwise refuses naming the conflicting files instead of warning; goal claim reports the checkout's lag behind the remote on every claim. A stale base is thereby impossible to start from, and a landing conflict can only come from work landed during the chain, which the rebase at landing already handles.
- Origin: main
- Next step: MECHANICAL, one chain: dispatch.sh (fetch, base on refs/remotes/<remote>/<branch>, refusal on fetch failure, clean-checkout fast-forward), the follow-up admission in internal/dispatch (merge-or-refuse instead of the warning), goal claim's lag line; fixtures: a dispatch against a stale local main bases on the remote tip; a fetch failure refuses; a follow-up on a behind worktree without product conflicts merges, with conflicts refuses naming the files; the existing dispatch fixtures keep passing. Any free seat; half a day.
- OpenedAt: 2026-09-06T12:31:31Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T12:31:59Z revision=2 opid=QTM46H2XYQ075XHFX1D7FD5WWW-m1-7cd0bd60 authority=proven digest=b1cd64f41ce42fee1f6d65f44fcf8210010887c256da435bf991cd96b587b7d9

History:
- 2026-09-06T12:31:31Z 5T8DE34F5JFW4ZZRYNBESXDPKW-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=dispatch-bases-chains-on-the-remote-tip
- 2026-09-06T12:31:59Z QTM46H2XYQ075XHFX1D7FD5WWW-m1-7cd0bd60 approve actor=human:Wido targets=dispatch-bases-chains-on-the-remote-tip
Integrity: sha256=28bc12a9c527e8b43c7afa7b68e058d142eb654bd36aed60e2ba17d801e57c34
