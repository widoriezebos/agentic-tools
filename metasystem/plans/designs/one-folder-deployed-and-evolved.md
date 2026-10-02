# One folder: what MetaSystem deploys, apart from what the application evolves

- Kind: design
- Id: 01M3YG08QFSF1W2HW3RN8WXWMZ
- Status: draft

Facts were read at `5fa845565` on 2026-10-02, unless marked *(recited)*. The inventory is `agentic-tools-evidence/shipped-folder-20261002/inventory.md`. All paths on this page are relative to this repository's root.

# Part 1: Intent

## Wido's words (2026-10-02, binding)

> "we really need to move everything required by metasystem runtime and agents participating in that into the folder that is 'deployed/shipped'. Otherwise the metasystem will break in an adopted repo."

> "ideally there is only 1 folder (metasystem) in an adopted repo. Having more folders will be very confusing for the people who just started using the metasystem."

> "plans/* APPLICATION specific. It is not metasystem RUNTIME specific. We need a separation between what we deploy and what we evolve over time as we build the app with the metasystem."

## The problem in plain English

**What breaks for an adopter today.**

- **Adoption pours MetaSystem into the application's root.** `system adopt` copies its files to the top of the target repository (`metasystem/internal/adopt/adopt.go:381`). About twenty entries land beside the application's own code: `cmd/`, `internal/`, `go.mod`, `docs/`, `memory/`, `plans/`, `records/`, `skills/`, `bin/`, `artifacts/` and the rest. Any Go application already has a `go.mod`, and most have `cmd/`, `internal/` or `docs/`. Adoption refuses on the first differing file (`adopt.go:880-901`), so a typical existing application cannot adopt at all.
- **The one-folder placement exists, but only half of it works.** The engine accepts an installation in a subfolder such as `vendor/metasystem`. It then looks for the application's state at the repository root, because the state root is the Git top for anything that is not the template (`metasystem/internal/stateroot/stateroot.go:145-153`). Adoption, however, writes the seeded plans, memory and records inside the installation. The runtime registrations also land inside it, where Claude Code, Codex and GitHub never look. The result is that state is split and partly unread.
- **Engine code assumes this repository's shape.** The Codex critique refuses when `<worktree>/metasystem` is missing (`metasystem/internal/launch/codex.go:106-109`), and that refusal fires on every root adoption. A landing writes its receipt row to `metasystem/memory/receipts.log` (`metasystem/internal/goal/branch/land.go:481`). The testing contract is fixed at `metasystem/testing.json` (`metasystem/internal/testpolicy/contractgit/driver.go:16`). The inventory names twelve such site groups.
- **Shipped texts cite files that never ship.** The design-critique skill points agents to `plans/memory-system-r6.md` "in the host repository", and to two `records/stop-loss/` files that exist only in an archive tag.
- **An upgrade cannot tell MetaSystem's files from the application's.** The one ownership classifier marks `memory/`, `plans/` and `records/` as MetaSystem's own in a root adoption (`metasystem/internal/stateroot/owner.go:147`). An upgrade guided by it would overwrite the application's goals and decisions. The manual upgrade checklist leaves out `docs/intent`, `docs/doctrine`, `docs/decisions`, `memory/`, `records/`, `testing.json` and both configuration files.

**What confuses a newcomer.** They open their repository and see twenty unfamiliar entries mixed with their own code. `docs/` holds MetaSystem's manuals next to their own intent and decisions, and only the filenames tell them apart. `memory/README.md` is MetaSystem's, while `memory/rulings.md` is theirs. Nothing says which files they may edit, or what an upgrade will replace.

## The four classes and the rule for each

| Class | What | Rule |
|---|---|---|
| 1. MetaSystem runtime | engine source and `bin/`, `AGENTS.md`, `wow.md`, MetaSystem's own docs, skills, brief templates, hooks, compiled defaults | Deployed. Replaced only by an upgrade, as a whole. The application never edits it. |
| 2. Application knowledge | plans (goals, designs, briefs, backlog), decisions, rulings, retros, records, receipts, doctrine, covenant, intent, the testing contract | Evolves with the application and is versioned in the application's Git. An upgrade never touches it. |
| 3. Application configuration | `metasystem.conf` (committed), `metasystem.conf.local` (per machine, never committed) | Owned by the application. An upgrade never touches it. |
| 4. Runtime state | jobs, leases, locks, evidence, caches, logs under `artifacts/` | Belongs to one machine and can be thrown away. Never committed. |

## What "done" looks like for a person adopting MetaSystem

They run `metasystem system adopt` in their repository. Afterwards the visible top level gains exactly one folder, `metasystem/`, and two small files, `AGENTS.md` and `CLAUDE.md`, each a few lines long and pointing into the folder. Inside `metasystem/`, `project/` is theirs: every goal, design and decision they make lands there and is committed with their code. Everything else in `metasystem/` is MetaSystem's, and an upgrade swaps it out without touching `project/`. An agent started anywhere in the repository finds every cited file, because every citation is written the same way.

# Part 2: Design

## Threat model and rabbit-hole risks (read before critiquing)

This is the template default. Our own agents and operators make mistakes: a seat writes state to the old path, a text cites a bare path, an upgrade copies over a project file. Nobody attacks.

| Rabbit hole | Mitigation |
|---|---|
| A migration framework for every past layout | One hard cutover. This repository moves in one commit. Root-placed and `vendor/`-placed adopters are deferred, and the new engine refuses those layouts with one line of guidance. |
| Making every path configurable | One fixed folder name, `metasystem`, always at the repository root. No key names a path. |
| Moving class 4 off the repository entirely | Class 4 stays in `metasystem/artifacts/`, ignored by Git. The evidence root that already lives outside the repository stays as it is. |
| Redesigning root resolution beyond the one rule | Only the state-root rule changes. Locating the installation from `bin/..` stays as it is. |
| Touching unrelated docs | Only texts that cite paths are rewritten, and only their path tokens. This repository's `development/`, `paper/`, `environment/` and `redesign/` stay where they are. |
| Moving the configuration files in step 1 | 169 Go sites join `metasystem.conf` onto the installation (*grep count*). The files stay where they are in step 1 and are listed by name as never touched. Moving them is deferred. |
| Renaming `artifacts/` to `local/` in step 1 | `artifacts` appears at 403 Go sites. `local/` is named in the layout and reserved, and the rename is deferred. |
| Rewriting the engine's ~120 Go messages that print state paths | Deferred. Step 1 fixes the texts agents route by. |
| Keeping a fallback for the old layout | None. Mixed engine versions are parked; old installations keep their old engine. |
| Moving the runtime discovery files into the folder | This is impossible: runtimes read `.claude/`, `.codex/`, `.agents/` and `.devin/` at the launch directory, and GitHub reads `.github/` at the root. They stay generated, and are never hand-edited. |
| Rewriting Git history for the move | `git mv` only. |

## The layout

```
<app>/
  AGENTS.md, CLAUDE.md          pointer files (managed block only)
  .claude/ .agents/ .codex/ .devin/ .github/workflows/metasystem.yml   generated registrations
  metasystem/                   class 1, replaced as a whole on upgrade
    AGENTS.md wow.md docs/ skills/ cmd/ internal/ go.mod bin/ ...
    metasystem.conf             class 3, committed (stays here in step 1)
    metasystem.conf.local       class 3, ignored (stays here in step 1)
    artifacts/                  class 4, ignored (becomes local/ later)
    project/                    classes 2+3, committed, never shipped
      plans/ (goals/, designs/, reviews/, briefs, goals.md)
      memory/ records/ testing.json
      docs/intent/ docs/doctrine/ docs/decisions/ docs/project-rules.md
    local/                      reserved, ignored, never shipped
```

This repository has exactly the same shape. The only difference is that its `metasystem.conf` carries `metasystem.template=true`.

## How the engine resolves roots after the change

- **Installation root:** unchanged. It is `bin/..` of the running executable and must hold `metasystem.conf` (`metasystem/internal/stateroot/stateroot.go:304-320`).
- **State root, the one rule:** the state root is `<installation>/project`, in every layout. `RootForInstallation` (`stateroot.go:145-153`) loses both its template branch and its `repositoryTop` branch. Template mode no longer decides where state lives.
- **Machine state:** `Steward` (`stateroot.go:297`) resolves against the installation, not the state root, so it stays in `metasystem/artifacts/agents/steward`.
- **Layout:** `ResolveLayout` (`stateroot.go:164-213`) accepts one placement, `<git top>/metasystem`. That gives `RepositoryRoot = GitRoot` and `InstallationRel = "metasystem"`. The engine then refuses an installation at the Git top, or at any other depth, with this guidance:
  > This installation uses an older layout. Adopt the current MetaSystem into a clean repository, or keep the engine it came with.
- **Readers of Git trees** join the prefix `metasystem/project/` before `RelativeRoot(kind)` (`stateroot.go:287`).
- **Second design homes:** both go away (`metasystem/internal/project/project.go:213-219`). The checkout's own `plans/designs/` moves into the state root. The flat historical designs are now under `project/plans/` and are read from there.

## The upgrade rule

- **Replaced:** every path inside `metasystem/` that `stateroot.Owner` classes as `metasystem-generic`.
- **Never touched:** `project/**`, `local/**`, `artifacts/**`, `metasystem.conf` and `metasystem.conf.local`.
- **Rebuilt:** `bin/`.
- **Enforced by:**
  - The classifier (`metasystem/internal/stateroot/owner.go:96-120`) becomes the single list. `project/` is app-owned. `artifacts/`, `bin/` and `local/` are runtime. The two configuration files are app-owned. The rest of the installation is generic. The root-inventory branch (`owner.go:110-160`) is deleted, because no installation sits at the root any more.
  - The payload never contains a never-touched path (audit B1). Replacing the generic set therefore cannot carry project content in either direction.
  - The upgrade verb itself is deferred. Today's manual checklist in `docs/metasystem-reconciliation.md` is rewritten to "replace everything but these five names".

## The move in this repository

The move is one commit made with `git mv`, while the lane is paused and no seat is writing, because the receipts and the digest are live. Every seat rebuilds its engine afterwards.

- `metasystem/{plans,memory,records}` move to `metasystem/project/{plans,memory,records}`.
- `metasystem/docs/{intent,doctrine,decisions,project-rules.md}`, and the files in `docs/stop-decision-moves/` other than its README, move under `metasystem/project/docs/`.
- `metasystem/testing.json`, `testing-coverage-floors*.json` and `testing-parallel-ratchet.json` move to `metasystem/project/`. Their `paths` and `inputs` that name moved files are rewritten.
- The root `plans/**`, designs included (no name collides, checked), moves to `metasystem/project/plans/`.
- `metasystem/.gitattributes` union lines gain the `project/` prefix. `metasystem/.gitignore` gains `local/`.
- The three READMEs (`plans/`, `memory/`, `records/`) move with their folders. Adoption seeds them once.

## How adoption creates the layout

- The payload is `git archive HEAD:metasystem` minus `project/`, `local/`, `artifacts/`, `bin/` and the `.local` file. The stripping code (`metasystem/internal/adopt/adopt.go:650`, `:684-687`, `:747`) is deleted, because nothing app-owned is left in the archive to strip.
- The payload is copied to `<target>/metasystem/`. `bin/metasystem` goes to `<target>/metasystem/bin/`.
- Adoption then seeds `metasystem/project/` once:
  - goal genesis, written to `metasystem/project` instead of the target (`adopt.go:408`);
  - the empty registers and READMEs;
  - `plans/goals.md`;
  - an incomplete `testing.json`;
  - `docs/project-rules.md` with the SHA marker.
- `artifacts/` and its ignore line go inside `metasystem/` (`adopt.go:418-421`).
- `collide()` now looks only at `metasystem/`, the two pointer files and the registrations. An application's own `go.mod`, `cmd/` and `docs/` can no longer collide.
- **Pointer files.** `system setup` writes the managed block into the root `AGENTS.md` and `CLAUDE.md` for every installation, not only the template (`metasystem/internal/hostsetup/setup.go:131`). The block reads:
  > MetaSystem is installed in `metasystem/`. Its agent contract is `metasystem/AGENTS.md`; route with `metasystem/wow.md`. This application's plans, decisions and records are in `metasystem/project/`. Paths in MetaSystem's texts are relative to this repository's root.

  The `development/` line stays template-only (`setup.go:228-235`). Text outside the markers belongs to the application and survives.
- **Why files and not a folder:** Claude Code, Codex and Devin discover instructions only as a file named `CLAUDE.md` or `AGENTS.md` in the directory they start in. A pointer file is the least that works, it adds no folder, and the application can keep its own text in it.

## Citing files from any working directory

- **One base for every agent-facing text:** the repository root, which is `git rev-parse --show-toplevel` of the checkout or worktree the agent is in.
  - MetaSystem's files are cited as `metasystem/...`.
  - The application's are cited as `metasystem/project/...`.
- This matches Git paths, the return schemas (`^metasystem/.+`) and the testing contract, all of which already use it.
- **What gets rewritten, token by token:**
  - the bare paths in `AGENTS.md`, `wow.md`, the role files, the brief templates and every shipped `SKILL.md`;
  - the self-hosted `R0/metasystem` base in `design-common.md`.
- The base sentence opens `metasystem/AGENTS.md` and `metasystem/wow.md`.
- Commands that must run inside the installation say so explicitly: "`cd metasystem && go run ./cmd/devgate static`".
- The design-critique skill's three dangling citations become plain prose with no path.

## The audit that keeps the boundary

These checks are added to `metasystem audit` (`metasystem/internal/audit/metasystem.go:74`) and run in the static gate:

- **B1, payload:** the staged payload holds nothing under `project/`, `local/`, `artifacts/` or `bin/`, and no `*.local` file.
- **B2, shape:** a fresh adoption into a temporary Git repository changes the top level only by `metasystem/`, `AGENTS.md`, `CLAUDE.md` and the registration entries. `git status` after one goal-and-design round shows changes only under `metasystem/project/`.
- **B3, citations:** every backticked path in a shipped agent text starts with `metasystem/`. It must either exist in the shipped tree, or be a seeded file or home under `metasystem/project/`. This is derived from the texts and replaces the hardcoded list of 32 paths in `TestShippedInstallationRoutedAssetsExist` (`metasystem/internal/audit/shipped_installation_test.go:109`).
- **B4, engine:** outside `internal/stateroot`, no non-test Go file contains `metasystem/memory`, `metasystem/plans`, `metasystem/records` or `metasystem/testing.json`. This is a literal check, and its limit is that a join built in pieces escapes it. B2 catches that case by behaviour.

## Moved effects

| Effect | From | To | Code |
|---|---|---|---|
| Choosing where application state lives | Git top for adopters; installation for the template | `<installation>/project` for both | `metasystem/internal/stateroot/stateroot.go:145-153` |
| Keeping steward machine state | state root plus `artifacts/agents/steward` | installation plus `artifacts/agents/steward` | `metasystem/internal/stateroot/stateroot.go:297` |
| Accepting an installation's placement | root, nested at any depth, or template | `<git top>/metasystem` only | `metasystem/internal/stateroot/stateroot.go:164-213` |
| Placing an adoption | target root | `<target>/metasystem` | `metasystem/internal/adopt/adopt.go:381`, `metasystem/internal/adopt/adopt.go:386` |
| Keeping project content out of the payload | stripping after archive | archive excludes `project/` | `metasystem/internal/adopt/adopt.go:650`, `metasystem/internal/adopt/adopt.go:747` |
| Seeding goal genesis | target root | `<target>/metasystem/project` | `metasystem/internal/adopt/adopt.go:408` |
| Ignoring runtime state | target `.gitignore` | `metasystem/.gitignore` | `metasystem/internal/adopt/adopt.go:418-421` |
| Writing the root pointer files | template only | every installation | `metasystem/internal/hostsetup/setup.go:131`, `metasystem/internal/hostsetup/setup.go:228-235` |
| Classifying what an upgrade may replace | prefix rule plus root inventory | `project/` app-owned, rest generic | `metasystem/internal/stateroot/owner.go:96-160` |
| Reading the self-hosted design homes | checkout `plans/designs`, installation `plans` | state root only | `metasystem/internal/project/project.go:213-219` |
| Locking the receipts log | installation `memory/` | state root `memory/` | `metasystem/internal/receiptlog/receiptlog.go:81` |
| Reading rulings | installation or `repoRoot/memory` | state root `memory/` | `metasystem/cmd/metasystem/ui.go:768`, `metasystem/internal/dispatch/slice.go:115` |
| Finding mission contracts | Git top `plans/` | state root `plans/` | `metasystem/cmd/metasystem/intent_process.go:2337` |
| Writing a landing's receipt row | `metasystem/memory/receipts.log` | `metasystem/project/memory/receipts.log` | `metasystem/internal/goal/branch/land.go:481` |
| Naming the testing contract in Git | `metasystem/testing.json` | `metasystem/project/testing.json` | `metasystem/internal/testpolicy/contractgit/driver.go:16` |
| Copying a critique's design input | `metasystem/plans` | `metasystem/project/plans` | `metasystem/internal/launch/codex.go:118` |
| Reading metrics from Git trees | prefix plus `plans/` | prefix plus `project/plans/` | `metasystem/internal/metrics/data.go:221`, `metasystem/internal/metrics/data.go:317` |
| Stating where designs live | `design-obligation-gate.md` (adopter root `plans/designs/`) | `metasystem/project/plans/designs/` everywhere | `metasystem/docs/design/design-obligation-gate.md:34` |

## Tests

1. **stateroot.** The state root is `<installation>/project` with and without template mode. `Steward` resolves under the installation's `artifacts/`. `<git>/metasystem` resolves with `InstallationRel "metasystem"`. An installation at the Git top, or at `vendor/metasystem`, is refused with the guidance line.
2. **Owner.** `project/x` is app-owned. `artifacts/x`, `bin/x` and `local/x` are runtime. The configuration files are app-owned. `docs/x` and `skills/x` are generic. Each case is checked with a mutation that flips the rule.
3. **Adoption end to end.** Adopt into a fresh repository that already has its own `go.mod`, `cmd/` and `docs/`; it succeeds. Then run `goal add`, `design list`, `receipt add` and `question list`, each from the application root, from `metasystem/` and from a subdirectory. Every read and write lands under `metasystem/project/`. Audits B1 and B2 pass.
4. **Each moved-effect site** is driven in the new layout: the receipt lock, rulings, mission contracts, the landing receipt row, the testing-contract merge driver, the critique input copy and the metrics prefix.
5. **Audits B3 and B4.** A fixture text with a bare `docs/x.md`, a `metasystem/docs/missing.md` or a Go literal `metasystem/plans` is refused. The shipped tree passes.
6. **Self-hosted parity.** `goal list`, `design list` and `receipt status` give the same counts before and after the move in this repository.

## Step 1

Step 1 is one slice, landed as a hard cutover:

- the state-root rule and the layout rule;
- the ownership classifier;
- adoption into `metasystem/` with the payload exclusions and seeding;
- pointer files for every installation;
- the eleven bypass sites in the Moved effects table;
- the citation base sentence and the token rewrite of shipped agent texts;
- audits B1 to B4;
- the move in this repository;
- the design-home rule text.

After step 1, a person adopting into an existing application gets one folder plus two pointer files, and every agent finds its files.

## Deferred

| Item | Builds on |
|---|---|
| Migrating existing root-placed or `vendor/`-placed adopters | the `ResolveLayout` refusal and the `Owner` classes |
| A `system upgrade` verb that replaces the generic set | `Owner`'s never-touched list and audit B1 |
| An installed-SHA marker outside `project/` (needed by upgrade) | the seeded `docs/project-rules.md` marker (`adopt.go:424`) |
| `metasystem.conf` into `project/`, `.local` into `local/` | one configuration-path helper replacing the joins (`metasystem/internal/config/template.go:18` is the pattern) |
| `artifacts/` renamed to `local/` | the `Steward` kind and the `artifacts`/`local` runtime class in `Owner` |
| Go messages printing state paths relative to the repository root | `Layout.GitRoot` |
| The CWD fallback `.metasystem/` in `metasystem/internal/registry/selection.go:23` | the installation's `artifacts/` |
