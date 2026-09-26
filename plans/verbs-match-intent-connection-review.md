# Root adjudication of the workflow connection amendment

Fable5.1 reviewed the full amendment in the same critic session (launch20260925t164057-54edd2fa11). Verdict fix-first, one material finding, smallestSufficient=true. This was the explicit design-skill exception for a demonstrated missing full-release transition after the baseline's two rounds, under Wido's machinery bypass. Root read the entire findings body and checked the actual amend and snapshot owners. The one material correction is folded; the accepted composition remains small and needs mandatory independent code review plus VMI-10 fixtures, not a further general prose round.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| VMI-CONN-01 | accepted | range.go refuses duplicate unit names. CommitRequest.Amend is the existing same-unit correction owner. Root read commit.go:530-650 and TestAmendDropsReadOfReplacedBuildAndReplaysLaterPlan. Refute only the proposed restriction to before any collected read: literal `if info.Kind == Read && info.CommitID == target` skips the old read, and that named test proves it. | Same unit follow-up uses Amend, invalidates old subject read, replays suffix, force-with-lease publication, new committed critic. Preserve actual replay refusals; VMI-CONN-01 fixture. |
| VMI-CONN-02 | noted | Already required join then full closure; dispositions are not autonomous authority. No new mechanism. | Existing VMI-6 full-close fixture covers join and register before closure. |
| VMI-CONN-03 | noted | Existing owner refuses invalid collection. The public continuation must distinguish terminal from real closure. | VMI-10 terminal/unclosed fixture. |
| VMI-CONN-04 | noted | Existing branch.Push owns remote-only adoption and origin-tip record. Direct raw remote fetch is insufficient. | VMI-10 preparation cases name actual branch owner; no new preparation engine. |
| VMI-CONN-05 | noted | Runtime prerequisite unverified, not a new policy or configuration copier. | Generated-worktree runtime fixture; only existing adapter-declared session-isolation manifest if required, never secret conf.local. |
| VMI-CONN-06 | noted | Existing successful proof is the requirement, preliminary read result is feedback. Root source also shows snapshot.Tree is raw diff text, and a follow-up's HEAD differs from plan.Base. | Enumerate accepted outcomes and exact completed-round tip/diff binding in design; VMI-10 stale/proof-wrote fixture. |
| VMI-CONN-07 | noted | Two reads preserve current runner and certification contracts. Changing UnitPlan.Read is a separate cost improvement; no work required for this release. | Deferred at UnitPlan.Read; approved budget still bounds both actual reads. |

No successful read, human decision, approval or landing is inferred from a process exit. Full release still unimplemented until all matrix rows have real proof.
