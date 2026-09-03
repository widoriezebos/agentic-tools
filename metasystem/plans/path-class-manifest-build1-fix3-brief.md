Working Mode: build
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Final correction round on the path-class manifest, first part (chain
path-class-build1, your reviewed tree 34e56d84). The closing code review
metasystem/records/misc/path-class-manifest-code-critique-r2.md left two
material findings, both in the waiver rule your last round changed, and
one fact outside the slice that blocks the landing gate. Fold the three;
nothing else changes.

# The three items

- PCM-CC2-001: the waiver rule in internal/validate/conformance.go now
  refuses only behavior and unclassified paths inside the installation;
  the certified design (section 3, the waiver consumer) and your round-2
  tree also refused ledger and runtime paths. Restore that: a waiver
  refuses behavior, ledger, runtime and unclassified inside the
  installation; only record paths, and paths outside the installation,
  stay waivable. Extend the test.
- PCM-CC2-002: in the layout scripts/adopt.sh produces (the installation
  is the repository root, the git prefix is empty), waiverPathMode reports
  adopted while ResolveRepositoryPath with an empty prefix resolves every
  changed path in the install namespace before looking at the mode, so an
  application file is refused. Decide by mode before namespace: in
  adopted mode with an empty prefix, a changed path that is not under the
  vendored installation is outside and waivable. Add the root-layout
  test.
- PCM-CC2-003 (gate blocker, outside the slice but on the landing path):
  internal/landing has no coverage floor or exemption in either baseline,
  so the full Go gate refuses on this tree whatever the diff does.
  Measure internal/landing's coverage in your worktree and register the
  floor in scripts/agents/coverage-ratchet.json and
  coverage-ratchet-linux.json at the measured number; state both numbers
  in your return.

The three low notes (tracked-path walk ignoring mode; the unreachable
exact-inverse legs in this part; the misinstalled diagnostic when the
manifest is absent) stay as recorded residuals for the second part
unless each is a one-line change.

# Gate

`go test ./internal/pathclass/ ./internal/landing/ ./internal/validate/ ./cmd/metasystem/ -count=1 -cover` green with the two coverage numbers reported; `bash scripts/agents/path-class-fixtures.sh` green; `gofmt -l` empty; `go vet` clean. The orchestrator replays the full gate outside the sandbox (KI-15).

# Constraints

Wall-clock budget: 35 minutes. DESIGN-BEARING reach (correction at high effort). R-31: no benchmarks.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the files you changed.

# Gap Rule

stop and report a gap; never fill it silently.
