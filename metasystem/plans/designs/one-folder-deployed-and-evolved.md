# One folder: what MetaSystem deploys, apart from what the application evolves

- Kind: design
- Id: 01M3YG08QFSF1W2HW3RN8WXWMZ
- Status: draft
- Goals: one-folder-deployed-and-evolved

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

Round 1 of the critique (Codex Astra, 10 material findings) is folded below. R1-05 and R1-06 are answered by deferring this repository's own move to step 2. The other eight are part of step 1.

## Threat model and rabbit-hole risks (read before critiquing)

This is the template default. Our own agents and operators make mistakes: a seat writes state to the old path, a text cites a bare path, an upgrade copies over a project file. Nobody attacks.

| Rabbit hole | Mitigation |
|---|---|
| A migration framework for every past layout | Step 1 covers fresh adoptions only. This repository's move is step 2, a separate decision. Existing adopters are deferred. The new engine refuses old adopted layouts with one line of guidance. |
| Making every path configurable | One fixed folder name, `metasystem`, at the repository root. No new key is added. |
| Moving class 4 off the repository | Class 4 stays in `metasystem/artifacts/`, ignored by Git. |
| Redesigning root resolution beyond the one rule | Only the state-root rule changes. Locating the installation from `bin/..` stays as it is, and configuration keeps being read at the installation. |
| Touching unrelated docs | Only texts that cite paths are rewritten, and only their path tokens. |
| Moving the configuration files in step 1 | 169 Go sites join `metasystem.conf` onto the installation (*grep count*). The files stay where they are and are listed by name as never touched. Moving them is deferred. |
| Renaming `artifacts/` to `local/` in step 1 | `artifacts` appears at 403 Go sites. `local/` is reserved, and the rename is deferred. |
| Rewriting the engine's ~120 Go messages that print state paths | Deferred. Step 1 fixes the texts agents route by. |
| Keeping a fallback for old adopted layouts | None. Mixed engine versions are parked. |
| Moving the runtime discovery files into the folder | Impossible. Runtimes read `.claude/`, `.codex/`, `.agents/` and `.devin/` at the launch directory, and GitHub reads `.github/` at the root. These stay generated. |

## The layout

```
<app>/
  AGENTS.md, CLAUDE.md          pointer files (managed block; app text kept)
  .claude/ .agents/ .codex/ .devin/ .github/workflows/metasystem.yml   generated registrations
  metasystem/                   class 1, replaced as a whole on upgrade
    AGENTS.md wow.md docs/ skills/ cmd/ internal/ go.mod bin/ ...
    metasystem.conf             class 3, committed (stays here for now)
    metasystem.conf.local       class 3, ignored (stays here for now)
    artifacts/                  class 4, ignored (becomes local/ later)
    project/                    classes 2+3, committed, never shipped
      plans/ (goals/, designs/, reviews/, briefs, goals.md)
      memory/ records/ testing.json launch.json
      docs/intent/ docs/doctrine/ docs/decisions/ docs/project-rules.md
    local/                      reserved, ignored, never shipped
```

This repository keeps its current shape until step 2, when it takes exactly this one.

## How the engine resolves roots after the change

- **Installation root:** unchanged. It is `bin/..` of the running executable and must hold `metasystem.conf` (`metasystem/internal/stateroot/stateroot.go:304-320`).
- **Configuration:** always read at the installation root. A caller that hands the state root to a configuration reader is a defect (R1-04). Budget calls (`metasystem/cmd/metasystem/intent_goals.go:509-514`) pass the state root for ledger operations and the installation for `metasystem.conf`.
- **State root, the one rule:** for an adopted installation, the state root is `<installation>/project`. `RootForInstallation` (`stateroot.go:145-153`) replaces its `repositoryTop` branch with this rule. The template branch, which returns the installation itself, stays until step 2 and is deleted there.
- **Project paths in Git trees:** code that names a project path in a Git tree (landing rows, metrics, the testing-contract driver) builds it from the state root's path relative to the repository: `metasystem/project` when adopted, `metasystem` in the template. It never uses a literal.
- **Machine state:** `Steward` (`stateroot.go:297`) resolves against the installation, so it stays in `metasystem/artifacts/agents/steward`.
- **Layout:** for a non-template installation, `ResolveLayout` (`stateroot.go:164-213`) accepts only `<git top>/metasystem`. That gives `RepositoryRoot = GitRoot` and `InstallationRel = "metasystem"`. An installation at the Git top, or at any other depth, is refused:
  > This installation uses an older layout. Adopt the current MetaSystem into a clean repository, or keep the engine it came with.
- **Contract defaults:** the defaults of `testing.contract` and `launch.contract` become `project/testing.json` and `project/launch.json`, still relative to the installation (`metasystem/internal/config/defaults.go:65-67`).
  - This repository sets both keys back to the old names in its `metasystem.conf` until step 2.
  - Registration drops its own `testing.json` fallback and reads the configured value (`metasystem/internal/hookswitch/switch.go:113-122`).

## The upgrade rule

- **Replaced:** every path inside `metasystem/` that `stateroot.Owner` classes as `metasystem-generic`.
- **Never touched:** `project/**` (and with it `testing.json` and `launch.json`), `local/**`, `artifacts/**`, `metasystem.conf` and `metasystem.conf.local`.
- **Rebuilt:** `bin/`.
- **Enforced by:**
  - The adopted branch of the classifier (`metasystem/internal/stateroot/owner.go:96-160`) becomes the single list. `project/` is app-owned. `artifacts/`, `bin/` and `local/` are runtime. The two configuration files are app-owned. The rest of the installation is generic. The root-inventory branch (`owner.go:110-160`) goes, because no adopted installation sits at the root any more.
  - The payload never contains a never-touched path (audit B1). This is the preservation check.
  - The upgrade verb itself is deferred.

## How adoption creates the layout

- **Payload.** The payload is `git archive HEAD:metasystem` minus `project/`, `local/`, `artifacts/`, `bin/`, `launch.json`, `testing.json` and the `.local` file. The template's own project files are therefore never shipped (R1-01). The stripping code (`metasystem/internal/adopt/adopt.go:650`, `:684-687`, `:747`) is deleted.
- **Placement.** The payload is copied to `<target>/metasystem/`. `bin/metasystem` goes to `<target>/metasystem/bin/`.
- **Seeding `metasystem/project/`, once:**
  - The empty registers and READMEs, `plans/goals.md`, an incomplete `testing.json`, and `docs/project-rules.md` with the SHA marker.
  - Goal genesis gets two roots (R1-02). The goal store takes `metasystem/project`. The commit-fence enrollment stays at `metasystem/`, because it needs `<root>/bin/metasystem` (`metasystem/cmd/metasystem/intent_adopt.go:194-202`, `metasystem/internal/ledgerfence/fence.go:36-46`).
- **Ignore rules.** `artifacts/` and its ignore line go inside `metasystem/` (`adopt.go:418-421`).
- **Instruction files (R1-08).**
  - `AGENTS.md` and `CLAUDE.md` leave the blanket preflight refusal list (`metasystem/internal/adopt/adopt.go:73-76`, used at `:528-532`).
  - The managed-block merger (`metasystem/internal/hostsetup/setup.go:237-266`) keeps the application's existing text and adds the block.
  - `collide()` now looks only inside `metasystem/`. An application's own `go.mod`, `cmd/` and `docs/` no longer collide.
- **CI workflow (R1-07).** The generated GitHub workflow takes `go-version-file: metasystem/go.mod`, builds with `cd metasystem && go build -o bin/metasystem ./cmd/metasystem`, and runs `metasystem/bin/metasystem test run` (`metasystem/internal/adopt/github-actions-metasystem.yml:13-21`).
- **Pointer files.** `system setup` writes the managed block for every installation, not only the template (`setup.go:131`). The block reads:
  > MetaSystem is installed in `metasystem/`. Its agent contract is `metasystem/AGENTS.md`; route with `metasystem/wow.md`. This application's plans, decisions and records are in `metasystem/project/`. Paths in MetaSystem's texts are relative to this repository's root.

  The `development/` line stays template-only (`setup.go:228-235`).
- **Why files and not a folder:** Claude Code, Codex and Devin discover instructions only as a file named `CLAUDE.md` or `AGENTS.md` in their start directory. A pointer file is the least that works, it adds no folder, and it keeps the application's own text.

## Citing files from any working directory

- **One base for agent-facing texts:** the repository root of the checkout or worktree. MetaSystem's files are cited as `metasystem/...` and the application's as `metasystem/project/...`. This matches Git paths, the return schemas (`^metasystem/.+`) and the testing contract.
- **What gets rewritten, token by token:**
  - the bare paths in `AGENTS.md`, `wow.md`, the role files, the brief templates and every shipped `SKILL.md`;
  - the `R0/metasystem` base in `design-common.md`.
- The base sentence opens `metasystem/AGENTS.md` and `metasystem/wow.md`. Commands that must run inside the installation say so: "`cd metasystem && go run ./cmd/devgate static`".
- The design-critique skill's three dangling citations become plain prose.
- **This repository until step 2.** Its template-only pointer line adds: "Until this repository's move, `metasystem/project/X` is at `metasystem/X`."

## The audit that keeps the boundary

- **B1, payload:** added to `metasystem audit` (`metasystem/internal/audit/metasystem.go:74`). The staged payload holds nothing under `project/`, `local/`, `artifacts/` or `bin/`, and no `launch.json`, `testing.json` or `*.local` file.
- **B2, shape (R1-09):** this is not part of the production audit. Adoption already calls that audit (`metasystem/internal/adopt/adopt.go:444-453`), so a fresh adoption inside the audit would loop. B2 is instead an end-to-end adoption test in the static selection. A fresh adoption must change the top level only by `metasystem/`, the two pointer files and the registrations. After one goal-and-design round, `git status` must show changes only under `metasystem/project/`.
- **B3, citations:** every backticked path in a shipped agent text starts with `metasystem/`. It either exists in the shipped tree or is in the seed list for `metasystem/project/`. This replaces the hardcoded list of 32 paths (`metasystem/internal/audit/shipped_installation_test.go:109`).
- **B4, engine:** outside `internal/stateroot`, no non-test Go file contains the literals `metasystem/memory`, `metasystem/plans`, `metasystem/records` or `metasystem/testing.json`. A join built in pieces escapes this check; B2 catches it by behaviour.

## Moved effects

| Effect | From | To | Code |
|---|---|---|---|
| Choosing where adopted state lives | Git top | `<installation>/project` | `metasystem/internal/stateroot/stateroot.go:145-153` |
| Keeping steward machine state | state root plus `artifacts/agents/steward` | installation plus `artifacts/agents/steward` | `metasystem/internal/stateroot/stateroot.go:297` |
| Accepting an adopted installation's placement | root or nested at any depth | `<git top>/metasystem` only | `metasystem/internal/stateroot/stateroot.go:164-213` |
| Reading a goal's budget configuration | the state root passed as the configuration root | the installation's `metasystem.conf` | `metasystem/cmd/metasystem/intent_goals.go:509-514`, `metasystem/internal/config/budget.go:256-261` |
| Locating the testing contract | installation `testing.json`; registration's own fallback | `project/testing.json` default; one reader | `metasystem/internal/config/defaults.go:65`, `metasystem/internal/hookswitch/switch.go:113-122` |
| Locating the launch contract | installation `launch.json`, shippable | `project/launch.json`, never shipped | `metasystem/internal/config/defaults.go:67`, `metasystem/cmd/metasystem/app.go:49-69` |
| Placing an adoption | target root | `<target>/metasystem` | `metasystem/internal/adopt/adopt.go:381`, `metasystem/internal/adopt/adopt.go:386` |
| Keeping project content out of the payload | stripping after archive | archive excludes project files | `metasystem/internal/adopt/adopt.go:650`, `metasystem/internal/adopt/adopt.go:747` |
| Seeding goal genesis | target root for store and fence | store at `metasystem/project`, fence at `metasystem/` | `metasystem/cmd/metasystem/intent_adopt.go:194-202`, `metasystem/internal/ledgerfence/fence.go:36-46` |
| Ignoring runtime state | target `.gitignore` | `metasystem/.gitignore` | `metasystem/internal/adopt/adopt.go:418-421` |
| Treating existing instruction files | blanket preflight refusal | managed-block merge | `metasystem/internal/adopt/adopt.go:73-76`, `metasystem/internal/adopt/adopt.go:528-532` |
| Building and running the engine in CI | root `go.mod`, root `bin/metasystem` | `metasystem/go.mod`, `metasystem/bin/metasystem` | `metasystem/internal/adopt/github-actions-metasystem.yml:13-21` |
| Writing the root pointer files | template only | every installation | `metasystem/internal/hostsetup/setup.go:131`, `metasystem/internal/hostsetup/setup.go:228-235` |
| Classifying what an upgrade may replace | prefix rule plus root inventory | `project/` app-owned, rest generic | `metasystem/internal/stateroot/owner.go:96-160` |
| Running the shape check | the production audit, as drafted | end-to-end adoption test, not the production audit | `metasystem/internal/adopt/adopt.go:444-453` |
| Locking the receipts log | installation `memory/` | state root `memory/` | `metasystem/internal/receiptlog/receiptlog.go:81` |
| Reading rulings | installation or `repoRoot/memory` | state root `memory/` | `metasystem/cmd/metasystem/ui.go:768`, `metasystem/internal/dispatch/slice.go:115` |
| Reading known issues and linking to them | installation `memory/` | `roots.StateRoot` | `metasystem/cmd/metasystem/ui.go:780-783`, `metasystem/cmd/metasystem/ui.go:1312-1313` |
| Finding mission contracts | Git top `plans/` | state root `plans/` | `metasystem/cmd/metasystem/intent_process.go:2337` |
| Writing a landing's receipt row | literal `metasystem/memory` | state root's repository path | `metasystem/internal/goal/branch/land.go:481` |
| Naming the testing contract in Git | literal `metasystem/testing.json` | state root's repository path | `metasystem/internal/testpolicy/contractgit/driver.go:16` |
| Copying a critique's design input | literal `metasystem/plans` | state root `plans/` | `metasystem/internal/launch/codex.go:118` |
| Reading metrics from Git trees | prefix plus `plans/` | state root's repository path | `metasystem/internal/metrics/data.go:221`, `metasystem/internal/metrics/data.go:317` |
| Stating where an adopter's designs live | the application's root `plans/designs/` | `metasystem/project/plans/designs/` | `metasystem/docs/design/design-obligation-gate.md:34` |

## Tests

1. **stateroot.** An adopted installation's state root is `<installation>/project`; the template's is unchanged. `Steward` resolves under `artifacts/`. `<git>/metasystem` resolves with `InstallationRel "metasystem"`. An adopted installation at the Git top or at `vendor/metasystem` is refused with the guidance line.
2. **Owner.** `project/x` is app-owned. `artifacts/x`, `bin/x` and `local/x` are runtime. The configuration files are app-owned. `docs/x` is generic. Each case has a mutation that flips it.
3. **Adoption end to end (B2).**
   - The target already has `go.mod`, `cmd/`, `docs/`, and populated `AGENTS.md` and `CLAUDE.md`. Adoption succeeds, and the application's text survives around the managed block.
   - Genesis enrolls the fence with `metasystem/bin/metasystem`.
   - `goal add`, `design list`, `receipt add` and `question list` are each run from the application root, from `metasystem/` and from a subdirectory, and every one lands under `metasystem/project/`.
   - The generated workflow's three commands run green in the target.
4. **Configuration cutover.** `metasystem.conf` sets a tier budget that differs from the compiled default, and `goal budget G norm` applies it. A completed `project/testing.json` is read by `settings check` and by test selection. `project/launch.json` is what `app` reads.
5. **Each moved-effect site** is driven in the new layout: the receipt lock, rulings, the Application page with a populated known-issues register, mission contracts, the landing receipt row, the merge driver, the critique input copy and the metrics prefix.
6. **Audits B1, B3 and B4.** A payload containing `launch.json`, a text with a bare `docs/x.md`, and a Go literal `metasystem/plans` are each refused. The shipped tree passes.

## Step 1

Step 1 is fresh adoption in the one-folder layout, plus the engine rules that adoption needs:

- the adopted state-root and layout rules;
- configuration read at the installation;
- the contract defaults;
- the ownership classifier;
- adoption into `metasystem/`, with its exclusions, seeding, two-root genesis, the instruction-file merge and the CI workflow;
- pointer files for every installation;
- the bypass sites in the Moved effects table;
- the citation rewrite;
- audits B1, B3 and B4, plus the B2 test.

This repository keeps its layout. After step 1, a person adopting into an existing application gets one folder plus two pointer files, and every agent finds its files.

## Deferred

**Step 2: this repository's own move.** It is a separate decision, taken after step 1 works. Its plan:

- In one `git mv` commit, with the lane paused, `metasystem/{plans,memory,records}`, `docs/{intent,doctrine,decisions,project-rules.md}`, `testing*.json`, `launch.json` and the root `plans/**` (no name collides) move under `metasystem/project/`; the `.gitattributes` union lines gain `project/`.
- Afterwards the template branch of `RootForInstallation`, the second design homes (`metasystem/internal/project/project.go:213-219`) and this repository's contract keys are deleted.

Two prerequisites must be specified and proven before step 2 lands:

- **R1-05:** an identity-preserving transition of the accepted goal-ledger ref to the moved tree. The next normal fetch must validate after it (`metasystem/internal/goal/fetchadvance.go:100-129`).
- **R1-06:** the record and ledger landing classifications, and their carriage path comparisons, move with the files (`metasystem/internal/pathclass/path-classes.txt:28-42`, `metasystem/internal/landing/observe.go:1074-1124`). A receipt-only landing is exercised after the move.

The parity test for step 2: `goal list`, `design list` and `receipt status` give the same counts before and after the move.

| Item | Builds on |
|---|---|
| Step 2, this repository's move (above) | R1-05 and R1-06 as prerequisites; the step-1 state-root rule |
| Migrating existing root-placed or `vendor/`-placed adopters | the `ResolveLayout` refusal and the `Owner` classes |
| A `system upgrade` verb | `Owner`'s never-touched list and audit B1 |
| An installed-SHA marker outside `project/` | the seeded `docs/project-rules.md` marker (`metasystem/internal/adopt/adopt.go:424`) |
| `metasystem.conf` into `project/`, `.local` into `local/` | configuration read at the installation (one place to change) |
| `artifacts/` renamed to `local/` | the `Steward` kind and the runtime class in `Owner` |
| Go messages printing state paths relative to the repository root | `Layout.GitRoot` |
| The CWD fallback `.metasystem/` (`metasystem/internal/registry/selection.go:23`) | the installation's `artifacts/` |
