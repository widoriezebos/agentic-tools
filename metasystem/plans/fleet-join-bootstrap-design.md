# Design: a fresh session boots itself into the fleet

Goal: fleet-join-bootstrap (plans/goals/fleet-join-bootstrap.md, revision 4).
Design revision 1, 2026-09-02, Fable-lane designer, job fleet-join-design-r1b.
Wido's open question, verbatim from the dispatching seat's record: "whether a
fresh session can boot itself into the fleet unaided". Three machines answered
no on 2026-09-02: m1b's fresh host clone, and the m0 and m0b guest clones
before hand-fixing.

Evidence base: the goal record's Intent, and the seams read in this worktree
on 2026-09-02. Every claim about the tree below cites file and line as they
stand in this worktree on that date. Where the Intent's diagnosis and the tree
disagree, the tree wins and the disagreement is named.

Out of scope, by the brief: the harness hook's wrong-root defect (goal
supervision-hook-wrong-root), the steward identity drift (goal
vm-epoch-identity-drift), and account provenance (goal account-provenance).
This design touches none of them and names where it borders them.

## 0. What a fresh clone has today, traced

A fresh clone of the repository, before anyone touches it:

- **No engine.** `bin/` is gitignored (metasystem/.gitignore line 2). The one
  fenced build is scripts/agents/go-build.sh; its only refusal that a newcomer
  can hit is "no go toolchain on PATH; the engine cannot be built"
  (go-build.sh:29), which names no version and no next command. Nothing in
  the tree tells a newcomer to run it: the session-start hook exits benign
  when the engine is missing (supervision-hook.sh:25-31) and only the stop
  event prints "reinstall or rebuild bin/metasystem" (line 28), without the
  script's name.
- **No roster.** metasystem.conf.local is gitignored (.gitignore line 1). The
  committed metasystem.conf carries placeholders for every model lane and for
  the evidence root: `evidence.root=<durable evidence root, outside the
  repository>`, `role.default.model.codex=<model>`, `.claude=<model>`,
  `.devin=<model>`, `role.design-critic.model.codex=<model>`,
  `role.implementer.model.codex=<model>`, `role.code-critic.runtime=<runtime>`,
  and `role.code-critic.model.<runtime>=<model>` (metasystem.conf, roster
  block). Resolution reads the `.local` overlay first for base keys
  (resolve.go:87-95) and for mode-scoped role keys (resolve.go:72-86), so on
  a seat every one of those lines is meant to be shadowed by `.local`. The
  design-mode lane R-25 requires (a design brief dispatches the implementer
  role on Claude) is carried only as `mode.design.role.implementer.runtime`
  and `mode.design.role.implementer.model.claude` in m1's `.local`; the
  committed file has only commented examples of the mode shape.
- **No machine identity.** The machine nickname is git configuration, not
  conf: `metasystem.goal.machine` (actor.go:21-27). Its refusal already names
  the exact command. `goal next` (goal.go:449-453), every claim
  (goalsync_mutations.go:34), dispatch admission (dispatch/admission.go:67),
  the steward tick, and the open-work read resolve it.
- **No notification channel on Linux.** `steward arm` refuses when
  `metasystem.steward.notify-command` (git configuration) is unset and the
  platform is not darwin (runner.go:387, notify.go:25-36). The refusal names
  the key but not the `git config` form. The two guest clones are Debian, so
  this is on their path.
- **No accepted ledger tree.** The clone-local pointer
  `refs/metasystem/goals/accepted` (txn.go:39) does not exist until something
  creates it. `goal list` and `goal next` on a converted checkout (the
  checkout carries plans/goals/backlog.md and not plans/goals.md,
  goal.go:323-329) go to the projection, which refuses with "no accepted
  tree; the first fetch or the migration bootstraps it" (project.go:37-39).
  That message names no command.
- **No enrollment.** `metasystem up` opens the enrolled engine first
  (up.go:418-421). With no artifacts/agents/steward/identity.json,
  `OpenEnrolledBinary` wraps the read error in `ENROLLMENT_DRIFT`
  (identity.go:242-247), and up prints `component=accepted-engine
  outcome=ENROLLMENT_DRIFT` with the remedy "from the enrolled agent-free
  terminal, explicitly run metasystem steward arm or steward restart for
  this repository" (up.go:391-397). The same outcome and remedy meet an
  existing seat after a pull that changed the engine, because the digest
  check at identity.go:284-287 fails. R-37-m3 (memory/rulings.md line 64)
  authorizes the relayed-word re-arm on every machine after an engine
  rebuild; the verb exists (`steward arm --temporary-human-word ...
  --review-by ...`, steward_verbs.go:490-514) and the remedy does not name it.
- **No documentation.** AGENTS.md, wow.md, docs/collaboration.md and
  docs/working-with-agents.md contain no join guidance (searched for join,
  fleet, clone, enroll: no hits). docs/backlog-mechanism.md documents pinning
  to a machine nickname (lines 104-116) and nothing about obtaining one.

### The Intent's ledger diagnosis does not match the tree

The Intent says the ledger lives under `refs/metasystem/*` on origin, that
`refs/metasystem/machines/<m>/accepted` is published there, and that the
`+refs/heads/*` fetch refspec is why a fresh clone has no ledger. The tree
says otherwise:

- The canonical ledger is a **branch**: `goal.sync-branch` defaults to
  `refs/heads/main` on the remote `goal.sync-remote` defaults to `origin`
  (txn.go:49-61). Every operation fetches that branch with `--refmap=` into a
  per-operation ref `refs/metasystem/goals/fetch/<opid>` (txn.go:66,
  126-127). The ordinary clone refspec already brings `refs/heads/main`.
- `refs/metasystem/goals/accepted` is **clone-local**. It is created and
  advanced by `AdvanceAccepted` (txn.go:392-396 and the forward loop after
  it): when the ref is absent, the first pass creates it with the empty
  old-value compare (txn.go:409-413). Nothing pushes it; nothing fetches it.
- There is no `refs/metasystem/machines/` anywhere in the tree. A search of
  every Go file, shell script, and document for `machines/` finds only the
  goal record and the brief. The only other `refs/metasystem/` namespaces are
  the mission anchors (mission/anchor.go:158, local, never pushed) and the
  reconcile base anchor (reconcile.go:239, local).
- `goal fetch --root <checkout>` (goalsync_verbs.go:388-412) runs
  `FetchAdvance` (fetchadvance.go:30-82). On a never-fetched clone:
  `CaptureTip` fetches main (txn.go:103-135); `SyncModeGate` reads the tip's
  root record (txn.go:195-221); the accepted ref is absent, so the acceptance
  gates are skipped (fetchadvance.go:58-62); `ValidateCommit` validates the
  whole tree (fetchadvance.go:65); `AdvanceAccepted` creates the ref. The
  first fetch does bootstrap, exactly as project.go:39 promises. What the
  three machines lacked was the name of that command.

So item 3 of the brief resolves to: no refspec is added anywhere. The join
step is `goal fetch`, and the two messages get the command's name. The
self-grade names this as the weakest claim, with its reject condition.

## 1. The join sequence and its owner

### Decision: one script composes; every decision stays in its Go owner

The join sequence is owned by a new script, `scripts/agents/join-fleet.sh`.
It is plumbing in the sense docs/architecture.md gives the word: shell calls
verbs, verbs never call scripts (architecture.md "Layering", the paragraph
after tier 3), and the script decides nothing. Every refusal on the path is
the refusal of the existing Go owner it calls, corrected in that owner so the
refusal names the real next command whether or not the script is used.

The two alternatives the brief names were weighed against the existing-owner
rule (docs/project-adaptation.md:42 "Does an existing owner already cover
it?"; skills/take-a-step-back/SKILL.md:18-21 names the existing owner before
adding a layer) and refused:

- **An engine verb (`metasystem join`).** The first step builds the engine.
  No engine verb can run before the engine exists, so a verb would still need
  a script in front of it and would be a second surface for the same
  sequence. The engine already owns every decision the sequence needs
  (config validate, goal fetch, steward arm, up); the composition is the only
  new thing, and composition before the binary exists is shell's tier.
- **An up mode (`up --join`).** Up deliberately only consults the standing
  enrollment (up.go:418-421 and the recovery path at 508-515;
  docs/orchestration.md's arming paragraph: "Ordinary, advisor, and
  recovery-only up only consult that standing enrollment"). Enrollment is a
  human-authority act with its own verb and its own ancestry proof
  (steward_verbs.go:561-577). Folding enrollment into up would breach the
  agent-free-terminal law that steward_verbs.go:508-514 records as having
  exactly one exception. Up also cannot build the binary it is.

The precedents for a composing script are the two existing first-use owners:
scripts/adopt.sh (tailors the conf at line 315, builds the engine at 364,
prints the remaining manual steps at 469-472) and
scripts/agents/second-session.sh (creates a worktree, copies local config,
arms through the engine). join-fleet.sh is the third member of that family:
adopt.sh sets up an adopted target, second-session.sh sets up a second
session on one machine, join-fleet.sh sets up a new machine on the
metasystem's own fleet.

### The sequence

Every step names its precondition, the exact refusal when it is missing (the
owner's text after the corrections in section 4), and the exact next command
that refusal names. The script runs the steps in this order and stops at the
first refusal, printing the owner's refusal verbatim and nothing else. It is
idempotent: an existing seat re-runs it after a pull and only the steps whose
precondition is now missing act.

Invocation:

```
scripts/agents/join-fleet.sh --machine <nickname> \
  [--evidence-root <absolute path outside the repository>] \
  [--notify-command '<command>'] \
  [--temporary-human-word '<verbatim word>' --review-by <YYYY-MM-DD>]
```

`--machine` is required on the first run only (a set nickname is never
overwritten; a different nickname on a later run refuses, because claims key
on it, actor.go:3-8). The other flags are optional and only fill what is
missing.

**Step 0. Host preconditions.** `git` and `go` on PATH, and a remote named by
`goal.sync-remote` (default `origin`, txn.go:50) that answers `git remote
get-url`.
Refusal when `go` is missing: go-build.sh:29, corrected to name the version
from go.mod (section 4). Refusal when the remote is missing: the script's
own line, `join-fleet: no remote named origin; add it with  git remote add
origin <url>  or set  git config goal.sync-remote <name>`. This is the one
message the script owns, because no Go owner runs before the fetch.

**Step 1. Build the engine.** `scripts/agents/go-build.sh`. Precondition:
none beyond step 0. The build is skipped when `bin/metasystem` exists and its
stamp equals `git rev-parse --short HEAD` (the stamp go-build.sh:44 embeds;
read back through `metasystem version` or the equivalent stamp reader the
implementer finds in internal/supervise, where `BuildStamp` lives). A live
gate refusal (go-build.sh:34-38) is printed verbatim and stops the join.

**Step 2. Lay down the roster.** If metasystem.conf.local is absent, copy
metasystem/metasystem.conf.local.template (section 2) to it, then substitute
only the flags given (`--evidence-root` replaces the evidence.root
placeholder). Then run:

```
bin/metasystem config validate --resolved --conf metasystem.conf --repo .
```

Refusal: one line per unresolved placeholder or missing required key, in the
form `invalid metasystem configuration: <key> is still the placeholder
<value>; fill it in metasystem.conf.local (template line N)` (section 2 gives
the exact rule). The next command the refusal names is the file to edit; the
script prints `join-fleet: fill the named keys in metasystem.conf.local and
rerun scripts/agents/join-fleet.sh` after the validator's lines.

**Step 3. Enroll the machine identity.** When `--machine` is given and
`git config --get metasystem.goal.machine` is empty, the script sets it.
When the key is still empty afterwards, the script runs `bin/metasystem goal
next --root .` and exits with its status: `goal next` resolves the machine
before it reads any ledger (goal.go:449-453), so the engine prints
actor.go:27's refusal, which already names `git config
metasystem.goal.machine <nickname>`. The script owns no copy of that text.
On a non-darwin host, also `git config metasystem.steward.notify-command
'<command>'` from `--notify-command` when the key is unset; when neither is
present, the refusal is runner.go:387's corrected text (section 4), reached
by letting step 5 run and refuse. The script does not pre-check it, because
the darwin default (notify.go:32-34) is the owner's knowledge, not the
script's.

**Step 4. Bootstrap the ledger.** `bin/metasystem goal fetch --root .`.
Precondition: step 0's remote and the machine name (the fetch itself needs
no nickname; the first `goal next` after it does). Refusals are the fetch
advance's own, unchanged: a git fetch failure (txn.go:126-129, git's text),
a sync-mode mismatch (txn.go:213-218), a validation failure naming file and
rule (validate.go:410), or a rewind on a re-run (fetchadvance.go:101,
corrected in section 4 to name the whole repair command). Success prints
`advanced=true tip=<oid> accepted <short>` (goalsync_verbs.go:410); the
script then runs `bin/metasystem goal next --root .` and prints its one
orientation line as the proof that this machine now reads the fleet's world.

**Step 5. Enroll the engine.** Three cases, decided by the engine:

1. `--temporary-human-word` given: `bin/metasystem steward arm --repo .
   --temporary-human-word '<word>' --review-by <date>`. The verb validates
   the pair (humanauthority.go:143-157), announces the temporary state
   (steward_verbs.go:513) and mints the identity with the word on it
   (runner.go:417-420). This is the R-37-m3 path every seat uses today; the
   word is the human's, typed by the operator, never manufactured by the
   script. The script has no default word.
2. No word, caller classified human: `bin/metasystem steward arm --repo .`
   succeeds through the ancestry proof (steward_verbs.go:561-577).
3. No word, caller classified as an agent: the verb refuses with
   steward_verbs.go:573's corrected text (section 4), which names both
   forms. This is the honest documented stop. The script exits 1 with the
   engine's refusal and prints nothing else; the fixture asserts this exact
   stop.

An existing seat whose engine changed lands here with the same three cases;
`arm` replaces the live runner and mints the next generation
(runner.go:403-407, 417-420).

**Step 6. Arm the session.** `bin/metasystem up --repo .`. Precondition: step
5. The join script passes no identity flags: from a runtime session the
signature ancestry proves the session (up.go:174-184); from a human shell
the explicit pair is required and the refusal at up.go:430-431 already names
it. Outcome `armed` or `advisor` (up.go:493-499) is green; `advisor` names
the holder and second-session.sh, which is correct for a second session on
the same checkout. `ENROLLMENT_DRIFT` here means step 5 was skipped or the
binary changed between the two steps; its corrected remedy (section 4) names
step 5's commands.

**Step 7. Verify.** `bin/metasystem steward health --repo .` and the `goal
next` line from step 4. Both are read-only and are the newcomer's proof.

### Documentation

One section, "Joining the fleet", in docs/backlog-mechanism.md immediately
before "Pinning a goal to a machine" (line 104), because that document
already owns the multi-machine ledger's operator prose and the nickname it
never explained. It lists the seven steps as the command lines above, names
join-fleet.sh as the one-shot path, and names the honest stop at step 5.
wow.md gains one pointer line to that section. AGENTS.md is unchanged: the
brief's rule is one documented path, and two would drift.

## 2. The roster template

File: `metasystem/metasystem.conf.local.template`, committed. The name does
not match the gitignore line for `metasystem/metasystem.conf.local`
(.gitignore line 1 is an exact path), so it ships. The template is copied,
never tailored: `config tailor` (config_verbs.go:25-88) rewrites a committed
conf for a runtime set and is adopt.sh's tool (adopt.sh:315); a seat on the
metasystem's own fleet keeps the committed runtime set and only overlays
values, which is what `.local` is for (resolve.go:23-27).

Contents, in this order, each with the comment that says whether it is hand
set or derived:

```
# metasystem.conf.local: this machine's roster. Copied from
# metasystem.conf.local.template by scripts/agents/join-fleet.sh.
# Hand-set keys carry <angle-bracket> placeholders; the join refuses
# while any remains. Not committed.

# HAND SET: the durable evidence root, absolute, outside the repository.
evidence.root=<absolute path outside the repository>

# HAND SET: the three model lanes, one per runtime in metasystem.runtimes.
role.default.model.codex=<codex model>
role.default.model.claude=<claude model>
role.default.model.devin=<devin model>

# DERIVED from the lanes above unless this machine differs: the two
# codex roles the committed file leaves as placeholders.
role.implementer.model.codex=<codex model>
role.design-critic.model.codex=<codex model>

# HAND SET: the code critic runs on a different effective model from the
# implementer (docs/orchestration.md, loop step 4).
role.code-critic.runtime=claude
role.code-critic.model.claude=<claude model>

# HAND SET: the design-mode lane (R-25: Fable designs). A design brief
# dispatches the implementer role on this runtime and model.
mode.design.role.implementer.runtime=claude
mode.design.role.implementer.model.claude=<claude model>

# OPTIONAL: cost ranks. Absent tiers mean dispatch overrides always
# escalate (config validate prints the INFO line).
# model.tier.1=devin:<devin model>
# model.tier.2=codex:<cheaper codex model>
# model.tier.3=codex:<codex model>
```

The minimum a seat sets by hand is therefore: the machine nickname (git
configuration, step 3, not in this file), the evidence root, the three lane
models, the code-critic model, and the design-mode model. The Linux
notification command is git configuration (step 3). Everything else is
derived or optional.

### The check: `config validate --resolved`

Today `config validate` reads the required keys from the committed file only:
`values` is filled from the committed content (validate.go:34-60), the
`.local` loop keeps only `cap.min.` keys (validate.go:68-93, comment at
63-66: "other keys are the developer's own and not template invariants"),
and evidence.root is read from `values` (validate.go:443). So on the
metasystem's own checkout the validator cannot see the roster at all, and
the committed placeholder key `role.code-critic.model.<runtime>` fails the
key pattern (validate.go:49-51) before any roster check runs. That is
correct for the template's adopters and useless for a seat.

The join therefore adds one flag to the existing owner: `config validate
--resolved`. Behaviour, mechanical:

1. Parse the committed file with placeholder tolerance: a line whose key or
   value contains a `<...>` token is recorded as a placeholder instead of an
   error. Exactly one committed key carries the token in the key,
   `role.code-critic.model.<runtime>`; its concrete key is derived as
   `role.code-critic.model.` joined to the resolved value of
   `role.code-critic.runtime`. The implementer does not generalize this
   derivation.
2. Overlay every `.local` key onto `values` (last writer wins, matching
   resolve.go:87-95's precedence for base keys and 72-86 for mode-scoped
   role keys).
3. Run the existing checks unchanged over the overlaid `values`: runtime
   roster (validate.go:103-104), role and mode model coherence
   (validate.go:299-325), evidence root absolute and outside the repository
   (validate.go:441-452), and the numeric knobs.
4. Add one check: every placeholder still present after the overlay is
   refused as `<key> is still the placeholder <value>; fill it in
   metasystem.conf.local (template line N)`, where N is the line of that
   key in metasystem.conf.local.template. If the audit's placeholder
   predicate already exists as a Go function (scripts/audit-metasystem.sh:14
   passes `--allow-placeholders` to an engine verb), reuse it; otherwise the
   predicate is "contains `<` followed later by `>`".
5. Without `--resolved`, behaviour is byte-identical to today.

The design-mode keys are not made mandatory by the validator; they are
mandatory by the template, which the join copies whole, and step 4's
placeholder check refuses them while unfilled. A seat that deliberately
deletes them has made a roster decision the validator does not second-guess.

## 3. The ledger refspec

No refspec is added, at clone time, at first goal verb, or in the join step,
because the ledger is the `refs/heads/main` branch and the accepted pointer
is clone-local (section 0). Adding `+refs/metasystem/*:refs/metasystem/*`
would be harmful: it would fetch another clone's mission anchors and
per-operation fetch refs into namespaces the engine treats as its own
(gittree.go:466, txn.go:66-67), and the accepted ref of one machine would
overwrite another's on fetch.

On a machine that has never fetched, the join step is `goal fetch --root .`
(step 4), whose first pass creates the accepted ref (txn.go:409-413). The
message at project.go:39 becomes:

```
no accepted tree on this clone; run  metasystem goal fetch --root <checkout>  once: the first fetch validates the canonical branch and creates the accepted ref (a checkout that still carries plans/goals.md needs goal migrate instead)
```

with `<checkout>` substituted by `e.Root`. The staleness banner at
project.go:73 becomes:

```
the accepted tree is %s old; metasystem goal fetch --root <checkout> validates and advances it
```

`goal list` has no `--fetch` flag (goal.go:270-274 defines root, pretty,
label) and passes `fetchFirst=false` (goal.go:339); the fetching read is
`goal fetch` (goalsync_verbs.go:388-412). The `Project` comment at
project.go:28-30 that calls the parameter "the --fetch flag" is corrected to
name `goal fetch`.

## 4. The remedy texts

Every message on the join path that names a nonexistent or incomplete
command, with its corrected text. Lines are this worktree's on 2026-09-02.

| File and line | Today | Corrected |
| --- | --- | --- |
| internal/goal/project.go:39 | `no accepted tree; the first fetch or the migration bootstraps it` | the text in section 3 |
| internal/goal/project.go:73 | `... goal list --fetch validates and advances it` | the text in section 3 |
| internal/goal/project.go:28-30 (comment) | "(the --fetch flag)" | "(goal fetch)" |
| internal/up/up.go:392 | `from the enrolled agent-free terminal, explicitly run metasystem steward arm or steward restart for this repository` | `enroll this engine: from an agent-free terminal run  metasystem steward arm --repo <root>  (steward restart when a runner is live); when the human is away, relay their recorded word:  metasystem steward arm --repo <root> --temporary-human-word '<verbatim word>' --review-by <YYYY-MM-DD>` with `<root>` substituted by options.Root |
| cmd/metasystem/steward_verbs.go:573 | `%s: explicit engine enrollment requires an agent-free terminal; caller classified %s` | append `; relay the human's recorded word instead with  --temporary-human-word '<verbatim word>' --review-by <YYYY-MM-DD>` |
| internal/steward/runner.go:221 and :387 | `no notification channel is configured; an unreachable watchdog guards nothing — set metasystem.steward.notify-command` | `... — run  git config metasystem.steward.notify-command '<command that delivers one message>'  (macOS falls back to the platform notifier)`; the implementer reads notify.go past line 50 for the exact message-passing contract before naming it |
| internal/up/up.go:482-483 | `configure a working notification channel, inspect artifacts/agents/steward/runner.log, then rerun metasystem up` | `set  git config metasystem.steward.notify-command '<command>'  (macOS has a default), inspect artifacts/agents/steward/runner.log, then rerun metasystem up` |
| internal/goal/fetchadvance.go:101 | `... repair --accept-remote is the deliberate path` | `... metasystem goal repair --accept-remote --by <human> --root <checkout>  is the deliberate path` (goalsync_verbs.go:419 is the usage line). Goal repair-accept-remote-verb is named as related in the goal record; if that goal already carries this line, slice 2 leaves it there |
| scripts/agents/go-build.sh:29 | `go-build: no go toolchain on PATH; the engine cannot be built` | `go-build: no go toolchain on PATH; install Go <version from go.mod's go line> and rerun scripts/agents/go-build.sh` |
| scripts/agents/supervision-hook.sh:28 | `metasystem engine missing; reinstall or rebuild bin/metasystem` | `metasystem engine missing; run scripts/agents/go-build.sh` |

Messages on the path that are already correct and stay: actor.go:27 (names
`git config metasystem.goal.machine <nickname>`), up.go:431 (names the
explicit pid pair), up.go:468 (names second-session.sh),
ledgerattention.go:644 (says `goal fetch` does not examine, which is true),
steward_verbs.go:513 and goalsync_mutations.go:415 and :647 (the temporary
announcements). The searched surfaces were internal/goal, internal/up,
internal/steward, and cmd/metasystem for the strings `no accepted tree`,
`--fetch`, `agent-free`, `goal list`, `goal fetch`, `first fetch`,
`the migration`, `ENROLLMENT_DRIFT`, `notify-command`, and `repair
--accept-remote`.

The ruling identifier R-37-m3 is not written into any message. The
source-comment rule (memory: never round, slice, or finding references in
code) covers message strings too; the message names the mechanism, this
design names the authority.

## 5. The fixture

File: `scripts/agents/fleet-join-fixtures.sh`, registered in
scripts/validate-metasystem.sh in the section list beside
second-session-fixtures.sh (line 984), the `bash -n` line beside 1063, and a
`run_section fleet-join-fixtures needs-engine bash
scripts/agents/fleet-join-fixtures.sh` block in the shape of lines 1161-1163.
Shape: second-session-fixtures.sh's bed-scenario harness
(fixture-bed-scenarios.sh, `run_fixture_bed_scenarios`) with
fixture-budget.sh's scaled ceilings (`harness_fixture_cap`,
fixture-budget.sh:341) on every wait, and adopt-fixtures.sh's failure
preservation (adopt-fixtures.sh:62-73) for the temporary tree.

Bed: a bare remote made with `git clone --bare` from the current checkout's
HEAD, then a fresh `git clone` of it into a temporary directory. The fresh
clone's metasystem.conf gets `metasystem.runtimes=fake` appended so the
explicit pid pair is fixture-authorized (up_test.go:342, fixtureauth's
package doc). Evidence root is a temporary directory outside the clone.
Notification command is `true`.

Scenarios, each one bed:

1. **green-join.** `join-fleet.sh --machine fixture-m9 --evidence-root
   <tmp> --notify-command true --temporary-human-word 'fixture word'
   --review-by <today plus one day>` followed by the engine's `up --repo .
   --pid $$ --start-time <started-at>`. Asserts: `bin/metasystem` exists
   and runs; `metasystem.conf.local` exists with no `<` in any value;
   `git config metasystem.goal.machine` prints `fixture-m9`;
   `refs/metasystem/goals/accepted` resolves and equals the bare remote's
   main tip; `artifacts/agents/steward/identity.json` carries
   `temporaryHumanWord` and `reviewBy`; `goal next --root .` exits 0; up
   prints `up outcome=armed`. Then `up --shutdown` and `steward disarm`
   for cleanup.
2. **no-machine.** Same without `--machine`: exits 1 and stderr contains
   actor.go:27's string, printed by the engine's `goal next` (goal.go:449-453)
   that the script execs; a direct `metasystem goal next --root .` on the
   same bed prints the same string.
3. **placeholder-roster.** Same with `--evidence-root` omitted: exits 1;
   stderr contains `evidence.root is still the placeholder`; no ledger
   fetch ran (the accepted ref is absent).
4. **ledger-before-fetch.** On a fresh clone with the engine built,
   `goal list --root .` exits 1 with the corrected project.go:39 text.
5. **honest-stop.** Same as 1 without the word pair: exits 1; stderr
   contains the corrected steward_verbs.go:573 text; no identity.json
   exists; no runner process was launched.
6. **rejoin-after-rebuild.** After scenario 1's join, rebuild with
   `METASYSTEM_BUILD_STAMP=changed scripts/agents/go-build.sh` (go-build.sh:44
   embeds the stamp, so the digest changes); `up` prints
   `outcome=ENROLLMENT_DRIFT` with the corrected up.go:392 remedy; re-running
   join with the word pair re-arms and `up` prints `outcome=armed` with
   generation 2.

What the fixture cannot assert in a delegate sandbox (KI-15: the suite needs
real process visibility, and every delegate sandbox denies it): scenarios 1
and 6's `up` leg and the runner launch in step 5, because they spawn detached
processes and read process identity. The orchestrator runs the suite outside
the sandbox. Scenarios 2, 3, 4 and 5 run anywhere, because they stop before
any process is launched. The fixture also cannot assert the human-terminal
form of step 5 (the caller is never classified human under the suite) or a
network fetch from GitHub (the bare remote stands in; the fetch code path is
the same, txn.go:126-127).

## 6. Slices

**Reservation accounting.** Reserved job minutes charge the dispatch cap,
not the runtime: `projection.ReservedJobMinutes += capMinutes`
(dispatch/budget.go:366-369), and admission refuses a launch whose cap does
not fit the remainder (dispatch/admission.go:156-160). The default cap is
120 (`dispatch.cap-min=120`, metasystem.conf). A 240-minute box therefore
holds two launches at the default cap, or four at `--cap 60`.

**Recorded precedent** (artifacts/agents/jobs on m1, wall time from
startedAt to endedAt):

| Job | Role | Wall |
| --- | --- | --- |
| recovery-analysis-r1b (design lane) | implementer | 11m35s |
| never-idle-analysis-r1 (design lane) | implementer | 16m43s |
| breach-design-r2 (design lane) | implementer | 18m44s |
| counselor-drift-s1, r2, r3 (build) | implementer | 8m, 23m, 13m |
| counselor-carriage, r2 (build) | implementer | 5m, 27m |
| counselor-drift-crit, crit2, crit3 | code-critic | 8m, 7m, 7m |
| counselor-carriage-crit | code-critic | 10m |

No recorded design-critic round completed on this machine (breach-design-crit2
and crit2b failed at launch in 12 seconds each), so the critique wall estimate
below is unsupported by a local record and borrows the code-critic times.

**Slice 1: design closure.** This job (one launch, cap 120) plus one Sol
design-critique round (one launch, cap 120): 240 reserved minutes exactly,
about 35 minutes of wall time on the precedent. A second critique round or a
revision round does not fit the box at the default cap; the seat launches it
at `--cap 60` or asks for a raise, and says which.

**Slice 2: build.** One implementer launch for the whole change (the
template, the `--resolved` flag, the script, nine message edits, the
documentation section, the fixture and its registration), then one code
critique: 240 reserved minutes at the default cap. Wall estimate on the
precedent: build 25 to 40 minutes, critique 8 to 15 minutes. A correction
round is likely (every build chain above needed one) and does not fit at
cap 120; the seat plans the correction at `--cap 60` from the start, which
makes the box three launches. The estimate for the build's wall time is
grounded; the estimate that one correction suffices is not, because the
recorded chains needed one to two.

## 7. Self-grade

Confidence: 0.8 that the sequence, the owner decision, the template, and the
message corrections are right and complete for the three machines' walls.

Weakest claim: that no `refs/metasystem/*` refspec is needed and that the
three machines' hand-fix was, in substance, `goal fetch`. The tree supports
the first half completely (section 0). The second half rests on absence: no
record under records/ or plans/ describes what m0 and m0b hand-fixed, and I
found no push of any `refs/metasystem/*` ref anywhere. Reject condition: a
seat's record showing that a fetch of `refs/metasystem/*` was necessary for
anything on the join path, or `goal fetch` on a fresh clone failing for a
reason section 0 did not trace (for example `ValidateCommit` needing state
only an enrolled machine has). Either sends section 3 back to design.

Second weakest: the placeholder-key derivation in `config validate
--resolved` (section 2, rule 1). It is mechanical and covers the one
committed key that needs it, but it is a new rule inside a validator that
deliberately ignored `.local`. Reject condition: the Sol critique finds a
second committed key with a placeholder token in the key, or a `.local`
precedence case where the overlay in rule 2 disagrees with resolve.go.
