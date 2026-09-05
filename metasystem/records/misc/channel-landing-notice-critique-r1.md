# Landing-notice design critique — round 1

Chain: revision 1 (landed fe5c2937b, sha256 b490e27c87bcc3012ed6cfe5f78aad274c5680a7ade5824ef618baa205e85d2e) -> critic channel-landing-notice-critique-r1-20260906 (claude, design-critic, fresh context), 2026-09-06. NINE material findings (six high, three medium): revision 2 is required before any build. Register root for closure: channel-landing-notice-critique-r1-20260906. CLN-R1-LANDING-SCOPE asks which event Wido authorized; that is his to answer and the fold records the cases and a proposed default without deciding.

## CLN-R1-EXACTLY-ONCE-ACK — high, material=True

CLAIM: The design cannot provide the promised exactly-once channel delivery. It performs Provider.Post first and only then rewrites the local record from pending to posted. If the provider accepts the message but its response is lost, or if the posted-state rewrite fails, the durable record remains pending and the steward posts it again. The provider interface and both adapters carry no landing idempotency key, so there is no way to reconcile that uncertainty. An implementer following this design would build at-least-once delivery while reporting exactly once.

EVIDENCE: metasystem/plans/channel-landing-notice-design.md:188-194 specifies “Provider.Post” followed by the posted-state rewrite, while lines 308-310 incorrectly say a replay cannot occur after a post-side disk failure. metasystem/internal/channel/channel.go:42-46 exposes only text and an optional thread reference. metasystem/internal/channel/telegram/telegram.go:121-130 sends chat_id and text, and metasystem/internal/channel/slack/slack.go:69-86 sends channel and text; neither sends a stable landing key. This is also the unhandled successful-post/failed-cursor-write attack named in the brief.

## CLN-R1-HANG-ORPHAN — high, material=True

CLAIM: The shell timeout stops waiting for the notifier but does not stop the notifier or release its channel lock. In the exact fallback case the design names—a provider that ignores cancellation—the background child can remain forever inside Provider.Post while holding the shared lock, so every steward retry returns Busy. The default path also deliberately waits up to twenty seconds, contradicting the approved requirement that a failed post never slows the landing. The literal proof row can use the existing fake pause, but that fake honors cancellation and therefore cannot prove this independent shell escape.

EVIDENCE: metasystem/plans/channel-landing-notice-design.md:227-247 launches the child with “&” and, after the bound, prints “left to finish” and returns zero without terminating it. Lines 180-194 place Provider.Post under the channel lock. The approved contract in metasystem/plans/goals/channel-tells-me-when-something-lands.md:6 says failure “never blocks or slows the landing itself and is retried rather than lost.” Proof row 8 at design line 421 accepts an ordinary “exit 3,” while metasystem/internal/channel/fake/fake.go:361-375 shows pauseBefore exits on ctx.Done. Thus the existing fake proves the context layer, not the provider-ignores-context layer that leaves the lock held.

## CLN-R1-REF-MOVE-KEY — high, material=True

CLAIM: The full new-tip commit identifier is not the identity of a ref move. Different branch moves can end at the same commit, a forced rollback can return a ref to an earlier tip, and the public verb can be invoked in a second checkout after fetching the range. In one checkout, two branches ending at the same commit collide in the same filename and the second notice is skipped; in separate checkouts, the same manually replayed range creates two local records and two posts. The claim that only the pusher can ever hold a record is therefore an assumption, not an enforced property.

EVIDENCE: metasystem/plans/channel-landing-notice-design.md:134-158 exposes a verb accepting caller-supplied branch and range and stores every record as metasystem/artifacts/agents/channel/landings/<new full sha>.json. Lines 284-291 nevertheless call that new tip the ref-move identity and assert only one checkout can hold it. Lines 303-308 rely on checkout-local files and only test replay within one checkout. A ref-move identity must distinguish at least the ref, old tip, and new tip; the proposed key distinguishes only the new tip.

## CLN-R1-RETRY-OWNER — high, material=True

CLAIM: The retry owner is not defined for a pusher that cannot continue servicing its local checkout. Pending records are ignored local files; a fetching machine intentionally creates no corresponding record. If the pusher has no provider configuration, its steward is absent, or the checkout is lost before delivery, no other machine can retry or even include that pending notice in its status. The design therefore needs either a shared retry owner or an explicit deployment precondition and accepted loss behavior; “the steward retries” is not a fleet-level guarantee as written.

EVIDENCE: metasystem/plans/channel-landing-notice-design.md:164-168 leaves an unconfigured pusher's record pending, lines 286-307 say only that pusher's checkout holds the record and a fetcher does nothing, and lines 312-315 rely on that checkout's future status. metasystem/.gitignore:1 excludes metasystem/artifacts/, so neither records nor their state move through Git. This contradicts the approved goal at metasystem/plans/goals/channel-tells-me-when-something-lands.md:4-6, which covers every machine and requires failed posts to be retried rather than lost.

## CLN-R1-LANDING-SCOPE — high, material=True

CLAIM: Decision 1 substitutes “origin push by either wrapper” for the approved event without adjudicating three conflicting cases: it omits raw pushes of new features to main, includes paper prose pushes that the design itself says are not fleet deliveries, and announces before the required transport synchronization has succeeded. The last case can tell Wido something landed even while the wrapper exits saying the landing failed. An implementer cannot resolve which event Wido authorized from this design.

EVIDENCE: The human-approved wording in metasystem/plans/goals/channel-tells-me-when-something-lands.md:20 is “not sending messages when new features land,” and line 7 names main. The design at metasystem/plans/channel-landing-notice-design.md:103-111 excludes raw pushes but includes paper while admitting paper edits are not fleet deliveries. Lines 115-120 trigger before transport synchronization. In contrast, metasystem/scripts/agents/commit.sh:547-565 says “The landing is both remotes or it is not a landing” and exits nonzero when transport synchronization fails; metasystem/scripts/agents/land.sh:362-364 likewise treats synchronization as required unless explicitly skipped.

## CLN-R1-CREATE-IF-ABSENT — high, material=True

CLAIM: The specified create-if-absent commit point is not supplied by the cited helper and is outside the lock. A check followed by writeDurable is racy because writeDurable renames over an existing path. Two concurrent invocations can both observe absence; one may post and mark the record posted before the other overwrites it with pending, causing a duplicate. The same helper also does not synchronize the parent directory, so the design's crash-durable record-before-post claim is stronger than the cited code.

EVIDENCE: metasystem/plans/channel-landing-notice-design.md:154-163 requires create-if-absent “through writeDurable,” but the channel lock is not acquired until lines 169-184. metasystem/internal/channel/question.go:90-110 creates and synchronizes a temporary file and calls os.Rename; it uses no exclusive creation, hard-link commit, or surrounding lock, and it does not synchronize the parent directory. Proof row 11 tests sequential replay only, not concurrent creation.

## CLN-R1-STATUS-COUNT — medium, material=True

CLAIM: The status count is not a count of landings, commits, or notices; it is the number of distinct known goal identifiers seen in the window. Two pushes for one goal produce two notices but “Landed: 1,” while unstamped and paper notices are omitted. It therefore merely labels a later digest and cannot be checked one-for-one against received notices. Replacing identities with a count also changes digest behavior: two successive status windows with the same count and otherwise identical content have the same digest, so ShouldPost suppresses the later status despite different landings.

EVIDENCE: metasystem/plans/channel-landing-notice-design.md:378-388 calls n “the number of goal-stamped commits” and says the count changes the digest exactly as Delivered lines did. Yet the same design at lines 93-95 acknowledges per-goal collapse. metasystem/internal/channel/report.go:172-193 stores one subject per goal identifier, while lines 218-228 remove the status timestamp from the digest and lines 259-261 refuse an equal digest. The proposed proof row 14 points to metasystem/internal/channel/channel_test.go:291-319, whose two landings use two different goals and therefore cannot catch this collapse or repeated-count suppression.

## CLN-R1-NEW-BRANCH-CONTENT — medium, material=True

CLAIM: The new-branch rule contradicts the declared message content. Decision 1 says a notice contains exactly all commits added by the push, but a new branch is specified to list only its tip. A newly published branch can contain several commits not previously reachable on origin, so an implementer following step 2 will silently omit landed commits.

EVIDENCE: metasystem/plans/channel-landing-notice-design.md:55-58 defines content as exactly the commits the push added. Lines 141-147 instead say the new-branch case “lists the single commit <new>.” Proof row 20 only tests parsing the new-branch marker, and proof row 17 checks one paper message; neither tests a new branch with multiple unique commits.

## CLN-R1-RUNE-BOUND — medium, material=True

CLAIM: The 1600-rune guarantee does not bound every field used by the renderer. Subjects and goal identifiers have stated bounds, but the machine nickname and branch name are inserted untrimmed, and the cited code imposes no length bound on either. The assertion that the longest header is under 120 runes is therefore unsupported, so a lawful long branch or configured nickname can produce an over-limit notice.

EVIDENCE: metasystem/plans/channel-landing-notice-design.md:328-353 inserts machine and branch verbatim and asserts the header always fits. metasystem/internal/goal/actor.go:21-27 accepts any nonempty trimmed machine nickname. metasystem/scripts/agents/sync-transport.sh:20-29 validates branch characters but sets no length bound. Proof row 3 uses ordinary synthetic values and does not exercise maximum metadata lengths.

## Critic-declared gaps (verbatim)

- No Go tests or live provider calls were run because the brief required a read-only critique and explicitly prohibited running Go. Timing, crash, cancellation, and lost-ack behavior was therefore assessed from the design and source code rather than execution.
