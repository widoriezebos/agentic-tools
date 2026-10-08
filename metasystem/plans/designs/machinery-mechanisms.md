# The machinery's shared mechanisms

Status: accepted
Written: 2026-10-06 by m1e in Wido's word, from `plans/machinery-open-findings-plan-2026-10-06.md` (the overlaps section). Record names are proposals for Wido's naming pass; the shapes are the design. The verbs follow the rule of the verb redesign (object-action, along a person's intent, intuitive without context): every mechanism's human face is an action on an object a person already knows; the section "Verbs" says which, and why no new object was needed but one.
Critique: four Astra rounds (gpt-6-astra). Round 1 on this page alone: 9 material, folded. Round 1 on the set with the plan: 12 material, folded. Round 2 on the set: 6 material, 4 of them repeats of the previous round's classes, folded as the acceptance list. Stopped under the stop rule (material fell 12 to 6, but classes repeated and two corrections are used). Round 4 on the verbs: 10 material (scope and precedence of policy settings, one value grammar, settings check stays structural, helm --all symmetry, questions reach people and agent asks reach the coordinator, an act request closes by its act, the trunk-red override is work land --exception, flakes are incidents only on main, worktree ownership in status, the split and dropped dispositions through work review --dispositions), folded; stopped after one round as a design round. Open: Wido's naming pass and the headless-session ruling.

## Why this page

Twenty-one open findings share eleven mechanisms. Without one design, each goal grows its own counter, register and lock, which is how the week ended with three registers and a stop rule that lived in a person. This page fixes the shapes once. Nothing on it is built on its own: each mechanism is built inside the first goal that needs it (the landing redesign builds most), as the smallest thing that consumer needs, to the shape here.

Everything here is language-neutral. Anything an application must say for itself is a declaration (mechanism 5).

## Invariants the mechanisms keep

- A person's act always takes effect at once; the machinery says why it would have decided otherwise, never refuses.
- The record holds every outcome; no verb reads a person's memory.
- An environment failure never counts as an attempt.
- Every loop declares its stop (mechanism 2); a loop without one does not ship.
- Every value an application must provide is a declaration; the machinery never guesses it.

## 1. Policies and the helm

A policy is a named decision the machinery takes repeatedly. Its value is one of `auto` (decide by rule), `capped N` (auto within N), `person` (prepare, say what auto would do, wait for a person's act).

Record `policy`: name, checkout (the seat or lane checkout whose setting it is; fleet-wide keys are the coordinator checkout's), value, set-by, at, previous (the value to restore after the helm). Precedence: a helm override, then the checkout's own setting, then the coordinator's default, then the built-in `auto`.

The policies this plan names, as settings keys a person reads as sentences, each with its one-token values: `landing.batch` (`auto` | a whole number, the cap | `person`); `landing.proof` (`auto` | `person`); `landing.on-red` (`auto` | `person`); `landing.trunk-red` (`auto` | `person`); `seat.driver` (`auto` | `person`); `review.stop` (`auto` | a whole number, the corrections allowed | `person`); `goal.raise` (`auto` | `person`); `question.route` (`auto` | `person`); `settings.apply` (`boundary` | `now`). One token per value, as `settings set KEY VALUE` takes it.

Verbs: a policy is a setting a person already knows how to read and set, in the checkout it governs: `settings set landing.on-red person` in the lane checkout (or with `--repo` pointing there), `settings set seat.driver person` in a seat checkout, fleet-wide keys (`question.route`) in the coordinator checkout (`settings coordinator` says which). `settings show KEY` prints the effective value, the checkout it came from, who set it, and the helm override when one stands; `settings check` validates the values' grammar. The helm exists: `helm take` on a seat checkout sets every policy of that seat to `person` and keeps the previous values; on the lane checkout it also drains the lane (`landing drain`); `helm take --all` does it for every machine on this computer (the form `machine stop --all` already uses); `helm return` and `helm return --all` restore; `helm status` and `helm status --all` say who holds what. The board shows the effective value and who set it.

First consumer: the lane's four policies (landing redesign, 1b).

## 2. The stop

Every repeated activity declares: an attempt budget; a progress measure that must change every attempt; a class whose repeat stops at once; the decision taken when the evidence arrives, before the next attempt is composed; a handoff at the stop.

Record `stop`: loop (`unit-round` | `design-round` | `lane-proof` | `lane-return` | `retry` | `revival` | `raise` | `hot-fix` | `fix-forward` | `refusal` | `supervision`), subject (goal, unit, batch, seat or component), attempt n of budget, measure (name, previous, now), class, decision (`close` | `continue` | `stop`), handoff (only with `stop`: `split <unit>` | `drop` | `return <goal> <cause>` | `hold <register id>` | `ask <ask id>` | `accept <ids>` | `split <goal>` | `stopped <cause>`), at. `close` is a decision with no handoff; `accept` is the design round's acceptance with its noted list, and `split <goal>` transfers its unresolved sections to the named follow-up goal; `stopped <cause>` retains an exhausted retry or unknown review input; for a unit or design round it holds that unit or chain and authorizes no continuation.

Rules, tested in this order: a clean result closes the loop (`close`, the unit or batch is done); otherwise `continue` only when the loop's declared improvement held (the material count fell, the red set shrank, the cause differs: never mere change), the class is new, and n < budget; otherwise `stop` with a handoff. The handoff is selected by loop and cause. Lane loops (`lane-proof`, `lane-return`): a proven `own` or `other` defect returns to the named goal (`return <goal> <cause>`); `main` joins the red entry and holds; an unresolved flake or an exhausted environment retry holds and asks with the cause; `unclassified` holds and asks; a return always names its target goal. `unit-round`: unknown read input permits one fresh examination under the retry rule, then records `stopped <cause>` on the unit-round and holds the unit if evidence is still unknown; otherwise `split <unit>` or `drop`, by the plan's rule (split when the design needs the unit); a split carries the finding's stop history into the new unit, and a second stop on the same unresolved finding is `ask`, never another split. `design-round`: unknown read input permits one fresh examination under the retry rule, then records `stopped <cause>` on the design-round and holds the chain if evidence is still unknown; otherwise `accept <ids>` after folding concrete changes into unit acceptance items, or `split <goal>` for the remaining unbuildable sections, following the plan's updated stop table; convergence never asks a person to choose that exit. A split with a buildable remainder records both acceptance of that remainder and transfer of unresolved sections. The owner publishes its accepted status, closed-critique head and acceptance items before closing the chain. Design-round handling (including these outcomes in `Decide`) and drop effects are built in `review-drops-and-design-convergence`; the source review-chain goal builds the unit unknown handoff and leaves optional drops as prepared asks. A fresh examination consumes only the one retry, not a completed review round; unknown evidence never refuses a person's explicit act, whose impact is stated and recorded first. `retry`: `stopped <cause>`. `revival`: stop the seat and `ask`. `raise`: `ask`. `hot-fix`: `ask` for a fix goal with a design. `fix-forward`: `ask` for a design (step back). `refusal`: a register entry and `ask` to the helm holder. `supervision`: a register `finding`. The loop's owner applies the handoff; the consumer named for it in the plan acts on it.

Attempts and retries are counted apart: a retry execution consumes the retry loop's own budget (one), while an environment failure consumes no round of the unit and no allowance of the goal's box. The identity of a stop is `(loop, subject, attempt)`, where a retry's subject is the failed step's launch id, so two retry sequences in one unit, or a retry and a correction, never join or overwrite each other. The handoff is a record the next level reads (the goal for a split or drop, the lane for a return or hold, the ask mechanism for an ask). A stop record is also a register entry of kind `stop` (mechanism 4). The policy `review.stop` and its siblings give the budget (`auto` the declared one, `capped N`, `person` stops at every attempt).

Verbs: none new. A stop is a fact about its subject, so `work status GOAL --work UNIT` shows a unit's stop (attempt, measure, class, handoff), `landing status` shows a batch's, `status` shows the seat's; the stop itself is taken by the loop's owner, never by a verb.

First consumer: the lane's on-red (landing 1a: record, close, budget, handoff), then unit rounds (review chain).

## 3. The cause

One classification of why an attempt failed, carried by every red proof, failed step, return and retry.

Values: `own` (the work's own change), `main` (main's own red), `other <goal>` (another goal in the batch), `flake <test>` (a registered flake), `environment <kind>` with kind one of `provider-limit`, `capacity`, `lost-process`, `busy-path`, `denied-sandbox`, `stale-engine`, `tree-changed`, `deadline`, and `unclassified` (the observer could not tell).

Rules: only `own` returns work to its seat or counts an attempt; `main` opens or joins a trunk-red register entry and holds the batch; `other` returns that goal; `flake` re-proves once; `environment` retries once and then stops with the cause. The observer records what it saw; it never guesses `own`. A failed test alone is `unclassified` until attribution: the lane attributes by the bounded replay of the plan (the merges one at a time with the cheap gate, the full proof only where the cheap gate passes, at most two full proofs), against the register (a known flake), the host record (an environment cause) and the trunk entry (main's red). Insufficient evidence or an exhausted proof budget is `unclassified`: hold and ask, never a return.

First consumer: the lane's red classification (1a).

## 4. The register

One register of what the machinery found, with kinds. Written only by the component that found the thing; read by `work land`, `goal done`, health and the board.

Record `register`: id, kind (`red` | `flake` | `refusal` | `stop` | `finding`), found-by (component and launch id), subject (commit, test, refusal code, unit), evidence (the path of the output that shows it; for a red, the failing test's own output), status (`open` | `cleared`), cleared-by (a proof that went green, a person, a landed fix goal), opened-at, cleared-at.

Rules: one entry per canonical key per kind, never a commit per tick: a `red` on main is keyed `(main commit, test)` and a red on a candidate tree (a batch's merge tree) is keyed `(candidate tree, test)`, so the lane and the trunk timer finding the same red create-or-join one entry atomically and the finder is provenance, not identity; a `flake` is keyed by test; a `refusal` by code; a `stop` by its loop, subject and attempt; a `finding` by the read's finding id; a `red` entry on main blocks landing except for the goal that claimed the incident as its fix (`incident claim`); a person overrides the block for one landing with the existing exception form, `work land GOAL --exception CODE --reason TEXT`, which the lane consumes while the incident stays open; a `flake` entry makes the lane re-prove that test once before returning; a `refusal` entry is written by the steward when one refusal code repeats N times in an hour across seats; a `finding` entry is what a read leaves open against a named unit after a split.

Verbs: none new; "register" is a store, not an intent. A red on main is an `incident` (`incident list`, `incident claim` by the goal whose work fixes it, which is the trunk-red policy's "fix goal", `incident close --reason`); a flake seen on main is an incident of its own kind, since someone must own its fix, and `incident list` keeps its meaning "a failure on main"; a flake seen only on a candidate tree is a `test` fact (`test status` shows the registered flakes) that the lane re-proves once, never an incident; a repeated refusal is an `alert` (`alert list`, `alert ack`, `alert clear`); a stop and an open finding against a unit show in `work status`. The machinery clears by evidence; a person closes with `incident close` or `alert clear`.

First consumer: trunk red (1a).

## 5. Declarations

One file per adopter, read by one reader, holding everything the machinery must not guess.

Keys: `proof.cheap` (command: static plus impacted), `proof.full` (command: the whole suite), `proof.audits` (commands), `launch.deadline` (minutes per launch kind: build, read, design, proof; never a seat session; `proof.deadline` is its proof entry); no session limit is declared: a headless session ends at the unit boundary (mechanism 7) and nothing caps its context, by Wido's ruling of 10-06, `proof.trunk-every` (duration), `host.builds` (parallel builds), `host.proofs` (parallel proofs, default 1), `host.load-max`, `remedy.<role>` (the verb that clears a health role, with the condition that proves it), `delete.rules` (fixed: empty ids refuse, errors never discarded, paths never widen, the protected set is explicit, a dry run precedes). A design page declares the `areas` its goal edits (paths or globs) in its own table; the claim copies them.

Rules: a missing key with no default refuses the verb that needs it, naming the key; a repository declaration (`proof.*`, `launch.deadline`, `remedy.*`, `delete.rules`, `settings.apply`) is committed-only: a local or environment value for it is ignored and reported, and `settings set` writes it to the committed file; a computer fact (`host.*`) lives in the registered owner checkout's local file; the values have grammars `settings check` validates (a command is one line, run as `landing.prove.command` is today); `system adopt` clears the template's declarations and leaves the adopter's required keys explicitly missing; no command anywhere else in the machinery runs a suite or an audit; `settings show KEY` prints the effective value with its source.

Verbs: none new. A declaration is a committed setting of the application: `settings show proof.full` with its source, `settings keys` for the whole set, `settings check` for "every declared key is present and every declared command resolves" (structural, changing nothing, as its help promises). Running the declared commands is `test run` for the cheap rung and `landing prove` for the full one. The adoption step (`system adopt`, `system setup`) writes the first set.

First consumer: the proof ladder (1a).

## 6. The proof ladder

| Rung | Who runs it | Command | When |
| --- | --- | --- | --- |
| builder's check | the builder, in the proof step's environment | `proof.cheap` + `proof.audits` | at the end of every build |
| unit proof | the unit's proof step | the same command, to attest | after every build |
| merge gate | the lane, per merge in a batch | `proof.cheap` | after every merge |
| batch proof | the lane, once per batch, on the exact tree it pushes | `proof.full` | before every push |
| trunk check | the trunk timer, on main | `proof.full` | every `proof.trunk-every` (a push's own batch proof already covers the pushed tree) |

Rules: every rung runs under `proof.deadline` in its own process group on a tree nobody else owns (mechanism 9); a red at any rung carries a cause (3); the batch proof and the trunk check write `red` register entries (4); nothing outside this table runs a suite. A builder's own check that runs `proof.full` is a defect of the scaffold.

First consumer: the lane (1a) for the merge gate, batch proof and trunk check; the review chain for the first two rungs.

## 7. The unit boundary

One event, owned by the seat's steward, emitted when the current unit's outcome is recorded: no running step, no pending proof, no critic job or read awaiting its decisions, no pending revise, no pending ask. A build whose proof is queued is not a boundary.

Record `boundary`: seat, at, consumers (which of the following acted and what they did).

A headless session ends at a completed unit boundary (this event) and nowhere else: the boundary ends the session with its handoff and the steward starts the successor when work remains. No context fraction, hour limit or token ceiling ends a session (Wido, 10-06: vendor defaults govern context and compaction until hard data says otherwise); the steward records each session's peak context, calls and tokens for the board. A session that dies without a handoff is classified before any restart, by its exit: a provider limit (every one of the week's 75 error deaths was one: 64 session limits, 11 weekly) is an environment cause that follows the host record's hold-and-probe (mechanism 8, plan finding 4), never a restart; any other death is a `revival` loop under the stop (mechanism 2: the same death reason twice stops the seat with the reason), with two restarts an hour as the backstop. The measurement covers failed sessions as well as completed ones (today 75 of 104 records are unmeasured), and records each boundary's time and the reason a boundary is blocked. Consumers of the boundary, in this order: the session ends with its handoff; a settings change takes effect; the engine re-arms to the host stamp (a session pinned to an engine is replaced at the boundary: a running session finishes on the engine it started with, ruling R-144); a budget raise is admitted; last, the driver starts the next unit of the design or hands in the finished goal. A round admitted at a boundary runs every step on the settings and engine pinned at its admission; the host stamp is compared only at admission, never against a running round. Nothing else happens between units; inside a round only the steward's deadline and ceiling enforcement (mechanism 8) may interrupt.

First consumer: sessions (fleet goal: the minimal event), then the driver (runs-advance).

## 8. The host record

One record per host, owned by the steward of the checkout registered as this computer's landing lane (`landing set`: the lane's record and lock already name one checkout per computer); a computer without a lane registers its owner checkout the same way. Two claimants or a missing owner refuse with a message; the facts an owner publishes carry its registration and go stale when it is unregistered. No election and no self-declaration.

Record `host`: providers (name, mark with its reset time, last probe), capacity (declared builds and proofs, current load, the queue of steps waiting with their reasons), engine (stamp, commit, armed-at), trunk (open red entry or none, last full proof at), usage (per seat and provider: calls and tokens in the current window, against the account's limits).

Rules: the host's steward enforces `launch.deadline` (declared per kind: build, read, design, proof) while work runs: a step over its deadline is stopped with cause `deadline` and the round stops with that reason (no retry, a stop record with handoff `ask`). This is the only interruption inside a round; no ceiling interrupts a session. One provider mark per host, cleared by a probe at its reset, never by a computed date; a mark older than its reset plus a grace is cleared and alerted; budget clocks pause while a mark stands; a step beyond capacity queues with its reason; the engine stamp is what preflights compare.

Verbs: none new. The host record is what `machine list` already shows for "this computer" (presence and what runs here), extended with the provider marks, the capacity queue, the engine stamp and the usage; `landing status` shows the trunk entry and the last full proof. "host" is not an object a person reaches for; "machine" and "this computer" are the words in use.

First consumer: the fleet goal; the trunk entry from 1a.

## 9. Tree ownership

A tree (a worktree, a checkout, the lane's merge tree) has one owner at a time: a round, a proof, a merge, or the lane's lease. The owner holds a lock in the tree's state; a second actor waits and says for whom.

Rules: nothing writes into an owned tree except its owner; machinery logs, digests and state live in the state root or an ignored artifacts directory, never in a code tree; a proof runs on a tree nobody else owns, and `tree-changed` during a proof is an environment cause, not the work's; the lane's lease is released while a model builds a resolution and taken again to apply it; a worktree is removed only after a repack.

Verbs: none new. Who owns a worktree, and why a build waits, is a fact `work status GOAL` shows for the goal's worktree and the top-level `status` shows for the seat's checkout (`session status` is the exact Stop report and stays that); `session isolate` already makes a worktree for a second writer.

First consumer: rounds (review chain) and the lane's lease (1a).

## 10. The read record

One shape for every read, returned by the reader and stored in the launch record and the unit round.

Record `read`: id, subject (tree and commit), engine stamp, model, material count, findings (id = read id plus a number, class, severity, title, where), output path (unique to this launch), carried-from (the read this one re-keys, for a carry).

Finding classes: `regression`, `weakened-test`, `incomplete-item`, `false-premise`, `faked-seam`, `missing-reader`, `scope`, `other`.

The author's decisions are a record of their own, `decision`: finding id (the read's), disposition (`accepted` | `refuted` | `out-of-scope` | `noted` | `accepted-risk` | `split <unit>` | `dropped`), reasoning, amendment, decided-by, at. The read stays the source of findings; the decision record is the source of dispositions. The decisions reach the machinery the way they do today, a row per finding in the file given to `work review GOAL --work UNIT --dispositions FILE`; `split <unit>` and `dropped` are two more words that file accepts. A transferred finding is cleared by the named unit's clean review; where a person must discharge one by hand, the existing `work review ... --finding F --test NAME` form applies.

Rules: the stop (2) reads the material count and the classes from here and the dispositions from the decision record, nowhere else; a carry after a correction re-keys unit reads and by-commit reads alike; an output path is never shared between launches.

First consumer: the review chain.

## 11. The scaffold

`work brief` composes every brief, and refuses one that breaks the rules.

Sections, in order: Goal; Decisions on round N (for a correction, from the decision record joined to the read's finding ids); Workspace; Units with the declared size; What this unit builds; Not in this unit; Readers of what this unit changes (the grep list filled for every name the brief cites); A test through the public verb; Check (composed from `proof.cheap` and `proof.audits`); Deletion rules (when the unit deletes); Constraints (effort from the unit's size: `high` under 150 lines, `xhigh` for design-level units); Expected Return; Acceptance Criteria.

Rules: a cited `file:line` that does not exist at the base commit refuses the brief; a brief with its own suite command refuses; a diff over twice the declared size holds the round for a person.

First consumer: the briefs goal; the check composition earlier, in the review chain.

## 12. Ask

One way for the machinery to ask a person for something it cannot decide or do.

Record `ask`: id, from (component and subject), needs (the act and its arguments, as a command a person can paste), class, to (`helm` | `person`), asked-at, answered-by, answer, answered-at.

Rules: an ask is a request for an act, distinct from a question that takes a text answer: its text is the one command that performs the act, `question show` prints it, and the ask closes when that act's record appears naming the subject (the steward matches act and subject; no text answer closes it). Routing: an act inside the coordinator seat's grant goes to that seat as an `agent ask` (agents message agents through `agent`, never through `question`); an act outside the grant, or when no grant stands, goes to the person through `question ask`, which reaches a person and never an agent; the grant is checked when the ask is routed and again when the act executes; when the grant expires or is revoked, every pending agent ask is rerouted to the person as a question, and that reroute is never suppressed as a repeat; a class that repeats N times in an hour becomes one `refusal` register entry that coalesces the notifications, while every subject's own ask (its component, subject and the act it needs) stays open beneath that entry, answerable and reroutable one by one; the board lists open asks with the one command each needs; a policy at `person` asks through this record and nothing else.

Verbs: for a person, the `question` object is the intent exactly: `question list`, `question show` (prints the command the act needs), `question withdraw`, `question wait`; a request for an act is closed by the act, so `question answer` is for text questions and says, for an act request, which command closes it. For the coordinator seat, `agent ask` and `agent reply` (exist). The routing rule lives in the machinery's side of `question ask` and `agent ask`.

First consumer: the lane's hold (landing 1a: the record), then routing and the repeat rule (remedies).

## Verbs

The rule of the verb redesign: a verb is object-action, along a person's intent, and intuitive without context; internal structure never names a verb. The first draft of this page proposed seven new objects (`policy`, `stop`, `register`, `declare`, `host`, `tree`, `ask`); every one named a mechanism, not an intent. Judged one by one:

| Draft | Intent behind it | Verb |
| --- | --- | --- |
| `policy set/show` | "let the machinery decide this itself", "cap it", "I decide" | `settings set KEY auto|N|person`, `settings show KEY` |
| `helm take/release` | "I take over", "it is yours again" | `helm take [--all]`, `helm return [--all]`, `helm status [--all]` (`helm` exists; `--all` as in `machine stop --all`) |
| `stop show` | "why did this unit stop, what happens next" | `work status GOAL --work UNIT` |
| `register list/show/clear` | "what is broken on main and who owns it", "which tests are flaky", "what should I look at" | `incident list/claim/close` (main only), `test status` (flakes on candidates), `alert list/ack/clear` (exist) |
| `declare show/check` | "what does this application tell the machinery", "is it complete", "do its commands run" | `settings show/keys/check` (exist, structural); `test run`, `landing prove` for running them |
| `host show` | "how is this computer doing" | `machine list` (exists), `landing status` |
| `tree who` | "who is working here, why does my build wait" | `work status GOAL`, `status` (exist) |
| `ask list/answer` | "what does the machinery need from me" | `question list/show/withdraw/wait` for a person (the act closes it; `question answer` for text questions); `agent ask/reply` for the coordinator seat (exist) |

One new action on an existing object, because no existing action says it: `landing drain`: "take no new work, finish what you have, then hold" (today `landing stop` pauses at once). A person at the helm of the lane gets it from `helm take` on the lane checkout.

The policy keys read as sentences a person would say about the object: `landing.on-red person` is "on a red, the lane asks me"; `review.stop auto` is "reviews stop by the rule"; `seat.driver auto` is "the steward advances this seat's work".

## Where each mechanism is built

| Mechanism | Built in | Reused by |
| --- | --- | --- |
| 1 policies and helm | landing 1b | runs-advance, review chain, fleet, remedies |
| 2 the stop | landing 1a (record, close, budget, handoff for on red) | review chain (rounds), fleet, runs-advance, remedies, briefs |
| 3 the cause | landing 1a | review chain, fleet, housekeeping |
| 4 the register | landing 1a (red, and the minimal stop kind for the lane's stops) | review chain (finding), remedies (refusal), housekeeping (flake) |
| 5 declarations | landing 1a | every later goal adds keys |
| 6 the proof ladder | landing 1a (lane rungs) | review chain (builder and unit rungs) |
| 7 the unit boundary | fleet (the event, for sessions) | runs-advance (the driver), housekeeping |
| 8 the host record | fleet | landing (trunk), housekeeping (engine) |
| 9 tree ownership | review chain (rounds), landing 1a (lease) | housekeeping |
| 10 the read record | review chain | housekeeping, briefs |
| 11 the scaffold | briefs (check composition in the review chain) | all |
| 12 ask | landing 1a (the record, for hold) | remedies (routing, repeat rule), review chain, runs-advance |

## Open for Wido

- The names of every verb and record above.
- Whether declarations live in `metasystem.conf` or in a file of their own. Recommendation (m1e, 10-06): no file of their own. A declaration is a settings key under the four sources that exist (compiled default, `metasystem.conf` committed for everyone who uses the repository, `metasystem.conf.local` uncommitted per checkout, the environment), so one reader, one precedence and `settings show KEY (source)` already serve it. Repository truths every seat must share (`proof.cheap`, `proof.full`, `proof.audits`, `launch.deadline`, `proof.trunk-every`, `remedy.<role>`, `delete.rules`, `settings.apply`) go in `metasystem.conf`; computer facts (`host.builds`, `host.proofs`, `host.load-max`) go in the `metasystem.conf.local` of the computer's registered owner checkout (the lane's, mechanism 8), where the roster and the ports already live, and other checkouts ignore `host.*`; the test groups stay in the generated `testing.json`, which `proof.cheap` and `proof.full` reach through `test run` and `test groups`; the areas a goal edits stay on its design page. The one bend: `metasystem.conf` says "overrides only, every default compiled", and a proof command has no default outside the template repository, so `settings check` refuses an adopter whose required declaration is missing, and the file's header gains the words "and this project's declarations". Astra (10-06, one round): agree with three changes, folded here and in mechanisms 5 and 8. First, a repository declaration is committed-only: the resolver ignores a local or environment value for it (as the budget keys already do), `settings set` writes it to the committed file, and `settings show` says so, so no seat can run a different "full" proof than the others. Second, the host's owner is not a self-declaration: it is the checkout registered as this computer's landing lane (`landing set`, the lane's record and lock), and where no lane is registered the owner is registered the same way; two claimants or a missing owner refuse with a message, and host facts published by an owner carry its registration and go stale with it. Third, adoption never passes the template's own declarations off as the adopter's: `system adopt` clears them and leaves the required keys explicitly missing until the adopter supplies them, which `settings check` then reports.
- Finding 7 of the plan, ruled by Wido on 10-06: no context or compaction cap of any kind, vendor defaults govern; a headless session ends at the unit boundary only; measurement on the board. Mechanism 7 is written to that ruling. The plan's finding 7 explains the relation: no cap returns (no absolute number, no truncation, the person's session untouched); a headless session ends cleanly at a unit boundary with its handoff, and the context fraction and hours are safety nets in the runtime's own terms. The week's numbers: 75 of 104 seat sessions died with an error, contexts to 967K with one compaction all week, the six longest sessions 191 to 728 million tokens each.
