# Design: a landing tells Wido the moment it lands

Goal: channel-tells-me-when-something-lands
(plans/goals/channel-tells-me-when-something-lands.md, revision 6, tier 3
DESIGN-BEARING). Author: implementer delegate under dispatch by m1 (lineage
main-1788594343-3833-fb64b9); revision 1 by job
channel-landing-notice-design-r1b-20260906, revision 2 by job
channel-landing-notice-fold2-20260906. **Revision 2, 2026-09-06.** Every seam
cited below was read in this worktree at commit 6ca7a0ac2; line numbers are
that commit's. The git facts about 2026-09-05 were read from `origin/main`
in this worktree for revision 1.

Wido's words, 2026-09-05: "the moment something lands, I want a message of
that."

## Revision 2: what changed and why

Revision 1 landed at fe5c2937b. Its critique
(records/misc/channel-landing-notice-critique-r1.md) raised nine material
findings. Each is folded below by id; the section named is where the fold
lives.

| Finding | Fold | Where |
| --- | --- | --- |
| CLN-R1-LANDING-SCOPE | Decision 1 now names the three conflicting cases, what each means for Wido, a proposed default, and a line awaiting his word. Not decided here. | Decision 1 |
| CLN-R1-NEW-BRANCH-CONTENT | A new branch's notice lists the commits the push made newly reachable on origin, not only the tip. | Decision 1, content rule; Decision 2 step 2 |
| CLN-R1-HANG-ORPHAN | The landing script kills its own notifier child at the bound (TERM, then KILL), the lock is released by the kernel on death, the bound is a new short key defaulting to 5 seconds, and the wait overlaps the transport sync. The proof rows say which layer each fixture proves. | Decision 2, prompt path; Decision 6 rows 8a and 8b |
| CLN-R1-CREATE-IF-ABSENT | Record creation moves inside the channel lock. The durability claim is reduced to what `writeDurable` provides: ordering against process death, not against power loss. | Decision 2, the verb, steps 5 and 6 |
| CLN-R1-RETRY-OWNER | The retry owner is the pushing machine's steward, stated as a precondition with the loss it accepts when that machine has no channel, no steward, or loses the checkout. | Decision 2, retry owner |
| CLN-R1-REF-MOVE-KEY | The record key is the ref move: remote, full ref, old tip, new tip, pushing machine. The verb refuses to run in a checkout whose remote-tracking reflog does not show it performed the push. Three cases worked. | Decision 3 |
| CLN-R1-EXACTLY-ONCE-ACK | The design promises at-least-once with a marked repeat, not exactly-once. Neither provider honours an idempotency key, so a lost acknowledgment is made harmless by a `posting` state and a repeat line on every retry. | Decision 3 |
| CLN-R1-RUNE-BOUND | The renderer bounds machine, branch and goal name, and the header maximum is computed from those bounds. | Decision 4 |
| CLN-R1-STATUS-COUNT | The count line is dropped. The status is byte-for-byte today's text, switch on or off, except that `Undelivered` counts unposted landing records. The overlap between digest and notice is deliberate and named. | Decision 5 |

## The defect, restated against the code

The channel is a digest being asked to be a notifier.

- `internal/channel/phase/phase.go:172-215` (`Run`) is the only production
  poster. It runs once per steward tick, after `Poll`, composes a status with
  `ComposeReport` (line 192) and posts only when `ShouldPost` agrees (line
  203).
- `internal/channel/report.go:259-261` (`ShouldPost`) is an AND: the
  interval (`channel.status.interval-minutes`, default 240 at `phase.go:197`)
  must have elapsed AND the content digest must differ. A landing inside the
  window waits for the window.
- `channel status --post` (`cmd/metasystem/channel_verbs.go:47-91`) posts a
  status on demand; its only callers are the channel fixture
  (`scripts/agents/channel-fixtures.sh:57,117`) and a human at a terminal,
  which is how Wido's two landings were finally announced at 11:05Z.
- `scripts/agents/land.sh:342-364` performs the landing: the push loop at
  342-360 moves origin, then `sync-transport.sh` mirrors. Nothing in that file
  touches the channel. `scripts/agents/commit.sh:547-565` is the second push
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

## Decision 1 — what a landing is: three cases, a proposed default, and Wido's word

**Proposed: a landing is one successful push to origin of `refs/heads/main`
performed by a landing script (`land.sh` or `commit.sh --push`). It is
announced after the origin push succeeds and before the transport sync
starts. Its content is exactly the commits that push made newly reachable
on origin: the range `<old>..<new>` git reports for the moved ref, or, for a
newly created branch, every commit reachable from `<new>` and from no other
remote-tracking ref of origin.**

**Awaiting Wido's word. The build cannot proceed on this default silently;
the three cases below need his answer, recorded on the goal, before the
implementer is dispatched.**

Three cases conflict, and the design records rather than decides them
(CLN-R1-LANDING-SCOPE). The human-approved wording on the goal
(`plans/goals/channel-tells-me-when-something-lands.md:20`) is "not sending
messages when new features land", and its Origin line names `main`.

| Case | If included | If excluded | Proposed |
| --- | --- | --- | --- |
| A raw `git push` to main outside both landing scripts | Every push to main is announced, including the goal ledger's own publishes (`goal open`, `goal claim`, fourteen on 2026-09-05), unless a sweep filters them; the sweep runs on the tick and arrives up to ten minutes late; the pusher is unknown, so the fleet needs a shared cursor. | A feature pushed raw is silent until the next four-hourly status (goal-stamped) or for good (unstamped). Wido hears nothing at landing time for a push that skipped the scripts. | **Excluded.** The scripts are the landing path the fleet is told to use; a raw push is already a rule break the status digest partly covers. |
| A `commit.sh --push` of paper prose (branch `paper`) | Every paper edit is a notice, one per commit, since the paper editor commits and pushes on every edit. That is a stream of prose landings in the same channel as fleet deliveries. | Paper landings produce no notice; Wido learns of them by reading the branch. | **Excluded.** Paper is not a fleet delivery. The renderer keeps its ` on <branch>` form so that including paper later is a one-line change in the branch rule below. |
| Announce before the transport sync, or only after it succeeds | Before: Wido hears "landed" the moment origin has it, and the landing script may still exit non-zero if the transport mirror fails; commit.sh's own words are "the landing is both remotes or it is not a landing" (`commit.sh:547-549`), and the VM validation lane reads transport, so a notice can precede the VM seeing the commit. After: no notice can precede a failed landing, and the notice waits for the mirror (a fetch plus a push, seconds), and a `--skip-transport` landing is announced at once either way. | | **Before, after the origin push succeeds.** Origin is what every machine's `origin/main` and the goal ledger read; the mirror is a copy. A transport failure is loud in the landing output and leaves origin as it was announced. |

Mechanics under the proposed default, each a rule the implementer applies
without deciding:

- The landing script calls the notice step only when the pushed branch is
  exactly `main`. Any other branch prints
  `-- landing notice: branch <branch> is not announced` and does nothing.
  Wido's word on paper changes this one condition.
- The step runs straight after the push step reports success and before the
  transport sync step starts (Decision 2). Wido's word on the transport case
  moves the launch to after the sync step; nothing else in the design moves.

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
   pusher.
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

Content rule for a new branch (CLN-R1-NEW-BRANCH-CONTENT): when the push
created the ref (porcelain flag `*`, summary `[new branch]`), the commits
the push added are those reachable from `<new>` and not from any other
remote-tracking ref of origin in the pushing checkout:
`git rev-list --reverse --topo-order <new> --not <every ref under
refs/remotes/origin/ except refs/remotes/origin/<branch>>`. With no other
tracking ref (a first branch on an empty remote) the list is everything
reachable from `<new>`. The remote-tracking refs are the pushing checkout's
view of origin at push time; a branch origin has that this checkout never
fetched can make the list longer than what origin actually gained, never
shorter, and Decision 4's tail line bounds the message either way. The
record's `old` for a created ref is the forty-zero id git itself uses for a
ref creation. Under the proposed default (`main` only) this case is
theoretical, since `main` exists; it is specified so that including another
branch later changes nothing here.

Consequences stated plainly:

- The goal ledger's own publishes (`goal open`, `goal claim`, and the rest,
  pushed by `goal.Publish` from inside the engine, never through land.sh) are
  not landings and produce no notice. The status already carries goal state
  through its `Needs you` and `Next up` lines.
- A raw `git push` by a human or an agent outside the two landing scripts
  produces no notice (case one above, proposed). The four-hourly status
  still digests goal-stamped ones through `landingLines`.
- A push to `paper` produces no notice (case two above, proposed).

## Decision 2 — where the trigger lives: the pusher announces, the pusher's steward retries

**The trigger fires in the landing script, straight after the push step
reports success. It launches one new engine verb as a child the script
owns, records its process id, and reaps or kills it at a short bound. The
verb writes a landing record under the channel lock, then attempts one post
under the same lock and a context deadline. What the verb could not post,
the steward tick's channel phase on the same machine retries from the
record. The verb's exit status is printed and never propagates.**

Why the pusher and not a tick sweep: the pusher is the only party that knows
the exact ref move (Decision 1) and the only party that performed it, so the
record's identity (Decision 3) falls out of the trigger's location instead of
needing a fleet-wide cursor. A sweep on the tick would arrive up to ten
minutes late against "the moment", would have to reconstruct the range from
a per-machine cursor over `origin/main` that every machine advances
independently, and would have to filter the goal ledger's bookkeeping commits
back out. The tick keeps one role: retrying records the pusher left behind.

### The verb

`metasystem channel landing-notice --root <repository root> --branch
<branch> --range <old>..<new> [--new-branch <new>]`, registered beside
`status` in the `channel` family table (`cmd/metasystem/main.go:395-398`).
Its steps, in order, each a mechanical rule. Steps 5 to 8 run under one
context whose deadline is `channel.landing-notice-timeout-sec` (below) from
the verb's start.

1. **Read the switch** (Decision 5). `channel.landing-notice` off: print
   `landing notice off` and exit 0. Nothing is recorded.
2. **Resolve the range.** `git rev-parse --verify <old>^{commit}` and
   `<new>^{commit}` with the git environment scrub `reportGitEnv`
   (`report.go:200-216`) already used by `landingLines`; then
   `git rev-list --reverse --topo-order <old>..<new>`. With `--new-branch
   <new>` the list is Decision 1's new-branch rule and `old` is the
   forty-zero id. An empty list is a refusal (exit 2, `nothing landed in
   <old>..<new>`); a resolution failure is exit 2 naming the id.
3. **Check this checkout performed the push** (Decision 3). `git reflog show
   --format='%H %gs' refs/remotes/origin/<branch>` must contain a line whose
   id is `<new>` and whose subject is `update by push`. Git writes that
   reflog line when a push updates the remote-tracking ref; a fetch writes
   `fetch`. Absent: exit 2, `not the pushing checkout: refs/remotes/origin/
   <branch> has no push entry for <new>`. Nothing is recorded.
4. **Compose the record** (`Landing`): `{key, remote: "origin", ref:
   "refs/heads/<branch>", branch, old, new, machine (goal.ResolveMachine,
   actor.go:21-28), pushedAt (UTC RFC3339, the verb's own clock), commits:
   [{sha, subject, goalItem}], state: "pending", attempts: 0, lastAttemptAt:
   null, lastError: "", messageRef: null, postedAt: null}`. `key` is
   Decision 3's. Subjects come from `git log --format=%s%x00%(trailers:
   key=Goal-Item,valueonly)` over the list, the same format `landingLines`
   uses.
5. **Take the channel lock** `artifacts/agents/channel/lock`, the same file
   `Poll` takes with `LOCK_EX|LOCK_NB` (`poll.go:56-67`). The verb retries
   `LOCK_NB` every 100 ms until it holds the lock or the context deadline
   passes. On deadline: exit 3, `record not written: channel lock busy;
   rerun  metasystem channel landing-notice <the same arguments>`. The
   rerun is safe: step 3 admits it in this checkout and step 6 is
   idempotent. This is the one path on which a landing leaves no record,
   and it is loud in the landing output (residual (b)).
6. **Create the record if absent, inside the lock** (CLN-R1-CREATE-IF-ABSENT).
   Every write under `landings/` in this design happens while holding the
   channel lock, so an existence check followed by `writeDurable`
   (`question.go:90-110`: temp file, fsync, rename) is not a race: the other
   writers (a second verb, the drain) are waiting on the same lock. If
   `landings/<key>.json` exists the verb prints `already recorded` and does
   not rewrite it. The rename is the commit point of the record against
   process death: a post is attempted only after the rename has returned,
   so a verb killed at any later instant leaves a complete record. The
   claim stops there: `writeDurable` fsyncs the file and not its directory,
   so a power loss between the rename and the directory's flush can lose the
   entry; such a landing is unannounced and joins residual (b). A write
   failure is exit 1 with the error and **no post is attempted**: a notice
   with no record cannot be retried, cannot be proven, and cannot be
   deduplicated.
7. **Load the provider** with `phase.Load(root, false)`; unconfigured
   channel (`Provider == nil`): release the lock, print `landing notice not
   sent: no channel is configured on <machine>; the record is pending and
   nobody else announces it` (retry owner, below), exit 0.
8. **Drain under the lock and the deadline** through the shared function
   `channel.PostPendingLandings(ctx, cfg)` (below), which the verb calls
   while still holding the lock it took in step 5. Print one line per record
   posted or left unposted. Exit 0 if every record is posted, exit 3 if any
   stays pending. The exit code is information for the landing output only.

### Shared drain: `PostPendingLandings`

One function in `internal/channel`, used by the verb and by the steward tick.
It requires the caller to hold the channel lock (the verb holds it from step
5; the tick path takes it as `Poll` does, with the same `LOCK_NB` retry every
100 ms until the context deadline, returning `Busy` and posting nothing on
deadline).

1. List `landings/*.json`, parse each, keep `state == "pending"` or `state ==
   "posting"`, sort by `pushedAt` then `key`. Oldest first, so a backlog
   after an outage arrives in landing order. A `posting` record found here
   was left by a poster that died or was killed mid-post (posting happens
   only under this lock, and the lock is free, so that poster is gone); its
   outcome is unknown.
2. For each record while the context is live: rewrite it as `state:
   "posting"`, `attempts + 1`, `lastAttemptAt: now` through `writeDurable`;
   render (Decision 4, with the repeat line when `attempts` was already
   above zero before this increment); `Provider.Post(ctx, dest, text, nil)`
   top-level, never threaded. On success: `state: "posted"`, `messageRef`,
   `postedAt`, rewritten. On a returned error: `state: "pending"`,
   `lastError` scrubbed with `Scrub` and the destination's secrets
   (`channel.go:77-84`), rewritten; continue to the next record only if the
   error is not a context deadline (a deadline ends the pass).
3. Delete `posted` records whose `postedAt` is older than 7 days. Nothing
   else is ever deleted.
4. Return counts `{posted, unposted, busy}`, where `unposted` counts records
   left `pending` or `posting`.

The steward path: `phase.Run` (`phase.go:172-215`) calls
`PostPendingLandings` after `Poll` returns and before `ComposeReport`, under
the same 15-second tick context (`runner.go:136`). Its `unposted` count is
added to the `Undelivered` figure the status carries (`phase.go:192`,
`report.go:100-106`), so a landing that could not be announced is visible in
the next status as an undelivered message rather than nowhere. `channel
poll` (`channel_verbs.go:224-260`) gains the same call so the fixture can
drive a retry without a live steward.

### Retry owner (CLN-R1-RETRY-OWNER)

**The owner of a pending record is the steward of the machine that pushed,
in the checkout that pushed. There is no shared owner.** Records live under
that checkout's `artifacts/` (`metasystem/.gitignore:1`, never in git), and a
fetching machine holds nothing to retry. This is a precondition with an
accepted loss, stated as such:

- Precondition: a machine that lands through the scripts has the channel
  configured in its `metasystem.conf.local` and its steward armed. Seat boot
  arms the steward; the verb's step 7 line names the machine when the
  channel is missing, at the terminal that landed.
- No provider: the record stays `pending`; the machine's own steward retries
  every tick and posts it the moment the channel is configured there; no
  other machine ever posts it.
- No steward: the verb's own attempt in step 8 is the only attempt; `channel
  poll` run by hand drains the record. Nothing retries on its own.
- Lost checkout: the record is lost with it. The landing is unannounced.
- What covers the loss: the four-hourly status from every machine with a
  channel still digests goal-stamped landings on `origin/main` through
  `landingLines` (Decision 5 keeps it unchanged), so a goal-stamped landing
  the pusher could not announce reaches Wido within one status interval
  from any machine. An unstamped landing lost this way is unreported.

### The prompt path's bound (CLN-R1-HANG-ORPHAN)

The prompt path is the wall clock the landing script spends on the notice.
The goal forbids a failed or hung post slowing the landing, so the bound is
short, the child is killed at the bound, and the wait overlaps work the
landing does anyway.

| Layer | Bound | Owner |
| --- | --- | --- |
| Whole verb | `channel.landing-notice-timeout-sec`, default 5, a positive integer parsed as `httpTimeout` parses its key (`phase.go:65-76`); a context deadline over lock wait, record write and every post | verb |
| HTTP request | `min(channel.http-timeout-sec, the verb's remaining deadline)`: both providers derive the request context from the caller's (`telegram.go:72`, `slack.go:47`), so the shorter wins | provider |
| Landing script | TERM at bound + 1 s, KILL at bound + 2 s, measured from the moment the reap starts | landing script |

Why the bound is short: a Telegram or Slack post is one HTTP round trip,
typically well under a second, and a post that fails (refused connection,
name resolution failure, a provider refusal) returns in milliseconds. Only a
post that hangs costs the full bound, and the bound is the hang ceiling,
not the expected cost.

Why a kill: the child is the landing script's own, launched with `&` and
known by the process id `$!` recorded at launch, which is exactly the
shared-machine rule's condition for a kill. The id cannot be reused while
the script has not yet waited on it. Killing it releases the channel lock:
the verb holds the lock through a file descriptor (`unix.Flock`, as
`poll.go:56-67`), and the kernel releases an `flock` lock when the last
descriptor on that open file is closed, which process death does. A verb
killed inside `Provider.Post` leaves its record in `posting`; the drain
treats that as an attempt of unknown outcome (Decision 3).

Why the wait overlaps the transport sync: the child is launched after the
push step and reaped after the transport sync step, so in the ordinary
landing the verb finishes while `sync-transport.sh` runs its fetch and push
(seconds), and the notice adds no wall clock at all. The added wall clock is
at most `max(0, bound + 2 s - the sync's own duration)`, and the sync is
required by the landing whether or not the notice exists. With
`--skip-transport` the reap follows the launch directly and the added wall
clock is at most bound + 2 s. Should Wido rule that the notice follows the
sync (Decision 1), the launch moves after the sync step and the added wall
clock is the same bound + 2 s.

```bash
# scripts/agents/landing-notice.sh, sourced by land.sh and commit.sh.
# Launched after the push step; reaped after the transport sync step and
# from the EXIT trap, so a landing that fails at transport still reaps it.
notice_pid= notice_output= notice_bound=
launch_landing_notice() { # branch, porcelain-output-file
  local branch=$1 porcelain=$2 flag summary args
  [[ "$branch" == main ]] || { printf -- '-- landing notice: branch %s is not announced\n' "$branch"; return 0; }
  read -r flag summary < <(landing_notice_parse "$branch" "$porcelain") || return 0
  args=(--root "$root" --branch "$branch")
  case "$flag" in
    ' ') args+=(--range "$summary") ;;
    '*') args+=(--new-branch "$(git rev-parse "refs/heads/$branch")") ;;
    *) printf -- '-- landing notice skipped: push flag %q moved nothing to announce\n' "$flag"; return 0 ;;
  esac
  notice_bound=$(landing_notice_bound)   # channel.landing-notice-timeout-sec, default 5
  notice_output=$(mktemp "${TMPDIR:-/tmp}/metasystem-landing-notice.XXXXXX") || return 0
  "$ms" channel landing-notice "${args[@]}" >"$notice_output" 2>&1 &
  notice_pid=$!
  return 0
}
reap_landing_notice() {
  [[ -n "$notice_pid" ]] || return 0
  local pid=$notice_pid waited=0 rc
  notice_pid=
  while kill -0 "$pid" 2>/dev/null && (( waited < notice_bound + 1 )); do sleep 1; waited=$((waited + 1)); done
  if kill -0 "$pid" 2>/dev/null; then
    kill -TERM "$pid" 2>/dev/null; sleep 1
    kill -0 "$pid" 2>/dev/null && kill -KILL "$pid" 2>/dev/null
    wait "$pid" 2>/dev/null
    printf -- '-- landing notice killed after %ss (landing unaffected); the steward retries from the record\n' "$((notice_bound + 1))"
  else
    wait "$pid"; rc=$?
    (( rc == 0 )) || printf -- '-- landing notice exit %s (landing unaffected):\n' "$rc"
  fi
  sed 's/^/-- /' "$notice_output"; rm -f -- "$notice_output"
  return 0
}
```

`landing_notice_bound` reads `"$ms" config get --key
channel.landing-notice-timeout-sec --default 5 --conf "$root/metasystem.conf"`
(`cmd/metasystem/config_verbs.go:94-101`, the same resolver `phase.Get`
uses) and falls back to 5 if that read fails or returns anything but a
positive integer. In land.sh, `launch_landing_notice "$branch" "$step_output"`
runs after the push loop exits at line 360 (the push step's captured output
is still in `$step_output`), `reap_landing_notice` runs after the transport
sync step at 362-364 or directly after the launch under `--skip-transport`,
and `cleanup` (`land.sh:140-144`) calls `reap_landing_notice` first so a
`fail_step` exit at transport reaps the child too. Neither function is
wrapped in `run_step`; both print with the `--` prefix and return 0.

`landing_notice_parse` reads the push step's own captured output:
`push_origin` (`land.sh:319-321`) already runs `git push --porcelain`, whose
success line for the pushed ref is `<flag>\t<from>:<to>\t<summary>`. The
parser takes the line whose `<to>` is `refs/heads/$branch`, reads the
one-character flag at column one, and takes the summary field:
`<old>..<new>` for a fast-forward (flag space), `[new branch]` for flag `*`,
`[up to date]` for flag `=`. A forced push (flag `+`) cannot occur: land.sh
never passes `--force`. Abbreviated ids in the summary are resolved by the
verb (step 2); both objects are local because land.sh fetched origin's tip
at `land.sh:311-313` and committed the new one.

`commit.sh --push` (`commit.sh:555`) changes its push to
`LC_ALL=C git -C "$root" push --porcelain origin "refs/heads/$branch:refs/heads/$branch"`,
captures the output to a file, keeps its existing failure exit, and calls
`launch_landing_notice` before its transport sync and `reap_landing_notice`
after it, from the same sourced file, so the two push sites cannot drift.

### What a post that fails does to the landing

Nothing. Both functions return 0 on every path; neither is wrapped in
`run_required_step`. The transport sync keeps its `run_required_step` and
its exit codes exactly as today. A hung post costs the landing at most
`channel.landing-notice-timeout-sec + 2` seconds beyond the transport sync,
leaves a `pending` or `posting` record, and the next steward tick posts it.

## Decision 3 — identity and delivery: the ref move is the key; delivery is at-least-once with a marked repeat

### The key (CLN-R1-REF-MOVE-KEY)

**A landing record is keyed by the ref move as a push defines it: the remote
(`origin`), the full ref (`refs/heads/<branch>`), the old tip, the new tip,
and the pushing machine. `key` is the first 32 hex characters of the SHA-256
over those five fields joined by newlines, and the record is
`artifacts/agents/channel/landings/<key>.json`. The five fields are also
stored in the record in clear.** The new tip alone is not the identity: two
refs can end at one commit, and a ref can return to an earlier tip.

The record lives in the checkout that pushed and nowhere else, and the verb
enforces that only the pushing checkout writes one (step 3): the pusher's
`refs/remotes/origin/<branch>` reflog carries an `update by push` entry for
`<new>` that a fetching checkout does not have. This turns revision 1's
assumption ("only the pusher holds a record") into a check the verb makes
before writing anything.

The three cases the critique named, each with the number of notices it
produces:

| Case | Keys | Notices | Why |
| --- | --- | --- | --- |
| Two branches ending at one commit `X`: `main` moves `A..X`, then `paper` moves `B..X` from the same checkout | Two keys: the refs differ | Two under an "all branches" rule; one under the proposed `main`-only default (the paper push is not announced by Decision 1) | The ref is part of the key; the same tip under two refs is two ref moves |
| A forced rollback of `main` from `Y` back to `X`, then a re-landing through land.sh | The rollback is a raw forced push outside the scripts and produces no record (Decision 1, case one). The re-landing creates a fresh commit `Z` (both scripts commit before they push), so its key `(origin, main, X, Z, machine)` differs from the earlier `(origin, main, W, X, machine)` | One, for the re-landing | The scripts never push an already-pushed tip: their new tip is always a commit they just made |
| The verb replayed in a second checkout that fetched the range | The second checkout's reflog for `refs/remotes/origin/main` shows `fetch`, not `update by push`, for `<new>` | One, from the pushing checkout; the replay exits 2 and writes nothing | Step 3's check |
| The verb replayed in the pushing checkout (land.sh rerun, or by hand) | Same key, same file | One | Step 6 finds the record and does not rewrite it; the drain posts only when `state` is not `posted` |

Residual of the check: linked worktrees of one repository share
`.git/logs/refs/remotes/`, so a replay from a sibling worktree of the pushing
repository passes step 3 and has its own `artifacts/`; it would write a
second record and post a second notice. The fleet's checkouts are separate
clones and the seat's delegate worktrees never land; this is named, not
closed. A checkout with `core.logAllRefUpdates` off has no reflog and every
landing from it exits 2 at step 3, loudly; the default for a non-bare clone
is on.

The brief's proposed cursor beside `fleet/cursor.json` (`poll.go:138`) stays
rejected: a cursor is a position in a stream, and the landing stream has no
single reader. Every machine's `origin/main` advances by everyone's pushes,
so a per-machine cursor announces other machines' landings, and a shared
cursor would have to live on origin and be advanced under a network lock,
which puts the network on the landing's prompt path twice.

### Delivery (CLN-R1-EXACTLY-ONCE-ACK)

**This design does not promise exactly-once delivery, because it cannot:
`Provider.Post` (`channel.go:42-43`) carries text and an optional thread
and nothing else, Telegram's `sendMessage` is sent with `chat_id` and `text`
(`telegram.go:121`) and Slack's `chat.postMessage` with `channel` and `text`
(`slack.go:70`), and neither API accepts a client idempotency key for a
message. A Telegram bot also cannot read the channel's history to check
whether an earlier copy arrived. What the design promises instead:**

- **At most one notice on every path where the provider's answer arrives.**
  Success marks the record `posted` under the lock; a provider refusal or a
  refused connection marks it `pending`; both are unambiguous.
- **At least one notice, and a marked repeat, when the answer is lost.** The
  ambiguous cases are a request sent and its response lost (deadline,
  reset), the poster killed or dead between the provider's acceptance and
  the `posted` rewrite, and a `posted` rewrite that fails. In each the
  record is `posting` (written before the post) or `pending` with `attempts
  > 0`. The next drain retries, and Decision 4 renders every retry with the
  line `(repeat: an earlier copy of this notice may have reached you)`. The
  reader sees at worst the same landing twice, the second copy saying so.
- **Why every retry carries the line, not only the ambiguous ones:** both
  adapters flatten the cause into `ProviderError.Problem` text
  (`telegram.go:86`, `slack.go:57`) and `ProviderError` has no `Unwrap`
  (`channel.go:58-63`), so the drain cannot tell a refused connection from a
  lost response without matching error strings. "May have reached you" is
  true in both cases.

The attack list from the brief, answered against the record design:

| Attack | What happens | Why it holds |
| --- | --- | --- |
| Two checkouts of one repository on one machine | Each checkout has its own `artifacts/` and its own lock. Checkout A pushes `X..Y` and records `(origin, main, X, Y, m)`; checkout B later rebases onto `Y` and pushes `Y..Z`, recording `(origin, main, Y, Z, m)`. Two records, two notices, no overlap. | A fast-forward push's summary is the exact range origin moved by; two pushes cannot move the same ref over the same range. Both land.sh (`land.sh:311-317`) and the porcelain summary make `old` origin's tip at push time. |
| A push during a tick's drain | The tick's drain holds the lock; the pusher's verb waits for it up to its deadline (step 5). If it gets the lock, it creates and posts; if not, it exits 3 loudly with no record and the rerun line. There is no sweep of commits. | Creation and posting both happen only under the lock; `state` on the record is the single truth both sides read after taking it. |
| A push whose record write fails, or whose lock wait times out | The verb exits 1 or 3 before posting; the landing script prints the line and continues; the landing is not announced and no retry exists for it until the printed rerun is done. | Chosen over "post without a record" because a record is what makes a retry, a proof and a dedup possible. Residual (b). |
| A rebase that changes commit ids after announcement | The notice names the ids that reached origin. land.sh's own rebase runs before the push (`land.sh:340`), so announced ids are post-rebase. A later local rebase changes nothing on origin. A forced rewrite of origin is outside both scripts; the landing that follows it is a new ref move with its own key and is announced once. | The key is the ref move, not a commit's content. |
| A machine that fetches another machine's landings | Fetching moves `origin/main` locally, writes `fetch` to the reflog and nothing under `landings/`; the verb refuses there (step 3). | Only a push writes `update by push`; only the pusher pushed. |
| The same push replayed in the pushing checkout | Step 6 finds the record; the drain posts only if `state` is not `posted`. | One file per key; `state` reaches `posted` once, under the lock. |
| Two machines land at the same second | Origin serializes ref updates; the second push is rejected non-fast-forward (`land.sh:323-325`), rebases, and pushes a different range. | Git's ref update is the atomic point; land.sh already relies on it. |
| The provider accepts and the answer is lost; or the poster dies after acceptance and before the `posted` rewrite | The record is `posting` (or `pending`, `attempts: 1`); the next drain posts again with the repeat line. Two copies reach the channel, the second marked. | At-least-once, stated above. Revision 1 claimed this could not recur; it can, and the repeat line is the answer. |
| A `posted` record lost from disk | The message stands in the channel; the same key cannot be recreated because the same ref move cannot happen again through the scripts. Nothing re-announces. | A key is minted by one ref move. |

Retention: `posted` records older than 7 days are deleted by the drain
(Decision 2, drain step 3). `pending` and `posting` records are never
deleted by the engine; a record that stays unposted for a day is a channel
outage the status's `Undelivered` line already reports.

## Decision 4 — the message

**One notice per landing (per push). One line per commit inside it, oldest
first, within the same 1600-rune bound the ask uses. Every field the
renderer inserts has a stated bound, so the whole message's maximum is a
sum the reader can check.**

The bound is the constant `questionMessageRuneLimit` (`question.go:74`,
landed in b52711d3a). The implementer renames it to `messageRuneLimit` in
`question.go` and uses it from the notice renderer; no second constant, no
copy. The Telegram provider's 4000-rune chunking (`telegram.go:17,152-178`)
stays as the transport safety net and is never relied on.

Rendering rules for `RenderLanding(l Landing, repeat bool, loc
*time.Location) string`:

1. **Time** is `pushedAt` (the landing moment, not the posting moment, so a
   retried notice still says when it landed) in the machine's zone,
   `time.Local` by default, the same rule `ComposeStatusReport` applies to
   the status header (`report.go:40-42`), formatted `2006-01-02 15:04 -0700`
   like the status header (`report.go:91`): 22 runes always.
2. **Field bounds** (CLN-R1-RUNE-BOUND). The code bounds none of these:
   `ResolveMachine` accepts any non-empty trimmed nickname
   (`actor.go:21-28`), `sync-transport.sh:20-29` restricts a branch's
   characters and not its length, and a goal id has no length rule the
   channel can cite. The renderer therefore trims, with the same
   `trimQuestionPart` ellipsis rule (`question.go:344-353`): machine to 32
   runes, branch to 64, goal name to 80, subject to 200. The commit count is
   rendered in decimal and is treated as at most 7 digits for the sum below.
3. **Single commit** (the common case), one line:
   `<machine> landed <time>[ on <branch>]: <subject>[ — <goal name>]` where
   `— <goal name>` is `featureName(goalItem)` and is omitted when the
   trailer is absent, and ` on <branch>` appears only for a branch other
   than `main`. Example: `m1 landed 2026-09-05 12:43 +0200: An ask fits one
   message and keeps its token — channel ask fits one message`. Maximum:
   32 + 8 + 22 + 4 + 64 + 2 + 200 + 3 + 80 = 415 runes.
4. **Several commits**, a header and a line per commit:
   `<machine> landed <n> commits <time>[ on <branch>]:` then `- <subject>[ —
   <goal name>]` per commit, oldest first. Header maximum: 32 + 8 + 7 + 9 +
   22 + 4 + 64 + 1 = 147 runes. Commit line maximum: 2 + 200 + 3 + 80 = 285.
5. **Repeat line.** When `repeat` is true the last line is `(repeat: an
   earlier copy of this notice may have reached you)`, 61 runes, always
   reserved before any commit line is added.
6. **Bound.** Lines are added in order while the total including a possible
   tail line and the reserved repeat line stays within `messageRuneLimit`;
   when the next line would not fit, the tail
   `- … and <k> more, <old7>..<new7> on origin` is appended, where `<old7>`
   and `<new7>` are the seven-character ids and `k` is the count of commits
   not listed: at most 2 + 1 + 5 + 7 + 6 + 7 + 2 + 7 + 10 = 47 runes. The
   header, the tail and the repeat line together are at most 147 + 47 + 61
   + 2 newlines = 257 runes, so the header always fits and at least four
   commit lines of maximum length always fit. A push of twenty commits with
   ordinary subjects lists roughly the first fifteen and names the rest by
   range.
7. **Never threaded.** A reply to a notice is not a command; `Poll` matches
   inbound replies to question threads and the status thread only
   (`poll.go:109-137,160-177`), so a reply to a notice lands in
   `unmatched.jsonl` as it does for any unrecognized message. The design adds
   no notice thread to that match, on purpose: the status thread carries the
   `start <goal-id>` token and nothing else should look like it.

## Decision 5 — the status stays what it is

**The four-hourly status keeps its interval, its digest semantics, its
`ShouldPost` AND, and its `Delivered` lines, byte-for-byte today's text
whether landing notices are on or off. The one change is that the
`Undelivered` figure also counts this machine's `pending` and `posting`
landing records. There is no count line.**

Revision 1 replaced the `Delivered` lines with `Landed: n since last status`
when notices were on. The critique showed the count was not checkable
against notices (it counted distinct known goals, so two pushes for one
goal gave two notices and `n = 1`, and unstamped landings gave a notice and
no count) and that two windows with the same count and otherwise equal text
would share a digest and suppress the later status (CLN-R1-STATUS-COUNT).
Both defects come from the count, so the count goes.

How the digest and the notice avoid saying the same thing twice, and where
they deliberately do:

- They say different things in different forms. The notice says, at the
  moment, which commits one push put on origin and from which machine. The
  status says, four-hourly, which known goals had any delivery in its window,
  one subject per goal, from every machine. A reader who received the
  notices can skip the `Delivered` lines; a reader who missed one finds the
  goal there.
- The overlap for goal-stamped landings is deliberate: it is the only path
  that reaches Wido when the pusher could not announce (Decision 2, retry
  owner). Removing it would make that loss silent.
- Unstamped landings appear in notices and never in the status, as today.
- The digest behaves exactly as today because the text is exactly as today:
  `Digest` (`report.go:218-229`) still drops the timestamp line and hashes
  the rest, and a window whose landings differ by goal still changes it.
  What did not change the digest before (two pushes for one goal in one
  window, `report.go:178-180`) still does not; the notices carried them.
- No notice ever resets the status interval, and no status ever marks a
  landing posted. The two ledgers (`status.json`, `landings/*.json`) do not
  read each other except through the `Undelivered` count.

The off switch: `channel.landing-notice` in `metasystem.conf` or
`metasystem.conf.local`, read through `phase.Get` with default `on`. Accepted
values are exactly `on` and `off`; any other value is a configuration error
that the verb (exit 2) and `phase.Run` (returned error, printed by the tick
at `runner.go:138`) both name. Off means: the landing verb records nothing
and posts nothing (Decision 2 step 1); the drain still posts records that
already exist, so switching off never strands an unposted record. The switch
does not touch the status.

## Decision 6 — proof

Every obligation below names the fixture that proves it. Two fixture homes,
matching what exists: Go tests in `internal/channel` for rendering and
record state (the pattern of `channel_test.go:600-620`), and the land and
channel shell fixtures for the script seams (`land-fixtures.sh` legs against
a local bare origin, `channel-fixtures.sh` against the fake provider whose
journal records every `sendMessage` and `chat.postMessage`,
`fake.go:193-194,234-238`). The fake's `pauseBefore` control (`fake.go:83-87,
307-319`) stalls any named method, including `sendMessage` and
`chat.postMessage`, until a file appears, and returns when the client
disconnects (`fake.go:361-377`, on `r.Context().Done()`); it proves the
context-deadline layer and cannot prove a child that ignores its deadline.

| # | Obligation | Fixture | Home |
| --- | --- | --- | --- |
| 1 | A fast-forward push of `main` through land.sh produces exactly one `sendMessage` in the fake journal before land.sh exits, naming the machine, the local time, the subject and the goal name | New land leg `notice`: bare origin plus fake provider configured in the leg's `metasystem.conf.local`; land one goal-stamped commit; assert one journal entry and one `posted` record whose `new` is origin's new tip and whose `key` matches the five stored fields | land-fixtures.sh |
| 2 | A landing without a `Goal-Item` trailer is announced without a goal name | Leg `notice`, second landing without `--goal`; assert the line has no ` — ` goal suffix | land-fixtures.sh |
| 3 | A push of twenty commits becomes one message within 1600 runes, oldest first, with the tail naming the range and the count | Go test: `RenderLanding` over a synthetic 20-commit record with 150-rune subjects; assert rune count, first line, tail line, and that the listed subjects are the first `k` in order | channel_test.go |
| 4 | The 1600 bound is the ask's constant, not a copy | Go test asserts `messageRuneLimit == 1600` from one identifier used by both `renderQuestion` and `RenderLanding`; a grep guard in the test fails if a second literal `1600` appears in `internal/channel` outside the constant | channel_test.go |
| 5 | The time is the push time in the local zone, and a retried notice still shows the push time | Go test: record with `pushedAt` fixed, `loc` fixed to a non-UTC zone; render at a later `now`; assert the formatted time | channel_test.go |
| 6 | A post that fails leaves land.sh's exit status untouched and leaves a `pending` record | Leg `notice-fails`: fake provider `api-base` pointed at a closed port; land; assert exit 0 from land.sh, `state: "pending"`, `attempts: 1`, `lastError` non-empty, and the transport sync step ran | land-fixtures.sh |
| 7 | A pending record is posted by the next channel pass, marked `posted` once, and carries the repeat line | Leg `notice-fails` continues: repoint the fake `api-base` at the live fake, run `channel poll`; assert one journal entry whose text ends with the repeat line, `state: "posted"`; run `channel poll` again; assert still one entry | land-fixtures.sh |
| 8a | A post that hangs and honours cancellation costs at most the verb's deadline, exits 3, and does not fail the landing | Leg `notice-hangs`: fake `control.json` with `pauseBefore` `{method: "sendMessage", until: <a path never created>}`; `channel.landing-notice-timeout-sec=2`; land; assert land.sh exits 0, prints the `exit 3` line, the record is `pending` with `attempts: 1`, and the notice's wall clock measured by the fixture is under 2 s plus the scaled fixture ceiling slack | land-fixtures.sh |
| 8b | A notifier that ignores its deadline is killed by the landing script at the bound, the landing exits 0, and the lock it held is free afterwards | Leg `notice-orphan`: `METASYSTEM_BIN` set to a wrapper that, for `channel landing-notice`, opens `artifacts/agents/channel/lock` with `flock` held and sleeps 600 s, and execs the real binary for every other verb; a hand-written `pending` record in `landings/`; `channel.landing-notice-timeout-sec=2`; land; assert exit 0, the `killed after 3s` line, no process with the recorded id, and that a following `channel poll` (real binary) posts the hand-written record, which it cannot do while the lock is held | land-fixtures.sh |
| 9 | The record is written before any post is attempted, and a record write failure posts nothing | Go test with a `Provider` fake that counts `Post` calls and a read-only `landings/` directory: assert zero posts and a returned error | channel_test.go |
| 10 | Two checkouts of one repository on one machine announce disjoint ranges once each | Leg `notice-two-checkouts`: clone the bare origin twice, land from A then from B (B rebases onto A's tip); assert two journal entries whose ranges do not overlap and two records with distinct keys | land-fixtures.sh |
| 11 | Replaying the verb on the same range in the pushing checkout posts nothing new | Leg `notice`: run `channel landing-notice` by hand with the same `--range`; assert `already recorded` and one journal entry | land-fixtures.sh |
| 12 | A checkout that did not push is refused by the verb and announces nothing | Leg `notice-two-checkouts`: after A lands, B fetches and runs the verb with A's range; assert exit 2 naming `not the pushing checkout`, no record in B, journal count unchanged; B runs `channel poll`; still unchanged | land-fixtures.sh |
| 13 | The drain returns `Busy` and posts nothing when the lock is held past its deadline; the verb writes no record when the lock is held past its deadline | Go test: hold the flock from the test, call `PostPendingLandings` with a 200 ms context; assert `busy == true`, zero posts, record still `pending`. Second test: hold the flock, run the verb's create step with a 200 ms context; assert no file and the `channel lock busy` error | channel_test.go |
| 14 | The status text is today's text with notices on and with notices off | Two Go tests beside `TestReportShowsOneQuestionTwoLandingsAndOnlyTwoNextItems` (`channel_test.go:291-319`), one per switch value, both asserting the existing expected text unchanged | channel_test.go |
| 15 | `Undelivered` includes `pending` and `posting` landing records | Go test: one `pending` and one `posting` record, `phase.Run` against the fake provider with posting disabled for landings; assert the status's `Undelivered` line counts two | phase_test.go |
| 16 | The off switch records and posts nothing, and an invalid value is refused by name | Leg `notice-off`: `channel.landing-notice=off`, land, assert no record and no journal entry; then `=maybe`, run the verb, assert exit 2 naming the key | land-fixtures.sh |
| 17 | `commit.sh --push` announces through the same path on `main` and not on `paper` | Leg `notice-commit-push`: `commit.sh --push` on `main`, assert one journal entry; `commit.sh --push` on a branch named `paper`, assert the `is not announced` line and the journal count unchanged | land-fixtures.sh |
| 18 | Goal-ledger publishes produce no notice | Leg `notice`: `goal open` then `goal claim` against the leg's origin; assert the journal count is unchanged | land-fixtures.sh |
| 19 | `posted` records older than 7 days are deleted, `pending` and `posting` never | Go test with backdated `postedAt`; assert deletion of the posted one and survival of the other two | channel_test.go |
| 20 | The porcelain parser takes the right line, flag and summary, including `[up to date]` and `[new branch]` | Shell unit inside the land fixture: feed three captured porcelain outputs to the parsing function; assert range, flag, and the skip line | land-fixtures.sh |
| 21 | A poster killed mid-post leaves `posting`, and the next drain posts once with the repeat line | Go test: `Provider` fake whose `Post` blocks until cancelled; run the drain with a context cancelled from the test; assert the record is `posting`, `attempts: 1`; run the drain again with a succeeding fake; assert one post whose text ends with the repeat line and `state: "posted"` | channel_test.go |
| 22 | The key distinguishes ref, old, new and machine, and two refs ending at one tip get two keys | Go test on the key function: five records differing in one field each have five distinct keys; `(origin, refs/heads/main, A, X, m)` and `(origin, refs/heads/paper, B, X, m)` differ | channel_test.go |
| 23 | A new branch's notice lists every commit the push made newly reachable | Leg `notice-new-branch` (runs with the branch rule widened to the leg's branch, since the proposed default announces `main` only): create a branch with three commits off origin's `main`, push it through the script; assert one notice listing three subjects, oldest first | land-fixtures.sh |
| 24 | The renderer bounds machine, branch and goal name, and the maximum header fits | Go test: a 300-rune machine, a 300-rune branch, a 300-rune goal id, a 7-digit commit count, 20 commits of 300-rune subjects, `repeat` true; assert the header is at most 147 runes, every line respects its field bounds, the total is within `messageRuneLimit`, and the last line is the repeat line | channel_test.go |
| 25 | Concurrent creation under the lock yields one record | Go test: two goroutines run the verb's lock-and-create step for the same key against one directory; assert one file, one `created` result and one `already recorded` result | channel_test.go |

Residual risks, named rather than claimed:

- (a) A raw `git push` outside the two landing scripts is not announced
  (Decision 1, case one, proposed). The status digest covers goal-stamped
  ones four-hourly; unstamped ones are unreported. Closing it would need a
  server-side hook on origin, which the fleet does not own.
- (b) A record write failure, a channel lock held past the verb's deadline,
  or a power loss before the record's directory entry is flushed leaves a
  landing unannounced with no retry; the landing output says so at the
  terminal for the first two, and nothing says so for the third.
- (c) Row 8b's wrapper needs `flock(1)` in the fixture bed; on a host
  without it the wrapper only sleeps, and the lock-release half of the row
  is carried by reading (`flock(2)` releases on the last close, which
  process death performs) plus row 13.
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
- (g) Linked worktrees share the remote-tracking reflog, so a replay from a
  sibling worktree of the pushing repository is not refused (Decision 3).
- (h) A pusher with no channel, no steward, or a lost checkout is the
  accepted loss of Decision 2's retry owner; goal-stamped landings reach the
  four-hourly digest from other machines, unstamped ones do not.
- (i) The `update by push` reflog subject is git's own English string and is
  not translated; row 12 pins it against the git in the fixture bed
  (`git --version` 2.50.1 here).

## Consistency pass

- Decision 1's "landing is a scripted push" is what Decision 3's key (the
  ref move) and Decision 2's trigger (the push site) both rest on; the
  three cases awaiting Wido change which pushes qualify and when the launch
  happens, and nothing else in Decisions 2 to 6.
- Decision 2's record-before-post, inside the lock, is what Decision 3's
  replay and concurrent-creation rows and Decision 6 rows 9, 11, 13 and 25
  assume. Decision 2's `posting` state is what Decision 3's at-least-once
  promise and Decision 4's repeat line read.
- Decision 2's kill is lawful because the child is the script's own, and it
  is safe because the lock is descriptor-bound and the record is complete
  before any post; Decision 6 row 8b exercises both halves.
- Decision 4's use of `pushedAt` is why a retried notice from Decision 2's
  drain still says when the landing happened; its field bounds are why the
  1600 claim is a sum and not an estimate.
- Decision 5 keeps the status text unchanged, which is what Decision 2's
  retry owner relies on as the fallback for a pusher that cannot announce.
  The one asymmetry, stated so nobody reads the status as "every notice":
  unstamped landings are announced and never digested.
- The goal's DONE line says "exactly once". This design delivers exactly
  once on every path where the provider answers and at-least-once with a
  marked repeat where the answer is lost, because neither provider offers
  the key exactly-once needs. That wording difference is for Wido to accept
  or refuse alongside Decision 1; the design does not restate the goal.
- Every configuration key named exists today except `channel.landing-notice`
  (Decision 5, default `on`) and `channel.landing-notice-timeout-sec`
  (Decision 2, default 5), each introduced with its default and its
  accepted values.
- The `plans/` guard: this design is a plan file and lands with
  `--allow-new-plan` (`land.sh:42-45,327-332`) as revision 1 did; the
  implementer's product touches no plan file.

## Self-grade

Grounding: every load-bearing claim is a file-and-line read in this worktree
at commit 6ca7a0ac2 (`report.go` whole, `phase.go` whole, `telegram.go`
whole, `slack.go` whole, `fake.go` whole, `channel.go` whole,
`question.go:60-130`, `poll.go:40-80`, `actor.go:1-60`, `land.sh` whole,
`commit.sh:540-575`, `sync-transport.sh` whole, `metasystem/.gitignore`),
the critique register whole, and the goal file whole. Revision 1's git
queries about 2026-09-05 are carried forward unchanged. Two facts are from
git's documented behaviour and not exercised here: the porcelain summary
format (`<old>..<new>`, `[new branch]`, `[up to date]`; row 20 pins it) and
the `update by push` reflog subject on the remote-tracking ref after a
successful push (row 12 pins it). One fact is from the operating system's
documented behaviour: `flock(2)` releases on the last close (row 8b pins
the observable half). Nothing in this design was executed against a
provider or a live push. Grade: pass against everything observed, with one
decision held open on purpose (Decision 1 awaits Wido) and one promise
narrowed on purpose (Decision 3: at-least-once with a marked repeat). The
reject condition below is the falsifier the implementation and its
critique must actively test.

**Reject condition — reject this design if any of the following is shown:**
a build dispatched before Wido's word on Decision 1 is recorded on the goal;
a path on which two records exist for one ref move in separate clones of
the pushing repository (the step 3 check defeated outside the named
worktree residual); a landing through `land.sh` or `commit.sh --push` of
`main` whose successful push produces neither a `sendMessage` within the
bound nor a `pending` or `posting` record nor a printed `record not
written` line (the 2026-09-05 silence recreated silently); any path on which
a post is attempted before its record is renamed into place, or on which a
record is created or its state rewritten without the channel lock held; any
exit of `land.sh` or `commit.sh --push` whose status changes because of the
notice step, including a hung provider and a notifier that ignores its
deadline; a notifier child still alive after the landing script exits; a
landing script wall clock extended by more than
`channel.landing-notice-timeout-sec + 2` seconds beyond its transport sync
by a hung notifier; a retried notice without the repeat line, or any claim
in the built product of exactly-once delivery; a notice longer than the one
shared `messageRuneLimit`, a header longer than 147 runes, or a second
literal for that bound anywhere in `internal/channel`; a goal-ledger publish
(`goal open`, `goal claim`, and the rest) that produces a notice; a status
whose text differs by a byte from today's, with the switch on or off, other
than the `Undelivered` figure; a `pending` or `posting` record the drain
deletes; a retried notice that shows the posting time instead of the
landing time; or a switch value other than `on` and `off` that is accepted
silently.
