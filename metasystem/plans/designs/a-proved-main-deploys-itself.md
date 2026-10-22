# A proved main deploys itself

- Kind: design
- Id: 01M5PR0VEDMA1NDEP10YS1TSLF
- Status: accepted
- Goals: a-proved-main-deploys-itself
- Critique: accepted at the stop by the steward in Wido's name, 2026-10-22: Astra rounds 1-3 found 9 → 6 → 2 material; round 3 fell below half; its 2 material (both common) and 2 minor (both rare) are folded in revision 4 (see Round 1, 2 and 3 folds)

Tier 3, revision 4 (ACCEPTED at the stop by the steward in Wido's name, 2026-10-22, after Astra round 3: 2 material, both common, folded as D3 and D4; 2 minor, both rare, folded into U2e and D7). Revision 3 (draft) folded round 2 under Wido's 08:45 rule. Design lane (Opus 5.5) for Wido's steward. Revisions 1 to 3 are kept beside this page (`a-proved-main-deploys-itself-revision-1.md`, `-revision-2.md`, `-revision-3.md`). Starting material: `designs/a-proved-main-deploys-itself-material.md` (D7 of main-proves-green-on-a-quiet-host, moved out on Wido's order of 2026-10-21 10:4x; findings QH-04, R-144-m1h, QH-19, QH-21, QH-R5-03, QH-R5-04). Code read at main 95047fae63 in the workspace `artifacts/agents/workspaces/goal-a-flake-is-not-a-red/u4/metasystem`. Paths are relative to `metasystem/`. Rulings relied on: R-144-m1h (decisions 2, 3, 5, 6, 7, 8), R-142-m1e and R-143-m1e (no HAL9000), R-149-m1e (an impact proof admits a push), R-104-m1e (artificial clocks, injected probers).

Wido, 2026-10-22 08:45, verbatim: "we're also not building a nuclear reactor so i expect some edge cases that are rare to fail and we should just have a generic mechanism to handle failure that way so that we can iterate while still not getting blocked. So, keep going and figure out what is a rare edge case and what not." Revision 3 adds that mechanism (D0) and sorts every finding into common (handled in the design) or rare (handled by D0).

Revision 3 subtracts again. The keeper deploys only what this computer's lane published itself. A hand landing ends with an explicit `deploy now --commit <sha>`. The hand-landing witness and its proof reader (revision 2 U3, D7 second bullet) are gone.

## Evidence

- Finding 418, behaviour B4 (`analysis/behaviour-analysis-2026-10-21.md:63`): hand landings do not deploy; the m1e and ui engines pin their checkout's untracked `bin/metasystem`, which no verb refreshes.
- Tonight's manual procedure, `panes/deploy-custodian-u1a.sh`: `deploy now` (line 10), copy-then-rename of the newest `~/.metasystem/engines/<sha>/metasystem` into each checkout's bin (lines 15-18), `system restart` per checkout (line 19), re-enrol the pane (line 22).
- Tonight's hand landings were raw pushes from pane scripts: `panes/land-flake2-u4.log` ends with `ae6074f48e..95047fae63 HEAD -> main` and `main-push=0`, checked by `panes/launchers/after-gate-land-flake2-u4.sh:28`. Ordinary `work land` refuses while a lane is registered (`cmd/metasystem/intent_delivery.go:1915-1922`, per Astra round 2; `intent_land_staged.go:126-133`).
- Astra round 1 (`designs/a-proved-main-deploys-itself-critique-1.log`, final message from line 8735) and round 2 (`-critique-2.log`, final message from line 12253).

## Problem

1. **Only the lane's own push deploys, and it deploys the wrong thing.** `deployAfterPush` (`cmd/metasystem/intent_deploy.go:465-494`) has one caller, `landing push` (`intent_landing_push.go:233-235`). It launches `deploy now` with no commit (`intent_deploy.go:504`); the runner fetches main and deploys its tip, then follows a newer one (`internal/deploy/runner.go:151`, `:202-204`, `:218-223`), as `plans/designs/landing-deploys-the-engine.md:74` says. A trigger for proved A while main is at unproved B builds B (QH-04, QH-19). A hand landing starts nothing.
2. **A busy trigger relies on the holder.** `RunBusy` (`runner.go:130-135`) trusts the holder to re-fetch after release (`runner.go:211-223`), and tells a person so (`intent_deploy.go:341-342`). Once runs are bound, that is false (QH-21, APD-03).
3. **Nothing guards a newer or unknown active deployment.** The runner compares only `current.Commit == tip` (`runner.go:183`). An active artifact the record never saw reconciles to "nothing current" (`runner.go:315-317`); an unreadable `pending.json` is ignored (`runner.go:241`); an unreadable `run.json` is ignored (`runner.go:141`, `ReadActive` error dropped; `record.go:208-219`). Any of these lets a late A replace a newer B: a rollback no person made (R-144-m1h decision 8).
4. **`pushes.jsonl` is not proof that this computer pushed.** `completePush` writes a line after a real push (`internal/landing/plain/push.go:222-226`), but also when HEAD is already on fetched main (`push.go:91-128`) and when a batch's members are found on main and a push line is rebuilt from a green result (`internal/landing/plain/batch.go:402-424`). The last two run no `git push` and, in the rebuild, no `provenGreen` (`push.go:347-384`). Another computer's publication can therefore appear there (Astra round 2, finding 1).
5. **Seat engines stay stale after a deploy.** The pin opens the path recorded at enrolment (`internal/steward/identity.go:223-232`); per finding 418 that is the checkout's `bin/metasystem` (hypothesis: I did not trace which path `system restart` enrols). Q1.

## Invariant

An automatic deploy moves an identified active deployment (or a verified "nothing active") forward, to a commit this computer's lane published after its push proof. When anything needed for that cannot be established, nothing is deployed, the active deployment stays, and one plain line says so and names the person's command. A person's act (pause, rollback, `deploy now`, `deploy resume`, `landing stop`) is never undone or overtaken silently.

## Decisions

**D0. The generic failure rule.** Whenever the automatic deploy cannot establish that a commit is safe to deploy (provenance unknown, a record unreadable, a lock or run state unclear, an outcome line out of order, a proof missing, ancestry unreadable or side by side), it deploys nothing, keeps the active deployment, and appends one line to the automatic outcome record (D6): "not deployed automatically: <reason>; to deploy it: metasystem deploy now --commit <sha>". That line is terminal for the commit: the keeper tries again only when a new commit is published. There is no bespoke handling for rare cases. A person's `deploy now --commit <sha>` is the escape hatch, and U1 makes it safe.

**D1. A bound run deploys one commit and never follows the tip.** `deploy now` gains a hidden `--commit SHA` (beside `--by`, `intent_deploy.go:166`); `deploy.Runner` gains `Commit` (`runner.go:29-50`). A bound pass fetches (for the objects, `runner.go:151`) but targets `Commit` and does not re-fetch or start over (`runner.go:202-204`, `:218-223`). Plain `deploy now` behaves as today, except its busy answer (D4).

**D2. A bound run moves only forward, under the deploy lock.** Before any adapter build or activation it needs an identified current deploy or a verified `version: none` (`runner.go:294-295`). Then: equal → `RunCurrent` ("already active"); current contains `Commit` → `RunCurrent` ("already covered by X"), recorded but not a failure; current a strict ancestor → attempt. Everything else goes to D0 as `RunSkipped` with its reason: an unmapped active artifact (`runner.go:315-317`), an unreadable `pending.json` (`:241`) or `run.json` (`:141`), unreadable ancestry, two commits side by side. Ancestry is a new `Git.Contains` seam (`internal/deploy/git.go:22-46`; `git merge-base --is-ancestor`: exit 1 = no, other failures = unreadable). Pause as today (`runner.go:127-129`, `:176-182`).

**D3. A recorded failure stops automatic retries only.** A recorded failure suppresses automatic retries; an explicit `deploy now --commit C` may retry C, recording the new attempt while retaining the pause, active-deployment and forward-only checks (Astra round 3, R-142-m1e). Mechanism: a failed line for `Commit` in `deploys.jsonl` (`Line.Failed()`, `internal/deploy/record.go:167`) ends a bound run `RunSkipped` only when the run is the lane's (`--by the landing lane`, the keeper's launch, D8); a person's or seat's bound run attempts C again and appends its own line (QH-R5-03; `landing-deploys-the-engine.md:74`: the next push or an explicit deploy tries again). A busy refusal is not a failure and is retried.

**D4. A busy answer never promises what a bound holder will not do, and keeps the request's binding.** Every busy answer (`intent_deploy.go:341-342`) says: "a deploy run is in progress (Op of C, process P), so nothing was done; when deploy status shows it ended, run <retry>". A busy bound request reports that nothing was done and names `metasystem deploy now --commit C` as its retry; only a request originally made without `--commit` recommends plain `deploy now` (Astra round 3, APD-R3-01). The orphaned-holder text (`:345-348`) stays. Unbound runs keep their follow-the-tip loop (APD-03).

**D5. A lane stop holds the automatic command.** A bound run `--by the landing lane` checks, before its first adapter call and before `attempt`, that the lane is registered and not held (the keeper's `TimerHeld`: lane pause or helm, `cmd/metasystem/landing_agent.go:267`), through a `Held` hook beside the deploy pause (`runner.go:521-524`). Held or unreadable: no adapter starts, D0 line "the lane is stopped". A build already running is stopped by `deploy pause`, as today (APD-02).

**D6. The automatic outcome record.** A new file beside the deploy record, `~/.metasystem/deploy/<project>/automatic.jsonl`, owned by `internal/deploy` and appended like `deploys.jsonl` (`record.go:170`): `launched` (commit, at), and the bound command's own outcome (`deployed`, `current`, `skipped` with reason, `failed`, `busy`), written by the command through the deploy owner. The keeper appends `launched` before it starts the child. For each commit, a terminal line (`deployed`, `current`, `skipped`, `failed`) wins over any `launched` or `busy`, whatever their order. `deploy status` prints the line for each commit newer than the current deploy (`internal/deploy/acts.go:358-399`, `intent_deploy.go:199-257`); the keeper's line carries the same sentence (APD-09, APD-R2-04).

**D7. Only this lane's own publications are owed.** `Pushed` (`push.go:35-47`) gains `Published bool`, set only on the path that ran `git push` and succeeded (`push.go:222-226`); the already-on-main path (`push.go:91-128`) and the batch rebuild (`batch.go:421`) leave it false. The keeper owes exactly the `pushes.jsonl` lines with `Published`. The deploy-specific push reader preserves decoding damage from `countedLines` (`internal/landing/plain/queue.go:161-163` drops it today; `:195-196` counts it); damaged history reaches D0 instead of being returned as a complete readable history (Astra round 3, APD-R3-04). Their proof is the push itself: `provenGreen` admitted the tree (`push.go:219`), impact admission included (`push.go:360-381`, R-149-m1e, QH-R5-04). Lines without the flag (another computer's work seen on main, a crash between push and record, every line written before this change) are not deployed automatically. When such a line is the newest, D0 writes its line. This keeps R-144-m1h decision 7 by the simplest evidence the lane already writes.

**D8. The keeper deploys what is owed, on every tick.** In `Observe` (`landing_agent.go:326-354`), after `RecoverMovedMain` (`:344`), `reconcileDeploy(record)` runs. Its error joins the progress error as `recoveryErr` does (`:345`): shown on the keeper's line, never holding the lane (`internal/landing/lane/agent.go:228-237`). It does nothing when there is no deploy contract, deploys are paused, the lane is held (`TimerHeld`, as `trunk_recovery.go:94`) or the deploy lock is held (new `deploy.Running(dir)`, try-lock and release). Otherwise it walks `pushes.jsonl` newest first. It stops at the first line the current deploy equals or contains: lane pushes form one line of history (`push.go:134-157` refuses a non-forward push; a records-only fold builds on fetched main, `push.go:166-189`). It takes the first `Published` line with no terminal outcome (D6). It appends `launched` and starts one detached `deploy now --commit C --by the landing lane` through a new `deployLaunch` field on `landingAgent` (`landing_agent.go:65-77`). A busy child records `busy`, so the next tick launches again. A keeper restart reads the same files. There is no line window (QH-21, APD-05).

**D9. `landing push` stops launching.** `deployAfterPush` keeps its failure and pause lines (`intent_deploy.go:478-488`). It replaces the launch (`:489-493`, `startDeployNow` at `:496-505`) with "the lane deploys <sha> on its next tick; metasystem deploy status shows it". It ships with D8 (APD-04). The keeper is the one automatic entry point (QH-19).

**D10. A hand landing deploys by its last step.** A hand landing (tonight a raw push from a pane script) ends with `metasystem deploy now --commit <sha>` for the commit it pushed. That is a person's act under R-144-m1h decision 7, and D1 to D4 make it safe: it never builds an unproved tip and never replaces a newer or unknown deployment; after a failure it may be run again (D3), and when busy it names its own bound retry (D4). `deploy now` has no lane refusal (`intent_deploy.go:115-131`), so it works with the keeper registered. Nothing detects raw pushes. Narrowing "lane or hand landing" in the goal's intent to this needs Wido's recorded word: Q4.

**D11. Test seams.** The runner gains per-instance seams for the adapter call (`exec.Command` and `identity.ProcessBirth` inline today, `internal/deploy/adapter.go:111-154`) and for the liveness probe (`Active.AdapterAlive`, `record.go:99-106`, used at `runner.go:141`). The defaults are today's code. The keeper gains `deployLaunch` (D8). Behaviour fixtures count calls and drive `Now`; only named wiring fixtures run a real adapter process (R-104-m1e).

## Units

Inventory read at 95047fae63, additions and deletions counted, comments included. Owners: `internal/deploy` (runner.go 569, adapter.go 188, git.go 57, record.go 239, acts.go 401), `cmd/metasystem/intent_deploy.go` (505), `cmd/metasystem/landing_agent.go` (523), `internal/landing/plain` (push.go 422, batch.go). Call sites: `deployAfterPush` one caller (`intent_landing_push.go:234`); `Runner.Run` only `deployRun` (`intent_deploy.go:321`); `RunBusy`/`RunCurrent` read only in `deployReport` (`:339-342`); `startDeployNow` only `deployAfterPush` (`:489`); `completePush` three callers (`push.go:128`, `:226`, `batch.go:421`).

| Unit | Decisions | Production lines (inventory) | Stop | Usable and safe alone |
|---|---|---|---|---|
| U1a bound forward run, a person's run | D0 (runner side), D1, D2, D4, D10 | 180 measured (U1 brief, U1a) | 200 | yes: deploy now --commit is safe on its own |
| U1b the lane's failure rule and stop check | D3, D5 | 54 measured (U1 brief, U1b) | 70 | yes, with U1a |
| U2a the lane's push deploys exactly what it pushed; status shows every outcome | D0 (record side), D6, D7 (without Published), D9 (status) | 226 measured (U2 brief, revision 5) | 240 | yes: a missed trigger shows the D0 line with deploy now --commit |

Goal: two units, 317-362 production lines, each under 250. Revision 2's U3 (60-75) is dropped. Allocation (tests included) is set by each brief.

## Tests

Through the production composition: `deploy now` through `deployCommand` (`intent_deploy.go:115`) with `deployOwners`; the runner with the D11 counting transport and probe, an injected `Git`, an injected `Now`; the keeper through `newLandingAgentKeeper` with injected `agent.now` and the counting `deployLaunch`; lane pushes through `landing push`. No wall clock, no sleeps (R-104-m1e). Concurrency is ordered by the transport's or launcher's hand-off, never by time.

- **U1.** (a) Fetched tip moves from A to unproved B during a bound run of A: one build and one activation, of A; none of B (QH-04). (b) B active, bound A: zero build/activate calls, "already covered by B". (c) Generic rule (D0), one fixture with four rows (unmapped active artifact, unreadable `pending.json`, unreadable `run.json` with the lock free, unreadable ancestry): zero build/activate calls each, the D0 sentence names `deploy now --commit <sha>`. (d) Automatic A (`--by the landing lane`) fails once; further lane runs of A make no attempt, also after a restart. The transient cause is removed; a person's explicit `deploy now --commit A`, while fetched main is B, builds and activates A, appends its new line, and builds nothing of B. A pause still refuses it and a newer active deployment still skips it (R-142-m1e, Astra round 3). (e) Bound A holds the lock (transport blocks build), plain `deploy now` gets busy and says nothing was done and names plain `deploy now` as its retry (APD-03). (e2) A bound manual request for A meets a held lock; fetch now returns B; the answer names `metasystem deploy now --commit A`; running that printed retry builds and activates only A (APD-R3-01). (f) Lane stop between start and first adapter call: zero adapter calls, D0 line (APD-02). (g) Plain `deploy now` follows the tip as today (existing runner tests unchanged).
- **U2.** (a) Real lane push A through `landing push` (full, scoped and impact admission each, R-149-m1e): `landing push` launches nothing; next tick launches one bound run of A. (b) Another computer publishes A. Both no-push paths then run: the already-on-main path (`push.go:91-128`) and the batch rebuild (`batch.go:402-424`). Next tick: zero local `git push` calls, zero launches, one D0 line (finding 1). (c) The competing lock is taken after the keeper's precheck and before the child takes it. The child records `busy`; the next tick, also after a keeper restart, deploys A (QH-21, minor 7). (d) The newest `Published` line sits behind 70 newer unpublished lines: still launched (window mutation, minor 7). (e) Generic rule on the record (D0, APD-R2-04): the injected launcher runs a child that records a terminal skip before the launcher returns. Over ten ticks and a restart: one launch, the same reason on status. The fixture also supplies terminal outcomes followed by later `launched` and `busy` lines, then checks after a restart that the terminal reason stays visible and no new launch occurs (APD-R3-03). (e3) A damaged line in `pushes.jsonl`: zero launches, one D0 line naming the damaged record (APD-R3-04). (f) A fails once; ten ticks and a restart: one launch, one adapter attempt (QH-R5-03, APD-10). (g) Lane stopped, deploys paused or lock held: zero launches. (h) No deploy contract: nothing done.

Mutations, each failing the named test: re-fetch and follow when bound (U1a); drop the forward decision (U1b); let any one D0 row through (U1c, per row); drop the runner's failed-line rule for lane runs (U1d, lane half); apply it to a person's bound run too (U1d, explicit half); restore the old busy promise (U1e); name plain `deploy now` as a bound request's retry (U1e2); drop the `Held` hook (U1f); set `Published` on the no-push paths (U2b); let the newest line win over a terminal line (U2e, later `launched`/`busy` rows); drop the damage count from the push reader (U2e3); add a 64-line window (U2d); keep the launch in `deployAfterPush` (U2a); drop the reconciler's terminal filter (U2f, launch count); drop the `TimerHeld` guard (U2g).

## Not in scope

- Re-pinning seats from the deployed runtime (Q1, its own goal).
- Skipping records-only commits (Q3, follow-up; until then a records-only lane push deploys a same-code engine).
- Detecting raw `git push` hand landings, and the push witness of revision 2 (Q4).
- Another computer deploying a commit it did not push (R-144-m1h decision 7).
- The lane's own engine generation (`plans/designs/the-lane-deploys-the-engine-it-runs.md`); retention of `~/.metasystem/engines/<sha>`; `deploy rollback`, `pause`, `resume`.

## Questions for Wido

**Q1. Seat engines after a deploy.** R-144-m1h decisions 2, 3 and 5 already decide it: a project declared central binds its seats to the central engine (replace, not feed); the choice is made at adoption; the switch happens at session start. Pros: the pane script's copy and restart go by the accepted contract. Cons: it needs the central/source setting and the amendment decision 4 names; the size is unknown. Recommendation (Astra: sound): keep seat migration separate. Central seats bind to the active deployment at session start; source projects keep their own engine. Until then the steward copies by hand.

**Q2. A green main another computer pushed.** Recommendation (Astra: sound): keep decision 7. D7 now enforces it.

**Q3. Records-only commits.** Recommendation (Astra: sound deferral): a follow-up goal. It compares against the reconciled deployment under the lock and keeps classification errors (today `recordsOnlyPaths` returns false on errors, `scope.go:266-287`, and `codeMoved` negates it, `trunk_recovery.go:77`).

**Q4. Hand landings: an explicit last step instead of detection.** The goal's intent says the proved commit deploys by itself "when this computer pushed it (lane or hand landing)". This design deploys a hand landing by its last step, `deploy now --commit <sha>` (D10), and detects no raw pushes. Astra (round 2) judged that a scope reduction that needs your word. Pros: no witness, no reflog reads, no host checkout list; the step is one line in the landing script and U1 makes it safe; `work land` stays out of it (it refuses while the lane is registered). Cons: a hand landing that forgets the step stays undeployed until the next lane push or a person's `deploy now`, and `deploy status` shows main's tip waiting. Recommendation: yes. Record the narrowed intent: lane landings deploy by themselves, hand landings end with `deploy now --commit`.

## Round 1 folds

Astra round 1 (`-critique-1.log`, from line 8735), checked against the code at 95047fae63: R-144-m1h unknown active deployment → D2 (now through D0); APD-02 → D5; APD-03 → D4; APD-04 → D9 with D8 in U2; APD-05 → D6 and D8 (no window, no proof expiry: the lane push is its own proof); APD-06 → the hand-landing path is now D10 and Q4 (no discovery); APD-07 → records-only skip removed (Q3); R-104 → D11; APD-09 → D6; APD-10 → test wording. Q1 rewritten with Astra's sentence.

## Round 2 folds

Astra round 2 (`-critique-2.log`, from line 12253), each checked against the code at 95047fae63 and sorted by Wido's 08:45 rule.

| Finding | Holds? | Class | Why | Fold |
|---|---|---|---|---|
| 1 R-144-m1h provenance: `pushes.jsonl` is not proof of a local push | Yes (`push.go:91-128`, `batch.go:402-424`) | COMMON | The batch rebuild runs whenever main already holds a batch's members: a hand landing of a queued goal does that, as tonight | D7: `Published` set only by the real push; others go to D0. Test U2b |
| 2 APD-R2-02 unreadable `run.json` | Yes (`runner.go:141`, `record.go:208-219`) | RARE | Needs a corrupt or half-written `run.json` while an orphaned adapter lives | D0 row; U1c |
| 3 Hand landing not preserved / `work land` refused with the lane registered | Yes (`intent_delivery.go:1915-1922`, Astra's cite) | COMMON | Hand landings happen every night | D10: explicit `deploy now --commit` as the landing's last step; scope change put to Wido as Q4. Witness machinery dropped |
| 4 APD-R2-04 late `launched` erases a terminal skip | Yes (launch before append) | RARE | The child must reach a terminal outcome (after a fetch) before the launcher returns | D6: `launched` appended before launch, terminal wins; D0 covers what is left. Test U2e |
| 5 APD-R2-05 two histories merged, newer descendant discarded | Yes for revision 2 | (gone) | Needed the hand witness file; revision 3 walks one linear file | Removed by subtraction (no witness) |
| 6 APD-R2-06 exact trunk-proof row never produced | Yes for revision 2 (`trunk_recovery.go:91-92`, `prove.go:572-579`) | (gone) | Only the hand witness needed a trunk proof; a lane push is its own proof | Removed by subtraction |
| 7 APD-R2-07 (minor) fixtures miss their failures | Yes | Test only | The window and busy-child fixtures did not reach the mutation | U2c (lock taken after the precheck), U2d (70 newer lines) |

Q-verdicts: Q1, Q2 and Q3 sound (kept); Q4 not sound as a resolution without Wido's word, now put to him as a scope decision with a recommendation.

## Round 3 folds

Astra round 3 (`-critique-3.log`, final message from line 8182; file time 09:29), checked against the code at 95047fae63. Astra judged the round 2 folds correct (finding 2 at design level; finding 3 pending Q4) and the estimates plausible.

| Finding | Holds? | Class | Why | Fold |
|---|---|---|---|---|
| APD-R3-01 a busy bound request names plain `deploy now` as its retry | Yes (`runner.go:151`, `:202-204`, `:218-223`: the plain retry fetches and follows main) | COMMON | A hand landing's `deploy now --commit` meeting another run is ordinary | D4 with Astra's sentence; U1e2 |
| R-142-m1e an explicit `deploy now --commit C` refused after a recorded failure | Yes (D3 of revision 3 applied to every bound run; `record.go:167` counts build failures) | COMMON | A transient build failure followed by a person's retry is ordinary | D3 with Astra's sentence: lane runs only; U1d rewritten |
| APD-R3-03 (minor) U2e no longer reaches the last-line-wins mutation | Yes (`launched` is now written before the launch) | RARE | Only out-of-order lines show it; D0 and D6 already cover the runtime | U2e adds later `launched`/`busy` rows after a terminal one |
| APD-R3-04 (minor) the all-push reader drops damage | Yes (`queue.go:161-163` discards the count from `countedLines`, `:195-196`) | RARE | Needs a crash mid-append in `pushes.jsonl` | D7 sentence; U2e3; routed to D0 |

Q-verdicts: Q1, Q2 and Q3 sound; Q4 a reasonable recommendation that needs Wido's recorded approval, because the committed intent (`plans/goals/a-proved-main-deploys-itself.md:8`) still promises automatic hand deployment.

## Critique record

- Round 1, 2026-10-22, log finished 08:20 (file time), `designs/a-proved-main-deploys-itself-critique-1.log`: 9 material, 1 minor (decision numbers of revision 2). Folded as D2 (R-144-m1h), D5 (APD-02), D4 (APD-03), D9 with D8 in U2 (APD-04), D6 to D8 (APD-05), witness plus Q4 (APD-06), records-only skip removed (APD-07), D10 (R-104), D6 (APD-09), test wording (APD-10); Q1 rewritten.
- Round 2, 2026-10-22, log finished 08:38 (file time), `designs/a-proved-main-deploys-itself-critique-2.log`: 6 material, 1 minor. Under Wido's 08:45 rule: D0 generic failure rule added; two common folded (finding 1 as D7 `Published`, finding 3 as D10 and Q4), two rare routed to D0 (findings 2 and 4), two removed with the hand witness (findings 5 and 6), minor folded into U2c and U2d. U3 dropped.
- Round 3, 2026-10-22, log finished 09:29 (file time), `designs/a-proved-main-deploys-itself-critique-3.log`: 2 material (both common), 2 minor (both rare). Folded as D3 (R-142-m1e, explicit retry after failure) and D4 (APD-R3-01, the bound retry), U2e (APD-R3-03) and D7 with U2e3 (APD-R3-04). Count 9 → 6 → 2: below half, so the steward accepted revision 4 at the stop in Wido's name; Q1-Q4 stay open for Wido.

## Wido's answers (2026-10-22, about 09:35, via the steward's question)

- Q4: yes, the intent is narrowed: lane landings deploy by themselves; a hand landing ends with `deploy now --commit <sha>`.
- Q1: a separate later goal (the central/source engine choice of R-144-m1h comes first).
- Q2 and Q3: accepted as recommended (another computer's push stays a person's deploy now; skipping records-only commits is a follow-up).


## Revision 5 (steward in Wido's name, 2026-10-22): units as built

The Units table names the units as they were cut and built: U1 became U1a and U1b when its brief measured 232 lines (stop 200); U2 became U2a after Astra's three rounds on the full U2 did not converge (Wido's decision 27, "Ship U2a, defer U2b"). U2b — the keeper re-deploying a lane push missed while another deploy was busy or across a keeper restart, the Published flag, the keeper's line — moves to a follow-up goal; its design text is in briefs/a-proved-main-deploys-itself-U2-brief.md ("Moved to the follow-up goal") and briefs/a-proved-main-deploys-itself-U2-brief-revision-3-full.md.
