# Spend-cap retirement design critique — round 4, closing (Sol)

Chain: revision 4 (Option B promoted) -> critic
design-critic-13d75a84ef6d81aa60a34371 (codex gpt-5.6-sol, xhigh,
fresh context), 2026-09-01. One material finding (the planned source
comment violates the comment-provenance law), one non-material (the
orchestrator's reject-condition reading ruled sound). Revision 5
folds the comment language; zero-material-finding closure expected on
the re-critique.

## SCR-R4-COMMENT-PROVENANCE-001 — medium, material=True

CLAIM: SCR-R4-COMMENT-PROVENANCE-001, the planned ClaudeBudget source comment would violate the repository's binding documentation contract. The specification explicitly tells the implementer to cite this design and ruling R-43-m0b in the source comment, even though source comments must describe only the standing runtime contract and must keep provenance in commits or decision records. An implementer following the design would therefore build a prohibited comment, so the design needs a further fold before implementation.

EVIDENCE: metasystem/plans/spend-cap-retirement-design.md lines 292–296 require the ClaudeBudget doc comment to cite the design and R-43-m0b. metasystem/AGENTS.md line 16 says provenance belongs in commit messages and decision records, never code. metasystem/docs/design/design-principles.md line 91 expressly prohibits durable comments from referencing plans or other project state. The build specification names only this one source comment, making the defect local and bounded.

## SCR-R4-REJECT-SCOPE-002 — low, material=False

CLAIM: SCR-R4-REJECT-SCOPE-002, the flagged reject-condition reading is sound and non-material. Its literal wording omits BuildClaudeCommand, but the numbered specification expressly requires that function's conditional append within the same adapter policy file, so an implementer would still make the same two-function change. This point does not require rewording to determine what gets built.

EVIDENCE: metasystem/plans/spend-cap-retirement-design.md lines 297–303 explicitly require BuildClaudeCommand to conditionally append the native dollar-budget pair, while lines 420–421 use the narrower shorthand. Under the binding materiality criterion, an unambiguous implementation controls over contradictory summary prose.
