Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal supervision-hook-wrong-root)
Date: 2026-09-02

# Goal

Round-4 critique of metasystem/plans/supervision-hook-root-design.md
revision 4 (landed, in your worktree), which folded your two round-3
findings (metasystem/records/misc/hook-root-critique-r3.md, landed):
SHR-R3-ENGINE-INSTALLATION-PAIR-01 (the engine answer bound to the same
installation every shell consumer uses, METASYSTEM_BIN override included) and
SHR-R3-GIT-STEERING-01 (the worktree mapper ignores inherited git steering
variables). Judge each fold BY ID against the code it cites
(metasystem/scripts/agents/supervision-hook.sh, metasystem/cmd/metasystem/up.go,
metasystem/scripts/agents/evidence-gc.sh) and confirm no regression in the
decisions the earlier rounds certified. Note for the build that follows:
goal turn-verdict-hardening's slice 1b later edits this hook's verdict lines
on top of this fix; say if revision 4 leaves the hook in a shape that makes
that edit unsafe.

Findings material and grounded, quoting the disagreeing text or code. Your
sandbox is read-only: verify by reading, do not run go. Zero material
findings is an acceptable, closing answer if the reading supports it.

# Constraints

Wall-clock budget: 20 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
