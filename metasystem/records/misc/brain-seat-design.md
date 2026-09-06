# Design: the brain seat boots with its standing instruction

Goal: fleet-coordinator-brain (arc headless-fleet, tier 3). The role packet is
landed at records/misc/fleet-coordinator-brain-role-packet.md; its four rules
(never builds, never dispatches, never approves, never a bottleneck) are fixed
and this page does not restate them. What the goal still owes, in Wido's words:
"the standing instruction wired into the seat's boot so it loads unprompted,
and the narrator's digest as its input." Written 2026-09-06, the day a seat
again stood in for the brain and the runner at once. Every claim about the
tree cites file and line as they stand in this worktree on that date.

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
  runtime registry already declares per-runtime session facts the hook reads
  indirectly, for example `runtime session-env` (main.go:330,
  internal/runtimes/runtimes.go:58-61); exit 1 from such a query is the
  declared absent capability (supervision-hook.sh:360).
- **The digest.** `steward digest-pending` returns the narrator log after one
  machine-local cursor (internal/narratordigest/digest.go:225-250); the cursor
  path is fixed at artifacts/agents/steward/narrator-digest-cursor.json
  (63-65) and advances only through `digest-advance` after the Stop payload is
  emitted (supervision-hook.sh:612-617). There is one cursor per checkout, and
  it is the human's: it is how highlights reach whoever reads the seat.
- **The Stop verdict.** `report turn-verdict` reads the claimable budgeted
  backlog for every agent caller (internal/goal/turnverdict.go:225-226) and
  `enforceIdleBacklog` refuses the stop three times for an unchanged claimable
  backlog (293-354); at the third refusal `escalateIdleBacklog` selects the
  machine's held or first claimable goal, prepares a steward continuation
  intent, and records the incident (371-462). The goal clause reads this
  machine's claim as the Current goal and prods its next step (597-715,
  734-764). None of this knows about a seat that is forbidden to take work.
- **The acts.** Every operator dispatch enters `runDelegate`
  (cmd/metasystem/delegate.go:29-103), which execs dispatch.sh; a direct
  dispatch.sh call without the internal marker is refused and told to use
  `metasystem delegate` (scripts/agents/dispatch.sh:28-35). Every landing
  enters scripts/agents/land.sh, which calls scripts/agents/commit.sh with the
  landing flags (land.sh:305-317); commit.sh is also the wrapper for plain
  record commits (commit.sh:9-17). Every human-only goal verb (approve,
  unapprove, set-budget, resume, accept-risk, and open or edit with a tier
  below the derivation) passes through `proveGoalHumanAuthority`
  (cmd/metasystem/goalsync_mutations.go:237, 347, 682, 731), which accepts an
  enrolled-terminal proof or the relayed word pair (152-155,
  internal/humanauthority/authority.go:302-311). `goal approve --budget box`
  approves with the tier's configured box (internal/goal/approval.go:471-479).
- **Configuration and identity.** Keys resolve env, then .local, then mode,
  then committed, then default (internal/config/conf.go:5); `config validate`
  ignores every .local key except capability floors (validate.go:66-95). The
  machine nickname is git configuration, `metasystem.goal.machine`
  (internal/goal/actor.go:21-28). The state root for this template is the
  metasystem directory, and the turn-verdict state already lives at
  artifacts/agents/turn-verdict-state.json (turnverdict.go:183-185). The
  machine-wide supervision registry lives under
  `$METASYSTEM_SUPERVISION_REGISTRY_HOME/.metasystem/`
  (internal/registry/selection.go:9).
- **The brain's other inputs.** `goal list` and `goal show` read the accepted
  ledger without fetching (cmd/metasystem/goal.go:270-300, 412-442). Channel
  questions are files under artifacts/agents/channel/questions/<id>.json with
  a `state` of open or answered (internal/channel/question.go:41-57, 78-80).
  The census verdict is artifacts/agents/supervision/last-census.json.

## 1. Designation: a record under the state root

**Decision.** One seat becomes the brain when its checkout holds the record
`<state root>/artifacts/agents/brain.json`:

```
{"schema":1,"machine":"<nickname>","declaredBy":"<name>","declaredAt":"<RFC3339>"}
```

The engine owns the path through one function, `brain.Path(stateRoot)`, beside
the turn-verdict state. The hook and every verb read it in one line,
`metasystem brain show --root <state root>` (exit 0 with the record as JSON,
exit 3 when undeclared, the same absent code `config conf-value` uses). The
record is never committed: artifacts/ is ignored, and no verb copies it.

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
  with this machine's nickname (ResolveMachine, actor.go:21) and writes the
  host pointer `<registry home>/.metasystem/brain` naming the checkout path.
  It refuses when this checkout already holds a record ("this checkout is
  already the brain, declared by <name> at <time>; withdraw it first:
  metasystem brain withdraw --root <checkout> --by <name>") and when the host
  pointer names another checkout whose record still exists ("this host
  already has a brain at <path>; one brain per host; withdraw it there
  first"). A pointer to a checkout whose record is gone is stale and is
  replaced. Refusal exit 2.
- `metasystem brain withdraw --root <checkout> --by <name>` removes the record
  and the pointer when the pointer names this checkout. Exit 3 when nothing
  was declared.
- Both verbs refuse an agent-classified caller (`lease.ClassifyVerb` on the
  parent pid, the check `goal discharge-review-obligation` already makes,
  goalsync_mutations.go:211): "brain declare is a human act; run it from an
  agent-free terminal". A declaration on a node halts that node's dispatch
  and landing, so a seat must not be able to declare or undeclare itself.
  Fixtures pass `--fixture-human-authority` under the same exact
  fake-runtime-root rule `goal approve` uses (goalsync_mutations.go:156-158).
- One brain per fleet is Wido's rule. This page enforces it per host through
  the pointer; across hosts nothing machine-local can enforce it, and the
  page does not pretend to. The brain's status post names the brain's
  nickname, so two brains are visible as two posts; that visibility is not a
  fence and is not claimed as one.

## 2. Boot: the payload, its channel, its bound

**Channel.** The standing instruction must reach the model, not the screen. A
`systemMessage` is the runtime's user-facing notice; the runtime's
additional-context field is what enters the session's context. The engine
does not know field names: the runtime registry gains one declaration,
`StartContextField`, read by `metasystem runtime start-context-field
<runtime>` with the same contract as `session-env` (a dotted field path on
exit 0; exit 1 is the declared absent capability). For claude it is
`hookSpecificOutput.additionalContext`, and the object also carries
`hookSpecificOutput.hookEventName=SessionStart` because the runtime requires
it. For codex and devin the declaration stays empty until their hook manuals
are read; on an absent field the hook puts the payload in the `systemMessage`
and the first payload line says "this runtime has no session-context channel;
the packet reached the screen only".

**One object.** The start path today can print several JSON objects. It
changes to collect every message (pending incidents, arming outcome, re-arm
notice) and print one object at exit, with the brain payload in the declared
context field and the collected notices in `systemMessage`. The brain payload
is composed before the identity check (supervision-hook.sh:907-910), so an
unidentified agent process still receives its instruction.

**The verb.** `metasystem brain boot --root <state root> --repo <checkout>`
prints `{"declared":false}` for an undeclared checkout, and otherwise:

```
{"declared":true,"payload":"<text>","lines":N,"bytes":N,"cut":["..."],
 "digestCursor":N,"digestPrefixSha256":"<hex>"}
```

The hook relays `payload` and, after the object is written to stdout, runs
`metasystem brain digest-advance --root <state root> --repo <checkout>
--cursor N --prefix-sha256 <hex>`, the Stop path's protocol (612-617) on the
brain's own cursor. Every read is local files: no fetch, no network.

**Payload sections, in this order.**

1. `BRAIN SEAT <nickname>, declared by <name> <date>. The standing instruction
   follows; it is the record at <path>, read verbatim at boot.` Then the
   packet file's bytes, read from the metasystem root at boot. Never a copy;
   a missing file yields `THE ROLE PACKET IS MISSING at <path>; this seat is
   declared brain and has no instruction: take no work, tell Wido` and the
   fence still holds.
2. `NARRATOR DIGEST since the brain last booted:` the log after the brain's
   cursor at artifacts/agents/steward/narrator-digest-brain-cursor.json. The
   digest package's `Pending` and `Advance` take a cursor name; the human
   cursor keeps its path and its Stop-hook protocol untouched, so Wido's
   highlights are never consumed by the brain's read.
3. `FLEET:` one line per claimed goal (`<id> claimed by <machine>+<lineage>
   since <date>, next: <step>`), one line per approved unclaimed goal, one
   count line for queued unapproved goals, then one line per non-terminal job
   record (`job <id> <status> <role> goal <id> since <date>`) and the census
   verdict line (`census <verdict> custody N announced N untracked N, age`).
4. `ASKS AWAITING WIDO:` one line per open channel question (`<id> goal <id>
   <kind> from <machine> since <date>: <wants>`), or `none`.
5. `HELD HERE:` claims this machine holds, with `release them to a node:
   metasystem goal release --root <checkout> --id <id>`; omitted when none.

**Bound.** 320 lines or 32,768 bytes, whichever is reached first. Section caps
first: digest 120 lines, fleet 60, asks 40, held 10; a capped section ends
with `N more; read <path or command>`. Over the total, cut in this order:
fleet job lines, then digest lines oldest first, then fleet claim lines. The
packet, the asks, and the held claims are never cut. A cut digest still
advances the brain's cursor to the end of the log: the log is a durable
record at records/narrator-digest.log and the cut line names it.

## 3. The fence

One Go function, `brain.Fence(stateRoot, act)`, returns the refusal text for
an act in a declared checkout and nothing for an undeclared one. It keys on
the record alone: never on the runtime, the model, or the caller's process
signature, except where decision 3d names the caller class. Exposed to the
shell as `metasystem brain fence --root <state root> --act <act>` (JSON
`{"fenced":bool,"detail":"..."}`), and called directly in Go elsewhere.

Enforced by the engine, and where:

- **delegate (a).** In dispatch.sh at the top of `dispatch_job` and
  `follow_up` (dispatch.sh:2596, 1807), the one place every launch passes,
  operator or internal. Refusal records the outcome `BRAIN_REFUSED` through
  `record_delegate_outcome` so `runDelegate` prints the typed JSON, and the
  detail is: `this checkout is declared the brain; the brain never
  dispatches. A node runs, from its own checkout: metasystem delegate --role
  <role> --brief <file> --goal <id> --destructive-reach <class>`. No job
  record is written. The breach-stop call the steward tick makes through
  dispatch.sh (internal/steward/tick.go:82) is not a launch and is not fenced.
- **land, chain and direct-fix (b).** In land.sh after argument parsing and
  before `verify checks` (land.sh:391), and in commit.sh when any landing
  flag is present (`--chain`, `--direct-fix`, `--revert-of`, `--root-job`,
  `--test-receipt`; commit.sh:40-46). Plain record commits through commit.sh
  stay open: the brain writes records. Refusal exit 2 with `land refused: this
  checkout is declared the brain; the brain never lands. A node lands from
  its own checkout: scripts/agents/land.sh -m <message> --goal <id> --chain
  <root-job> <pathspec>`.
- **claim (c).** In `goal claim` and `goal open --claim`: `this checkout is
  declared the brain; the brain never claims. A node claims: metasystem goal
  claim --root <checkout> --id <id>`.
- **approve and the other human-only goal verbs (d).** In
  `proveGoalHumanAuthority`: when the checkout is declared, the relayed-word
  path is refused for every verb that passes through it; the enrolled
  terminal proof and the fixture proof pass untouched, because Wido approves
  at this very checkout from his own terminal. Refusal: `this checkout is
  declared the brain; the brain never carries a relayed word into goal
  <verb>. Wido runs, from an agent-free terminal: metasystem goal approve
  --root <checkout> --id <id> --by <name> --budget box`. The verb named in the
  command is the one refused.

Enforced by the instruction only: writing code, tests, designs, and briefs
(file writes have no engine seam); `git commit` outside commit.sh (the
pre-commit guard already refuses it for everyone); setting the internal
dispatch marker by hand (a lie, not a path). The page names these so the
builder does not look for a fence that cannot exist.

## 4. Turn end

`report turn-verdict` reads the declaration from its `--root`. For a declared
brain:

- `readClaimableBudgetedWork` and `enforceIdleBacklog` are not called
  (turnverdict.go:225-226, 241). `IdleRefusal` is never true, `IdleBlocks`
  never increments, and `escalateIdleBacklog` is unreachable: no continuation
  intent, no steward claim, no idle alarm on the brain's behalf. The
  claimable backlog is the nodes' work and the fleet-pull duty's to notice.
- The goal clause (decide, 660-709) is replaced by one non-blocking display
  line: `BRAIN SEAT: nodes hold <n> claims (<ids>); <m> approved goals await
  a node; <a> asks await Wido (<ids>); <d> drafts await approval (<ids>)`. A
  claim this machine holds is reported as `HELD HERE: <ids>; release them to
  a node` and is not prodded. Asks and drafts are the brain's own open work
  and are reported, never blocked on: waiting on the human never blocks
  (the ladder's own rule, 646-653).
- Every other branch is unchanged: plan open work blocks once per signature,
  unwatched work blocks once, unreadable inputs veto the all-clear, human
  stop authorization is consumed as today.

A draft is a queued goal with no `Approved` record whose opening history line
carries this machine's actor (internal/goal/file.go:25, 54, 72). An ask is an
open channel question from any machine. The hook prints nothing new; it
relays the display as today.

## 5. Handover in one word

When Wido says the word, the brain writes exactly one record: a goal open.

```
metasystem goal open --root <checkout> --id <id> --intent "<Wido's words>" \
  --next "<the step a cold seat resumes from>" \
  --risk severity=<n>,novelty=<n>,exposure=<n>,accumulation=<n> --basis "<why>" \
  [--elapsed-limit <d> --attempt-limit <n> --reserved-job-minutes-limit <n> \
   --active-job-limit <n> --review-round-limit <n>]
```

The tier derives from the four answers; the brain passes no `--tier` below it
and never `--temporary-human-word` (decision 3d refuses it anyway). The box is
the tier's configured box unless the brain proposes another tuple with the
five limits. The record is queued and unapproved; nothing on it is an
approval. The brain then hands Wido this line, and nothing after it:

```
metasystem goal approve --root <checkout> --id <id> --by <name> --budget box
```

When the draft carried its own tuple the line repeats the same five limits in
place of `--budget box`, so what Wido approves is byte-for-byte what the brain
proposed. No new ledger field is needed: intent, tier, tuple, the absent
approval, and the opening actor are already on the record.

## 6. Never a bottleneck

No verb a node runs reads, waits on, or is refused by the brain's declaration,
its session, or its presence; every fence in decision 3 keys on the calling
checkout's own record, so a brain that is down changes nothing a node does.
The fixture that proves it is `brain-absent-node-proceeds` (decision 8).

## 7. Agnosticism and boundaries

- Core names no runtime. The context field is a registry declaration, and
  the hook asks the registry; the engine's `brain boot` output is the same
  text for every runtime.
- The hook decides nothing: `brain boot`, `brain fence`, and `turn-verdict`
  decide; the hook relays JSON and advances the brain's cursor.
- The packet's text lives in records/misc/fleet-coordinator-brain-role-packet.md
  and is read at boot. No copy, no excerpt in code.
- The ledger's bytes and verbs are unchanged. The one new verb family is
  `brain`; the one new registry declaration is `StartContextField`; the digest
  package gains a cursor-name parameter with the human cursor as default.
- Power of attorney (a scoped, time-boxed delegation of approval) is a later
  ruling and a later fence exception; this page leaves the fence whole.

## 8. Fixtures

Each names what it observes and where it joins.

| Fixture | Observes | Joins |
| --- | --- | --- |
| brain-boot-declared | A scratch checkout declared brain, one digest line appended to records/narrator-digest.log, one open question written by `channel ask` on the fake provider; `supervision-hook.sh claude start` prints one JSON object whose `hookSpecificOutput.additionalContext` carries the packet's first heading, the digest line, and the question id; the brain cursor advanced; the human cursor file unchanged | supervision-hook-fixtures.sh |
| brain-boot-undeclared | The same checkout after `brain withdraw`: the start object has no context field and none of the three texts | supervision-hook-fixtures.sh |
| brain-boot-bound | 400 digest lines and 80 job records: payload within 320 lines and 32,768 bytes, `cut` names job lines first then digest; the packet bytes are intact | brain-fixtures.sh (new) |
| brain-delegate-refuses | `metasystem delegate` on the fake runtime from a declared checkout: outcome `BRAIN_REFUSED`, detail ends with the node's `metasystem delegate` command, no job record exists | dispatch-fixtures.sh |
| brain-land-refuses | `land.sh --chain <root>` and `land.sh --direct-fix tier-1 ...` from a declared leg exit 2 with the fence text; HEAD unchanged; a plain `commit.sh` of a record succeeds | land-fixtures.sh |
| brain-approve-refuses | `goal approve --temporary-human-word ... --review-by ...` in a declared fake-root bed is refused with the fence text and the terminal command; the same approve with `--fixture-human-authority` succeeds | goal-cli-fixtures.sh |
| brain-stop-not-idle | A declared bed with two approved unclaimed goals; `report turn-verdict --stop-hook-active=true` three times: `idleRefusal` false every time, `blockSource` never idle-backlog, no steward intent staged, display carries `BRAIN SEAT` | goal-cli-fixtures.sh |
| brain-second-declaration-refuses | `brain declare` twice on one checkout refuses naming the withdraw command; a second scratch checkout under the same scratch registry home refuses naming the first checkout's path; after `brain withdraw` there, it succeeds | brain-fixtures.sh (new) |
| brain-verbs-human-only | `brain declare` and `brain withdraw` from an agent-classified caller refuse; with the fixture authority under the fake root they act | brain-fixtures.sh (new) |
| brain-absent-node-proceeds | Two clones of one bare remote; clone A declared brain with no session; clone B lands the goal leg and pushes green, and clone B's fake-runtime delegate starts | land-fixtures.sh (landing) and dispatch-fixtures.sh (dispatch) |

brain-fixtures.sh registers in scripts/validate-metasystem.sh beside the
other fixture scripts. What no fixture can prove in a delegate sandbox: the
`up` leg of the start path and the runtime's own consumption of the context
field; the orchestrator runs the suite outside the sandbox, and the field's
effect is checked once by a human opening a declared seat.

## 9. Self-grade

Confidence 0.8 in the designation, the fence seams, and the verdict change;
each seam is a line in the tree today. Weakest claim: that the runtime adds
the declared context field to the session and accepts one JSON object with
both that field and a `systemMessage`. It rests on the runtime's manual, not
on anything in this tree. Reject condition: a declared seat opens and the
model does not see the packet; then the channel decision returns here and
the payload rides plain stdout instead.
