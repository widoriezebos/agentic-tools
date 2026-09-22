# g1-s1 Server lifecycle: Sol's design review

Reviewer: Codex on `gpt-5.6-sol`, the design reviewer under D12, read-only, 2026-09-21. Subject: revision 5 of the [slice design](g1-s1-server-lifecycle-design.md). The report follows verbatim; the dispositions are in the [critique record](g1-s1-server-lifecycle-design-critique.md).

## 1. Verdict build after the listed changes State placement: neither, because derive it thro...
[codex] Turn completion inferred after the main thread finished and subagent work drained.
## 1. Verdict

build after the listed changes  
State placement: neither, because derive it through `stateroot.RootForInstallation(installation)`: beneath the installation when self-hosted, at the application Git root when adopted.

## 2. Findings

D1; MATERIAL; **State on disk**; the checkout placement rests on a false source premise. Current process-control code uses the installation as its lifecycle root: `resolveProcessScope` assigns `Root: installation` (`metasystem/cmd/metasystem/process_verbs.go:50-75`), `processTransition` passes that root to every family (`metasystem/cmd/metasystem/process_verbs.go:83-89`), and both steward and supervision consume `config.Root` (`metasystem/internal/stoptransition/families.go:35-47`). Ordinary `up` likewise replaces the initial checkout root with the installation before arming (`metasystem/cmd/metasystem/up.go:164-205`). The path helpers merely append to the root they receive (`metasystem/internal/steward/runner.go:32-39`, `metasystem/internal/supervise/arming.go:937-960`). Meanwhile the state-root owner is deliberately mode-aware and already includes a Steward kind (`metasystem/internal/stateroot/stateroot.go:77-120`, `metasystem/internal/stateroot/stateroot.go:247-260`). A builder would key the self-hosted lock at the outer Git checkout, while the owner resolves that workspace beneath the nested installation; using the literal installation instead would then be wrong for a nested adopted installation. Fix direction: derive the lifecycle directory from `RootForInstallation` and pass that resolved state root to `Dir`, `Serve`, `Read`, `Stop`, and `Restart`.

D2; MATERIAL; **Commands / Child arguments / Three roots**; `--repo` and `--metasystem-root` are resolved independently, with no contract requiring them to describe the same workspace. The design then tells later slices to locate subject state solely through the installation. `upRepositoryScope` returns one Git top (`metasystem/cmd/metasystem/up.go:45-50`), while `upMetasystemRoot` independently derives or accepts an installation (`metasystem/cmd/metasystem/up.go:27-43`). `RootForInstallation` returns the installation in template mode but the installation’s containing Git repository otherwise (`metasystem/internal/stateroot/stateroot.go:108-120`). Thus a builder would accept checkout A with installation B; the lifecycle record could name A while the accepted-tip refresh and later APIs read B’s state. This contradicts the workspace-binding contract at `plans/user-interface-design.md:451-461`. Fix direction: validate the canonical checkout/installation relationship once at command entry, refuse a mismatched pair, then pass the resolved checkout, installation, and state root unchanged to the child.

D3; MATERIAL; **Configuration / Conventions / Verification**; the required `UIListen(confPath, flag, flagSet)` API cannot support a hermetic in-process “default value” test under the stated test rules. `config.Get` reads `os.LookupEnv` unless `GetParams.LookupEnv` is injected (`metasystem/internal/config/resolve.go:118-143`), but the proposed wrapper exposes no such seam. `testenv.Main` clears only its enumerated controls and prefixes, which do not include `METASYSTEM_UI_LISTEN` (`metasystem/internal/testenv/testenv.go:44-67`, `metasystem/internal/testenv/testenv.go:426-460`). The design simultaneously requires every test to be parallel and forbids `os.Setenv`. A builder must guess between an ambient-environment-dependent test, a subprocess outside the stated injected-spawn convention, or an undocumented helper. Fix direction: specify an unexported resolver accepting `LookupEnv`, with exported `UIListen` supplying `os.LookupEnv` and tests supplying an empty lookup.

## 3. State placement

- **Verified:** `upRepositoryScope` returns the canonical Git top through `stateroot.RepositoryTop` (`metasystem/cmd/metasystem/up.go:45-50`, `metasystem/internal/stateroot/stateroot.go:234-237`).

- **Verified:** the default installation comes from the executable’s `<installation>/bin/metasystem` location and is validated by the presence of `metasystem.conf` (`metasystem/cmd/metasystem/up.go:27-43`).

- **Wrong:** existing process families do not generally receive the Git checkout as their lifecycle root. `processScope.Root` is the installation (`metasystem/cmd/metasystem/process_verbs.go:50-75`), and `up` explicitly changes `Options.Root` from checkout scope to installation before arming (`metasystem/cmd/metasystem/up.go:189-205`). Steward and supervision then build their paths below that supplied root (`metasystem/internal/steward/runner.go:32-39`, `metasystem/internal/supervise/arming.go:937-960`).

- **Verified:** the state-root owner resolves self-hosted state beneath the nested installation, but adopted state at the containing application repository (`metasystem/internal/stateroot/stateroot.go:108-120`, `metasystem/internal/stateroot/stateroot.go:293-299`).

- **Wrong:** “in an adopted project the checkout and installation are the same place” is true only when adoption is installed at the application root. `ResolveLayout` explicitly supports an adopted installation nested beneath a different Git root (`metasystem/internal/stateroot/stateroot.go:158-167`).

- **Wrong as a general owner claim:** the goal journal and legacy channel do not themselves resolve an installation. The journal joins its caller-provided root with `artifacts/agents/goal-transactions` (`metasystem/internal/goal/journal.go:104-126`); `goal.ResolveStateRoot` is the separate mapping that redirects a template checkout to its nested installation (`metasystem/internal/goal/project.go:266-300`). The legacy channel similarly joins its caller-provided root (`metasystem/internal/channel/question.go:80-90`), and commands such as `channel status` pass the raw `--root` (`metasystem/cmd/metasystem/channel_verbs.go:49-60`).

- **Wrong as existing-engine evidence:** calling the proposed UI server a “process family” does not establish its owner. The current aggregate family list contains mission, job, proof-run, run, steward, supervision, and untracked—no UI family (`metasystem/internal/stoptransition/families.go:35-47`).

Consequences by layout:

- **`metasystem stop` and `metasystem status`:** both construct the existing family transition from the installation root (`metasystem/cmd/metasystem/process_verbs.go:101-128`, `metasystem/cmd/metasystem/process_verbs.go:154-164`). Putting UI state under that root does not make those commands stop or report the UI server, because UI is absent from `LocalFamilies` (`metasystem/internal/stoptransition/families.go:35-47`). Under the outer checkout in self-hosted mode, the engine commands would not even inspect the same artifact tree. Independence therefore comes from excluding UI from the engine family list, not from choosing a different directory.

- **Two checkouts on one machine:** under raw checkout placement they always get separate locks. Owner-derived placement also gives separate locks when each supported checkout carries its own installation: each installation resolves to its own self-hosted installation root or adopted application root. If two arbitrary checkouts are allowed to name one shared installation, owner-derived placement would collapse their lock identities; D2 therefore requires refusing that mismatched pairing. This is an inference from the mappings above.

- **Adopted installation vendored beneath an application repository:** `templateMode` is false, so `RootForInstallation` returns the containing application Git root (`metasystem/internal/stateroot/stateroot.go:108-120`). UI state is therefore `<application>/artifacts/agents/ui/`, not `<application>/<vendored-installation>/artifacts/agents/ui/`. In this layout owner-derived and raw-checkout placement coincide.

- **Adopted installation at the application repository root:** installation, Git checkout, and resolved state root coincide, so both disputed spellings produce the same directory (`metasystem/internal/stateroot/stateroot.go:111-120`). There is no lock-identity difference.

- **Self-hosted layout:** the Git checkout is the outer repository, while `templateMode` makes the state root the nested `metasystem/` installation (`metasystem/internal/stateroot/stateroot.go:111-120`, `metasystem/internal/stateroot/stateroot.go:293-299`). The correct path is therefore `metasystem/artifacts/agents/ui/`.

## 4. Not checked

- No commands, scripts, Go builds, tests, or server processes were run, as required.
- Darwin and Linux runtime behavior of `flock`, `Setsid`, and pipe deadlines was not exercised. Source inspection found no additional design-level choice requiring a builder to diverge, but runtime portability remains unproved.
- `metasystem/metasystem.conf.local` and `.claude/worktrees/g1-s1` were not read.
- The pre-review implementation on `ui/g1-s1` was not inspected or used as evidence.
