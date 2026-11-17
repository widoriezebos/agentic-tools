# Apple Git mirrored-prefix experiment

Run on this laptop on 17 November 2026 (Europe/Amsterdam), starting from
`88a9cbb3e5` on `a1/real-git`. No developer-directory setting was changed.
Each subprocess received its own `DEVELOPER_DIR`. Temporary experiment
directories were inside this worktree and were removed afterward.

The mirror works for both installations. Match means the same resolved
resource, not identical path text: Git reports the namespace prefix without
resolving its resource symlinks. The guard therefore compares existing,
resolved exec-path directories. Submodule help succeeds and all 14 initialized
hook templates have identical names, bytes, and modes within each pair.

## Experiment setup

For each installation, `xcrun -f git` selected the binary below. Its real prefix
is the parent of its `bin` directory. The temporary namespace contained:

- `git-prefix/bin/git`: symlink to the selected binary; the only entry in bin.
- `git-prefix/libexec`: symlink to the installation's libexec directory.
- `git-prefix/share`: symlink to the installation's share directory.
- Neither installation has a prefix-relative `etc` directory.

All inherited `GIT_*` variables were removed, including
`GIT_CONFIG_NOSYSTEM`. HOME and XDG_CONFIG_HOME pointed at the empty experiment
directory. Both entries ran from that directory. The wrapper entry was always
`/usr/bin/git`.

Abbreviations below expand to exact paths:

| Name | Path |
| --- | --- |
| C | /Library/Developer/CommandLineTools/usr |
| X | /Applications/Xcode.app/Contents/Developer/usr |
| E | /Users/wido/agentic-tools-evidence/a1-realgit/.git-prefix-experiment-4sd9ecbo |
| MC | E/CLT/git-prefix |
| MX | E/Xcode/git-prefix |

Command Line Tools used `DEVELOPER_DIR=/Library/Developer/CommandLineTools`;
Xcode used `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer`.
`xcrun -f git` returned `C/bin/git` and `X/bin/git`, respectively.

## Recorded command outputs

All path and version commands exited 0 with empty stderr.

| Command | CLT wrapper stdout | CLT mirror stdout | Xcode wrapper stdout | Xcode mirror stdout |
| --- | --- | --- | --- | --- |
| --exec-path | C/libexec/git-core | MC/libexec/git-core | X/libexec/git-core | MX/libexec/git-core |
| --html-path | C/share/doc/git-doc | MC/share/doc/git-doc | X/share/doc/git-doc | MX/share/doc/git-doc |
| --info-path | C/share/info | MC/share/info | X/share/info | MX/share/info |
| --version | git version 2.50.1 (Apple Git-155) | git version 2.50.1 (Apple Git-155) | git version 2.54.0 (Apple Git-157) | git version 2.54.0 (Apple Git-157) |

Resolving symlinks in each of the three path outputs gave the wrapper's path
for both installations. The exec-path directories exist. HTML and info
directories are absent through both entries in both installations; resolving
their existing ancestors still gives the same missing path.

For all four entries, `git config --system --list --show-origin` exited 128,
with empty stdout and this stderr:

```text
fatal: unable to read config file '/etc/gitconfig': No such file or directory
```

For all four entries, `git var GIT_EXEC_PATH` exited 129, with empty stdout and
this stderr (the variable is unsupported by these versions):

```text
usage: git var (-l | <variable>)
```

For all four entries, `git submodule -h` exited 0, with empty stderr and this
stdout:

```text
usage: git submodule [--quiet] [--cached]
   or: git submodule [--quiet] add [-b <branch>] [-f|--force] [--name <name>] [--reference <repository>] [--] <repository> [<path>]
   or: git submodule [--quiet] status [--cached] [--recursive] [--] [<path>...]
   or: git submodule [--quiet] init [--] [<path>...]
   or: git submodule [--quiet] deinit [-f|--force] (--all| [--] <path>...)
   or: git submodule [--quiet] update [--init [--filter=<filter-spec>]] [--remote] [-N|--no-fetch] [-f|--force] [--checkout|--merge|--rebase] [--[no-]recommend-shallow] [--reference <repository>] [--recursive] [--[no-]single-branch] [--] [<path>...]
   or: git submodule [--quiet] set-branch (--default|--branch <branch>) [--] <path>
   or: git submodule [--quiet] set-url [--] <path> <newurl>
   or: git submodule [--quiet] summary [--cached|--files] [--summary-limit <n>] [commit] [--] [<path>...]
   or: git submodule [--quiet] foreach [--recursive] <command>
   or: git submodule [--quiet] sync [--recursive] [--] [<path>...]
   or: git submodule [--quiet] absorbgitdirs [--] [<path>...]
```

For all four entries, `git init -q <fresh-repository>` exited 0 with empty
stdout and stderr. The sorted `.git/hooks` listing was:

```text
applypatch-msg.sample
commit-msg.sample
fsmonitor-watchman.sample
post-update.sample
pre-applypatch.sample
pre-commit.sample
pre-merge-commit.sample
pre-push.sample
pre-rebase.sample
pre-receive.sample
prepare-commit-msg.sample
push-to-checkout.sample
sendemail-validate.sample
update.sample
```

A second isolated experiment compared SHA-256 digests and file modes for every
hook: 14 matches for CLT and 14 for Xcode.

## Implementation and failure behavior

`internal/testenv/git.go` owns mirror construction and acceptance. It publishes
only the bin directory, with libexec, share and optional etc alongside it.
Acceptance requires the same resolved, existing exec-path directory, identical
nonempty hook-template maps from init, and successful submodule help. Probe
environments remove inherited Git overrides and user template configuration.
DEVELOPER_DIR is preserved. Command errors and resource differences keep the inherited PATH and remove the
unpublished mirror. The probes add no independent wall-clock deadline. Probe repositories are removed on return.

Injected-seam tests exercise resource mismatch, missing/non-directory paths,
command failures, missing or different templates, resource directory layout,
optional etc, cleanup and fake precedence. The real-host identity test checks
the full experiment list, including system configuration lookup with
GIT_CONFIG_NOSYSTEM unset. It selected `git-prefix/bin/git` under both forced
developer directories.

## Completion obligations

| Obligation | Owner | Evidence | Status |
| --- | --- | --- | --- |
| Only Git is exposed on PATH | namespaceGitPath | unit and real-host directory assertions | Done |
| Helpers and templates keep resource identity | namespaceGitPath and probeGitResources | injected failures and CLT/Xcode identity runs | Done |
| Probe failure preserves the wrapper | namespaceGitPath | rejected paths/errors/templates keep PATH and remove the mirror | Done |
| Inherited overrides cannot conceal missing resources | gitProbeEnvironment | injected environment test; isolated real probes | Done |
| CLT lifecycle remains correct and becomes faster | process namespace | before/after lifecycle results below | Done |

## Lifecycle timing

All lifecycle runs used:

```sh
DEVELOPER_DIR=/Library/Developer/CommandLineTools /usr/bin/time -p go test ./cmd/metasystem -run '^TestIntentBuiltUnitToLanding$' -count=1 -timeout 15m
```

| Source | Result | go test package time | Command wall time |
| --- | --- | --- | --- |
| Unchanged main, 88a9cbb3e5, wrapper fallback | Pass | 557.253 s | 559.21 s |
| Initial mirror implementation | Pass | 293.182 s | 310.25 s |
| Final mirror implementation after the audit fixes | Pass | 198.218 s | 202.77 s |

The final mirror run was 356.44 seconds (63.7%) shorter in wall time than the
wrapper baseline. The earlier mirror observation is retained above; the repeat
followed removal of the independent probe deadline required by the wall-clock
audit. It did not change resource selection or the normal probe commands.
These are single observations on a shared, loaded laptop, including compilation.
Other test processes were observed on the host; this task started only one test
command at a time. Testenv retained DEVELOPER_DIR, and the real-host test logged
the mirror selected for CLT and Xcode. No xcode-select setting was changed.

## Verification

The first requested package-group run found two introduced policy violations:
an executable fake hook written with os.WriteFile, and an independent
context.WithTimeout in test-support code. The fixture now uses testexec.WriteFile;
the extra deadline was removed. Neither audit nor its allowance was weakened.
The focused rerun, including both failed audits, the guard tests and CLT identity,
passed (8.679 seconds package time), and vet passed again (0.78 seconds wall).

All final requested commands passed. They ran from metasystem with
DEVELOPER_DIR=/Library/Developer/CommandLineTools. The package group, diskstore
selection and command selection also set METASYSTEM_TESTING_WORKERS=9 without
overriding METASYSTEM_TEST_WORKERS. Package binaries were serialized with -p 1.

| Command | Exit | Wall time |
| --- | --- | --- |
| go vet ./internal/testenv/ | 0 | 0.78 s |
| go test -p 1 ./internal/testenv/ ./internal/layering/... ./internal/testpolicy/... ./internal/gittree/... ./internal/ledgerfence/... -count=1 | 0 | 216.07 s |
| go test ./internal/diskstore/ -run 'TestB3Read4' -count=1 | 0 | 8.52 s |
| go test ./cmd/metasystem -run '^(TestAudit.*\|TestFreshInitialization.*\|TestFleetBoundaryPublicLoopRevivesWithoutSession)$' -count=1 -timeout 20m | 0 | 86.76 s |

The initial package-group command exited 1 after 371.80 seconds, solely for the
two audit findings described above; its other packages passed. The final group
reran every requested package, including testenv (113.089 s), gittree (75.947 s)
and ledgerfence (5.351 s). The final command selection's package time was 67.430 s.
The real-host identity test also passed again on the final source with Xcode
forced (4.158 s package time), selecting git-prefix/bin/git.
The final CLT identity rerun also selected git-prefix/bin/git and passed
(1.381 s package time), with the same outputs recorded above.

## Limits

Neither installation contains the reported HTML/info directories or
/etc/gitconfig. The experiment proves matching lookup locations, not matching
contents of files absent from this laptop. Neither version supports
git var GIT_EXEC_PATH. The MacBook itself and other Git versions were not run.
No whole suite was run.

The receipt command reported that a repository retro is due; no instruction
changes were made as part of this fix. The required end-of-turn ready-goal read
could not read a ledger because this checkout has not fetched it. That read
did not change the scope of the requested fix or the test results above.
