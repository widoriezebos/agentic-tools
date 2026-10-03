# Design for agent-works-as-project-partner

- Kind: design
- Id: 01M4182NVB9EZFCQWDTW4SZKXR
- Status: draft
- Goals: agent-works-as-project-partner

Revision 1, first draft. Author: Claude on Fable 5.1. Every cite was read at main `7aeb29edc`; paths are under `metasystem/`. A "checkout" is one working copy of the repository; each has its own machine name on the goal ledger (m1e, m1l). A "seat" is a checkout in which an agent session works goals.

Terms used below. The **lease** records which session holds a checkout. A **lineage** is the name a family of sessions runs under, so a successor can take over its predecessor's lease. A **grant** (general power of attorney) is a ledger entry by which a person lets one session act in his name for a stated time. Taking the **helm** switches the machinery's gates off for one seat while a person steers. A **fence** is an engine refusal that holds for a whole checkout.

# Part 1: understanding

## 1. Intent and use cases

Wido, 2026-10-03, verbatim from the goal record:

> I really like the way you and I are interacting, you are almost like the project partner with the exception that you do not need the UI-bridge (approval/verb trick). I think we currently do not have a mode for an agent to be project partner ... Right now an agent in the machinery would claim open goals (which is not what I would want of the project partner), nor would I have to take the helm for this ... I want a design for this and a goal so I can open an agent and work in project-partner mode with it

> It would also enforce the role of the project partner in the UI; that is basically the project partner mode (with the UI integration on top)

The goal adds: the partner never claims or builds and is not nagged to; it acts in the person's name through the ordinary verbs, without the helm and without terminal tricks; every act is recorded as the person's through the partner, with its plain-English impact shown first (rulings R-142-m1e and R-143-m1e); the machinery keeps running beside it; the browser's Project Partner is the same role on a second surface.

The nine use cases of the evidence session, and what step 1 serves:

| # | Use case | Step 1 |
|---|---|---|
| 1 | Status on request, with what waits on the person | yes |
| 2 | Explaining a decision first: options, recommendation, impact, undo | yes |
| 3 | Backlog work in the person's name (open, edit, approve, prioritize, pin, park, fold, conclude) | yes |
| 4 | Answering seats' questions in the person's word, or bringing his own choices to him | partly: seats over the agent inbox; channel questions in step 2 |
| 5 | Budgets: raising them, noticing a goal near its bound | yes |
| 6 | Supervising seats and the landing lane, routing each wall | yes |
| 7 | Reviews: sending work to a critic, folding findings | no: it asks the owning seat (deferred) |
| 8 | Recording rulings, decisions with impact, notes | partly: ledger acts; files wait for the records route |
| 9 | Emergency repair | partly: restarts in its own checkout |

## 2. Threat model

Nobody outside this machine attacks. As the general power of attorney states (`plans/designs/general-power-of-attorney.md` §5), this is a guardrail and an audit trail for cooperating agents that run as the same operating-system user, not a security boundary. Each protection guards a named invariant.

**(a) Our own agents make mistakes.**

| Mistake | Protection | Invariant |
|---|---|---|
| The partner misreads the person or acts on a stale word | Every act in the person's name needs `--impact TEXT`; without it the act is refused (§8). The authority ends by itself within a week and any seat can revoke it (§6). `status` shows the mode | I1: no act in a person's name without its impact stated first and recorded |
| A verb on the wrong goal | The impact line names the goal and the undo; acts are idempotent (R-129-ui); the one act no verb undoes, `goal done --force`, stays the person's (§6) | I1 |
| A worker session believes it is the partner | Authority comes only from the grant's binding: the caller must be the announced main session that holds the lease of the named checkout under the named lineage (`cmd/metasystem/attorney_admits.go:146-152`). A seat that sets the lineage variable in its own checkout finds no grant for that checkout (`:130`) | I2: a session's belief or environment never confers a person's authority |
| The partner drifts into doing the work (the brain seat's failure of 2026-09-05, `internal/brain/role-packet.md:46-56`) | The engine refuses claim, dispatch and landing in the partner's checkout (§5) | I3: the partner's checkout produces no goal work |

**(b) An agent that is not the partner uses its authority.**

- A seat or a delegate job elsewhere: I2. A delegate job classifies as DELEGATE and is refused (`attorney_admits.go:147`).
- The browser's read-only Partner has no shell and no network (`internal/ui/partner/runtime.go:11-17`); its proposals act only on a signed-in person's press (`internal/ui/httpd/acts.go:97-100`).
- No agent can add or extend the authority: a general grant takes the person's own terminal proof, never the helm or another grant (`internal/goal/general_attorney.go:33-35`).
- A subagent the partner starts inside its own process cannot be told apart from the partner by process ancestry (`internal/lease/classify.go:420-426`); the grant design deferred this too (§10 there). Residual risk, stated plainly: the role text forbids subagents to run acts, and every act prints a notice and is logged with its impact, so a wrong act is visible and undoable.

## 3. Rabbit-hole risks

| Risk | Mitigation |
|---|---|
| A new authority model beside the grant and the helm | None is added: one existing grant entry and one new history key (§6, §8) |
| Rebuilding the browser Partner | Step 1 changes no browser code (§12) |
| Solving per-computer enrollment (parked goal terminal-enrollment-per-computer) | `partner start` uses today's per-checkout enrollment. Reach into other checkouts is a property of the grant, deferred and Wido's to decide (question 4) |
| The full question classification (goal questions-reach-the-person-only-when-his) | This design gives it an addressee field and an answer verb, nothing more (§10) |
| The roster item (goal rosters-are-configuration-items, a draft) | Step 1 reads two existing keys (§4) |
| The launcher growing into goal system-start-launches-your-agent | `partner start` runs the runtime's plain interactive command; no new setting |
| A decisions-page generator | None: the session's boot reads lists that exist (§11) |
| One impact shape for all overrides (goal overrides-state-their-impact) | Step 1 takes free text; that goal owns the shape (§8) |

# Part 2: decisions

## 4. How the mode is declared and entered

**Decision: one command, `metasystem partner start`, run by the person in the checkout that becomes the partner's home.** In order:

1. It proves the person at the enrolled terminal with no agent in the chain (`internal/humanauthority/authority.go:872-887`), the same proof a grant takes. An agent running it is refused.
2. It prints what entering the mode causes (R-143-m1e): what the partner may do in his name, until when, how to end it.
3. It declares the checkout the partner's. This **extends the brain seat's declaration**: the record (`internal/brain/brain.go:57-63`) gains a role, `partner`. As today's declaration does, it refuses while the checkout holds claims or running jobs (`cmd/metasystem/brain.go:150-182`).
4. It adds the authority (§6).
5. It replaces itself with the agent runtime in the same terminal, setting `METASYSTEM_OWNER_LINEAGE=project-partner` as a seat launch sets its own (`internal/launch/seat.go:17-19`). Session start adopts the lineage (`internal/up/up.go:375-381`); the boot hook delivers the role text (§11).

A repeat whose declaration and authority already hold just opens the session. If a partner session already runs there, it says so and opens nothing. A checkout declared the brain is refused, with the command that withdraws that declaration first.

**Runtime and model.** Until the roster's `partner` row is built (roster draft, unit 12), step 1 reads `ui.partner.runtime` and `ui.partner.model` (`internal/config/ui.go:26-30,227`), the keys the browser Partner reads and that the row will replace. One row for both surfaces is part of "one mode". An unset runtime takes the automatic choice of goal partner-runtime-defaults-to-an-available-one. `--runtime` and `--model` override for one start.

**A lineage of its own: yes**, `project-partner`, beside `steward-seat` (`internal/launch/settings.go:47`) and `landing-agent` (`internal/launch/landing.go:27`). The grant binds to a lineage, so a successor partner session inherits it and a plain agent opened in the same checkout does not.

**Its own checkout; it cannot share one with a seat.** The declaration fences the whole checkout (§5). One main session holds a checkout's lease and a second "only advises" (`internal/hooks/runtime_hook_stop.go:705-706`). And the steward counts every announced session as a live seat and starts no seat beside it (`internal/steward/census.go:72-74`, `seat_start.go:375-377`).

**Ending.** `metasystem partner end` revokes the authority and withdraws the declaration (the person's act). `grant revoke` alone leaves the role and removes the authority: the partner then explains and proposes only.

**The brain seat: extended, with one part replaced.** Reused: the declaration, the fence, the boot channel, the exemption from claim nagging, and one coordinator per host (`internal/brain/brain.go:297-304`), which is then either a brain or a partner. Replaced for the partner role: the brain's rule that it never approves (`role-packet.md:25-27`) and the checks that a coordinator's checkout never carries a person's word (`cmd/metasystem/goalsync_mutations.go:734-747`, `:2793-2795`; `internal/ui/act/act.go:139-146`). They keep holding for the brain role.

## 5. What a partner never does

**Decision: the partner never claims, dispatches a builder or lands, and the engine refuses each.** The fence is the brain's (`internal/brain/brain.go:445-468`), already called at claim (`cmd/metasystem/goalsync_mutations.go:1807`), dispatch (`cmd/metasystem/delegate.go:133`) and landing (`cmd/metasystem/landing_path.go:535`). Its refusals become role-aware ("this is the project partner's checkout; the partner never claims; a seat does"). "Never builds" follows: with landing fenced and an agent's commit on main refused by the pre-commit guard (`internal/landing/landpath/precommit.go:126-134`), nothing it edits in code reaches main. When work needs a seat and none runs, it says so and routes it; it does not stand in.

**Emergency repair, the part decided here.** Restarting the machinery or the browser interface are person-gated verbs the authority admits (§7, wall 2). They are ordinary acts in the person's name, with their impact. Hand-landing a fix stays fenced; whether a recorded word may lift that is question 3.

What stays the person's own: the declaration itself (`partner start`, `partner end`).

## 6. Its authority

**Decision: reuse the general power of attorney unchanged in its record.** `partner start` records one "everything" entry (`internal/goal/root.go:48-73`) for this machine, this checkout and the lineage `project-partner`. Today `grant add` reads the lineage from whichever session holds the lease and refuses with none (`cmd/metasystem/intent_grant_everything.go:116-130`); `partner start` knows the lineage beforehand and passes it. Why not something new: the entry already carries the binding (§2, I2), the one-week ceiling, the visibility on every seat, the re-check at each ledger write (`internal/goal/general_attorney.go:158-186`) and the local log.

- **Scope:** every act a person may make, bar the limits below.
- **Duration:** at most one week (`root.go:73`); `partner start` renews it (question 1).
- **Revocation:** `grant revoke` from any seat, `partner end`, or expiry.

**Acts that stay the person's own, refused by the engine even in the mode** (the grant's existing limits, kept): adding or extending a grant (`general_attorney.go:33-35`); enrolling a terminal, taking the helm and authority-defining settings (grant design §5); starting the browser interface as a person (`internal/ui/act/act.go:127-131`); `goal done --force` (`cmd/metasystem/helm_force.go:14-22`; question 2).

**Decisions the partner brings to the person** (instructed in the role text; no engine can know what he said): a choice no recorded word of his covers, such as a contract, a scope change, a new ruling, or accepting a severe risk he has not spoken on. It brings the material, a recommended answer and the impact, and acts when he answers. Everything his spoken word in the session or a standing record covers, it does, naming that word in the impact line. The rule that sorts the two is goal questions-reach-the-person-only-when-his.

## 7. How the gates admit the partner

The mechanism exists: when the terminal walk refuses because an agent is in the chain, the proof falls back to the helm and then to a live grant (`authority.go:937-972`), and the act is the person's. Wall by wall:

| # | Wall | What changes at the gate |
|---|---|---|
| 1 | An agent may not commit on main; `work land` refuses non-goal work beside a lane (`precommit.go:126-134`; `cmd/metasystem/intent_delivery.go:1713-1721`) | Stays in step 1. Removed by the records route (unit 9), which needs goal records-land-through-the-lane. Until then the partner keeps record edits on a branch in its checkout and lists them in its handoff; a push stays the person's |
| 2 | `system restart`, `ui restart` refuse an agent's shell | No change in the home checkout: the person-class check already admits a grantee (`internal/lease/person.go:16-36`, `cmd/metasystem/process_verbs.go:452`). Other checkouts: unit 8 |
| 3 | `goal done --force` needs the helm | Stays (question 2). The partner states the impact and prints the commands |
| 4 | `goal accept-risk` refused in the other checkout | The verb reads the review from the checkout's own records (`cmd/metasystem/intent_planning.go:1393-1404`) and a grant binds one checkout (`attorney_admits.go:130`). Unit 8 |
| 5 | `goal edit --tier` wants a machine reference | With a proven person, at the terminal or through the partner, the evidence may be his words; today the edge demands a job, finding or refusal reference (`goalsync_mutations.go:3404-3408`). Unit 7 |
| 6 | `goal budget --under` refused | Already removed: `--under` naming a general grant is dropped and the act takes the person's route (`cmd/metasystem/intent_goals.go:485,528-543`) |
| 7 | A channel question is answered only in its thread | §10, unit 6 |
| 8 | A budget act from another checkout rebinds the holder's claim | The claim epoch is the counter by which the engine knows which session holds a claim. The edge takes it from the caller's own lease (`goalsync_mutations.go:699-703`) and the engine writes it (`internal/goal/verbs.go:299-305,1822-1829`). Change: only the claim's own holder replaces the epoch; any other actor keeps the recorded one (`:315-317`). Unit 4 |
| 9 | The turn-end rule says claim | §9 |
| 10 | A seat's design draft has no route to main | Not the partner's gate (machinery item 59, in work). The partner's own drafts ride unit 9 |

## 8. Recording and showing its acts

**Today** an act a grant admitted is recorded as `actor=human:<name>` with nothing naming the grant: the proof is shaped like the helm's (`authority.go:916-933`), and a history field for it was left out (grant design §10). Only the local log names the grant (`attorney_admits.go:171`).

**Decision, subject to question 7: one optional history key, `through=<grant id>`,** valid only beside a `human:` actor (`internal/goal/file.go:447-472`, validation `:2112-2133`). It is stamped in one place: the wrapper that already re-checks the grant on every ledger write (`general_attorney.go:158-186`) marks the lines of its own operation. `goal show --history` and the browser read it as "Wido, through the partner" when the grant's lineage is `project-partner`. The history grammar is closed, so every seat's engine is rebuilt before the first partner act; the help says so, as the grant's does.

**Decision: the impact statement is required and kept.** Every verb accepts `--impact TEXT` beside `--json` and `--verbose` (`cmd/metasystem/intent.go:99-103`). When a partner grant answers the person check, the admitter (`attorney_admits.go:163-183`) refuses the act without it, prints it as the act's first line and writes it into the act's log line. The role text adds: say the impact in the conversation first, and for the person's own choices wait for his answer.

**From goal overrides-state-their-impact** this design needs two things: the history field that holds an impact statement, and that goal's generated statement per override verb. When it lands, `--impact` fills that field and the generated statement prints beside it. Until then the statement lives in the act's output and the grant log. This design defines no ledger shape for it.

`status` in the home checkout leads with the mode, on the line the grant already has (`cmd/metasystem/intent_helm.go:517-551`).

## 9. Stewards, the turn-end rule and the lane

- **Turn end.** The Stop gate skips the idle-with-backlog refusal for a declared coordinator checkout (`internal/goal/turnverdict.go:471,575-577`); the partner role inherits it. The session still runs and reads `metasystem session status`; the report says no claim is asked. The claim sentence of `AGENTS.md:35` does not apply, and the role text says so.
- **Stewards.** A steward that finds claimable backlog and no live work escalates on every tick (`internal/steward/verdict.go:111-114`), reports a live idle session (`:125-127`), and may start a seat (`seat_start.go:319-326`). Decision: one guard where the tick decides (`internal/steward/tick.go:610-624`): a declared coordinator checkout takes no work, so no seat starts there and no idle alarm is raised. Nothing revives a partner session; the person opens it again with `partner start`.
- **The lane.** Unit 9 (§7, wall 1).

## 10. Answering channel questions

Today `question answer` on a channel question refuses (`cmd/metasystem/intent_process.go:2206-2215`); the engine's answer needs the signed-in channel message and writes a fixed `human:wido` (`internal/goal/verbs.go:157-158,204`).

**Decision (unit 6):** when the caller is the person through a partner grant, `question answer Q TEXT --impact …` on an ordinary channel question:

1. writes the answer on the question record (`internal/channel/question.go:32-43`), naming the grant's person and the grant;
2. writes the goal's `answer` history line with that person as actor and `through=`;
3. posts in the thread "answered for you by your partner: TEXT", with the impact, so his phone shows what was said in his name.

Questions of an authority kind (stop, budget above norm, carry; `intent_process.go:337`) are refused with the verb to run instead, because their answers carry a ledger effect tied to a channel proof (`internal/channel/poll.go:214-231`).

**For goal questions-reach-the-person-only-when-his (unit 10):** a question gains an addressee. One addressed to the partner is not posted to the person's channel; the partner answers it as above, or escalates it to him with the material and a recommended answer. With no live partner session it is posted as today: the partner is never a bottleneck.

## 11. Continuity between sessions

A new partner session receives at start, through the brain's boot channel (`cmd/metasystem/brain_boot.go:92-100`), within its size bound:

1. the partner role text, compiled into the engine as the brain's is (`internal/brain/brain.go:41-47`);
2. the previous session's handoff note (`metasystem session handoff --note`, an existing verb);
3. the tail of the grant log: what was done in his name since;
4. the open questions and the goals that wait on his word.

Rulings stay in `memory/rulings.md`, read when deciding; a new ruling is his words verbatim. The decisions page is a file under `plans/`. Both reach main by unit 9.

## 12. The browser's Project Partner

**"One mode" means concretely:** one role text (the terminal packet and `skills/project-partner/SKILL.md` state the same role), one roster row, one wording in history ("through the partner"), and one continuity (both read §11's sources). The surfaces differ only in how the person's authority arrives: in the terminal by the standing grant; in the browser by his press on each proposal (`internal/ui/partner/host.go:234-241`), which keeps the interface's rule that an agent request never becomes a human act (`metasystem.conf:40-47`).

- **Step 1:** no browser change.
- **Deferred (unit 11):** history and the workspace show "through the partner"; the mode's line with its end time and a revoke button.
- Whether the panel may ever act under the grant is question 5.

# Then

## 13. Step 1

One slice of five units. What Wido does and sees:

1. In the checkout he chooses as the partner's home, at his own terminal: `metasystem partner start --for 1w`. He reads: "From now until Sat 17:20 (2026-10-10) the agent opened here acts in your name on the backlog and this checkout's machinery. Every act states its impact first and is recorded as yours through the partner. It never claims, builds or lands. End it: metasystem partner end." His agent opens in the same terminal with the role text.
2. He talks. The partner explains, plans and runs backlog and budget verbs, each printing its impact line and the grant's notice. No helm, no pane.
3. `goal show G --history` reads "approve — Wido, through the partner (grant …)".
4. At turn end the partner is not told to claim; the steward starts nothing there and raises no idle alarm.
5. A seat whose budget the partner raised keeps working without releasing its claim.

This is the step 1 the brief proposed. Left for later, named in §14: walls 1, 3, 4, 5, 7 and 10, and the browser.

## 14. Deferred

| Item | Builds on |
|---|---|
| Channel answers (unit 6) | `through=` and `--impact` |
| Tier raise on a person's word (unit 7, machinery item 61) | the person proof the grant already gives |
| Reach into other checkouts of this computer (unit 8, question 4) | the grant entry's checkout as the partner's home |
| Records route (unit 9) | the fence's role; goal records-land-through-the-lane |
| Partner as first addressee (unit 10) | unit 6's answer path |
| Browser wording and revoke (unit 11) | `through=`; the status line |
| Roster row (unit 12) | the two keys step 1 reads |
| Sending a review to a critic (use case 7) | a fence exception for review jobs funded by a named goal |
| Generated impact statements | `--impact`; goal overrides-state-their-impact |
| Telling the partner's subagents apart | the grant design's own deferred item |

## 15. Units

Tests use the existing owner seams and stubbed Git. Lines are changed lines including tests.

| # | Builds | Files | Test | Lines |
|---|---|---|---|---|
| 1 | Role on the declaration; role-aware fence texts; partner role text; the coordinator's "never a person's word" checks apply to the brain role only | `internal/brain/brain.go`, new `internal/brain/partner-packet.md`, `cmd/metasystem/goalsync_mutations.go`, `internal/ui/act/act.go` | `TestPartnerRoleFencesClaimDispatchLandAndCarriesThePersonsWord` | 260 |
| 2 | `partner start`, `partner end`; proof, declaration, grant with the fixed lineage, launch, status line | new `cmd/metasystem/intent_partner.go`, new `internal/launch/partner.go`, `cmd/metasystem/intent_helm.go`, `brain_boot.go` | `TestPartnerStartDeclaresGrantsAndLaunchesUnderThePartnerLineage`; `TestPartnerStartRefusesAnAgentCaller` | 420 |
| 3 | `--impact` on every verb, required under a partner grant and logged; `through=` parsed, rendered, stamped and shown | `cmd/metasystem/intent.go`, `attorney_admits.go`, `internal/humanauthority/attorney_log.go`, `internal/goal/file.go`, `general_attorney.go` | `TestPartnerActNeedsImpactAndIsRecordedThroughTheGrant`; `TestHistoryLineThroughRoundTrips` | 380 |
| 4 | A budget act by another checkout keeps the holder's claim epoch (machinery item 49; only its test if that goal lands first) | `internal/goal/verbs.go`, `cmd/metasystem/goalsync_mutations.go` | `TestBudgetActFromAnotherCheckoutKeepsTheHoldersClaimEpoch` | 70 |
| 5 | The steward leaves a declared checkout alone | `internal/steward/tick.go`, `seat_start.go` | `TestStewardLeavesACoordinatorCheckoutAlone` | 80 |
| 6 | `question answer` through the partner | `cmd/metasystem/intent_process.go`, `internal/channel/`, `internal/goal/verbs.go` | `TestPartnerAnswerWritesARealAnswerRecord` | 260 |
| 7 | Tier raise on a person's word | `cmd/metasystem/goalsync_mutations.go`, `internal/dispatch/` | `TestPersonRaisesAnApprovedGoalsTierOnTheirWord` | 60 |
| 8 | Reach into other checkouts | `cmd/metasystem/attorney_admits.go`, `internal/lease/person.go` | `TestPartnerGrantReachesAnotherCheckoutOnlyForThePartnerSession` | 220 |
| 9 | Records hand-in passes the fence | `internal/brain/brain.go`, the records goal's hand-in | `TestPartnerRecordsHandInPassesTheLandFence` | 120 |
| 10 | Question addressee | `internal/channel/`, `cmd/metasystem/intent_process.go` | `TestQuestionForThePartnerIsNotPostedUntilEscalated` | 250 |
| 11 | Browser wording and revoke | `internal/ui/`, the frontend | `TestWorkspaceShowsActsThroughThePartner` | 300 |
| 12 | Roster row | `internal/config/ui.go` | the roster's `TestPartnerReadsItsRowAndShowsRunningUntilRestart` | 30 |

Step 1 is units 1 to 5, about 1210 lines.

## Moved effects

| Effect | From | To | Code |
|---|---|---|---|
| The claim epoch written when a budget act rebinds a claimed goal | the caller's own checkout lease, whoever calls | the claim's recorded epoch, unless the caller holds the claim | `metasystem/cmd/metasystem/goalsync_mutations.go:699-703`; `metasystem/internal/goal/verbs.go:299-317` |
| The lineage a general grant binds | read from the lease holder at `grant add` | for a partner grant, named by `partner start` | `metasystem/cmd/metasystem/intent_grant_everything.go:116-130` |
| First delivery of a question addressed to the partner (unit 10) | the person's channel | the partner; the channel on escalation or with no live partner | `metasystem/cmd/metasystem/intent_process.go:331-358` |

## 17. Open questions for Wido

1. **Does `partner start` grant the authority itself, and for how long?** (a) Yes; `--for` or `--until` is required when no grant is live, and a live one is reused. (b) Yes, 24 hours when omitted. (c) Yes, one week when omitted. (d) No; `grant add` stays a second command. Recommended: (a). Impact: one command opens the mode; you name the duration at most once a week and no duration is ever silent.
2. **`goal done --force`.** (a) It stays yours at the helm; the partner states the impact and prints the commands. (b) The partner may run it on your word, with the impact. Recommended: (a). Impact: wall 3 stays; three typed commands on rare occasions. It is the one act that skips the proof of done, and two accepted designs keep it in your hands.
3. **Emergency hand-landing when the lane is stuck.** (a) Never; the partner routes the fix, or you land. (b) On your recorded word for one commit, logged with its impact. Recommended: (a) until the records route exists. Impact: the fence stays whole; in a stuck night you are still needed for the push. Hand pushes caused five reds on main on 2026-10-03.
4. **Reach into other checkouts of this computer** (accept-risk where the review lives, restarting another seat). (a) The partner grant is honoured in every checkout here that shares the ledger, for the one partner session only. (b) No; the partner asks that seat's agent, or you run it. Recommended: (a), as unit 8. Impact: walls 2 and 4 go; one grant then reaches every seat's machinery on this computer.
5. **The browser panel.** (a) It stays propose-and-press. (b) It becomes a window on the live partner session, which acts under the grant when you ask in the panel. Recommended: (a) now; decide (b) after goal ui-connects-to-a-running-agent lands. Impact of (b): it changes the interface's rule that an agent request never becomes a human act.
6. **Acts refused to the partner beyond today's limits.** (a) None; it approves, accepts a risk and raises a budget on your word. (b) Accept-risk and budgets above a tier's norm stay yours by hand. Recommended: (a). Impact: a misheard word shows in history and the log and is undone by a verb; with (b) those acts return to your terminal or phone.
7. **A new key in the ledger's history lines (`through=`).** It changes the ledger's grammar, which older engines refuse. (a) Add it; every seat's engine is rebuilt before the first partner act, as the grant required. (b) No ledger change; "through the partner" shows only in the local log of the partner's checkout. Recommended: (a). Impact: one fleet rebuild; every seat and the browser can then show which acts the partner made in your name.

## 18. What the budget did not allow me to check

- Which checkout walls 2 and 4 were met in; that the grant already admits restarts in its own checkout is inferred from the code, not run.
- How a steward treats a brain-declared checkout today, and whether a steward is armed in m1e.
- How a handoff note reaches a successor session.
- Whether the ledger marks an act the browser Partner proposed.
- The ledger commit's own record of the executing lineage (`internal/goal/txn.go:704`), which might replace `through=`.
- The helm designs and the two Partner-proposal designs (g1-s58, g1-s60) were not read; the helm and proposal facts come from code.
- The person-verb lines the brief cites in `cmd/metasystem/intent.go:184,203,236`, and the interactive command line of each runtime.
- Line estimates are judgement; nothing was built or run.
