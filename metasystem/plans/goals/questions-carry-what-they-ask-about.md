# questions-carry-what-they-ask-about

- State: approved
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="Read-only views of seat files on this computer plus a channel attachment; answering keeps its existing authentication"
- Tier: 2
- Intent: A person can answer every question from where they are, without opening a seat's local files (Wido 2026-10-03, about m1h's design question naming plans/designs/landing-deploys-the-engine.md in m1h's checkout: 'hard to answer over telegram because it references a local file I cannot read from there ... the UI should help me with this'). (1) The UI's Decisions page shows every open question from every seat on this computer, not only this checkout's, each with its goal, choices and recommendation. (2) A question names the documents it is about; the UI opens each one read-only as the asking seat has it (its checkout or goal worktree), rendered, with what changed since main. (3) On the phone, the channel message carries what is needed to decide without the file: a short plain summary of the document's relevant part, and the document itself as an attachment (Telegram sends files) or a readable link; a question never relies on a path the person cannot open.
- Origin: main
- Next step: Inventory how questions are stored per checkout and how /api/decisions and /api/documents read them; short tier-2 design (fleet-wide question read, document references in a question, read-only document view from another checkout, channel attachment); Astra critique; build; check on a phone. Coordinate with channel-questions-stand-alone (same message shape) and fleet-page-redesign (Needs you links to the question).
- OpenedAt: 2026-10-03T07:09:13Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-03T07:09:21Z revision=2 opid=X8AEFTD7FCQ6N6VR4XN9WBT4X8-m1e-718ba0eb authority=proven digest=63e6d9484e1c59dd27e8af47d68b1719c2d2c063fb9eac87aac1c6fae0878d91 episode=2

History:
- 2026-10-03T07:09:13Z VRZNVBQNG2YXRVE11RF8CXEHMZ-m1e-718ba0eb open actor=human:Wido targets=questions-carry-what-they-ask-about
- 2026-10-03T07:09:21Z X8AEFTD7FCQ6N6VR4XN9WBT4X8-m1e-718ba0eb approve actor=human:Wido targets=questions-carry-what-they-ask-about
Integrity: sha256=8306ff31cf04e34c5ff6d9d27181fa9f118e51e4265fe0d71fb3e6a889cfff4f
