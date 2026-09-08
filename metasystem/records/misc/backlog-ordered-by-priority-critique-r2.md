# Backlog ordered by priority: build critique round 1 (chain backlogorder-build1, round 1)

Critic backlogorder-crit2 (code-critic, Opus 5) on reviewed tree 0fc99f2124701adf61cafa7e7f4f1670a9124ee0. Two material findings and three notes. The high finding was independently confirmed by the orchestrator: scripts/agents/goal-cli-fixtures.sh exits 1 on this tree, exactly as the read predicted from reading the printer and the bed's patterns.

## BOB-01 - high - material=True

CLAIM: The human-readable listing changed shape and the fixture bed that reads it was not carried forward, so the run the design itself names as its gate fails. `goal list --pretty` used to print each goal as two spaces followed by its identifier under state headings; it now prints an aligned table whose rows start with the priority column. scripts/agents/goal-cli-fixtures.sh still tests the old shape in four places: two positive checks (lines 754 and 760), one negative check that an unlabelled goal does not appear (line 756), and an identifier extractor (lines 766-769). Under the new output the positive checks fail with "one-label list filtering lost a match". The negative check is worse than a failure: it can no longer match anything, so it silently stops proving that a filtered listing excludes unlabelled goals.

EVIDENCE: The critic ran the new printer against the build's own command-level fixture and applied the bed's exact patterns: the output is a banner, the header "PRIORITY  SEQUENCE  STATE    PIN  GOAL", rows such as "1         1         queued   m2   z-ranked", then the archived count. The two-space pattern matched nothing and the extractor returned zero identifiers. Confirmed by execution: the orchestrator ran the bed on this tree and it exited 1.

## BOB-02 - low - material=True

CLAIM: docs/backlog-mechanism.md was not updated, although the design's build-boundary paragraph names it among the files this build changes. That document explains the sibling human act, set-pin, in prose, and now says nothing about the priority and sequence fields or about goal set-priority, even though slice 1 is precisely the part of the story it covers. Nothing in it became false; the gap is an omission.

EVIDENCE: Searching the reviewed tree for priority and sequence in that file returns only the pre-existing dispatch-delegate-sequencing paragraph. The diff contains no documentation file: its fifteen paths are all under cmd/metasystem and internal/goal.

## BOB-03 - low - material=False

CLAIM: The read-side fetch option landed on the listing but not on the frontier reader, so half of one design sentence is outstanding. Section 3 says to add it to the synced list and next readers. The build added it to goal list and left goal next untouched, which is consistent with the build brief deferring section 4 to slice 2. Recorded so it is carried into slice 2 deliberately rather than lost between two scopes.

EVIDENCE: runGoalList declares the fetch flag and passes it into the projection; runGoalNext declares only the root and the label flag, and the diff does not touch it.

## BOB-04 - low - material=False

CLAIM: One clause of the new command-level listing canary cannot fail. In TestGoalPriorityListing the assertion that the table still carries state, pin, the unranked marker and the intent detail is a chain of substring checks, and one link looks for a bare hyphen. Every listing contains a hyphen somewhere, in the banner, in identifiers such as "a-unranked", in timestamps, so that link is satisfied whatever the code does and the unranked marker is not proven by it. The other links and the ordering checks above them are real.

EVIDENCE: The assertion chain in cmd/metasystem/goal_priority_test.go includes a check for the single-character string "-", and the captured pretty output contains hyphens in the banner, in two identifiers and in the empty cells.

## BOB-05 - low - material=False

CLAIM: There is no in-product way to repair the new invariant once violated, but that is the ledger's existing rule rather than something this build introduced. Every transaction validates the tip it captured before mutating, so a canonical branch violating the dense-rank rule would refuse every verb including the one that could renumber the offending priority. Reconcile now refuses direct rank edits, so the hand-edit route in is closed.

EVIDENCE: The transaction validates the captured tip before mutation; the new validator rejects a tree with a hole or duplicate; reconcile refuses a direct rank edit and names set-priority as the remedy.

## Coordinator's reading (m1b, 2026-09-08)

BOB-01 and BOB-02 accepted and folded in round 2. BOB-04 is folded too
although the critic marked it not material, because a dead clause inside
a canary is the exact defect Sol found twice in this page's design and
Wido's standing instruction is that canaries must be able to fail; it is
one line.

BOB-03 is not built here. It belongs to slice 2 with the rest of section
4, and it is written into the goal's next step so it is carried
deliberately rather than lost between two scopes.

BOB-05 is recorded and not actioned. The route into the broken state is
closed, and widening the repair path would change how every ledger
transaction validates its tip, which is far outside this goal. Round 2
does add one thing for it: the refusal a person meets must name the
ledger's existing repair process, so nobody is left with a queue that
refuses every verb and no sentence telling them what to do.

The high finding is the strongest argument yet for Wido's canary rule
cutting both ways. The build ran thirty focused canaries and all of them
passed, while the bed the design itself names as its gate was broken by
the same change. A canary proves the behaviour it names; it does not
prove that everything reading the old shape still works. So the
orchestrator ran the two beds the design's gate names, and that is what
caught it, independently and at the same time as the read.
