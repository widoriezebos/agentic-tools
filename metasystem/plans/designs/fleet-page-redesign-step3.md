# The Fleet page, step 3: a steward check reaches the page as one plain line with its act, or not at all

- Kind: design
- Id: 01M4228Q4RQW9FY24R24F4XKCM
- Status: accepted
- Goals: fleet-page-redesign

Accepted 2026-10-04 01:55 by the ui seat for the build: tier 2, so no design round (R-54-m1); m1e passed on the order to build step 3 at 01:29; and sections 1 to 5 carry out Wido's own words of 17:50 and 20:14. The five open questions in section 8 go to Wido with the hand-in. The build follows the four "yes" recommendations, and the fifth stays on the later list.

Refines `fleet-page-redesign.md` (accepted; steps 1 and 2 are on main). It does not restate that page.

Wido, 2026-10-03 17:50, on the health lines in Needs you: "no mere mortal will know what to do with this ... should not end up in front of a human ... understandable and actionable". 20:14, on Doing's "building, round 1 of 20, 2 h 9 min": 20 is the review-round cap, a backstop, not a plan; show "round 1" alone, and the cap only within two of it.

Evidence: `~/metasystem-evidence/agentic-tools-ui/fleet-page-redesign-20261004/mocks/needs-you-step3.html` and `needs-you-step3-1440.png` (before, after on the same facts, and the one line that stays). Desktop only, 1440 px; no phone width was asked for.

What binds. R-121-m0 and R-124-m1u: the smallest thing; a finding is material only when this step does not work or is not safe without it. R-5: a number or a line ships with its act or not at all, which is this step's whole rule. R-11: if it needs explaining, it fails. R-126-m1e: the two commands here are public verbs as a person types them. R-142-m1e and R-143-m1e: nothing in this step overrides a gate or a check; `system start` and `system check` take no decision away from anyone, so no item here needs an impact line (R-129-ui: `system start` on a running steward is a success, not a refusal). The accepted page's FR-01 and FR-04 are amended in section 4, with the reason. `memory/rulings.md` holds no other ruling on health lines or on rounds; the goal's human decisions are Wido's "yes" of 2026-10-03 (step 1 as drawn, 7878 restarted) and the two quotes above, relayed by m1e.

## 1. The rule

A steward check reaches the Fleet page only when the person has an act for it on this page; the item is one plain line (what is wrong, in words a non-engineer reads) and that act, a button that exists today or one command. Today one check qualifies: the steward itself is not running, and the act is `metasystem system start`; the two record-level lines stay (the health record could not be read; the steward stopped writing it), because without them the page cannot vouch for this computer at all. Every other check, failing or undecided, is the steward's own business or an engineer's and is not shown here; `metasystem system check` lists every check with its reason and its remedy, so nothing is lost.

## 2. Every role in `health.go`, and what the page does with it

Role names appear in this table only; none is on screen. The five lines Wido saw are marked (corpus 1 to 5). "Why" names what deals with it instead of the person.

| Role | Its failure means | The page shows | The act | Why |
|---|---|---|---|---|
| steward-runner | the steward, the program that keeps this computer's seats going, is gone or stuck | "This computer's steward is not running." Undecided: "This computer's steward could not be checked." | `metasystem system start`; undecided: `metasystem system check` | the one check whose fix is a person's command; already an item today |
| supervision-owner | the steward's supervisor process is gone | not shown | | the steward's own part; when the steward is gone the line above says so. I could not tell whether `system start` repairs a lost supervisor while the steward lives, so the safe default is `system check` |
| repo-watcher | the steward's watcher stopped or fell behind | not shown | | the steward restarts it itself (`tick.go`, requestWatcherRepair) |
| census-freshness | the count of running work is stale | not shown | | repaired with the watcher |
| narrator-freshness | the digest writer is behind | not shown | | repaired with the steward |
| retro-debt (corpus 1) | a lessons review is owed after a concluded arc | not shown | | a seat runs the retro and records the receipt; `metasystem receipt status` says it |
| session-main | the seat's main agent session is gone | not shown | | the steward starts sessions |
| hook-freshness | the agent hooks have not reported a turn | not shown | | the next agent turn records one |
| stop-hook-duration (corpus 2) | the last Stop hook was slow | not shown | | an engineering cost under its own goal |
| context-budget (corpus 3) | a seat's session holds more context than allowed | not shown | | the seat hands its session off; nothing for a person |
| ledger-attention (corpus 4) | the shared goal ledger moved and this seat has not looked yet | not shown | | the seat looks on its next turn |
| seat-presence | this computer has not told the others it is here | not shown | | the next tick republishes; the effect already shows in the others' Seen column |
| claimed-goal-appetite | a claimed goal has no budget, a malformed one, or is past it | not shown | | a budget is a person's word (R-13) but the line carries no title and the goal room carries the decision; the steward's typed causes are the later item (section 7) |
| stop-capability-epoch | a goal's claim and its live lease disagree about the owner | not shown | | an engineer's consistency check |
| claimed-goal-delivery | a claimed goal's job failed and nothing recovered | not shown | | the returns and the red proof items already carry what a person acts on |
| trunk-red (corpus 5) | the deep validation is overdue or red, or main is red with no owner | not shown | | the lane runs the validation when it runs, and a paused lane is already an item with Resume; a red main is claimed by a seat |
| spend-fence | spending passed a ceiling | not shown | | raising a ceiling is Wido's word (R-60-m1), but no public verb does it and the steward's alert carries it; later item |
| governed-obligations | a standing obligation lacks its evidence or tripped its breaker | not shown | | an engineer's |
| nonterminal-jobs | job records say running but the process is gone | not shown | | a stuck seat already shows with Stop; `work stop` is a seat's verb |
| proof-attempts | a proof's launcher died | not shown | | the job reaper reconciles it |
| proof-admission | a heavy proof's lease is dead | not shown | | the next admission reclaims it |
| capability-snapshots | a runtime's capability snapshot is stale | not shown | | the next delegated job refreshes it; its own remedy says "nothing needs doing now" |
| disk | the disk pass asks for attention | not shown | | the engine owns disk lifetimes; `metasystem disk show` |

## 3. Defaults

A role this table does not know (one added later) is not shown: the code names only what is shown, so a new check is hidden by construction and reaches the page only when someone writes its line and its act. A check that failed with no reason changes nothing: the shown line's words are fixed and never quote the reason, and a hidden role stays hidden. The Details disclosure of raw reasons goes from every health item; the unreadable-record item keeps the reader's one message behind Details, because that message is the only fact anyone has.

## 4. FR-01 and the verdict

Decision: a hidden role does not count toward the verdict, in any state. "All good on this computer" is defined in its own help as "whether anything on this computer needs you"; a hidden check needs nobody on this page, so counting it would say "1 thing needs you" over an empty list, which is worse than either showing or hiding. FR-01's protection stays where it matters: the page is never green while it cannot know this computer's state. Three items keep that: the record could not be read (`system check`), the steward stopped writing it (`system start`), and the steward's own check is undecided (`system check`, new, in place of today's "last health check could not decide" over any role). The verdict's help gains one sentence: "The steward's own checks are not counted here; metasystem system check lists them." FR-04's `system check` item over failing checks goes with its Details.

## 5. Doing's round

"round N" alone; "round N of M" only when N >= M - 2 (so "round 18 of 20", "19 of 20", "20 of 20"). One exported helper in `fleet.ts` carries the words, and every writer of a round uses it: the Doing column from the job records (`workingWords`), the Doing column from the board's card (`cardWords`, review and revise), the opened row's "This job" line (`WorkingBlock`), and the rail's phase sentence (`phaseWords`). A build whose roundLimit is null is unchanged: "round N" alone. Today's first-round rule is untouched: a build's round 1 is said when a cap travels with it and not when none does; that oddity is a question of its own (section 7), not this step's. The chain and the older-engine running words never wrote a cap and do not change.

## 6. Mocks

`needs-you-step3.html` (1440 px, three slices: A, before, the five lines open behind Details and "round 1 of 20"; B, after, the same facts, Needs you absent, "round 1" and "round 18 of 20"; C, after, the steward not running with `system start`) and `needs-you-step3-1440.png`, made with Playwright 1.63 from a scratch folder.

## 7. The build

Files: `panel.ts` (healthNeeds keeps the three record-level items and the steward item, adds the undecided-steward item, drops the failed-checks and could-not-decide items, `reasonsOf` and the Details; `roundWords` and `cardWords` call the helper), `fleet.ts` (the round helper; `phaseWords` uses it), `FleetPane.tsx` (`WorkingBlock` uses it), `help/terms.ts` (the verdict sentence; the Needs you text names "this computer's steward not running" instead of "its own health"), the tests beside each, and the bundle in the same commit.

The mapping lives in the client, `panel.ts`, because it is a deletion there: the payload keeps every role unchanged, no Go message is added or reworded and no JSON moves, so `TestAuditMessagesAPersonReads`, `TestAuditMessagesTraced` and `TestAuditOutputLayoutJSONUnchanged` do not run for this step, and the words are tested where they are written. A server filter would hide the roles from the payload too and gain nothing.

Tests, failing first. `panel.test.ts`: each of the five corpus lines, placed as its role's reason on an otherwise healthy record with the steward alive, yields no item and the verdict "All good on this computer"; a dead steward-runner yields the one item with `system start` and no details; an undecided steward-runner yields "could not be checked" with `system check`; a dead role whose name is not in `health.go` yields nothing; a dead role with an empty reason yields nothing; the unreadable and stale record tests stand; the "failed health check ... Details" and "could not decide" tests are rewritten to the rule. Rounds, in `panel.test.ts`, `fleet.test.ts` and the opened row's test: N = 1 of 20 says "round 1"; 17 of 20 says "round 17"; 18 of 20 says "round 18 of 20"; 20 of 20 says "round 20 of 20"; a null limit says "round N"; the board card "reviewing, round 3 of 20" becomes "reviewing, round 3".

Later, when it hurts: budget and spend crossings as items with Open goal, from the typed causes the steward already records (RemedyFacts), once such a line is seen on the page; the steward's own alert (`alert_episode.go`) sends the same raw lines to the channel, the engine's lane, where this table could serve as the plain words; a hidden check whose automatic repair ended (FailureEscalation, not in the payload) shown as "a check of this computer's steward has failed for N h" with `system check`, when one stays dead for hours with nobody told; whether a build's first round is said at all; Box's "attempt 3 of 10", the same cap-as-plan reading, not asked.

## 8. Open questions for Wido

1. Of 23 steward checks, one stays on the page (the steward not running) with the two record-level lines; the other 22, the five you saw among them, leave the page and are read at `metasystem system check`. Recommended: yes.
2. This amends FR-01 of the accepted page: an undecided check of a hidden role no longer holds the verdict; an unreadable record, a stale record and an undecided steward check still do. Recommended: yes.
3. Details with the raw reasons goes from the health items; only the unreadable record keeps the reader's message. Recommended: yes.
4. The round cap from two below (18, 19, 20 of 20), everywhere a round is written, and "round N" alone before that. Recommended: yes.
5. Budget and spend crossings: not on the page until one is seen there. Recommended: later.
