Working Mode: build
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal fixture-stewards-reach-the-desktop)
Date: 2026-09-06

# Goal

Goal fixture-stewards-reach-the-desktop (tier 3, approved by Wido at
his terminal on 2026-09-06). Its record,
metasystem/plans/goals/fixture-stewards-reach-the-desktop.md, is the
contract. In short: stewards that fixtures arm in temporary
repositories deliver alerts to the operator's desktop. Deliver in
metasystem/internal/steward/notify.go resolves the delivery command from
the repository's git configuration and, when none is set, falls back
on macOS to the platform notifier (osascript "display notification")
for ANY repository root; the health, dispatch and supervision-hook
fixture suites arm stewards in temporary repositories, and every
builder round runs them on this host, so Wido gets "HEALTH unhealthy"
pop-ups naming a folder under a temp directory. Wido's word: the host
receives real messages only, never test messages.

# The change

1. The steward identity record (InstallIdentity in
   metasystem/internal/steward/identity.go) gains `Enrollment string`
   with json tag `enrollment,omitempty`, one of `human-terminal`,
   `temporary-word`, `fixture`, written by every mint (arm, restart and
   the machine rebuild path in metasystem/internal/steward/runner.go)
   from how the enrollment was authorized: `temporary-word` when a
   temporary human word is recorded; `fixture` when the arming caller's
   HUMAN class was granted by a fixture authorization
   (metasystem/internal/fixtureauth, consulted by
   metasystem/internal/lease/classify.go: find the field or predicate
   that says the class came from a fixture grant and use it; say in the
   return which) OR the repository declares `metasystem.runtimes=fake`
   in its metasystem.conf (the marker runnerExclusion in runner.go
   already reads); `human-terminal` otherwise. A machine rebuild
   carries the prior generation's enrollment forward, exactly as it
   carries the human-witnessed generation. An identity without the
   field (minted before this lands) reads as `human-terminal` until its
   next restart, so no live steward goes silent.

2. NotifyCommand and Deliver in metasystem/internal/steward/notify.go
   consult it: an explicit `metasystem.steward.notify-command` in the
   repository's git configuration still wins for any enrollment (a
   fixture may point it at a recording command); without one, the
   darwin platform notifier is used only for a `human-terminal` or
   `temporary-word` enrollment; a `fixture` enrollment appends the
   message, with a UTC timestamp, to
   log (a file named notifications.log in the steward directory under its own repository's artifacts)
   root and returns nil (delivered, so launches gated on delivery still
   proceed). Where the identity cannot be read, deliver as today
   (human), never silently drop.

3. The platform message carries the installation root after the title,
   so an operator with several seats on one host can tell them apart:
   title "metasystem steward - <repository root>" (the absolute path
   the steward serves).

4. Tests and fixtures:
   - Go unit tests in the steward package for NotifyCommand and Deliver
     per enrollment kind, with the platform notifier behind an injected
     runner so no osascript runs in tests: human-terminal with no
     configured command chooses the platform notifier with the root in
     its title; fixture with no configured command writes the log line
     and returns nil; a configured command wins for both; a missing
     identity delivers as human; the identity round-trips the field
     and an old record without it reads as human-terminal.
   - metasystem/scripts/agents/health-fixtures.sh: its steward's alert
     lands in the notifications log under the fixture repository, and
     no osascript ran: put a recording `osascript` shim first on PATH
     for the suite that fails the suite when invoked. Keep the suite's
     existing configured-notifier scenario as it is (the configured
     command still wins).
   - metasystem/scripts/agents/supervision-hook-fixtures.sh and
     metasystem/scripts/agents/dispatch-fixtures.sh: the same osascript
     shim on PATH so a regression anywhere fails a suite.

Nothing else changes.

# Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l .` (empty;
the host's Go 1.27 toolchain and pinned static checker are current, no
toolchain override);
`go test ./internal/steward/ -count=1` green with the coverage floor
in metasystem/docs/project-rules.md respected;
`bash scripts/agents/health-fixtures.sh` and
`bash scripts/agents/supervision-hook-fixtures.sh` as far as the
sandbox allows (the seat replays them outside; process inspection is
denied in the sandbox).

# Constraints

Wall-clock budget: 60 minutes; return before it ends even if something
is red, naming it. Declare the boundary as every file that differs from
main. Never touch plans. Gap rule: stop and report a gap with your
proposed contract written out.

# Expected Return

The implementer return per the role schema: `riskiestPart` first,
`diffBoundary` with every touched path relative to the repository root
(each starts with `metasystem/`), `whatWasDone`, `gaps`, and `evidence`
in the settled `{command, observed, level}` shape.

# Acceptance Criteria

- A steward armed in a fake-runtime or fixture-authorized repository
  never invokes osascript; its alerts are in its own notifications log.
- A human-enrolled steward's desktop message names its repository root.
- A configured notify command wins for every enrollment kind.
- An identity minted before this change delivers as before.

# Gap Rule

stop and report a gap; never fill it silently.
