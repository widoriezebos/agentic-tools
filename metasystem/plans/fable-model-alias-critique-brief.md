Working Mode: design
Orchestrator Identity: m3+main-1788172645-85501-aa86ee (dispatch delegate under goal fable-model-alias)
Date: 2026-09-03

# Goal

Independent critique of metasystem/plans/fable-model-alias-design.md (landed,
in your worktree; its brief is metasystem/plans/fable-model-alias-design-brief.md
and the goal is metasystem/plans/goals/fable-model-alias.md). Wido's two
orders, verbatim: "i want claude-fable-5 to be an alias for claude-fable-5.1
to avoid running into DESSIGNM-BEARING", then "I want it to be the alias for
the latest class 5 model, is that possible?". The design answers: yes, as a
tracked pointer, one line runtime.claude.model-alias.claude-fable-5=claude-fable-5-1
in the committed configuration, applied once inside ResolveRoster
(metasystem/internal/dispatch/roster.go) so that the hazard gate, the cap
key, the fingerprint, the record fields and the adapter's --model argument
only ever see the canonical id; the pointer moves only by a landing of that
line on his word. It extends metasystem/plans/fable-5-1-rollover-design.md
(R-46-m0b) and must not contradict its live-round safety argument.

# Your mandate

1. ATTACK THE SINGLE POINT OF APPLICATION. The design claims that aliasing
   the three values inside ResolveRoster (roster id, requested id, --model
   override) is sufficient and that every later consumer is downstream of
   it, and lists what would still see the old id if the alias were applied
   later. Verify against the shipped code: the shell dispatcher's use of the
   verb's JSON, compose-role-packet and its hazard gate
   (metasystem/internal/dispatch/composition.go,
   metasystem/internal/dispatch/hazard.go), claim-launch and the fingerprint
   (metasystem/internal/dispatch/claim_fingerprint.go), build-record
   (metasystem/internal/dispatch/build.go), the steward's revive path
   (metasystem/cmd/metasystem/steward_verbs.go), follow-ups re-reading the
   latest record, and the runtime adapters under metasystem/scripts/agents.
   Any path by which the literal claude-fable-5 can still reach the Claude
   CLI or a record's requestedModel/effectiveModel/canonicalModelKey is a
   material finding (R-46-m0b: the retired id never reaches the CLI).
2. ATTACK THE FOUR GAPS the delegate reported instead of filling. Give a
   grounded recommendation on each; where the answer is Wido's to give, say
   so and say what the question is:
   (a) READ ORIGIN. The design honors the alias key from the committed
   configuration only and refuses it by name in the machine-local overlay
   and the environment, mirroring how budget law keys are read
   (metasystem/internal/config/budget.go). Wido's orders do not say this.
   Is the restriction a necessary consequence of R-46-m0b and of "moved
   only by a landing on his word", or a policy the design invented? Would
   an overlay alias be any more dangerous than an overlay roster line that
   names claude-fable-5-1 directly (which is allowed today)?
   (b) STALE CAP ROWS. Cap rows are read on the canonical id only
   (metasystem/internal/dispatch/cap.go), so a machine-local
   cap.min.<role>.claude.claude-fable-5 row is silently never consulted and
   the chain falls to the pair or general row. Should the validator name
   such a row (and as what: refusal, or an informational line), given the
   non-goal of not forcing any seat to edit its overlay under this goal?
   Is there a case where the fallthrough gives a role a LARGER cap than its
   operator wrote, which is the direction the design says it protects?
   (c) FOLLOW-UP RECORDS. aliasedFrom is written only where a roster is
   resolved; follow-up rounds inherit the canonical id and carry no
   aliasedFrom. Does the "which seats still write the old id" sweep still
   answer correctly with round-1 records only, or must follow-ups copy the
   field?
   (d) ROLLOVER SHAPE. Chained aliases are refused, so when a 5.2 ships the
   line is edited in place (claude-fable-5 to claude-fable-5-2) and, if
   Wido wants claude-fable-5-1 retired the same way, a second line
   claude-fable-5-1=claude-fable-5-2 lands beside it. Is that shape sound,
   and does the validator rule "source absent from maximal-models" then
   force the maximal-models edit and the alias edit into the same landing?
   Say whether that coupling is right.
3. ATTACK THE VALIDATOR RULES AND THE TESTS. Does each rule in section 2
   have a test in section 5 that discriminates it? Is the "target not
   required in maximal-models" decision right, or can it leave a seat with
   an alias whose target the gate then refuses, producing the same refusal
   Wido hit with a less legible message? Do the existing fixtures that spell
   claude-fable-5 as an arbitrary string keep their meaning as the design
   claims (temp roots without the alias key), or does any of them read the
   real committed configuration?
4. ATTACK THE OVERRIDE DECISION. --model claude-fable-5 on a roster at
   claude-fable-5-1 becomes equal pairs, Overridden true, no escalation
   asked. Is there an escalation or fingerprint consequence the design
   missed (the fingerprint refuses non-canonical keys; is a canonical
   aliased id ever different from CanonicalModel of the target)?

Findings material and grounded, quoting the disagreeing text or code. A
clean return closes the design phase and the build dispatches; the
dispatching seat folds the gap recommendations before the build brief and
carries any that are Wido's to him.

# Constraints

Wall-clock budget: 30 minutes. Reading only; do not run the engine. Return
per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
