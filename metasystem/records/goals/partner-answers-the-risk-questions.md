# partner-answers-the-risk-questions

- State: done
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="a wrong suggestion can set a tier, but the person sees and confirms every answer before opening; the suggestion and proposal paths already exist; only the new-goal sheet and the Partner's skill change"
- Tier: 2
- Intent: What: on the new-goal sheet, the person can ask the Project Partner to answer the four risk questions (severity, novelty, exposure, accumulation) and the basis line from the goal's title and intent; 'Use these' sets the four answers and the basis, the tier stays derived from the answers, and the person can change any answer or override the tier as today. The Partner's skill says how to judge the four answers from the level meanings the sheet shows, starting low and raising an answer only for a reason it names in the basis. Why: Wido, 2026-10-01: 'would it be possible to ask the project partner to assess the tier based on the goal description? Right now we have a nice UI component where the human can select criteria and then the tier is determined; but it would be great to be able to have the project partners assistance with this'. Today only the basis line has an Ask the Partner link, the four answers cannot be filled by a suggestion, and the Partner has no guidance for judging them. Not in it: changing an existing goal's risk, which stays at a terminal.
- Origin: main
- Next step: Next: a short build brief (no design round, tier 2), Opus builds it in a worktree, one code read, land through m1e. Done when: on the new-goal sheet, asking the Partner fills the four answers and the basis with its suggestion, the tier follows from them, and the person can still change every answer before opening.
- Concluded: Landed on main 1c4399bcb (g1-s78: 7e65b42ab build, f00f2b650 fix round), landed by m1e from a clean clone under Wido's 'yes, c'. The New goal sheet's risk section has Ask the Partner: the Partner suggests the four answers in the command's form and the basis line; Use this sets all four pills, the tier stays derived, a malformed suggestion is shown not usable and changes nothing; the Partner's skill (both copies) judges from the level meanings, starts low and never names a tier. Code read: Codex Sol, one material finding (S78-01, Undo lost on a padded suggestion) fixed. Accepted as built under the stop rule, nothing further planned: the answers and the basis take two Use presses (S78-02), and the tests are pure functions and static markup (S78-03). Concluded in Wido's word (2026-10-01: 'you can conclude in my word').
- OpenedAt: 2026-10-01T09:23:52Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-01T09:39:51Z revision=2 opid=YESZT0MGQ6VHWC2NM9VG0TJ1WG-ui-31a738e9 authority=proven digest=263873cd74c3bb1ead92c126d3e52652e84bfcb10f447349a2e24dac0d2beb5a episode=2

History:
- 2026-10-01T09:23:52Z TSWEVPGKN45F6G1X44AY94W3MK-ui-31a738e9 open actor=human:Wido targets=partner-answers-the-risk-questions
- 2026-10-01T09:39:51Z YESZT0MGQ6VHWC2NM9VG0TJ1WG-ui-31a738e9 approve actor=human:Wido targets=partner-answers-the-risk-questions
- 2026-10-01T10:22:05Z TVDSXB3F2A2S7X1CP5SFDMW4WJ-ui-31a738e9 done actor=human:Wido targets=partner-answers-the-risk-questions
Integrity: sha256=d19343262ab2556de5d38f8f61c53c67592574ba61dc4cdbd8231e3999f420ba
