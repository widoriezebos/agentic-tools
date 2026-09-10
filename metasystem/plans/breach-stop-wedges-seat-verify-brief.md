Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal breach-stop-wedges-seat, live proof of chain bsws-build1b-20260909, final work round bsws-build1b-20260909-r8)

Date: 2026-09-10

# Goal

Goal breach-stop-wedges-seat (metasystem/plans/goals/breach-stop-wedges-seat.md):
a breach-stopped claim must stop wedging its machine. The chain has been
built and read four times; this job is its live proof, the evidence the
chain needs to close. You are the verifier: drive the changed behaviour
through the real command surface and report what you observed. Do not
certify; do not infer from green tests.

# The behaviour to observe

With the engine built from the chain (below), on one machine holding goal A
that has been breach-stopped:
1. `goal claim` of another goal B is confirmed (before the chain it was
   refused with "the quota is one claim per machine").
2. `goal next` and the turn verdict name B as the work and print one
   FENCED line for A.
3. A dispatch for B is admitted and starts (before the chain, admission
   refused every dispatch on that machine with A's live stop reason).
4. A dispatch for A itself is still refused, naming A's stop.
5. `goal park`, `goal done` and `goal release` on A are still refused with
   "only goal resume may clear its launch fence".

# Facts

- The chain's engine, built from the reviewed round-8 tree, is at
  /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1d/1256ce4b-2cff-499e-9e81-ed1cf7deb77a/scratchpad/proof-engine-r8.
  Use it, as METASYSTEM_BIN and as the binary you invoke, for every step
  above. For the "before" contrast use the checkout's current trunk engine,
  metasystem/bin/metasystem.
- Do not touch this checkout's live ledger or its claims. Build the drive
  in a scratch converted checkout the way the goal command-line bed does:
  read metasystem/scripts/agents/goal-cli-fixtures.sh for how it creates a
  converted checkout with a synced goal ledger, enrolls it, opens and claims
  goals, and breach-stops a claim (the bed calls
  `dispatch.sh __breach-stop-goal --goal <id> --revision <n>` with
  METASYSTEM_DELEGATE_INTERNAL=1; see also metasystem/scripts/agents/dispatch-fixtures.sh,
  scenario brain-breach-stop-exempt). Reuse the bed's helpers rather than
  inventing your own ledger; the fake runtime
  (metasystem/scripts/agents/adapters/fake.sh) is the dispatch target for B.
- The delegate sandbox denies process enumeration and some process-ownership
  proofs; if a step needs those, report exactly which step and what was
  denied, and stop there rather than inferring the result.

# Decisions (the orchestrator's; decided, not open)

D1. Evidence level for every step is `ran` with the exact command and the
observed output or exit code, read from the verifying command's own exit
code captured into a variable, never from a wrapper. Each of the five
behaviours above is one evidence entry, and the "before" contrast for
behaviours 1 and 3 is two more.

D2. You change nothing under the checkout. The scratch checkout lives under
the temp directory and is removed when you are done.

D3. If any observation contradicts the expected behaviour, stop and report
it as the riskiest part; do not work around it.

# Constraints

Read-only on the repository. Return per the verifier schema. Wall-clock
budget: 60 minutes. Plain English in every human-visible field.
