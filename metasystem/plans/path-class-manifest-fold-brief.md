Working Mode: design
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Revision 2 of metasystem/plans/path-class-manifest-design.md (revision 1,
landed; edit it in place, bump the revision line): fold the thirteen
material findings of metasystem/records/misc/path-class-manifest-critique-r1.md
by id, and decide the four gaps the critic declared. This is the ONE fold
the chain gets (Wido's stop criterion): after it, one closing review, then
the build. Every closure is a design change that names the artifact it
changes, never a softened claim. Keep the document under 330 lines; a
fold that grows the design is a finding against itself.

# Workspace

The delegate worktree the dispatcher created for this job. Edit exactly one
existing file, the design; nothing else.

# Direction per finding (the critic's artifact names bind)

- PCM-R1-001: add the row `plans/goals.md ledger` and the compatibility
  fixture case; state the rule for absent-but-named paths once.
- PCM-R1-002: give the two key spaces distinct namespaces in the manifest
  grammar (for example a `repo:` and an `install:` prefix per row, or two
  sections), so `metasystem` the tracked directory and `metasystem` the
  built binary never share a key, and template-only outer rows never
  classify an adopted application's paths; adjust the resolver and the
  fixtures.
- PCM-R1-003: exact behavior rows for development/README.md,
  development/project-rules-local.md, development/devin-selftest.md and
  any other instruction-bearing file under development/ (read the
  directory and list them); the rest of development/ stays record.
- PCM-R1-004: exact revert never deletes an appended record; the record
  outcome for exact revert is refuse, and the revert test covers all five
  class outcomes.
- PCM-R1-005: the record ownership rule binds the base goal's claimed
  machine and lineage to the wrapper actor (read how commit.sh already
  stamps the machine nickname and how the goal file records Claimed);
  a goal name the actor does not hold is refused.
- PCM-R1-006 and gap 2: tie-break is the longest complete goal identifier
  that is a prefix of the filename followed by a hyphen; state it, test
  it.
- PCM-R1-007: carry --goal through metasystem/scripts/agents/land.sh (it
  rejects unknown options today), add land.sh and land-fixtures.sh to the
  diff boundary, and move machine resolution in commit.sh before the
  evaluator call; add the end-to-end land-to-evaluator fixture.
- PCM-R1-008: Goal-Item is validated (the id exists in the ledger) and
  single-valued: a Goal-Item already present in -m, -F or --trailer input
  is refused; fixture covers malformed and conflicting inputs.
- PCM-R1-009: the evaluator returns the unclassified-path detail (path,
  nearest classified ancestor or the sentinel) in its Observation from
  the BASE manifest; commit.sh prints it; no call to the checked-out verb
  on the refusal path; base-versus-candidate fixture.
- PCM-R1-010 and gap 4: define the complete fail-closed code set and say
  for each code whether this slice promotes it or leaves it observed,
  with the reason; promote at least path-unclassified, ledger, runtime,
  register-carriage-policy-unreadable and register-carriage-not-append-only
  in this slice; record-not-owned may stay observed for one observation
  window only if the design says how long and who promotes it.
- PCM-R1-011: keep the invariant by conformance: reject any runtime-
  declared instruction file whose manifest class is not behavior
  (metasystem/internal/validate/conformance.go and its test).
- PCM-R1-012 and gap 1: the sentinel for "no classified ancestor" is the
  literal text "no classified ancestor; add a row for <path> or its
  directory to <manifest>"; exact expected output in the verb tests.
- PCM-R1-013: the deleted-reader fixture derives its search set from the
  manifest's behavior rows, excluding declared historical record trees.
- Gap 3, decide: existing plans files that match no open goal are FROZEN
  for register carriage (they change only through a reviewed chain, or
  by a goal that claims them by an explicit row in the manifest's
  ownership section); state the migration note for the 158 legacy files
  as a one-time listing in the design, not a build task.

# Size (the non-material note, decided here)

Two build slices, each under 240 reserved minutes with a correction round
intact: slice 1 the manifest, the verb, the resolver, conformance, the
three deletions and their fixtures; slice 2 the landing evaluator's class
table, exact revert, record ownership, the wrapper inputs (commit.sh,
land.sh), the promotion set, and the end-to-end fixtures. Name each
slice's diff boundary exhaustively.

Fold record: add a section mapping each finding id and gap to its fold.
Self-grade per the house rule.

# Constraints

Wall-clock budget: 35 minutes. Design only; edit nothing but the design
file. R-31: no benchmarks.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file.

# Gap Rule

stop and report a gap; never fill it silently.
