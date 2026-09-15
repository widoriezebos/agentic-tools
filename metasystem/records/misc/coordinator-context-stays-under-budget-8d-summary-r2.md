# Amendment 8d, critique round 2: findings judged against origin/main

Reader for m1e, 2026-09-15, at origin/main b70fe4f1a, the critic's commit. Transcript facts: 9292cf37-9f88-4700-a7f9-1fe2e45bf000.jsonl. Proposals; the seat rules.

Counts, 13 material, all confirmed (206 with a correction): REOPENED 7 (202, 204, 206, 209, 210, 212, 213); NEW-FROM-FOLD 5 (201, 203, 205, 207, 208); mixed 1 (211: Q2 from the fold, Q3 missed in r1). Under option 2: accept into r3 5 (203, 206, 209 narrowed, 210, 212); split 2 (211, 213); accept-moved 6 (201, 202, 204, 205, 207, 208); reject 0. 214 non-material. RN-1: reader finding missed by both rounds.

## a) Findings by root cause

### G1. Successor authority and lifecycle (U3a-2, U3c-*): SR2's delegate-class successor needs a parallel authority path
- 201 critical, NEW-FROM-FOLD. The successor cannot register the recorded waits: `wait` refuses a caller that is not class MAIN and the live holder (wait_verb.go:149-152); U1 registers fresh only for a successor main (registered-wait-matches-the-runtime-session-design.md:158-160). Accept-moved (delegate-class registration, or SR2's lease question).
- 202 critical, REOPENED 110. `runSyncOnly` is not the one assembly point: 19 handlers call `syncReq`/`syncReqClassified` directly (goalsync_mutations.go:121, 207, 262, 736, 783, 944, 1454, 1467, 1552, 1606, 1649, 1723, 1767, 1835, 1881, 1954, 2011, 2129, 2223); the common edge is `syncReqClassifiedWithTerminalGrade` (:432). The r1 reader's range :2260-2289 carried the error. Accept-moved.
- 204 critical, REOPENED 109. A blocked headless Stop is not durable: every job gets a finite `--max-turns`, default 150 (adapter/claude.go:333-338, :407); the host contract wants dated evidence for block behaviour and the continuation limit (turn-verdict-delivery-contract.md:43-48); the one live `claude -p` fixture is an allow (supervision-fixtures.sh:824-837). Accept-moved: the reaper (D8.4's owner at job close) owns every continuation exit with an unlaunched handoff.
- 205 critical, NEW-FROM-FOLD. D9.1 rewrites a "main id" and keeps an "epoch"; `ClaimRecord` has neither (goal/file.go:280-303), and D9.1 is silent on the other fields; ledger and intent (intervene.go:26-60) are two stores with no crash order. Accept-moved.
- 207 high, NEW-FROM-FOLD. resume.json is written once, exclusively (design :99), yet D8.4 relaunches under the same nonce (:108) and S6 makes resume the first act (orchestration.md:369). Accept-moved (attempt-scoped record).
- 208 high, NEW-FROM-FOLD. D3.3 does not allow `goal claim --continue-handoff` or `goal next`, the successor's second and third acts (design :67, :101). Accept-moved (the satellite adds rows to U3e-2's table).

### G2. Proof arithmetic (U3a-1)
- 203 critical, NEW-FROM-FOLD. The diagnosis has no boot figure (no "boot", no "110K"). Its only call-level m1e main row is 9292cf37: 16 calls, 122k (token-diagnosis-2026-09-15.md:265). In that transcript the first call is 40,504, the sixteenth 121,869, mean growth 5,424, largest increment 14,454 (lines 51 to 71). Now 69 calls, 207,575, same largest step. P1 calls 4K an average (seats-spend page :44). Accept: construct on a stated maximum increment; withdraw Q1; offer no restated p95 (SR4; R-115-m1e item 6).

### G3. Records against real shapes (U3b-2)
- 206 high, REOPENED 104, with a correction. Every completion is a top-level `queue-operation` `enqueue` whose content holds `<task-notification>` (8: lines 220, 270, 346, 379, 430, 472, 521, 676). Delivery is either `dequeue` then a `user` record with the same text (222, 278, 529, 682; the critic's "not user records" is wrong here), or `remove` then an `attachment` of type `queued_command` (351, 383, 446, 477), which a user-record matcher misses. Only `completed` seen. r2 followed SR5's "user-turn" wording; SR5 needs amending. Accept.
- 212 medium, REOPENED 113. internal/usage resolves transcripts only (calls_claude.go:14-38); `git grep -i memory` hits one comment (codex.go:8). Accept: a U3b-2 resolver on the transcript resolver's slug, `pathWithin`, `Lstat` rules.

### G4. Tool gate and landed prefixes (U3e-1, U3e-2, U3b-1, order)
- 209 high, REOPENED 112. The hook runs engine subprocesses before its work (supervision-hook.sh:1461-1478, :1581-1589) and already carries a start epoch for Stop (:1444-1449); r2 starts the clock in the verb. Accept narrowed: deadline from shell entry; classify before sampling so an allowlisted call never depends on it; host timeout behaviour as dated integration evidence. Reject a wall-clock focused witness (Wido 2026-09-12).
- 210 high, REOPENED 102. U2c "after U3c-4a" (design :205) against P7 "after U3c" (program page :141); D3.4 denies delegate launches at the ceiling while handoff only records until U3c-1b. Accept. Same class, reader-found: U3b-1 ends waits before any successor can re-register them; today the record refuses (handoff_capture.go:420).
- RN-1, reader, critical, NEW-MISSED in both rounds. The dispatcher exits 2 for any event but receipt, stop, end (supervision-hook.sh:1067-1068). D3.1 ships PreToolUse in U3e-1, the `claude tool` case lands in U3e-2, and the command has no fallback like Stop's (claude-code-hooks.json:16-26). Exit 2 from PreToolUse blocks the call (harness docs), so every call, landings included, would be denied (R-114-m1e item 5). Accept: one unit ships both, an allow fallback, no exit 2.
- 213 medium, REOPENED 116. Section 5 has no rows for D6.3, D7.5, D8.5; D2.1's witness parses flags only. Split: D2.1's script witness into r3; D6.3, D7.5, D8.5 moved; the weekly `context report` list moved to U5a/U5b (R-115-m1e item 3).
- 211 medium, mixed. Q2's one-line ceiling raise is refused by D1.1; job records have no receipt field (jobrecord.go, no match). Split: the valve-versus-D1.1 rule into r3; Q2's alternative and Q3 moved with U3c-3, U3c-4a.

## b) Convergence

The count fell (16 to 13) but splits by area.
- Settling, the cap group (U3a-1, U3b-1, U3b-2, U3e-1, U3e-2). r1 had nine findings here with two criticals (103, 104). r2 has 203, 206, 209, 212, the 210 prefix edges, 213's D2.1 row and RN-1. Each is a number, a grammar, a start time, a resolver, an order edge or a witness row; none adds an actor. U3e-1's reader half builds from r2 as written; its hooks-file half moves to U3e-2 (RN-1); the rest need one narrow fold.
- Churning, the succession group (U3a-2, U3c-0, U3c-5, U3c-3, U3c-2, U3c-1a, U3c-1b, U3c-4a, U3c-4b). r1 had six findings with one critical (101). r2 has six with four criticals, and four of the six come from r2's own mechanism: the delegate-class successor, the claim leg and chain, the relaunch under one nonce, and the first-act order. That is the skill's "machinery grown under critique pressure" pattern; its satellite rule applies (findings cluster in a separable region).

## c) Options for round 3

1. Fold everything into r3. Built: all 14 units, only if critique round 3 finds nothing severe. Waits: everything until then. Risk: G1 yields new criticals per fold and the skill forbids a fourth round ("whenever a severe or unproven finding remains, stop with the design waiting on the human"), so likely nothing builds. R-115-m1e item 6 holds.
2. Narrow 8d to the cap group; sever the succession group into a satellite with its own design loop. Built after r3 and critique 3: U3a-1, U3b-1, U3b-2, U3e-1, U3e-2 (about 1,380 lines). Waits: about 2,360 lines, blocked behind U1 B2, B3, C anyway, so no calendar time is lost. R-115-m1e item 6 holds if the goal's DONE text is unchanged and the goal cannot conclude before the satellite lands; the tool gate is new, so staging its ceiling column removes no gate. It touches R-114-m1e item 6 ("never a wait on Wido"), since the satellite's review budget is his by precedent (R-113-m1e decision 1: "three goals open, each with its own design and review rounds").
3. Escalate SR2's lease question now: may a confirmed-idle predecessor yield the lease to its confirmed successor ("A live holder that is a genuinely different process never loses the lease", lease/claim.go:117-118)? It dissolves 201 and part of 202, not 204, 205, 207; alone it builds nothing beyond option 2. R-115-m1e item 6 holds.

Recommendation: option 2 plus one question to Wido (below). The settled part lands (per-call bound, complete record, S7 checked); the churning part gets its own loop while U1 builds.

## d) Open questions for Wido as posed in r2
- Q1 (working room): posed on false arithmetic. The "boot near 110K" is not in the diagnosis; 9292cf37's first call is 40.5K, so the room to a 135K trigger is about 94.5K, not 25K. With the maximum step instead of the average: T + 3 x 14,454 <= 150K gives T <= 106K, about 65K of room, and six calls past the trigger stay under 200K. A disclosed cost; successor boot unmeasured. Withdraw Q1 (SR4).
- Q2 (dead time): its recommendation contradicts D1.1, and the alternative has no unit (211). The real conflict: R-115-m1e item 3 ("the ceiling goes up by one configuration line") raises the trigger past the p95 construction. Recommend: no question now; the seat carries it to Wido if the valve's condition fires.
- Q3 (console line): a fair preference question, but "no new source" is false (211). It moves with U3c-4a.

## Question for Wido (option 2)
Sever the automatic successor (U3a-2, U3c-0 to U3c-4b, findings 201, 202, 204, 205, 207, 208, 211b, 213b) into a new goal with its own design and three review rounds, which coordinator-context-stays-under-budget depends on, with the parent DONE unchanged? Alternative: amendment 8e with rounds over the tier box (R-111-m1e precedent). Recommended: the new goal. Touches R-115-m1e item 6 (no DONE narrowed) and R-114-m1e item 6 (its revision 1 is written meanwhile; only its critique waits).

## Not checked (budget)
Harness semantics of PreToolUse exit 2 and hook timeouts; status words other than `completed`; a successor's boot size; the 19 handlers' mutation classes; whether a continuation's `goal claim` classifies today.
