Working Mode: design
Orchestrator Identity: m1 (lineage main-1788594343-3833-fb64b9, dispatch delegate under goal channel-tells-me-when-something-lands, tier 3 DESIGN-BEARING)
Date: 2026-09-06

# Goal

Revision 2, nine-finding fold of
metasystem/plans/channel-landing-notice-design.md (revision 1 landed at
fe5c2937b). Register: metasystem/records/misc/channel-landing-notice-critique-r1.md,
landed; it carries the critic's exact claims and evidence, and they bind.
Fold each finding BY ID. One of them is not yours to decide; see below.

# The folds, by id

- CLN-R1-EXACTLY-ONCE-ACK: post-then-mark with no idempotency key is
  at-least-once. Either the design promises at-least-once and says how a
  duplicate is made harmless to the reader, or it defines a landing
  idempotency key the provider can honour (Telegram and Slack both) and
  says what happens when a response is lost after acceptance. Do not
  promise exactly-once you cannot deliver.
- CLN-R1-HANG-ORPHAN: the shell timeout must stop the notifier, not just
  stop waiting for it - kill the child, release the channel lock - and the
  prompt path's wait must be short enough that a failed or hung post
  provably does not slow the landing; twenty seconds inside the landing is
  the thing the goal forbids. The fake provider only stalls on its
  long-poll, not on Post; say what the proof row uses.
- CLN-R1-REF-MOVE-KEY: the new-tip commit id is not the identity of a ref
  move. Define the key from what a push actually is (remote, ref, old tip,
  new tip, and the pushing machine) and show that two branches ending at
  one commit, a forced rollback, and a replay in a second checkout each
  produce the right number of notices.
- CLN-R1-RETRY-OWNER: name who retries a pending record when the pusher
  has no provider, no steward, or a lost checkout - a shared owner, or an
  explicit precondition with accepted loss, stated as such.
- CLN-R1-LANDING-SCOPE: NOT YOURS TO DECIDE. Three cases conflict - raw
  pushes of features to main outside any landing script, paper prose
  pushes, and announcing before the transport sync has succeeded. Write
  Decision 1 as: the three cases named, what each would mean for the
  reader, a PROPOSED default (landing scripts only; announce after the
  origin push succeeds, before transport sync, because origin is what the
  fleet reads; paper excluded as not a fleet delivery), and a clearly
  marked line "awaiting Wido's word" so the build cannot proceed on the
  default silently.
- CLN-R1-CREATE-IF-ABSENT: the create-if-absent point moves inside the
  lock, and the record's durability claim is reduced to what writeDurable
  actually provides, or the design names the sync it needs.
- CLN-R1-STATUS-COUNT: the count line either counts what the reader can
  check one-for-one against notices, or the design drops it and says how
  the digest and the notice avoid saying the same thing twice; and the
  digest must still change when the count does not.
- CLN-R1-NEW-BRANCH-CONTENT: a new branch's notice lists what a push
  added, not only its tip, consistent with Decision 1's content rule.
- CLN-R1-RUNE-BOUND: bound the machine nickname and the branch name in the
  renderer, or state the proven maximum of each from the code.

Consistency pass; self-grade; reject condition restated. Bump the revision
header to 2 with today's date.

# Constraints

Wall-clock budget: 25 minutes. The nine folds only; no other decision
moves. Read metasystem/internal/channel/report.go,
metasystem/internal/channel/phase/phase.go,
metasystem/internal/channel/telegram/telegram.go,
metasystem/internal/channel/slack, metasystem/internal/channel/fake and
metasystem/scripts/agents/land.sh before writing.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/channel-landing-notice-design.md (that one file).

# Gap Rule

Stop and report a gap; never fill it silently.
