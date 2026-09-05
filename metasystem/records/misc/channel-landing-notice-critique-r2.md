# Landing-notice design critique — round 2 (closing round on revision 2)

Chain: revision 2 (landed 6c5939356, sha256 05cf0fefb417b9b415c84351063c1b3f2ac63d4a702b69f47b21f6f1363b61fe) -> critic channel-landing-notice-critique-r1-20260906-r2 (claude, design-critic, round 2 of root channel-landing-notice-critique-r1-20260906), 2026-09-06. SEVEN material findings (three high, four medium): revision 3 is required before any build, and this root has spent both review rounds. Two authority gaps the critic confirms are Wido alone to settle: the landing scope (the proposed default is coherent and one word settles it) and accepting at-least-once delivery with a marked repeat in place of "exactly once".

## CLN-R2-COMMIT-ERREXIT — high, material=True

CLAIM: The shared reap function can change commit.sh's exit status after a successful origin push. Revision 2 proposes an unguarded `wait "$pid"; rc=$?` and unguarded termination commands, then says the function always returns zero. Under commit.sh's `set -euo pipefail`, a notifier that exits nonzero or is killed makes the shell exit at `wait` before `rc=$?` or `return 0`. An implementer copying this shape would violate the promised failure isolation precisely on the failure and hang paths the fold is meant to cover.

EVIDENCE: metasystem/plans/channel-landing-notice-design.md specifies the shared reap shape in Decision 2. metasystem/scripts/agents/commit.sh starts with `set -euo pipefail`; metasystem/scripts/agents/land.sh does not enable errexit. Expected nonzero commands must be placed in an errexit-safe conditional context for the stated contract to hold.

## CLN-R2-REFLOG-REPLAY — high, material=True

CLAIM: The ref-move fold still permits two unmarked announcements for one push. Revision 2 expressly says linked worktrees share the remote-tracking reflog but keep separate notice artifacts, so either worktree can accept the same historical `new-id update by push` line and create the same logical notice independently. Because the key also includes the pushing machine identifier and the check searches any matching reflog line rather than binding the old-to-new transition, separate clones can likewise assign different identities to the same ref move. Calling this residual does not resolve the round-one identity and concurrency finding.

EVIDENCE: Decision 3 in metasystem/plans/channel-landing-notice-design.md says linked worktrees can duplicate and defines the key over remote, ref, old object identifier, new object identifier, and pushing machine. Decision 2 accepts any matching historical remote-tracking reflog line with subject `update by push`. No cited check in metasystem/scripts/agents/land.sh excludes sibling worktrees or establishes a globally unique owner.

## CLN-R2-RETRY-FALLBACK-STALE — high, material=True

CLAIM: The fallback promised for loss of the pushing checkout is not supported by the cited status path. The design says another machine's four-hourly status will digest the landing within the status interval, but status reads that machine's local origin/main and performs no fetch. A machine that has not fetched the pusher's ref move can therefore omit the landing indefinitely; the accepted-loss boundary is wider than the design tells Wido.

EVIDENCE: metasystem/internal/channel/report.go derives landing lines from the local origin/main remote-tracking ref. metasystem/internal/channel/phase/phase.go contains no fetch and uses the offline projection path; metasystem/internal/goal/project.go documents `Project(..., false)` as offline-capable. This disagrees with the revision-2 statement that status from another machine supplies a bounded fallback.

## CLN-R2-TIMEOUT-CAP — medium, material=True

CLAIM: The configurable hang deadline is not actually bounded. Revision 2 accepts every positive `channel.landing-notice-timeout-sec` value and derives the shell termination window from it. A value such as 3600 would allow a hung prompt to delay the landing command for roughly an hour, contradicting the stated short-bound and failure-isolation contracts. If configurability is retained, the design must decide a strict accepted range; otherwise a fixed constant is the buildable contract.

EVIDENCE: The hang fold in metasystem/plans/channel-landing-notice-design.md introduces `channel.landing-notice-timeout-sec`, default 5 seconds, and says parsing follows the positive-value pattern used by httpTimeout. The cited parser in metasystem/internal/channel/phase/phase.go rejects nonpositive values but imposes no maximum.

## CLN-R2-UNDELIVERED-AGE — medium, material=True

CLAIM: The status fold adds pending and posting records to Undelivered but does not specify the corresponding oldest-undelivered timestamp. Report composition computes an age from OldestUndelivered, while the proof row asserts only the expanded count. Implementers are left to guess whether and how pending and posting creation times participate, and leaving the zero value would produce a nonsensical age.

EVIDENCE: Decision 5 and proof row 15 in metasystem/plans/channel-landing-notice-design.md specify the expanded Undelivered count but not its oldest timestamp. metasystem/internal/channel/report.go computes `Now.Sub(OldestUndelivered)` when rendering that field, while metasystem/internal/channel/phase/phase.go currently supplies the count without supplying OldestUndelivered.

## CLN-R2-NEW-BRANCH-STALE-REF — medium, material=True

CLAIM: The new-branch content algorithm can make the announcement shorter, contrary to its stated conservatism. It excludes commits reachable from every locally known origin remote-tracking ref. A stale local ref for a branch that has been deleted on origin can therefore hide a commit that the new branch has just made reachable on origin. The design states that incomplete local knowledge can make the list longer but never shorter, which is false without a fetch-and-prune or a different authoritative comparison set.

EVIDENCE: Decision 1 in metasystem/plans/channel-landing-notice-design.md specifies `git rev-list <new> --not <every refs/remotes/origin ref except the target>` and claims missing remote knowledge can only lengthen the result. metasystem/scripts/agents/commit.sh does not establish a fresh, pruned origin-ref snapshot before that calculation.

## CLN-R2-PROVIDER-CAPABILITY — medium, material=True

CLAIM: The design overstates the evidence for the delivery narrowing. The adapters prove that this repository does not currently pass an idempotency key, not that neither provider accepts one. Telegram's published sendMessage contract exposes no such input, but Slack's official documentation contains client_msg_id-related duplicate errors despite omitting that field from the ordinary argument list. The human decision should therefore be framed as accepting at-least-once delivery for the uniform current adapter contract, not as accepting a conclusively proven provider impossibility. A marked repeat remains a narrowing of human-readable “exactly once,” not an equivalent reading, so Wido's explicit word is required as revision 2 already recognizes.

EVIDENCE: metasystem/internal/channel/telegram/telegram.go and metasystem/internal/channel/slack/slack.go send no idempotency field, and metasystem/internal/channel/channel.go exposes none. The current official documentation is not sufficient to prove the stronger categorical Slack claim: [Slack chat.postMessage](https://api.slack.com/methods/chat.postMessage); Telegram does document no key: [Telegram Bot API](https://core.telegram.org/bots/api).

## Critic-declared gaps (verbatim)

- No Go tests or live provider calls were run, as required by the read-only brief. The fake-provider deadline assessment is source-level proof only.
- The landing scope remains an intentional authority gap: the proposed default—main-only landings performed by the two landing scripts, paper excluded, notice launched after the origin push and before transport synchronization—is coherent and one explicit word from Wido can settle it. The design is not otherwise buildable yet because the material findings above remain.
- The at-least-once, visibly marked-repeat promise is an explicit narrowing of “exactly once” and still requires Wido's separate acceptance or refusal; it should not be inferred from the scope decision.
