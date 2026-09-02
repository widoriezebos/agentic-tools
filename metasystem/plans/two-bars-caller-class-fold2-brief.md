Working Mode: design
Orchestrator Identity: m1b+main-1788333346-60696-6a3256 (dispatch delegate under goal two-bars-for-changes)
Date: 2026-09-02

# Goal

Revise your design metasystem/plans/two-bars-caller-class-design.md to
revision 2 by folding the three ACCEPTED findings of critique round 1
(critic chain two-bars-cc-crit-3, gpt-5.6-sol; dispositions with the
orchestrator's evidence in
metasystem/plans/two-bars-caller-class-dispositions.md). The design is
yours: rewrite the affected sections in one pass rather than patching
sentences, keep every line-and-file grounding, and re-run your own
reject condition at the end.

# Workspace

The delegate worktree the dispatcher created for this job. Read
anything; write exactly one file, the existing
metasystem/plans/two-bars-caller-class-design.md (edit in place; mark
the header "revision 2" and keep a two-line changelog naming the three
findings).

# What revision 2 must settle

1. THE WORKER PATH (TBCC-R1-LAWFUL-DELEGATE-COMMIT-PATH). Your section
   10 reject condition fired: a worker committing inside its own
   dispatched worktree is an anticipated, lawful flow (the codex
   adapter grants the worktree's git metadata for exactly that,
   metasystem/internal/adapter/codex.go:59-64 and :165-171; the
   dispatcher builds the object quarantine for delegate git writes,
   metasystem/scripts/agents/dispatch.sh:1374-1402; and the pre-commit
   guard refuses every non-HUMAN raw commit without the wrapper token,
   metasystem/scripts/agents/pre-commit-guard.sh:50-58, so the "use
   plain git" fallback is a dead end). Design a FOURTH verdict path,
   `worker`, with these constraints fixed by the orchestrator:
   - A DELEGATE is on the worker path only when its ancestry maps to a
     dispatched job AND the repository root the wrapper runs in is that
     job's own worktree (metasystem/artifacts/agents/worktrees/<job>).
     The classifier already returns the adapter ancestor's pid
     (classify.go:343-346); the adapter process carries the job's
     instance tag in its command line (codex.go:58 writes
     `metasystem_instance_tag=...`; the claude and devin adapters carry
     their own tag, find the exact carrier for each, cite it). Decide
     whether the engine verb derives the job from that tag, or from the
     job record whose worktree path contains the root, or both; refuse
     on any disagreement.
   - A DELEGATE anywhere else (the main checkout, another job's
     worktree, a stale worktree whose job is terminal) stays REFUSED
     with the recorded sentence.
   - The worker path is ungated by the landing bar, because a worktree
     commit never lands: landings ride `land.sh --chain` from the main
     checkout after conformance. Say which of the wrapper's proofs
     (the fast gate, the audit, the landing observation) run on the
     worker path and which are skipped, and why; the observe-only
     stamping may stay.
   - The Machine trailer on a worker commit names the job:
     `<nickname>+delegate:<jobId>`; say how lineage is set on the
     verdict for this path.
   - Update the caller table (section 4), the branch table (section 2),
     the verdict struct (section 3) and the fixture list (section 7)
     for the new path, and rewrite the weakest-claim and reject
     condition in section 10.
2. THE STUB CONTRACT (TBCC-R1-FIXTURE-STUB-CONTRACT). Section 7 must
   specify, so the implementer invents nothing: the exact `json get`
   answers the bed's stub gives for --field path, claimEpoch, mainId,
   lineage, message and code, keyed on the commit-authority answer it
   is configured to give (read the stub at
   metasystem/scripts/agents/static-reproof-fixtures.sh:220-252 and
   :400-424 and the consumer-wiring stub at
   metasystem/internal/behaviorsurface/consumer_wiring_test.go:66-76);
   the default verdict rule that keeps every existing `__lease-held 1`
   and `__lease-held human` leg meaningful; a `lease commit-token` stub
   that writes a marker file so "no token was minted" discriminates;
   and how the run-held stub execs its argv.
3. THE NEGATIVE LEGS (TBCC-R1-NEGATIVE-BRANCH-PROOFS). Add three named
   shell legs: an empty or unknown `path` refuses with the named
   message; an accepted human or agent verdict with empty `lineage`
   refuses with the named message; an `agent` verdict while re-entering
   as `human` refuses with the mismatch message. Each leg states the
   stub answer, the entry point, the expected exit code, stderr bytes,
   and that HEAD is unchanged.

Ground every new claim in file-and-line evidence from the worktree per
metasystem/docs/design/design-principles.md; where a seam is
implementer-private, say so in section 9. Self-grade again.

# Constraints

Wall-clock budget: 30 minutes. Edit only the design file. Do not
weaken anything round 1 did not touch.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file.

# Gap Rule

stop and report a gap; never fill it silently.
