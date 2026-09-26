Working Mode: implementation
Orchestrator Identity: root Codex, main-1790272787-5030-13dde3
Date: 2026-09-26

| Unit | Changed lines |
| --- | --- |
| finish-public-recovery-and-complete-manual-runtime | 25000 |

Resume original Opus context, SAME primary checkout. Root fully read return8 and
the actual manual implementation. User requires complete intent and capability,
authorizes machinery bypass/ignoring stale hooks. No commits/pulls/pushes or root
plans/AGENTS/rulings/receipts edits. Root updates main before this start.

FIRST fix reproduced manual staging data loss. Root ran an external Go overlay
through production stageManual: already fully staged code.go, capture --changes,
stageManual(same=true), then its undo (the commit-refusal cleanup). The previously
nonempty staged diff becomes EMPTY. Actual exit1, test failure names lost staging.
Reproducer and result live at ABSOLUTE:
/Users/wido/LocalStorage/agentic-tools-evidence/intent-redesign-20260926/manual-staging-diagnostic/
Fix and add a meaningful real-owner refusal regression: preserve staging the user
had before submission, including a mix of pre-staged and newly staged paths. Do
not reset all captured paths indiscriminately. A failed stage may restore only
its own changes. Do not run reverse-apply/cleanup against a HEAD that moved unless
the actual resulting state was proven to belong to this submission; uncertain
installation needs truthful partial recovery, not a guessed rollback. Never say
cleanup succeeded while ignoring its error. Keep unchanged CommitStaged checks.

Other manual completion required:
- --patch inside the target goal checkout is ordinary supported input. A supplied
  patch is not automatically already applied just because cwd equals destination.
  Clean destination applies plain --index --binary; exact captured/staged replay
  rejoins; conflicts preserve source/index. No arbitrary use-another-checkout rule.
- Changed brief after a read must be tested against actual frozen-brief owner,
  never create or reuse a clean certification for changed input without its check.
- Validate goal selection before writing inputs under a goal-derived path. Public
  invalid/unknown goal input must not escape intended artifact custody.
- Narrow immutable content-addressed patch/brief files are acceptable using the
  existing atomic-file owner; no submission registry/counter or new generic input
  framework. The committed-read owner still controls the actual reviewed brief.
- Current TestIntentManualWorkDelivery proves batch admission/join with fake land
  effects, NOT the complete delivery. Supply actual committed payload on endpoint
  through the public manual -> independent review -> bound decisions -> land flow.
  A model/provider may be fake; commit/push/close/collect/landing cannot be successful
  callbacks where their actual effect is the claim. Reuse authentic owner fixtures.
- Deliver executable public CLI recipe with real child processes and fake provider,
  so root can rerun final binary. Built-binary argument refusals alone are insufficient.

THEN complete the FRESH source audit, read BOTH ABSOLUTE paths in full:
/Users/wido/LocalStorage/GitHub/agentic-tools-intent-workflows-20260926/metasystem/plans/intent-workflows-final-surface-audit.md
/Users/wido/LocalStorage/GitHub/agentic-tools-intent-workflows-20260926/metasystem/artifacts/agents/intent-public-continuation-audit.md
Some reserved review continuation fixes already exist in pass8: preserve them and
only fix remaining routes. Required: generated brief works before first workspace;
show --goal G handles reserved names; ask -> qualified public question wait/exact
retry, keep qualification after bounded wait; public archived-goal reopen; status
work discovery + --all, opaque j1:ID/j2:ID references in status/wait/stop job. No
new root verb/registry, no internal owner commands as normal remedies. Retain actual
requested operation on ambiguity and original raw IDs for legacy owner calls.
No claim that checking a command string proves following it succeeds: drive it.

Integration coordination:
- Read owner FINAL is already imported through tree2678096eed92dd25b7b9156f17830002747ce771.
  Register TestWorktreeDiffRacyCleanIndexGitAdapter too; root's three imported
  capture checks passed0. The four launch files are no longer under active edits.
- One parallel correction in intent_authority_positive_test.go is already imported.
  You may now add truthful serial exemptions for the remaining nine from ABSOLUTE
  artifacts/agents/intent-authority-parallel-audit.md under PRIMARY above, using the
  final renamed carried test when it arrives. Do not weaken ratchet or floors.
- Carried builder STILL owns intent_exception.go, carried test/helper part of
  intent_authority_positive_test.go, intent_carried*_test.go and landing/carried_prepare*.
  Planning builder STILL owns intent_design*/intent_planning*, design_request*,
  dispatch/design_chain.go and dispatch scripts, plus necessary adopted-design-path
  guards in dispatch/read_admission.go, dispatch/build.go, critique/model.go.
  Both isolated returns arrive via root; do not edit their files meanwhile.
- Root will import their exclusive files and write markers:
  artifacts/agents/intent-planning-completion-integrated.json and
  artifacts/agents/intent-carried-delivery-integrated.json. Their necessary SHARED
  intent_delivery*.go / intent_operations_test.go patches will be left as ignored
  artifacts for YOU to inspect/apply to your shared code, not raced by root.
  Read markers before final checks; if absent finish your independent items first.
  Planning already found real dispatcher requires design-critique/code-critique
  brief modes and repository-relative --design paths. Preserve those corrections.
- Once real design public closure passes, hide close discovery but retain legacy
  behavior. Finish public docs/skills consistently. Root owns canonical AGENTS.

Focused failed-before/passed-after checks. Do not repeat the previous 275s broad
pattern for a tiny wording fix; choose changed tests until integrated candidate
is stable. Final selected native/coverage/section gate is root's independent run.
Keep nine workers and inherited allowances, all assertions/coverage floors.
Budget200 NEW tools/40actualminutes; precise complete remaining gaps on checkpoint.
Output artifacts/agents/intent-workflows-build-return-9.md. Root then freezes full
computed patch for independent Sol6 conformance/adversarial review, still pending.
