Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal landing-receipt-races-the-narrator-digest, tier 3, hazard DESIGN-BEARING, code critique round two of chain lrr-build1)
Date: 2026-09-06

# Review brief: landing-receipt-races-the-narrator-digest, round two

Round budget: three focused rounds for the goal's tier-3 box; this is
round two, after one fold. The orchestrator adjudicates every finding; you edit nothing.

Threat model: one seat landing reviewed chains on one machine, no
adversaries. In scope: a receipt that names a tree the command did not
run against; a receipt that survives a change to the candidate during
the command; a temporary worktree left behind on any exit path or
placed under the repository; a receipt file the unchanged landing
evaluator would refuse; the live root's index or working tree changed
by the verb; a Go test weakened. Out of scope: hostile inputs, the
evaluator's reading of receipts, land.sh, and the full battery's own
contents.

Scope: the computed diff of chain lrr-build1 after follow-up round two
(job lrr-build1-r2) against its base. Round one implemented
metasystem/plans/landing-receipt-races-the-narrator-digest-brief.md
(landed c3f47f56); round two folded your findings per
metasystem/plans/landing-receipt-races-the-narrator-digest-fold-r2-brief.md
(landed f7d457f7): the repository-wide worktree prune is gone from the
helper's cleanup with a comment saying why, the narrator-append test
asserts a zero exit, and the flag help names the isolated workspace.
Both bind. The computed diff is
metasystem/artifacts/agents/lrr-build1/rounds/2/diff.patch (it carries the
new helper file under the gittree package, the receipt changes and the
verb's help text) and its reviewed tree is
7f838321c4b079df3927f457b34ab1151020072f; carry that hash into your
return exactly.

# Goal

Say whether the change ships a defect, violates its brief, or damages
what certifies it. Apply the materiality criterion verbatim:

> Would the change ship a defect, violate its brief, or damage what certifies it?

# What to attack

0. The fold first: no `git worktree prune` is invoked anywhere in the
   receipt path (production code; the gittree package's own object-prune
   tests are unrelated and untouched); the helper's own worktree is still
   unregistered and removed on every path without it; the comment names
   the hazard; the narrator-append test asserts exit status zero.

1. The worktree reproduces the exact staged tree: how the index is
   carried into the detached worktree, and that `git write-tree` there
   equals the named tree before the command runs; what happens with
   untracked files, ignored files, submodules, and a staged deletion.
2. The command's environment and working directory: the same
   environment as before, the worktree's metasystem directory as cwd,
   output where it was; the engine binary the fixtures need exists or
   is built there.
3. The after-check: the worktree tree still equals the named tree;
   a command that changes a tracked file is refused with a sentence
   naming the candidate change; the receipt's binding fields name the
   worktree trees, and readTestReceipt in
   metasystem/internal/landing/receipt.go accepts the file unchanged.
4. Cleanup: the worktree is removed and pruned on success, on a
   failing command, on a refused after-check, and on a signal; its
   path is under the system temporary directory; a leftover from a
   crashed earlier run does not break the next run.
5. The live root: its index and working tree are untouched by the verb;
   the receipt lands in the live root's receipts directory; the
   before-run posture check still requires the named tree to equal the
   live index and projection.
6. Tests: the new legs (a narrator append during the command stays
   green; a candidate change inside the worktree is refused) prove
   what they claim and cannot pass vacuously; existing receipt and
   observe tests unchanged. Conformance: only files under the brief's
   May-touch list; nothing under plans.

# Evidence you may run

If your runtime gives you a shell, from the reviewed worktree root (the
metasystem directory): `go test -count=1 ./internal/landing`,
`go vet ./internal/landing`, `gofmt -l ./internal/landing`. The real
verb against the full battery is not yours to run; the orchestrator
runs it seat-side and lands the chain with a receipt the new verb made.

# Expected Return

The code-critic role's version-3 JSON with every required property,
the reviewed tree hash carried exactly, findings only, each with a
stable id, severity, a boolean material value, a plain-English claim
and evidence marked ran, read or inferred; one rigor row per material
finding. A round with zero material findings is the closure candidate;
say so plainly and list what you checked.

# Gap Rule

stop and report a gap; never fill it silently.
