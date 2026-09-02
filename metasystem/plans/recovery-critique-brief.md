Working Mode: design
Orchestrator Identity: <dispatching seat>+<its session main> (dispatch delegate under goal recovery-to-good-state)
Date: 2026-09-02

# Goal

Round-1 critique of the ANALYSIS metasystem/plans/recovery-analysis.md
(landed, in your worktree), written by the Fable lane for goal
recovery-to-good-state (read metasystem/plans/goals/recovery-to-good-state.md
first: Wido's order, the seat's root-cause reading, the eight specimens,
Wido's idempotent start-and-stop word, and the done criterion). This is
the analysis-challenge rung of the bug ladder: you attack the diagnosis
before any design exists. The analysis confirms the root cause in
substance and corrects it in five places; it declares four gaps, and two
risks in its self-grade that you must settle first.

# Your mandate

1. SETTLE THE TWO DECLARED RISKS against the tree, quoting file and line:
   (a) Correction B claims metasystem up can itself create the
   drifted-owner state because the owner branch compares the full process
   reference (metasystem/internal/supervise/arming.go around 152-158 and
   716-725, metasystem/internal/supervise/identity.go around 223-241)
   while the armed inspection compares the start second only
   (metasystem/internal/supervise/verifyarmed.go around 32-41, 79, 103,
   calling census.Alive in metasystem/internal/census/verbs.go around
   17-19). Read both comparators. If the seconds-only function consults
   ticks and boot id after all, say so: correction B and slice S2 shrink
   to a tolerance rule. (b) The placement of specimen 2's thirteen silent
   minutes before the session announcement rather than during the
   supervision replacement rests on three record timestamps
   (metasystem/artifacts/agents/supervision/arming.log on the primary
   checkout) and no up transcript; judge whether the placement is
   supported or merely consistent, and whether the print-at-return claim
   (up prints nothing until it returns) is true of metasystem/internal/up/up.go
   and metasystem/cmd/metasystem/up.go.
2. VERIFY CORRECTION A: that only steward arm without a word and steward
   restart are terminal-gated (metasystem/cmd/metasystem/steward_verbs.go
   around 561-578), that up already takes over a dead owner, replaces a
   drifted generation and restarts a dead runner (the cited lines in
   arming.go and metasystem/internal/steward/runner.go), and that the
   drift wall is therefore a remedy-text defect at
   metasystem/internal/up/up.go around 392 plus a harness defect. Then
   answer the law question the analysis leaves open: after a rebuild under
   a standing relayed word, may up re-pin the engine itself, or must the
   seat call steward arm with the word first; state what each answer does
   to the enrollment law's protection.
3. ATTACK THE REFUSAL INVENTORY: the analysis admits it was built from
   greps for remedy and refusal patterns, not an exhaustive read. Sample
   the six packages for refusal strings that use neither word and report
   how many the inventory missed; verify the counts it gives (fourteen
   runnable, three terminal-only, one nonexistent flag, forty-five naming
   nothing).
4. ATTACK THE LEAK ACCOUNT (section 4): that every fixture launches
   runners and owners in their own session, that the runner loop never
   notices its bed vanished, that shutdown stops the owner but not the
   runner, that only one fixture disarms a runner, and that the census
   classifies orphans correctly while no verb reaps an unowned process.
   Read the fixtures it names and the runner loop; name any cleanup path
   it missed.
5. ATTACK THE ARC (section 6.2): is every slice at most 240 reserved
   minutes with a correction round intact; is the dependency order right;
   does each rehearsal fixture actually replay its specimen; does the
   arc deliver Wido's word (up idempotent from every partial state, a
   full stop with nothing lingering, never a terminal) or only part of it;
   is the disposition of the eight partial goals (absorbed, re-scoped,
   left) defensible against their records?
6. NEW FINDINGS only if material and grounded.

Findings quote the disagreeing text or code. Your sandbox is read-only:
verify by reading, do not run go. Declared gaps are residuals, not
findings, unless one hides a false claim. Zero material findings is an
acceptable, closing answer if the reading supports it.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
