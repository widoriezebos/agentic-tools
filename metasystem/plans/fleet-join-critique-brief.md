Working Mode: design
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal fleet-join-bootstrap)
Date: 2026-09-02

# Goal

Round-1 critique of metasystem/plans/fleet-join-bootstrap-design.md
(revision 1, landed, in your worktree), the design for goal
fleet-join-bootstrap (read metasystem/plans/goals/fleet-join-bootstrap.md
first; its Intent was corrected on the design's finding that no
refs/metasystem fetch refspec is needed). The design decides one
composing script, scripts/agents/join-fleet.sh, over an engine verb or an
up mode; specifies seven steps with corrected refusal texts; a committed
roster template and a --resolved flag for config validate; nine message
corrections with file and line; a six-scenario fixture; two slices with
reservation accounting. Five declared gaps ride its return, foremost that
no record describes what the m0 and m0b guest clones hand-fixed.

# Your mandate

1. ATTACK THE OWNER DECISION (section 1): is a composing script the right
   owner given the rule that an existing owner is preferred over a new
   surface (metasystem/docs/project-adaptation.md around line 42,
   metasystem/skills/take-a-step-back/SKILL.md around lines 18-21), the
   architecture (metasystem/docs/architecture.md) and the precedent of
   scripts/adopt.sh and scripts/agents/second-session.sh? Are the two
   refusals of the alternatives (the engine cannot build itself; up is
   forbidden to enroll, metasystem/internal/up/up.go) true as stated?
2. VERIFY THE LEDGER CORRECTION (section 3) against
   metasystem/internal/goal/txn.go around lines 49-61, 126-127 and
   409-413: does the first goal fetch on a truly fresh clone create the
   accepted pointer, or is there a state (no remote, no main fetched, the
   sync-mode file absent) where it refuses? Note that origin carries, under its
   refs namespace for the metasystem ledger, leftover m0 accepted and materialized-base refs; the design
   says nothing in the tree pushes or reads them. Confirm or refute.
3. WALK THE SEVEN STEPS (section 1): for each, is the precondition
   checkable, is the corrected refusal text true of the code path, does
   the named next command exist and can the seat's class run it, and is
   the re-run on an existing seat after an engine change genuinely
   idempotent? Check the honest stop where enrollment needs a human word
   against metasystem/cmd/metasystem/steward_verbs.go and the relayed-word
   path R-37-m3 authorizes (metasystem/memory/rulings.md).
4. ATTACK THE TEMPLATE AND THE VALIDATOR FLAG (section 2): is the hand-set
   minimum complete against metasystem/internal/config/resolve.go and
   validate.go and the machine nickname resolution in
   metasystem/internal/goal; does --resolved change any existing
   validation contract.
5. ATTACK THE MESSAGE CORRECTIONS (section 4) and THE FIXTURE (section 5):
   each corrected text against its current text and line; each fixture
   scenario against what a sandbox can observe (KI-15); anything the
   fixture claims that it cannot assert.
6. ATTACK THE SLICES (section 6): reservation accounting (240 reserved
   minutes as two launches at cap 120), the borrowed critique estimate the
   design marks unsupported, and whether slice 2 fits with a correction
   round intact.
7. NEW FINDINGS only if material and grounded.

Findings quote the disagreeing text or code. Your sandbox is read-only:
verify by reading, do not run go. Declared gaps are residuals, not
findings, unless one hides a false claim. Zero material findings is an
acceptable, closing answer if the reading supports it.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
