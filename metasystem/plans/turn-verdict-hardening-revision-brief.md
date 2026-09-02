Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal turn-verdict-hardening)
Date: 2026-09-02

# Goal

Revision 4 of metasystem/plans/turn-verdict-hardening-design.md (revision 3
landed, in your worktree). Sol's round-3 register is
metasystem/records/misc/turn-verdict-hardening-critique-r3.md: four material
findings, all in HUMANSTOP (section 5, slices 4a and 4b) and the R3 Move
quoting (section 1.2.1). Slices 1a through 3 drew no finding — do not touch
sections 1.2.0, 2, 3, 4 or their slice rows except where a fold below says
so. Edit in place; diffBoundary is that one file. Mark each closure with the
finding identifier. This is a SMALL round: four targeted folds, then the
consistency pass. Keep it under fifteen minutes.

# The folds, by id

1. TVH-R2-HUMANSTOP-SEAT-AUTHORITY-UNSPECIFIED (critical, still partial). The
   general classifier checks the exact caller only for an announcement and
   starts runtime-signature checks at its parent
   (metasystem/internal/lease/directinvoker.go, metasystem/internal/lease/classify.go),
   so a delegate binary that spawns the verb directly is judged by its
   ancestors and can be classified as its ancestor MAIN — minting the main
   seat's marker. Specify the caller check the relay form needs: the direct
   invoker must itself be the announced MAIN session (or its runtime process),
   not merely have one among its ancestors; name the existing direct-invoker
   facility in that package if it fits, or the exact additional check; add
   the test where a DELEGATE child invokes the verb and is refused with its
   class named.
2. TVH-R3-F11-DISCOVERED-AFTER-HUMANSTOP (high). F11 (verdict-state lock or
   write failure) is discovered in the state-file phase, AFTER the single
   marker phase — so a valid marker cannot rescue it as sections 3.2(d), 7
   and slice 4b promise. Choose one and specify it: move the state-file
   phase before the marker comparison, or perform a second compare-and-consume
   when F11 fires. Fix section 7's F11 row and slice 4b's
   `TestHumanstopRescuesStateFileFailure` to match.
3. TVH-R3-HUMANSTOP-MARKER-PATH-UNSAFE (high). The marker path embeds the
   machine nickname raw; the vocabulary allows any non-whitespace word
   (metasystem/internal/goal/actor.go, metasystem/internal/goal/verbs.go), so
   `region/a` nests and `../jobs/a` escapes the directory. Specify a
   traversal-safe, injective filename encoding for `<machine>+<lineage>`
   (state the encoding exactly), and add boundary fixtures with a slash, a
   dot-dot segment and a very long nickname to slice 4a.
4. TVH-R1-R3-NAMES-ILLEGAL-EXIT (medium, still partial). The rendered Move
   lines pass `--root <root>` unquoted; the repository already owns the rule
   in metasystem/cmd/metasystem/goalsync_verbs.go (shell-single-quote paths).
   Apply that rule to every rendered path and lineage in section 1.2.1, and
   extend `TestReadyMoveLinesParseUnderParseSyncFlags` in slice 1a with a
   hostile root (space, dollar, backtick, quote) so the tokenizer proves it.

Consistency pass over sections 7, 8, 9, 10 and 11 only where these four
folds touch them; bump the revision header to 4 naming the round. R-31: no
benchmarks.

# Constraints

Wall-clock budget: 15 minutes. Design only; edit nothing but the design file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the design file.

# Gap Rule

stop and report a gap; never fill it silently.
