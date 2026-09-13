Working Mode: implement
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal wait-verb-returns-on-recorded-events)
Date: 2026-09-13

# Goal

Fix forward a bed defect of the landed wait member (commit 57ba0586):
the five wait scenarios of metasystem/scripts/agents/supervision-fixtures.sh
(wait-job-run, wait-proof, wait-ledger, wait-restart, wait-bounds) fail
when a seat runs the bed by hand with `METASYSTEM_BIN` pointing at a
candidate engine built outside any metasystem tree, which is the seats'
standard way to check a candidate. Each scenario runs Go tests with
`METASYSTEM_WAIT_BINARY` set to that engine, and the tests' installed-binary
branch runs it as a subprocess with `--root` set to the test's temporary
directory.

The failure, reproduced by the seat on trunk 3a6353c3:

```
METASYSTEM_WAIT_BINARY=<candidate outside a tree> go test ./cmd/metasystem -run 'TestWaitInstalledRunCommand$' -count=1
wait_verb_test.go:302: installed run wait code=65 stderr="exit status 65" output="wait delivery adapter fake is unavailable\n"
```

The same test passes when the binary sits inside a tree (copied to a
worktree's metasystem/bin/). The cause: `waitAdapterPathForRuntime` in
metasystem/cmd/metasystem/wait_verb.go looks for the adapter under the
executable's installation (`upMetasystemRoot("")`, which needs
metasystem.conf beside bin/), and otherwise under `--root`; the test's
temporary root has no adapters, and the in-process override the test
installs does not reach a subprocess. The engine's own proof bed passes
because its engine sits inside an installation.

# Workspace

Your job worktree, branch agent/<job>. Change only test code: the
installed-binary branches of the tests that read `METASYSTEM_WAIT_BINARY`
in metasystem/cmd/metasystem/wait_verb_test.go,
metasystem/internal/run/waiter_pair_test.go,
metasystem/internal/goal/attention_test.go and
metasystem/internal/goal/txn_test.go, plus a shared test helper if one
helps. Do not change metasystem/cmd/metasystem/wait_verb.go or any
production adapter lookup, and do not change the bed script. Do not
commit.

# What the fix must satisfy

- Every test that reads `METASYSTEM_WAIT_BINARY` passes with that
  variable pointing at an engine built outside any metasystem tree, and
  still passes with it pointing at an engine inside one, and with it
  unset.
- The subprocess finds the real fake adapter of the source tree the
  tests run from (metasystem/scripts/agents/adapters/fake.sh with what it
  sources), never a stub written by the test, so the installed branch
  still proves the adapter answers `blocking`.
- No test writes into the source tree.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json):
jobId, round, runtime, sessionId, model, evidence, gaps, mode, riskiestPart,
diffBoundary, whatWasDone. evidence carries `{command, observed, level}` items
replayable verbatim from the worktree's repository root: (1)
`git -C metasystem status --short`; (2) a candidate built outside the tree,
`( cd metasystem && scripts/agents/go-build.sh --trimpath --out /tmp/wait-bedfix-cand/metasystem )`;
(3) each of the five scenarios' Go test commands, copied from their case
arms in metasystem/scripts/agents/supervision-fixtures.sh, run from
metasystem/ with `METASYSTEM_WAIT_BINARY=/tmp/wait-bedfix-cand/metasystem`,
with their pass lines; (4) the same five with the variable unset; (5)
`( cd metasystem && scripts/agents/go-gate.sh --fast )` with its last line.

Every path in your return (diffBoundary, files) is relative to the
repository root, so it starts with `metasystem/`.

# Constraints

- Go owns the logic; no new shell assertions. Plain-English error texts;
  source comments say what and why, never which round or finding.
- Wall clock: 45 minutes. A partial round returns with its tests green
  for what exists and names what is left.

# Gap Rule

stop and report a gap; never fill it silently.
