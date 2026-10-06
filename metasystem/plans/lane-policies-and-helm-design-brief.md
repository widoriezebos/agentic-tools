# Design brief: lane-policies-and-helm (landing redesign 1b)

Working Mode: Design
Goal: lane-policies-and-helm. Tier 3 (Wido 2026-10-03: lane goals generic, top priority, tier 3). Coordinated by m1e in Wido's word. Built after lane-lands-finished-goals (1a) is on main; this brief is written while 1a is designed so that 1b's design can start the hour 1a lands.

Read first: `plans/designs/machinery-mechanisms.md` (accepted; mechanisms 1, 7 (the boundary as a reader only), 12 are consumed here, 2 and 4 reused from 1a), `plans/machinery-open-findings-plan-2026-10-06.md` (findings 1, 12 and the stop section), the accepted design of lane-lands-finished-goals (1a, at plans/designs/lane-lands-finished-goals.md once it exists; accepted before this design starts), and the code sites cited below (read on main 9eb87cd15; cite the site you build against, and if it moved, say so).

## Intent

The lane's decisions become policies a person can read and set as settings, with the three values `auto`, a cap, and `person`; the helm sets them all to `person` at once and gives them back; the lane can drain; a goal's claim records the areas its design edits, so two goals in one area are sequenced instead of colliding. Wido's rule: every policy has the person value; a person's act is never refused (No HAL 9000); the helm is the person value on all of them, not a separate mode.

## What is true today (read, with sites)

1. Settings: `internal/config/resolve.go:303` `Get` resolves a key through default, `metasystem.conf`, `metasystem.conf.local` and the environment; `settings set` writes the local file for this checkout's seat (`cmd/metasystem/intent_work.go:2303`, per the Astra round of 10-06 on declarations); `settings show KEY` prints `value · source`; `settings check` validates grammars (`internal/config/validate.go:593`). The mechanisms page (mechanism 5, as ruled) makes repository declarations committed-only; policies are per-checkout settings and may stay local.
2. The helm: `cmd/metasystem/intent_helm.go:112` registers `helm take` (human audience), `:172` runIntentHelmTake, `:388` runIntentHelmReturn, `:144` runIntentHelmStatus. Today it "takes this seat out of the machinery's hands until you return it" for one checkout; there is no `--all` and nothing it does to the lane.
3. The lane's pause: `cmd/metasystem/intent_landing.go:212` `landing stop` pauses the lane at once ("no landing agent starts; also ends a running regeneration"), `:192` `landing start` resumes; the pause state lives in `internal/landing/lane/pause.go`. There is no drain (stop admitting, finish the queue, hold).
4. The lane's decisions that today are code or the landing agent's judgment: how many waiting lines a batch takes (`skills/landing-agent/SKILL.md` "The loop": all of them), whether to repeat a red proof (`last_proof.repeat`), what to do on a red (SKILL case 3), whether to proceed while main is red (nothing today; 1a adds the trunk-red hold). The keeper's barren hold (`internal/landing/lane/agent.go:42` barrenLimit, `:144` Run) is the lane's only built-in stop.
5. Claims record no areas: `internal/goal/verbs.go:1390` claimQuotaRefusal and `internal/goal/validate.go:399-490` know machines and arcs, not paths. The accepted design's Units table (`internal/launch/admit.go:457` DeclaredUnits) names units, not paths. The week: five goals edited the same three files for two days; the -04 branch needed nine hand resolutions.
6. The boundary (mechanism 7) does not exist yet; 1b reads nothing from it, but `settings.apply` (when a settings change takes effect) is named here and implemented when the boundary exists (runs-advance goal). Until then a settings change applies at the next verb.

## What the design must deliver (the units; re-cut allowed, intent not)

| Unit | Intent | Lines |
| --- | --- | ---: |
| U1 policy settings | The keys, each with its one-token values, read through `Get` and validated by `settings check`: `landing.batch` (`auto` \| N \| `person`), `landing.proof` (`auto` \| `person`), `landing.on-red` (`auto` \| `person`), `landing.trunk-red` (`auto` \| `person`), `seat.driver` (`auto` \| `person`), `review.stop` (`auto` \| N \| `person`), `goal.raise` (`auto` \| `person`), `question.route` (`auto` \| `person`), `settings.apply` (`boundary` \| `now`). `settings show KEY` prints the effective value, its checkout, who set it, and a helm override when one stands (the `policy` record of mechanism 1: name, checkout, value, set-by, at, previous). A lane key is set in the lane checkout; `settings set` in a seat checkout refuses a `landing.*` key with the lane checkout named. | 120 |
| U2 the lane reads its policies | The landing agent's loop takes `landing.batch` (all waiting, at most N, or a person's pick through a question), `landing.on-red` (1a's classification runs, or the lane holds at the red and asks), `landing.trunk-red` (hold except the fix goal, or ask), `landing.proof` (the ladder, or a person's word per proof); `person` values produce `question` records (mechanism 12) whose text is the one command; `landing status` shows the effective policies. | 120 |
| U3 drain | `landing drain`: the lane admits no new hand-in (a hand-in is refused with the drain named and a next step), finishes the queue it holds, then holds; `landing start` ends the drain; `landing status` says "draining, N waiting". `landing stop` keeps its meaning (pause at once). | 60 |
| U4 the helm sets every policy | `helm take` in a seat checkout sets that seat's policies to `person`, keeping the previous values in the policy record; in the lane checkout it also drains the lane; `helm take --all` does it for every machine on this computer (the form of `machine stop --all`); `helm return` and `helm return --all` restore the previous values and end the drain; `helm status` and `helm status --all` say who holds what and which policies are overridden. An agent is still refused the helm. | 100 |
| U5 areas on the claim | A design page's Units table gains an `areas` column (paths or globs the unit edits), or a page-level `Areas:` line; `goal claim` copies the goal's areas onto the claim; a second claim whose areas overlap a claimed goal's, or a goal waiting in the lane's queue (1a's claim-and-queue view), is sequenced behind it (`auto`: refused with the blocking goal named and `goal claim` as the next step when it lands; `person`: allowed with a warning); areas are released when the goal lands or is dropped. The overlap check is a reader of the claim, never a lock on files. | 100 |

Build order: U1, U3, U4, U2, U5. Each unit: one build, at most two corrections, material must fall, a class repeat stops the unit. Each unit's acceptance has one test through the public verb (`settings set/show/check`, `landing drain/status`, `helm take/return/status`, `goal claim`).

## Estimates

About 70 minutes per unit when the first read is clean; five units, about 6 to 8 hours of machinery time. Box proposed: 2d/30/2500m/2/20.

## Not in this goal

The trunk check, red classification, the stop and question records (1a). The driver (`seat.driver` is declared here, consumed by runs-advance). The budget raise policy's behaviour (`goal.raise` declared here, consumed by goal-budget-follows-its-plan). The boundary (runs-advance). The records ref (finding 3).

## Readers of what this goal changes

Settings: `internal/config/resolve.go`, `internal/config/validate.go`, `internal/config/defaults.go`, `cmd/metasystem/intent_work.go` (settings verbs), `cmd/metasystem/intent_settings*.go` if present; the helm: `cmd/metasystem/intent_helm.go` and its tests; the lane: `skills/landing-agent/SKILL.md`, `cmd/metasystem/landing_agent.go`, `internal/landing/lane/{agent,pause}.go`, `cmd/metasystem/intent_landing.go` (start, stop, status), `internal/landing/plain/queue.go` (the hand-in refusal under drain); claims: `internal/goal/verbs.go`, `internal/goal/validate.go`, `internal/goal/file.go` (the claim record), `internal/launch/admit.go` (the Units table reader), `cmd/metasystem/intent_planning.go` (goal claim); the board (`internal/board/card.go`) for the effective policies; tests pinning today's behaviour: `cmd/metasystem/intent_helm_test.go`, `landing_*_test.go`, `internal/goal/validate_test.go`, `internal/config/*_test.go`.

## Acceptance of the goal

- Every lane decision named above is a setting a person reads with `settings show` and sets with `settings set`; `person` makes the lane ask through a question and wait.
- `helm take` turns every policy of the scope to `person` and `helm return` restores them; `--all` covers the computer.
- `landing drain` admits nothing new, finishes the queue, holds; `landing status` says so.
- Two goals with overlapping areas do not run at once under `auto`.
- The full cmd/metasystem package is green on the integrated tree.

## Questions the design may put to Wido

Only ones whose answer changes what is built. Name the mechanism and the two options.
