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

This page folds three critique rounds (Codex Astra: 10, 4, then 2 material findings). R1-05 and R1-06 are answered by deferring this repository's move to step 2. The root-mixing class (R2-02 to R2-04, R3-02) is answered by giving the two roots distinct Go types, so the compiler finds every crossing. Everything else, including R3-01, is in step 1.

## Threat model and rabbit-hole risks (read before critiquing)

This is the template default. Our own agents and operators make mistakes: a seat writes state to the old path, a text cites a bare path, an upgrade copies over a project file. Nobody attacks.

| Rabbit hole | Mitigation |
|---|---|
| A migration framework for every past layout | Step 1 covers fresh adoptions only. This repository's move is step 2. Existing adopters are deferred, and there is no fallback: old adopted layouts are refused with one line of guidance. |
| Making every path configurable | One fixed folder name, `metasystem`, at the repository root. No new key is added. |
| Moving class 4 off the repository | Class 4 stays in `metasystem/artifacts/`, ignored by Git. |
| Redesigning root resolution beyond the one rule | Only the state-root rule changes. The installation is still found from `bin/..`. |
| Patching root-mixing call sites one finding at a time | One boundary rule, and an audit that lists every violation for the build. |
| Touching unrelated docs | Only texts that cite paths are rewritten, and only their path tokens. |
| Moving the configuration files, or renaming `artifacts/` to `local/` | Deferred. `metasystem.conf` has 169 Go joins and `artifacts` has 403 (*grep counts*). Both stay where they are, listed by name as never touched. |
| Rewriting the ~120 Go messages that print state paths | Deferred. Step 1 fixes the texts agents route by. |
| Moving the runtime discovery files into the folder | Impossible. Runtimes read `.claude/`, `.codex/`, `.agents/` and `.devin/` where they start, and GitHub reads `.github/` at the root. These stay generated. |

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
      memory/ records/ testing.json launch.json covenant
      docs/intent/ docs/doctrine/ docs/decisions/ docs/project-rules.md
    local/                      reserved, ignored, never shipped
```

This repository keeps its current shape until step 2, when it takes exactly this one.

## The boundary: two roots, two owners

**The rule.** Project state (the goal ledger and goals, plans, designs, reviews, records, memory registers, intent, doctrine, decisions, the covenant, and the testing and launch contracts) is reached only through the state-root owner, `internal/stateroot`. Installation things (the engine binary, `metasystem.conf` and its `.local` file, commit-fence enrollment, skills, docs, templates, runtime discovery and hooks) are reached only through the installation root. No caller derives either root from the other. No code assumes the two are equal, and no code joins a state segment onto the installation.

**The owners.**

- **State root:** `stateroot.RootForInstallation(installation)` and `stateroot.StateRoot(kind)` (`metasystem/internal/stateroot/stateroot.go:120-153`). For an adopted installation it is `<installation>/project`. For the template it stays the installation until step 2, which deletes that branch. Project paths inside Git trees are built from the state root's repository-relative path.
- **Installation root:** the executable's installation, `bin/..` holding `metasystem.conf` (`stateroot.go:304-320`, exported as `stateroot.ExecutableInstallation()`). For a checkout named explicitly, the owners are `Layout.InstallationRoot` from `stateroot.ResolveLayout` and `stateroot.RootForCandidate`.
- **Machine state:** `Steward` (`stateroot.go:297`) resolves under the installation's `artifacts/`.

**Layout.** For a non-template installation, `ResolveLayout` (`stateroot.go:164-213`) accepts only `<git top>/metasystem`. That gives `RepositoryRoot = GitRoot` and `InstallationRel = "metasystem"`. Anything else is refused:

> This installation uses an older layout. Adopt the current MetaSystem into a clean repository, or keep the engine it came with.

**Contract defaults.** `testing.contract` and `launch.contract` default to `project/testing.json` and `project/launch.json` (`metasystem/internal/config/defaults.go:65-67`). Registration's own `testing.json` fallback goes (`metasystem/internal/hookswitch/switch.go:113-122`). This repository sets both keys back to the old names until step 2.

## Two root types: the compiler enforces the boundary

**Types.** `stateroot` defines `type Installation string` and `type State string`. Only the resolver's constructors produce them: the installation owners above return `Installation`, and `RootForInstallation` returns `State`. Configuration readers, `ledgerfence.Ensure`, hooks and runtime discovery take `Installation`. The goal store, registers, records, the covenant check, the contract readers and landing take `State`. Structs such as `lifecycle.Roots`, `processScope` and `adapter.ToolGateOptions` carry typed fields. A crossing like `Installation: stateRoot` (`metasystem/cmd/metasystem/adapter_runtime_verbs.go:62`) is then a compile error.

**The build's first act** converts the signatures, and the compiler then lists every site to fix. Examples known now, not a complete list: fence enrollment given the state root (`metasystem/cmd/metasystem/intent_operations.go:372`), any directory named `metasystem` taken as the state root (`metasystem/internal/goal/project.go:333`), the covenant search skipping the state root (`metasystem/cmd/metasystem/intent_process.go:1779`), and the tool hook above.

**The conversion rule.** A typed root becomes a string path only through its own method, `Path(segments ...string) string`, and only at the call that touches the filesystem, Git or a child process. That string is never stored back into a root-typed value. Where a string re-enters, for example a `--root` flag of a child process or a JSON field, it goes back through `stateroot.ParseInstallation` or `stateroot.ParseState`, which check the shape. A plain cast such as `State(x)` or `Installation(x)` outside `stateroot` and its tests is never legitimate.

**B5 shrinks to what types cannot see.** It is a static-gate scan that refuses `State(` and `Installation(` casts outside `stateroot`, and the literals `metasystem/memory`, `metasystem/plans`, `metasystem/records` and `metasystem/testing.json`, which are Git-tree paths that bypass the state root. Its limit: a `Path()` string handed to the wrong consumer as a plain string is still invisible to it. Consumers therefore take typed roots, never a pre-joined string, and B2 catches what remains by behaviour.

**Landing classification (R3-01).** Ownership and landing policy are separate. `project/**` stays app-owned, so an upgrade never replaces it. Landing, however, no longer treats an app-owned path under the state root's repository path as `Outside` (`metasystem/internal/pathclass/pathclass.go:232`). The existing record and ledger rules classify it, keyed relative to the state root as the template's are keyed relative to the installation. The existing append-only comparisons use the same key (`metasystem/internal/landing/observe.go:1118`, `:1162`; `metasystem/internal/landing/landpath/commit.go:605`).

**Other boundary checks.**

- **B1, payload:** in `metasystem audit` (`metasystem/internal/audit/metasystem.go:74`). The staged payload holds nothing under `project/`, `local/`, `artifacts/` or `bin/`, and no `launch.json`, `testing.json` or `*.local` file.
- **B2, shape:** an end-to-end adoption test in the static selection, not the production audit, because adoption calls that audit (`metasystem/internal/adopt/adopt.go:444-453`). The top level changes only by `metasystem/`, the pointer files and the registrations. After a goal-and-design round, `git status` shows changes only under `metasystem/project/`.
- **B3, citations:** every backticked path in a shipped agent text starts with `metasystem/`, and either exists in the shipped tree or is in the seed list for `metasystem/project/`. This replaces the hardcoded list of 32 paths (`metasystem/internal/audit/shipped_installation_test.go:109`).

## The upgrade rule

- **Replaced:** every path inside `metasystem/` that `stateroot.Owner` classes as `metasystem-generic`.
- **Never touched:** `project/**`, `local/**`, `artifacts/**`, `metasystem.conf` and `metasystem.conf.local`.
- **Rebuilt:** `bin/`.
- **Enforced by:** the adopted branch of the classifier (`metasystem/internal/stateroot/owner.go:96-160`), which becomes the single list. `project/` and the configuration files are app-owned; `artifacts/`, `bin/` and `local/` are runtime; the rest is generic; the root-inventory branch goes. B1 keeps never-touched paths out of the payload. The upgrade verb itself is deferred.

## How adoption creates the layout

- **Payload.** The payload is `git archive HEAD:metasystem` with today's history stripping kept as it is (`metasystem/internal/adopt/adopt.go:649-664`, `:747`). On top of that, `project/`, `local/`, `artifacts/`, `bin/`, `launch.json`, `testing.json` and the `.local` file are excluded. The stripping is still needed because this repository's history stays outside `project/` until step 2.
- **Configuration.** The staged configuration drops the template's two contract keys, `testing.contract` and `launch.contract`, alongside the template-mode line (`metasystem/internal/adopt/adopt.go:723-740`), so an adopter gets the `project/` defaults (R2-01).
- **Placement.** The payload is copied to `<target>/metasystem/`, with the engine at `metasystem/bin/metasystem`.
- **Seeding.** `metasystem/project/` is seeded once with goal genesis, the empty registers and READMEs, `plans/goals.md`, an incomplete `testing.json`, and `docs/project-rules.md` with the SHA marker.
- **Ignore rules.** `artifacts/` and its ignore line go inside `metasystem/`.
- **Instruction files.** `AGENTS.md` and `CLAUDE.md` leave the blanket preflight refusal list (`adopt.go:73-76`, used at `:528-532`). The managed-block merger (`metasystem/internal/hostsetup/setup.go:237-266`) keeps the application's text. `collide()` looks only inside `metasystem/`.
- **CI.** The generated workflow uses `go-version-file: metasystem/go.mod`, builds with `cd metasystem && go build -o bin/metasystem ./cmd/metasystem`, and runs `metasystem/bin/metasystem test run` (`metasystem/internal/adopt/github-actions-metasystem.yml:13-21`).
- **Pointer files.** `system setup` writes the managed block for every installation, not only the template (`setup.go:131`):
  > MetaSystem is installed in `metasystem/`. Its agent contract is `metasystem/AGENTS.md`; route with `metasystem/wow.md`. This application's plans, decisions and records are in `metasystem/project/`. Paths in MetaSystem's texts are relative to this repository's root.

  They are files, not a folder. Claude Code, Codex and Devin discover instructions only as `CLAUDE.md` or `AGENTS.md` in the directory they start in. A pointer file is the least that works, adds no folder, and keeps the application's own text.

## Citing files from any working directory

- **One base for agent-facing texts:** the repository root of the checkout or worktree. MetaSystem's files are cited as `metasystem/...`, the application's as `metasystem/project/...`. This matches Git paths, the return schemas and the testing contract.
- **Rewritten token by token:** the bare paths in `AGENTS.md`, `wow.md`, the role files, the brief templates and every shipped `SKILL.md` (including where inception writes the covenant), and the `R0/metasystem` base in `design-common.md`. The base sentence opens `metasystem/AGENTS.md` and `metasystem/wow.md`. The three dangling design-critique citations become prose.
- Until step 2, this repository's template-only pointer line says: "Until this repository's move, `metasystem/project/X` is at `metasystem/X`."

## Moved effects

| Effect | From | To | Code |
|---|---|---|---|
| Reaching project state or installation things | plain strings that cross roots | typed `State` and `Installation`; the compiler lists every site | `metasystem/cmd/metasystem/intent_operations.go:372`, `metasystem/internal/goal/project.go:333`, `metasystem/cmd/metasystem/intent_process.go:1779`, `metasystem/cmd/metasystem/adapter_runtime_verbs.go:62` |
| Classifying an adopter's record and ledger landings | app-owned means `Outside` | record and ledger rules keyed on the state root | `metasystem/internal/pathclass/pathclass.go:232`, `metasystem/internal/landing/observe.go:1118` |
| Choosing where adopted state lives | Git top | `<installation>/project` | `metasystem/internal/stateroot/stateroot.go:145-153` |
| Keeping steward machine state | state root plus `artifacts/agents/steward` | installation plus `artifacts/agents/steward` | `metasystem/internal/stateroot/stateroot.go:297` |
| Accepting an adopted installation's placement | root or nested at any depth | `<git top>/metasystem` only | `metasystem/internal/stateroot/stateroot.go:164-213` |
| Defaulting the testing and launch contracts | installation `testing.json`, `launch.json`; registration fallback | `project/testing.json`, `project/launch.json` | `metasystem/internal/config/defaults.go:65-67`, `metasystem/internal/hookswitch/switch.go:113-122` |
| Shipping the template's contract overrides | kept in the adopter's config | dropped with template mode | `metasystem/internal/adopt/adopt.go:723-740` |
| Placing an adoption | target root | `<target>/metasystem` | `metasystem/internal/adopt/adopt.go:381`, `metasystem/internal/adopt/adopt.go:386` |
| Ignoring runtime state | target `.gitignore` | `metasystem/.gitignore` | `metasystem/internal/adopt/adopt.go:418-421` |
| Treating existing instruction files | blanket preflight refusal | managed-block merge | `metasystem/internal/adopt/adopt.go:73-76`, `metasystem/internal/adopt/adopt.go:528-532` |
| Building and running the engine in CI | root `go.mod`, root `bin/metasystem` | `metasystem/go.mod`, `metasystem/bin/metasystem` | `metasystem/internal/adopt/github-actions-metasystem.yml:13-21` |
| Writing the root pointer files | template only | every installation | `metasystem/internal/hostsetup/setup.go:131` |
| Classifying what an upgrade may replace | prefix rule plus root inventory | `project/` app-owned, rest generic | `metasystem/internal/stateroot/owner.go:96-160` |
| Running the shape check | the production audit, as drafted | end-to-end adoption test | `metasystem/internal/adopt/adopt.go:444-453` |
| Stating where an adopter's designs live | the application's root `plans/designs/` | `metasystem/project/plans/designs/` | `metasystem/docs/design/design-obligation-gate.md:34` |

## Tests

1. **stateroot.** The adopted state root is `<installation>/project`; the template's is unchanged. `Steward` resolves under `artifacts/`. Adopted installations at the Git top or at `vendor/metasystem` are refused.
2. **Owner.** Each class, checked with a mutation that flips it.
3. **Types and B5.** A cast outside `stateroot` and a literal `metasystem/plans` are each refused. `ParseState` refuses a path without the state shape.
4. **Separated-root fixtures.** On a fresh adopter, a receipt-only landing succeeds and a receipt rewrite refuses. The tool hook applies non-default context thresholds from `metasystem.conf`.
5. **Adoption end to end (B2).** Adopt into a target that already has `go.mod`, `cmd/`, `docs/`, and populated `AGENTS.md` and `CLAUDE.md`, and use the configuration adoption produces. Then run, in order: `goal sync --upgrade`; `goal open`; `goal budget G norm` with a non-default tier budget; Stop with populated goals; `system check` with a covenant written as inception directs; `settings check` and test selection against a completed `project/testing.json`; `app` against `project/launch.json`; the Application page with populated known issues; `design list` and `receipt add` from the application root, from `metasystem/` and from a subdirectory; and the workflow's three commands. Every state read and write lands under `metasystem/project/`, and the application's text in the pointer files survives.
6. **B1 and B3.** A payload containing `launch.json`, and a text with a bare `docs/x.md`, are each refused. The shipped tree passes.

## Step 1

Step 1 is fresh adoption in the one-folder layout, plus the engine rules it needs. It covers the adopted state-root and layout rules; the two root types, with the signatures converted first and every site the compiler lists fixed, and the shrunken B5; landing classification keyed on the state root; the contract defaults, and the dropped contract overrides; the ownership classifier; adoption into `metasystem/`, with its exclusions, seeding, instruction-file merge and CI workflow; pointer files for every installation; the citation rewrite; and B1, B3 and the B2 test. This repository keeps its layout.

## Deferred

**Step 2: this repository's own move.** It is a separate decision, taken after step 1 works.

- In one `git mv` commit, with the lane paused, `metasystem/{plans,memory,records}`, `docs/{intent,doctrine,decisions,project-rules.md}`, `testing*.json`, `launch.json` and the root `plans/**` (no name collides) move under `metasystem/project/`. The `.gitattributes` union lines gain `project/`.
- Then the template branch of `RootForInstallation`, the second design homes (`metasystem/internal/project/project.go:213-219`), this repository's contract keys and adoption's history stripping are deleted.

Two prerequisites must be specified and proven before step 2 lands:

- **R1-05:** an identity-preserving transition of the accepted goal-ledger ref to the moved tree. The next normal fetch must validate after it (`metasystem/internal/goal/fetchadvance.go:100-129`).
- **R1-06:** the record and ledger landing classifications, and their carriage path comparisons, move with the files (`metasystem/internal/pathclass/path-classes.txt:28-42`, `metasystem/internal/landing/observe.go:1074-1124`). A receipt-only landing is exercised after the move.

Parity: `goal list`, `design list` and `receipt status` give the same counts before and after the move.

| Item | Builds on |
|---|---|
| Step 2, this repository's move (above) | R1-05 and R1-06; the step-1 root types |
| Migrating existing root-placed or `vendor/`-placed adopters | the `ResolveLayout` refusal and the `Owner` classes |
| A `system upgrade` verb | `Owner`'s never-touched list and B1 |
| An installed-SHA marker outside `project/` | the seeded marker (`metasystem/internal/adopt/adopt.go:424`) |
| `metasystem.conf` into `project/`, `.local` into `local/` | configuration readers typed `Installation` |
| `artifacts/` renamed to `local/` | the `Steward` kind and the runtime class in `Owner` |
| Go messages printing state paths relative to the repository root | `Layout.GitRoot` |
| The CWD fallback `.metasystem/` (`metasystem/internal/registry/selection.go:23`) | the installation's `artifacts/` |
