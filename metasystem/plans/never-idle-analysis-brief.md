Working Mode: design
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal never-idle-ironclad)
Date: 2026-09-02

# Goal

Author the ANALYSIS for goal never-idle-ironclad: read
metasystem/plans/goals/never-idle-ironclad.md first; its Intent carries
Wido's order verbatim, today's honest answer, the specimens, and the
four-part guarantee the goal owns. This round is analysis, not design: it
establishes what exists, what fired, what did not and why, and where the
holes are, so that the design round after the critique designs against
facts. Write exactly one NEW file named never-idle-analysis.md in the
metasystem plans directory. Every claim about the tree cites file and line;
read the seams before you write about them.

# Workspace

The delegate worktree the dispatcher created for this job. Read anything;
write only that one new analysis file.

# What the analysis must establish

1. THE STOP PATHS, enumerated. Every way a seat ends up idle while the
   backlog holds claimable work: the harness Stop hook letting a turn end
   (scripts/agents/supervision-hook.sh, the verdict verb it calls, and the
   block-once state it consults); the seat ending a turn on a decision-ask
   or a permission-classifier refusal while other work needed no answer;
   session death (process gone); the human's instruction channel failing
   silently (remote-control messages stranded unsent in a tmux pane, seen
   on m0, m0b and paper on 2026-09-02); a seat parked on one blocked item
   without claiming the next. Name any path you find that is not listed.
2. THE MECHANISMS, as they are in the tree today and in the six bound
   goals: the steward's idle detection (metasystem/internal/steward/narrate.go
   around line 149 and the claimed-goal-appetite and claimed-goal-delivery
   health roles), the Stop hook and its verdict (the goal verbs' turn
   verdict, metasystem/internal/goal/turnverdict.go), health alerts and
   the narrator digest (where an idle verdict goes and who reads it),
   revival (metasystem/internal/steward/revive.go: what it revives and
   what it cannot), and the in-flight designs: idle-with-backlog-alarm
   (m0: the turn-exit gate and the human stop marker, session stop verb),
   turn-verdict-hardening (metasystem/plans/turn-verdict-hardening-design.md),
   watch-verb (metasystem/plans/goals/watch-verb.md and its design records
   under metasystem/records), alert-escalation-channel
   (metasystem/plans/alert-channel-design.md), supervision-hook-wrong-root
   (metasystem/plans/supervision-hook-root-design.md), seat-mutual-awareness.
   For each mechanism: what it guarantees, on which machines it is live,
   and its failure modes.
3. THE SPECIMEN MAP. For each specimen in the goal's Intent: which
   mechanism should have caught it, whether that mechanism existed at the
   time, whether it fired, and the exact reason it did not close the loop
   (with evidence: health records, narration lines, hook logs under
   artifacts/agents/supervision/hooks.log, the goal records).
4. THE GAP MAP against the four-part guarantee: for each part, what
   exists, what is in flight and where, what is missing entirely. Say
   plainly which of the six bound goals, if all landed as designed, would
   still leave a hole, and which hole no goal owns. The instruction-channel
   failure and the seat-nudge (re-injecting the continue order through a
   seat's declared channel, such as its tmux pane) are suspected to be
   unowned; confirm or refute.
5. THE SPLIT. Propose the arc: slices of at most 240 reserved minutes, in
   dependency order, each naming the fixture that replays a specimen and
   must refuse or recover it. State which existing goals are absorbed,
   re-scoped or left as they are.

Self-grade per the house rule: confidence, weakest claim, reject condition.
The analysis is challenged by an independent critic before any design is
written; write so that every sentence can be checked against the tree.

# Constraints

Wall-clock budget: 45 minutes. Analysis only; no design decisions, no
builds, no benchmarks (R-31). Edit nothing but the analysis file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one analysis file
named under Goal.

# Gap Rule

stop and report a gap; never fill it silently.
