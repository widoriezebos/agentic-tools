# Astra round 1 (task-mumwd5x7-fuci2s, commit 1f99214b9), verbatim

**Two material findings:** forced completion still fails for an unfinished dependency, and the specified carry-word bypass can suppress inspection errors.

**HF-01 — High; material: yes. Forced completion cannot retain unfinished dependency edges.**

[D-force, line 28](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/plans/designs/at-the-helm-the-person-can-force-done.md:28) explicitly leaves `Blocked` unchanged. However, [validate.go:382](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/validate.go:382) rejects every done goal with an unfinished blocker, and [txn.go:795](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/txn.go:795) rejects that candidate before publication. The existing `TestBlockedDoneRefuses` also pins this invariant.

Concrete failure: G waits for unfinished X. The person forces G done; mutation skips the blocker refusal, but validation rejects the archive. G stays open despite the promised override.

Change D-force to remove the overridden unfinished edges in the same transaction, retaining their names in the conclusion. Keep validation unchanged. Require the blocked-dependency fixture to pass through publication and read back the archive.

**Test 1: DIFFERENT. Test 2: WORKS without it—no; SAFE—yes.**

**HF-02 — Medium; material: yes. The carry-word site returns more than an overridable refusal.**

D-force says each of the four sites proceeds whenever `ForceBy` is set. But [verbs.go:5390](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/verbs.go:5390) returns inspection errors as well as the open-word refusal. Those errors originate, for example, from the repository’s trailer lookup at [verbs.go:5054](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/verbs.go:5054).

Concrete failure: during forced completion, inspecting a carry reservation fails. Bypassing this entire error site allows completion without establishing what was overridden, contrary to the promise that force overrides only the four identified goal-state refusals.

Change D-force to suppress only a positively identified open-carry-word refusal and propagate every other error. Add a fixture injecting a trailer-read error and asserting no publication under force. This needs no broader error framework.

**Test 1: DIFFERENT. Test 2: WORKS without it—yes on the ordinary path; SAFE—no on the inspection-error path.**

Deferred and non-material:

- **HF-03 — Low; material: no.** The “no yield” promise for rejected agent force calls conflicts with proof acquisition occurring before `forceAdmission`. Slice 2’s [helm_admits.go:129](/Users/wido/LocalStorage/GitHub/agentic-tools-helm/metasystem/cmd/metasystem/helm_admits.go:129) records the yield while producing the proof subsequently rejected. The goal remains protected. Defer yield suppression; the design’s wording/test expectation needs alignment. **Test 1: DIFFERENT; Test 2: WORKS/SAFE without it—yes/yes.**
- **HF-04 — Low; material: no.** The enrolled-terminal explanation overlooks the owner’s [ProveTerminal fallback](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/goalsync_mutations.go:643). Where helm admission declines, another agent-free terminal can supply a terminal-valid, non-helm proof. This does not admit an agent or break the specified enrolled-terminal scenario. Defer any stricter enrollment requirement. **Test 1: DIFFERENT if tightened; Test 2: WORKS/SAFE without it—yes/yes.**
- **HF-05 — Low; material: no.** Reconstructing `ForceBy` does not make human completion replayable: [recover.go:158](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/recover.go:158) rejects journaled human intents before reconstruction. Preserve that boundary; narrow the recovery claim rather than adding replay authority. **Test 1: SAME for first-use behavior; Test 2: WORKS/SAFE without it—yes/yes.**

What the source review supports:

- The admission predicate excludes helm and fixture proofs explicitly. Temporary, channel and signed-in-session proofs fail `TerminalValidFor`; the current UI act surface provides no force-done route.
- Reading helm state through `f.root` binds admission to the act’s seat.
- Return removes the signature before catch-up. Enrollment remains available, so a fresh walk from the enrolled terminal can still succeed; removal prevents helm fallback. The proposed forced call supplies that proof and the removed helm record separately.
- The proposal is guarded by active helm state; outside-helm refusal text remains unchanged. The design already requires tests pinning all four recognized refusal texts.
- Read-item acceptance uses the existing closure fields. Branch sweeping remains after publication and retains unlanded work on refusal.

Both material findings are fixture-expressible and require local changes to D-force.

Proposed receipt, unwritten: `Design critique, helm slice 4 round 1: two material findings—unfinished dependency validation and carry inspection errors; source review only.`

**VERDICT: 2 material findings (fail test 2): HF-01, HF-02.**

Commit: `1f99214b939c7b2e54c56142efe6ad962c9b60c0`, `ui-development`. Limitations: read-only review; no tests or terminal walkthrough executed. Slice 2–3 dependencies were inspected in the companion worktree at `a0b4f817f4a2e659380cc0d81507dea013a2e19b`. Runtime outcomes above are inferred from source. No files changed; excluded secrets and artifact directories were not read.

Codex session ID: 01a0ee04-f893-7312-b6b6-235900ed2cfb
Resume in Codex: codex resume 01a0ee04-f893-7312-b6b6-235900ed2cfb
