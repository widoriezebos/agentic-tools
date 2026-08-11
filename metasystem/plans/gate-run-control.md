# Gate runs: use the engine's own launch mechanism

Owner: main session (claude). Status: PLANNED (shrunk by the human's
correction, 2026-08-11) — implement after the in-flight suite run.

The launching mechanism already exists in Go and is the standing way
to start a suite from now on:

    bin/metasystem gate check --root .          # 1 => refuse: already running
    bin/metasystem supervise launch-detached \
        --log <log> --cwd . -- bash scripts/validate-metasystem.sh
    bin/metasystem identity started-at --pid <pid>   # track by kernel identity

The suite already registers itself (gate register), so gate check is
the live-run signal — proven during the four-concurrent-runs incident,
where the signal was 1 the whole time and simply was not consulted.

What is genuinely missing is only that the scripts consult it
THEMSELVES instead of trusting the operator:

1. scripts/validate-metasystem.sh refuses at startup when gate check
   reports a live gate run in this checkout (override env for a human
   who knows: METASYSTEM_ALLOW_CONCURRENT_GATE=1).
2. scripts/agents/go-gate.sh refuses to rebuild bin/metasystem while
   gate check reports a live run (same override) — a rebuild during a
   suite swaps the binary mid-run so later sections run different code.

Two guards, thin bash over shipped verbs, tests in the fixtures. No
new Go, no new locks, no new family.
