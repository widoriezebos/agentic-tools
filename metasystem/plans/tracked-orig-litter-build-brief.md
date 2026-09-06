Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal tracked-orig-files-litter)
Date: 2026-09-06

# Goal

Goal tracked-orig-files-litter (tier 1, approved by Wido on
2026-09-06). Its record, metasystem/plans/goals/tracked-orig-files-litter.md,
is the contract: patch backup files ending in .orig are tracked on
main, mislead readers and critics, and nothing refuses a new one.

# Scope under the tier-1 floor

The path-class manifest refuses tier-1 landings under
metasystem/internal/dispatch, so this round deletes only
metasystem/cmd/metasystem/dispatch_verbs.go.orig. The two under
internal/dispatch (finding_register.go.orig and record.go.orig) are
deleted by the next tier-2 chain on this seat, which touches that
package with a critic; do not touch them here.

# The change

1. Delete metasystem/cmd/metasystem/dispatch_verbs.go.orig.
2. In metasystem/scripts/agents/pre-commit-guard.sh, beside the
   existing new-plan-file challenge, refuse any staged path whose name
   ends in `.orig` (added or modified), with a one-line message naming
   the path and saying patch backups are never tracked. No override
   variable: a backup file has no lawful reason to be committed.
3. In metasystem/scripts/agents/pre-commit-guard-fixtures.sh, one case
   proving a staged `.orig` file is refused and one proving an ordinary
   staged file still passes, in the bed's existing style.

# Gate

`bash -n scripts/agents/pre-commit-guard.sh` and `bash
scripts/agents/pre-commit-guard-fixtures.sh` from the metasystem
directory; `git ls-files | grep '\.orig$'` from the repository root
lists only the two internal/dispatch files. Report each with its
evidence level.

# Constraints

Wall-clock budget: 15 minutes. MECHANICAL reach, tier 1: no critic; the
orchestrator's gate is the examination. Declare the boundary as every
file that differs from main. Gap rule: stop and report a gap with your
proposed contract written out; never fill it silently.
