# Public capability preservation

Goal: verbs-match-intent. This is the pre-implementation capability inventory for
[Complete tasks through intent](designs/intent-workflows.md), read at
c8ecd4bb62706c02331f12a4de4ae0025b634568. Proposed routes are design obligations,
not claims of implemented behavior. Root will bind each group to actual tests
and runtime observations before completion. Aliases alone do not prove a route.

| Existing command | Meaning that must remain | Proposed public route | Current descriptor (cmd/metasystem/) |
| --- | --- | --- | --- |
| `goals` | list the open goals | `goals (same explicit intention, focused help)` | `intent.go:88` |
| `show` | one goal: state, budget, next step and linked design | `show G; show question Q; show designs/decisions; show record ID` | `intent.go:110` |
| `approve` | approve goals for execution | `approve (same explicit intention, focused help)` | `intent.go:118` |
| `budget` | read a goal's budget, or give it a box | `budget (same explicit intention, focused help)` | `intent.go:137` |
| `pause` | park a goal with a reason | `pause (same explicit intention, focused help)` | `intent.go:157` |
| `resume` | resume a parked goal, or a stopped goal under its standing box | `resume G; resume mission M` | `intent.go:170` |
| `done` | conclude a goal with its conclusion | `done (same explicit intention, focused help)` | `intent.go:189` |
| `start` | let this checkout's work run again, or start an agent session | `start [checkout]; start session/ui/mission M/machine NAME` | `intent_process.go:50` |
| `stop` | stop this checkout, one job, or the current session | `stop [checkout]; stop work/task or session/ui as supported` | `intent_process.go:63` |
| `restart` | stop then start this checkout, or restart the interface | `restart checkout/ui (no unsupported target guessed)` | `intent_process.go:78` |
| `status` | what is running: this checkout, one job, or one unit run | `status [G] [--work NAME]; status --machines; status mission M; status ui` | `intent_process.go:90` |
| `enroll` | enroll this terminal as the person's | `enroll (same explicit intention, focused help)` | `intent_process.go:103` |
| `ask` | ask the person a question through the channel | `ask G --question TEXT --option TEXT; ask --retry Q; ask --withdraw Q` | `intent_process.go:112` |
| `answer` | answer a mission's question | `answer Q [TEXT]; authenticated channel directions preserved` | `intent_process.go:136` |
| `fleet` | every machine's presence, this one first | `status --machines (fleet remains compatible)` | `intent_process.go:149` |
| `doctor` | diagnose this checkout's machinery, changing nothing | `check (doctor remains compatible)` | `intent_process.go:157` |
| `ui` | the browser interface: start, stop or status | `start ui / stop ui / status ui (ui remains compatible)` | `intent_process.go:166` |
| `open` | declare a new goal | `open (same explicit intention, focused help)` | `intent_planning.go:83` |
| `edit` | change a goal's intent, next step, risk or labels in place | `edit (same explicit intention, focused help)` | `intent_planning.go:107` |
| `claim` | claim a goal for this session, or the next ready goal | `claim (same explicit intention, focused help)` | `intent_planning.go:131` |
| `release` | release a claim this session holds | `release (same explicit intention, focused help)` | `intent_planning.go:150` |
| `ready` | mark the held goal built and waiting to land | `land G --queue-only` | `intent_planning.go:163` |
| `decide` | accept the risk of one severe or unproven review finding | `accept-risk G --finding F [--review R] --reason TEXT` | `intent_planning.go:172` |
| `pin` | pin a goal to one machine, or clear its pin | `pin (same explicit intention, focused help)` | `intent_planning.go:190` |
| `prioritize` | place an open goal in priority 1, 2 or 3 | `prioritize (same explicit intention, focused help)` | `intent_planning.go:198` |
| `reopen` | return a done or abandoned goal to the queue with a fresh next step | `reopen (same explicit intention, focused help)` | `intent_planning.go:210` |
| `abandon` | record that a goal will never be worked, and why | `abandon (same explicit intention, focused help)` | `intent_planning.go:219` |
| `block` | record that a goal waits for another | `block (same explicit intention, focused help)` | `intent_planning.go:238` |
| `unblock` | remove one blocker from a goal | `unblock (same explicit intention, focused help)` | `intent_planning.go:247` |
| `unapprove` | withdraw a goal's execution approval | `unapprove (same explicit intention, focused help)` | `intent_planning.go:256` |
| `grant` | record a power of attorney a seat acts under | `grant (same explicit intention, focused help)` | `intent_planning.go:265` |
| `revoke` | close a power of attorney early | `revoke (same explicit intention, focused help)` | `intent_planning.go:281` |
| `split` | split a goal into independently claimable members | `split (same explicit intention, focused help)` | `intent_planning.go:289` |
| `group` | move a goal into an arc | `group (same explicit intention, focused help)` | `intent_planning.go:298` |
| `ungroup` | take a goal out of its arc | `ungroup (same explicit intention, focused help)` | `intent_planning.go:306` |
| `resolve` | discharge a review obligation with its test | `review G --finding F --test NAME (explicit qualified evidence preserved)` | `intent_planning.go:315` |
| `notes` | read, add or close a goal's non-breaking read findings | `notes (same explicit intention, focused help)` | `intent_planning.go:337` |
| `recover` | recover the goal journal and this session's durable waits | `repair goals; repair waits (explicit scope)` | `intent_planning.go:360` |
| `red` | own or close a trunk-red incident | `incidents; incidents claim I --goal G; incidents close I --reason TEXT` | `intent_planning.go:373` |
| `brief` | write a brief scaffold from a goal and its accepted design | `brief (same explicit intention, focused help)` | `intent_work.go:139` |
| `build` | build one unit: build, proof and independent read, then await judgement | `build G [--work NAME] --brief FILE --check COMMAND...` | `intent_work.go:152` |
| `wait` | wait for a job, a goal event or a unit run | `wait G [--work NAME]; wait question Q` | `intent_work.go:187` |
| `test` | run the risk-selected tests for this checkout | `test (same explicit intention, focused help)` | `intent_work.go:208` |
| `settings` | show the selected installation's settings and where each value comes from | `settings [KEY]; settings coordinator; settings compatibility` | `intent_work.go:224` |
| `review` | review a design, a finished job, a built unit or a goal-branch commit | `review G [--work NAME] [--dispositions FILE]; explicit design/job/commit subjects` | `intent_delivery.go:41` |
| `fold` | fold a review's dispositioned findings into a follow-up round | `revise G [--work NAME] --brief FILE [--dispositions FILE]` | `intent_delivery.go:65` |
| `close` | close a finished job chain after its findings are dispositioned | `review G --dispositions FILE completes lawful closure and collection` | `intent_delivery.go:83` |
| `land` | land a goal's read units, or a certified job chain | `land G; land G --queue-only; explicit human exception` | `intent_delivery.go:99` |

Additional retained outcomes currently reached through internal families:

| Meaning | Existing owner | Public home to prove |
| --- | --- | --- |
| Start/resume autonomous work | mission runner | start/resume/status mission |
| Accept or restore a disputed workspace | mission resolve-taint | repair mission with explicit human decision |
| Add a machine | seat launch | start machine |
| Designate or withdraw the coordinator | brain declare/show/withdraw | settings coordinator |
| Inspect project memory | project readers | show record/designs/decisions/design |
| Retry delivery, await answer, withdraw a question | channel poll/wait/close | ask retry/withdraw and wait question |
| Accept a precise proof exception | goal carry and landing | explicit land exception; exact subject/human proof |
| Continue after an abandoned-goal succession was partly recorded | goal carry successor | same abandon/successor intention |
| Recover transaction journals or a session's waits | goal recover/session start | repair goals/waits with accurate scope |
| Accept rewritten remote history or reviewed manual edits | goal repair/reconcile | repair goals with exact reviewed choice |
| Upgrade legacy goal state and engine floor | goal migrate/engine-floor | repair goals/settings compatibility |
| Inspect settings beyond launch settings | config keys/get/validate | settings and check |
| Discover currently running work before stopping it | launch/dispatch/run readers | status work, owner-scoped references |

Mechanical protocols (process identity, locks, custody, low-level ledger writes,
worker execution, canonical hashing, accounting) remain callable by their existing
machine consumers. They are executed by owners, not presented as extra tasks a
person or agent must manually perform. Their preservation is checked through
consumer regression tests, not by displaying a technical catalogue in public help.
