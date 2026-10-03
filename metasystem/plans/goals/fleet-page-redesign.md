# fleet-page-redesign

- State: approved
- Priority: 1
- Sequence: 25
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="Human-facing UI redesign over existing read paths and existing verbs; acts stay behind the signed-in session; one wrong-fact bug in a health read"
- Tier: 2
- Intent: The Fleet page (http://127.0.0.1:7878/fleet) is elegant, intuitive and powerful (Wido 2026-10-03: 'a mess from a UX perspective. Long page with a lot of text, and it states .. needs you but it is unclear what I can do and where from this'). A UX designer steps back first: who looks, what they must decide, what they can do; then a redesign where every 'needs you' item says in one plain line what is wrong and carries the one button (or one command) that deals with it, the page fits on one screen at a glance with detail behind disclosures, and nothing internal (pids, generations, threshold names, file paths) shows by default. Must also fix a wrong fact seen 09:25: 'This computer's steward has not recorded its health since 2026-09-22' while the steward runs, because the health read looks for metasystem.conf at the repository root, not under metasystem/.
- Origin: main
- Next step: UX step back: use cases and jobs (reuse the accepted fleet-panel-ux Part 1 where it holds), a review of today's page with screenshots (agentic-tools-evidence/fleet-panel-ux/fleet-0925-top.png: ~1,200 words, one stale wrong 'needs you'), then a redesign page with a mock per state (all good, one thing needs you, lane red, seat stuck, phone width); Astra critique tier 2; build; check in a browser with Playwright at 390 and 1440 px.
- OpenedAt: 2026-10-03T06:25:15Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-03T06:25:26Z revision=2 opid=0DS710R47TV1D9T0E451S6CZXN-m1e-718ba0eb authority=proven digest=ae70358e64dde97bdd8c3e538fe37fc545889d295120052b64091b216882e651 episode=2

History:
- 2026-10-03T06:25:15Z XAYSKY425H0C7DTK3BKJ7WJKTZ-m1e-718ba0eb open actor=human:Wido targets=fleet-page-redesign
- 2026-10-03T06:25:26Z 0DS710R47TV1D9T0E451S6CZXN-m1e-718ba0eb approve actor=human:Wido targets=fleet-page-redesign
- 2026-10-03T06:25:36Z B2D5QM1W77XNBZBEYSAAQGASD4-m1e-718ba0eb set-priority actor=human:Wido targets=fleet-page-redesign reason=priority-order subject=fleet-page-redesign from=unranked to=1:25 requested-sequence=append
Integrity: sha256=9d6084bb1a506a22c67963347ef68c51b2975e228120aaa8c55d53c341e903c5
