# Design brief: coordinator-context-stays-under-budget, amendment 8d, revision 3

Template: scripts/agents/templates/design-brief.md. Draft by a reader for seat m1e, 2026-09-15, written for option 2 of ctx8d-crit-r2-summary.md (same directory); the seat's round-3 rulings replace anything below that they contradict. Paths are relative to the metasystem module of /Users/wido/LocalStorage/GitHub/agentic-tools-m1e unless absolute.

## Revision

Revision: revision 3 of amendment 8d of plans/coordinator-context-stays-under-budget-design.md (revision 3 plus 8c.15), against the program page plans/seats-spend-tokens-in-bounded-sessions-design.md revision 3 (495ddd0f2). Kind: design.

Reason: the Codex gpt-5.6-sol critique of revision 2 (code at b70fe4f1a) returned rework with 13 material findings, CC8D-201 to 213 (201 to 205 critical); a reader confirmed each on origin/main b70fe4f1a and added RN-1. Six findings, four of them critical, fall on the automatic successor, and four of those six were introduced by revision 2's own mechanism; the cap group's findings are narrow. Revision 3 therefore narrows amendment 8d to the cap group and severs the automatic successor into its own design.

Scope of revision 3:

- Keep, fold and restate: D1 (U3a-1), D3 (U3e-1, U3e-2), D4 (U3b-1, U3b-2), with their facts, failure-mode rows, witness rows and unit rows. Re-split any unit a fold pushes over 300 changed lines.
- Sever without rewriting: D2 (U3a-2), D5 to D9 (U3c-0, U3c-5, U3c-3, U3c-2, U3c-1a, U3c-1b, U3c-4a, U3c-4b), D10, and open questions 2 and 3, with their rows. Revision 3 writes one section, "Severed to the successor design", that (a) lists each severed rule by its revision-2 id in one line, (b) routes CC8D-201, 202, 204, 205, 207, 208, 211b and 213b to it, and (c) states the interface the successor design consumes from the cap group: the schema-2 state fields, the record-only path of `context handoff`, and the tool-gate table with the way a later unit adds rows or switches the ceiling column. The severed rules are not re-argued here.
- The goal's DONE text is unchanged. Revision 3 states that the goal cannot conclude before the successor design's units land and that the proof week starts only after they do.

This round ends at a written revision 3, the goal's last design round; the critique round 3 and the builds are later rounds; a NOT LAND or critique verdict never lands.

## Context pack

Read this pack first. Open another file only to check one of the cited lines below; do not widen the read beyond that check.

Diagnosis or prior revision:

The complete revision 2 is a member of this pack by path, because inlining its 56.6 KB would break this brief's 60 KB ceiling. Read it whole, once: /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/9b7e334d-919d-47e9-9555-a9f2ed557072/scratchpad/coordinator-context-8d-design-r2.md (221 lines, sha256 cb5021934f142fc50a24098f69b26c00ae819191afb696ad0ef0deb28fb5fbe7). Its line numbers name origin/main a24ecff03; the excerpts below name b70fe4f1a, and revision 3 cites b70fe4f1a. Section map of revision 2: 0 findings folded :8-28; 1 facts :30-40; 2 shape :42-44; D1 :48-53; D2 :55-61; D3 :63-69; D4 :71-77; D5 :79-87; D6 :89-93; D7 :95-101; D8 :103-109; D9 :111-116; D10 :118-120; 4 failure modes :122-140; 5 witnesses :142-184; 6 units :186-205; 7 questions :207-213; 8 not checked :215-217.

Observed facts (reader, 2026-09-15; not repository lines, so not byte-exact excerpts):

- Transcript /Users/wido/.claude/projects/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/9292cf37-9f88-4700-a7f9-1fe2e45bf000.jsonl, main-thread calls deduplicated by `message.id`, prompt tokens = input + cache read + cache creation: the first call is 40,504 (line 28), the sixteenth 121,869 (line 214); mean growth over those 16 calls is 5,424; the largest single increment is 14,454 (from line 51 to line 71). The session later reached 69 calls and 207,575, with the same largest increment and a mean growth of 2,457. One session is thin evidence; the proof week measures.
- The token diagnosis (records/misc/token-diagnosis-2026-09-15.md) holds no boot figure: neither "boot" nor "110K" occurs in it. Its call-level row for that session is excerpt 13.
- Task records in the same transcript. Agent launch: an assistant `tool_use` named `Agent` with id `toolu_<x>` (line 126), then at once a user `tool_result` for `toolu_<x>` whose text reports the asynchronous launch and the agent id (line 127). Background Bash: an assistant `tool_use` named `Bash`, then a user `tool_result` whose content is the string `Command running in background with ID: <id>. Output is being written to: <path>` (lines 155-156). Completion: always a top-level record `{"type":"queue-operation","operation":"enqueue","content":"<task-notification>\n<task-id><id></task-id>\n<tool-use-id>toolu_<x></tool-use-id>\n<output-file><path></output-file>\n<status>completed</status>..."}` (eight times: lines 220, 270, 346, 379, 430, 472, 521, 676). Delivery then takes one of two forms: a `queue-operation` `dequeue` followed by a `user` record holding the same notification text (lines 222, 278, 529, 682), or a `queue-operation` `remove` followed by a record of type `attachment` whose attachment type is `queued_command` and which holds that text (lines 351, 383, 446, 477). `enqueue`, `dequeue` and `remove` records without a task id also occur (for example lines 286, 301, 555, 556) and are not task events. Only the status `completed` occurs in this transcript.

Critique findings answered in revision 3 (proposed as accepted; the seat rules):

1. CC8D-203 (critical). D1.4 and its witness `TestTriggerPlusHeadroomStaysUnderTheProofLine` treat 4K, an observed average (excerpt 12), as a per-call maximum, and they start from a "boot near 110K" that the diagnosis does not contain. The checked transcript shows a first call of 40,504 and a largest increment of 14,454, so five admitted calls at that step, starting from 135K, cross 200K, and no frequency argument supports the p95. Open question 1 rests on that arithmetic and offers restating p95 as 200K, which SR4 withdrew and R-115-m1e item 6 bars. Asked: withdraw question 1. Choose the margin by a construction on a stated maximum per-call increment (a named proof constant with its source), counting the sampled call, the note and the verb, plus a frequency statement for the p95. State how many allowlisted calls past the trigger stay under 200K at that step, and name the residual (a seat that runs more never-denied calls than that) as the proof's finding, never as a denial of a landing or a wait. If the data cannot support a construction at the DONE's numbers, return the conflict to Wido without choosing weaker numbers. D1.1's refusal line follows the new constant. Disclose the working room (from the first-call figure to the trigger) as a cost, not a question. [Reader arithmetic, not binding: T + 3 x 14,454 <= 150,000 gives T <= 106,638; at T near 105K, six calls past the trigger stay under 200K.]
2. CC8D-206 (high). D4.3 detects a completion as "a user record holding `<task-notification>`" and ends a flight on "any `<status>` word". Real completions are `queue-operation` records delivered either as a user record or as a `queued_command` attachment (observed facts above), so a user-record matcher misses half of them, and a future status word that is not terminal would end a flight. Asked: specify the record types, the text grammars (Agent result, background Bash result, notification tags), the identity joins (task id, tool-use id), and an exact terminal status set, with every other word non-terminal. Only `completed` is observed: name the set and say how an unobserved word is treated. Count a task once across enqueue, dequeue, remove and delivery. Fixtures are sanitized from the observed shapes and cover both delivery forms, a repeated notification for one id, a resume, and a non-terminal status. [If the seat so rules, these facts supersede SR5's "user-turn" wording.]
3. CC8D-209 (high). D3.2 starts the 100 ms clock at verb entry, after shell start, engine discovery and exec (excerpts 6, 7), while SR6 binds the whole invocation; and it relies on harness timeout behaviour that nothing here proves (excerpt 8). Asked, narrowed: the deadline counts from shell entry. Name one mechanism, for example the shell's start time passed to the verb, or the parent shell's birth read through the identity prober. Focused witnesses still use an injected clock. Command classification runs before any transcript read, so an allowlisted call is allowed without a sample and never depends on the deadline. Host behaviour on a hook timeout is dated integration evidence, never the only guard. No wall-clock focused witness (Wido 2026-09-12).
4. CC8D-210 (high). Landed prefixes of the unit order can break continuation. With U3e landed and no launching handoff (none exists until U3c-1b), D3.4 denies delegate launches at the ceiling, which leaves no lawful way to continue (R-114-m1e item 6). U2c is ordered "after U3c-4a", against P7's "after U3c" (excerpt 10). Asked: every landed prefix of the cap group leaves a lawful continuation path. Until the successor design's launch lands, the ceiling column allows what the trigger column allows (delegate launches and the Agent tool at every size), and the successor design owns that switch and its witness. U2c follows every successor unit. Same class, found by the reader: U3b-1 makes the handoff end in-flight waits (D4.1), but until a successor exists nobody registers them again, whereas today the record refuses while a wait is in flight (excerpt 9). Keep that refusal on the record-only path. Ending and recording waits is callable only from the launching path the successor design adds, with a witness that the record-only path still refuses.
5. RN-1 (reader, critical). The hook's ordinary dispatcher exits 2 for any event other than receipt, stop and end (excerpt 4), and D3.1's PreToolUse command has no fallback, unlike Stop's (excerpt 5). D3.1 ships the entry in U3e-1 while the `claude tool` case lands in U3e-2. In that interval every tool call would exit 2, which in Claude Code blocks a PreToolUse call (harness documentation, not re-read by the reader), so landings and waits would be denied against R-114-m1e item 5. Asked: the shipped entry and the `claude tool` case land in one unit; the command carries an allow fallback; the tool branch runs before the dispatcher's `exit 2` and exits 2 on no path. A focused witness execs the shipped command with a missing engine cache, a malformed runtime and a failing engine, and asserts an allow or no decision, never exit 2.
6. CC8D-212 (medium). D4.4 says internal/usage already resolves the seat's memory directory; it resolves only transcripts (excerpts 2, 3). Asked: one memory-directory resolver whose inputs are home plus the toplevel or installation slug, as the transcript resolver uses, and never a caller-supplied transcript path. Its symlink rule is `os.Lstat` and `pathWithin`, as in excerpt 3. It goes in U3b-2's file list and allocation. Path tests accept the project memory directory and refuse a transcript, slug or symlink that escapes it.
7. CC8D-211a (medium, part). D1.1 refuses a trigger above the proof line, while R-115-m1e item 3 raises "the ceiling ... by one configuration line" when warm-up plus dead time exceed a stated share. With the margin fixed, that raise pushes the trigger past the construction. Asked: state the rule in D1. The refusal stands; pulling item 3's valve past the proof line changes the DONE's numbers, so it goes to Wido with the proof week's evidence; no key or code path lowers the line. It is not an open question now.
8. CC8D-213a (medium, part). D2.1's focused witness only parses flags; whether the hook passes `--transcript`, `--runtime` and `--runtime-session` is observed only by a bed leg. Asked: a focused script-to-command witness, in the unit that edits the hook's Stop arguments, that execs the hook with a stub engine recording its argv (as D3.2's hook witness does). Every rule kept in revision 3 has a non-bed mutation witness row, and every surface in a unit cites a finding or the DONE.

Findings severed to the successor design (not folded here; the severance section names each by id with the claim below):

- CC8D-201 (critical). The delegate-class successor cannot run D7.2's fresh registrations: the wait verb admits only class MAIN and the live holder (cmd/metasystem/wait_verb.go:149-152), and U1 registers fresh only for a successor main. It needs a delegate-class registration with an authority rule and a race witness, or SR2's lease question put to Wido.
- CC8D-202 (critical). A fence at `runSyncOnly` misses the 19 handlers that call `syncReq` or `syncReqClassified` directly (cmd/metasystem/goalsync_mutations.go); their common edge is `syncReqClassifiedWithTerminalGrade` (:432). It needs every mutating edge enumerated, with one mutation per edge.
- CC8D-204 (critical). A blocked headless Stop is not a durable owner: every job runs under a finite `--max-turns` (default 150; internal/adapter/claude.go:333-338), and block behaviour has no dated host evidence. It needs an owner that survives provider exit and the turn limit; the reaper at job close is the candidate.
- CC8D-205 (critical). D9.1 names claim fields that do not exist (`ClaimRecord` has no main id and no epoch; internal/goal/file.go:280-303), omits the fields to preserve, and gives no crash order between the ledger and the intent's chain.
- CC8D-207 (high). resume.json is written exclusively (D7.3), yet D8.4 relaunches under the same nonce and S6 makes resume the first act, so the evidence must be attempt-scoped.
- CC8D-208 (high). The successor's claim leg and `goal next` are not allowed at the trigger; the startup sequence needs to be allowed and its room reserved. The successor design adds these rows to the cap group's tool-gate table.
- CC8D-211b (medium, part). Question 2's alternative has no unit or witness, and question 3's "no new source" is false (job records carry no receipt). Both move with dead time and the console line.
- CC8D-213b (medium, part). D6.3, D7.5 and D8.5 have no witness rows. The weekly `context report` handoff list has no owner; dead time is read by U5a and U5b of spend-fence-reports-tokens-per-model-and-cause (R-115-m1e item 3).

Not answered: CC8D-214 (non-material; agreed).

Cited code excerpts (copied byte-exact from origin/main b70fe4f1a; paths relative to the metasystem module):

1. `docs/orchestration.md:370-370` (CC8D-212; seat rule S7)

```text
| S7 handoff keeps lessons | A handoff is recorded only after the seat's memory note holds this session's lessons, its reasoning in flight, and every in-flight delegate's output path. | The note's modification time precedes the handoff record. |
```

2. `internal/usage/calls.go:64-72` (CC8D-212, CC8D-209)

```text
type ReadOptions struct {
	Capability   Capability
	Transcript   string
	Home         string
	Toplevel     string
	Installation string
	Now          time.Time
	NonBlocking  bool
}
```

3. `internal/usage/calls_claude.go:14-38` (CC8D-212)

```text
func claudeTranscript(opts ReadOptions, session string) (path string, reason string) {
	if opts.Transcript != "" {
		return opts.Transcript, ""
	}
	home, reason := callHome(opts)
	if reason != "" {
		return "", reason
	}
	var candidates []string
	for _, cwd := range []string{opts.Toplevel, opts.Installation} {
		if cwd == "" {
			continue
		}
		directory := filepath.Join(home, ".claude", "projects", claudeSlug(cwd))
		candidate := filepath.Join(directory, session+".jsonl")
		candidates = append(candidates, candidate)
		if !pathWithin(directory, candidate) {
			continue
		}
		info, err := os.Lstat(candidate)
		if err == nil && info.Mode().IsRegular() {
			return candidate, ""
		}
	}
	return "", fmt.Sprintf("unknown (no transcript at %s)", strings.Join(candidates, " or "))
```

4. `scripts/agents/supervision-hook.sh:1061-1068` (RN-1)

```text
if [[ "$event" == start ]]; then
  start_main
  start_finish notice unexpected-termination
fi
# SessionStart cannot pass this dispatcher.

[[ "$runtime" =~ ^[a-z][a-z0-9-]{0,31}$ ]] || exit 2
case "$event" in receipt|stop|end) ;; *) exit 2 ;; esac
```

5. `scripts/enforcement/claude-code-hooks.json:16-26` (RN-1; the Stop command's fallback)

```text
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "(bash scripts/agents/supervision-hook.sh claude stop) || printf '%s\\n' '{\"systemMessage\":\"Task unknown; Stop allowed; needs supervision repair; hook-bootstrap-failed. The steward must restore supervision. Status unavailable.\"}'",
            "timeout": 60
          }
        ]
      }
    ],
```

6. `scripts/agents/supervision-hook.sh:1444-1449` (CC8D-209)

```text
stop_started_epoch=${METASYSTEM_STOP_DEADLINE_STARTED:-}
if [[ "$stop_started_epoch" =~ ^[0-9]+$ ]]; then
  stop_started_epoch=$((10#$stop_started_epoch))
else
  stop_started_epoch=$(date -u +%s)
fi
```

7. `scripts/agents/supervision-hook.sh:1581-1589` (CC8D-209)

```text
local_delegate_rc=0
local_delegate=$("$ms" lease hook-delegate --root "$repo" --metasystem-root "$world_installation" \
  --caller-pid "$PPID" 2>/dev/null) || local_delegate_rc=$?
if (( local_delegate_rc == 0 )) && [[ "$local_delegate" == *'"delegate":true'* ]]; then
  intentional_hook_skip
elif (( local_delegate_rc != 0 && local_delegate_rc != 3 )); then
  echo "supervision hook refused: local delegate custody evidence was unreadable" >&2
  exit 1
fi
```

8. `docs/design/turn-verdict-delivery-contract.md:43-48` (CC8D-209)

```text
Conformance also requires dated evidence for the actual host version,
effective configuration and instruction bytes, trusted hook firing, total
human transcript, exact report lookup from the seat's command environment,
both block and allowance behavior, and the finite continuation limit. A
source fixture or static declaration proves emission shape only. Missing or
stale observations remain unobserved.
```

9. `internal/steward/handoff_capture.go:414-424` (CC8D-210)

```text
		}
		known := false
		for _, state := range run.WaiterStates {
			if state.Name == waiter.State {
				known = true
				if state.Class == run.WaiterStateInFlight {
					return refusal("HANDOFF_WAIT_IN_FLIGHT", "")
				}
				break
			}
		}
```

10. `plans/seats-spend-tokens-in-bounded-sessions-design.md:141-141` (CC8D-210)

```text
| U2c | stop-gate-sees-harness-tracked-work | the idle bound of two, escalation at two through the `seatIdle` continuation beside the live seat, the block until the running signal, the handed-off marker at that signal | about 100 | after U3c | none beyond P3 |
```

11. `plans/seats-spend-tokens-in-bounded-sessions-design.md:130-130` (CC8D-213, CC8D-210)

```text
Every unit is at most 300 changed lines; a unit marked split is sized by its goal into units under that bound. Every unit that adds a verb names cmd/metasystem/main.go (`families()`, :718-744) in its boundary (SSTB-208, stated once here). Every owning goal's design gives each rule a focused witness a builder runs without a fixture bed or a live session, not only U1 (SSTB-311, stated once here). Order: U6a first, at no build cost; then the cap group U3a, U3e, U3b; then U1, because U3c's fresh registration of every `openWork` row depends on it (SSTB-308); then U3c, which completes the largest saving (main sessions 779.5M to about 85M); then U2a, U2c, U2b; then U4; then U5a, U5b, U5c; then U7; then the proof.
```

12. `plans/seats-spend-tokens-in-bounded-sessions-design.md:44-44` (CC8D-203)

```text
The bound. The trigger is observed at Stop time, so one turn can carry the sample past the trigger before the gate sees it. No tool-use hook is installed and the supervision hook receives none, so growth inside a turn is bounded only by turn length. The number: growth is 4K per call (diagnosis, m1c's 217 calls to 869K). Turns of the causes that survive the cap average 2.8 to 6.1 calls; the longest surviving turn class is the usage-limit continuation at 11.3 calls; the compaction continuation at 41.3 calls does not exist under the cap, because no compaction fires under 200K. Maximum overshoot at 12 calls: 48K, so a Stop sample of at most 198K. Maximum overshoot at the observed worst turn of 42 calls: 168K, so 318K. The 4K is the observed average growth per call, not a maximum, so these are forecast bounds (SSTB-304). What DONE's "under a stated context size" then means: every Stop-time sample is under 200K by construction; every call is under 200K only while turns hold at 12 main-thread calls (rule S2 in P6); the landed proof's maximum rule (internal/steward/contextreport.go:566-569) reports any call over the ceiling and fails the proof. Whether that is enough is open question 4; the recommendation there installs a tool-use hook (U3e) that denies any tool call but the handoff over the trigger, which bounds every call at 150K plus three calls: 162K at the average growth, and 200K by the margin.
```

13. `records/misc/token-diagnosis-2026-09-15.md:265-265` (CC8D-203)

```text
| m1e | main | 9292cf37-9f88- | m1e coordinator (in flight) | 16 | 1.5M | 1.3M | 122k | 12:36-12:45 | 0 |
```

14. `docs/design/design-principles.md:120-125` (unit allocations)

```text
When a design divides implementation into units, allocate at most 300 changed
lines to each unit. Count additions plus deletions across the whole candidate,
including production code, tests, scripts, and documentation. State the
allocation explicitly in the design's implementation map and in every unit
brief as `Changed-line allocation: <number>`. This planning maximum leaves
room below a separately declared enforcement ceiling; it neither sets nor
```

Example page:

Revision 2 is the shape (header, findings folded, facts, shape, decisions, failure modes, rule-to-witness table, build units, open questions, not checked, critique record), narrowed to D1, D3 and D4 and extended by the section "Severed to the successor design". Section 6 rows keep the form `| U3e-2 the tool gate | files (tests included) | 300 | DONE | witness | after | why over P7's share |`, where the number is the unit's `Changed-line allocation` (excerpt 14), and the cap group's total stays near its share of P7.

## Rulings carried

Seat m1e's rulings on round 1 (ctx8d-crit-r1-summary.md), binding for revision 2, carried verbatim; they still bind revision 3 wherever the round-3 rulings do not replace them:

- SR1. Dispositions. All sixteen material findings CC8D-101 to 116 are accepted and folded. CC8D-113 is accepted in the narrowed form: S7's check stays the modification-time order (docs/orchestration.md seat rule S7); r2 adds where the note lives (the seat's memory directory), that the note names every declared in-flight delegate's output path, and strict same-second handling; r2 does not machine-check the content of lessons or reasoning. CC8D-117 is rejected (the header name is right in the scratchpad).
- SR2. Authority while two sessions live (101, 106, 110). r2 adds no second exception to the rule that a live holder never yields (lease/claim.go:117-122). The successor works through paths that exist: claim ownership is machine plus lineage (goal/verbs.go:502-506), and an active continuation already records a handoff in the delegate class (context_verbs.go:223-233). The lease and main-ness move only when the predecessor has ended, and the predecessor ends only after its successor is confirmed started (program page P2). The handed-off session's goal mutations are refused by a fence keyed on member A's RuntimeSession token (lease/classify.go:23-25), which member B1 of registered-wait-matches-the-runtime-session makes the hook pass; name B1 as a dependency. If r2 finds a live-predecessor lease handover unavoidable, it poses that as an open question for Wido with a recommendation and puts the unit that needs it behind his word; every other unit still builds.
- SR3. Unit graph (102, 116, 114, 108). No unit calls a surface before the unit that adds it. Every U3c unit follows all of registered-wait-matches-the-runtime-session (members A landed c4e7d0f31, B1 in build, B2, B3, C open). Every unit states `Changed-line allocation: <number>` (docs/design-principles.md:120-130), its test files, and a focused witness that needs no fixture bed and no live session (program page P7). Aim the total near P7's about 1,320 lines for U3a, U3e, U3b and U3c; if more is needed, say per unit why, and never narrow a DONE to fit. U3a-1 names validate.go and metasystem.conf; U3e edits the shipped hooks file scripts/enforcement/claude-code-hooks.json and what `hooks check` reads, not .claude/settings.json.
- SR4. Proof floor (103). The goal's DONE numbers stand: p95 under 150K and no call over 200K. R-114-m1e item 3 fixes only the ceiling (250K in configuration, trigger at ceiling minus margin); r2 chooses the margin and gate so the DONE numbers hold, with the arithmetic from the diagnosis. No interim build at weaker numbers. Open question 1 is withdrawn; ask Wido only if r2 shows with evidence that the numbers cannot be reached.
- SR5. In-flight delegates (104). Observed by m1e in this Claude Code session on 2026-09-15: an Agent launch returns at once with a tool_result carrying the agent id and output file; completion arrives later as a user-turn <task-notification> with <task-id> equal to the agent id, <tool-use-id>, <output-file> and <status> (completed, and other terminal states); the same task id may notify more than once if the agent is resumed. A background Bash command follows the same shape (task id, status, exit code). A delegate or background command is in flight from its launch result until a terminal notification for its task id. Background Bash tasks count as in-flight work the same way as Agent delegates.
- SR6. The hook (107, 112). The gate never blocks a landing or an in-flight wait (R-114-m1e item 5): at least `metasystem landing`, `goal land-ready`, `metasystem wait`, `job watch` and the landing driver stay allowed at every size; no absent command is named. Open question 2 is dropped unless r2 re-poses it on the corrected grammar. The 100 ms bound covers the whole hook invocation including `lease hook-delegate`, with a byte cap and a deadline in the reader's options; timing witnesses use an injectable clock, never wall time (Wido, 2026-09-12: artificial clocks, never load-fragile tests).
- SR7. Launch and failure lifecycle (105, 109). No path ends a session with no successor started (program page P2). r2 does not rely on "the tick retries": a failed launch leaves a live, restageable intent or an explicit escalation that a named actor acts on, matching revive.go, runner.go and reap.go as cited.
- SR8. Records (111, 115). openWork carries every WaiterTarget field a fresh registration needs (waiter.go:45-53, :133-176). The handoff state gets a schema version bump with a migration or a dual reader (handoff_state.go:22, :139-151).
- SR9. Open questions 3 and 4 stay as posed. 8c.7 (held handoff expiry) stays out of scope and waits on Wido.
- SR10. Work style for the designer: batch independent reads into one request; open no file outside the context pack except to check a cited line.

## Seat rulings, round 3

Seat m1e (coordinator) rules on ctx8d-crit-r2-summary.md, 2026-09-15. Binding for revision 3; SR1 to SR10 stay binding except where amended here.

- SR11. Scope (option 2). Revision 3 covers only the cap group: U3a-1, U3b-1, U3b-2, U3e-1, U3e-2 (about 1,380 changed lines). The successor group (U3a-2, U3c-0 to U3c-4b) leaves this amendment. Revision 3 ends with a short hand-off section for the successor design: the moved findings CC8D-201, 202, 204, 205, 207, 208 and the successor halves of 211 and 213, each with its evidence line, plus the interfaces the cap group exposes to it. The goal's DONE is unchanged, and the goal does not conclude without the successor (R-115-m1e item 6). Where the successor design lives is Wido's question; revision 3 does not decide it.
- SR12. Folds. Accepted into revision 3: CC8D-203 (no 110K boot figure exists; use the diagnosis's measured first call and largest step), CC8D-206, CC8D-209 as narrowed in the summary, CC8D-210, CC8D-212, the cap-group halves of 211 and 213, and RN-1: the hook exits 2 on an unknown event (supervision-hook.sh:1061-1068), so no unit registers PreToolUse in the shipped hooks file before the hook handles that event, and a witness proves an unregistered or failing tool-gate path allows the call.
- SR13. SR5 amended (CC8D-206): a delegate or background command is in flight from its launch result until a terminal completion for its task id, whether that completion arrives as a user record or as a queued_command attachment; every status word the harness can emit is classified, and an unknown word is treated as still in flight and reported.
- SR14. The gate never strands a seat (Wido, 2026-09-15: caps and budgets trigger a handoff or delegate that continues the work, never a stop). Until the successor design's launch unit lands, the tool gate runs in observe mode: it allows every call and records the decision it would have made; turning deny on is one configuration line owned by the successor's launch unit. The deny table and its witnesses are still built and proven now, so no witness is removed. At every size the gate allows the Agent tool, task and wait tools, and the landing and wait commands of SR6, because S1 runs the remaining work as delegates.
- SR15. Open questions. Q1 stays withdrawn (SR4); if revision 3 finds a conflict between R-115-m1e item 3's ceiling raise and the p95, it names the condition under which it fires and poses it to Wido only then. Q2 (dead time) and Q3 (console line) move with the successor.

## Tool-call budget

Maximum delegate tool calls: 25

Stop when this number is reached. List anything the budget did not allow you to check.

## Page-size ceiling

Maximum page size: 450 lines

Cut a draft that exceeds this ceiling. If cutting would make the page incomplete, stop and propose a split instead.

## Fresh session

This revision runs in a new delegate session. Use the prior page and critique only through the context pack above. Never resume a delegate from an earlier revision.

## Rulings to honour, verbatim

- Program rule: no unit of the efficiency program lowers a proof floor, removes a witness or a gate, or narrows a DONE to save tokens, and each unit's read checks it.
- R-115-m1e item 2: "Design questions on coordinator-context-stays-under-budget: U3b's open-work record lists every in-flight Agent delegate (id, what it was asked, output path) or the quiet-point rule adds no delegate running; U3c's successor starts within a stated number of seconds, not a tick, dead time from the handoff record to the successor's first act is recorded, and the console pane shows the worker's status (goal next and the health line)."
- R-115-m1e item 3: "On spend-fence-reports-tokens-per-model-and-cause: U5a adds a warm-up cause class (calls from context resume to the first act on a goal) and dead time per handoff, U5b shows both in health, and when warm-up plus dead time exceed a stated share of the day's saving the ceiling goes up by one configuration line."
- R-114-m1e item 2: "the automatic launcher: the steward continuation, a headless session the steward starts. No machinery may assume or rely on tmux." Item 3: "a context ceiling of 250K in configuration, with the trigger at the ceiling minus the margin." Item 5: "build U3e, a separate tool-use hook under about 100 ms that never blocks a landing or an in-flight wait, so the DONE's stated size holds by construction." Item 6: "Never stop work because of a limit: a cap or budget triggers a handoff or a delegate that continues the work, never a stop and never a wait on Wido."
- Wido 2026-09-14 (fix upstream, not at the check): every rule the design adds names the check or test that fails if the rule is removed, so the builder proves each rule by mutation.
- Wido 2026-09-12: artificial clocks, never load-fragile tests; timing witnesses use an injected clock, never wall time.
- Seat rule S7 is excerpt 1.

## Page artifact and return shape

Write the page to: /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/9b7e334d-919d-47e9-9555-a9f2ed557072/scratchpad/coordinator-context-8d-design-r3.md

Return only these two lines:

```text
/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/9b7e334d-919d-47e9-9555-a9f2ed557072/scratchpad/coordinator-context-8d-design-r3.md
DESIGN: ready (<lines> lines)
```

or:

```text
/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/9b7e334d-919d-47e9-9555-a9f2ed557072/scratchpad/coordinator-context-8d-design-r3.md
DESIGN: blocked (<reason>)
```
