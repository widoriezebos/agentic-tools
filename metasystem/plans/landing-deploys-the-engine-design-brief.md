# Design brief: landing-deploys-the-engine

## Revision

Revision: first draft

Reason: goal `landing-deploys-the-engine` is approved (tier 2) and its stated
next step is one design page, critiqued in at most two rounds, then one builder
and one real run. No design record exists for the goal yet.

## Context pack

Read this pack first. Open another file only to check one of the cited lines
below; do not widen the read beyond that check.

Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn.

All paths are relative to the installation, `metasystem/` in this repository.

### What the person asked for (Wido, binding; quoted from the goal record `plans/goals/landing-deploys-the-engine.md`)

- Intent, 2026-10-01: "after a successful landing push, the lane runs the
  project's deploy step on the pushed commit. For the metasystem itself: build
  the engine from that commit into ~/.metasystem/engines/<sha>/, atomically
  repoint ~/.metasystem/bin/metasystem (on PATH) to it, seats pick it up at
  their next safe point and their hooks call the central binary; deploy
  rollback repoints to the previous engine. Other apps supply their own deploy
  step (docker, kubernetes) at that extension point."
- Why, 2026-10-01: "per-checkout rebuild/restart caused today's stale engines,
  generation mismatches and refused proofs; one place with the newest proven
  engine removes that class."
- Known costs he named: "must replace or feed the per-checkout engine pins; a
  bad landing reaches every seat at once; skills/docs still come by pull."
- Control, 2026-10-01: "full control by verbs where applicable (deploy
  status/now/rollback/pause, which engine each seat runs) and all of it from
  the UI too (Fleet page: current engine, per-seat engine, deploy now,
  rollback, history); the design names each verb and its UI action."
- Other languages, 2026-10-01: "must fit other languages, e.g. Java/Maven: the
  lane only calls the project's deploy adapter (build artifact from the pushed
  commit, place/activate it, report the version, roll back) with
  language-neutral input/output; the metasystem's Go engine is just one
  adapter; a Maven adapter (mvn package/deploy of a jar, or a container image)
  is the design's worked second example."
- The goal names the design's core question: **engine pins versus the central
  binary**.

### Diagnosis: how it works today

Traced on 2026-10-02 at commit `b31a1e8e9`. Items marked (seat read) were
opened by the seat; the others come from a read-only trace and are claims to
check before the page depends on them.

The lane and its push:

1. (seat read) `internal/landing/plain/push.go:50-91`: `plain.Push` fetches
   origin's main, requires a green proof for HEAD's exact tree, refuses a
   non-fast-forward, pushes with a lease, sets `Changed`, and appends one
   `Pushed{old, commit, tree, at}` line to
   `artifacts/agents/landing/pushes.jsonl`. Nothing else happens inside it.
2. (seat read) `cmd/metasystem/intent_landing_push.go:46-61`: the
   `landing push` verb calls `plain.Push`; when `outcome.Changed` it posts the
   channel notice through `postLanded`, whose failure is reported and fails
   nothing. This is the only act after a push; there is no general after-push
   extension point.
3. Merging is done by the landing agent itself under
   `skills/landing-agent/SKILL.md` (about line 30), and proving runs the shell
   command in `landing.prove.command` (`cmd/metasystem/intent_landing_prove.go:25,93-135`).
4. A seat that lands its own work (no lane on this computer) pushes through a
   different path: `internal/landing/landpath/land.go:952` (`pushOrigin`), then
   `pushed` at `internal/landing/landpath/land.go:562`. The page must say
   whether deploy runs there too.
5. The lane record is `~/.metasystem/host/landing-lane.json`
   (`internal/landing/lane/lane.go:79,132,248`).

How a checkout gets its engine:

6. `cmd/devgate/build.go:53,134-141,159`: the build compiles to a temporary
   file and renames it over `<installation>/bin/metasystem`, stamped with the
   HEAD commit.
7. `internal/hooks/runtime_hook_start.go:786-817` and
   `cmd/metasystem/hook_entry.go:595,646-693`: at session start, an engine
   built from sources the checkout has moved past starts a detached rebuild
   (`go run -trimpath ./cmd/devgate build`) and the session ends unarmed with
   the `engine-rebuilding` notice. This is the per-checkout rebuild the goal
   wants gone.
8. `internal/steward/identity.go:58-97,223-258`: the enrollment record
   `artifacts/agents/steward/identity.json` holds
   `InstallIdentity{Generation, InstallPath, InstallDigest, EngineBuild,
   LandedCommit, LandingRef}`; the enrolled path must be canonical (symbolic
   links resolved), a regular executable, and match its sha256.
9. (seat read) `internal/up/up.go:618-640`: the invoking binary's canonical
   path must equal the enrolled `InstallPath`, or startup refuses with
   `ErrEnrollmentDrift`. A symbolic link at `~/.metasystem/bin/metasystem`
   resolves to `engines/<sha>/...`, so under today's rule every deploy would
   read as drift.
10. `internal/steward/runner.go:526-560`: `ReArmRebuiltEngine` mints a new
    generation only when the changed bytes' build stamp resolves to landed
    history. `internal/steward/disk_pins.go:30-47`: the engine pins are
    `artifacts/agents/steward/engine-pins/generation-N-<sha256>`.

How hooks find the engine:

11. (seat read) `internal/hooks/setup.go:331-353`: the rendered hook command
    sets `engine="$(pwd -P)/bin/metasystem"` (the primary checkout's for a
    linked worktree) and execs `${METASYSTEM_BIN:-$engine}` only when the
    installation's own engine exists.
12. Other readers that assume `<installation>/bin/metasystem`:
    `internal/hooks/setup.go:517`, `internal/hooks/runtime_hook_start.go:490-496,783`,
    `internal/missionrunner/host.go:362`, `internal/delegation/lifecycle.go:76`,
    `internal/adapter/supervisor/deps.go:103`, `cmd/metasystem/delegate.go:315`.

The host directory and existing contract patterns:

13. `internal/board/card.go:160-176` owns the `~/.metasystem` path (with a
    fixture override). It has `host/`, `launch/`, `stores/` and others; there
    is no `bin/` or `engines/` today, and no deploy verb, package or
    configuration key anywhere.
14. The launch contract is the nearest pattern for a project-supplied step:
    package `internal/applaunch` (`internal/applaunch/contract.go:1-20`), file `launch.json`
    selected by `launch.contract` in `metasystem.conf`
    (`internal/config/defaults.go:67`), verbs in `cmd/metasystem/intent_app.go:37-98`.
    External executables with JSON on standard input and output are the other
    pattern (`internal/runtimes/external/external.go:1-16`, `docs/agent-adapters.md`).

The Fleet page:

15. `GET /api/fleet` at `internal/ui/httpd/fleet.go:27-53`, composed by
    `internal/ui/fleet/fleet.go:313` from inputs built at
    `cmd/metasystem/ui_fleet.go:80`. Each seat's engine already travels on its
    presence record (`internal/seat/record.go:67`, `internal/ui/fleet/fleet.go:230,342`).
16. A state-changing example to copy: `POST /api/fleet/land-now` at
    `internal/ui/httpd/landnow.go:57-75`, which requires a signed-in session and
    then runs the verb (`cmd/metasystem/ui.go:489`, `cmd/metasystem/ui_landnow.go:28`). The
    React page is `internal/ui/web/_app/src/fleet/FleetPane.tsx` and
    `internal/ui/web/_app/src/fleet/LandingLane.tsx`.

A neighbouring accepted design:

17. `plans/designs/one-folder-deployed-and-evolved.md` (accepted, its goal held
    by another seat) keeps the engine at `metasystem/bin/metasystem` (about
    line 131) and defers a `system upgrade` verb (about line 198). The page
    must say how the central binary fits beside it and must not rewrite it; a
    real conflict is listed as a decision for Wido, not settled on the page.

### What the page must decide

1. **The core question.** Does the central binary replace the per-checkout
   enrollment and pins, or feed them? Answer it against facts 8 to 10, and say
   what a seat's enrollment binds to afterwards, what "the seat's next safe
   point" is in existing terms, and what a seat does when the central pointer
   moved under it. The seat's leaning, for the author to test and overturn with
   evidence: the deploy record (commit, digest, time, previous) becomes the one
   pin and checkouts stop building, because feeding each checkout keeps the
   class of fault the goal exists to remove.
2. **The deploy contract.** One language-neutral contract the lane calls and
   nothing more: its operations (build from the pushed commit, activate, report
   the version, roll back), inputs, outputs, exit meanings, where the project
   declares it, and where the record of each deploy lives. Show it twice: the
   Go engine adapter for this repository, and a Maven adapter as the worked
   second example.
3. **Failure behaviour.** A build that fails after a successful push, an
   activation interrupted halfway, a deploy while paused, two pushes in quick
   succession, a rollback with no previous engine, an engine that deploys but
   cannot start. State for each what the lane does, what stays in force, and
   who is told. A failed deploy never undoes the push.
4. **Every verb and its UI action**, as a table: `deploy status`, `deploy now`,
   `deploy rollback`, `deploy pause` (and its resume), which engine each seat
   runs; and on the Fleet page: current engine, per-seat engine, deploy now,
   rollback, history. Name who may run each (person, lane, seat).
5. **Step 1 and what is deferred.** The design-completeness rule in
   `docs/design/design-obligation-gate.md` binds: name step 1, the smallest
   slice that exists and is usable by itself, and list every deferred item with
   the step-1 field or home it will build on. The whole answer in one slice is
   not a complete design. One builder builds step 1; say what its one real run
   proves.
6. **Decisions reserved for Wido**, listed plainly at the end: anything that
   sets a contract, changes what an adopted project sees, or conflicts with
   the neighbouring accepted design.

### Constraints

- Committed logic is Go; shell is plumbing only. No new dependency.
- Each behaviour has one owner; name the package that owns the deploy record,
  the contract and the pointer.
- A check refuses only for a named invariant whose violation is a real defect.
- Plain English a colleague new to the repository can follow; expand every
  identifier on first use; the phrase "load-bearing" is banned.
- Skills and documents still arrive by pull; say so and leave it out of scope.

Critique findings being answered:

1. none

Example page:

`plans/designs/one-folder-deployed-and-evolved.md` is the example for the
record head, the "Wido's words" section, the plain-English problem statement
and the level of detail. Read its first 60 lines for the shape; do not copy its
length.

The page starts with the record head, a blank line after the title:

```markdown
# <title>

- Kind: design
- Id: <a new ULID>
- Status: draft
- Goals: landing-deploys-the-engine
```

## Recurring findings

none recorded

## Tool-call budget

Maximum delegate tool calls: 60

Stop when this number is reached. List anything the budget did not allow you
to check.

## Page-size ceiling

Maximum page size: 3000 words

Cut a draft that exceeds this ceiling. If cutting would make the page
incomplete, stop and propose a split instead.

## Fresh session

This revision runs in a new delegate session. Use the prior page and critique
only through the context pack above. Never resume a delegate from an earlier
revision.

## Page artifact and return shape

Write the page to: /Users/wido/LocalStorage/GitHub/agentic-tools-m1h/metasystem/plans/designs/landing-deploys-the-engine.md

Return only these two lines:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1h/metasystem/plans/designs/landing-deploys-the-engine.md
DESIGN: ready (N words)
```

or:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1h/metasystem/plans/designs/landing-deploys-the-engine.md
DESIGN: blocked (the reason in one sentence)
```

When blocked, stop and return `BLOCKED:` with the refusal's first line, what you tried and the decision needed; your seat asks.
