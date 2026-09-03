Working Mode: design
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Round-1 critique of metasystem/plans/path-class-manifest-design.md
(revision 1, landed, in your worktree), the design for goal
path-class-manifest (read metasystem/plans/goals/path-class-manifest.md
first: Wido's order, the problem, the four classes, the done criterion).
The design: one plain-text manifest with four classes and longest-prefix
matching; a path class verb; three consumers (the landing evaluator, the
waiver rule, register carriage) reading it and nothing else; three
deletions with nine reader sites; record semantics by location; ledger
paths refused in any wrapper landing; fixtures; one build slice.

# The stop criterion, which binds you (Wido's word, 2026-09-03)

A finding is MATERIAL only if it changes what gets built, and you must
name the build artifact that changes (a function, a file, a fixture, a
manifest row). Anything else is a note, listed separately and not
counted. This chain gets one review, one fold, one closing review, then
the build; whatever is still disputed at the closing review becomes a
named test obligation for the builder. Write findings so the fold can act
on each in one edit.

# Your mandate

1. THE MANIFEST TABLE (section 1): check every row against the tracked
   tree (git ls-tree at the repository root and inside metasystem/). Name
   any tracked top-level entry the table misses, any row that classifies
   a behavior path as record or the reverse, and whether the two lookup
   key spaces (installation-relative inside metasystem/, repository-
   relative outside) can ever give one path two answers.
2. THE VERB (section 2) and THE CONSUMERS (section 3): verify every
   function named with file and line exists and that the proposed change
   is complete: the landing evaluator's class-by-declaration table
   against metasystem/internal/landing/observe.go; the waiver rule
   against every reader of metasystem/scripts/agents/instruction-bearing-paths.txt;
   register carriage against loadCarriagePolicy and
   metasystem/scripts/agents/register-carriage-paths.txt.
3. THE THREE DECLARED GAPS, one verdict each: (a) plans/goals-accepted.json
   is named in metasystem/internal/goal/goal.go and deleted in migrate.go
   but absent from the tree; should the manifest carry it? (b) ledger
   paths refuse in ANY wrapper landing because goal-verb commits bypass
   the wrapper (metasystem/internal/goal/txn.go builds them with
   commit-tree); is that sound, and is there a wrapper landing that must
   lawfully touch a goal file (reconcile, the human-word paths)? (c) the
   --goal wrapper flag and the stamped Goal-Item trailer: can
   metasystem/scripts/agents/commit.sh's proved-tree postcondition carry a
   new flag, and does the evaluator get that input on every path it
   needs?
4. RECORD SEMANTICS (section 5): ownership by name prefix for goal-owned
   designs and by machine nickname for handoffs, everything else a
   shared register; is that safe enough for the second bar, or does it
   let a seat revise another seat's design through the shared-register
   door? Say what would change in the build if not.
5. THE DELETIONS (section 4) and FIXTURES (section 7): confirm the nine
   reader sites are all of them (run the same search the grep fixture
   proposes); each fixture deterministic and at the seam that can fail.
6. SIZE (section 8): does the build fit one slice of 240 reserved
   minutes with a correction round intact; is the diff boundary complete.

Findings quote the disagreeing text or code. Your sandbox is read-only:
verify by reading, do not run go. Zero material findings is an
acceptable, closing answer if the reading supports it.

# Constraints

Wall-clock budget: 35 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
