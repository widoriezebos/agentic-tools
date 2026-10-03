# Design for landing-deploys-the-engine

- Kind: design
- Id: 01M3Z76QRHDCBSE48GQY6G0MSS
- Status: draft
- Goals: landing-deploys-the-engine

Revision 3: folds critique rounds 1 and 2 (Codex on Astra: 8 then 4 material findings, all accepted; round 2's fold removes the request file whose patches caused three of its four findings; the adjudication is at the end of this page). Facts were read at `b31a1e8e9` on 2026-10-02. Paths are relative to the installation, `metasystem/` in this repository. Nothing here is built yet.

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
  "tools": [{"id": "go", "executable": "go", "versionArgs": ["version"]}]
}
```

**How it is called.** The adapter is an executable in any language, called as `<argv> OPERATION` with one JSON request on standard input and one JSON response on standard output, the pattern of the agent-adapter contract (`internal/runtimes/external/external.go:1-7`). Standard error goes to the deploy's log. Every operation runs in a clean tree of the commit it is about: a detached Git worktree the engine creates under `~/.metasystem/deploy/<project>/work/<commit>/` and removes afterwards. The lane checkout is never built in, because its HEAD moves with the next merge.

**Request.** `{"schema":1, "operation":…, "project":…, "commit":…, "source":…, "artifact":…, "previous":{"commit","version","artifact","digest"} or null}`. `source` is the clean tree's absolute path. `artifact` is what `build` returned.

| Operation | Must do | Response |
|---|---|---|
| `build` | Build the artifact from `source`. Never changes what is active. | `{"outcome":"built","version","artifact","digest"}` |
| `activate` | Make `artifact` the active one. Repeating it is harmless. | `{"outcome":"active","version"}` |
| `version` | Report what is active now. Reads only. | `{"outcome":"active","version","artifact","digest"}`, or `{"outcome":"none"}` when nothing has ever been activated |
| `rollback` | Make `previous.artifact` active again, without building. | as `activate` |

**Exit meanings.** 0: done, and the response is the truth. 1: failed, and the response carries `"outcome":"failed"` and a `reason`; nothing changed. 64: the adapter does not support this operation. Any other exit or unreadable output: failed with the state unknown; the engine calls `version` to learn what is active.

**No time limit.** A slow build is still a build: the runner waits for the adapter to exit and never fails it on elapsed time (ruling R-35-m3). `deploy status` names the running adapter's process, when it started and when its log last grew, so a stalled adapter is visible. A person ends it with `deploy pause`, which stops the run in progress: it ends the adapter's process group and appends a `stopped` line, and the next run's `version` call learns what is active.

**The record.** `~/.metasystem/deploy/<project>/deploys.jsonl`, one line per finished attempt, appended under a lock file beside it. `<project>` names the repository the same way for every checkout of it on this computer: the fetch URL of the landing endpoint's remote (origin), normalized and hashed to a short hexadecimal name. Every checkout of one repository therefore shares one record, one lock and one `pause.json`, as they share the one pointer they all move; a pause made from a person's checkout holds the lane's deploys too. The enrollment's repository identity is the checkout's own path (`internal/steward/runner.go:849-854`) and is not used here. The record lives under the home directory because every seat on the computer must read the same one. Fields: `kind` (deploy or rollback), `commit`, `version`, `artifact`, `digest`, `previous` (the commit that was current), `by`, `startedAt`, `endedAt`, `outcome` (`active`, `build-failed`, `activate-failed`, `verify-failed`, `stopped`), `detail`, `log`. The current deploy is the newest line whose outcome is `active`; there is no second "current" file. Before the first deploy there is no current deploy, and `version` answers `none`. Beside it: `pause.json` (who, when, why). There is no request file: what to deploy is always origin's main.

**The run.** Every trigger starts the runner; it carries no commit. The runner's target is always origin's main tip as it fetches it: landed by construction, and never behind what is active, because main only moves forward. The runner takes the lock (when the lock is held it exits at once: the holder fetches main again before it finishes), asks `version` and reconciles the record with it (`none`: no deploy is current), and fetches main. It stops when paused, when the tip is current, or when this run already failed on the tip; otherwise it runs `build`, `activate` and `version`, appends a line, and fetches main again. A failed attempt appends its failure line and ends the run: a tip is tried once per run, and the next push or `deploy now` tries again. After releasing the lock, and only when not paused, the runner fetches main once more and starts over when the tip is neither current nor already tried by this run, so a push that found the lock held just before the release is not lost.

**Forward only.** A run only ever deploys main's tip, so it only moves the pointer forward. Moving backwards is `deploy rollback` alone.

**The lane's call.** In `runIntentLandingPush`, when `outcome.Changed`, the verb starts `metasystem deploy now` detached, the way `landing prove` starts its proof (`cmd/metasystem/intent_landing_prove.go:85-88`), and adds one detail line. A failure to start is a detail line and fails nothing, like the channel notice. `plain.Push` is unchanged.

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
| First deploy on a computer | `version` answers `none`; the line's `previous` is null. `deploy rollback` is refused as `DEPLOY_NO_PREVIOUS`. | Nothing was active before. |
| Build fails after a push | A `build-failed` line with the log's path; the run ends. The next push or `deploy now` tries again. | The previous engine; the pointer is untouched. |
| Activation interrupted | No line was written. The lock dies with the process. The next run asks `version`, records what is active, removes leftover staging directories and continues. | Whichever the atomic rename left: old or new, never half. |
| Trigger while paused | Nothing is built; the runner exits at once and does not start over. `deploy status` shows main's tip waiting. `deploy now` is refused as `DEPLOY_PAUSED`, naming `deploy resume`. | The current deploy. |
| Two pushes close together, or triggers out of order | A trigger that finds the lock held exits. The holder fetches main after its attempt and once more after releasing the lock, and deploys the newest tip. A commit main has moved past is never built. | One run at a time. |
| `deploy now` when main's tip is already active | Nothing is built or written; the verb reports that the deploy already holds (ruling R-129-ui). | The current deploy. |
| An adapter stalls | `deploy status` shows the run, its start and its log's last growth. A person's `deploy pause` stops it: the adapter's process group is ended and a `stopped` line is appended. | Whatever `version` reports; paused. |
| Rollback asked again | When the current line is already the rollback to the deploy before the newest forward one, nothing changes and nothing is written; the verb reports that it already holds (ruling R-129-ui). | The rolled-back engine, paused. |
| Rollback with no previous | Refused as `DEPLOY_NO_PREVIOUS`; also when the previous artifact is gone or its checksum differs. | The current deploy. |
| Engine deploys but cannot start | `build` runs the engine before answering, so it is never activated. If `version` fails after activation, the runner rolls back, pauses and writes `verify-failed`. | The previous engine, paused. |
| Engine starts but misbehaves | A person runs `deploy rollback`. From step 2, a seat in a session is still on its enrolled engine. | The previous engine, paused. |

A rollback always pauses. Otherwise the next unrelated landing would deploy the bad commit again, since it is still in main.

**What a rollback returns to.** Its target is the deploy that was current before the newest forward deploy (the newest `active` line of kind `deploy`; its `previous` field). A rollback writes a line of kind `rollback` whose `previous` is the commit it moved away from. It never reactivates that commit: only a person's `deploy resume`, or a `deploy now` of that or a newer commit after resuming, moves forward again.

## Decision 4: verbs and UI actions

"Person" means a signed-in human, as `POST /api/fleet/land-now` requires (`internal/ui/httpd/landnow.go:63`).

| Verb | What it does | Who | Fleet page action |
|---|---|---|---|
| `deploy status` (`--history N`, `--verify`, `--json`) | Current version, commit, time and actor; previous; main's tip when it is not yet active; paused; a run in progress with its start and its log's last growth; last failure. `--verify` asks the adapter. | person, lane, seat | "Engine" card: current engine and state; a history list |
| `deploy now` | Deploys origin's main tip as fetched; when it is already active, says so and writes nothing. | person, lane, seat | "Deploy now" button |
| `deploy rollback` | Activates the deploy before the newest forward one and pauses; asked again, it already holds. | person | "Roll back" button, with a confirmation |
| `deploy pause --reason TEXT` | Holds deploys and stops a run in progress. | person | "Pause" button |
| `deploy resume` | Lifts the pause and deploys main's tip. | person | "Resume" button |
| `deploy seats` | Each seat's engine and generation, marked current, behind or unknown. | person, lane, seat | the per-seat engine column, with a "behind" mark |

`deploy seats` reads what each seat already publishes: its presence record carries the enrolled build commit and generation (`internal/seat/record.go:66-67`, `internal/steward/runner.go:128-132`), and the Fleet page already shows them (`internal/ui/fleet/fleet.go:230-231`).

The UI adds a `deploy` block to `GET /api/fleet`, and four routes, `POST /api/fleet/deploy-now`, `deploy-rollback`, `deploy-pause` and `deploy-resume`. Each copies `land-now`: require a session, run the verb, return its outcome. The card is a new React component beside `internal/ui/web/_app/src/fleet/LandingLane.tsx`.

## Owners

| Behaviour | Owner |
|---|---|
| The contract, the adapter call, the record, pause, the lock and the runner | new package `internal/deploy`; it interprets no language |
| The engines directory and the pointer | new package `internal/enginedeploy` |
| The `~/.metasystem` path | `internal/board` (`internal/board/card.go:160-176`), unchanged |
| The verbs | new file `cmd/metasystem/intent_deploy.go` |
| The lane's trigger | `cmd/metasystem/intent_landing_push.go` |
| The enrollment and its re-arm | `internal/steward`, changed only in step 2 |

## Moved effects

Step 1 moves no effect: it adds the deploy chain beside what exists, and hooks, enrollment, pins and the per-checkout build keep their owners. Decision 1 moves these in step 2; they are listed so step 2 starts from them, and none is built in step 1.

| Effect | From | To | Code |
|---|---|---|---|
| Building the engine a checkout runs | each checkout's session-start rebuild, its log and lock handoff | the deploy runner's `build` (step 2) | `metasystem/cmd/metasystem/hook_entry.go:646-692` |
| Keeping a running generation's bytes alive | engine pin files per checkout | the never-rewritten `engines/<commit>/` directory (step 2) | `metasystem/internal/steward/identity.go:301-379` |
| Deciding which old engine bytes may be removed | pin retention per checkout | the deploy record's history (step 2, with the deferred cleanup) | `metasystem/internal/steward/disk_pins.go:143-178` |
| Naming the engine an enrollment binds to | the checkout's `bin/metasystem` | the deploy record's current line (step 2) | `metasystem/internal/steward/identity.go:58-96`, `metasystem/internal/up/up.go:634-638` |

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
| Deploy after a seat lands its own work on a computer with no lane (`pushed`, `internal/landing/landpath/land.go:562`) | the runner; until then a person runs `deploy now` |
| A computer that did not push follows main by itself | the runner; the steward's tick |
| Removing old `engines/` directories: keep the current, the previous, and any an enrollment or a live process names | the record's history; the pin rules in `internal/steward/disk_pins.go` |
| Telling a person of a failed deploy without being asked | the record's `outcome` and `detail` |
| A built Maven adapter | the contract; the worked example |
| Deploying a named landed commit (`deploy now --commit`) | the runner; the forward-only rule |

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
8. **Who may roll back.** Recommended: a person only, with `deploy now` open to seats because it only deploys main's tip, which only moves forward.
9. **The search path.** Should `system setup` add `~/.metasystem/bin` to PATH, or only print the line?

## Not checked

The tool-call budget was not reached. These were not opened, and the builder checks them first:

- whether `cmd/devgate build --out` runs in a worktree outside a registered checkout (it resolves a cache domain from its root, `cmd/devgate/build.go:76-84`);
- whether the landed-history check at re-arm fetches, or needs the seat to have fetched the commit;
- whether the repository identity is safe as a directory name;
- which engine command runs without touching state, for the build's start check;
- the reader list in fact 12 of the brief, beyond a file-name search that found about forty files;
- how a seat that changes engine code runs its own build after step 2.

## Critique round 1 (Codex on Astra), adjudication

| Finding | Disposition | Amendment |
|---|---|---|
| DEPLOY-SHARED-IDENTITY | accepted: the enrollment's identity is the checkout path, so sibling checkouts would keep separate records, locks and pauses for one pointer | `<project>` is the hashed fetch URL of origin, shared by every checkout (Decision 2, the record) |
| DEPLOY-INITIAL-STATE | accepted: the first real run starts with no pointer | `version` may answer `none`; a first-deploy row in Decision 3 |
| DEPLOY-BACKWARD-AUTHORITY | accepted: `deploy now --commit` could reach the previous engine without a person | forward only; a backward move is `deploy rollback` alone (Decision 2, the run; Decision 4) |
| RULING-R-129-ui | accepted: a repeated rollback would reactivate the rejected engine | rollback's target and its "already holds" repeat (Decision 3) |
| DEPLOY-LOST-HANDOFF | accepted: a trigger between the last check and the release was lost | one more read of `wanted.json` after the release (Decision 2, the run) |
| DEPLOY-FAILURE-TERMINATION | accepted: the loop would rebuild a failing commit forever | one attempt per commit per trigger (Decision 2, the run; Decision 3) |
| RULING-R-35-m3 | accepted: elapsed limits turned slowness into failure | `buildMs` and `activateMs` removed; `deploy status` shows a stall instead (Decision 2) |
| MOVED-EFFECTS-INVENTORY | accepted: Decision 1's step-2 moves had no inventory | the Moved effects section, every row step 2 |

## Critique round 2 (Codex on Astra), adjudication

Three of the four findings sat in round 1's fold of the request file, so the fold removes the file instead of patching it again.

| Finding | Disposition | Amendment |
|---|---|---|
| DEPLOY-WANTED-ORDER | accepted: a delayed older trigger could overwrite a newer request | no request file: the runner always deploys origin's main tip (Decision 2, the run) |
| RULING-R-129-ui | accepted: `deploy now` of the active engine was refused | it reports that the deploy already holds and writes nothing (Decisions 3 and 4) |
| RULING-R-126-m1e | accepted: recovery named a process id | `deploy pause` stops a run in progress (Decisions 2, 3 and 4) |
| DEPLOY-PAUSED-RESTART | accepted: a paused runner restarted itself after release | the after-release check runs only when not paused (Decision 2, the run) |

