# g1-s43: launch a machine on this host

- Kind: design
- Id: 01M3BQAVYXE2AT6F0JG9YB64PG
- Status: draft
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
   which enrolls TEMPORARILY with the word on the identity until the date;
   then `metasystem up`. Neither scripts/agents/join-fleet.sh nor
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
   root) and is copied wholesale between seats of one user on one host, as
   second sessions already do; nothing in this design reads it.
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

- D1. **A launched machine is a clone beside this checkout**,
  `<parent of this checkout>/agentic-tools-<nickname>`, cloned from this
  checkout's own objects (fast, offline, no credential prompt) with its
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
  freshly built engine takes over from the fetch onward. The verb runs the
  fenced build script the way launch lanes run adapter scripts.
- D3. **The human's own word enrolls the machine, through the sheet.** The
  interface never invents an authorization. The signed-in human types the
  word in their own words and picks the review-by date, and the verb passes
  both to `steward arm --temporary-human-word --review-by` on the new
  clone: the lawful path for a human who is not at that checkout's terminal
  (R-37-m3), unchanged. The launched machine is enrolled temporarily until
  the date, exactly as a seat armed that way is today, and the page says so.
  Making the signed-in session itself a full enrollment is the bridge
  design's D5 and not this slice.
- D4. **The launch is a long act with a record.** The server starts the
  verb detached, the way `ui start` starts its server, and the verb writes
  `artifacts/agents/ui/launches/<nickname>.json` after every step: the
  step, its outcome, the owner's words, the time. The page reads that
  record through the payload and re-reads on the `fleet` event, which the
  server now also emits when a launch record changes. No timer.
- D5. **One launch at a time on this host, and only signed in.** A second
  launch while one runs is refused with the running one's nickname. The act
  requires the signed-in session whether or not a Partner is configured,
  because it spends disk, a build and the human's credentials.
- D6. **Joining is the slice; running is the next.** A launched machine is
  reachable and idle. Starting its session main, in a tmux window the human
  can attach to or headless under the unattended machinery once that
  lands, is g1-s44, and the launch card says how to start one by hand
  meanwhile.

## 3. The verb: `metasystem seat launch`

```
metasystem seat launch --machine <nickname> [--from <this checkout>] \
  --temporary-human-word '<the human's words>' --review-by <YYYY-MM-DD> \
  [--destination <dir>] [--record <path>] [--json]
```

Refusals before any step, by name: `SEAT_LAUNCH_NICKNAME_INVALID` (the
presence charset `[A-Za-z0-9._-]+`, and not this checkout's own nickname);
`SEAT_LAUNCH_NICKNAME_TAKEN` when a sibling clone already carries it or a
presence ref or a claim at the accepted tip names it; `SEAT_LAUNCH_DESTINATION_EXISTS`;
`SEAT_LAUNCH_RUNNING` when a launch record on this host is not terminal;
the word pair's own refusal. The steps, each a record entry with
`outcome: pending | done | skipped | failed` and the owner's words:

| step | precondition | command, from the launching seat | refusal |
|---|---|---|---|
| 1 clone | destination absent | `git clone --quiet <this checkout> <dest>`; `git -C <dest> remote set-url origin <origin url>`; `git -C <dest> config metasystem.steward.landing-ref refs/remotes/origin/main`; every `goal.human.*` key copied | git's words |
| 2 engine | `<dest>/metasystem/bin/metasystem` absent or its stamp is not `<dest>`'s HEAD | `<dest>/metasystem/scripts/agents/go-build.sh` | the build's or the gate fence's words |
| 3 configuration | `<dest>/metasystem/metasystem.conf.local` absent | copy this seat's `metasystem.conf.local`, then rewrite `evidence.root` to `<this seat's evidence root>/../<nickname>` (read through `config get`, never by the verb's own parsing), create that directory; then `validate session-isolation` with the adapters' manifest for the local runtime files; then `<dest>/metasystem/bin/metasystem config validate --repo <dest>` | the validator's words |
| 4 nickname | `metasystem.goal.machine` unset in `<dest>` | `git -C <dest> config metasystem.goal.machine <nickname>` | none |
| 5 ledger | always | `<dest>/metasystem/bin/metasystem goal fetch --root <dest>`, then `goal next --root <dest>` for the one orientation line the record keeps | the fetch's words |
| 6 enrollment | no identity at `<dest>` | `<dest>/metasystem/bin/metasystem steward arm --repo <dest>/metasystem --temporary-human-word '<word>' --review-by <date>` | the arm's words |
| 7 supervision | the steward runner not alive there | `<dest>/metasystem/bin/metasystem up --repo <dest>/metasystem` | up's words |
| 8 presence | after 7 | wait at most three ticks for `refs/metasystem/presence/<nickname>` (or the branch rung) to appear in this seat's presence copy, reading the copy on each `fleet` event; done when it does, failed with `no presence within three ticks; read <dest>'s health` otherwise | |

Every step runs under a bound (the ledger fetch's budget for git and
network steps, ten minutes for the build) on an owned process group, and
the verb's own exit is the last step's outcome. `--json` prints the record.
The verb runs from a terminal as well as from the interface, and a human at
the terminal may omit the word pair, in which case step 6 uses their
enrolled terminal like `steward arm` does today.

The record, `artifacts/agents/ui/launches/<nickname>.json`:

```
{ "schemaVersion": 1, "machine": "m1f", "destination": "/…/agentic-tools-m1f",
  "startedAt": "...", "endedAt": "..." | null, "outcome": "running" | "done" | "failed",
  "reviewBy": "2026-10-02", "steps": [ { "step": "clone", "outcome": "done", "at": "...", "words": "" }, ... ],
  "orientation": "goal next's one line", "next": "cd /…/agentic-tools-m1f && claude" }
```

## 4. The act and the page

`POST /api/fleet/launch` with `{machine, word, reviewBy}`: refuses without
a signed-in session (`SIGN_IN_REQUIRED`, the act layer's existing words),
validates the nickname and the pair with the same rules as the verb, refuses
a running launch, then spawns `bin/metasystem seat launch` detached under
the server's ownership gate with the record path, and answers 202 with the
record's first shape. `GET /api/fleet` gains `launches`: every record on
this host, newest first, so a page opened later still sees a running or a
failed launch.

On the Fleet page, in the fleet block's header, one button: **Launch a
machine**. It opens a work-area sheet (the g1-s35 sheet, so the Partner
stays usable) with:

1. **Nickname**, proposed as the next free letter of this host's series
   (`m1f` after `m1e`), editable, validated live against the charset and
   against the machines the page already shows.
2. **Where**: the destination path, shown and not editable, "beside this
   checkout".
3. **What it gets**: one sentence naming what is copied, "this seat's
   roster and local configuration, its runtime settings, and the fleet's
   ledger", and one naming what it will not do, "it starts no session; it
   joins, publishes presence and waits".
4. **Your authorization**: a text field for the word, in the human's own
   words, with the help term explaining that the machine runs under this
   word until the review-by date and a human re-approves it at a terminal
   afterwards; and the review-by date, defaulting to seven days out,
   editable.
5. **Launch**, disabled until the nickname and the word validate.

On Launch the sheet becomes the launch card, which also lives in the fleet
block above the table while a launch is running or failed: the nickname,
the destination, the steps as a vertical list each with its outcome and the
owner's words under a failed one, the orientation line when the ledger
step is done, and at the end either the machine's row appearing in the
table (the card then folds to one line, "m1f joined 2 min ago, enrolled
until 2 October", with the `next` command to start a session by hand), or
the failed step with **Retry**, which runs the same verb again and skips
what already holds. No Remove: a failed clone is a directory the human
deletes, and the card says its path.

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
admission cap already bounds what several stewards may run at once. The
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

Go: unit tests on the verb's sequencer with a fake runner: every step's
command and precondition, stop at the first refusal with the owner's words
verbatim, idempotent re-run skipping what holds, the record after each
step, the refusals by name, the one-at-a-time rule; one narrow integration
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
