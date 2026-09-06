# Design: the brain seat boots with its standing instruction (revision 2)

Goal: fleet-coordinator-brain (arc headless-fleet, tier 3). The role packet is
landed at records/misc/fleet-coordinator-brain-role-packet.md; its four rules
(never builds, never dispatches, never approves, never a bottleneck) are fixed
and this page does not restate them. What the goal still owes, in Wido's words:
"the standing instruction wired into the seat's boot so it loads unprompted,
and the narrator's digest as its input." Revision 1 was written 2026-09-06,
the day a seat again stood in for the brain and the runner at once; revision 2
folds the eight findings of its critique, all accepted. Every claim about the
tree cites file and line as they stand in this worktree on that date.

## Where each critique finding lands

| Finding | Section | One line |
| --- | --- | --- |
| BRAIN-R1-01 | 1 | The record carries the fleet identity; the host pointer is keyed by fleet; cross-host uniqueness is a named status line, stated as visibility |
| BRAIN-R1-02 | 1, 3 | A malformed or unreadable record is a third state that fails closed: every guarded act refuses with the remedy, boot carries the packet plus one corruption line, publication is atomic |
| BRAIN-R1-03 | 3 | The authority fence moves to the seam where `--by` becomes a human actor; every verb it reaches is listed, the routes outside it are named and decided |
| BRAIN-R1-04 | 2 | The runtime registry declares the context channel's byte bound beside its field; the packet fits whole or shrinks to its path and standing-instruction section; asks and claims are one capped line each |
| BRAIN-R1-05 | 2 | Every boot input maps to one payload line on error; composition runs under a deadline inside the hook's fifteen seconds, packet first, never cut |
| BRAIN-R1-06 | 4 | The brain summary is the first display line, emitted before the ladder branches; the scan gains open questions and drafts; drafts are owned by machine |
| BRAIN-R1-07 | 3 | The legacy-grammar guard in dispatch.sh prints the brain refusal and node command when the checkout is declared |
| BRAIN-R1-08 | 8 | The fixture table gains every seam the fence names, the corrupted record, the over-bound packet, the long ask, and a seeded Stop |

## 0. What is true today, read in the tree

- **What a seat loads unprompted.** CLAUDE.md is three lines: read AGENTS.md,
  load wow.md on demand. Nothing names the brain, the packet, or a role.
- **The boot seam.** The runtime's SessionStart hook runs
  `scripts/agents/supervision-hook.sh <runtime> start` with a 15-second
  timeout (scripts/enforcement/claude-code-hooks.json:10-11; the codex and
  devin templates carry the same line). The start path resolves the checkout
  and the state root (supervision-hook.sh:368-383), prints pending steward
  incidents (886-887), then identifies the agent process and runs `up`
  (907-932). Every message it emits is one `{"systemMessage": ...}` object
  from `surface_json` (485-491); several calls print several objects. The
  runtime registry declares per-runtime session facts the hook reads
  indirectly, for example `runtime session-env` (main.go:330,
  internal/runtimes/runtimes.go:58-61); exit 1 from such a query is the
  declared absent capability (supervision-hook.sh:360). The Claude Code hooks
  reference caps a hook's context string at 10,000 characters and replaces a
  longer one in context with a preview and a file path; this is the runtime's
  manual, not the tree.
- **The digest.** `steward digest-pending` returns the narrator log after one
  machine-local cursor (internal/narratordigest/digest.go:225-250); the cursor
  path is fixed at artifacts/agents/steward/narrator-digest-cursor.json
  (63-65) and advances only through `digest-advance` after the Stop payload is
  emitted (supervision-hook.sh:612-617). A malformed cursor or a log changed
  before the cursor is an error (digest.go:201-222, 241-243). There is one
  cursor per checkout, and it is the human's.
- **The Stop verdict.** `report turn-verdict` reads the claimable budgeted
  backlog for every agent caller (internal/goal/turnverdict.go:225-226) and
  `enforceIdleBacklog` refuses the stop three times for an unchanged claimable
  backlog (293-354); at the third refusal `escalateIdleBacklog` selects the
  machine's held or first claimable goal, prepares a steward continuation
  intent, and records the incident (371-462). The display is composed from a
  run prefix, the ladder's text, and the green lines (586-594); the ladder
  (597-715) is one switch where Busy, Open, WaitingOnHuman, and Unreadable
  each end the turn's text before the goal clause runs (626-709). The scanner
  (internal/report/scan.go:25-72) reads jobs, gates, missions, runs, and
  plans; it reads no channel question and no goal record.
- **The acts.** Every operator dispatch enters `runDelegate`
  (cmd/metasystem/delegate.go:29-103), which execs dispatch.sh; a direct
  dispatch.sh call without the internal marker exits through the
  legacy-grammar guard, which prints only "use metasystem delegate"
  (scripts/agents/dispatch.sh:28-35), before the router reaches
  `dispatch_job` or `follow_up` (2596, 1807). Every landing enters
  scripts/agents/land.sh, which calls scripts/agents/commit.sh with the
  landing flags (land.sh:305-317); commit.sh is also the wrapper for plain
  record commits (commit.sh:9-17, flags 40-46). `goal claim` is
  `goal.Claim` and `goal.ClaimArc` in internal/goal/verbs.go:582 and the
  claim paths that call them (internal/dispatch/claim.go:386,
  internal/steward/revive.go:191).
- **How a human actor is made.** `syncReq` assembles the request every synced
  goal verb consumes and turns any `--by` string into `Actor.Human`
  (cmd/metasystem/goalsync_mutations.go:32-77); it classifies the caller's
  parent process with `lease.ClassifyVerb` (65-75) but uses the class only
  for the lease epoch. `runSyncOnly` routes claim, release, steal, edit,
  set-pin, set-arc, and detach through it (447-475, 1152-1261). Approve,
  unapprove, set-budget, accept-risk, and the tier-lowering forms of open and
  edit prove authority first through `proveGoalHumanAuthority` (485; calls at
  237, 347, 682, 731, 1211), then call `syncReq`; resume and set-obligation
  carry their own proof paths, an enrolled terminal, a relayed word, or an
  authenticated channel answer (802-857, 1044-1116), then call `syncReq`.
  Three routes admit a named human without `syncReq`: `goal repair
  --accept-remote --by` (goalsync_verbs.go:416-449, internal/goal/accepted.go:22-25),
  `goal migrate` through `goalActor` (goalsync_verbs.go:298-311), and the
  channel poll, which approves under a verified channel answer with the
  answering user as the human (internal/channel/poll.go:322, 367). `goal
  recover` replays a stored classify-sweep confirmation with the human it
  already carried (internal/goal/recover.go:235-243, 416-418). In
  internal/goal, `Steal` requires a human (verbs.go:1658-1665) and `SetPin`
  is a human act (2294-2304); both accept whatever `Actor.Human` the request
  carries. An agent opening a goal is recorded as machine plus lineage
  (verbs.go:172-186).
- **Configuration and identity.** Keys resolve env, then .local, then mode,
  then committed, then default (internal/config/conf.go:5); `config validate`
  ignores every .local key except capability floors (validate.go:66-95). The
  machine nickname is git configuration, `metasystem.goal.machine`
  (internal/goal/actor.go:21-28). The ledger endpoint is the remote
  `goal.sync-remote` names, default origin (internal/goal/txn.go:49-61); on
  this machine origin is the shared GitHub URL and `transport` is a local
  mirror path that differs per host. The turn-verdict state lives at
  artifacts/agents/turn-verdict-state.json (turnverdict.go:183-185); the
  machine-wide supervision registry lives under
  `$METASYSTEM_SUPERVISION_REGISTRY_HOME/.metasystem/`
  (internal/registry/selection.go:9). Atomic publication of a state file is
  `atomicfile.WriteText` (digest.go:141).
- **The brain's other inputs.** `goal list` and `goal show` read the accepted
  ledger without fetching (cmd/metasystem/goal.go:270-300, 412-442). Channel
  questions are files under artifacts/agents/channel/questions/<id>.json with
  a `state` of open or answered and an unbounded `wants` text
  (internal/channel/question.go:41-57, 78-80); `listQuestions` returns an
  error at the first unreadable file (119-134). The status post already lists
  every open question as a "Needs you" line (internal/channel/report.go:36-60).
  The census verdict is artifacts/agents/supervision/last-census.json.

## 1. Designation: a record under the state root

**Decision.** One seat becomes the brain when its checkout holds the record
`<state root>/artifacts/agents/brain.json`:

```
{"schema":1,"fleet":"<ledger remote URL>","machine":"<nickname>",
 "declaredBy":"<name>","declaredAt":"<RFC3339>"}
```

The fleet identity is the URL of the remote `goal.sync-remote` names, read
with `git remote get-url`, trailing `.git` and slash removed. That URL is the
same on every host of one fleet, which is what identity needs; the
`transport` remote is a per-host mirror path and cannot serve. The engine
owns the path through `brain.Path(stateRoot)` and the read through one
function, `brain.Read(stateRoot, ledgerRemoteURL)`, which returns one of
three states:

- **declared**: the file parses, `schema` is 1, every field is non-empty,
  `declaredAt` parses, and `fleet` equals this checkout's ledger remote URL;
- **undeclared**: the file does not exist;
- **corrupt**: anything else, with a reason: unreadable, not JSON, wrong
  schema, a missing field, or a `fleet` that is not this checkout's remote.

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
keys (validate.go:85-86), so a misspelled key would silently designate
nothing. And .local is the roster the join script copies from a template
(plans/fleet-join-bootstrap-design.md, step 2): a template line for a
one-of-a-kind declaration is wrong on every node or absent on the brain.

**The verbs** (family `brain`, human page `metasystem help brain`):

- `metasystem brain declare --root <checkout> --by <name>` writes the record
  with this machine's nickname and this checkout's fleet URL, and writes the
  host pointer `<registry home>/.metasystem/brain/<fleet-key>` naming the
  checkout path, where `<fleet-key>` is the first sixteen hex characters of
  the fleet URL's sha256. It refuses when this checkout already holds a
  declared record ("this checkout is already the brain of <fleet>, declared
  by <name> at <time>; withdraw it first: metasystem brain withdraw --root
  <checkout> --by <name>"), when the record is corrupt (the remedy line
  below), and when the pointer for this fleet names another checkout whose
  record reads declared ("this host already has a brain for <fleet> at
  <path>; one brain per host per fleet; withdraw it there first"). A pointer
  to a checkout whose record is gone or corrupt is stale and is replaced. Two
  fleets on one host each get their own pointer and may each have a brain.
  Refusal exit 2.
- `metasystem brain withdraw --root <checkout> --by <name>` removes the record
  in any state, declared or corrupt, and the pointer when it names this
  checkout. Exit 3 when nothing was there.
- Both verbs refuse an agent-classified caller (`lease.ClassifyVerb` on the
  parent pid): "brain declare is a human act; run it from an agent-free
  terminal". A declaration on a node halts that node's dispatch and landing,
  so a seat must not be able to declare or undeclare itself. Fixtures pass
  `--fixture-human-authority` under the same exact fake-runtime-root rule
  `goal approve` uses (goalsync_mutations.go:156-158).

**The remedy line** every corrupt-state refusal ends with:
`this checkout's brain declaration is unreadable (<reason>); until a human
repairs it nothing here dispatches, lands, claims, or carries a human's word:
metasystem brain withdraw --root <checkout> --by <name>, then metasystem
brain declare --root <checkout> --by <name> if this seat is the brain`.

**One brain per fleet across hosts** is visibility, not a fence, and the page
says so in one sentence: nothing machine-local can refuse a second host, so
the status post carries the line `BRAIN: <nickname> for <fleet> since
<declaredAt>` whenever the posting checkout reads declared, produced by
`ComposeStatusReport` (report.go:36) on the status post's own cadence, and two
brains in one fleet appear as two such lines with two nicknames that every
seat reading the channel can see.

## 2. Boot: the payload, its channel, its bound

**Channel.** The standing instruction must reach the model, not the screen. A
`systemMessage` is the runtime's user-facing notice; the runtime's
additional-context field is what enters the session's context. The engine
does not know field names or limits: the runtime registry gains two
declarations, `StartContextField` and `StartContextBytes`, read by
`metasystem runtime start-context <runtime>` with the same contract as
`session-env` (exit 0 with `field=<dotted path> bytes=<n>`; exit 1 is the
declared absent capability). For claude the field is
`hookSpecificOutput.additionalContext` with 10,000 as the bound, and the
object also carries `hookSpecificOutput.hookEventName=SessionStart` because
the runtime requires it. The bound is applied in bytes, which is never more
permissive than the runtime's character count. For codex and devin the
declaration stays empty until their hook manuals are read; on an absent field
the hook puts the payload in the `systemMessage`, bounded at 10,000 bytes,
and the first payload line says "this runtime has no session-context channel;
the packet reached the screen only".

**One object.** The start path today can print several JSON objects. It
changes to collect every message (pending incidents, arming outcome, re-arm
notice) and print one object at exit, with the brain payload in the declared
context field and the collected notices in `systemMessage`. The brain payload
is composed before the identity check (supervision-hook.sh:907-910), so an
unidentified agent process still receives its instruction.

**The verb.** `metasystem brain boot --root <state root> --repo <checkout>
--bytes <n> --deadline-ms <n>` prints `{"declared":false}` for an undeclared
checkout, and otherwise:

```
{"declared":true,"state":"declared|corrupt","payload":"<text>","bytes":N,
 "cut":["..."],"errors":["..."],"digestCursor":N,"digestPrefixSha256":"<hex>"}
```

A corrupt record is `declared` for boot: the seat may be the brain, so it
gets the packet, and the second payload line is the corruption reason with
the remedy line from decision 1. The hook passes `--bytes` from the registry
and `--deadline-ms 5000`, relays `payload`, and after the object is written
to stdout runs `metasystem brain digest-advance --root <state root> --repo
<checkout> --cursor N --prefix-sha256 <hex>`, the Stop path's protocol
(612-617) on the brain's own cursor, only when the digest section was
emitted. Every read is local files: no fetch, no network.

**Composition, in order, under the byte bound and the deadline.**

1. The header, one line: `BRAIN SEAT <nickname> for <fleet>, declared by
   <name> <date>. The standing instruction is the record at <path>.`
2. The packet. If the header plus the whole packet file is at most sixty
   percent of the bound, the packet rides verbatim. Otherwise the payload
   carries the packet's path and, verbatim, only its section headed "The
   standing instruction" (the four rules), preceded by `PACKET TOO LARGE FOR
   THIS CHANNEL (<packet bytes> of <bound>); read the whole record at <path>
   before anything else`. If even that does not fit, the payload is the
   header plus `THE STANDING INSTRUCTION DOES NOT FIT THIS CHANNEL; read
   <path> now and take no work until you have`. A missing or unreadable
   packet yields `THE ROLE PACKET IS MISSING at <path> (<reason>); this seat
   is declared brain and has no instruction: take no work, tell Wido`. The
   packet section is read first and is never cut by the deadline or by any
   later section.
3. The remaining budget is shared in this order, each section taking what it
   needs up to its share and passing the rest on: asks thirty percent, held
   claims ten percent, fleet twenty-five percent, digest the remainder.
   - `ASKS AWAITING WIDO:` one line per open channel question, `<id> goal
     <id> <kind> from <machine> since <date>: <wants>`, each line cut at 160
     bytes with a trailing `…`, newest first, then `N more; run metasystem
     channel show --root <checkout> --id <id>` for the rest. Enumeration is
     the boot verb's own tolerant walk, not `listQuestions`: an unreadable or
     malformed question file is skipped and counted.
   - `HELD HERE:` one line per claim this machine holds, cut at 160 bytes,
     then `release them to a node: metasystem goal release --root <checkout>
     --id <id>`; omitted when none.
   - `FLEET:` one line per claimed goal (`<id> claimed by <machine> since
     <date>: <next step>`), one per approved unclaimed goal, one count line
     for queued unapproved goals, each cut at 160 bytes, then one line per
     non-terminal job record (`job <id> <status> <role> goal <id>`), then
     the census line (`census <verdict> custody N announced N untracked N,
     <age>`). Cut order inside the section: job lines first, then claim
     lines, always with a `N more` count.
   - `NARRATOR DIGEST since the brain last booted:` the log after the brain's
     cursor at artifacts/agents/steward/narrator-digest-brain-cursor.json,
     oldest lines cut first with `N older lines cut; read
     records/narrator-digest.log`. The digest package's `Pending` and
     `Advance` take a cursor name; the human cursor keeps its path and its
     Stop-hook protocol untouched. A cut digest still advances the brain's
     cursor to the end of the log, because the log is a durable record.

**Every input maps to one line on error**, appended to the section it
replaces and listed in `errors`:

| Input | Line on error |
| --- | --- |
| brain cursor or digest log | `DIGEST unreadable (<reason>); read records/narrator-digest.log by hand; the brain cursor was not advanced` |
| accepted ledger projection | `LEDGER unreadable (<reason>); run metasystem goal list --root <checkout>` |
| a channel question file | counted in `N unreadable question files; run metasystem channel show --root <checkout>` |
| a job record | counted in `N unreadable job records under artifacts/agents/jobs` |
| census verdict | `CENSUS absent or unreadable (<reason>); run metasystem supervise status --repo <checkout>` |
| the deadline | `BOOT DEADLINE: <sections not read> skipped after <n> ms; run metasystem brain boot --root <state root> --repo <checkout> by hand` |

The deadline is checked between sections and once inside each enumeration
every 200 files. A boot verb that exits non-zero, or whose output is not the
object above, is handled by the hook as a corrupt declaration would be: the
hook reads the packet's standing-instruction section itself, emits it with
`BRAIN BOOT FAILED: <stderr tail>; run metasystem brain boot by hand`, and
advances no cursor. An uninstructed brain never starts silently.

## 3. The fence

One Go function, `brain.Fence(stateRoot, act, ledgerRemoteURL)`, reads the
record (decision 1) and returns the refusal text for an act in a declared
checkout, the remedy line for a corrupt one, and nothing for an undeclared
one. It keys on the record alone: never on the runtime, the model, or the
caller's process signature, except where decision 3d names the caller class.
Exposed to the shell as `metasystem brain fence --root <state root> --act
<act>` (JSON `{"state":"...","fenced":bool,"detail":"..."}`), and called
directly in Go elsewhere. A corrupt record fences every act below.

Enforced by the engine, and where:

- **delegate (a).** Two places in dispatch.sh that between them every launch
  attempt passes: the legacy-grammar guard (28-35), which for a declared or
  corrupt checkout prints the engine's detail instead of its own one-liner,
  as `{"outcome":"BRAIN_REFUSED","headline":"refused","detail":"<detail>"}`,
  exit 2; and the top of `dispatch_job` and `follow_up` (2596, 1807), which
  record `BRAIN_REFUSED` through `record_delegate_outcome` so `runDelegate`
  prints the typed JSON. The detail is: `this checkout is declared the
  brain; the brain never dispatches. A node runs, from its own checkout:
  metasystem delegate --role <role> --brief <file> --goal <id>
  --destructive-reach <class>`. No job record is written. The breach-stop
  call the steward tick makes through dispatch.sh (internal/steward/tick.go:82)
  is not a launch and is not fenced.
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
- **a human's word (d).** In `syncReq` (goalsync_mutations.go:32-77), the
  seam where `--by` becomes `Actor.Human`: when the checkout reads declared
  or corrupt, `by` is non-empty, and the caller classification is not
  `ClassHuman`, the request is refused before any verb publishes or records
  a proof: `this checkout is declared the brain; the brain never carries a
  human's word into goal <verb>, not as --by, not as a relayed word, not as a
  channel reference. Wido runs, from an agent-free terminal: metasystem goal
  <verb> --root <checkout> --id <id> --by <name> <the verb's own flags>`. The
  enrolled terminal passes, because Wido acts at this very checkout from his
  own terminal; the fixture proof passes under the fake root only. Every verb
  that builds its request through `syncReq` inherits the fence by
  construction, today: open, set-next, promote, park, unpark, done, reopen,
  declare-free, prune, claim, release, steal, edit, set-pin, set-arc, detach,
  split, approve, unapprove, set-budget, accept-risk,
  discharge-review-obligation, classify-sweep, set-obligation, resume, and
  enroll-terminal. A verb added later that calls `syncReq` inherits it
  without a line of its own.

  The routes that admit a named human without `syncReq`, and what each does:
  - `goal repair --accept-remote --by` (goalsync_verbs.go:416-449): gains the
    same check before `RepairAcceptRemote`; refusal names `goal repair`.
  - `goal migrate` through `goalActor` (goalsync_verbs.go:298-311): gains the
    same check; a converted brain checkout never migrates, so this is
    closure, not a live path.
  - `goal recover` (recover.go:235-243): replays a classify-sweep
    confirmation whose human passed the seam when the journal entry was
    written; it mints nothing and is not fenced.
  - the channel poll (poll.go:322, 367): approves under a verified channel
    answer, with the answering user as the human and the code as proof. The
    brain adds no word of its own; this is Wido's act arriving by transport,
    and it passes. It is the one route that publishes a human's approval from
    a brain checkout, and the page names it so nobody looks for a fence
    there.
  - `goal discharge-review-obligation` (211-212) sets the human only for a
    `ClassHuman` caller and is covered by the seam's own rule.
  - `goal.Actor{...}` literals inside internal/ (dispatch/claim.go,
    dispatch/stop.go, dispatch/finding_register.go, steward/revive.go,
    channel/question.go) carry no human and need no fence; the claim ones
    are covered by (c).

  The static fixture `brain-actor-seam-coverage` (decision 8) lists every
  `goal.Actor{` and `Actor.Human =` site outside `syncReq` in cmd/metasystem
  and internal/, and fails when a new one appears that is not in its
  allow-list, so "inherits by construction" stays checkable.

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
  `composeDisplay` as the first prefix line (586-594), so no ladder branch
  can suppress it: `BRAIN SEAT: nodes hold <n> claims (<ids>); <m> approved
  goals await a node; <a> asks await Wido (<ids>); <d> drafts await approval
  (<ids>)`. A corrupt record adds the remedy line from decision 1 as the
  second line. A claim this machine holds is a third line, `HELD HERE:
  <ids>; release them to a node`, and is not prodded.
- `readClaimableBudgetedWork` and `enforceIdleBacklog` are not called
  (225-226, 241). `IdleRefusal` is never true, `IdleBlocks` never
  increments, and `escalateIdleBacklog` is unreachable: no continuation
  intent, no steward claim, no idle alarm on the brain's behalf.
- The goal clause (660-709) does not run; the ladder's other branches are
  unchanged: plan open work blocks once per signature, unwatched work blocks
  once, unreadable inputs veto the all-clear, human stop authorization is
  consumed as today. Asks and drafts are the brain's own open work and are
  reported, never blocked on.

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

- Core names no runtime. The context field and its bound are registry
  declarations, and the hook asks the registry; the engine's `brain boot`
  output is the same text for every runtime.
- The hook decides nothing: `brain boot`, `brain fence`, and `turn-verdict`
  decide; the hook relays JSON and advances the brain's cursor.
- The packet's text lives in records/misc/fleet-coordinator-brain-role-packet.md
  and is read at boot. No copy, no excerpt in code; the shrunk form is the
  record's own section, read by heading at boot.
- The ledger's bytes and verbs are unchanged. The one new verb family is
  `brain`; the new registry declarations are `StartContextField` and
  `StartContextBytes`; the digest package gains a cursor-name parameter with
  the human cursor as default; the scan gains two fields; the status post
  gains one conditional line.
- Power of attorney (a scoped, time-boxed delegation of approval) is a later
  ruling and a later fence exception; this page leaves the fence whole.

## 8. Fixtures

Each names what it observes and where it joins. A declared bed means a
scratch checkout with a fake-runtime root, a bare ledger remote, a scratch
registry home, and `brain declare --fixture-human-authority`.

| Fixture | Observes | Joins |
| --- | --- | --- |
| brain-boot-declared | Declared bed, one digest line appended to records/narrator-digest.log, one open question written by `channel ask` on the fake provider; `supervision-hook.sh claude start` prints one JSON object whose `hookSpecificOutput.additionalContext` carries the packet's first heading, the digest line, and the question id, and is at most 10,000 bytes; the brain cursor advanced; the human cursor file unchanged | supervision-hook-fixtures.sh |
| brain-boot-undeclared | The same bed after `brain withdraw`: the start object has no context field and none of the three texts | supervision-hook-fixtures.sh |
| brain-boot-corrupt | The record overwritten with `{broken`, then with a valid record whose `fleet` is another URL: the start object carries the standing-instruction section and the remedy line; `brain show` exits 1 with the reason | supervision-hook-fixtures.sh |
| brain-boot-bound | 400 digest lines, 80 job records, 60 open questions each with a 2,000-byte `wants`: the payload is at most the bound, every ask line is at most 160 bytes and ends with `…`, `cut` names job lines then digest, the packet bytes are intact; with `--bytes 2000` the payload carries the packet path and the standing-instruction section only and the `PACKET TOO LARGE` line; with `--bytes 300` it carries the `DOES NOT FIT` line | brain-fixtures.sh (new) |
| brain-boot-errors | A malformed brain cursor, one unparsable question file, one unparsable job record, and no census file: the payload carries the packet whole plus one error line per input, `errors` lists four, and the cursor file is untouched; with `--deadline-ms 1` the payload carries the packet and the `BOOT DEADLINE` line | brain-fixtures.sh (new) |
| brain-boot-verb-fails | The boot verb replaced by a shim that exits 1: the start object still carries the standing-instruction section and `BRAIN BOOT FAILED` | supervision-hook-fixtures.sh |
| brain-delegate-refuses | `metasystem delegate` on the fake runtime from a declared bed: outcome `BRAIN_REFUSED`, detail ends with the node's `metasystem delegate` command, no job record exists; a direct `scripts/agents/dispatch.sh dispatch ...` from the same bed prints the same outcome and detail and exits 2 | dispatch-fixtures.sh |
| brain-land-refuses | From a declared leg: `land.sh --chain <root>`, `land.sh --direct-fix tier-1 ...`, `commit.sh --chain <root> -F <msg>`, and `commit.sh --direct-fix register-carriage -F <msg>` each exit 2 with the fence text and leave HEAD unchanged; a plain `commit.sh -F <msg> <record path>` succeeds | land-fixtures.sh |
| brain-claim-refuses | Declared bed with one approved goal: `goal claim` and `goal open --claim` are refused with the node's claim command; the ledger tip is unchanged | goal-cli-fixtures.sh |
| brain-human-word-refuses | Declared bed, agent-classified caller: one scenario per verb, each refused with the fence text naming that verb and leaving the tip unchanged: `approve --temporary-human-word`, `resume --temporary-human-word`, `resume --approved-ref`, `set-obligation --temporary-human-word`, `steal --by`, `set-pin --by`, `classify-sweep --confirm --by`, `set-budget --by`, `unapprove --by`, `accept-risk --by`, `repair --accept-remote --by`; then `approve --fixture-human-authority` succeeds, and `channel poll` on the fake provider with a verified answer approves the goal | goal-cli-fixtures.sh |
| brain-actor-seam-coverage | A grep over cmd/metasystem and internal/ for `goal.Actor{` and `Actor.Human =` outside `syncReq` equals the allow-list in the fixture; a new site fails it | brain-fixtures.sh (new) |
| brain-stop-seeded | Declared bed with two approved unclaimed goals, one open question, one draft opened by a different lineage on this machine, one plan carrying an open next step, and one plan waiting on the human; `report turn-verdict --stop-hook-active=true` three times: the first display line is `BRAIN SEAT` naming one ask and one draft every time, `idleRefusal` false every time, `blockSource` never idle-backlog, the open-work block fires once as today, no steward intent staged | goal-cli-fixtures.sh |
| brain-stop-corrupt | The same bed with `{broken` as the record: the first line is `BRAIN SEAT`, the second the remedy line, no idle refusal | goal-cli-fixtures.sh |
| brain-second-declaration-refuses | `brain declare` twice on one checkout refuses naming the withdraw command; a second checkout of the same bare remote under the same scratch registry home refuses naming the first checkout's path; a third checkout of a different bare remote under that home succeeds; after `brain withdraw` in the first, the second succeeds | brain-fixtures.sh (new) |
| brain-verbs-human-only | `brain declare` and `brain withdraw` from an agent-classified caller refuse; with the fixture authority under the fake root they act; `withdraw` removes a `{broken` record | brain-fixtures.sh (new) |
| brain-status-line | `channel status` composed on a declared bed carries `BRAIN: <nickname> for <fleet>`; on the undeclared bed it does not | channel-fixtures.sh |
| brain-absent-node-proceeds | Two clones of one bare remote; clone A declared brain with no session; clone B lands the goal leg and pushes green, and clone B's fake-runtime delegate starts | land-fixtures.sh (landing) and dispatch-fixtures.sh (dispatch) |

brain-fixtures.sh registers in scripts/validate-metasystem.sh beside the
other fixture scripts. What no fixture can prove in a delegate sandbox: the
`up` leg of the start path and the runtime's own consumption of the context
field; the orchestrator runs the suite outside the sandbox, and the field's
effect is checked once by a human opening a declared seat.

## 9. Self-grade

Confidence 0.8 in the designation, the fence seams, and the verdict change;
each seam is a line in the tree today. Weakest claim: that the runtime adds
the declared context field to the session, accepts one JSON object with both
that field and a `systemMessage`, and honours the 10,000-character bound as
its manual states. It rests on the runtime's manual, not on anything in this
tree, and the manual's version is not pinned here. Reject condition: a
declared seat opens and the model does not see the packet; then the channel
decision returns here and the payload rides plain stdout instead. Second
weakest: that the channel poll is the only route outside `syncReq` that
publishes a human's approval; the static fixture exists so that a new one
fails loudly instead of silently.
