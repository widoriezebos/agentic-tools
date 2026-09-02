# Never-idle analysis: why seats sit idle with backlog, and what would have to be true for that to be impossible

Goal: never-idle-ironclad (`plans/goals/never-idle-ironclad.md`, revision 3, claimed by m1).
Round: analysis, not design (the goal's Next step, line 6, and the brief
`plans/never-idle-analysis-brief.md`). Author: implementer delegate
`never-idle-analysis-r1` under dispatch by m1+main-1788333680-2840-7f79f4,
2026-09-02. Every seam cited below was read in this worktree at commit
6f57cb58. Live evidence marked "m1 primary" was read from the primary checkout
at `/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/artifacts/` and its
wrapper root `/Users/wido/LocalStorage/GitHub/agentic-tools/artifacts/`; those
files are untracked and are not in this worktree. The artifacts of m0, m0b, m2,
m3 and the paper seat were not reachable from this machine; where a claim about
them rests on a record another seat wrote, the record is named and the claim is
marked relayed.

Time zones: `artifacts/agents/narration.log` stamps local time
(`internal/steward/narrate.go:184` formats `now` without converting to UTC; the
machine's offset is +02:00, as the commit dates show). Health records, alert
episodes and goal histories stamp UTC. Both are quoted as written.

Wido's order, verbatim from the goal (line 4): "You must never ever stop as
long as you are not stopped by me, and there is backlog to work on left" and "I
want no room for conduct failures. I want the Go app and its background job and
roles to be iron clad against this. Is it? Is there backlog for this?"

## 0. The answer today, in one paragraph

It is not iron-clad, and the reason is structural rather than a missing
feature. Three things are true at once in the tree. First, the only mechanism
that can refuse a turn end is the harness Stop hook calling the turn verdict,
and that verdict deliberately blocks once per unchanged state and then lets the
same state pass (`internal/goal/turnverdict.go:363-373, 389-425`;
`internal/report/stopblock.go:11-13` promises "This refusal does not repeat for
the same work"). Second, on the machine this analysis ran on, the hook has not
recorded a turn since 2026-08-30 and the verdict has not run since 2026-08-27
(section 3.4), so even the block-once gate is absent in practice. Third, the
steward, the one process that keeps running when the seat goes quiet, calls an
unclaimed backlog "no work": with queued goals and no local claim it returns
`WorkNone` (`internal/steward/openwork.go:59-60`), the decision ladder maps that
to `ActNone` (`internal/steward/verdict.go:95-96`), and the only trace is one
sentence per tick in a local log file (`internal/steward/narrate.go:148-149`).
Nothing consumes that sentence. No health role goes red for it
(`internal/steward/delivery.go:81-83`, `internal/steward/health.go:800-802`),
no alert episode opens, nothing reaches the human, and nothing touches the seat.
The seven goals that circle this problem (the six the goal binds plus
`idle-every-runtime-enforcement`, opened by Wido at 2026-09-02T15:54:37Z) cover
the first two guarantee parts on the Claude runtime and the external alert
path; none of them owns re-injecting a continue order into a live idle seat,
none owns detecting a failed instruction channel, and none owns proving that
the Stop hook is actually loaded in the session it is supposed to guard.

## 1. The stop paths

A "stop path" is any sequence that ends with a seat quiet while
`goal.Next` would report at least one ready queued goal
(`internal/goal/project.go:90-124`: queued, labels match, not pinned to another
machine, every blocker done). The paths below are enumerated from the code and
the records. Paths P7, P8, P10 and P11 are not in the brief's list.

| # | Path | Where the seat goes quiet | What today's machinery does |
| --- | --- | --- | --- |
| P1 | The Stop hook runs, the verdict allows the turn end. Sub-cases: (a) block-once already spent for this open-work signature, goal revision, queue digest, or unwatched digest (`turnverdict.go:365-372, 393-398, 417-425, 430-435, 245-257`); (b) any `scan.Busy` item suppresses every goal clause, so one unrelated live job or run launders idleness on every other goal (`turnverdict.go:355-361`; Busy is filled from job records with status pending or running, gate markers, mission runners and live runs, `internal/report/scan.go:33-63`); (c) a plan whose `Waiting on the human` field is non-empty is removed from open work and suppresses the goal clause (`internal/report/openwork.go:234-236`, `scan.go:87`, `turnverdict.go:375-382`); (d) unreadable inputs or a degraded ledger produce display only, never a block (`turnverdict.go:384-387, 439-440`) | at the Stop event | the hook emits a `systemMessage` and exit 0 (`scripts/agents/supervision-hook.sh:298-305`) |
| P2 | The Stop hook runs but never reaches the verdict: engine missing exits 0 (`supervision-hook.sh:26-31`), no git toplevel exits 0 (line 65), session-environment failure exits 0 (lines 60-63), a verdict I/O error becomes a non-blocking `systemMessage` (lines 306-320), and every unguarded command under `set -euo pipefail` before line 321 is a silent exit (line 2) | at the Stop event | nothing blocks; sometimes a health line is printed |
| P3 | The Stop hook runs against the wrong world: on a nested checkout the hook takes `git rev-parse --show-toplevel` of the payload cwd as the repository (`supervision-hook.sh:50-66`), which on the fleet layout is the wrapper repository with no goal ledger (`plans/supervision-hook-root-design.md:17-25, 41-52`). The verdict reads an empty world and cannot block. Turn evidence lands where health never reads it, so `hook-freshness` is dead permanently (`internal/steward/health.go:285-321`) | at the Stop event | a wrong-world verdict, plus a permanent dead health role that alerts on aggregate changes (section 2.4) |
| P4 | The Stop hook is not loaded in the session at all. `metasystem/.claude/settings.json` was deleted on 2026-09-02 12:31 +02:00 (commit 55f10c44, "Remove the settings file that surfaced the doubled hook path"); the only registration left is the wrapper's `.claude/settings.json`, whose Stop entry runs `cd "$CLAUDE_PROJECT_DIR/metasystem" && bash scripts/agents/supervision-hook.sh claude stop` with a 5 second timeout. The m1 main seat runs with working directory `/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem` (m1 primary: `supervision/last-census.json`, ANNOUNCED pid 2840). Whether Claude Code loads a parent directory's settings in that configuration is disputed by the m0 critique (`records/misc/idle-rootcause-critique.md:20-24`, F-CONFIG-003: "the preserved runtime contradicts it"). The observed fact on m1 is in section 3.4: no hook attempt anywhere since 2026-08-30T19:06:23Z | at the Stop event | nothing runs; `hooks check` validates the file's contents, not its loading (`scripts/validate-metasystem.sh:2187-2203`; F-PROBE-004, `idle-rootcause-critique.md:26-30`) |
| P5 | The runtime kills the hook before it emits. Every shipped Stop timeout is 5 seconds (`scripts/enforcement/claude-code-hooks.json`, `codex-hooks.json:22`, `devin-hooks.json`; the wrapper `.claude/settings.json` Stop entry), and the Stop path runs `metasystem up` (`supervision-hook.sh:148-156`), a health preview, a digest read, a watchdog report and evidence collection before the verdict (lines 161, 166, 256, 265, 274). A killed hook records nothing and blocks nothing. m1 primary: the wrapper-root hook evidence shows generations 1 to 3 ending `ERROR / INTERRUPTED_BY_NEXT_TURN` (`components/supervision-hook.json`; the outcome is written by `internal/steward/component_evidence.go:191` when the next attempt finds the previous one unfinished) | at the Stop event | nothing |
| P6 | The seat is classified an advisor (an announced main that does not hold the checkout lease) and the hook exits early with `OWNED-ELSEWHERE`, bypassing the verdict entirely (`supervision-hook.sh:232-247`; the comment at 227-231 names the cost) | at the Stop event | a non-blocking message |
| P7 | The seat ends its turn on a decision-ask written as prose, with ready work that needed no answer. Nothing in the verdict models "a question to the human is pending": `WaitingOnHuman` is plan-field text and fenced claims, both seat-writable, and the queue clause blocks once per digest (P1a, P1c). Specimen: m1 at 15:45 local on 2026-09-02 (section 3.5) | at the Stop event | the block-once gate, when it runs at all |
| P8 | The seat is frozen inside a turn by a permission prompt or classifier dialog. No Stop event fires because the turn has not ended; the process is alive, so the census counts it live (`internal/steward/census.go:67-71`). With a claim held, the steward says healthy until five ticks without progress (`verdict.go:100-108`); with no claim it says no work (`verdict.go:95-96`). Specimens: m2 on 2026-08-31 for four hours (`records/misc/idle-loss-2026-08-31.md:96-102`), m1 on 2026-09-02 before 13:35 local (section 3.4) | inside the turn | at most a `stalled-idle` notification after 50 minutes, and only when a claim is held |
| P9 | Session death: the runtime process is gone. With a claim held on this machine and a complete census proving death, the steward revives (`verdict.go:116-132`) by dispatching one `steward-continuation` delegate job (`internal/steward/revive.go:68-151`, `internal/steward/stage.go:16-17`, `scripts/agents/roles/steward-continuation.md:1-17`). With no claim it does nothing (`openwork.go:59-62`, `verdict.go:95-96`). The `session-main` health role goes dead ("every announced session main is dead", `health.go:663`) with no lawful automatic remedy (`health.go:416-445` has no case for it), so it alerts and waits | when the process exits | revival of owned work only; never a seat relaunch; nothing for an unclaimed backlog |
| P10 | The human's instruction channel fails silently: remote-control messages sit unsent in a seat's input box and the seat never receives the order (m0, m0b and paper on 2026-09-02, `never-idle-ironclad.md:4`). No record of the event exists in the tree apart from the goal and the brief (searched `plans/`, `records/`, `memory/`, `docs/` for "remote-control", "stranded", "tmux", "input box": only `plans/never-idle-analysis-brief.md:30,59` and the goal match). No engine component knows what a seat's inbound channel is; the seat announcement records pid, runtime and instance tag only (m1 primary: `mains/session-2840-2840.json`) | between turns | nothing observes the channel; the seat looks idle to the steward exactly as in P1 |
| P11 | The seat holds one claim that is lawfully blocked (a breach-stop fence, a human word pending, an attempt fence) and does not claim the next item. The steward sees `WorkOwned` with a live worker and judges progress by two marks that any machine can move: the checkout HEAD and the accepted goals ref tip (`internal/steward/marks.go:23-52`). On a busy fleet those marks advance without this seat doing anything, so `TicksSinceAdvance` resets and `stalled-idle` never trips. Observed on m1 today: the stall count reached 3 at 14:49 local and reset at 14:59 when "the shared ledger moved 0m ago" (narration.log lines 1963-1965); the same masking is recorded for m3 on 2026-08-31 (`idle-loss-2026-08-31.md:46-53, 140-143`). The specimens of `records/misc/seat-stop-analysis.md:10-25` are this path at the Stop event | at the Stop event or inside the turn | healthy, by construction |
| P12 | The seat runs on a runtime whose Stop hook is only declared: Codex and Devin are "declared; live observation pending" (`docs/design/turn-verdict-delivery-contract.md`, runtime table), and only Claude declares a live self-check (`internal/runtimes/runtimes.go:226`; `cmd/metasystem/hooks.go:25`) | at the Stop event | unknown; nothing has been observed |

Two facts bind these paths together. Every path that ends at the Stop event
depends on one shell script and one verb, and the verb's memory of what it
already refused is the escape. Every path that ends inside or between turns
depends on the steward, and the steward has no verdict for "a live seat, a
claimable backlog, no claim here".

## 2. The mechanisms, as they are today and as designed

### 2.1 The steward tick

Cadence: `TickSeconds` defaults to 600 seconds (`internal/steward/runner.go:52-59`);
`StaleTicks` defaults to 5 and `MaxRevivals` to 3 (`internal/steward/tick.go:36-41`).
So a stall is noticed after 50 minutes at the earliest.

Decision (`tick.go:339-375` assembles, `verdict.go:90-133` decides):

- `ReadOpenWork` on a converted checkout returns `WorkOwned` only when this
  machine's nickname holds a claim (`openwork.go:49-50`), `WorkNone` with the
  text "N queued goals await a claim; none is owned here" when goals are queued
  and none is claimed here (`openwork.go:59-60`), and `WorkDegraded` when the
  ledger cannot be read (`openwork.go:37-45`). The comment at `openwork.go:30-33`
  states the intent: "queued unclaimed goals stay visible, not revivable".
- `Decide` maps `WorkNone` to `VerdictNoWork, ActNone` (`verdict.go:95-96`)
  before it looks at any worker. So the census is not even run for an unclaimed
  backlog (`tick.go:347-354` runs it only for `WorkOwned`).
- With `WorkOwned` and a live worker: healthy while `TicksSinceProgress <
  StaleTicks`, otherwise `stalled-idle` with `ActNotify` and the text "a live
  holder is never displaced" (`verdict.go:100-108`).
- With `WorkOwned`, a complete census and no live worker: `stalled-dead`, and
  `ActRevive` unless a continuation is already open, the dry-revival cap is
  reached, or the provider is in outage (`verdict.go:116-132`).

What the tick does with each action:

- `ActNotify` queues one durable pending notification keyed on the verdict
  (`tick.go:215-226`), and the runner delivers pending notifications after every
  tick (`runner.go:131`) through `Deliver` (`internal/steward/notify.go:40-59`):
  the git-config command `metasystem.steward.notify-command` when set, the macOS
  `osascript` notification banner on Darwin when not, and a refusal on any other
  platform (`notify.go:25-36`). m1 primary: the config key is unset (`git config
  --get` exits 1), so every m1 notification is a macOS banner.
- `ActRevive` dispatches a `steward-continuation` delegate for the claimed goal
  (`revive.go:68-151`; the intent's `Role` is "always steward-continuation;
  recorded, never chosen", `internal/steward/intervene.go`). It revives OWNED
  work whose worker is provably dead. It cannot claim queued work, cannot touch a
  live seat, and does not restart a seat: the continuation is a delegate job in
  build mode (`stage.go:65-69`).
- `ActNone` writes nothing except the narration sentence.

Where the idle sentence goes: `Narrate` is best-effort and appends one line to
`artifacts/agents/narration.log`, capped at 2000 lines (`narrate.go:26, 36-42,
107-129`). The idle wording is composed at `narrate.go:148-149`. It is not a
noticing (`noticingsAt`, `narrate.go:222-277`, has four noticings: standing
outage, unexamined ledger, stall approaching, revivals building; none is
"idle with backlog"), so `ReachTheHuman` (`narrate.go:318-325`) never queues it.
It is not a digest entry (`NarrateDigest`, `narrate.go:47-95`, records ledger
moves, landings, breach stops, escalations and revivals only). It is not a
health role (next paragraph). The sentence is read by whoever opens the file.

Health roles the brief names (`health.go:54-55`, evaluated at `health.go:255-256`):

- `claimed-goal-appetite` iterates every claimed goal in the projection
  (`health.go:705-708`, not filtered by machine) and judges budgets, fences and
  breaches; with no claimed goal at all it is alive with "there are no claimed
  goals" (`health.go:800-802`). It has no verdict about queued goals.
- `claimed-goal-delivery` builds entries only for goals claimed by THIS machine
  (`delivery.go:63`) and returns alive with "there are no goals claimed by this
  machine" when the set is empty (`delivery.go:81-83`). The m0 critique confirms
  this reading (F-CONFIRM-011, `idle-rootcause-critique.md:68-72`). A dead
  verdict here carries `NoAutomaticRemedy` (`delivery.go:297-301`).

So a seat with no claim and eighty queued goals is, to the steward and to
health, at rest.

Progress marks (`marks.go:23-52`): checkout HEAD and the accepted goals ref tip.
Both move when ANY machine lands or mutates the ledger. This is P11.

### 2.2 The Stop hook and the turn verdict

Registration: the wrapper's `.claude/settings.json` (Stop timeout 5 seconds,
command runs the hook from `$CLAUDE_PROJECT_DIR/metasystem`); the shipped
template `scripts/enforcement/claude-code-hooks.json` runs it from
`$CLAUDE_PROJECT_DIR`. The nested copy that ran the hook when the session's
project directory is `metasystem/` was removed on 2026-09-02 (commit 55f10c44).

Flow on Stop (`supervision-hook.sh`): resolve the engine at the script's own
installation (23-25); refuse an unregistered runtime (32-40); resolve the world
from the payload cwd's git toplevel (50-66); record a hook attempt (84-104);
identify the caller (109-137); run `up` (148-156); health preview and narrator
digest (160-177); advisor early exit (232-247); watchdog report (256); write
`hooks.log` (263-265); call `report turn-verdict` (274-276); on success log
`stop verdict block=<bool>` (282-283) and emit either the block object
(290-297) or a `systemMessage` (298-305); on verdict failure emit a degraded
`systemMessage` (306-320).

The verdict (`turnverdict.go:164-191`): scan first (`cmd/metasystem/goal.go:543`),
then the run rules (`decideRuns`, 197-259), then the ladder (`decide`, 326-448).
The ladder, in order: Busy suppresses everything (355-361); open plan work
blocks once per signature (363-373); `WaitingOnHuman` is reported, no block
(375-382); unreadable inputs are displayed, no block (384-387); otherwise the
goal has the floor: a claimed goal blocks once per revision and once per queue
change (389-410), a queue with no local claim blocks once per queue digest
(411-425), a stale goal-free declaration blocks once (426-435), a degraded
ledger is displayed only (439-440). The claimed-goal test is by machine, not by
seat (483-484). The projection is read offline (`Project(endpoint, false, now)`,
line 476).

What it guarantees: exactly one refusal per unchanged state per session, for a
Claude session whose hook is loaded, whose engine resolves, whose world is the
right one, and whose hook finishes inside 5 seconds. Everything else is P1
through P6 and P12.

Where it is live: proven by fixture (`scripts/agents/supervision-fixtures.sh:1549-1555`
asserts the first block and asserts that the second identical Stop is NOT
blocked: "refused the same open work twice, which is the loop the design
forbids"). On the fleet: dead on m2, m3 and m0b since enrollment
(`supervision-hook-wrong-root.md:4`; `supervision-hook-root-design.md:48-52`);
not loaded on m0 during its idle night (`idle-rootcause-critique.md:20-24`); on
m1 not run since 2026-08-30 (section 3.4). This analysis found no machine on
which the gate is currently judging turns.

### 2.3 Revival

`PrepareIntent` mints an intent under the arbitration lock (`revive.go:31-63`);
`CompleteRevival` re-runs the decision, cancels if the world changed, consumes
the intent, counts the attempt against the dry cap before launch, and dispatches
(`revive.go:68-151`). It revives one thing: a claimed goal on this machine whose
worker is provably dead, by launching a delegate. It cannot revive an unclaimed
backlog (`WorkNone` never reaches it), cannot act on a live idle seat (the
ladder returns `ActNotify` for a live worker, `verdict.go:105-108`), and cannot
relaunch the seat process itself. The watch-verb design records the same limits
as code facts (`records/watch-verb/watch-verb-design.md:31-36`).

### 2.4 Health alerts and the narrator digest

`completeTickHealth` observes health, narrates the health line durably, and
calls `UpdateAlertEpisodes` (`tick.go:270-297`). Episodes are keyed on the digest
of every non-alive role and status (`health.go:1116-1124`); a dead role with no
lawful remedy alerts on its first observation (`health.go:386-389`); an episode
whose transport accepted the message is never retried
(`internal/steward/alert_episode.go:315-322`); the transport call runs inside
the alert lock (line 341); and the type comment says what a submission proves:
"it does not prove delivery to a person" (`alert_episode.go:17-18`). The m0
critique measured the consequence: seventeen submissions of the same hook
failure in forty-two hours, none acknowledged (F-ALERT-010,
`idle-rootcause-critique.md:62-66`). m1 primary today: nine episodes opened on
2026-09-02, all `TRANSPORT_SUBMITTED`, none acknowledged; the three read carry
`hook-freshness=dead` among their roles.

Who reads what:

- The health line: appended to `narration.log` every tick (`tick.go:283-284`)
  and shown to the seat at its own next Stop through `health --hook-preview`
  (`supervision-hook.sh:161`). A seat that has stopped does not stop again, so
  it never sees it.
- The narrator digest: `steward digest-pending` at the seat's next Stop
  (`supervision-hook.sh:166-176`); same limitation. Its durable copy is
  `records/narrator-digest.log`, git-tracked and merged across machines, which
  is why entries from several machines interleave there and why an escalation
  entry cannot be attributed to a seat from the file alone
  (`idle-loss-2026-08-31.md:107-111, 140-143`).
- `steward pending` at session start (`supervision-hook.sh:332-333`): only when
  a new session starts.
- The alert transport: a macOS banner on Darwin, a configured command
  elsewhere.

None of these reaches a human who is asleep or away, and none reaches the idle
seat.

### 2.5 The bound goals and the seventh

For each: state in the tree, what it would guarantee if landed as designed, and
its failure modes as its own records state them.

**idle-with-backlog-alarm** (`plans/goals/idle-with-backlog-alarm.md`, claimed
by m0, revision 13). Its Intent names the three real failures: block-once
(causal), `WorkNone` for unclaimed backlog (semantic), transport treated as
receipt (delivery) (line 4). Its Next step records a fork at "the every-runtime
wall" and Wido's choice of fork (a): land a Claude turn-exit gate now, split
every-runtime into its own goal (line 6). The fix under critique is NOT in this
tree: `cmd/metasystem/session_stop.go`, `internal/goal/sessionstop.go` and
`internal/goal/turnverdict_idle_test.go` exist on no ref fetched here (`git log
--all` over those paths is empty; the critiqued tree af5954fa is not a known
object). Its third critique (`records/misc/idle-fix-critique-r3.md`) returned
seven material findings, six critical, and named the terminal fact: the steward
is notify-only, so idle is not impossible on Codex or Devin (F-001, lines 11-15);
a live process is treated as proof of work (F-002, 17-21); the template
checkout's hook judges the wrong state root (F-003, 23-27); world detection
fails open (F-004, 29-33); the 5 second deadline is not end to end (F-005,
35-39); the session-stop library can be called without human classification
(F-006, 41-45); the marker's lifecycle does not bind to a no-hook session
(F-007, 47-51). This goal and turn-verdict-hardening both redesign
`internal/goal/turnverdict.go` and both mint a human stop marker
(`session_stop.go` versus `goal humanstop`, `plans/turn-verdict-hardening-design.md:884`).
That is two live claims on one decision owner.

**turn-verdict-hardening** (`plans/goals/turn-verdict-hardening.md`, claimed by
m0b, revision 4, priority-1). Design at revision 3
(`plans/turn-verdict-hardening-design.md`), the most complete statement of
guarantee parts (1) and (4) in the tree: READY as a seat-scoped predicate with
no block-once (§1, lines 97-371); INFLIGHT joined to the ready goal by goal and
revision with exact process identity (§2, 373-456); a fail-closed outcome table
with two classes, where class B (no verdict reached) blocks with a repair
command and consults no marker (§3, 458-635); a Stop budget with one emitter
and an entry clock (§3.0-3.2, 511-817); freshness by bounded fetch (§4,
819-858); HUMANSTOP minted by the enrolled terminal or, by ruling R-47-m0b, a
relayed word, consumed atomically at one Stop (§5, 860-1034). Its own residuals
(§8, 1088-1137) are the honest boundary: hooks disabled or untrusted; Codex and
Devin unobserved; the runtime killing the hook before emission (F18); emission
failure (F19); `up` leaving the Stop path so a dead supervision re-arms only at
the next session start. Its critique round 3
(`records/misc/turn-verdict-hardening-critique-r3.md`) left four material
findings open (a delegate can mint its ancestor's marker; F11 is discovered
after the marker phase; the marker path embeds an unvalidated machine nickname;
the recovery commands are unquoted). It is sequenced behind
supervision-hook-wrong-root (§9 ask 7, lines 1160-1171). Its slices 1a through
4b are each cut to 240 reserved minutes (§10, 1197-1255).

**supervision-hook-wrong-root** (`plans/goals/supervision-hook-wrong-root.md`,
queued, revision 11). Design at revision 3
(`plans/supervision-hook-root-design.md`): the hook resolves its world from the
engine's own executable location through a new `path state-root` verb, maps a
linked delegate worktree to its primary, and turns engine skew into a visible
line (Decisions 1 and 2, lines 59-393). Critique round 3
(`records/misc/hook-root-critique-r3.md`) left two material findings (the
`METASYSTEM_BIN` override splits one turn across two worlds; the worktree
mapper trusts inherited git steering variables). The goal's Next step says it
waits on Wido for an attempt-fence word (line 6); its Budget line now shows
`attemptLimit=10` after the 2026-09-02T06:53:09Z set-budget by m0b, consistent
with the R-45-m0b re-box, while the Next step text still says it waits. What it fixes: P3. What it does not touch: P4 (a hook that is not
loaded resolves no world at all), P5, P12. Its failure map keeps every
identification failure a silent exit 0 (design lines 365-377), which
turn-verdict-hardening's class B then converts to a block (design §3, 496-509).

**watch-verb** (`plans/goals/watch-verb.md`, queued, revision 14). The
zero-write read surface is landed and its final critique closed with zero
material findings (`records/watch-verb/watch-verb-final-critique.md:4-5`). The
acting side is designed as a ladder (`records/watch-verb/watch-verb-design.md:94-117`)
whose only candidate class is `W-RECOVER`: one recovery round for a goal-bound
root implementer job that died by `process-lost` or `budget-cap` (taxonomy,
lines 215-231), admitted only through marking mode, a committed class manifest,
a per-machine Law 2 record written by a human-only verb, and Wido's promotion
after ten adjudicated samples over seven days (lines 353-426). There is no class
for "a live seat is idle while backlog is claimable", no class that touches a
seat, and the design says so: "W-HEAL is not on the ladder" (line 114) and
"Kills remain out of scope" (line 230). Its read surface has eight sections
(jobs, completed-rounds, census, health, delivery, alerts, intents,
breach-routes; `internal/watch/watch.go`) and no idle-seat item. Landing
watch-verb as designed does not consume the idle verdict, because no idle
verdict exists to consume.

**alert-escalation-channel** (`plans/goals/alert-escalation-channel.md`,
queued, revision 37). Design at revision 13 (`plans/alert-channel-design.md`).
Paused at a scope fork that Wido resolved by R-45-m0b word 2 (facts only, one
fold then build; `memory/rulings.md:89`). Slice 1 (design §11, lines 989-1071):
the Telegram alert path, the journal/transport split, and exactly two producers
by Wido's 2026-09-01 word, `delegate-job-failed` and `stop-awaiting-resume`
(§7, 886-903; §11a.8, §11a.9). The undelivered-alert count joins the health line
(§10, 979-987). The inbound half is explicitly not this design's and is assigned
to seat-mutual-awareness (§2b, 520-551). The legacy pending queue, which is
where a `stalled-idle` verdict goes today, is retired in slice 5 with "every
`QueueNotification` caller migrated" (line 1055). There is no producer for
"seat idle with claimable backlog" in any slice, because nothing produces that
fact. Landing this goal as designed carries job failures and breach stops to a
phone; it does not carry idleness.

**seat-mutual-awareness** (`plans/goals/seat-mutual-awareness.md`, queued,
revision 7). No design file exists in `plans/` (listing checked). Its Next step
carries Wido's binding word for the inbound loop: every inbound message on the
external channel carries a TOTP code (line 6). If built, seats gain a second,
authenticated instruction channel and a peer-question path. It does not
observe the primary channel (the runtime's own input), does not detect its
failure, and does not nudge a seat.

**idle-every-runtime-enforcement** (`plans/goals/idle-every-runtime-enforcement.md`,
queued, revision 1, opened by Wido 2026-09-02T15:54:37Z). Not among the six
bound goals. It owns exactly the missing act: "the steward must ACTIVELY
RE-ENGAGE an idle backlogged seat" (line 4), gated by marking mode and Law 2,
composing with watch-verb (line 6). Two facts qualify it. Its premise "The
claude case is handled by the landed turn-exit gate" (line 4) is false in this
tree: no turn-exit gate beyond the block-once one has landed anywhere reachable
(section 2.5, first entry). And its scope is "codex/devin seats, not only
claude"; a Claude seat that bypasses the hook through P4, P5, P8, P9 or P10 is
not in its stated scope.

## 3. The specimen map

Each row: the mechanism that should have caught it; whether it existed at the
time; whether it fired; why the loop did not close; evidence.

### 3.1 m0's eight-hour idle night, 2026-09-01

Relayed: m0's artifacts were not reachable; the evidence is
`idle-with-backlog-alarm.md:4` and the codex critique of m0's own analysis,
`records/misc/idle-rootcause-critique.md`.

| Mechanism | Existed | Fired | Why the loop stayed open |
| --- | --- | --- | --- |
| Stop hook + verdict (P1, P4) | yes in the tree | no | the session ran from `.../m0/agentic-tools/metasystem` under Claude Code 2.1.237 with no metasystem hook loaded (F-CONFIG-003, lines 20-24); and had it run, the queue clause blocks once per digest (F-PROPERTY-002, lines 14-18, citing `turnverdict.go:411-425`) |
| Steward idle detection (P1, P11) | yes | no action | unclaimed backlog is `WorkNone`, mapped to `ActNone` (F-OWNER-006, lines 38-42, citing `openwork.go:30-63` and `verdict.go:90-97`) |
| Health alert on the dead hook | yes | yes | the steward opened episode `alert-99020c96056dc6af-1` and the transport accepted it 96 minutes before the human intervened (F-ROOT-001, lines 8-12); transport acceptance proved neither receipt nor acknowledgment, and a submitted episode is never retried (`alert_episode.go:315-322`) |
| claimed-goal-delivery | yes (landed 2026-09-01, `aad398c2`) | alive | no claim held by m0, so "there are no goals claimed by this machine" (`delivery.go:81-83`; F-CONFIRM-011) |
| Revival | yes | no | nothing owned, nothing dead |
| External delivery | no | not applicable | alert-escalation-channel unbuilt (`alert-escalation-channel.md:3`) |

Gap in this row: the alert episode file and m0's narration are cited from the
critique, not read here.

### 3.2 m3's six silent hours, night of 2026-08-31 to 2026-09-01

Relayed: `records/misc/idle-loss-2026-09-01.md`, written by the m3 seat.

| Mechanism | Existed | Fired | Why the loop stayed open |
| --- | --- | --- | --- |
| Reaper marks the dead worker | yes | yes, 82 seconds in (record lines 17-19) | "no mechanism carried that fact to the delegator or the human" (lines 38-45); the alert channel's classes did not include a failed delegate job until Wido's word that morning |
| Steward stalled-idle (P8, P11) | yes | not recorded in the incident | m3 held a claim and a live main, so the verdict path is `stalled-idle` after five stale ticks with `ActNotify` (`verdict.go:100-108`); the record does not state whether it tripped; on a Mac the delivery would have been an `osascript` banner (`notify.go:32-34, 48-50`) at 22:17Z with nobody watching. If m3's fleet peers moved the ledger during the night, the marks reset (P11) |
| Breach-stop custodian | yes | yes, at 04:15-04:17Z (lines 23-26) | "It notifies nobody and revives nothing" (line 26); the custodian runs inside the tick (`tick.go:69-99, 153`) and returns a report, no alert |
| claimed-goal-delivery | not provable from here | unknown | the role landed 2026-09-01 (`aad398c2`); whether m3's engine carried it at 22:17Z the night before is not provable from this machine |
| Revival | yes | no | the worker that died was a delegate job; the seat (the "worker" the census counts) was alive, and "a live holder is never displaced" |
| The seat's own wait | conduct | no | an unbounded wait on a return file a failed worker never writes (lines 20-23) |

### 3.3 The three seat-stops in `records/misc/seat-stop-analysis.md` (m3 2026-08-31, m0b 2026-09-01, m0b 2026-09-02)

These are the specimens of the bound goal turn-verdict-hardening, listed here
because the umbrella's guarantee (1) must refuse them. All three ended at a
Stop event with ready work on the board (`seat-stop-analysis.md:10-25`). The
mechanism that should have caught them existed (the gate) and did not fire
correctly: on the fleet's nested layout the hook judged the wrong world (P3),
and against the right world the gate has the four escapes the fold names,
block-once, existential INFLIGHT, fail-open, relay-minted HUMANSTOP (lines
128-147, from the Sol critique `records/misc/seat-stop-analysis-critique-r1.md`).
Evidence is the goal histories the hardening design read: the m0b pair had
released every claim before the third stop and `account-provenance` was queued
with a complete stored budget (`turn-verdict-hardening-design.md:90-95`).

### 3.4 m1, first idle on 2026-09-02: a permission classifier

The goal attributes m1's first idle of the day to "waiting on a permission
classifier" (`never-idle-ironclad.md:4`). The tree holds no record of that wait
beyond the goal (searched `plans/`, `records/`, `memory/`, `docs/` for
"permission classifier" and "classifier": only the goal and the brief match).
What m1 primary shows around it:

- The steward runner restarted at 2026-09-02T11:21:42Z (`steward/runner.json`),
  and the main seat announced at 11:35:48Z (`mains/session-2840-2840.json`,
  `supervision/arming.log`). The narration line at 13:35 local (11:35Z) reads
  "m1 is idle while work waits in the queue (84 queued goals await a claim;
  none is owned here)" (`narration.log:1949`); at 13:46 local the seat is
  "working on breach-clock-and-budget-honesty" (line 1951).
- Alert episode `alert-02edd24f6026e14c-1` opened at 11:35:47Z with
  `steward-runner=dead ... [failure 5/5; auto heal ended]` among its roles and
  was `TRANSPORT_SUBMITTED`; it concerns the re-arm, not the seat.

Which mechanism should have caught a seat frozen inside a permission prompt:
none exists (P8). No Stop event fires inside a turn; the steward saw `WorkNone`
because no claim was held; no health role reads a seat's prompt state.

The Stop hook on m1, stated once for every m1 row: the metasystem world's hook
component evidence does not exist (`steward/components/` holds only
`supervision-hook.flock`), so `hook-freshness` is dead with "no hook turn
generation is recorded" and `NO_LAWFUL_REMEDY` (`steward/health.json`, observed
2026-09-02T15:54:52Z; every one of the 423 health lines in the current
`narration.log` window says `hook-freshness=dead`). The wrapper world holds the
hook's misdirected evidence: `components/supervision-hook.json` at generation 4,
last attempt 2026-08-30T19:06:23Z, outcome `ATTEMPTING`, with generations 1 to
3 each ending `ERROR / INTERRUPTED_BY_NEXT_TURN`; the wrapper's
`supervision/hooks.log` is empty (0 bytes, created 2026-08-30 18:46 local). The
metasystem world's `hooks.log` last line is `2026-08-27T06:12:37Z stop verdict
block=false`; `turn-verdict-state.json` was last written 2026-08-27 08:12
local. `arming.log` has two entries today, both at 11:35:48Z (session start),
none at any Stop. Conclusion: on m1 the hook has recorded no Stop attempt in
either world since 2026-08-30T19:06:23Z and the verdict has not run since
2026-08-27. The cause is not proven from files alone. The two candidates are P4
(the session's project directory is `metasystem/`, whose own settings file was
deleted at 12:31 local today and may not have been loaded before that either)
and P5 (the hook is killed inside 5 seconds before line 92, since `up` and the
ceremonies run first). Distinguishing them needs a live probe, which is exactly
what F-PROBE-004 says the current check cannot do.

### 3.5 m1, second idle on 2026-09-02: a decision-ask ends the turn at 15:45 local

| Mechanism | Existed | Fired | Why the loop stayed open |
| --- | --- | --- | --- |
| Stop hook + verdict (P1, P4, P5, P7) | yes | no | no attempt and no verdict recorded (section 3.4); had it run, the queue clause would block once per digest and the seat's next quiet stop would pass (`turnverdict.go:411-425`) |
| Steward (P1) | yes | narration only | thirteen consecutive ticks from 15:41 to 17:54 local read "m1 is idle while work waits in the queue (88 queued goals await a claim; none is owned here)" (`narration.log:1973-1999`); each is `WorkNone` and `ActNone` |
| Health | yes | alive for the two roles that matter | `claimed-goal-appetite` alive listing other machines' claims; `claimed-goal-delivery` alive with "there are no goals claimed by this machine" (`health.json` at 15:54:52Z); `hook-freshness` dead, unchanged, so no new episode from idleness |
| Alerts | yes | submitted | nine episodes on 2026-09-02, all `TRANSPORT_SUBMITTED` to the macOS banner, none acknowledged; none says the seat is idle |
| Revival | yes | no | no claim held |
| External delivery | no | not applicable | unbuilt |
| The human | the monitoring system | yes | the goal was opened by the woken seat at 15:55:13Z (17:55 local), two hours and ten minutes after the first idle line |

The goal Intent says 84 goals were queued at 15:45; the narration says 88 at
15:41. The difference is four goals opened between the seat's count and the
tick's count; both are quoted as written.

### 3.6 m0, m0b and paper: stranded remote-control messages, 2026-09-02

Relayed: `never-idle-ironclad.md:4` only. No record, log line or goal in the
tree describes the event (section 1, P10). Which mechanism should have caught
it: none exists. The seat announcement carries no channel identity
(`mains/session-2840-2840.json` fields: sessionId, mainId, pid, pidStartedAt,
pgid, runtime, instanceTag, commandHash, announcedAt, identityProvenance).
Nothing in `internal/steward` reads a tmux pane, a remote-control queue, or any
inbound surface. From the steward's side the three seats were simply seats with
or without claims; if they held claims, P11 applies (a busy fleet resets the
marks); if not, P1 applies (`WorkNone`). Evidence gap: the seats' own
narration and health records were not readable from here, so whether any
`stalled-idle` notification was queued on those machines is unknown.

## 4. The gap map against the four-part guarantee

The guarantee, from the goal (line 4): (1) a seat's turn cannot end while
claimable backlog exists and no human stop word is recorded, fail-closed on
every hook failure path; (2) if a seat is nonetheless idle with backlog, the
steward notices within one tick, re-injects the continue order through the
seat's declared channel, and re-launches a dead session where lawful; (3) when
(1) and (2) fail within a bound, the human is reached externally; (4) the only
quiet exit is the human's recorded stop word.

### Part (1): the turn cannot end

| | |
| --- | --- |
| Exists | the block-once gate for Claude sessions whose hook is loaded, resolves the right world, and finishes in 5 seconds (section 2.2); a fixture that asserts the second identical Stop passes (`supervision-fixtures.sh:1553-1555`) |
| In flight | turn-verdict-hardening (READY every Stop, relevant INFLIGHT, fail-closed table, Stop budget, freshness); supervision-hook-wrong-root (reach the verdict on nested layouts); idle-with-backlog-alarm fork (a) (a parallel gate on the same file, under critique) |
| Missing entirely | proof that the hook is loaded and completing in the live session, not just present in a file (P4, P5): `hooks check` is a static file comparison (`validate-metasystem.sh:2187-2203`), `hook-freshness` detects the absence after the fact and alerts with no remedy (`health.go:290`, `health.go:416-445`), and no goal owns making a session refuse to run without a live hook; runtime coverage for Codex and Devin (P12), named as a residual by turn-verdict-hardening §6 and §8 and owned by nobody as a build; the ordering hazard of two designs on one owner (idle-with-backlog-alarm and turn-verdict-hardening) |

If all six bound goals landed as designed, part (1) would hold on a Claude seat
whose hook is loaded. It would not hold for a session whose hook is not loaded,
for Codex or Devin, or for the runtime-kill path F18, all three of which the
hardening design records as residuals rather than closes.

### Part (2): the steward notices within one tick, nudges, relaunches

| | |
| --- | --- |
| Exists | detection of a stalled or dead worker on OWNED work after five ticks (`verdict.go:100-132`); revival of owned work by a delegate job (`revive.go`); nothing for an unclaimed backlog (`openwork.go:59-60`); no channel identity for any seat; no write to any seat's input |
| In flight | idle-with-backlog-alarm's semantic fix (count claimable backlog as work) with no acting side (F-001); idle-every-runtime-enforcement (re-engage or restart, Codex and Devin scope, marking mode and Law 2, premise false); watch-verb's acting ladder, whose only candidate class recovers dead implementer jobs |
| Missing entirely | a verdict for "live seat, claimable backlog, no claim here" (the steward has no such verdict; the narration line is not a decision); a seat channel registry (which tmux pane, which remote-control queue, which process to signal) and the act of writing a continue order to it; relaunch of a dead SEAT (revival launches a delegate for a claim, `stage.go:16-17`, never a seat, and only when a claim exists); a progress definition that is this seat's progress rather than the fleet's (P11, `marks.go:43-52`); one-tick latency (today's stall detection is five ticks, and the idle case has no detector at all) |

The seat-nudge is confirmed unowned for Claude seats and owned only in name for
Codex and Devin by a goal outside the six that rests on a false premise
(section 2.5, last entry). The instruction-channel failure is confirmed
unowned: no goal, design, record or code mentions the seat's inbound channel
(section 1, P10; section 3.6).

### Part (3): the human is reached externally within a bound

| | |
| --- | --- |
| Exists | the legacy pending queue delivered to a macOS banner or a configured command (`notify.go`); alert episodes with a journaled transport attempt, no receipt, no retry after acceptance (`alert_episode.go:17-18, 315-322`); an acknowledgment verb (`AcknowledgeAlert`, `alert_episode.go:366-392`) |
| In flight | alert-escalation-channel slice 1: Telegram, single-flight sender, two producers (`delegate-job-failed`, `stop-awaiting-resume`), undelivered count on the health line; later slices retire the legacy queue and migrate its callers |
| Missing entirely | a producer for idleness of any kind (neither "idle with backlog" nor `stalled-idle` is a slice-1 producer; `stalled-idle` rides the legacy queue until slice 5); a stated bound from idle onset to external alert (nothing measures idle onset today); receipt semantics beyond transport acceptance (R-45 word 2 chose facts only; acknowledgment exists as a verb, nothing escalates on its absence) |

### Part (4): the only quiet exit is the human's recorded stop word

| | |
| --- | --- |
| Exists | nothing a program can read ("Today it lives nowhere a program can read", `turn-verdict-hardening-design.md:862-863`); the standing order is prose (`docs/orchestration.md:132`) |
| In flight | HUMANSTOP: enrolled-terminal or relayed word (R-47-m0b, `memory/rulings.md:91`), one marker per seat, compare-and-consume at one Stop (hardening §5); m0's session-stop marker (`session_stop.go`, not in this tree), which the critique found mintable without human classification (F-006) and unbound to a no-hook lifecycle (F-007) |
| Missing entirely | one owner (two markers are designed); consumption on a runtime with no Stop event (F-007: the marker is consumed only when `TurnVerdict` runs); the redirect clause of the order ("or explicitly directing the work elsewhere") is handled by the hardening design as the ordinary redirect verbs recomputing READY (§5.2, lines 996-998), which is sufficient if READY is the gate and unstated for the steward side |

### Holes that survive all six goals as designed

1. Hook loading and completion in the live session (P4, P5). Detected after
   the fact by `hook-freshness`, never prevented, never repaired; the m1 seat
   has been in this state since 2026-08-30. Unowned.
2. The seat nudge: re-injecting the continue order through a declared channel.
   Unowned for Claude; owned in name for Codex and Devin by
   idle-every-runtime-enforcement, whose premise is false. The channel registry
   the nudge needs does not exist anywhere.
3. The instruction channel failing silently (P10). Unowned; unrecorded.
4. An idle-with-backlog verdict in the steward, and a producer carrying it
   externally. The steward has no such verdict and the alert channel has no
   such producer. idle-with-backlog-alarm's semantic fix would create the
   verdict; nothing routes it.
5. Stall masking by fleet motion (P11). `marks.go` measures the fleet, not the
   seat. The absorbed goal open-work-scanner-blindspots covers the Stop-time
   scanner (`stop-message-truth.md:4`), not the steward's marks. Unowned.
6. Seat relaunch (P9 with no claim, or with a claim whose worker is a seat).
   Revival launches delegates for claims. Unowned.
7. Two designs on one decision owner (`turnverdict.go`) with two human stop
   markers. A coordination hole, not a code hole.
8. Codex and Devin Stop delivery (P12). A residual in every design that
   mentions it; unobserved; unowned as a build.

## 5. The split

Slices of at most 240 reserved minutes (the R-44-m0b and R-45-m0b standing
tuple, small 4h/10/240m/1, `memory/rulings.md:86, 89`), in dependency order.
Each names the fixture that replays a specimen and must refuse or recover it.
This is a proposed arc for the design round, not a design: it says what each
slice must prove, not how.

| # | Slice | Depends on | Fixture replaying a specimen | Guarantee part |
| --- | --- | --- | --- | --- |
| A | Hook reaches the right world: supervision-hook-wrong-root as designed at revision 3 with its two open findings folded | nothing | `nested-root` case 1 (fleet layout, Stop from the nested checkout blocks on the sentinel) and case 4 (delegate worktree maps to the primary), `supervision-hook-root-design.md:408-463`; replays the m3, m0b and m0b seat-stops of 3.3 at the world level | (1) |
| B | Hook is provably loaded and completing in the live session: a session-start proof driven through a real Stop event (a nonce round trip, not a file read, per F-PROBE-004), `hook-freshness` gaining an applicability rule and a session join (F-FRESHNESS-005), and `up` or the seat refusing to proceed when the proof fails | A | replay of 3.4/3.5: a Claude session whose project directory is `metasystem/` with no nested settings file must either fire the hook on Stop or refuse to start; the m1 wrapper-root evidence file (generation 4, ATTEMPTING since 2026-08-30) is the negative fixture shape | (1) |
| C | The gate refuses READY every Stop and fails closed: turn-verdict-hardening slices 1a and 1b | A | `TestReadyBlocksEveryStopWithoutMemory`, `TestSpecimen1_M3HoldBlocks`, `TestSpecimen2_M0bFenceStopBlocks`, `TestSpecimen3_M0bBoardStopBlocks` (design §10, line 1249); plus a new replay of 3.5: 88 budgeted queued goals, no claim here, two consecutive Stops, both blocked | (1) |
| D | One emitter, entry clock, bounded hook: turn-verdict-hardening slices 2a and 2b | C | `hook-single-response`, `hook-clock-at-entry`, `hook-budget-verdict-hang` (design §10, lines 1250-1251); replays P5 as observed on m1 (INTERRUPTED_BY_NEXT_TURN generations) | (1) |
| E | The steward has an idle-with-backlog verdict: `ReadOpenWork` or a sibling reports claimable backlog with no local claim as a first-class state, the ladder decides on it within one tick, and the census runs for it | nothing (parallel to A-D) | replay of 3.5 at the tick: 88 queued budgeted goals, no claim, a live announced main; one tick yields the new verdict, not `no-work`; replay of 3.1 (m0's night) as the same shape at 8 hours | (2) |
| F | Idleness reaches the human externally within a stated bound: the new verdict and `stalled-idle` become alert-channel producers with an onset clock, joining slice 1's two producers; receipt or acknowledgment absence escalates | E, and alert-escalation-channel slice 1 | replay of 3.1: verdict at tick N, episode journaled at tick N, transport attempt within one sender pass, a second attempt when unacknowledged past the bound; replay of 3.2 for the failed-job class already in slice 1 | (3) |
| G | Seat channel registry and the nudge: the announcement declares the seat's inbound channel (a tmux pane, a remote-control queue, a pipe), the steward re-injects the continue order on the new verdict, and the act enters watch-verb's Law 2 manifest as a marking-mode class first | E; design-bearing (R-38-m2 ladder, joint round likely per R-39-m0) | replay of 3.5 and 3.6 with a fake seat whose declared channel is a pipe: the idle verdict produces a `WOULD_ACT` record in marking mode and, once promoted, the order arrives in the pipe within one tick; a second fixture with the pipe closed proves the failure is journaled and escalated | (2) |
| H | Instruction-channel liveness: the declared channel carries a heartbeat or acknowledgment, and a stranded order is detected and escalated | G | replay of 3.6: an order written to the declared channel with no acknowledgment inside the bound becomes an episode and a nudge through the second channel | (2), (3) |
| I | Dead seat relaunch where lawful: revival extended from "a delegate for a claim" to "a seat for a claimable backlog", under the same intent, fence and dry-cap machinery | E, G | replay of P9: an announced main proven dead with budgeted queued goals and no claim; one tick mints an intent; the launch is a seat, not a delegate; the dry cap holds | (2) |
| J | The human stop word, one owner: turn-verdict-hardening slices 4a and 4b, with consumption also reachable from the steward side so a no-hook runtime can honour it (F-007) | C, D | `TestHumanstopConsumedByExactlyOneOfConcurrentStops`, `TestHumanstopConsumesOnlyAgainstClassABlock` (design §10, line 1254); a new fixture: a marker set for a Codex-shaped seat is consumed by the steward's idle verdict path, not only by a Stop | (4) |
| K | Freshness: turn-verdict-hardening slice 3 | C | `TestFreshnessFetchTimeoutBlocksNoReady`, `TestFreshnessNoTimeWindowExists` (design §10, line 1252) | (1) |

Slices A, C, D, J and K are the existing hardening and wrong-root work in their
own cut; slices B, E, F, G, H and I are new. Slices G and I carry acting-side
authority and must travel the marking-mode and Law 2 path the watch-verb design
fixed (`watch-verb-design.md:94-117, 353-426`); they are the ones most likely to
need a joint round under the R-39-m0 finding.

Existing goals, disposition proposed for the design round:

- turn-verdict-hardening: left as it is; it is the owner of parts (1) and (4)
  at the Stop boundary (slices C, D, J, K).
- supervision-hook-wrong-root: left as it is; first in order (slice A).
- idle-with-backlog-alarm: re-scoped. Its semantic half (the steward counts
  claimable backlog as work) becomes slice E, which nothing else owns. Its
  causal half (a second turn-exit gate) and its session-stop marker duplicate
  turn-verdict-hardening on the same file and should yield to it, so that one
  design owns `turnverdict.go` and one marker exists. This needs m0's and
  Wido's word because both goals are claimed.
- idle-every-runtime-enforcement: re-scoped to runtime-neutral seat
  re-engagement (slices G and I), with its premise corrected: no Claude gate has
  landed, and a Claude seat that bypasses the hook needs the same
  re-engagement.
- watch-verb: left as it is; slices G and I register their classes through its
  manifest rather than a parallel ladder.
- alert-escalation-channel: left as it is; slice F adds producers after slice 1
  lands.
- seat-mutual-awareness: left as it is; slice H uses its authenticated second
  channel as the fallback route but owns the primary channel's liveness itself.
- New goals needed: hook-session-liveness (slice B), seat-channel-and-nudge
  (slices G and H), seat-relaunch (slice I). Slice F may be a slice of
  alert-escalation-channel or of the umbrella; that is a design-round choice.

## 6. Self-grade

Confidence: high on the code facts of sections 1 and 2 (every line was read in
this worktree at 6f57cb58) and on the m1 evidence of sections 3.4 and 3.5
(files read on the primary checkout the same afternoon). Moderate on the
specimen rows for m0, m3, m0b and the paper seat, which rest on records other
seats wrote and on one critique; their artifacts were not reachable. Moderate
on the disposition of idle-with-backlog-alarm, which rests on reading its goal
file and its third critique, not on the code under critique, since that code is
on no ref fetched here.

Weakest claim: the cause of the dead Stop hook on m1 (section 3.4). The facts
are that no attempt has been recorded in either world since 2026-08-30T19:06:23Z
and the verdict has not run since 2026-08-27; whether that is because the hook
is not loaded for a session whose project directory is `metasystem/` (P4) or
because the 5 second timeout kills it before the attempt record (P5) is not
decided by any file. Slice B is the probe that decides it. If the cause is P5
alone, slice B shrinks to the hardening design's slice 2a and the "unowned
hole" of section 4 item 1 narrows to detection-only.

Reject this analysis if any of these is shown: a code path by which an
unclaimed queued goal produces a steward action other than narration
(`openwork.go:59-60` and `verdict.go:95-96` would then be misread); a health
role that goes dead for a seat with no claim and queued work (`delivery.go:81-83`
and `health.go:800-802` would then be misread); a Stop verdict recorded on m1
after 2026-08-27 in either world; any goal, design or record in the tree that
names a seat's inbound channel or a nudge to a live seat (the searches in
section 1 would then have missed it); or a fetched ref carrying m0's
session-stop code, which would move that goal's disposition from "not in the
tree" to "landed elsewhere" and change slice E's starting point.
