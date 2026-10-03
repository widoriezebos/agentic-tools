# Design for landing-deploys-the-engine

- Kind: design
- Id: 01M3Z76QRHDCBSE48GQY6G0MSS
- Status: draft
- Goals: landing-deploys-the-engine

First draft; no critique round has run. Facts were read at `b31a1e8e9` on 2026-10-02. Paths are relative to the installation, `metasystem/` in this repository. Nothing here is built yet.

## Wido's words (2026-10-01, binding)

> "after a successful landing push, the lane runs the project's deploy step on the pushed commit. For the metasystem itself: build the engine from that commit into ~/.metasystem/engines/<sha>/, atomically repoint ~/.metasystem/bin/metasystem (on PATH) to it, seats pick it up at their next safe point and their hooks call the central binary; deploy rollback repoints to the previous engine. Other apps supply their own deploy step (docker, kubernetes) at that extension point."

> "per-checkout rebuild/restart caused today's stale engines, generation mismatches and refused proofs; one place with the newest proven engine removes that class."

> "full control by verbs where applicable (deploy status/now/rollback/pause, which engine each seat runs) and all of it from the UI too"

> "the lane only calls the project's deploy adapter (build artifact from the pushed commit, place/activate it, report the version, roll back) with language-neutral input/output; the metasystem's Go engine is just one adapter"

## The problem in plain English

The engine is the compiled `metasystem` program. A seat is one checkout with its own nickname and enrollment; several seats share one computer and one `~/.metasystem` directory.

- **A landing ends at the push.** `plain.Push` pushes the proven commit and appends one line to `artifacts/agents/landing/pushes.jsonl` (`internal/landing/plain/push.go:84-90`). The `landing push` verb then posts one channel notice (`cmd/metasystem/intent_landing_push.go:55-60`). Nothing else can run after a push.
- **Every checkout rebuilds for itself.** At session start, a checkout whose engine is older than its sources starts a detached build and ends the session unarmed with the `engine-rebuilding` notice (`internal/hooks/runtime_hook_start.go:794-815`). The next start re-enrolls the new bytes. Until each checkout has done this, it runs a stale engine; while it does, the recorded generation and checksum disagree with the file, and proofs refuse.
- **Nothing is language-neutral.** A project built with Maven has no place to say how its landed commit is deployed.

## Decision 1: the central engine replaces the per-checkout build and pins

**Today.** Each checkout's enrollment record, `artifacts/agents/steward/identity.json`, names an engine path, its SHA-256 checksum, a generation number and the build's commit (`internal/steward/identity.go:58-96`). Startup refuses when the running engine's resolved path is not the enrolled path (`internal/up/up.go:634-638`). When the bytes at that path change, startup mints the next generation only if the build's commit is in landed history (`internal/steward/runner.go:526-561`). Engine pins, `artifacts/agents/steward/engine-pins/generation-N-<sha256>`, keep one generation's bytes alive while the checkout's file is overwritten (`internal/steward/disk_pins.go:3-11,44-48`).

**Feeding is rejected.** Copying the central engine into every checkout keeps one file, one checksum and one generation mint per checkout, each able to lag or fail alone. That is the class of fault the goal removes.

**Replacing, in existing terms.**

- **What an enrollment binds to.** The record keeps its fields. `InstallPath` becomes the resolved path `~/.metasystem/engines/<commit>/metasystem`; `InstallDigest`, `EngineBuild` and `LandedCommit` are copied from the deploy record's line for that commit. The deploy record is the one pin.
- **Why pins go.** An `engines/<commit>/` directory is never rewritten, so a running process's bytes cannot change under it. The directory is the pin; per-checkout pin files are no longer written.
- **The next safe point** is session start: the ordinary startup path, the only place that mints a generation today (`internal/up/up.go:703`). The scheduler's recovery path never mints (`internal/up/up.go:840`) and stays that way.
- **When the pointer moved under a seat.** The pointer is the symbolic link `~/.metasystem/bin/metasystem`. Hooks call it. At its entry, the engine compares its own resolved path with the checkout's enrolled path. If they differ and the call is not session start, it hands the call to the enrolled engine, which still exists. A session therefore finishes on the engine it started with. At session start, the engine mints the next generation bound to itself, on the same evidence as today (its build commit is in landed history) plus one new check: its path and checksum equal the deploy record's current line.
- **Checkouts stop building.** The session-start rebuild (`cmd/metasystem/hook_entry.go:646-693`) is deleted for seats on the central engine.

This switch is step 2, not step 1. It changes how the engine finds its installation: today it refuses unless it sits at `<installation>/bin/metasystem` (`internal/stateroot/stateroot.go:317`), and about forty files read that path.

## Decision 2: the deploy contract

**Where a project declares it.** A new key `deploy.contract` in `metasystem.conf`, default `deploy.json`, beside `launch.contract` (`internal/config/defaults.go:67`). A project without the file has no deploy: the lane does nothing more after its push, and each deploy verb says so.

```json
{
  "schema": 1,
  "adapter": {"argv": ["go", "run", "-trimpath", "./cmd/devgate", "deploy"], "cwd": "metasystem"},
  "tools": [{"id": "go", "executable": "go", "versionArgs": ["version"]}],
  "buildMs": 900000,
  "activateMs": 60000
}
```

**How it is called.** The adapter is an executable in any language, called as `<argv> OPERATION` with one JSON request on standard input and one JSON response on standard output, the pattern of the agent-adapter contract (`internal/runtimes/external/external.go:1-7`). Standard error goes to the deploy's log. Every operation runs in a clean tree of the commit it is about: a detached Git worktree the engine creates under `~/.metasystem/deploy/<repository>/work/<commit>/` and removes afterwards. The lane checkout is never built in, because its HEAD moves with the next merge.

**Request.** `{"schema":1, "operation":…, "project":…, "commit":…, "source":…, "artifact":…, "previous":{"commit","version","artifact","digest"} or null}`. `source` is the clean tree's absolute path. `artifact` is what `build` returned.

| Operation | Must do | Response |
|---|---|---|
| `build` | Build the artifact from `source`. Never changes what is active. | `{"outcome":"built","version","artifact","digest"}` |
| `activate` | Make `artifact` the active one. Repeating it is harmless. | `{"outcome":"active","version"}` |
| `version` | Report what is active now. Reads only. | `{"outcome":"active","version","artifact","digest"}` |
| `rollback` | Make `previous.artifact` active again, without building. | as `activate` |

**Exit meanings.** 0: done, and the response is the truth. 1: failed, and the response carries `"outcome":"failed"` and a `reason`; nothing changed. 64: the adapter does not support this operation. Any other exit, a timeout or unreadable output: failed with the state unknown; the engine calls `version` to learn what is active.

**The record.** `~/.metasystem/deploy/<repository>/deploys.jsonl`, one line per finished attempt, appended under a lock file beside it. `<repository>` is the repository identity the enrollment already carries. It lives under the home directory because every seat on the computer must read the same record. Fields: `kind` (deploy or rollback), `commit`, `version`, `artifact`, `digest`, `previous` (the commit that was current), `by`, `startedAt`, `endedAt`, `outcome` (`active`, `build-failed`, `activate-failed`, `verify-failed`), `detail`, `log`. The current deploy is the newest line whose outcome is `active`; there is no second "current" file. Beside it: `wanted.json` (the commit asked for) and `pause.json` (who, when, why).

**The run.** Every trigger writes `wanted.json` and starts the runner. The runner takes the lock, asks `version` and reconciles the record with it, then loops: stop when paused or when the wanted commit is current; otherwise `build`, `activate`, `version`, append a line. A commit is deployed only when it is in the history of origin's main; anything else is refused as `DEPLOY_NOT_LANDED`, because deploying unproven code is the defect.

**The lane's call.** In `runIntentLandingPush`, when `outcome.Changed`, the verb starts `metasystem deploy now --commit <pushed>` detached, the way `landing prove` starts its proof (`cmd/metasystem/intent_landing_prove.go:85-88`), and adds one detail line. A failure to start is a detail line and fails nothing, like the channel notice. `plain.Push` is unchanged.

### Adapter one: the Go engine (this repository)

Its logic is the new package `internal/enginedeploy`, reached as `go run -trimpath ./cmd/devgate deploy OPERATION`, beside the existing build owner `cmd/devgate build`.

- `build`: runs the existing build with `--out` (`cmd/devgate/build.go:115-123`) into `~/.metasystem/engines/<commit>.staging-<pid>/metasystem`. It runs the built engine once and checks its build stamp equals the commit. It then renames the directory to `engines/<commit>/`. An existing `engines/<commit>/` with the same checksum is reused. `version` is the commit; `artifact` is the file's path.
- `activate`: writes a new symbolic link beside the pointer and renames it over `~/.metasystem/bin/metasystem`. A rename is atomic: a reader sees the old engine or the new one.
- `version`: reads the link, the file's stamp and its checksum.
- `rollback`: checks `previous.artifact` exists with `previous.digest`, then activates it.

### Adapter two: Maven (worked example)

`deploy.json` names the project's own executable, for example `{"argv": ["java", "deploy/Deploy.java"]}`.

| Operation | A jar on a server | A container image |
|---|---|---|
| `build` | `mvn -B -DskipTests package`; copy the jar to `/opt/app/releases/<commit>/`; `version` is `1.4.2+<commit>` | `mvn -B package jib:build -Dimage=registry/app:<commit>`; `artifact` is the image digest |
| `activate` | repoint `/opt/app/current`, restart the service | `kubectl set image deployment/app app=<artifact>`, wait for `kubectl rollout status` |
| `version` | read the jar manifest under `current` | read the deployment's image |
| `rollback` | repoint to `previous.artifact`, restart | `kubectl set image` to `previous.artifact` |

Tests are skipped because `landing prove` already proved the tree. The engine sees only the four operations.

## Decision 3: failure behaviour

A failed deploy never undoes the push. In step 1 a failure is told by the record, by `deploy status`, and by a detail line on the next `landing push`.

| Case | What happens | What stays in force |
|---|---|---|
| Build fails after a push | A `build-failed` line with the log's path. | The previous engine; the pointer is untouched. |
| Activation interrupted | No line was written. The lock dies with the process. The next run asks `version`, records what is active, removes leftover staging directories and continues. | Whichever the atomic rename left: old or new, never half. |
| Trigger while paused | `wanted.json` is updated; nothing is built. `deploy status` shows the commit waiting. `deploy now` is refused as `DEPLOY_PAUSED`, naming `deploy resume`. | The current deploy. |
| Two pushes close together | The second finds the lock held, writes `wanted.json` and exits. The first run re-reads it and deploys the newest. An overtaken commit is never built. | One run at a time. |
| Rollback with no previous | Refused as `DEPLOY_NO_PREVIOUS`; also when the previous artifact is gone or its checksum differs. | The current deploy. |
| Engine deploys but cannot start | `build` runs the engine before answering, so it is never activated. If `version` fails after activation, the runner rolls back, pauses and writes `verify-failed`. | The previous engine, paused. |
| Engine starts but misbehaves | A person runs `deploy rollback`. From step 2, a seat in a session is still on its enrolled engine. | The previous engine, paused. |

A rollback always pauses. Otherwise the next unrelated landing would deploy the bad commit again, since it is still in main.

## Decision 4: verbs and UI actions

"Person" means a signed-in human, as `POST /api/fleet/land-now` requires (`internal/ui/httpd/landnow.go:63`).

| Verb | What it does | Who | Fleet page action |
|---|---|---|---|
| `deploy status` (`--history N`, `--verify`, `--json`) | Current version, commit, time and actor; previous; wanted; paused; last failure. `--verify` asks the adapter. | person, lane, seat | "Engine" card: current engine and state; a history list |
| `deploy now` (`--commit C`) | Deploys origin's main as fetched, or a named landed commit. | person, lane, seat | "Deploy now" button |
| `deploy rollback` | Activates the previous deploy and pauses. | person | "Roll back" button, with a confirmation |
| `deploy pause --reason TEXT` | Holds deploys; triggers only record the wanted commit. | person | "Pause" button |
| `deploy resume` | Lifts the pause and deploys the wanted commit. | person | "Resume" button |
| `deploy seats` | Each seat's engine and generation, marked current, behind or unknown. | person, lane, seat | the per-seat engine column, with a "behind" mark |

`deploy seats` reads what each seat already publishes: its presence record carries the enrolled build commit and generation (`internal/seat/record.go:66-67`, `internal/steward/runner.go:128-132`), and the Fleet page already shows them (`internal/ui/fleet/fleet.go:230-231`).

The UI adds a `deploy` block to `GET /api/fleet`, and four routes, `POST /api/fleet/deploy-now`, `deploy-rollback`, `deploy-pause` and `deploy-resume`. Each copies `land-now`: require a session, run the verb, return its outcome. The card is a new React component beside `internal/ui/web/_app/src/fleet/LandingLane.tsx`.

## Owners

| Behaviour | Owner |
|---|---|
| The contract, the adapter call, the record, wanted, pause, the lock and the runner | new package `internal/deploy`; it interprets no language |
| The engines directory and the pointer | new package `internal/enginedeploy` |
| The `~/.metasystem` path | `internal/board` (`internal/board/card.go:160-176`), unchanged |
| The verbs | new file `cmd/metasystem/intent_deploy.go` |
| The lane's trigger | `cmd/metasystem/intent_landing_push.go` |
| The enrollment and its re-arm | `internal/steward`, changed only in step 2 |

## Step 1

Step 1 is the deploy chain with no seat depending on it:

- `internal/deploy` with the contract, the record and the runner;
- the Go engine adapter and this repository's `deploy.json`;
- the lane's trigger after a push;
- `deploy status`, `deploy now`, `deploy rollback`, `deploy pause` and `deploy resume`.

Hooks, enrollment, pins and the per-checkout build are untouched. It is usable by itself: `~/.metasystem/bin/metasystem` is always the newest landed engine for a person's terminal, with history and rollback. When `~/.metasystem/bin` is not on the shell's search path (PATH), `deploy status` prints the line to add; it never edits a shell profile.

**Tests.** A fixture adapter drives the runner through every row of the failure table. The pointer swap is tested under a concurrent reader. Each refusal has a test that fails when the refusal is removed.

**The one real run.** One real landing through the plain lane on this computer. Afterwards: `deploys.jsonl` holds an `active` line for the pushed commit; `~/.metasystem/engines/<commit>/metasystem` exists and its stamp is that commit; the pointer names it; `deploy status` run through the pointer reports it. A second landing moves the pointer. `deploy rollback` returns to the first and pauses, and `deploy resume` redeploys the second. This proves the chain from a real push to the pointer and back. It proves nothing about seats.

## Deferred

Skills and documents still arrive by pull; that is out of scope.

| Item | Builds on |
|---|---|
| Step 2: seats run the central engine (Decision 1): the enrollment binds to `engines/<commit>/`, the handover at entry, the session-start re-arm, hooks call the pointer, the installation is found from the working directory, the per-checkout build and pins are retired | the record's `artifact`, `digest` and `commit`; the pointer |
| Step 3: the Fleet card, the four routes and the per-seat "behind" mark | the verbs' `--json`; `deploy seats` |
| `deploy seats` | the record's current line; the presence record's `Engine` |
| Deploy after a seat lands its own work on a computer with no lane (`pushed`, `internal/landing/landpath/land.go:562`) | `wanted.json` and the runner; until then a person runs `deploy now` |
| A computer that did not push follows main by itself | `wanted.json`; the steward's tick |
| Removing old `engines/` directories: keep the current, the previous, and any an enrollment or a live process names | the record's history; the pin rules in `internal/steward/disk_pins.go` |
| Telling a person of a failed deploy without being asked | the record's `outcome` and `detail` |
| A built Maven adapter | the contract; the worked example |

## Beside `one-folder-deployed-and-evolved`

That accepted design keeps the engine at `metasystem/bin/metasystem`, lists `bin/` as rebuilt by an upgrade, and says the installation is found from `bin/..`. Step 1 here changes none of that: it adds an engine outside every checkout and no reader moves. When that design's step 1 lands, `deploy.json` should take the same default home as `launch.json` (`project/`) and join its payload exclusions. Step 2 here does conflict with it; see decisions 3 and 4 below. This page does not rewrite it.

## Decisions reserved for Wido

1. **The contract.** Accept the key `deploy.contract`, the file `deploy.json`, the four operations and the exit meanings as written? It is a contract every adopted project will see.
2. **Replace, not feed.** Accept that enrollments bind to `~/.metasystem/engines/<commit>/` and that per-checkout builds and pins are retired in step 2?
3. **Who gets the central engine.** Recommended: this repository only; an adopted project keeps `metasystem/bin/metasystem` as the neighbouring design says, and uses the contract only to deploy its own application. The alternative changes that accepted design.
4. **Finding the installation.** Step 2 needs the engine to find its installation from the working directory, not from `bin/..`, and a new value for `metasystem.engine-delivery` (today only `source`, `internal/config/defaults.go:61`). Both touch the neighbouring design.
5. **The safe point.** Recommended: session start, with a running session finishing on its enrolled engine. Your "hard cutover" could instead mean every call runs the newest engine at once, which changes a generation in mid-session.
6. **Telling of a failed deploy.** The channel carries one kind of news today, a landing. Should a failed deploy be a second kind?
7. **Other computers.** Should a computer that did not push deploy by itself when it sees main moved, or only when a person runs `deploy now`?
8. **Who may roll back.** Recommended: a person only, with `deploy now` open to seats because it can only reach a landed commit.
9. **The search path.** Should `system setup` add `~/.metasystem/bin` to PATH, or only print the line?

## Not checked

The tool-call budget was not reached. These were not opened, and the builder checks them first:

- whether `cmd/devgate build --out` runs in a worktree outside a registered checkout (it resolves a cache domain from its root, `cmd/devgate/build.go:76-84`);
- whether the landed-history check at re-arm fetches, or needs the seat to have fetched the commit;
- whether the repository identity is safe as a directory name;
- which engine command runs without touching state, for the build's start check;
- the reader list in fact 12 of the brief, beyond a file-name search that found about forty files;
- how a seat that changes engine code runs its own build after step 2.
