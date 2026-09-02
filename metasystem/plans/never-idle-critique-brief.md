Working Mode: design
Orchestrator Identity: <dispatching seat>+<its session main> (dispatch delegate under goal never-idle-ironclad)
Date: 2026-09-02

# Goal

Round-1 critique of the ANALYSIS metasystem/plans/never-idle-analysis.md
(landed, in your worktree), written by the Fable lane for goal
never-idle-ironclad (read metasystem/plans/goals/never-idle-ironclad.md
first: Wido's order, the specimens, the four-part guarantee). This is the
analysis-challenge rung of the bug ladder: you attack the diagnosis before
any design exists. The analysis claims: twelve stop paths; that no machine
in the fleet has a working turn-exit gate today; that the steward treats
unclaimed backlog as no work and its idle sentence reaches only a local
log; eight holes that survive all seven bound goals as designed; an
eleven-slice arc. Six declared gaps ride the document, foremost that other
seats' artifacts were unreachable and the cause of the dead Stop hook on m1
is undecidable from files.

# Your mandate

1. VERIFY THE STRONGEST CLAIMS against the tree, one verdict each, quoting
   file and line: (a) the turn verdict blocks once per unchanged state and
   then allows (metasystem/internal/goal/turnverdict.go,
   metasystem/internal/report/stopblock.go); (b) the steward maps
   unclaimed backlog to no work and no health role or alert carries the
   idle verdict (metasystem/internal/steward/openwork.go, verdict.go,
   narrate.go, delivery.go, health.go); (c) revival serves delegates for
   claims and never a seat (metasystem/internal/steward/revive.go); (d) the
   m1 hook evidence claim in section 3.4 (no Stop attempt recorded since
   2026-08-30, no verdict since 2026-08-27): if a Stop verdict recorded on
   m1 after 2026-08-27 exists in artifacts/agents/supervision on the
   primary checkout, the analysis' own reject condition fires and you say
   so. Check whether m0's Claude turn-exit gate has landed on main since
   the analysis read the tree (git log on main for goal
   idle-with-backlog-alarm) and whether that changes claims (a) or (d).
2. ATTACK THE TWELVE STOP PATHS: is any path double-counted, not a stop
   path at all, or missing (a path a seat can take that leaves backlog
   unworked and that the list does not name)?
3. ATTACK THE SPECIMEN MAP: for each specimen, is the named mechanism the
   one that should have caught it, and is the stated reason it did not
   fire supported by the cited evidence or merely plausible? Rows marked
   relayed rest on other seats' records; say which of those you could
   verify from records on main (metasystem/records) and which remain
   unverified.
4. ATTACK THE GAP MAP AND THE HOLES: for each of the eight holes, is it
   really unowned once the seven goals land as designed (read their goal
   records and designs: metasystem/plans/turn-verdict-hardening-design.md,
   metasystem/plans/supervision-hook-root-design.md,
   metasystem/plans/alert-channel-design.md, metasystem/plans/goals/watch-verb.md,
   metasystem/plans/goals/idle-every-runtime-enforcement.md,
   metasystem/plans/goals/idle-with-backlog-alarm.md,
   metasystem/plans/goals/seat-mutual-awareness.md)? Name any hole that a
   goal already owns, and any hole the analysis missed.
5. ATTACK THE SPLIT: is every slice at most 240 reserved minutes with a
   correction round intact; is the dependency order right; does each
   fixture actually replay its specimen; does the proposed disposition of
   idle-with-backlog-alarm (its causal half yielding to
   turn-verdict-hardening so one design owns turnverdict.go and one human
   stop marker exists) hold, given both goals are claimed and Wido chose
   to land the Claude gate now?
6. NEW FINDINGS only if material and grounded.

Findings quote the disagreeing text or code. Your sandbox is read-only:
verify by reading, do not run go. Declared gaps are residuals, not
findings, unless one hides a false claim. Zero material findings is an
acceptable, closing answer if the reading supports it.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
