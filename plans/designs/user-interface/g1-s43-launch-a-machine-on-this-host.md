# g1-s43: launch a machine on this host

- Kind: design
- Id: 01M3BQAVYXE2AT6F0JG9YB64PG
- Status: accepted
- Goals: browser-interface fleet-join-bootstrap

Wido, 2026-09-25: "I want to be able to launch a new machine for the fleet,
obviously, with the restriction that that will be at the same host where the
UI is running." Author Fable. Every cite re-read at `b50abb959`.

## 1. What exists and binds

1. **A machine is a clone.** Every machine of this fleet on this host is a
   full clone of the same repository beside the others
   (`agentic-tools-m1b`, `-m1c`, `-m1d`, `-m1e`), with its own `.git`,
   its own nickname in git configuration (`metasystem.goal.machine`, read
   by `goal.ResolveMachine`, internal/goal/actor.go:3-8) and these keys
   beside it: `metasystem.steward.landing-ref refs/remotes/origin/main`
   and `goal.human.<name> <display name and address>`. A worktree cannot
   be a machine: git configuration and the ledger's accepted ref are per
   repository, so two worktrees of one clone are one machine (that is what
   scripts/agents/second-session.sh makes, a second session of the same
   machine). The kit's own `agentic-tools-*` worktrees all carry `m1`.
2. **The join sequence is designed and not built.** plans/fleet-join-bootstrap-design.md
   (accepted, goal fleet-join-bootstrap queued) specifies it step by step:
   host preconditions; build the engine with scripts/agents/go-build.sh
   (the one fenced, stamped build, go-build.sh:1-7, 95-96); lay down the
   roster in `metasystem.conf.local` and validate it; set the nickname;
   `goal fetch`; enroll the engine with `steward arm`, either from the
   agent-free terminal (`requireHumanTerminal`, cmd/metasystem/steward_verbs.go:684)
   or with the human's own temporary word, `--temporary-human-word <word>
   --review-by <date>` (steward_verbs.go:662-663; the pair rule
   `ValidateTemporaryWordPair`, internal/humanauthority/authority.go:329-343),
   which enrolls TEMPORARILY with the word recorded on the identity (the
   exception is R-29-m2; R-37-m3 is the re-arm of rebuilt engines); the
   date is a review date the identity carries and nothing enforces
   (`ValidateTemporaryWordPair` accepts a past date, authority.go:325-343;
   the identity reader never compares it to the clock, identity.go:129).
   `steward arm` mints the identity AND launches the runner (runner.go:837).
   The browser session is not terminal authority: `OutcomeSession` has no
   such standing (authority.go:42). Neither scripts/agents/join-fleet.sh nor
   `metasystem.conf.local.template` exists; that design chose a script
   because the engine does not exist before its first step builds it.
3. **Local configuration travels by a manifest.** second-session.sh copies
   the adapter-declared local files (`.claude/settings.json`,
   `.claude/settings.local.json`, `.codex/config.toml`, the three
   `.devin` files, from each adapter's `local-config-paths`) with
   `metasystem validate session-isolation --source-root --destination-root
   --manifest --harness-root`, which copies and audits isolation, and
   returns the new harness root. `metasystem.conf.local` carries the
   roster and the secrets (the channel's one-time-code secret, the evidence
   root). second-session.sh copies only the adapter files, not this one; a
   second session shares the primary's harness. Copying it to a new machine
   is this design's own decision (D7), and nothing in this design reads it.
4. **The interface's acts and their standing.** An act runs in the server
   under the human standing of the signed-in session
   (internal/ui/act/act.go:136, `SignedIn`; `humanauthority.OutcomeSession`
   is recorded as such, act.go:387-389). The server spawns and owns
   processes through internal/ui/lifecycle (`Launch`, `ExecSpawn`,
   `Child`, launch.go:29-141) and its own serve lifecycle under the
   ownership gate (cmd/metasystem/ui.go:178). Long acts have no pattern yet:
   every act today returns within one request.
5. **The Fleet page** (g1-s42, landed at `b50abb959`) lists every machine
   from the presence copy and the accepted tip's claims, re-reads on the
   `fleet` stream event, and shows this seat's arming and publication.
   A launched machine appears there by itself the moment its steward
   publishes presence, which is the first tick after `up`.
6. **What a machine runs.** Arming starts the steward runner, which ticks,
   reconciles, publishes presence and watches. Work happens when a session
   main runs in the checkout: today an interactive `claude` or `codex` a
   human starts there (its SessionStart hook announces it, scripts/enforcement/claude-code-hooks.json),
   or, unattended, `metasystem mission start` under a sealed contract
   (plans/first-headless-run-plan.md), which goal machinery-runs-unattended-on-codex
   is still building. Starting the session is not part of joining.

## 2. Decisions

- D1. **A launched machine is a clone beside this checkout**, by default
  `<parent of this checkout>/<repository>-<nickname>`, where `<repository>`
  is the name of the ledger remote's repository read from the origin URL
  (`agentic-tools` for `https://github.com/widoriezebos/agentic-tools.git`;
  another project gets its own name, Wido 2026-09-25: the remote's name is
  the right default, never a fixed word). The destination is a default the
  human may change in the sheet, kept on this host. The clone comes from
  this checkout's own objects (fast, offline, no credential prompt) with its
  `origin` set to this checkout's origin URL, then fetched from origin so
  it reads the fleet's tip. Never a worktree (1.1). Same host by
  construction: the interface's server runs here and writes here.
- D2. **The sequence is the fleet-join design's, owned by one engine verb.**
  `metasystem seat launch` runs the steps of 1.2 in order as subprocesses
  of the existing owners, records each step's outcome, stops at the first
  refusal with the owner's words verbatim, and is idempotent: run again, it
  skips every step whose precondition already holds. The fleet-join design
  chose a script because no engine exists before the first step; here the
  LAUNCHING seat's engine exists and composes for the new clone, whose own
  freshly built engine takes over from the fetch onward. The verb invokes
  scripts/agents/go-build.sh as the kit's one designated build owner; the
  layering rule (docs/architecture.md:101) admits caller-supplied adapter
  and gate commands, not engine-owned shell orchestration, so the build
  adds one sentence there naming this invocation as that owner's, and no
  other shell runs under the verb.
- D3. **The human's own word enrolls the machine, through the sheet.** The
  interface never invents an authorization. The signed-in human types the
  word in their own words and picks the review-by date, and the verb passes
  both to `steward arm --temporary-human-word --review-by` on the new
  clone: the lawful path for a human who is not at that checkout's terminal
  (R-37-m3), unchanged. The launched machine carries a temporary enrollment with a review due
  on that date; the date stops nothing, a human re-approves at a terminal,
  and the page says exactly that. The sheet refuses a date in the past,
  which the engine's validator does not.
  Making the signed-in session itself a full enrollment is the bridge
  design's D5 and not this slice.
- D4. **The launch is a long act with a lock and a record.** The server
  writes the record first, then starts the verb detached under its
  ownership gate with a scrubbed environment (no `METASYSTEM_*` overrides,
  section 3), and answers. The verb holds one host-scoped lock for its whole
  life, `<host temp dir>/metasystem-seat-launch.lock` taken with flock the
  way proof admission takes its own, so two interface checkouts or two
  requests cannot launch at once; the record carries the launch id and the
  verb's process identity (pid and start time). After every step the verb
  rewrites the record atomically. Reconciliation: whenever the record is
  read, on `/api/fleet` and at server start, a `running` record whose
  process identity is dead is marked `failed` with `the launch process
  died after <step>`, and its lock is stale by definition. The page reads
  the record through the payload and re-reads on the `fleet` event, which
  the server also emits when a record changes. No timer.
- D5. **One launch at a time on this host, and only signed in.** The lock
  refuses a second launch with the running one's nickname. The act requires
  a signed-in session and nothing weaker: the act layer's `mayAct` still
  admits boot authority without a Partner (httpd/acts.go:338), so this route
  checks `OutcomeSession` itself and answers the existing `signIn: true`
  shape otherwise. It spends disk, a build and the human's credentials.
- D6. **Joining is the slice; running is the next.** A launched machine is
  reachable and has no session. It is not quiet: when claimable work exists
  and no session is joined, its steward raises `IDLE_BACKLOG_DEAD` and
  queues that notification again after every delivery (verdict.go:111,
  tick.go:368). The card says so in words beside the two commands a human
  has: start a session by hand, or stop the machine with `metasystem stop
  --repo <dest>/metasystem`. Starting the session from the page is g1-s44.
- D7. **The roster and secrets are copied, once, as bytes.** A machine
  needs this seat's roster and the fleet channel's one secret to be a seat
  at all; the fleet-join design's template would stop the launch for a hand
  edit, which is the wrong step for a button. So the verb copies this
  seat's `metasystem.conf.local` to the clone as bytes, rewrites one key it
  knows, the evidence root, through the engine, and never parses the rest;
  the human edits the copy afterwards if the machine should differ.

## 3. The verb: `metasystem seat launch`

```
metasystem seat launch --machine <nickname> [--from <this checkout>] \
  [--destination <dir>] \
  [--temporary-human-word '<the human's words>' --review-by <YYYY-MM-DD>] \
  [--resume <launch id>] [--record <path>] [--json]
```

Preflight, before any step and before the lock, by name:
`SEAT_LAUNCH_NICKNAME_INVALID`, judged by `seat.ValidateMachineName`
(internal/seat/record.go:36: the charset, and never `.` or a `..`
substring), and never this checkout's own nickname;
`SEAT_LAUNCH_NICKNAME_TAKEN` when a sibling clone carries it, a presence
ref names it, or a claim at the accepted tip names it; `SEAT_LAUNCH_DESTINATION_EXISTS`
unless `--resume` names the launch that created it; `SEAT_LAUNCH_DESTINATION_INSIDE_A_CHECKOUT`;
the word pair's own refusal. Then the lock (D4): `SEAT_LAUNCH_RUNNING`
with the running launch's nickname and id. Then the steps, each a record
entry with `outcome: pending | done | skipped | failed`, the owner's words
and the time. Every subprocess runs on an owned process group under a
bound, the ledger fetch's budget for git and network, ten minutes for the
build, with a scrubbed environment: `METASYSTEM_*` variables of the
launching process are dropped, so an inherited `METASYSTEM_EVIDENCE_ROOT`
cannot outrank the copied file (config/resolve.go:164; the interface's
spawn inherits its environment, lifecycle/launch.go:95).

| step | precondition, verified as a postcondition on resume | command, from the launching seat | refusal |
|---|---|---|---|
| 1 clone | `<dest>` absent, or created by this launch and `git -C <dest> rev-parse HEAD` answers | `git clone --quiet <this checkout> <dest>`; `git -C <dest> remote set-url origin <origin url>`; copy `goal.sync-remote`, `goal.sync-branch`, `metasystem.steward.landing-ref`, `metasystem.steward.notify-command` and every `goal.human.*` key that this checkout sets; record the commit cloned | git's words |
| 2 tracking | always | `git -C <dest> fetch --no-tags origin`, so `refs/remotes/origin/main` exists; the ledger fetch of step 6 deliberately does not refresh it (txn.go:151) | git's words |
| 3 engine | `<dest>/metasystem/bin/metasystem` absent, or its stamp is not `<dest>`'s HEAD | `<dest>/metasystem/scripts/agents/go-build.sh` (D2); record the stamp built | the build's or the fence's words |
| 4 configuration | `<dest>/metasystem/metasystem.conf.local` absent | write it atomically (temp file, rename): this seat's file as bytes, with `evidence.root` rewritten to the sibling of this seat's EFFECTIVE root, `filepath.Join(filepath.Dir(canonical(<root>)), <nickname>)`, where the root is read through `<this>/bin/metasystem config get evidence.root` under the scrubbed environment; the verb creates that directory, refuses when its canonical path equals this seat's root or resolves through a symlink elsewhere, and on resume accepts an existing directory only when this launch created it; then `validate session-isolation --source-root <this> --destination-root <dest> --manifest <adapter paths> --harness-root <this>/metasystem` for the runtime files; then `<dest>/metasystem/bin/metasystem config validate --conf <dest>/metasystem/metasystem.conf --repo <dest>` (config_verbs.go:145 takes the file by `--conf`; the roster in `.local` is not overlaid by today's validator, validate.go:74, and `--resolved` from the fleet-join design is not built: the roster copied from a running seat is complete by construction, and this is stated to the human) | the validator's words |
| 5 nickname | `metasystem.goal.machine` unset in `<dest>`, or set to this launch's nickname | `git -C <dest> config metasystem.goal.machine <nickname>` | |
| 6 ledger | always | `<dest>/metasystem/bin/metasystem goal fetch --root <dest>` (a fresh clone validates the canonical tree and creates its accepted ref, fetchadvance.go:51), then `goal next --root <dest>` for the one orientation line the record keeps | the fetch's words |
| 7 enrollment | no identity at `<dest>`, or an identity whose enrolled digest is not the binary now installed (identity.go:403 is presence, not proof) | `<dest>/metasystem/bin/metasystem steward arm --repo <dest>/metasystem --temporary-human-word '<word>' --review-by <date>`; from a terminal without the pair, the caller's enrolled terminal, as `steward arm` does. Arming mints the identity and launches the runner (runner.go:837). The word is never written into the record | the arm's words; `SEAT_LAUNCH_WORD_REQUIRED` when a resume reaches this step without the pair |
| 8 supervision | the supervision owner not alive at `<dest>` | `<dest>/metasystem/bin/metasystem up --repo <dest>/metasystem --recover-only --if-down`, the machinery-only form that authenticates the enrolled binary and starts what is missing without announcing a session (up.go:778); ordinary `up` announces a session and needs a runtime's ancestry (up.go:215, 693), which a detached child does not have | up's words |
| 9 presence | after 8 | the verb reads presence itself through the seat transport, in a fetch namespace of its own (`refs/metasystem/presence-fetch/<ulid>`, deleted after), first at once and then on each tick of an injected clock, until a record for `<nickname>` with this launch's generation is read or three of the new machine's ticks have elapsed; done when read; otherwise outcome `armed` with the note `armed; presence not confirmed within three ticks: read <dest>'s health`, which is not a failure to arm. The interface's own copy is not consulted: its fetcher runs only while a browser is open and starts at most once a minute (fleet/fetch.go:306) | |

`--resume <launch id>` reads that launch's record, verifies each done
step's postcondition from the table, redoes what does not hold, and
exempts from the preflight only the destination and the nickname the
record says this launch created. The verb's exit is the last step's
outcome; `--json` prints the record.

The record, `artifacts/agents/ui/launches/<launch id>.json`, one per
launch, the nickname inside it:

```
{ "schemaVersion": 1, "launch": "<ulid>", "machine": "m1f",
  "destination": "/…/agentic-tools-m1f", "clonedCommit": "…", "builtStamp": "…",
  "process": { "pid": 0, "startedAt": 0 }, "startedAt": "...", "endedAt": "..." | null,
  "outcome": "running" | "done" | "armed" | "failed",
  "reviewBy": "2026-10-02", "created": { "destination": true, "nickname": true, "evidenceRoot": true },
  "steps": [ { "step": "clone", "outcome": "done", "at": "...", "words": "" }, ... ],
  "orientation": "goal next's one line",
  "next": { "session": "cd /…/agentic-tools-m1f && claude", "stop": "metasystem stop --repo /…/agentic-tools-m1f/metasystem" } }
```

## 4. The act and the page

`POST /api/fleet/launch` with `{machine, destination, word, reviewBy}`, and
`{resume: <launch id>, word?, reviewBy?}` for a retry: refuses without a
signed-in session with the existing `signIn: true` answer (D5), validates
the nickname with `seat.ValidateMachineName`, the destination and the pair
(and a past date) with the verb's rules, writes the record with a fresh
launch id before anything runs, spawns `bin/metasystem seat launch` detached
under the server's ownership gate with the scrubbed environment, and
answers 202 with the record. The lock inside the verb decides the race
between two requests; the loser's record is marked failed with the
winner's nickname. The word travels to the verb's argument list and
nowhere else: not the record, not the capture, not a log. `GET /api/fleet` gains `launches`: every record on
this host, newest first, so a page opened later still sees a running or a
failed launch.

On the Fleet page, in the fleet block's header, one button: **Launch a
machine**. It opens a work-area sheet (the g1-s35 sheet, so the Partner
stays usable) with:

1. **Nickname**, proposed as the next free letter of this host's series
   (`m1f` after `m1e`), editable, validated live by the same rule as
   `seat.ValidateMachineName` and against the machines the page already
   shows.
2. **Where**: the destination path, proposed as the remote repository's
   name with the nickname appended, beside this checkout, and editable; an
   absolute path on this host, refused when it exists or lies inside another
   checkout.
3. **What it gets**: one sentence naming what is copied, "this seat's
   roster and local configuration, its runtime settings, and the fleet's
   ledger", and one naming what it will not do, "it starts no session; it
   joins, publishes presence and waits".
4. **Your authorization**: a text field for the word, in the human's own
   words, with the help term explaining that the machine carries a
   temporary enrollment with a review due on the date, that the date stops
   nothing, and that a human re-approves or stops it at a terminal; and the
   review-by date, defaulting to seven days out, editable, refused in the
   past. This field is the one thing on the page the Partner's capture never
   carries, and the sheet's draft is handed over like every g1-s35 sheet.
5. **Launch**, disabled until the nickname and the word validate.

On Launch the sheet becomes the launch card, which also lives in the fleet
block above the table while a launch is running or failed: the nickname,
the destination, the steps as a vertical list each with its outcome and the
owner's words under a failed one, the orientation line when the ledger
step is done, and at the end either the machine's row appearing in the
table (the card then folds to one line, "m1f joined 2 min ago; temporary
enrollment, review due 2 October", with the `next` commands), or
the failed step with **Retry**, which resumes this launch by its id; a
retry that has to reach enrollment again asks for the word and the date
again in the card, because the record never held them. The card names the
idle alerts a sessionless machine will raise (D6) beside the `next`
commands. No Remove: a failed clone is a directory the human deletes, and
the card says its path. An `armed` outcome shows as "armed; presence not
yet seen" with the health command, distinct from failed.

Help terms: `launch-machine`, `temporary-word`, `review-by`. The Partner's
page capture on Fleet gains the launches shown. The Partner's `fleet` tool
prints running and failed launches after the machines. No Partner act
launches a machine.

## 5. Authority, cost, safety

The act is the signed-in human's, and the enrollment's authority is the
human's own word, recorded on the identity the way the temporary path
already records it; the interface adds no authority of its own. The verb
never reads the secrets it copies. A launch costs one clone of this
repository, one stamped build and one armed steward; the host's proof
admission cap bounds proof reservations under this account and nothing
else: not builds, not disk, not the number of stewards, not git traffic.
The lock bounds launches to one at a time, the verb checks free disk at
the destination's parent against twice the size of this checkout before
cloning, and the card says what a machine costs while it runs. The
verb refuses to launch onto an existing directory, to reuse a nickname, or
to run two launches at once. A launched machine that is never given a
session sits idle and publishes presence, which the fleet page shows; the
human can stop it with `metasystem stop --repo <dest>/metasystem` from a
terminal, which is the existing human act.

## 6. Not here

Starting the session main (g1-s44). Launching on another host. Removing or
retiring a machine from the page (the bridge's membership slice). A machine
whose roster differs from this seat's: it copies, and a human edits the
copy afterwards. Automatic re-enrollment at the review-by date.

## 7. Verification and box

Go: unit tests on the verb's sequencer with a fake runner and an injected
clock: every step's command and precondition, stop at the first refusal
with the owner's words verbatim, resume verifying each postcondition and
redoing what does not hold, the word never in the record, the refusals by
name, the lock's one-at-a-time rule with two sequencers, the scrubbed
environment, the evidence-root sibling and its refusals, the presence
observation through a fake transport with the three-tick deadline and the
`armed` outcome, the reconciliation of a record whose process is dead; one narrow integration
test on `t.TempDir()`: clone from a local repository, the remote rewritten,
the keys copied, the configuration copied and rewritten, the nickname set,
the ledger fetched from a bare fixture, stopping before enrollment (arming
spawns a runner and is proven by the steward's own tests). Interface: unit
tests on the sheet's validation and the card's states; the walkthrough
fixture serves a running and a failed launch record; screenshots at 1280
and 400. The real proof, at rollout on this host: launch `m1f` from the
page, watch its row turn reachable, and read its health. Budgets as always.
Box: one build lane (Claude on Opus), one code read (Codex on Sol), after
Astra's read; two attempts, 150 to 210 job-minutes.

## 8. Self-grade

High on the shape: every step is an existing owner's verb and the sequence
is an accepted design's. Medium on the build step: go-build.sh runs the
gate fence, and a fence refusal in the new clone is a new failure class the
card must show verbatim. Medium on the configuration step: a copied
`metasystem.conf.local` may carry keys that are wrong for a second machine
beyond the evidence root; the design names only the one it knows about and
leaves the rest to the human's edit. Weakest: step 8's wait rides the
interface's presence copy, so a machine that armed and publishes on a rung
the interface does not yet read would report failed while being alive; the
card's words send the human to the new machine's health line. Reject
condition: Wido wants the machine to start working on launch, in which case
g1-s44 comes first and this slice grows.

## Dispositions (Astra read, 2026-09-25)

Ten material findings, each checked against the code it cites and folded.

| id | finding | fold |
|---|---|---|
| A1 | `steward arm` already launches the runner, and ordinary `up` announces a session a detached child cannot have | step 8 is `up --recover-only --if-down`; supervision readiness is checked apart from runner liveness |
| A2 | `config validate` takes the file by `--conf` and never overlays the copied roster; `--resolved` is unbuilt | the command corrected; the roster's completeness rests on its source being a running seat, and the page says so |
| A3 | no host-wide exclusion and no crash recovery | one flock-held host lock; process identity in the record; reconciliation of dead launches on every read |
| A4 | Retry contradicted the refusals and could not prove completed steps; the word was gone | resume by launch id, postconditions verified per step, exemptions only for what the launch created, the word asked again |
| A5 | the presence wait depended on a browser being open | the verb observes through the seat transport in its own namespace, on an injected clock; `armed` is an outcome distinct from failed |
| A6 | "enrolled until the date" promised an expiry nothing enforces | "temporary enrollment, review due"; the sheet refuses a past date |
| A7 | environment outranks the copied file and the sibling path could resolve wrong | a scrubbed environment for every step; the effective root read through the engine; canonical checks |
| A8 | the clone missed the ledger endpoint keys and the remote-tracking fetch; Linux needs a notify command | the keys copied; a tracking fetch step; the built commit recorded |
| A9 | a sessionless machine raises idle alerts repeatedly | said on the card beside the session and stop commands (D6) |
| A10 | the nickname rule admitted names the publisher refuses | `seat.ValidateMachineName` at the verb, the route and the sheet |

Also folded from the read: the `.local` copy is this design's own decision
(D7), not second-session.sh's precedent; the build script's invocation is
named as the designated build owner's (D2); the launch act checks the
session outcome itself (D5); the authorization field is excluded from the
Partner's capture; the admission cap's true scope and a disk check
(section 5); R-29-m2 named as the temporary enrollment's ruling. Wido's
naming rule of the same day, the remote repository's name as the default,
is in D1.
