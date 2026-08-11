# Gate runs get a control plane

Owner: main session (claude). Status: PLANNED — implement after the
in-flight suite run completes (implementing now would trip hazard 4).
Ruled by the human 2026-08-11 after the four-concurrent-suites
incident: suite launches need a reliable, controlled path in Go.

# Intent

The engine owns the LIFECYCLE of a validation-suite run; the suite
itself stays bash (the checks are not the problem — the launching is).
Evidence, one afternoon: four concurrent suite launches trampling
shared fixtures because nothing refuses a second launch; a hand-rolled
cleanup grep that killed the repository's real supervision components;
and the latent hazard that any `go build -o bin/metasystem` during a
run swaps the binary mid-suite so later sections run different code.

# Design (small; the gate family already exists)

- `gate run --suite metasystem|kit [--log PATH]`: takes the run lock
  (rename-born directory with owner identity — the lock IS the launch,
  a second call refuses with the live owner named), launches the suite
  detached via the existing launch-detached machinery with kernel
  identity recorded, registers the existing gate-run marker, prints
  the run id and log path.
- `gate run-status`: owner identity, liveness (kernel proof), elapsed,
  log tail path. `gate run-cancel`: TERM the run's process group with
  the custodian proof, then sweep fixture supervisors by their fixture
  tag prefixes with the same proven-death discipline as the janitor —
  never a bare pkill.
- Rebuild fence (hazard 4): go-gate.sh asks `gate run-status` first and
  REFUSES to rebuild bin/metasystem while a suite run is live (override
  flag for the human). The suite entry itself also takes the lock when
  invoked directly by a human, so the guarantee holds on both paths.
- Lock staleness: proven-death takeover exactly as every other lock;
  a crashed run's husk is healed, never waited on.

# Tests

Lock refusal with a live owner and takeover of a dead one; detached
launch identity; cancel sweeps only fixture-tagged supervisors (the
repository's real supervision must survive a cancel — regression for
today's incident); go-gate refusal while a run is live.
