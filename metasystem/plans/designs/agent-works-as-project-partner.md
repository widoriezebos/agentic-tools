# Design for agent-works-as-project-partner

- Kind: design
- Id: 01M4182NVB9EZFCQWDTW4SZKXR
- Status: accepted
- Goals: agent-works-as-project-partner

Accepted 2026-10-03 21:16 CEST by m1e for Wido under his power of attorney (grant EJSNF4E0Q4GRGW0R3YHWM7RX5X-m1e-718ba0eb), after Astra rounds 3/1/0; Wido may overturn.

Revision 3 restructures revision 2 under Wido's ruling of 2026-10-03 19:25 (step 1 at most 1,500 words, everything else a deferred list); no step 1 decision changed, no mechanism was added, and the three folds of Astra's round-1 critique stand (critic job design-critic-acd7603454fa1738a4d730e7). Round 2 found one material omission, folded by seat m1l: the unset-runtime rule of revision 1 is restored in `partner start` step 5, with its test in unit 2. Round 3 found nothing; the critique closed at round 3 (3, 1, then 0 material findings). The four open questions were answered for Wido on 2026-10-03 (below). Author: Claude on Fable 5.1. Code paths are under `metasystem/`, re-read at main `63939e24e`.

## Why, and the threat model

Wido, 2026-10-03, verbatim from the goal record:

> I really like the way you and I are interacting, you are almost like the project partner with the exception that you do not need the UI-bridge (approval/verb trick). I think we currently do not have a mode for an agent to be project partner ... Right now an agent in the machinery would claim open goals (which is not what I would want of the project partner), nor would I have to take the helm for this ... I want a design for this and a goal so I can open an agent and work in project-partner mode with it

> It would also enforce the role of the project partner in the UI; that is basically the project partner mode (with the UI integration on top)

| Use case | Step 1 |
|---|---|
| 1 status and what waits on him; 2 explaining a decision first; 3 backlog acts in his name; 5 budgets; 6 supervising seats and the lane | yes |
| 4 answering seats' questions | partly: not channel questions |
| 8 recording rulings and decisions | partly: ledger acts, not files |
| 9 emergency repair | partly: restarts in its checkout |
| 7 sending reviews to a critic | no |

**Terms.** A checkout is one working copy of the repository; a seat is one where an agent works goals. The lease records which session holds a checkout; a lineage is the name a family of sessions runs under. A grant (general power of attorney) is a ledger entry letting one session act in a person's name for a stated time. The helm switches the gates off for one seat while a person steers. A declaration marks a checkout as a coordinator's (today the brain seat's); its fence refuses goal work there. The direct proof is the person at the enrolled terminal with no agent in the chain, never the helm or a grant.

**Threat model.** Nobody outside this machine attacks: this is a guardrail and an audit trail for cooperating agents running as one operating-system user, not a security boundary. The invariants guard against our own agents' mistakes; one residual risk is deferred (the partner's subagents cannot be told apart from it).

| Invariant | Protection |
|---|---|
| I1: no act in a person's name without its impact stated first and recorded | `--impact` is required, or the act is refused |
| I2: a session's belief or environment never confers a person's authority | Only the grant's binding: the announced main session holding the named checkout's lease under the named lineage (`cmd/metasystem/attorney_admits.go:146-152`) |
| I3: the partner's checkout produces no goal work | The engine refuses claim, dispatch and landing there; its mutating verbs have no agent route |
| I4: a checkout's declaration changes only by the direct proof | One owner of declaration changes; a grant never answers it |

**Rabbit-hole risks** and what keeps step 1 out:

- A new authority model: none; one existing grant entry, one new history key.
- Rebuilding the browser's Partner: no browser code changes.
- Per-computer enrollment: today's per-checkout enrollment serves.
- Question classification: deferred with the addressee.
- The roster item: two existing keys are read.
- A launcher: the runtime's plain interactive command runs.
- A decisions-page generator: none; the boot reads existing lists.
- One impact shape for all overrides: free text now; goal overrides-state-their-impact owns it.

## Step 1

One slice of five units. What Wido does and sees:

1. In the checkout he chooses as the partner's home, at his own terminal: `metasystem partner start --for 1w`. He reads: "From now until Sat 17:20 (2026-10-10) the agent opened here acts in your name on the backlog and this checkout's machinery. Every act states its impact first and is recorded as yours through the partner. It never claims, builds or lands. End it: metasystem partner end." His agent opens in the same terminal with the role text and, from the second session on, its handoff note.
2. He talks. The partner explains, plans and runs backlog and budget verbs, each printing its impact line and the grant's notice. No helm, no pane.
3. `goal show G --history` reads "approve — Wido, through the partner (grant …)".
4. At turn end the partner is not told to claim; the steward starts nothing there and raises no idle alarm.
5. A seat whose budget the partner raised keeps working without releasing its claim.
6. An edit without `--impact`, or after he revoked the grant, is refused and the goal is unchanged; `settings coordinator --withdraw` from the partner is refused and the fences stay.
7. Before a session ends the partner updates `plans/handoff-project-partner.md`; his next `partner start` opens with it.

**`partner start`**, in order:

1. Proves the person at the enrolled terminal with no agent in the chain (`Prove`, `internal/humanauthority/authority.go:872`), as `grant add` requires (`internal/goal/general_attorney.go:33-35`). An agent caller is refused.
2. Prints what the mode causes (ruling R-143-m1e): what the partner may do, until when, how to end it.
3. Declares the checkout the partner's. The brain seat's record (`internal/brain/brain.go:57-63`) gains a role, `partner`; a record without one stays the brain's. As today, declaring refuses while the checkout holds claims or running jobs (`cmd/metasystem/brain.go:150-182`), and a host has one coordinator: a brain-declared checkout is refused, naming the command that withdraws it.
4. Records one "everything" grant (`internal/goal/root.go:48-73`; at most one week) for this machine, this checkout and the lineage `project-partner`. `grant add` reads the lineage from the lease holder and refuses with none (`cmd/metasystem/intent_grant_everything.go:116-130`); `partner start` passes the fixed lineage. A live grant is reused (question 1).
5. Replaces itself with the agent runtime in the same terminal, setting `METASYSTEM_OWNER_LINEAGE=project-partner` as a seat launch sets its own (`internal/launch/seat.go:17-19`). Runtime and model come from `ui.partner.runtime` and `ui.partner.model` (`internal/config/ui.go:26-30`), the browser Partner's keys; `--runtime` and `--model` override for one start. An unset runtime, the default, takes the automatic choice (`ResolveAutoRuntime`, `internal/config/autoruntime.go:59`: the first of claude, codex and devin whose program is on PATH), as goal partner-runtime-defaults-to-an-available-one asks of the browser Partner; when none is installed the launch refuses, naming what to install.

A repeat whose declaration and grant hold only opens the session; if a partner session already runs there, it says so and opens nothing. The partner has a checkout to itself, because the declaration fences the whole checkout. `status` there leads with the mode, on the grant's existing line (`cmd/metasystem/intent_helm.go:517-551`).

**`partner end`** revokes the grant and withdraws the declaration, by the direct proof. `grant revoke` alone, from any seat, leaves the role: the partner then only explains and proposes.

**The partner role.** The coordinator's checks that its checkout never carries a person's word (`cmd/metasystem/goalsync_mutations.go:734-747`, `:2806-2808`; `internal/ui/act/act.go:139-146`) hold for the brain role only. The role text is compiled in beside the brain's (`internal/brain/brain.go:41-47`). It tells the partner to say each impact in the conversation first, to bring Wido any choice no recorded word of his covers, never to let a subagent run an act, and that the claim sentence of `AGENTS.md` does not apply.

**Fences.** The brain's fence (`internal/brain/brain.go:445-468`) already refuses claim (`cmd/metasystem/goalsync_mutations.go:1820`), dispatch (`cmd/metasystem/delegate.go:133`) and landing (`cmd/metasystem/landing_path.go:535`). Its texts become role-aware: "this is the project partner's checkout; the partner never claims; a seat does", and the same shape for dispatching a builder and for landing.

**One owner of declaration changes** (finding PARTNER-DECLARATION-AUTHORITY). Today the owner (`cmd/metasystem/brain.go:54-76,85,198`) classifies its caller by `personVerbCaller` (`:37`), which makes a grantee human (`cmd/metasystem/person_class.go:39-43`): a partner running `settings coordinator --withdraw` (`cmd/metasystem/intent_operations.go:456`) by mistake would keep its grant and lose its fences. Decision: `settings coordinator --declare` and `--withdraw`, `partner start` and `partner end` all go through this one owner, which takes the direct proof instead of the person class. `personVerbCaller` keeps its other callers. Refusal: "withdrawing the coordinator declaration is the person's own act, at their own terminal; a grant never stands in for it; nothing was done".

**No agent route** (finding PARTNER-IMPACT-FALLBACK). Today `actingAs` tries the grant (`cmd/metasystem/intent_planning.go:491-498`) and otherwise takes the agent route (`:499`), which queued-goal edits accept (`internal/goal/verbs.go:3782`): a forgotten `--impact`, or an act after revoke or expiry, would land as an agent's act. Decision: in a checkout declared with the partner role, a caller under the `project-partner` lineage has no agent branch for a mutating verb. Unless the grant admits the session and `--impact` is present, the verb is refused at actor selection, before any owner runs. Verbs that select their actor elsewhere (budget, `cmd/metasystem/intent_goals.go:485`; restarts) call the same check; reads are untouched. The refusal names what is missing, then the command to run:

- "goal edit is the person's act through the partner; the grant does not admit this session (revoked 17:20 by m1e); nothing was done", then `metasystem partner start`, at his terminal;
- "…; --impact is missing; nothing was done", then the verb with `--impact "WHY; undo: HOW"`.

The grant log gets a "refused" line (`internal/humanauthority/attorney_log.go:51`).

**`--impact`.** Every verb accepts `--impact TEXT` beside `--json` and `--verbose` (`cmd/metasystem/intent.go:99-103`). It is free text, required for every act a partner grant answers: checked at actor selection and in the admitter (`cmd/metasystem/attorney_admits.go:163-183`). The act's first printed line is the impact, and the grant's log line carries it.

**`through=`** (question 7). One optional history key, `through=<grant id>`, valid only beside a `human:` actor (`internal/goal/file.go:447-472`, validation `:2112-2133`). One place stamps it: the wrapper that re-checks the grant on every ledger write (`internal/goal/general_attorney.go:158-186`) marks the lines of its own operation. `goal show --history` reads it as "Wido, through the partner" when the grant's lineage is `project-partner`. The history grammar is closed, so every seat's engine is rebuilt before the first partner act; the help says so.

**Limits.** The grant's existing limits stay the person's own (adding a grant, enrolling a terminal, the helm, authority-defining settings, `goal done --force`: question 2); question 6 asks whether to add any.

**Budget.** A budget act from the partner's checkout must not replace the holder's claim epoch (the counter by which the engine knows which session holds a claim). The engine keeps the recorded epoch for an actor that does not hold the claim, on main since commit `73aa5158f` (`internal/goal/verbs.go:1824-1831`). Unit 4 proves it through a partner grant; the edge that passes the caller's lease epoch (`cmd/metasystem/goalsync_mutations.go:699-703`) changes only if that test fails.

**Turn end.** The Stop gate already skips the idle-with-backlog refusal for a declared coordinator checkout (`internal/goal/turnverdict.go:471,575-577`); the partner role inherits it. The session still reads `metasystem session status`, whose report says no claim is asked.

**Steward guard.** A steward that finds claimable backlog and no live work escalates on every tick (`internal/steward/verdict.go:111-114`), reports a live idle session (`:125-127`) and may start a seat (`internal/steward/seat_start.go:319`). Decision: one guard where the tick decides (`internal/steward/tick.go:621-639`): in a declared coordinator checkout no seat starts and no idle alarm is raised. Nothing revives a partner session; the person opens it again.

**Handoff note** (finding MOVED-EFFECTS-PARTNER-HANDOFF). `session handoff` refuses a session without a goal (`HANDOFF_NO_GOAL`, `internal/steward/handoff_capture.go:396-406`) and stages a continuation that revives the session; the partner holds no goal and is never revived. Decision: the partner keeps one note, `plans/handoff-project-partner.md`, in its own checkout (the convention of `plans/README.md`, "Handoff Notes"), as an ordinary file edit before a session ends. The boot (`cmd/metasystem/brain_boot.go:92`) delivers the role text and reads the note from the working tree as a fifth optional section beside asks, held, fleet and digest (`:145`), with 20 of the 100 shares, taken from the digest's 35 (`:177`). A longer note is cut at its end and reported "cut"; a missing one "skipped". Nothing is staged.

**Units.** Tests use the existing owner seams and stubbed Git. Lines are changed lines including tests, by judgement: about 1,410.

| # | Builds | Files | Tests | Lines |
|---|---|---|---|---|
| 1 | Role, fence texts, role text, direct proof | `internal/brain/brain.go`, new `internal/brain/partner-packet.md`, `cmd/metasystem/brain.go`, `goalsync_mutations.go`, `person_class.go`, `intent_operations.go`, `internal/ui/act/act.go` | `TestPartnerRoleFencesClaimDispatchLandAndCarriesThePersonsWord`; `TestPartnerGrantNeverWithdrawsTheDeclaration` | 330 |
| 2 | `partner start`, `partner end`, status line, boot note | new `cmd/metasystem/intent_partner.go`, new `internal/launch/partner.go`, `cmd/metasystem/intent_helm.go`, `brain_boot.go` | `TestPartnerStartDeclaresGrantsAndLaunchesUnderThePartnerLineage`; `TestPartnerStartRefusesAnAgentCaller`; `TestPartnerStartWithUnsetRuntime` (unset key, claude absent and codex on PATH: codex is launched); `TestPartnerBootReadsTheHandoffNoteWithinItsShare` | 490 |
| 3 | `--impact`, no agent route, `through=` | `cmd/metasystem/intent.go`, `intent_planning.go`, `intent_goals.go`, `attorney_admits.go`, `internal/humanauthority/attorney_log.go`, `internal/goal/file.go`, `general_attorney.go` | `TestPartnerActNeedsImpactAndIsRecordedThroughTheGrant`; `TestHistoryLineThroughRoundTrips`; `TestPartnerEditWithoutImpactLeavesTheGoalUnchanged` (`goal edit G --next TEXT` on a queued goal); `TestPartnerEditAfterRevokeLeavesTheGoalUnchanged` | 470 |
| 4 | The budget proof | `internal/goal/verbs.go`, `cmd/metasystem/goalsync_mutations.go` | `TestBudgetActFromAnotherCheckoutKeepsTheHoldersClaimEpoch` | 40 |
| 5 | The steward guard | `internal/steward/tick.go`, `seat_start.go` | `TestStewardLeavesACoordinatorCheckoutAlone` | 80 |

## Where each critique finding lands

| Finding | In step 1 | Unit and test |
|---|---|---|
| PARTNER-DECLARATION-AUTHORITY | "One owner of declaration changes"; invariant I4 | 1, `TestPartnerGrantNeverWithdrawsTheDeclaration` |
| PARTNER-IMPACT-FALLBACK | "No agent route" | 3, the two `…LeavesTheGoalUnchanged` tests |
| MOVED-EFFECTS-PARTNER-HANDOFF | "Handoff note"; Moved effects below | 2, `TestPartnerBootReadsTheHandoffNoteWithinItsShare` |
| PARTNER-START-UNSET-RUNTIME (round 2) | `partner start` step 5 | 2, `TestPartnerStartWithUnsetRuntime` |

## Moved effects

| Effect | From | To | Code |
|---|---|---|---|
| The claim epoch written when a budget act rebinds a claimed goal | the caller's own checkout lease, whoever calls | the claim's recorded epoch, unless the caller holds the claim | `metasystem/cmd/metasystem/goalsync_mutations.go:699-703`; `metasystem/internal/goal/verbs.go:1824-1831` |
| The lineage a general grant binds | read from the lease holder at `grant add` | for a partner grant, named by `partner start` | `metasystem/cmd/metasystem/intent_grant_everything.go:116-130` |

The first row's engine half is on main since commit `73aa5158f` (goal machinery-blocks-of-2026-10-02); unit 4 proves it for the partner. The handoff note moves no owner: it is an ordinary file the partner edits and the boot reads, and handoff capture and continuation keep theirs. The direct proof and the actor-selection refusal change checks inside existing owners.

## Open questions for Wido that step 1 needs, and their answers

Numbered as in revision 2; questions 3, 4 and 5 are in their deferred rows. Each was answered on 2026-10-03 at 21:14 by seat m1e for Wido, under his power of attorney, because each follows a ruling he had already given; each answer is recommendation (a), and Wido can overturn any of them.

- **Question 1. Does `partner start` grant the authority itself, and for how long?** Answer (a): yes; `--for` or `--until` is required when no grant is live, a live one is reused, and no duration is ever silent. Following his ruling of 2026-09-30 on the general power of attorney (`--acts everything`, durations he names, at most a week). The other options were: (b) 24 hours when omitted; (c) one week when omitted; (d) `grant add` stays a second command.
- **Question 2. `goal done --force`.** Answer (a): it stays his at the helm; the partner states the impact and prints the commands. Following R-142-m1e and the two accepted designs that keep the act that skips the proof of done in his hands. The other option was (b): the partner runs it on his word.
- **Question 6. Acts refused to the partner beyond today's limits.** Answer (a): none; the partner approves, accepts a risk and raises a budget on his word, each act with its impact and undo in history. Following his word of 2026-10-03 ("all is approved, you can use my word") and the grant of 2026-09-30. The other option was (b): accept-risk and budgets above a tier's norm stay his by hand.
- **Question 7. A new key in the ledger's history lines (`through=`).** Answer (a): add it. Every seat's engine is rebuilt in one act before the first partner act, the hard cutover he asked for on 2026-09-30 (no mixed engine versions); that rebuild runs once goal landing-deploys-the-engine's step 1 is on main and proven, and the partner's first act waits for it. The other option was (b): no ledger change, "through the partner" only in the partner checkout's local log.

## Deferred

| Item | Builds on | What it is |
|---|---|---|
| Records route (wall 1, unit 9) | the land fence | The partner's record edits (rulings, decisions page, drafts) wait on a branch until a hand-in passes the fence (needs goal records-land-through-the-lane). Decide when this is taken up: emergency hand-landing (a) never, recommended; (b) on Wido's recorded word for one commit. |
| `goal done --force` (wall 3) | question 2 | Stays at the helm unless question 2 says otherwise. |
| Reach into other checkouts (wall 4, unit 8) | the grant's home checkout | The partner session's grant honoured in every checkout here that shares the ledger: accept-risk where the review lives, restarting another seat. Decide when this is taken up: (a) yes, recommended; one grant then reaches every seat's machinery here; (b) no; the partner asks that seat. |
| Tier raise on his word (wall 5, unit 7) | the grant's person proof | `goal edit --tier` takes his words, not a machine reference, as evidence (machinery item 61). |
| Channel answers (wall 7, unit 6) | `through=`, `--impact` | `question answer` through the partner writes the answer record and history line and posts "answered for you by your partner" in the thread; authority-kind questions stay refused. |
| Question addressee (unit 10) | channel answers | A question addressed to the partner reaches Wido's channel only on escalation or with no live partner; this moves a question's first delivery (goal questions-reach-the-person-only-when-his). |
| Seats' design drafts reaching main (wall 10) | nothing | Not the partner's gate (machinery item 59, in work). |
| Browser panel (unit 11) | `through=`, the status line | History and the workspace show "through the partner", with the mode's end time and a revoke button. Decide when this is taken up: the panel (a) stays propose-and-press, recommended; (b) becomes a window on the live partner session, so an agent request could become a human act. |
| Roster row (unit 12) | the two keys | One `partner` row for both surfaces replaces them (goal rosters-are-configuration-items). |
| Sending reviews to a critic | the dispatch fence | A fence exception for review jobs funded by a named goal; until then the owning seat is asked. |
| Generated impact statements | `--impact` | Goal overrides-state-their-impact owns the generated statement and the history field `--impact` will fill. |
| Telling the partner's subagents apart | the grant's binding | Process ancestry cannot separate them (`internal/lease/classify.go:420-426`); the grant design deferred it too. |
| A handoff verb for goal-free sessions | the handoff note | Handoff capture learns a caller with no goal and no revival. |
| The grant log's tail at boot | the note at boot | A new session also reads what was done in Wido's name since. |

## What was not checked

- Nothing was built or run; line estimates are judgement.
- Read, not run: the grant admits restarts in its own checkout; the budget verb's path (`cmd/metasystem/intent_goals.go:485`) admits a partner grant without `actingAs`.
- Whether a claimed goal always records the epoch the landed budget change keeps; unit 4's test decides.
- Whether the ledger commit's record of the executing lineage (`internal/goal/txn.go:704`) could replace `through=` (question 7); the scoped grant's history fields cannot, being refused beside a `human:` actor (`internal/goal/file.go:2115-2118`).
- How a steward treats a brain-declared checkout today, or whether one is armed in m1e.
- Not read: the helm designs, the two Partner-proposal designs, each runtime's interactive command line.
