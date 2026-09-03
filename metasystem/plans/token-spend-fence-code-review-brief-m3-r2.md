Working Mode: implement
Orchestrator Identity: m3+main-1788172645-85501-aa86ee (dispatch delegate under goal token-spend-fence)
Date: 2026-09-03

# Goal

The one re-review (R-60-m1: "material findings return to the implementer
as one correction round and you review the corrected tree once") of the
token spend fence build, implementer chain fence-build-m3, after its
correction round fence-build-m3-r2. The first review
(job fence-review-code-m3-5, reviewedTree
2c0572533ba23f967e63eaab321d3f9e003a7e4c) found one material finding,
TSF-C1-seat-slug-not-git-toplevel, and recorded one brief violation
beside it, TSF-C1-unstubbed-tests-list-real-home. The correction round
was briefed by metasystem/plans/token-spend-fence-correction-brief.md
and its review-stage conformance passed at reviewedTree
750ca6cf9bc982d1316eeb1b2f7c52672cc238d7; its artefacts are
artifacts/agents/fence-build-m3/rounds/<n>/diff.patch and review.json,
n=2 for this round. Review that diff, never the delegate's own summary.
The first review's brief, metasystem/plans/token-spend-fence-code-review-brief-m3.md,
and everything it cites bind unchanged; the design
(metasystem/plans/token-spend-fence-design.md, revision 2) is not reopened.

# Review brief

The corrected tree differs from the reviewed tree of the first review by
one worktree commit (e9c4ef60, three files: the spend package's transcript
reader and its measure test file, and the steward's spend fence test file).
Two ordered questions, in the code-critique skill's two layers:

LAYER 1, conformance of the correction. (a) Correction 1: the seat
meter's slug prefix and cwd rule are now the Git toplevel's exactly as
design section 3 words them (walk every ~/.claude/projects directory
whose name starts with the toplevel's slug; keep a line whose cwd is at
or below the toplevel and not below the design's second guard, the
worktrees directory under the agents control plane); an
unresolvable toplevel is one `seat unreadable` unmeasured entry, Measure
never fails, the role stays alive; the toplevel resolution mechanism the
implementer chose (walking parents for a .git file or directory) is one
of the two the correction brief allowed and is named in the return;
TestSeatSlugIsGitToplevelNotRepoRoot exists in the spend package and discriminates (both slug
directories counted, the foreign slug not read). (b) Correction 2: no
test in the spend package or the steward package reads the real ~/.claude;
name the mechanism (the return says a steward TestMain sets a temporary
HOME) and judge whether it covers every reaching site. (c) Nothing else
moved: the seven findings recorded non-material were not corrected, no
non-goal touched, no test weakened, the diff stays inside the declared
diffBoundary (36 files, identical to round 1's list).

LAYER 2, adversarial, on the changed lines only: the parent walk (a
`.git` file in a linked worktree; a repoRoot that IS the toplevel; a
symlinked repoRoot; a `.git` entry that is neither file nor directory);
the cwd rule (a cwd equal to the toplevel; a cwd under a nested
worktrees control-plane directory at any depth; a cwd with `..` segments;
a cwd on another toplevel whose path merely starts with this one's
string); the slug prefix (another checkout whose toplevel path starts
with this one's, e.g. `…-m3` versus `…-m3-metasystem` versus a sibling
`…-m30`: the design's prefix rule reaches all of them by design, say so
rather than finding it); and the TestMain (does it leak the temporary
HOME across packages, does removal on failure hide a failing test).

Materiality criterion, verbatim: would the change ship a defect,
violate its brief, or damage what certifies it? Count only material
findings in the verdict. This is the once; there is no further
correction round under R-60-m1, so a material finding here goes to
Wido, not back to the implementer.

Run what the sandbox allows: `go build ./...`; `go test` over the spend
and steward packages, once normally and once with HOME set to a path
that does not exist (the correction brief's proof); and the
`go list -deps` proof that no admission package (dispatch, goal,
goalbudget) imports the spend package. Report what
could not run.

Return format: the code-critic schema; stable identifiers TSF-C2-<name>;
carry the reviewedTree from review.json into the return; for each of the
first review's nine findings state in one line whether the corrected
tree changes its standing (expected: the material one closed, the
real-home one closed, seven unchanged).

# Constraints

Wall-clock budget: 30 minutes. Do not edit the implementation; findings
only.

# Gap Rule

stop and report a gap; never fill it silently.
