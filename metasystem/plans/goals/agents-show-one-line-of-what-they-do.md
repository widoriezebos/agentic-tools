# agents-show-one-line-of-what-they-do

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="A person-facing line in every agent pane of the UI; a wrong or stale line misleads the person about what runs, but nothing in the machinery changes; the activity data exists for the Partner; the UX is the new part"
- Tier: 2
- Intent: Every agent a person watches in the web UI (the Project Partner, the reviewer, and any other agent pane) shows, beside its activity pulse, one line of plain text naming its latest activity while it works, and nothing beyond that it is active now (Wido 2026-10-03 16:25 CEST: 'I want more feedback about what an agent is doing if possible. I see the mcp calls pulsating; this is nice; but can I have a single line of text showing the latest activity as a detail of an agent working (and showing nothing more than that it is active at the moment). This is also a UX question; so get a UX design by an UX expert on this too. Implement for all agents'). Today the pulse says only that something happens; the person cannot tell what, so a long turn and a stuck one look the same.
- Origin: human
- Next step: UX design first, by a UX designer: what the one line says (the tool or step in plain words, never raw tool names, paths or JSON), where it sits relative to the pulse on the desktop and at phone width, how it updates (latest wins, no scrolling history), when it clears (the turn ends), and one component used by every agent pane: the Partner (whose store already carries a doing field and an activity stream), the review sitting's agent, and any future one. Inventory first which agents the UI shows and what activity each already reports to the browser; the design names the one source of the line per agent and adds no second activity feed. Tier-2 critique, then build in one slice per pane, the Partner first. Done when: the Partner and the reviewer show the line while their turn runs and nothing when idle, on desktop and phone width; a test covers both states for each; the message audits pass.
- OpenedAt: 2026-10-03T14:18:53Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-03T14:18:53Z BHGSHTMFD3NBVSJSJ5G7K8PG7F-m1e-718ba0eb open actor=human:Wido targets=agents-show-one-line-of-what-they-do
Integrity: sha256=067d42c755907b257c2d53ec597a91c66f4ba4eceaf4d25cd9b0d5487be7a93d
