Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal alert-escalation-channel)
Date: 2026-09-01

# Goal

Recover and certify revision 8 of metasystem/plans/alert-channel-design.md.
CONTEXT RECORDED HONESTLY: the authoring round (job
implementer-c002e6035a243bdbc1400067, this same Fable design lane) completed
the revision in its worktree and died on the runtime spend cap BEFORE emitting
its return — the same failure class recovered once already this goal
(revision 7's chain), fourth specimen for goal budget-death-on-return. Its
product is preserved verbatim as a unified diff at the read-only path under
Inputs. Apply it, verify it against the fold-8 mandate, fix only what
verification finds missing, and return properly.

# Workspace

The delegate worktree the dispatcher created for this job. Exactly one file
changes: metasystem/plans/alert-channel-design.md (revision 7 is in the
worktree).

# Inputs

- The recovered revision-8 diff, apply verbatim:
  /home/wido.guest/m0b/agentic-tools/metasystem/artifacts/agents/implementer-c002e6035a243bdbc1400067/rounds/1/recovered.diff
  (328 insertions, 112 deletions, one file; produced against the same
  revision 7 your worktree contains — from the worktree root, `git apply
  <that path>` applies cleanly).
- The fold-8 mandate: plans/alert-channel-fold8-brief.md (in your worktree).
- The critique register the mandate folds:
  records/misc/alert-channel-critique-r7.md (in your worktree).

# The task, in order

1. Apply the diff to metasystem/plans/alert-channel-design.md in your worktree.
2. VERIFY against the fold-8 mandate: each of the nine findings
   (AC7-PRODUCER-STATE-001, AC7-PRODUCER-ATOMICITY-001, AC7-STOP-OUTCOME-001,
   AC7-JOB-WRITER-001, AC7-MESSAGEREF-PERSISTENCE-001, AC7-SEND-OUTCOME-001,
   AC7-COMPOSER-BYTES-001, AC7-DEDUP-ENCODING-001, AC7-TICK-ERROR-PATH-001)
   is folded or refuted BY ID and none is silently narrowed; the
   self-consistency pass statement names its section pairs; the self-grade
   carries the third-gap-stop reject condition; Wido's standing design words
   are untouched (adapter abstraction, Telegram first, session bridge second
   consumer, Slack threading via conversation identity, the two slice-1
   producer classes).
3. If a mandated element is missing or half-done, complete exactly that
   element; change nothing else.
4. Return version-2 implementer JSON; diffBoundary exactly
   metasystem/plans/alert-channel-design.md; whatWasDone maps each finding id
   to its resolution in one line each and states what step 2 found.

# Constraints

Wall-clock budget: 20 minutes. Recovery-and-verify, not re-authoring.

# Gap Rule

stop and report a gap; never fill it silently.
