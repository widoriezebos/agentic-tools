# Design: the brain seat boots with its standing instruction (revision 3)

Goal: fleet-coordinator-brain (arc headless-fleet, tier 3). The role packet is
landed at records/misc/fleet-coordinator-brain-role-packet.md; its four rules
(never builds, never dispatches, never approves, never a bottleneck) are fixed
and this page does not restate them. What the goal still owes, in Wido's words:
"the standing instruction wired into the seat's boot so it loads unprompted,
and the narrator's digest as its input." Revision 1 was written 2026-09-06,
the day a seat again stood in for the brain and the runner at once; revision 2
folded the first critique; revision 3, written 2026-09-07, folds the second
and is the last design round: the page goes to the build behind its fixtures
next, so every decision below is one a builder implements without asking.
Every claim about the tree cites file and line as they stand in this worktree
on 2026-09-07.

## Where each critique finding lands

| Finding | Section | One line |
| --- | --- | --- |
| BRAIN-R2-01 | 1 | The host pointer is written under the lock package with compare-and-swap; the visibility line is produced at brain boot and every brain turn end, the status post carries it; one brain per fleet across hosts is a human rule, stated as such |
| BRAIN-R2-02 | 1 | Fleet identity is the ledger identity the goal package already reads, a 26-character ULID; no remote URL is stored, compared, or emitted anywhere |
| BRAIN-R2-03 | 1 | Declare refuses while the checkout holds a claim, a live job, a live run, or an active mission, and prints the release or cancel command to run first |
| BRAIN-R2-04 | 2 | The hook decides nothing about the role; without the engine or the boot verb it prints one notice naming the failure and injects nothing; the shell fallback is gone |
| BRAIN-R2-05 | 2 | The standing-instruction section is composed by a fast path before any other input is read; the rest runs in a bounded child the engine kills at the deadline; the payload names what was cut; the deadline fixture stalls a reader with a named pipe |
| BRAIN-R2-06 | 1, 2 | Declaration fields are capped at declare time; the registry bound has a minimum the engine refuses below; the header and the standing-instruction section always fit |
| BRAIN-R2-07 | 2 | The runtime's session-start matcher gains the compact source from the registry's declaration, and the payload is re-injected then |
| BRAIN-R2-08 | 3 | Cancel, close, and reap are fenced like launch in dispatch.sh and in the delegate verb; breach-stop stays the one exemption, with the reason |
| BRAIN-R2-09 | 3 | Classification runs first at the request-assembly seam and fails closed on the brain checkout; discharge-review-obligation inherits the check because the seam owns the classification |
| BRAIN-R2-10 | 8 | Corrupt declaration at every guarded act, the cancel, close and reap routes, classification failure, a stalled reader, the compact source, two registry homes, and the status line in a script the validation driver runs |

## 0. What is true today, read in the tree

- **What a seat loads unprompted.** CLAUDE.md is three lines: read AGENTS.md,
  load wow.md on demand. Nothing names the brain, the packet, or a role.
- **The boot seam.** The runtime's SessionStart hook runs
  `scripts/agents/supervision-hook.sh <runtime> start` with a 15-second
  timeout on the sources `startup|resume|clear`
  (scripts/enforcement/claude-code-hooks.json:6-11; the codex and devin
  templates carry the same command). The Claude Code hooks reference names a
  fourth source, `compact`, fired after context compaction, and caps a hook's
  context string at 10,000 characters; both facts are the runtime's manual,
  not the tree. In the hook, when the engine binary is absent the start event
  exits silently and only the Stop event prints a refusal
  (supervision-hook.sh:351-356). The start path resolves the checkout and
  the state root (381-402), prints pending steward incidents (914-915), then
  identifies the agent process and runs `up` (935-955); every message is a
  separate `{"systemMessage": ...}` object from `surface_json` (513). The
  runtime registry declares per-runtime session facts the hook reads
  indirectly, for example `runtime session-env` (main.go:330,
  internal/runtimes/runtimes.go:58-61); exit 1 from such a query is the
  declared absent capability. The registry validates its declarations in
  `Validate` (runtimes.go:385).
- **The digest.** `steward digest-pending` returns the narrator log after one
  machine-local cursor (internal/narratordigest/digest.go:225-250); the cursor
  path is fixed at artifacts/agents/steward/narrator-digest-cursor.json
  (63-65) and advances only through `digest-advance` after the Stop payload is
  emitted (supervision-hook.sh:641). A malformed cursor or a log changed
  before the cursor is an error (digest.go:201-222, 241-243). There is one
  cursor per checkout, and it is the human's.
- **The Stop verdict.** `report turn-verdict` reads the claimable budgeted
  backlog for every agent caller (internal/goal/turnverdict.go:226) and
  `enforceIdleBacklog` refuses the stop three times for an unchanged claimable
  backlog (293); at the third refusal `escalateIdleBacklog` selects the
  machine's held or first claimable goal, prepares a steward continuation
  intent, and records the incident (401). The display is composed from a run
  prefix, the ladder's text, and the green lines (616-624); the ladder (627)
  is one switch where Busy, Open, WaitingOnHuman, and Unreadable each end the
  turn's text before the goal clause runs (656-676 and the default branch).
  The scanner (internal/report/scan.go:25-72) reads jobs, gates, missions
  (50), runs (61), and plans; it reads no channel question and no goal
  record. In-flight job statuses are pending and running
  (internal/report/openwork.go:50).
- **The acts.** Every operator dispatch enters `runDelegate`
  (cmd/metasystem/delegate.go:29-103), which execs dispatch.sh; `--cancel`
  maps to the shell's `cancel` command (224-229). A direct dispatch.sh call
  without the internal marker exits through the legacy-grammar guard for
  `dispatch`, `follow-up`, `cancel`, and `--*` (scripts/agents/dispatch.sh:28-35)
  before the router; the router (2894-2905) reaches `dispatch_job`,
  `follow_up`, `cancel_job` (2494-2513, which marks a husk or hands the
  runtime adapter a cancel that winds down a process group), `close_chain`
  (2515-2568, which mutates the root record), and `reap_jobs` (2577); `close`
  and `reap` are not covered by the guard. The steward tick's breach-stop
  runs through `internal_breach_stop_goal` (2741-2757) under the
  stop-custodian authority on a goal revision this checkout holds. Every
  landing enters scripts/agents/land.sh, which calls scripts/agents/commit.sh
  with the landing flags (land.sh:305-317); commit.sh is also the wrapper for
  plain record commits (commit.sh:9-17, flags 40-46). `goal claim` is
  `goal.Claim` and `goal.ClaimArc` (internal/goal/verbs.go:582) and the claim
  paths that call them (internal/dispatch/claim.go:386,
  internal/steward/revive.go:191).
- **How a human actor is made.** `syncReqWithProof` assembles the request
  every synced goal verb consumes (cmd/metasystem/goalsync_mutations.go:49-126;
  `syncReq` at 32-34 is its wrapper). It builds `Actor.Human` from `--by` at
  line 110 and only afterwards classifies the caller's parent process with
  `lease.ClassifyVerb` (114-124), using the class for the lease epoch alone
  and ignoring a classification error. Fourteen call sites reach it:
  discharge-review-obligation (255, which then classifies a second time at
  260 and keeps the human only for a human caller), accept-risk (291), the
  legacy-routed open, set-next, promote, park, unpark, done, reopen,
  declare-free and prune (363), `runSyncOnly`'s claim, release, steal, edit,
  set-pin, set-arc and detach (520), classify-sweep (650, 663), approve
  (736), unapprove (785), set-budget (829), resume (901), split (958),
  enroll-terminal (1076), and set-obligation (1165). Three routes admit a
  named human without it: `goal repair --accept-remote --by`
  (goalsync_verbs.go:416-449, internal/goal/accepted.go:22-25), `goal
  migrate` through `goalActor` (goalsync_verbs.go:298-311), and the channel
  poll, which approves under a verified channel answer with the answering
  user as the human (internal/channel/poll.go:322, 367). `goal recover`
  replays a stored classify-sweep confirmation with the human it already
  carried (internal/goal/recover.go:235-243, 416-418). An agent opening a
  goal is recorded as machine plus lineage (verbs.go:172-186).
- **Identity and configuration.** The machine nickname is git configuration,
  `metasystem.goal.machine` (internal/goal/actor.go:21-28), and the nickname
  grammar the ledger enforces is one word with no whitespace (verbs.go:2318).
  The ledger identity is a ULID minted once into the root record
  (internal/goal/root.go:19, 131-134) and read from the accepted ref by
  `ExistingLedgerIdentity` (actor.go:41-54), which returns the empty string
  before migration; it names the ledger independent of any remote or
  transport. Configuration keys resolve env, then .local, then mode, then
  committed, then default (internal/config/conf.go:5). The turn-verdict state
  lives at artifacts/agents/turn-verdict-state.json (turnverdict.go:183);
  the machine-wide registry lives under
  `$METASYSTEM_SUPERVISION_REGISTRY_HOME/.metasystem/`
  (internal/registry/selection.go:9-25) and is written under the lock package
  through `LockedAppend` (internal/registry/append.go:20-32), which acquires
  `<path>.lock.d` with a liveness probe. The lock package's `Acquire` is a
  staged rename with death-only takeover (internal/lock/lock.go:141-223).
  Atomic publication of a state file is `atomicfile.WriteText`
  (digest.go:141).
- **The brain's other inputs.** `goal list` and `goal show` read the accepted
  ledger without fetching (cmd/metasystem/goal.go:270-300, 412-442). Channel
  questions are files under artifacts/agents/channel/questions/<id>.json with
  a `state` of open or answered and an unbounded `wants` text
  (internal/channel/question.go:41-57, 78-80); `listQuestions` returns an
  error at the first unreadable file (119-134). The status post is composed
  by `ComposeStatusReport` (internal/channel/report.go:36) and posted only
  when someone runs `channel status --post` (cmd/metasystem/channel_verbs.go:47-91);
  no tick or hook posts it today. The census verdict is
  artifacts/agents/supervision/last-census.json.
- **The packet's size.** The packet file is 3,227 bytes; its section headed
  "The standing instruction" (the four rules) is 786 bytes.

## 1. Designation: a record under the state root

**Decision.** One seat becomes the brain when its checkout holds the record
`<state root>/artifacts/agents/brain.json`:

```
{"schema":1,"ledger":"<26-character ULID>","machine":"<nickname>",
 "declaredBy":"<name>","declaredAt":"<RFC3339 UTC>"}
```

The fleet identity is the ledger identity: the ULID `ExistingLedgerIdentity`
reads from the accepted ref (actor.go:41-54). It is the same on every host of
one fleet, changes with no remote, transport, spelling, or credential, and
carries nothing secret, so it may appear in the payload, the status line, and
every diagnostic. No remote URL is read, stored, compared, or printed by
anything this page adds.

**Caps at declare time.** `machine` is the enrolled nickname and must match
the ledger's own grammar, one word with no whitespace, at most 32 bytes;
`declaredBy` is one line of at most 64 bytes with no control characters;
`declaredAt` is the fixed 20-byte RFC3339 UTC form; `ledger` is exactly 26
characters. Declare refuses anything longer with the cap in the message.
The header line of decision 2 is therefore at most 256 bytes by construction.

The engine owns the path through `brain.Path(stateRoot)` and the read through
`brain.Read(stateRoot, ledgerIdentity)`, which returns one of three states:

- **declared**: the file parses, `schema` is 1, every field meets its cap,
  `declaredAt` parses, and `ledger` equals this checkout's ledger identity;
- **undeclared**: the file does not exist;
- **corrupt**: anything else, with a reason: unreadable, not JSON, wrong
  schema, a missing or over-cap field, or a `ledger` that is not this
  checkout's.

`metasystem brain show --root <state root>` prints the state and the record
as JSON; exit 0 declared, exit 3 undeclared (the absent code `config
conf-value` uses), exit 1 corrupt with the reason and the remedy line below.
The record is written with `atomicfile.WriteText` (temp file and rename), so
a reader never sees a partial file. It is never committed: artifacts/ is
ignored, and no verb copies it.

**Why a .local key is wrong.** A configuration key has five sources
(conf.go:5): an environment variable inherited by a child process, or a line
someone commits into metasystem.conf, would designate a brain, and keeping
"machine-local, never committed" true would need two new validator refusals
that the record has by construction. `config validate` ignores unknown .local
keys, so a misspelled key would silently designate nothing. And .local is the
roster the join script copies from a template (plans/fleet-join-bootstrap-design.md,
step 2): a template line for a one-of-a-kind declaration is wrong on every
node or absent on the brain.

**The verbs** (family `brain`, human page `metasystem help brain`):

- `metasystem brain declare --root <checkout> --by <name>` runs these steps
  in order and stops at the first refusal, exit 2:
  1. Caller: refuses an agent-classified caller (`lease.ClassifyVerb` on the
     parent pid): "brain declare is a human act; run it from an agent-free
     terminal". Fixtures pass `--fixture-human-authority` under the same
     exact fake-runtime-root rule `goal approve` uses
     (goalsync_mutations.go, the approve flag block).
  2. Identity: refuses when the checkout has no ledger identity (an
     unmigrated checkout) or no nickname, printing the existing remedies of
     those two readers.
  3. Caps: the field caps above.
  4. Existing record: refuses a declared record ("this checkout is already
     the brain of ledger <ULID>, declared by <name> at <time>; withdraw it
     first: metasystem brain withdraw --root <checkout> --by <name>") and a
     corrupt one (the remedy line below).
  5. Quiescence: refuses while this checkout has anything its fences would
     strand, one line per obstacle, each ending with the command to run
     first:
     - a claim held by this machine's nickname on the accepted ledger, any
       lineage: `release it to a node: metasystem goal release --root
       <checkout> --id <id>` (an arc: `--arc <arc>`);
     - a job record with status pending, running, or pending-setup under
       artifacts/agents/jobs: `metasystem delegate --cancel <job id>` (for a
       running job this waits for the adapter's wind-down before the record
       is terminal);
     - a live run (status launching, running, or draining, from the run
       store scan.go:61 reads): `wait for it or conclude it: metasystem run
       watch --root <checkout> --id <run id>`;
     - an active mission runner (missionstate.Survey, scan.go:50): `wait for
       mission <id> to finish or park; metasystem mission status --root
       <checkout> --mission <id>`.
     The obstacle read is the scanner's own (`report.Scan`), so declare and
     the Stop verdict agree on what counts as live.
  6. Host pointer, under the lock package: acquire
     `<registry home>/.metasystem/brain/<ULID>.lock.d` with `lock.Acquire`,
     identity pid plus start time plus the tag `brain-declare`, wait 5
     seconds, poll 25 milliseconds, the kernel liveness probe the registry's
     `LockedAppend` callers already supply. Holding the lock, read the
     pointer file `<registry home>/.metasystem/brain/<ULID>` (one line: the
     checkout path). If it names another checkout whose record reads
     declared for this ULID, refuse: "this host already has a brain for
     ledger <ULID> at <path>; one brain per host per fleet; withdraw it there
     first". If it names a checkout whose record is gone or corrupt, or it
     is absent, write the pointer with `atomicfile.WriteText`, then write the
     record, then release. A lock acquisition failure (a live or unproven
     holder after the wait) refuses: "another brain declaration is in
     progress on this host (<holder>); retry". Two concurrent declarations
     for one ledger therefore serialize on the lock and the second reads the
     first's pointer. Two ledgers on one host each have their own pointer
     and lock and may each have a brain.
- `metasystem brain withdraw --root <checkout> --by <name>` refuses an
  agent-classified caller as declare does, then under the same lock removes
  the record in any state, declared or corrupt, and the pointer when it names
  this checkout. Exit 3 when nothing was there.
- A declaration on a node halts that node's dispatch and landing, which is
  why both verbs are human acts and why declare refuses a checkout that is
  still working.

**The remedy line** every corrupt-state refusal ends with:
`this checkout's brain declaration is unreadable (<reason>); until a human
repairs it nothing here dispatches, lands, claims, cancels, closes, reaps, or
carries a human's word: metasystem brain withdraw --root <checkout> --by
<name>, then metasystem brain declare --root <checkout> --by <name> if this
seat is the brain`.

**Visibility across hosts.** One brain per fleet is Wido's rule; nothing
machine-local can refuse a second host, and this page does not claim to. What
it does is make a second brain visible on a named cadence:

- The producer is the brain seat itself. `brain boot` and the brain's
  `turn-verdict` each write `<state root>/artifacts/agents/brain-status.json`
  (`{"line":"BRAIN: <nickname> for ledger <ULID> since <declaredAt>",
  "producedAt":"<RFC3339>","lastPostedAt":"<RFC3339 or empty>"}`) with
  `atomicfile.WriteText`.
- The carrier is the status post. `ComposeStatusReport` (report.go:36) adds
  the `line` as the post's first line when the file is present and its
  record reads declared, and `channel status --post` records `lastPostedAt`
  when the post succeeds.
- The cadence: at every brain turn end, the verdict returns
  `brainStatusDue: true` when `lastPostedAt` is empty or older than four
  hours (the status window `ComposeStatusReport` already uses when no post
  exists, report.go:45-48); the Stop hook then runs `metasystem channel
  status --post --root <checkout>` as plumbing, bounded by
  `channel.poll-timeout-sec`, after the verdict and outside the Stop refusal
  decision. A post that fails or finds no provider adds one line to the
  Stop's system message, `the brain's status line was not published:
  <reason>`, and never blocks the stop. Boot writes the file and posts
  nothing, because boot has no network budget; the first turn end posts.
- Two brains in one fleet appear as two `BRAIN:` lines with two nicknames in
  the fleet channel within four hours of both turning. That is visibility,
  not a fence.

## 2. Boot: the payload, its channel, its bound

**Channel.** The standing instruction must reach the model, not the screen. A
`systemMessage` is the runtime's user-facing notice; the runtime's
additional-context field is what enters the session's context. The engine
does not know field names, limits, or lifecycle sources: the runtime registry
gains three declarations on `Declaration` (runtimes.go:43), read by
`metasystem runtime start-context <runtime>` with the same contract as
`session-env` (exit 0 with `field=<dotted path> bytes=<n> sources=<a,b,c>`;
exit 1 is the declared absent capability):

- `StartContextField`: for claude `hookSpecificOutput.additionalContext`; the
  object also carries `hookSpecificOutput.hookEventName=SessionStart` because
  the runtime requires it.
- `StartContextBytes`: for claude 10,000. `Validate` (runtimes.go:385)
  refuses a declaration below 2,048, the minimum at which the header (at most
  256 bytes), the standing-instruction section (786 bytes), and one
  diagnostic line (at most 200 bytes) always fit with room to spare; the
  bound is applied in bytes, never more permissive than a character count.
- `StartContextSources`: for claude `startup,resume,clear,compact`. The
  shipped hooks template's matcher must equal this list joined with `|`; the
  existing hooks conformance test (internal/hooks) gains that assertion, and
  scripts/enforcement/claude-code-hooks.json:6 becomes
  `"matcher": "startup|resume|clear|compact"`. On `compact` the hook runs the
  same start path and the payload is re-injected, so a long-lived brain
  never continues past compaction without its standing instruction; the
  brain's digest cursor advanced at the previous boot, so a compaction
  carries only the digest since then.

For codex and devin the declarations stay empty until their hook manuals are
read; on an absent field the hook puts the payload in the `systemMessage`,
bounded at 2,048 bytes, and the first payload line says "this runtime has no
session-context channel; the packet reached the screen only".

**The hook decides nothing about the role.** The start path today prints
several objects; it changes to collect every message and print one object at
exit, with the brain payload in the declared context field and the collected
notices in `systemMessage`. The role decision is the engine's alone, in this
order:

1. Engine absent (supervision-hook.sh:351-356): the start event prints one
   object, `{"systemMessage":"Metasystem engine missing: this session
   received no role context; if this checkout is a declared brain it is
   uninstructed until the engine is rebuilt: run scripts/agents/go-build.sh,
   then start a new session"}`, injects nothing, and exits. The hook does not
   read the declaration file, the packet, or anything else, because a hook
   that reads the declaration is a second decider.
2. Engine present: the hook runs `metasystem brain boot` (below) before the
   identity check (935-938), so an unidentified agent process still receives
   its instruction. When the verb exits non-zero, times out at the hook's own
   bound of 8 seconds, or prints anything other than the object below, the
   hook prints one object, `{"systemMessage":"Metasystem brain boot failed
   (<exit code or timeout>; stderr tail: <text>): this session received no
   role context; if this checkout is a declared brain it is uninstructed:
   run metasystem brain boot --root <state root> --repo <checkout> by hand
   and rebuild if it fails"}`, injects nothing, and advances no cursor. A
   declared brain is then visibly uninstructed on the screen and the remedy
   is the rebuild; an undeclared node receives no role text under any
   failure.
3. The verb prints `{"declared":false}`: the hook adds nothing.
4. The verb prints the object: the hook relays `payload` into the declared
   field and, after the object is written to stdout, runs `metasystem brain
   digest-advance --root <state root> --repo <checkout> --cursor N
   --prefix-sha256 <hex>` only when `digestEmitted` is true.

**The verb.** `metasystem brain boot --root <state root> --repo <checkout>
--bytes <n> --deadline-ms <n>` refuses `--bytes` below 2,048 with exit 2 and
`refused: the context bound <n> is below the minimum 2048`. For an undeclared
checkout it prints `{"declared":false}`. Otherwise it prints one object:

```
{"declared":true,"state":"declared|corrupt","payload":"<text>","bytes":N,
 "sections":{"asks":"complete|cut|skipped|error","held":"...","fleet":"...",
             "digest":"..."},
 "digestEmitted":bool,"digestCursor":N,"digestPrefixSha256":"<hex>"}
```

A corrupt record is `declared` for boot: the seat may be the brain, so it
gets the packet, and the second payload line is the corruption reason with
the remedy line from decision 1. Every read is local files: no fetch, no
network.

**Two phases, one hard deadline.** The hook passes `--bytes` from the
registry and `--deadline-ms 5000`.

Phase one, the fast path, runs in the verb's own process and reads exactly
three small local files: the declaration, the ledger identity, and the
packet. It composes:

1. The header, one line, at most 256 bytes: `BRAIN SEAT <nickname> for
   ledger <ULID>, declared by <name> <date>. The standing instruction is the
   record at records/misc/fleet-coordinator-brain-role-packet.md.`
2. The packet. If the header plus the whole packet file is at most sixty
   percent of the bound, the packet rides verbatim. Otherwise the payload
   carries, verbatim, only the packet's section headed "The standing
   instruction", preceded by `PACKET TOO LARGE FOR THIS CHANNEL (<packet
   bytes> of <bound>); read the whole record before anything else`. The
   minimum bound guarantees this form always fits. A missing or unreadable
   packet yields `THE ROLE PACKET IS MISSING at <path> (<reason>); this seat
   is declared brain and has no instruction: take no work, tell Wido`.

Phase one's text is final before any other input is opened; nothing later
can cut it.

Phase two composes the four optional sections in a child process,
`metasystem brain boot-inputs` (internal), started by the parent with its
own process group. The child writes each completed section to its own temp
file with `atomicfile.WriteText`, in the order asks, held, fleet, digest. The
parent waits until the deadline minus phase one's elapsed time, then sends
the child's group SIGTERM and, 200 milliseconds later, SIGKILL, and reads
whatever section files are complete. A section whose file is absent is
`skipped`, and the payload ends with `BOOT DEADLINE: <section names> not
read within <n> ms; run metasystem brain boot --root <state root> --repo
<checkout> by hand`. The deadline is therefore a wall-clock bound the parent
enforces, not a check the reader must reach: a blocked read of one file
cannot hold the payload past it. `digestEmitted` is true only when the
digest section is `complete` or `cut`, so the cursor never advances past
text the session did not receive.

The remaining budget after phase one is shared in this order, each section
taking what it needs up to its share and passing the rest on: asks thirty
percent, held claims ten percent, fleet twenty-five percent, digest the
remainder.

- `ASKS AWAITING WIDO:` one line per open channel question, `<id> goal <id>
  <kind> from <machine> since <date>: <wants>`, each line cut at 160 bytes
  with a trailing `…`, newest first, then `N more; run metasystem channel
  show --root <checkout> --id <id>` for the rest. Enumeration is the child's
  own tolerant walk, not `listQuestions`: an unreadable or malformed question
  file is skipped and counted.
- `HELD HERE:` one line per claim this machine holds, cut at 160 bytes, then
  `release them to a node: metasystem goal release --root <checkout> --id
  <id>`; omitted when none.
- `FLEET:` one line per claimed goal (`<id> claimed by <machine> since
  <date>: <next step>`), one per approved unclaimed goal, one count line for
  queued unapproved goals, each cut at 160 bytes, then one line per
  non-terminal job record (`job <id> <status> <role> goal <id>`), then the
  census line (`census <verdict> custody N announced N untracked N, <age>`).
  Cut order inside the section: job lines first, then claim lines, always
  with a `N more` count.
- `NARRATOR DIGEST since the brain last booted:` the log after the brain's
  cursor at artifacts/agents/steward/narrator-digest-brain-cursor.json,
  oldest lines cut first with `N older lines cut; read
  records/narrator-digest.log`. The digest package's `Pending` and `Advance`
  take a cursor name; the human cursor keeps its path and its Stop-hook
  protocol untouched. A cut digest still advances the brain's cursor to the
  end of the log, because the log is a durable record.

**Every input maps to one line on error**, written by the child into the
section it replaces, with the section marked `error`:

| Input | Line on error |
| --- | --- |
| brain cursor or digest log | `DIGEST unreadable (<reason>); read records/narrator-digest.log by hand; the brain cursor was not advanced` |
| accepted ledger projection | `LEDGER unreadable (<reason>); run metasystem goal list --root <checkout>` |
| a channel question file | counted in `N unreadable question files; run metasystem channel show --root <checkout>` |
| a job record | counted in `N unreadable job records under artifacts/agents/jobs` |
| census verdict | `CENSUS absent or unreadable (<reason>); run metasystem supervise status --repo <checkout>` |
| the deadline | `BOOT DEADLINE: <sections> not read within <n> ms; run metasystem brain boot --root <state root> --repo <checkout> by hand` |

An uninstructed brain never starts silently: phase one is written before any
input that can stall is opened, the parent's kill makes the deadline real,
and a failure of the verb itself is the hook's one notice above.

## 3. The fence

One Go function, `brain.Fence(stateRoot, act, ledgerIdentity)`, reads the
record (decision 1) and returns the refusal text for an act in a declared
checkout, the remedy line for a corrupt one, and nothing for an undeclared
one. It keys on the record alone: never on the runtime, the model, or the
caller's process signature, except where decision 3d names the caller class.
Exposed to the shell as `metasystem brain fence --root <state root> --act
<act>` (JSON `{"state":"...","fenced":bool,"detail":"..."}`), and called
directly in Go elsewhere. A corrupt record fences every act below.

Enforced by the engine, and where:

- **delegate, in every form (a).** Four acts, `dispatch`, `follow-up`,
  `cancel`, `close`, and `reap`, are fenced at three places that between
  them every attempt passes:
  - `runDelegate` (delegate.go:29-103), before minting the claim capability,
    for every mode it accepts including `--cancel` and `--revive`: a fenced
    checkout prints `{"outcome":"BRAIN_REFUSED","headline":"refused",
    "detail":"<detail>"}` and exits 2 without execing dispatch.sh;
  - the legacy-grammar guard (dispatch.sh:28-35), extended to `close` and
    `reap` as well as `dispatch`, `follow-up`, `cancel`, and `--*`, which for
    a declared or corrupt checkout prints the same JSON detail instead of its
    one-liner;
  - the router (2894-2905), at the top of `dispatch_job`, `follow_up`,
    `cancel_job`, `close_chain`, and `reap_jobs`, which record `BRAIN_REFUSED`
    through `record_delegate_outcome` and exit 2 so an internal or steward
    path is fenced too.
  The detail for launch is: `this checkout is declared the brain; the brain
  never dispatches. A node runs, from its own checkout: metasystem delegate
  --role <role> --brief <file> --goal <id> --destructive-reach <class>`. For
  cancel: `... the brain never cancels a node's job. The node that owns it
  runs: metasystem delegate --cancel <job id>`. For close and reap: `... the
  brain never closes or reaps dispatcher records. The node that owns the
  chain runs: scripts/agents/dispatch.sh close --job <root job>` (or `reap`).
  No record is written or mutated. `watch` and `status` are read-only and
  stay open.
- **The one exemption: breach-stop.** `internal_breach_stop_goal`
  (dispatch.sh:2741-2757) is not fenced, for two reasons the builder should
  keep together. It is the budget law's own stop: the steward tick closes a
  breached goal revision's launch fence and winds its jobs down, under the
  stop-custodian authority, and a fence that could make a breach unstoppable
  would turn a budget overrun into a runaway. And it can act only on a
  revision this checkout holds a claim on; declare's quiescence rule leaves a
  brain checkout with no claim, so on a brain the route has nothing to act
  on. The route is reached only from the tick (internal/steward/tick.go:82),
  never from an operator grammar.
- **land, chain and direct-fix (b).** In land.sh after argument parsing and
  before `verify checks` (land.sh:391), and in commit.sh when any landing
  flag is present (`--chain`, `--direct-fix`, `--revert-of`, `--root-job`,
  `--test-receipt`; commit.sh:40-46). Plain record commits through commit.sh
  stay open: the brain writes records. Refusal exit 2 with `land refused: this
  checkout is declared the brain; the brain never lands. A node lands from
  its own checkout: scripts/agents/land.sh -m <message> --goal <id> --chain
  <root-job> <pathspec>`.
- **claim (c).** In `goal.Claim` and `goal.ClaimArc` (verbs.go:582), keyed
  on `r.Endpoint.Root`, so `goal claim`, `goal open --claim`, the dispatch
  claim path (dispatch/claim.go:386) and the steward's revival claim
  (steward/revive.go:191) all inherit it: `this checkout is declared the
  brain; the brain never claims. A node claims: metasystem goal claim --root
  <checkout> --id <id>`.
- **a human's word (d).** In `syncReqWithProof`
  (goalsync_mutations.go:49-126), the seam where `--by` becomes
  `Actor.Human`. The change to the seam: classification moves to the top,
  before the actor is built (today it runs at 114, after the actor at 110),
  its result rides on the request as `req.CallerClass`, and the brain rule
  runs there:
  1. read the declaration; undeclared means no rule;
  2. declared or corrupt, and `lease.ClassifyVerb` returned an error:
     refuse, fail closed, `this checkout is declared the brain and the
     caller could not be classified (<error>); an unclassified caller
     carries no human's word here. Wido runs, from an agent-free terminal:
     metasystem goal <verb> --root <checkout> --id <id> --by <name> <the
     verb's own flags>`;
  3. declared or corrupt, `by` non-empty, and the class is not `ClassHuman`:
     refuse, `this checkout is declared the brain; the brain never carries a
     human's word into goal <verb>, not as --by, not as a relayed word, not
     as a channel reference. Wido runs, from an agent-free terminal: ...`
     (the same command form). The fixture proof passes under the fake root
     only, as `observedProof.FixtureOnly`.
  The refusal happens before any verb publishes or records a proof. Every
  verb that builds its request through the seam inherits the fence by
  construction; discharge-review-obligation inherits it because it passes
  `--by` through the seam (255), and its second classification at 260 is
  replaced by reading `req.CallerClass`, so the seam is the only classifier.
  The verbs the seam reaches today: open, set-next, promote, park, unpark,
  done, reopen, declare-free, prune, claim, release, steal, edit, set-pin,
  set-arc, detach, split, approve, unapprove, set-budget, accept-risk,
  discharge-review-obligation, classify-sweep, set-obligation, resume, and
  enroll-terminal. A verb added later that calls `syncReq` or
  `syncReqWithProof` inherits it without a line of its own.

  The routes that admit a named human without the seam, and what each does:
  - `goal repair --accept-remote --by` (goalsync_verbs.go:416-449): gains
    the same three-step rule before `RepairAcceptRemote`; refusal names
    `goal repair`.
  - `goal migrate` through `goalActor` (goalsync_verbs.go:298-311): gains
    the same rule; a converted brain checkout never migrates, so this is
    closure, not a live path.
  - `goal recover` (recover.go:235-243): replays a classify-sweep
    confirmation whose human passed the seam when the journal entry was
    written; it mints nothing and is not fenced.
  - the channel poll (poll.go:322, 367): approves under a verified channel
    answer, with the answering user as the human and the code as proof. The
    brain adds no word of its own; this is Wido's act arriving by transport,
    and it passes. It is the one route that publishes a human's approval
    from a brain checkout, and the page names it so nobody looks for a fence
    there.
  - `goal.Actor{...}` literals inside internal/ (dispatch/claim.go,
    dispatch/stop.go, dispatch/finding_register.go, steward/revive.go,
    channel/question.go) carry no human and need no fence; the claim ones
    are covered by (c).

  The static fixture `brain-actor-seam-coverage` (decision 8) lists every
  `goal.Actor{`, `Actor.Human =`, and `lease.ClassifyVerb(` site outside the
  seam in cmd/metasystem and internal/, and fails when a new one appears
  that is not in its allow-list, so "inherits by construction" stays
  checkable.

Enforced by the instruction only: writing code, tests, designs, and briefs
(file writes have no engine seam); `git commit` outside commit.sh (the
pre-commit guard already refuses it for everyone); setting the internal
dispatch marker by hand (a lie, not a path). The page names these so the
builder does not look for a fence that cannot exist.

## 4. Turn end

`report turn-verdict` reads the declaration from its `--root`. For a declared
or corrupt record:

- The brain summary is the first line of `Display`, produced in `TurnVerdict`
  before `decideRuns` and `decide` run (turnverdict.go:234-235) and passed to
  `composeDisplay` as the first prefix line (616-624), so no ladder branch
  can suppress it: `BRAIN SEAT: nodes hold <n> claims (<ids>); <m> approved
  goals await a node; <a> asks await Wido (<ids>); <d> drafts await approval
  (<ids>)`. A corrupt record adds the remedy line from decision 1 as the
  second line. A claim this machine holds is a third line, `HELD HERE:
  <ids>; release them to a node`, and is not prodded.
- `readClaimableBudgetedWork` and `enforceIdleBacklog` are not called (226,
  241). `IdleRefusal` is never true, `IdleBlocks` never increments, and
  `escalateIdleBacklog` is unreachable: no continuation intent, no steward
  claim, no idle alarm on the brain's behalf.
- The goal clause (the ladder's default branch) does not run; the ladder's
  other branches are unchanged: plan open work blocks once per signature,
  unwatched work blocks once, unreadable inputs veto the all-clear, human
  stop authorization is consumed as today. Asks and drafts are the brain's
  own open work and are reported, never blocked on.
- The verdict writes the brain status record of decision 1 and returns
  `brainStatusDue` as decided there; the hook posts as plumbing after the
  verdict is emitted.

`ScanResult` gains two fields the scanner fills for every checkout and only
the brain line reads: `Questions`, one item per open channel question from
the boot verb's tolerant walk (an unreadable file joins `Unreadable`), and
`Drafts`, one item per queued goal with no `Approved` record whose opening
history line names this machine (internal/goal/file.go:25, 54, 72; the
opening actor is machine plus lineage, verbs.go:172-186). A draft is owned by
machine, not lineage: a draft opened by an earlier brain session on this
machine is this brain's draft. For an undeclared checkout the two fields are
filled and not displayed, so no other seat's verdict changes.

## 5. Handover in one word

When Wido says the word, the brain writes exactly one record: a goal open.

```
metasystem goal open --root <checkout> --id <id> --intent "<Wido's words>" \
  --next "<the step a cold seat resumes from>" \
  --risk severity=<n>,novelty=<n>,exposure=<n>,accumulation=<n> --basis "<why>" \
  [--elapsed-limit <d> --attempt-limit <n> --reserved-job-minutes-limit <n> \
   --active-job-limit <n> --review-round-limit <n>]
```

The tier derives from the four answers; the brain passes no `--by`, no
`--tier` below the derivation, and never `--temporary-human-word` (decision
3d refuses all three from an agent caller). The box is the tier's configured
box unless the brain proposes another tuple with the five limits. The record
is queued and unapproved; nothing on it is an approval. The brain then hands
Wido this line, and nothing after it:

```
metasystem goal approve --root <checkout> --id <id> --by <name> --budget box
```

When the draft carried its own tuple the line repeats the same five limits in
place of `--budget box`, so what Wido approves is byte-for-byte what the brain
proposed. No new ledger field is needed: intent, tier, tuple, the absent
approval, and the opening machine are already on the record.

## 6. Never a bottleneck

No verb a node runs reads, waits on, or is refused by the brain's declaration,
its session, or its presence; every fence in decision 3 keys on the calling
checkout's own record, so a brain that is down changes nothing a node does.
The fixture that proves it is `brain-absent-node-proceeds` (decision 8).

## 7. Agnosticism and boundaries

- Core names no runtime. The context field, its bound, and its lifecycle
  sources are registry declarations, and the hook asks the registry; the
  engine's `brain boot` output is the same text for every runtime.
- The hook decides nothing: `brain boot`, `brain fence`, and `turn-verdict`
  decide; the hook relays JSON, advances the brain's cursor when told, and
  posts the status when told. Without the engine it relays one notice and
  nothing else.
- The packet's text lives in records/misc/fleet-coordinator-brain-role-packet.md
  and is read at boot. No copy, no excerpt in code; the shrunk form is the
  record's own section, read by heading at boot.
- The ledger's bytes and verbs are unchanged. The one new verb family is
  `brain`; the new registry declarations are `StartContextField`,
  `StartContextBytes`, and `StartContextSources`; the digest package gains a
  cursor-name parameter with the human cursor as default; the scan gains two
  fields; the status post gains one conditional line; the request-assembly
  seam gains `CallerClass` and moves classification first.
- Nothing this page adds reads, stores, or prints a remote URL.
- Power of attorney (a scoped, time-boxed delegation of approval) is a later
  ruling and a later fence exception; this page leaves the fence whole.

## 8. Fixtures

Each names what it observes and which script it joins. A declared bed means a
scratch checkout with a fake-runtime root, a bare ledger remote, a scratch
registry home, and `brain declare --fixture-human-authority`. brain-fixtures.sh
is new: it is added to the script list scripts/validate-metasystem.sh
checks with `bash -n` and gets its own `run_section brain-fixtures
needs-engine bash scripts/agents/brain-fixtures.sh` block beside the
goal-cli block (validate-metasystem.sh:1102-1104). Rows that join an existing
bed script add their scenario name to that script's scenario list
(goal-cli-fixtures.sh:81-85, dispatch-fixtures.sh:83-87,
land-fixtures.sh:30-32); supervision-hook-fixtures.sh runs the hook directly.
channel-fixtures.sh is not run by the validation driver, so nothing joins it.

| Fixture | Observes | Joins |
| --- | --- | --- |
| brain-boot-declared | Declared bed, one digest line appended to records/narrator-digest.log, one open question written by `channel ask` on the fake provider; `supervision-hook.sh claude start` with source `startup` prints one JSON object whose `hookSpecificOutput.additionalContext` carries the packet's first heading, the digest line, and the question id, and is at most 10,000 bytes; the brain cursor advanced; the human cursor file unchanged; brain-status.json written | supervision-hook-fixtures.sh |
| brain-boot-compact | The same bed, the hook run with the payload `{"source":"compact",...}`: the context field is present again with the packet and only the digest lines appended since the first boot; the shipped matcher in scripts/enforcement/claude-code-hooks.json equals the registry's declared sources joined with a bar | supervision-hook-fixtures.sh |
| brain-boot-undeclared | The same bed after `brain withdraw`: the start object has no context field and none of the three texts | supervision-hook-fixtures.sh |
| brain-boot-corrupt | The record overwritten with `{broken`, then with a valid record whose `ledger` is another ULID: the start object carries the standing-instruction section and the remedy line; `brain show` exits 1 with the reason | supervision-hook-fixtures.sh |
| brain-boot-no-engine | The hook run with `METASYSTEM_BIN` pointing at a missing path on a declared bed and on an undeclared bed: both print one `systemMessage` naming the missing engine and the rebuild, neither carries a context field or any packet text | supervision-hook-fixtures.sh |
| brain-boot-verb-fails | The boot verb replaced by a shim that exits 1, then by one that sleeps 30 seconds, then by one that prints `not json`: each time the start object is one `systemMessage` naming the failure and the by-hand command, with no context field, and the brain cursor is untouched | supervision-hook-fixtures.sh |
| brain-boot-bound | 400 digest lines, 80 job records, 60 open questions each with a 2,000-byte `wants`: the payload is at most the bound, every ask line is at most 160 bytes and ends with `…`, `sections` marks fleet and digest `cut`, the packet bytes are intact; with `--bytes 2048` the payload carries the standing-instruction section and the `PACKET TOO LARGE` line and still fits; with `--bytes 2047` the verb exits 2 with the minimum in its message; a registry declaration test asserts `Validate` refuses a `StartContextBytes` of 2,047 | brain-fixtures.sh (new) |
| brain-boot-caps | `brain declare` with a 33-byte nickname, a 65-byte `--by`, and a `--by` containing a newline: each refused naming the cap; a maximal valid declaration yields a header line of at most 256 bytes | brain-fixtures.sh (new) |
| brain-boot-errors | A malformed brain cursor, one unparsable question file, one unparsable job record, and no census file: the payload carries the packet whole plus one error line per input, the four sections are marked `error` or `complete` accordingly, `digestEmitted` is false and the cursor file is untouched | brain-fixtures.sh (new) |
| brain-boot-stalled | A named pipe (`mkfifo`) at artifacts/agents/channel/questions/stalled.json with no writer, so the asks reader blocks forever; `brain boot --deadline-ms 1500` returns within 3 seconds with the packet whole, `sections.asks` equal to `skipped`, the `BOOT DEADLINE` line naming asks, and no child process of the verb alive afterwards (the process group is gone) | brain-fixtures.sh (new) |
| brain-delegate-refuses | From a declared bed and again from a corrupt bed: `metasystem delegate` on the fake runtime prints outcome `BRAIN_REFUSED` with the node's `metasystem delegate` command and writes no job record; a direct `scripts/agents/dispatch.sh dispatch ...` prints the same outcome and detail and exits 2 | dispatch-fixtures.sh |
| brain-cancel-close-reap-refuse | A declared bed seeded with one pending job record and one closed chain root: `metasystem delegate --cancel <job>`, `dispatch.sh cancel --job <job>`, `dispatch.sh close --job <root>`, and `dispatch.sh reap` each print `BRAIN_REFUSED` with the owning node's command; the job record's status and the root record's bytes are unchanged; the same four from a corrupt bed print the remedy line | dispatch-fixtures.sh |
| brain-breach-stop-exempt | A declared bed: `dispatch.sh __breach-stop-goal --goal <id> --revision 1` is not refused by the brain fence (it fails only on its own authority or missing claim, with its own text) | dispatch-fixtures.sh |
| brain-land-refuses | From a declared leg and again from a corrupt leg: `land.sh --chain <root>`, `land.sh --direct-fix tier-1 ...`, `commit.sh --chain <root> -F <msg>`, and `commit.sh --direct-fix register-carriage -F <msg>` each exit 2 with the fence text (or the remedy line) and leave HEAD unchanged; a plain `commit.sh -F <msg> <record path>` succeeds on the declared leg | land-fixtures.sh |
| brain-claim-refuses | Declared bed with one approved goal, then the same bed corrupt: `goal claim` and `goal open --claim` are refused with the node's claim command (or the remedy line); the ledger tip is unchanged | goal-cli-fixtures.sh |
| brain-human-word-refuses | Declared bed, agent-classified caller: one scenario per verb, each refused with the fence text naming that verb and leaving the tip unchanged: `approve --temporary-human-word`, `resume --temporary-human-word`, `resume --approved-ref`, `set-obligation --temporary-human-word`, `steal --by`, `set-pin --by`, `classify-sweep --confirm --by`, `set-budget --by`, `unapprove --by`, `accept-risk --by`, `discharge-review-obligation --by`, `repair --accept-remote --by`; then the same verbs from a corrupt bed print the remedy line; then `approve --fixture-human-authority` succeeds, and `channel poll` on the fake provider with a verified answer approves the goal | goal-cli-fixtures.sh |
| brain-classification-fails | Declared bed with the lease state made unreadable so `lease.ClassifyVerb` errors: `goal approve --by <name> --fixture-human-authority` and `goal steal --by <name>` are refused with the "could not be classified" text; on the undeclared bed the same commands behave as today | goal-cli-fixtures.sh |
| brain-actor-seam-coverage | A grep over cmd/metasystem and internal/ for `goal.Actor{`, `Actor.Human =`, and `lease.ClassifyVerb(` outside `syncReqWithProof` equals the allow-list in the fixture; a new site fails it | brain-fixtures.sh (new) |
| brain-declare-quiescence | Declared-bed prerequisites on a checkout that holds a claim, then one with a pending job record, then one with a running run record, then one with an active mission state: `brain declare` refuses each naming the release, cancel, run watch, or mission command; after the obstacle is removed it succeeds | brain-fixtures.sh (new) |
| brain-declare-race | Two `brain declare` processes started together on two checkouts of one bare remote under one scratch registry home: exactly one succeeds, the other prints the "already has a brain" refusal naming the winner's path, and the pointer names the winner; a held lock directory with a live foreign owner makes declare print "in progress" after its wait | brain-fixtures.sh (new) |
| brain-second-declaration-refuses | `brain declare` twice on one checkout refuses naming the withdraw command; a second checkout of the same bare remote under the same scratch registry home refuses naming the first checkout's path; the same second checkout under a different scratch registry home succeeds (two hosts, visible only through the status line); a third checkout of a different bare remote under the first home succeeds; after `brain withdraw` in the first, the second succeeds there too | brain-fixtures.sh (new) |
| brain-verbs-human-only | `brain declare` and `brain withdraw` from an agent-classified caller refuse; with the fixture authority under the fake root they act; `withdraw` removes a `{broken` record and the pointer | brain-fixtures.sh (new) |
| brain-stop-seeded | Declared bed with two approved unclaimed goals, one open question, one draft opened by a different lineage on this machine, one plan carrying an open next step, and one plan waiting on the human; `report turn-verdict --stop-hook-active=true` three times: the first display line is `BRAIN SEAT` naming one ask and one draft every time, `idleRefusal` false every time, `blockSource` never idle-backlog, the open-work block fires once as today, no steward intent staged; brain-status.json written with `producedAt`; `brainStatusDue` true on the first verdict and false after `lastPostedAt` is set to now | goal-cli-fixtures.sh |
| brain-stop-corrupt | The same bed with `{broken` as the record: the first line is `BRAIN SEAT`, the second the remedy line, no idle refusal | goal-cli-fixtures.sh |
| brain-status-line | `channel status` composed on a declared bed after one `brain boot` carries `BRAIN: <nickname> for ledger <ULID>` as its first line; on the undeclared bed it does not; the line contains no `://` | goal-cli-fixtures.sh |
| brain-absent-node-proceeds | Two clones of one bare remote; clone A declared brain with no session; clone B lands the goal leg and pushes green, and clone B's fake-runtime delegate starts | land-fixtures.sh (landing) and dispatch-fixtures.sh (dispatch) |

What no fixture can prove in a delegate sandbox: the `up` leg of the start
path and the runtime's own consumption of the context field and the compact
source; the orchestrator runs the suite outside the sandbox, and the field's
effect is checked once by a human opening a declared seat and once by
compacting it.

## 9. Self-grade

Confidence 0.85 in the designation, the fence seams, the two-phase boot, and
the verdict change; each seam is a line in the tree today, and each fixture
fails on today's tree and passes only with the named behavior. Weakest claim:
the runtime's behavior at its context field, its 10,000-character bound, and
its `compact` source rest on the runtime's manual, not on anything in this
tree, and the manual's version is not pinned here. Reject condition: a
declared seat opens or compacts and the model does not see the packet; then
the channel decision returns here and the payload rides plain stdout instead.
Second weakest: the four-hour status cadence publishes a second brain only
when the brain seat turns; a brain declared and never used is published
never, which is the human rule's own limit and is stated as such.

## Addendum: two build gaps decided by the coordinator (m1, 2026-09-07)

The first build round stopped on two gaps in this page and left the tree
clean. Both are decided here so the build can proceed; neither changes a
rule of the role packet or any authority.

1. **`goal open --claim` and the fence.** Section 3 says the claim fence
   is inherited from `goal.Claim` and `goal.ClaimArc`. The `goal open
   --claim` route calls `goal.OpenClaim` instead, which reaches neither
   and always answers APPROVAL_REQUIRED for a goal that was just opened.
   Decision: the fence for that route lives at the command layer, in the
   `goal open` handler of cmd/metasystem/goalsync_mutations.go, checked
   before `goal.OpenClaim` is called, with the same refusal text and
   node command the claim fence prints. `goal claim` keeps its fence at
   `goal.Claim` and `goal.ClaimArc` as written. The brain-claim-refuses
   fixture row is unchanged; it now observes the command-layer refusal
   for the open form.

2. **The packet in adopted installations.** scripts/adopt.sh removes the
   records tree from an adopted installation and restores only the
   goals migration manifest, so boot could not read
   records/misc/fleet-coordinator-brain-role-packet.md there. Decision:
   the packet is an engine input like the manifest. adopt.sh preserves
   that exact record at its own path in the adopted tree, the same way
   it preserves the manifest, and the missing-packet line of section 2
   stays the answer for any installation that lacks it. The packet's
   canonical source does not move and is not copied into code.

## Addendum 2: the header bound (m1, 2026-09-07)

The second build round stopped on an arithmetic contradiction in section
1: the exact header grammar with every field at its cap is 276 bytes,
and the page says at most 256. Decision: the header bound is 320 bytes.
It is derived, not chosen: the fixed text of the header plus the four
field caps (32, 26, 64, 20) plus a margin of 32 bytes for a later fixed
word, so a maximal valid declaration always fits with room to spare. The
brain-boot-caps fixture row asserts at most 320 bytes. Nothing else in
sections 1 or 2 changes; the payload's minimum bound of 2,048 bytes
already leaves room for a 320-byte header.

A builder who meets a contradiction that the page's own numbers settle
(a bound smaller than the sum of the caps that feed it) reports it in
the return with the resolution used and keeps building; built and
passing sections are never removed because a later row is in doubt.
