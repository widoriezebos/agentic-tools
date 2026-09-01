Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal alert-escalation-channel)
Date: 2026-09-01

# Goal

Recover and certify revision 7 of metasystem/plans/alert-channel-design.md.
CONTEXT RECORDED HONESTLY: a prior fresh-context design round (job
implementer-c1fa9f5accadf92b074c9e34, this same Fable design lane) completed
the revision in its worktree and died on the runtime spend cap BEFORE emitting
its return; its session cannot be resumed (the machinery refuses follow-up on
this failure class). Its complete product is preserved verbatim as a unified
diff at the read-only path named under Inputs. You are the same design lane
with fresh context: apply it, verify it against the mandate, fix only what
verification finds missing, and return properly.

# Workspace

The delegate worktree the dispatcher created for this job. Exactly one file
changes: metasystem/plans/alert-channel-design.md (revision 6 is in the
worktree).

# Inputs

- The recovered revision-7 diff, apply verbatim:
  /home/wido.guest/m0b/agentic-tools/metasystem/artifacts/agents/implementer-c1fa9f5accadf92b074c9e34/rounds/1/recovered.diff
  (216 insertions, 55 deletions, one file: metasystem/plans/alert-channel-design.md;
  produced against the same revision 6 your worktree contains — from the
  worktree root, `git apply <that path>` applies cleanly).
- The original fold-7 mandate, restated under "The task" below (its authority:
  plans/alert-channel-fold7-brief-m0b.md, also readable).

# The task, in order

1. Apply the diff to metasystem/plans/alert-channel-design.md in your worktree.
2. VERIFY the revised document against the fold-7 mandate:
   a. The four cross-section contradictions are resolved decisively:
      MessageRef retention decided into ONE slice with sections 5a and 11
      agreeing; AdapterSend and 11a.7 as one context-bearing contract; the
      external steward tick command's DeliverDueAlerts wiring stated in
      sections 5 AND 11; the truncation law exact (tail bytes, UTF-8
      boundary, which fields shorten).
   b. Wido's binding word is folded: the alert classes include "delegate job
      failed under a claimed goal" (carrying goal id, job id, failure reason,
      deduplicated per job) and the breach-stop stop-awaiting-resume is an
      explicitly wired slice-1 producer — both in the class enumeration and
      mechanically in section 11a.
   c. The self-consistency pass statement names its section pairs; the
      self-grade carries the third-gap-stop reject condition.
3. If verification finds a mandated element missing or half-done, complete
   exactly that element; change nothing else.
4. Return version-2 implementer JSON: diffBoundary exactly
   metasystem/plans/alert-channel-design.md; whatWasDone maps each
   contradiction to its resolution, names the consistency pass's section
   pairs, and states what step 2 found (adopted whole, or what was completed).

# Constraints

Wall-clock budget: 25 minutes. This is recovery-and-verify, not re-authoring.
No design content changes beyond step 3's narrow condition.

# Gap Rule

stop and report a gap; never fill it silently.
