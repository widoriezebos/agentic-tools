Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal wait-verb-returns-on-recorded-events)
Date: 2026-09-13

# Review brief: the bed fix of the landed wait member

FINDING IDS: chain-unique across this goal's critic chains; WVB-40 to
WVB-56 are taken; new findings get WVB-57 onward.

Why this review exists: the wait member landed at 57ba0586. Its five wait
scenarios in metasystem/scripts/agents/supervision-fixtures.sh
(wait-job-run, wait-proof, wait-ledger, wait-restart, wait-bounds) run Go
tests with `METASYSTEM_WAIT_BINARY` set to the bed's engine, and the
tests' installed-binary branch runs that engine as a subprocess with
`--root` at a temporary directory. With a candidate built outside any
metasystem tree, the seats' usual by-hand recipe, the engine found no
adapter (`waitAdapterPathForRuntime` in
metasystem/cmd/metasystem/wait_verb.go looks beside the executable's
installation, then under `--root`) and every scenario failed with "wait
delivery adapter fake is unavailable". The build chain
wait-member1-bedfix-1 fixes it in test code only. Round 1 added
`InstalledWaitBinary`, a new test-support package, internal/testutil,
that copies the candidate into a temporary installation whose
metasystem.conf and adapter directory link to the source tree's, and
applied it to every branch that reads the variable. Round 2 records an
exemption for internal/testutil in both coverage-ratchet baselines,
because a package with no tests and no floor fails the full gate.

Round budget: 1 focused round. A finding is material only if the change
must be altered before it lands, and it names the artifact.

# Mandate

1. Does every test that reads `METASYSTEM_WAIT_BINARY` now run the exact
   candidate against the source tree's real fake adapter, for a candidate
   outside a tree, inside one, and with the variable unset, without
   writing into the source tree?
2. Does the helper change what the installed branch proves (for example,
   could a stale engine or a stub adapter make a test pass), and do its
   temporary installations and symlinks stay inside the test's temporary
   directories on both darwin and linux?
3. Is the coverage exemption scoped to internal/testutil alone, with no
   floor lowered and no production package exempted?
4. Nothing else.

# Expected Return

The code-critic return schema, findings sorted by materiality, each with
file:line evidence and its rigor row. Every path in your return is
relative to the repository root, so it starts with `metasystem/`.

# Gap Rule

stop and report a gap; never fill it silently.
