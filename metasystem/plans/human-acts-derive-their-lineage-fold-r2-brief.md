Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal human-acts-derive-their-lineage, tier 2, hazard DESIGN-BEARING, fold round two of chain hal-build1b)
Date: 2026-09-06

# Goal

Round one built metasystem/plans/human-acts-derive-their-lineage-brief.md;
the round-one critique (hal-critic1) found two material defects and
three notes, dispositions in
metasystem/records/misc/human-acts-derive-their-lineage-critique-r1-dispositions.md.
The brief's premise that a name plus an enrollment proves who acts was
too wide: only six verbs obtain a terminal proof, and the rest accept
a name unproven, so the derivation as built lets any shell that passes
--by in an enrolled checkout record a terminal lineage. This round
makes the derivation itself proven.

# The fold

1. In syncReq (metasystem/cmd/metasystem/goalsync_mutations.go), the
   derived path (lineage empty, `by` set) first proves the invoking
   process reaches the enrolled terminal:
   humanauthority.Prove(root, int64(os.Getppid()), nil, now) with the
   same reader and clock the human verbs use (read how
   proveGoalHumanAuthority in the same file calls it and follow it).
   Only a proof with a passing outcome derives the lineage
   terminal-<sanitised id>-<generation>, taking the id and generation
   from the enrollment the proof was checked against. A failing proof
   refuses: "a human act derives its lineage only at the enrolled
   terminal: this shell does not descend from it (<proof outcome>);
   run the act at the terminal, or pass --lineage". No enrollment
   keeps round one's refusal, now carrying the reader's error text
   after a colon (critique F-4).
2. goal enroll-terminal (critique F-1): it must not take the derived
   path before it has enrolled. Build its request after Enroll returns
   and derive the lineage from the enrollment it just published (its
   TerminalID and Generation); when enrollment fails, refuse with the
   enrollment error as before. Read how the verb orders syncReq and
   Enroll today and reorder only what this needs.
3. goal discharge-review-obligation passes its --by name to syncReq
   (critique F-5, one line).
4. Tests: TestSyncReqLineage gains a case where the enrollment exists
   but the invoking process does not descend from the terminal (use
   whatever the humanauthority tests use to make Prove fail, for
   example a reader or a fixture process tree; say what) asserting the
   new refusal; the existing derive case must now run under a passing
   proof (use the package's fixture proof path, humanauthority.FixtureGoalProof,
   if Prove cannot pass in a test process; say which). The fixture leg
   in metasystem/scripts/agents/goal-cli-fixtures.sh keeps its journal
   grep and replaces the opid-differs check with a comparison of the
   opid's last eight hex characters against sha256 of the derived
   lineage (critique F-3); if the headless bed cannot pass a terminal
   proof, the leg uses --fixture-human-authority exactly as the other
   human legs do and you say how the derivation is reached under it.

# Workspace

The same job worktree, on top of round one.
May touch: metasystem/cmd/metasystem/goalsync_mutations.go
May touch: metasystem/cmd/metasystem/goalsync_mutations_test.go
May touch: metasystem/cmd/metasystem (the enroll-terminal verb's file, if separate; say which)
May touch: metasystem/scripts/agents/goal-cli-fixtures.sh
Must not touch: internal/humanauthority, internal/goal, anything under plans.

# Constraints

- Bash 3.2 clean. Never weaken a test. One round, at most 60 minutes.
- If the fixture bed cannot reach a passing terminal proof at all,
  stop and say so (gap rule) with the Go test carrying the proof.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `go test -count=1 -run 'Lineage|SyncReq|Enroll' ./cmd/metasystem` (expected: ok; name the tests)
- `go vet ./cmd/metasystem` and `gofmt -l ./cmd/metasystem` (expected: clean)
- `bash -n ./scripts/agents/goal-cli-fixtures.sh` (expected: clean)
- `git diff --stat` (expected: only files under May touch)

# Acceptance Criteria

1. A --by act without a lineage derives terminal-<id>-<generation>
   only when the invoking process proves the enrolled terminal; from
   any other shell it refuses naming the terminal.
2. enroll-terminal works on an unenrolled checkout without a lineage
   and records the lineage of the enrollment it published.
3. Tests and the fixture leg pin it as above.

# Gap Rule

stop and report a gap; never fill it silently.
