Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Correction round on chain path-class-build2b (your reviewed tree
63b76e52). The closing code review (path-class-build2b-cc1) found one
material defect, PCM-CC9-001, and three notes. Fold the defect; the
notes are settled below.

# The defect

The README leg you carried in TestRealCommitWrapperStampsParseableObservation
(metasystem/scripts/agents/static-reproof-fixtures.sh) expects a
register-carriage landing of an unlisted root file to refuse
path-unclassified. The fixture repository is an adopted ROOT-layout
installation, so after the PCM-CC8-001 fold the ownership oracle calls
README application-owned, the class resolves to `outside`, and the
outside row of design section 3 lets the carriage landing pass
(register-carriage-path-refused stays observed). The evaluator is
design-correct; the leg's expectation is wrong. With the shipped
manifest no root-layout path can reach path-unclassified, because every
inventory-owned path has a row.

# The change

1. metasystem/scripts/agents/static-reproof-fixtures.sh only: make that
   leg reach path-unclassified the way production does, in the VENDORED
   layout: the installation one level below the fixture repository root
   (a `metasystem/` directory holding the fixture's scripts, bin, memory
   and manifest, git initialised at the parent), with the README staged
   INSIDE the installation so it resolves through the install namespace
   and has no manifest row. Keep the assertion: status non-zero,
   `would-refuse code=path-unclassified`, and the exact refusal text
   naming scripts/agents/path-classes.txt. The rest of the test keeps
   its layout and expectations; if the vendored leg needs its own small
   fixture repository, build it beside the existing one rather than
   moving the existing legs.
2. Nothing else changes. Declare the boundary as the files you touch,
   with the metasystem/ prefix.

# The notes, settled

- PCM-CC9-002 (owner.go comment no longer names goal
  adoption-inventory-from-install-set): accepted as a residual; the
  goal record names the file, the comment need not name the goal.
- PCM-CC9-003 (the oracle is called once per changed path): accepted
  knowingly; latency only.
- PCM-CC9-004 (class-check order differs between exact revert and
  carriage on mixed record-plus-ledger inputs): recorded as a residual
  test obligation for the second landing bar's chain, not this one.

# Gate

`bash metasystem/scripts/agents/static-reproof-fixtures.sh` green on
this host, and `cd metasystem && go test ./internal/landing/ -count=1`
green. Do not run path-class-fixtures.sh here (it needs ripgrep, which
this host lacks; backlog item path-class-fixture-ripgrep).

# Constraints

Wall-clock budget: 25 minutes. MECHANICAL reach for this round.

# Gap Rule

stop and report a gap; never fill it silently.
