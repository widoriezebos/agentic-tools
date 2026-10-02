# rosters-are-configuration-items

- State: approved
- Risk: severity=2 novelty=2 exposure=3 accumulation=1 basis="configuration change touching every dispatch and launch; new interface surface; every seat"
- Tier: 2
- Intent: What: A roster becomes a configuration item of its own: a named set that says which runtime and model does each kind of work (design, design critique, build, code critique, and so on). There can be many rosters, each with a type that says where it applies: one per risk tier, one for the landing lane, one for the Project Partner. People see and change them in the Settings page of the interface and with a few plain verbs, instead of editing many separate lines in the configuration file. Why: Wido, 2026-10-01: "I want a goal and a design for rosters as a configuration item, but separate (not a bunch of line level setting in the config). Reason is I want many rosters, one per risk tier, one for the 'lane', one for the project partner. So this needs a roster-type (identifier) and a UI and verb(s) for maintaining them easily. UI in the settings of the metasystem." Today the roster is spread over many role and launch keys in the configuration files, one setting per line, with no way to say "this set of models for high-risk work, that set for the lane". Pros: one place to see and change who does what, a different roster per kind of work, and easy edits from the interface. Cons: the engine has to read rosters instead of today's keys, and existing seats' settings need a clear path over.
- Origin: human
- Next step: Next: Fable writes the design (the roster item and its types, where rosters live, how the engine picks the roster for a piece of work, the verbs, the Settings page, and the path from today's keys), one Astra critique, then Wido decides the build. Done when: rosters of each type can be listed, created and changed from the Settings page and from the verbs, and work of each kind runs on the roster its type selects. Added 2026-10-02 (Wido: 'the running agent; is that defined in the roster which agent and which model? That should be the case'): today runtime/model/effort are scattered over two overlapping key families (launch.<kind>.* and role.<role>.*) and copied per checkout (five seat checkouts on this host). The roster must: cover EVERY agent kind incl. the seat agent and the landing lane agent; be ONE key family; be shared by every checkout on the host (one place to change a model); refuse an unknown key instead of silently falling back; have a 'roster show' verb plus the Settings view; include effort (Wido 2026-10-01).
- OpenedAt: 2026-10-01T08:27:46Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-02T21:13:11Z revision=3 opid=TT96HT9FAHK2KTV99T0ZY77KS6-m1e-9c612d71 authority=proven digest=0971b629f8bf0145a53609941a9ddaf48895bae9215c10b46461dcbb37a7d9b9 episode=3

History:
- 2026-10-01T08:27:46Z NR62KBQS8JYD6Z4EZ90EX155JH-m1e-528c72bf open actor=human:Wido targets=rosters-are-configuration-items
- 2026-10-02T21:13:05Z VRZCZ2P0EATSZMBGB88MN3QFVF-m1e-9c612d71 edit actor=human:Wido targets=rosters-are-configuration-items
- 2026-10-02T21:13:11Z TT96HT9FAHK2KTV99T0ZY77KS6-m1e-9c612d71 approve actor=human:Wido targets=rosters-are-configuration-items
Integrity: sha256=f3f45e984822d91e3ec2ab7faeb8be175d06ed27199b2ee51f9535a572479e13
