Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal claude-critic-sandbox-allows-loopback, tier 2, hazard MECHANICAL)
Date: 2026-09-06

# Goal

A claude critic's sandboxed shell (goal code-critic-runtime-has-no-shell,
landed 07d18614) refuses loopback binding: go test ./internal/adapter
fails 13 of 131 tests with "listen tcp 127.0.0.1:0: bind: operation not
permitted", and any package whose tests open a local listener cannot
be reviewed by running it (proof job ccs-proof1). The orchestrator
probed the sandbox on 2026-09-06 with the engine's own settings and
argv for a code-critic record: with `sandbox.network.allowLocalBinding`
set to true, a Python socket bound 127.0.0.1 ("BIND-OK") and
`go test -run TestTelegramTokenRoutes ./internal/channel/fake`, a
listener test, passed; the top-level `sandbox.allowLocalBinding`
placement did nothing.

When you are done, every claude delegate's settings carry
`sandbox.network.allowLocalBinding: true` beside the existing network
rule (an implementer's go test needs it as much as a critic's), a
settings test pins it, and nothing else changes.

# The change

1. BuildClaudeSettings (metasystem/internal/adapter/claude.go): the
   network sandbox object gains `allowLocalBinding: true` in both
   branches (the ordinary-egress shape and the deny shape); the
   egress rule itself is untouched. Say in the comment why: loopback
   listeners are how Go tests and local fixtures work, and a bound
   loopback port is not egress.
2. Tests (metasystem/internal/adapter/runtime_test.go): the existing
   settings tests keep their assertions; add one assertion in the
   code-critic settings test and one in the write-and-network test
   that sandbox.network.allowLocalBinding is true. Change no existing
   assertion.

# Workspace

The job worktree the dispatcher creates for you, branched from main.
May touch: metasystem/internal/adapter/claude.go
May touch: metasystem/internal/adapter/runtime_test.go
Must not touch: anything else.

# Constraints

- Never weaken a test. One round, at most 30 minutes. Hazard
  MECHANICAL: one documented sandbox key, proven by probe; the
  orchestrator proves it live through a dispatched critic running the
  adapter package green.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `go test -count=1 ./internal/adapter` (expected: ok)
- `go vet ./internal/adapter` and `gofmt -l ./internal/adapter` (expected: clean)
- `git diff --stat` (expected: only the two May-touch files)

# Acceptance Criteria

1. Every generated settings file has sandbox.network.allowLocalBinding true.
2. Two tests pin it; no existing assertion changed.

# Gap Rule

stop and report a gap; never fill it silently.
