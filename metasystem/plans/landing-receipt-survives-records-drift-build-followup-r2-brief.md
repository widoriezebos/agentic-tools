Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, coordinator under goal landing-receipt-survives-records-drift)
Date: 2026-09-09

# Follow-up brief: round 3 of chain lrsrd-build1 — one line, so the shell canaries can run

Round 2 is clean: the fresh-context closing read returned zero material
findings on your tree, and all five Go packages are green outside the
sandbox. But the orchestrator's run of `scripts/agents/land-fixtures.sh` on
your tree failed its `full-width-chain` scenario in SETUP, before any of the
three canaries you wrote ran, with the fixture's own message:

    land full-width-chain fixture: full battery command source is unreadable

The cause predates this chain. At its line 647 the leg extracts the full
battery command with a sed expression that matches a line of
`metasystem/internal/landing/tierone.go` beginning with `const
fullBatteryCommand = "` and captures the quoted string. Since yesterday's
landing, line 18 of that file reads `const fullBatteryCommand =
validate.FullBatteryCommand`, a reference, not a literal; the literal now
lives in `metasystem/internal/validate/recertification.go` at line 28 as
`const FullBatteryCommand = "..."`, one line, quoted. The old expression
matches nothing on main or on your tree, so the leg exits 1 before it starts.

## What this round does, and nothing else

Change that one extraction in `metasystem/scripts/agents/land-fixtures.sh`
so it reads the quoted literal from `recertification.go`'s
`FullBatteryCommand` line instead, keeping the same "unreadable" refusal
when the extraction is empty. Nothing else in the file, nothing outside it.
Then `bash -n` on the script, and show the extracted command as evidence
(run the sed alone). You cannot run the bed in your sandbox; the orchestrator
runs it on your tree the moment you return.

Do not change any Go file, any canary, or any other script. Your
`diffBoundary` is exactly `metasystem/scripts/agents/land-fixtures.sh`.
Wall-clock budget: 15 minutes. Gap rule unchanged.
