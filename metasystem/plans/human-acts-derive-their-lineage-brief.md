Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal human-acts-derive-their-lineage, tier 2, hazard DESIGN-BEARING)
Date: 2026-09-06

# Goal

Every goal mutation goes through syncReq in
metasystem/cmd/metasystem/goalsync_mutations.go, which refuses
"mutations carry their coordinator's identity: export
METASYSTEM_OWNER_LINEAGE or pass --lineage" whenever neither is set,
including the human-only acts a person runs at the enrolled terminal
with --by (approve, unapprove, set-budget, done, edit, park, resume,
accept-risk, set-obligation, classify and approve sweeps). Wido hit it
on 2026-09-06 concluding a goal from his terminal: the value he must
paste is a seat's session-main id he has no reason to know. A human
act at the enrolled terminal already proves who acts (the terminal
enrollment in metasystem/internal/humanauthority/authority.go,
ReadEnrollment, and the --by name); its lineage should follow from
that.

When you are done, a --by act run without --lineage and without the
environment variable derives its lineage from the enrolled terminal
and records it; an agent call (no --by) keeps the current refusal
text; a --by act on a machine with no terminal enrollment refuses
with a sentence that names enrolling the terminal as the repair.

# The design

1. In syncReq, when the lineage is empty and `by` is non-empty, read
   the enrollment with humanauthority.ReadEnrollment(root). When it
   succeeds, the lineage is `terminal-<terminalId>-<generation>` with
   every byte outside [A-Za-z0-9-] replaced by '-' (the terminal id
   carries a colon on macOS); when it fails, refuse with: "a human act
   derives its lineage from the enrolled terminal, and this checkout
   has none: run metasystem goal enroll-terminal here once, or pass
   --lineage". When `by` is empty, the existing refusal text stays
   byte for byte.
2. Nothing else in the request changes: Machine, Human, the claim
   epoch classification, the opid (which hashes the lineage) and the
   history actor are as today. The derived lineage appears wherever a
   lineage is recorded (a claim line, a journal entry) like any other.
3. A unit test in metasystem/cmd/metasystem (beside the existing
   goalsync tests, if any; otherwise a new _test.go in that package)
   for the three cases: by set and enrollment present derives the
   terminal lineage; by set and no enrollment refuses with the new
   sentence; by empty refuses with the old sentence. Write the
   enrollment record through the package's own writer if one is
   exported, else through the on-disk shape ReadEnrollment reads (say
   which).
4. A goal-cli fixture leg in metasystem/scripts/agents/goal-cli-fixtures.sh:
   with METASYSTEM_OWNER_LINEAGE unset and no --lineage, an approve
   --by Wido --fixture-human-authority on a checkout with an
   enrollment record succeeds and the recorded approval's opid differs
   from a fixture-lineage run; if the bed cannot host an enrollment
   record, say so (gap rule) and the Go test carries the proof.

# Workspace

The job worktree the dispatcher creates for you, branched from main.
May touch: metasystem/cmd/metasystem/goalsync_mutations.go
May touch: metasystem/cmd/metasystem (one new or existing _test.go)
May touch: metasystem/scripts/agents/goal-cli-fixtures.sh
Must not touch: internal/humanauthority (read it, do not change it), internal/goal, anything under plans.

# Constraints

- Bash 3.2 clean. Never weaken a test. One round, at most 60 minutes
  of wall clock. Hazard DESIGN-BEARING: a recorded identity is
  derived instead of supplied; an independent critique follows.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `go test -count=1 -run 'Lineage|SyncReq|Sync' ./cmd/metasystem` (expected: ok; name the tests)
- `go vet ./cmd/metasystem` and `gofmt -l ./cmd/metasystem` (expected: clean)
- `bash -n ./scripts/agents/goal-cli-fixtures.sh` (expected: clean)
- `git diff --stat` (expected: only files under May touch)

# Acceptance Criteria

1. A --by act without --lineage or the variable succeeds at an
   enrolled terminal with lineage terminal-<id>-<generation>.
2. Without an enrollment it refuses naming enroll-terminal; an agent
   call refuses with the old text.
3. The Go test pins all three; the fixture leg pins the first or its
   absence is explained.

# Gap Rule

stop and report a gap; never fill it silently.
