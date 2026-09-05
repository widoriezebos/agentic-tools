# Design: a landing tells Wido the moment it lands

Goal: channel-tells-me-when-something-lands
(plans/goals/channel-tells-me-when-something-lands.md, revision 4, tier 3
DESIGN-BEARING). Author: implementer delegate under dispatch by m1 (lineage
main-1788594343-3833-fb64b9), job channel-landing-notice-design-r1b-20260906.
**Revision 1, 2026-09-06.** Every seam cited below was read in this worktree
at commit bfcef805e; line numbers are that commit's. The git facts about
2026-09-05 were read from `origin/main` in this worktree.

Wido's words, 2026-09-05: "the moment something lands, I want a message of
that."

## The defect, restated against the code

The channel is a digest being asked to be a notifier.

- `internal/channel/phase/phase.go:171-215` (`Run`) is the only production
  poster. It runs once per steward tick, after `Poll`, composes a status with
  `ComposeReport` (line 192) and posts only when `ShouldPost` agrees (line
  203).
- `internal/channel/report.go:259-261` (`ShouldPost`) is an AND: the
  interval (`channel.status.interval-minutes`, default 240 at `phase.go:197`)
  must have elapsed AND the content digest must differ. A landing inside the
  window waits for the window.
- `channel status --post` (`cmd/metasystem/channel_verbs.go:47-91`) posts a
  status on demand; its only callers are the channel fixture
  (`scripts/agents/channel-fixtures.sh:59,109`) and a human at a terminal,
  which is how Wido's two landings were finally announced at 11:05Z.
- `scripts/agents/land.sh:342-364` performs the landing: the push loop at
  342-360 moves origin, then `sync-transport.sh` mirrors. Nothing in that file
  touches the channel. `scripts/agents/commit.sh:550-568` is the second push
  site (`--push`, the paper editor's landing path); it touches the channel no
  more than land.sh does.
- The steward tick cadence is ten minutes by default
  (`internal/steward/runner.go:53-63`), and the tick's channel call carries a
  fifteen-second context (`runner.go:136-140`).

What origin saw on 2026-09-05 between 12:00 and 13:30 local (read with
`git log origin/main --format='%h %ci %s' --since ... --until ...`): two
goal-stamped landings, `b52711d3a` at 12:43:23 and `5cf0acaa6` at 13:01:24,
each carrying a `Goal-Item` trailer, and fourteen goal-ledger commits around
them (`goal open`, `goal claim`, `goal approve`, `goal done`, `goal
set-budget`, `goal slice-start`, `goal release`, `goal edit`) carrying none.
The ledger commits are bookkeeping the goal engine pushes itself; the two
stamped commits are what Wido meant by "something lands". Recent main also
carries landings without a trailer that are still landings (the critique
record commits `bfcef805e`, `66702fdcc`, `789acb343`, `12ed490c3`, all landed
through land.sh without `--goal`). Any definition that keys on the trailer
alone misses them; any definition that keys on "every commit on origin/main"
announces the bookkeeping.

## Decision 1 — what a landing is: a push by a landing script, not a commit on origin/main

**A landing is one successful push to origin performed by a landing script
(`land.sh` or `commit.sh --push`), and its content is exactly the commits
that push added to the origin branch: the range `<old>..<new>` that git
itself reports for the moved ref.**

`landingLines` (`report.go:165-194`) is reused for two things and rejected
for three, each with its reason.

Reused:

1. **The goal identity of a commit is its `Goal-Item` trailer**, read with the
   same format string `%(trailers:key=Goal-Item,valueonly)` (`report.go:166`),
   stamped by `commit.sh --goal` alone (`commit.sh:524,528-542`: exactly one
   byte-exact trailer or the commit is rolled back). The notice never
   guesses a goal from a subject.
2. **The goal is rendered as `featureName(id)`** (`report.go:132-134`,
   hyphens to spaces), so the notice and the status name the same goal the
   same way.

Not reused:

1. **Not the time window.** `landingLines` selects commits by
   `--since=<window start>` (`report.go:196-198`), where the window start is
   the last status post (`report.go:43-48`). A window is what let two
   landings sit unannounced; a window boundary is also where a landing is
   reported twice or never (a commit dated in the past by a rebase falls
   before the window start; `landingSince` sends the author-facing commit
   date, and land.sh's own rebase at `land.sh:340` rewrites commits whose
   committer date moves but whose author date does not). The notice's set is
   the ref move the push performed, which is exact, ordered, and known to the
   pusher and nobody else.
2. **Not the known-goal filter.** `landingLines` drops a commit whose
   `Goal-Item` names no goal in the projection's Live or Done trees
   (`report.go:177`), and drops every commit with no trailer. The status is a
   digest by goal and may do that; the notice reports what moved on origin.
   A critique record landed without `--goal` is still a landing Wido asked to
   hear about. The trailer enriches the line when present and is absent
   otherwise.
3. **Not the per-goal collapse.** `landingLines` keeps one subject per goal
   (`report.go:178-180`). A push of three commits for one goal is three lines
   in the notice, bounded by Decision 4.

Consequences stated plainly:

- The goal ledger's own publishes (`goal open`, `goal claim`, and the rest,
  pushed by `goal.Publish` from inside the engine, never through land.sh) are
  not landings and produce no notice. The status already carries goal state
  through its `Needs you` and `Next up` lines.
- A raw `git push` by a human or an agent outside the two landing scripts
  produces no notice. This is a named residual (Decision 6), not a silent
  omission: the four-hourly status still digests it through `landingLines`.
- A push to a branch other than `main` (the `paper` branch through
  `commit.sh --push`) is a landing on that branch and is announced naming the
  branch. Today's `landingLines` reads only `origin/main`; that stays as it
  is, so paper landings appear in notices and not in the status digest. This
  is deliberate: the status is the fleet's delivery digest, and paper edits
  are not fleet deliveries.

## Decision 2 — where the trigger lives: the pusher announces, the steward retries

**The trigger fires in the landing script, straight after the push step
reports success and before the transport sync. It calls one new engine verb
that first writes a durable landing record, then attempts one post under a
hard time bound. What the verb could not post, the steward tick's channel
phase retries from the record. The verb's exit status is printed and never
propagates.**

Why the pusher and not a tick sweep: the pusher is the only party that knows
the exact ref move (Decision 1) and the only party that performed it, so
"exactly once across machines" (Decision 3) falls out of the trigger's
location instead of needing a fleet-wide cursor. A sweep on the tick would
arrive up to ten minutes late against "the moment", would have to
reconstruct the range from a per-machine cursor over `origin/main` that
every machine advances independently, and would have to filter the goal
ledger's bookkeeping commits back out. The tick keeps one role: retrying
records the pusher left pending.

### The verb

`metasystem channel landing-notice --root <repository root> --branch
<branch> --range <old>..<new> [--new-branch <new>]`, registered beside
`status` in the `channel` family table (`cmd/metasystem/main.go:395-398`).
Its steps, in order, each a mechanical rule:

1. **Read the switch** (Decision 5). `channel.landing-notice` off: print
   `landing notice off` and exit 0. Nothing is recorded.
2. **Resolve the range.** `git rev-parse --verify <old>^{commit}` and
   `<new>^{commit}` with the git environment scrub `reportGitEnv`
   (`report.go:200-216`) already used by `landingLines`; then
   `git rev-list --reverse --topo-order <old>..<new>`. An empty list is a
   refusal (exit 2, `nothing landed in <old>..<new>`); a resolution failure
   is exit 2 naming the id. `--new-branch <new>` (the porcelain flag `*`)
   lists the single commit `<new>`.
3. **Compose the record** (`Landing`): `{id: <new full sha>, branch,
   old, new, pushedAt (UTC RFC3339, the verb's own clock), machine
   (goal.ResolveMachine), commits: [{sha, subject, goalItem}], state:
   "pending", attempts: 0, ref: null, lastError: ""}`. Subjects come from
   `git log --format=%s%x00%(trailers:key=Goal-Item,valueonly)` over the
   list, the same format `landingLines` uses.
4. **Write the record durably** to
   `artifacts/agents/channel/landings/<new full sha>.json` through
   `writeDurable` (`question.go:90-110`: temp file, fsync, rename) with
   create-if-absent semantics: if the file already exists, the verb prints
   `already recorded` and skips to step 6 without rewriting it. The rename is
   the commit point of the record. A write failure is exit 1 with the error
   and **no post is attempted**: a notice with no record cannot be retried,
   cannot be proven, and cannot be deduplicated, so the ledger stays the
   truth and the failure is loud in the landing output (Decision 3 says why
   this is the right side of the trade).
5. **Load the provider** with `phase.Load(root, false)`; unconfigured
   channel (`Provider == nil`): the record stays `pending` with `lastError:
   "no channel configured"`, exit 0 after printing so. The status's
   `Undelivered` count already covers this state on a machine whose channel
   is later configured.
6. **Post under the lock and the bound** through the shared function
   `channel.PostPendingLandings(ctx, cfg)` (below), with a context deadline
   equal to `channel.poll-timeout-sec` (default 15, the same key and parse as
   `channelPollContext`, `channel_verbs.go:33-45`). Print one line per record
   posted or left pending. Exit 0 if every record is posted, exit 3 if any
   stays pending. The exit code is information for the landing output only.

### Shared drain: `PostPendingLandings`

One function in `internal/channel`, used by the verb and by the steward tick:

1. Take the channel lock `artifacts/agents/channel/lock`, the same file
   `Poll` takes (`poll.go:56-67`). `Poll` uses `LOCK_EX|LOCK_NB` and reports
   `Busy`. The drain instead retries `LOCK_NB` every 100 ms until it holds the
   lock or the context deadline passes; on deadline it returns `Busy` and
   posts nothing. The lock is released on return.
2. List `landings/*.json`, parse each, keep `state == "pending"`, sort by
   `pushedAt` then `id`. Oldest first, so a backlog after an outage arrives
   in landing order.
3. For each pending record while the context is live: render (Decision 4),
   `Provider.Post(ctx, dest, text, nil)` top-level, never threaded. On
   success: `state: "posted"`, `ref`, `postedAt`, rewritten through
   `writeDurable`. On failure: `attempts++`, `lastError` scrubbed with
   `Scrub` and the destination's secrets (`channel.go:77-84`), rewritten;
   continue to the next record only if the error is not a context deadline
   (a deadline ends the pass).
4. Delete `posted` records whose `postedAt` is older than 7 days. Nothing
   else is ever deleted.
5. Return counts `{posted, pending, busy}`.

The steward path: `phase.Run` (`phase.go:171-215`) calls
`PostPendingLandings` after `Poll` returns and before `ComposeReport`, under
the same 15-second tick context (`runner.go:136`). Its `pending` count is
added to the `Undelivered` figure the status carries (`phase.go:192`,
`report.go:100-106`), so a landing that could not be announced is visible in
the next status as an undelivered message rather than nowhere. `channel
poll` (`channel_verbs.go:224-260`) gains the same call so the fixture can
drive a retry without a live steward.

### The prompt path's bound

The prompt path is the time land.sh spends between "push succeeded" and
"transport sync starts". Its bound has two layers, both mechanical:

| Layer | Bound | Owner |
| --- | --- | --- |
| HTTP request | `channel.http-timeout-sec`, default 30, per request (`phase.go:65-76`, `telegram.go:68-73`) | provider |
| Whole verb | `channel.poll-timeout-sec`, default 15, as a context deadline over lock wait plus every post; the provider request honors it (`telegram.go:72,79`) | verb |
| land.sh step | `channel.poll-timeout-sec + 5` seconds of wall clock, after which the step stops waiting | landing script |

The verb's deadline is expected to hold: a Telegram post is one HTTP round
trip of well under a second, and the drain checks the context before each
post. The third layer exists for the case the brief names: a post that hangs
rather than fails, through an engine defect or a provider that ignores the
deadline. It is implemented in the landing script without any process kill
(the shared-machine rule forbids killing what is not provably yours, and the
child is left to finish or die on its own deadline):

```bash
# land.sh: advisory step, runs between the push loop and the transport sync.
notify_landing() { # summary-field
  local range=$1 flag=$2 args=(--root "$root" --branch "$branch")
  case "$flag" in
    ' ') args+=(--range "$range") ;;
    '*') args+=(--new-branch "$(git rev-parse "refs/heads/$branch")") ;;
    *) printf -- '-- landing notice skipped: push flag %q moved nothing to announce\n' "$flag"; return 0 ;;
  esac
  "$ms" channel landing-notice "${args[@]}" &
  local pid=$! waited=0 bound=$((notice_timeout + 5))
  while kill -0 "$pid" 2>/dev/null && (( waited < bound )); do sleep 1; waited=$((waited + 1)); done
  if kill -0 "$pid" 2>/dev/null; then
    printf -- '-- landing notice still running after %ss; left to finish, the steward retries what it did not post\n' "$bound"
    return 0
  fi
  wait "$pid"
  local rc=$?
  (( rc == 0 )) || printf -- '-- landing notice exit %s (landing unaffected):\n' "$rc"
  return 0
}
```

`notice_timeout` is read once with `"$ms" config get --key
channel.poll-timeout-sec --default 15 --conf "$root/metasystem.conf"`
(`cmd/metasystem/config_verbs.go:94-101`, the same resolver `phase.Get` uses)
before the step, falling back to 15 if that read fails or returns anything
but a positive integer. The
step is run through a new `run_advisory_step` that mirrors `run_step`
(`land.sh:147-159`) but prints the step's output inline with a `--` prefix
and returns 0 regardless. The summary field and flag come from the push
step's own captured output: `push_origin` (`land.sh:319-321`) already runs
`git push --porcelain`, whose success line for the pushed ref is
`<flag>\t<from>:<to>\t<summary>`. The parser takes the line whose `<to>` is
`refs/heads/$branch`, reads the one-character flag at column one, and takes
the summary field: `<old>..<new>` for a fast-forward (flag space), `[new
branch]` for flag `*`, `[up to date]` for flag `=`. A forced push (flag
`+`) cannot occur: land.sh never passes `--force`. Abbreviated ids in the
summary are resolved by the verb (step 2); both objects are local because
land.sh fetched origin's tip at `land.sh:311-313` and committed the new one.

`commit.sh --push` (`commit.sh:555`) changes its push to
`LC_ALL=C git -C "$root" push --porcelain origin "refs/heads/$branch:refs/heads/$branch"`,
captures the output, keeps its existing failure exit, and calls the same
`notify_landing` shape before its transport sync. The parsing function moves
to a small shared file `scripts/agents/landing-notice.sh` sourced by both
scripts, so the two push sites cannot drift.

### What a post that fails does to land.sh

Nothing. `notify_landing` returns 0 on every path; `run_advisory_step`
returns 0 on every path; neither is wrapped in `run_required_step`. The
transport sync that follows keeps its `run_required_step` and its exit codes
exactly as today. A hung post costs the landing at most
`channel.poll-timeout-sec + 5` seconds and leaves a `pending` record that the
next steward tick posts.

## Decision 3 — exactly once across machines: the ref move is the identity

**The unit of announcement is a ref move on origin, identified by the full
id of the new tip. A ref move happens once, on one machine, in one checkout,
so the machine that performed it is the only one that ever holds a record
for it. The record is written before the post and is the only thing a retry
reads. There is no cross-machine cursor because there is nothing to
share.**

The brief's proposed cursor beside `fleet/cursor.json` (`poll.go:138`) is
rejected: a cursor is a position in a stream, and the landing stream has no
single reader. Every machine's `origin/main` advances by everyone's pushes,
so a per-machine cursor announces other machines' landings, and a shared
cursor would have to live on origin and be advanced under a network lock,
which puts the network on the landing's prompt path twice. The attack list
from the brief, answered against the record design:

| Attack | What happens | Why it holds |
| --- | --- | --- |
| Two checkouts of one repository on one machine | Each checkout has its own `artifacts/` (`metasystem/.gitignore:1`, so it is never shared through git) and its own lock. Checkout A pushes `X..Y` and records `Y`; checkout B later rebases onto `Y` and pushes `Y..Z`, recording `Z`. Two records, two notices, no overlap. | A fast-forward push's summary is the exact range origin moved by; two pushes cannot move the same ref over the same range. Both land.sh (`land.sh:311-317`) and the porcelain summary make `old` origin's tip at push time, not the local tracking ref's stale idea. |
| A push during a tick's sweep | There is no sweep of commits. The tick's drain reads `landings/*.json` under the lock; a push in flight is writing a new record by atomic rename (`writeDurable`), so the drain sees either no file or a complete `pending` file. If the drain holds the lock, the pusher's verb waits up to its deadline; if it times out, the record stays `pending` and the next tick posts it. | The lock serializes posting, the rename serializes visibility, and `state` on the record is the single truth both sides read after taking the lock. |
| A push whose record write fails | The verb exits 1 before posting; land.sh prints the error and continues; the landing is not announced and no retry exists for it. The next status's `Delivered` line (goal-stamped) or nothing (unstamped) is what Wido sees. | Chosen over "post without a record" because at-most-once and provability both need the record to exist before the message. The alternative would announce and leave no trace to test against. Named as residual risk (Decision 6). |
| A rebase that changes commit ids after announcement | The notice names the ids that reached origin. land.sh's own rebase runs before the push (`land.sh:340`), so announced ids are post-rebase. A later local rebase changes nothing on origin. A force-push that rewrites origin's history is outside both landing scripts (neither passes `--force`); if a human does it and re-lands the same content, the new tip is a new ref move and is announced again, which is true: it landed again. | The record's identity is origin's new tip, not a commit's content or its pre-rebase id. |
| A machine that fetches another machine's landings | Fetching moves `origin/main` locally and writes nothing under `landings/`; the fetcher announces nothing. Its status digest still lists goal-stamped landings from every machine, as today. | Only a push writes a record; only the pusher pushes. |
| The same push replayed (land.sh rerun after a partial failure, or the verb run twice by hand) | Step 4's create-if-absent finds the record and skips to the drain, which posts only if `state` is still `pending`. | One file per new tip; `state` transitions `pending -> posted` once under the lock. |
| Two machines land at the same second | Origin serializes ref updates; the second push is rejected non-fast-forward (`land.sh:323-325`), rebases, and pushes a different range. | Git's ref update is the atomic point; land.sh already relies on it. |
| A posted record lost (disk failure after post) | The message stands in the channel; a rerun of the same push cannot happen (origin has moved on). Nothing re-announces. | The record's absence can only recreate a post if the same range is pushed again, which git prevents. |

Retention: `posted` records older than 7 days are deleted by the drain
(mechanical rule, Decision 2 step 4). `pending` records are never deleted by
the engine; a record that stays `pending` for a day is a channel outage the
status's `Undelivered` line already reports.

## Decision 4 — the message

**One notice per landing (per push). One line per commit inside it, oldest
first, within the same 1600-rune bound the ask uses.**

The bound is the constant `questionMessageRuneLimit` (`question.go:74`,
landed in b52711d3a). The implementer renames it to `messageRuneLimit` in
`question.go` and uses it from the notice renderer; no second constant, no
copy. The Telegram provider's 4000-rune chunking (`telegram.go:17,152-178`)
stays as the transport safety net and is never relied on.

Rendering rules for `RenderLanding(l Landing, loc *time.Location) string`:

1. **Time** is `pushedAt` (the landing moment, not the posting moment, so a
   retried notice still says when it landed) in the machine's zone,
   `time.Local` by default, the same rule `ComposeStatusReport` applies to
   the status header (`report.go:40-42`), formatted `2006-01-02 15:04 -0700`
   like the status header (`report.go:91`). One format, one zone rule, for
   every human-facing time on the channel.
2. **Single commit** (the common case), one line:
   `<machine> landed <time>: <subject> — <goal name>` where `— <goal name>`
   is `featureName(goalItem)` and is omitted when the trailer is absent.
   For a branch other than `main`, ` on <branch>` follows `landed`.
   Example: `m1 landed 2026-09-05 12:43 +0200: An ask fits one message and
   keeps its token — channel ask fits one message`.
3. **Several commits**, a header and a line per commit:
   `<machine> landed <n> commits <time>[ on <branch>]:` then `- <subject>[ —
   <goal name>]` per commit, oldest first.
4. **Bound.** Each subject is trimmed to 200 runes with the same
   `trimQuestionPart` ellipsis rule (`question.go:344-353`). Lines are added
   in order while the total including a possible tail line stays within
   `messageRuneLimit`; when the next line would not fit, the tail
   `- … and <k> more, <old7>..<new7> on origin` is appended, where `<old7>`
   and `<new7>` are the seven-character ids and `k` is the count of commits
   not listed. A push of twenty commits with ordinary subjects lists roughly
   the first fifteen and names the rest by range. The header alone always
   fits (its longest form is under 120 runes).
5. **Never threaded.** A reply to a notice is not a command; `Poll` matches
   inbound replies to question threads and the status thread only
   (`poll.go:109-137,160-177`), so a reply to a notice lands in
   `unmatched.jsonl` as it does for any unrecognized message. The design adds
   no notice thread to that match, on purpose: the status thread carries the
   `start <goal-id>` token and nothing else should look like it.

## Decision 5 — the status stays a digest; the two relate by one count line

**The four-hourly status keeps its interval, its digest semantics, its
`ShouldPost` AND, and its `Delivered` lines. When landing notices are on at
the reporting machine, the status replaces its per-landing `Delivered` lines
with one line: `Landed: <n> since last status, each announced when it
landed`. When notices are off, the status is byte-for-byte what it is
today.**

Why a count and not silence: Wido can check the number against the notices
he received; a status that says nothing about landings makes a lost notice
invisible. Why a count and not the list: the list is the wall of text by
another door the brief names. Why the switch decides: a machine with notices
off has no other way to tell him what landed, so it keeps today's lines.

Mechanics:

- `n` is the number of goal-stamped commits `landingLines` finds
  (`report.go:165-194`), unchanged in what it counts, so the count line and
  the old `Delivered` lines agree on the same set. The line is omitted when
  `n == 0`. It occupies one line of the 12-line cap
  (`report.go:87-99`) in the `delivered` slot.
- The status's `Undelivered` figure includes `pending` landing records
  (Decision 2), so `Landed: 2 since last status` beside `Undelivered: 1
  channel messages` tells him one of the two did not reach him yet.
- `ShouldPost` is untouched. The count line changes the digest exactly as
  the `Delivered` lines did, so the status posts at its interval when
  landings occurred and stays quiet when nothing changed.
- No notice ever resets the status interval, and no status ever marks a
  landing posted. The two ledgers (`status.json`, `landings/*.json`) do not
  read each other except through the `Undelivered` count.

The off switch: `channel.landing-notice` in `metasystem.conf` or
`metasystem.conf.local`, read through `phase.Get` with default `on`. Accepted
values are exactly `on` and `off`; any other value is a configuration error
that the verb (exit 2) and `phase.Run` (returned error, printed by the tick
at `runner.go:138`) both name. Off means: the landing verb records nothing
and posts nothing (Decision 2 step 1); the drain still posts records that
already exist, so switching off never strands a pending record; the status
keeps its per-landing `Delivered` lines.

## Decision 6 — proof

Every obligation below names the fixture that proves it. Two fixture homes,
matching what exists: Go tests in `internal/channel` for rendering and
record state (the pattern of `channel_test.go:600-620`), and the land and
channel shell fixtures for the script seams (`land-fixtures.sh` legs against
a local bare origin, `channel-fixtures.sh` against the fake provider whose
journal records every `sendMessage` and `chat.postMessage`,
`fake.go:193,234`).

| # | Obligation | Fixture | Home |
| --- | --- | --- | --- |
| 1 | A fast-forward push through land.sh produces exactly one `sendMessage` in the fake journal before land.sh exits, naming the machine, the local time, the subject and the goal name | New land leg `notice`: bare origin plus fake provider configured in the leg's `metasystem.conf.local`; land one goal-stamped commit; assert one journal entry and one `posted` record whose `id` is origin's new tip | land-fixtures.sh |
| 2 | A landing without a `Goal-Item` trailer is announced without a goal name | Leg `notice`, second landing without `--goal`; assert the line has no ` — ` goal suffix | land-fixtures.sh |
| 3 | A push of twenty commits becomes one message within 1600 runes, oldest first, with the tail naming the range and the count | Go test: `RenderLanding` over a synthetic 20-commit record with 150-rune subjects; assert rune count, first line, tail line, and that the listed subjects are the first `k` in order | channel_test.go |
| 4 | The 1600 bound is the ask's constant, not a copy | Go test asserts `messageRuneLimit == 1600` from one identifier used by both `renderQuestion` and `RenderLanding`; a grep guard in the test fails if a second literal `1600` appears in `internal/channel` outside the constant | channel_test.go |
| 5 | The time is the push time in the local zone, and a retried notice still shows the push time | Go test: record with `pushedAt` fixed, `loc` fixed to a non-UTC zone; render at a later `now`; assert the formatted time | channel_test.go |
| 6 | A post that fails leaves land.sh's exit status untouched and leaves a `pending` record | Leg `notice-fails`: fake provider `api-base` pointed at a closed port; land; assert exit 0 from land.sh, `state: "pending"`, `attempts: 1`, `lastError` non-empty, and the transport sync step ran | land-fixtures.sh |
| 7 | A pending record is posted by the next channel pass and marked `posted` once | Leg `notice-fails` continues: repoint the fake `api-base` at the live fake, run `channel poll`; assert one journal entry, `state: "posted"`; run `channel poll` again; assert still one entry | land-fixtures.sh |
| 8 | A post that hangs bounds the landing to `poll-timeout + 5` seconds and does not fail it | Leg `notice-hangs`: `api-base` at a listener that accepts and never answers (a `nc -l` or the fake with a new `hang` mode, whichever the fixture bed already has; if neither, this row is residual); `channel.poll-timeout-sec=2`; assert land.sh exits 0 within the scaled fixture ceiling and prints the "still running" or "exit 3" line | land-fixtures.sh |
| 9 | The record is written before any post is attempted, and a record write failure posts nothing | Go test with a `Provider` fake that counts `Post` calls and a read-only `landings/` directory: assert zero posts and a returned error | channel_test.go |
| 10 | Two checkouts of one repository on one machine announce disjoint ranges once each | Leg `notice-two-checkouts`: clone the bare origin twice, land from A then from B (B rebases onto A's tip); assert two journal entries whose ranges do not overlap and two records with distinct ids | land-fixtures.sh |
| 11 | Replaying the verb on the same range posts nothing new | Leg `notice`: run `channel landing-notice` by hand with the same `--range`; assert `already recorded` and one journal entry | land-fixtures.sh |
| 12 | A fetching machine announces nothing | Leg `notice-two-checkouts`: after A lands, B fetches and runs `channel poll`; assert B has no records and the journal count is unchanged | land-fixtures.sh |
| 13 | The drain returns `Busy` and posts nothing when the lock is held past its deadline | Go test: hold the flock from the test, call `PostPendingLandings` with a 200 ms context; assert `busy == true`, zero posts, record still `pending` | channel_test.go |
| 14 | With notices on, the status carries the `Landed: n since last status` line and no per-landing `Delivered` lines; with notices off, the status is today's text | Two Go tests beside `TestReportShowsOneQuestionTwoLandingsAndOnlyTwoNextItems` (`channel_test.go:291-319`): one with the switch on asserting the count line, one with it off asserting the existing expected text unchanged | channel_test.go |
| 15 | `Undelivered` includes pending landing records | Go test: one `pending` record, `phase.Run` against the fake provider with posting disabled for landings; assert the status's `Undelivered` line counts it | phase_test.go |
| 16 | The off switch records and posts nothing, and an invalid value is refused by name | Leg `notice-off`: `channel.landing-notice=off`, land, assert no record and no journal entry; then `=maybe`, run the verb, assert exit 2 naming the key | land-fixtures.sh |
| 17 | `commit.sh --push` announces through the same path | Leg `notice-commit-push`: `commit.sh --push` on a branch named `paper`; assert one journal entry containing ` on paper` | land-fixtures.sh |
| 18 | Goal-ledger publishes produce no notice | Leg `notice`: `goal open` then `goal claim` against the leg's origin; assert the journal count is unchanged | land-fixtures.sh |
| 19 | `posted` records older than 7 days are deleted, `pending` never | Go test with backdated `postedAt`; assert deletion of the posted one and survival of the pending one | channel_test.go |
| 20 | The porcelain parser takes the right line, flag and summary, including `[up to date]` and `[new branch]` | Shell unit inside the land fixture: feed three captured porcelain outputs to the parsing function; assert range, flag, and the skip line | land-fixtures.sh |

Residual risks, named rather than claimed:

- (a) A raw `git push` outside the two landing scripts is not announced.
  The status digest covers goal-stamped ones four-hourly; unstamped ones are
  unreported. Closing it would need a server-side hook on origin, which the
  fleet does not own.
- (b) A record write failure leaves a landing unannounced with no retry; the
  landing output says so at the terminal, and nothing else does.
- (c) Row 8 (a hanging provider) depends on a hang-capable listener in the
  fixture bed; if none exists and adding one exceeds the build's appetite,
  the shell-side bound is proven only by reading, and the Go deadline (row 13
  proves the lock deadline; the HTTP deadline is the provider's existing
  contract at `telegram.go:72`) carries the claim.
- (d) `commit.sh --push` today does not fetch before pushing; the porcelain
  summary is still origin's authoritative old tip, so the range is exact,
  but a stale local tracking ref means the abbreviated `<old>` may name an
  object the local repository does not have if it was never fetched. Git
  reports the remote's old value, and a fast-forward push implies the local
  repository has every commit from old to new, so the object is present; this
  reasoning is by reading, not by a fixture with a deliberately stale
  tracking ref. Row 17 covers the ordinary case.
- (e) The Slack face renders the same text; no Slack-specific fixture is
  listed because the fake's Slack face is journal-equivalent and row 1
  exercises the rendering once. Telegram is the fleet's live provider.
- (f) Clock skew between machines makes the `pushedAt` in a notice the
  pusher's clock, as the status header is the poster's clock today.

## Consistency pass

- Decision 1's "landing is a push" is what Decision 3's identity (origin's
  new tip) and Decision 2's trigger (the push site) both rest on; none of the
  three works with a commit-on-origin/main definition.
- Decision 2's record-before-post is what Decision 3's replay and
  write-failure rows and Decision 6 rows 9 and 11 assume.
- Decision 4's use of `pushedAt` is why a retried notice from Decision 2's
  drain still says when the landing happened.
- Decision 5's count line reads the same `landingLines` set as today's
  `Delivered` lines, so the `n` in the status and the goal-stamped notices
  Wido received describe the same commits; unstamped landings are announced
  (Decision 1) but not counted in the status, which is the one asymmetry,
  stated here so nobody reads the count as "every notice".
- Every configuration key named exists today except `channel.landing-notice`,
  which Decision 5 introduces with its default and its accepted values.
- The `plans/` guard: this design is a new plan file and lands with
  `--allow-new-plan` (`land.sh:42-45,327-332`); the implementer's product
  touches no plan file.

## Self-grade

Grounding: every load-bearing claim is a file-and-line read in this worktree
at commit bfcef805e (`report.go` whole, `phase.go` whole, `poll.go` whole,
`question.go` whole, `channel.go` whole, `telegram.go` whole, `inbox.go`
whole, `channel_verbs.go` whole, `land.sh` whole, `commit.sh:33-35,524-568`,
`sync-transport.sh` whole, `runner.go:53-63,97-149`, `main.go:395-398`,
`channel-fixtures.sh:1-140`, `land-fixtures.sh:1-40`, `fake.go:193,234`,
`metasystem/.gitignore`), or a git query run here on 2026-09-06 (`git log
origin/main` for the 2026-09-05 window and the newest 25 commits with their
`Goal-Item` trailers; `git --version` 2.50.1). The porcelain summary format
(`<old>..<new>` for a fast-forward, `<old>...<new>` for a forced update,
`[new branch]`, `[up to date]`) is git's documented `--porcelain` contract
and was not exercised against a live push here; row 20 pins it. Nothing in
this design was executed against a provider. Grade: pass against everything
observed; the reject condition below is the falsifier the implementation and
its critique must actively test.

**Reject condition — reject this design if any of the following is shown:**
a path on which two machines, or two checkouts, both hold a record for the
same origin ref move (exactly-once broken at the source); a landing through
`land.sh` or `commit.sh --push` whose successful push produces neither a
`sendMessage` within the bound nor a `pending` record (the 2026-09-05
silence recreated); any path on which a post is attempted before its record
is durably renamed into place; any exit of `land.sh` or `commit.sh --push`
whose status changes because of the notice step, including a hung provider;
a prompt path longer than `channel.poll-timeout-sec + 5` seconds on a hung
provider; a notice longer than the one shared `messageRuneLimit`, or a second
literal for that bound anywhere in `internal/channel`; a goal-ledger publish
(`goal open`, `goal claim`, and the rest) that produces a notice; a status
that, with notices on, still itemizes `Delivered` lines, or, with notices
off, differs by a byte from today's text; a `pending` record the drain
deletes; a retried notice that shows the posting time instead of the landing
time; any drain that posts without holding the channel lock; or a switch
value other than `on` and `off` that is accepted silently.
