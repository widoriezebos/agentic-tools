Working Mode: implement
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal codex-handshake-budget-load-fragile)
Date: 2026-09-02

# Goal

Two-layer implementation critique of the Codex handshake build (job
ch-build-1c, Sol; diff.patch in its round evidence, one commit 35b779ef
"Disable operator plugins for Codex delegates"). The build delivered PART 1
of the certified design metasystem/plans/codex-handshake-design.md (revision
7, section 2, D1.1 to D1.6) and then stopped at a specification gap in Part
2 (D2.3 versus an out-of-boundary test), writing no Part 2 bytes; the gap is
the orchestrator's and is being folded separately. Critique what was built:
first conformance of the diff against section 2, then adversarial defect
review. Sol's four registers are metasystem/records/misc/codex-handshake-critique-r1.md
to -r4.md.

# Attack surface

- Exactly three files change: metasystem/internal/adapter/codex.go, the new
  codexcommand_test.go beside it, and metasystem/internal/adapter/runtime_test.go (the existing pin).
  Any other hunk, and any Part 2 byte, is a finding.
- `plugins={}` is emitted as a `-c` pair immediately after the
  `approval_policy="never"` pair on BOTH the dispatch and the resume verb
  (D1.2); the resume verb takes `-c` but no `--sandbox` or `-C` (section 7).
- The pins are `TestBuildCodexCommand` and
  `TestBuildCodexCommandDisablesOperatorPlugins` as D1.6 names them, and
  they assert the pair's position, not merely its presence.
- The return says the focused adapter tests ran green but the full gate (go
  build, vet, gofmt, bash -n, go test ./..., fixtures) did NOT run. Read the
  diff for anything the full suite would catch: another test pinning the
  exact Codex argv (search the tree for `approval_policy` in _test.go and
  fixture files) that the new pair now breaks.
- No weakened refusal, no narrowed guarantee, no benchmark (R-31).

# Constraints

Wall-clock budget: 20 minutes. Your sandbox is read-only; verify by reading.
Return per the code-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
