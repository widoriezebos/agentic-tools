# The fleet panel, step 2: a stuck seat shows, a red proof says why, and the buttons act as you

- Kind: design
- Id: 01M401A1E933TWF0E87J2TZ65R
- Status: accepted
- Goals: fleet-panel-ux

Critique: Codex Astra (tier 2, run directly), closed after round 2, the failsafe (material per round 2, 0; round 1 found two in 2b, both folded, and none in 2a; round 2 confirmed all five round-1 ids closed). Slice 2b also waits on Wido's ruling (open question 1).

Revision 2: Astra's round 1 folded (S2-01 and S2-02 in 2b; S2-03 to S2-05 corrected the text); the table at the foot. Open question 2 of revision 1 is answered by m1e.

Step 2 of `fleet-panel-ux.md` (accepted, Astra-closed after three rounds). Step 1 is on main (bb738c1f6, 8136f0421). This page slices step 2 in two. Slice 2a is reads and needs no new authority. Slice 2b is the three buttons and the authority bridge. Each has value on its own. Build 2a first.

What binds this page. R-121-m0 and R-124-m1u: the smallest thing that works first; a finding is material only when the slice does not work or is not safe without it. The accepted design's folds: R1-01 (every action takes the signed-in path Land now takes), R1-02 (Stop is `machine stop NAME`), R1-04 (no invented facts), R2-1 (Pause, Resume and Stop carry the session; Stop only for machines on this computer), R2-9 (no log link without an endpoint). Wido's accepted risk on step 1 (goal history, 2026-10-03 03:43: "stuck seats in Needs you with Stop in step 2"). R-125-m1u and R-128-ui: a browser session is admitted per verb, and "nothing else that asks a terminal-grade proof admits a session", so 2b asks one more row (question 1). R-129-ui: a repeat is success. Wido 2026-09-29, "the browser is my terminal if I am logged in", is recorded as R-125's reading in `rulings-are-project-wide-and-current.md`, still a draft. Lane ownership is settled: m1e has no objection to the ui lane building both `internal/landing/plain` pieces of 2a (the red reason, `ProofLog`); m1e lands them, as it landed step 1's `status.go`.

## Slice 2a: the panel sees a stuck seat and says why a proof is red

### 2a.1 A stuck seat is in Needs you

The fact exists. `/api/board` carries every card of this computer's seats. A card the board does not believe comes with `unknown`, the board's own reason (`internal/board/board.go`, `unknownReason`): `stalled` when a progressing stage made no progress for longer than `landing.pipeline-stall-min` (default 20), or `writer dead (pid N)` when the process writing the card is gone. The card keeps `stage`, `since` and `lastProgressAt`. Step 1 drops every unknown card from Doing, so a stuck seat reads as idle and the verdict can say All good (Sol F-2).

Rule, in `panel.ts` (`needsOf`, `doingOf`): one Needs you item per card whose goal the machine holds (the rule Doing already applies) and whose `unknown` is `stalled` or starts with `writer dead`. It is dated by `lastProgressAt`, so it sorts with the rest. Words:
- "<title>" on m1f has made no progress since 14:02 (building).
- "<title>" on m1f: the process writing its progress is gone (pid 4242); last progress 14:02.
What to do, in 2a: "If it is stuck, stop that machine with machine stop m1f at a terminal; its steward will not start it again." In 2b this sentence becomes [Stop].
Doing for that machine says "stalled", then the stage and the minutes since its last progress, with no live dot; it does not count as working. The opened row keeps the machine's jobs as today. Every other unknown reason (unprobeable, reused, no owner, claim moved, not claimed) stays as step 1 left it. No server change.

Proof, failing first (`panel.test.ts`, `FleetPane.test.tsx`): a stalled card for a held goal is a Needs you item dated by its last progress, and the verdict says "1 thing needs you"; a dead writer is one too, with its pid; a stalled card for a goal the machine no longer holds is dropped as before; Doing says stalled and the seats-working count leaves it out.

### 2a.2 The prover records why a proof is red

Today `plain.Run` (`internal/landing/plain/prove.go`) writes a red `Result` with no `Reason` when the proof command fails; the reason exists only as a line in the log. The smallest true reason is the command's own exit, which `Run` holds in `runErr` without reading any log: "the proof command exited 1", "the proof command ended: signal: killed", or "the worktree of commit abc1234 could not be made: ...". `Run` records that sentence as `Result.Reason` only when the command ran and failed. Since 703ecb830 `Run` already records a reason for an inherited green ("inherits green from tree X: only goal ledger files changed since"); that stays. An ordinary green records none. Needs you shows a reason only for a red result (`laneNeeds` checks `result === "red"` first), so a green reason never reads as a problem. Nothing is parsed. A failing gate section would need the proof command to declare its sections (later).

Nothing changes in the panel: it already shows `proof.reason` and keeps "the proof command failed" for older records. `landing status --json` carries the reason as it carries it now.

Files: `prove.go`, `prove_test.go`. Owner: m1e (`internal/landing`); about ten lines. Proof, failing first: `TestARedProofRecordsWhyItIsRed` (exit 3 records "the proof command exited 3"; a commit no worktree can be made for records that reason; a command that exits 0 records none) and `TestAnInheritedGreenKeepsItsReason` (the ledger-only tree still records "inherits green from tree ...").

### 2a.3 [Open log] on a red proof

Endpoint: `GET /api/fleet/proof-logs/<attempt>` answers `text/plain; charset=utf-8`, the whole file with its length. The client never names a path. The server looks the attempt up in this computer's lane records (`results.jsonl`, then `running.json`) and serves the `Log` that record names, only when that path, cleaned, lies directly inside the lane's proofs folder (`plain.Dir(install)/proofs`, where `Start` writes every detached proof's log). Anything else is 404 in words: an attempt that is not a safe name (letters, digits, `.`, `-`, `_`; no separator), an attempt no record names, a record whose log lies elsewhere (a result an older engine wrote), a file that is gone. A person's `landing prove --wait` at a terminal writes its log into the same folder (`intent_landing_prove.go`), so it is found like a detached one. A running proof's log is served as far as it is written.

Sign-in: none. It is a read on the same terms as `/api/board`, which already names the log's path, the commit and the reason, under the policy every read takes (allowed host, same site, one origin, GET and HEAD). The threat model is the template default: our own agents and operators make mistakes, nobody attacks. One tap on the phone must open it.

Where it shows: the Needs you red-proof item gets [Open log], a link that opens in a new tab. The Details rows "Proof log" and the running proof's "Log" become the same link. Files: `plain.ProofLog(install, attempt) (string, error)`, the lookup and the folder rule (m1e, built with 2a.2); a `ProofLog` seam on `BoardSource` and the route in `internal/ui/httpd` (new `prooflog.go`), the seam's body in `cmd/metasystem/ui.go`; `api.ts`, `panel.ts`, `LandingLane.tsx`, the bundle. Proof, failing first: `TestProofLogServesARecordedAttempt`; `TestProofLogRefusesWhatNoRecordNamesOrLiesOutsideTheFolder` (an unknown attempt, `../results.jsonl`, a record pointing at `/tmp/x.log`); `TestProofLogTakesGetAndHeadOnly`; the panel test that the red item and the two Details rows carry the attempt's link.

### What the person sees after 2a

Needs you: "The landing lane's last proof is red: the proof command exited 1. [Open log]" and ""Plain lane landing" on m1f has made no progress since 14:02 (building). If it is stuck, stop that machine with machine stop m1f at a terminal." The verdict counts both. Browser check: the walkthrough harness with a stubbed board holding one stalled card and one red proof, at 1280 and 390 px.

## Slice 2b: Pause, Resume and Stop as the signed-in person

### The path Land now takes, and where it stops short

`POST /api/fleet/land-now` (`internal/ui/httpd/landnow.go`) checks what every write checks, then a live session whose proof is `SessionValidFor` this checkout, then runs `landing run --json --repo <checkout>` in this process (`cmd/metasystem/ui_landnow.go`, `runIntentIn`) and passes the verb's envelope through, 200 whatever the outcome. Nothing of the person reaches the verb, and `landing run` asks for nothing.

The three verbs differ (`cmd/metasystem/intent_landing.go`, `intent_machine.go`, `process_verbs.go`):

| Verb | Asks a person? | How it proves one today |
|---|---|---|
| `landing stop [--by NAME] [--reason TEXT]` | no | records `by` in the pause file as given |
| `landing start` of a paused lane | yes | `laneVerbOwners.person(root)`: the verb's parent process at the enrolled terminal (`provenPerson`) |
| `machine stop NAME`, which is `system stop` per checkout | yes | `processOwners.classify`: the parent process's lease class must be human (`humanTerminalCheck`) |

Run inside the ui server, the parent is the server's launcher. Resume answers "only a person may resume the landing lane Wido stopped"; Stop answers "MetaSystem's own machinery started this shell". That is the gap R2-1 named.

### D1 The bridge: the session answers the two seams

Both proofs are seams the server already fills per call: `intentOwners.landing.person` and `intentOwners.processes.process.classify` (`cmd/metasystem/intent.go`). The route first calls `act.SignedIn(stateRoot, signed.Human, signed.Reference, signed.Proof)` (`internal/ui/act`), the same call every goal act makes. It checks the proof is a fresh session proof for this checkout and that the name and session are the ones the proof was minted for, and it classifies the session as human, as the act layer already does: "the thing that was proven is that a human answered a one-time code". Then the route runs the verb in this process with owners whose `person` returns that human and whose `classify` returns the human class. No verb body changes, no flag is added, nothing is serialized: a proof parsed from JSON has no authority by design, so neither a subprocess nor a token could carry it. Only `cmd/metasystem/ui*.go` and `internal/ui/httpd` change.

Routes: `POST /api/fleet/lane/pause` and `/api/fleet/lane/resume` with body `{}`; `POST /api/fleet/machines/stop` with `{"machine":"m1f"}`. Verbs run: `landing stop --json --repo C --by <human>`, `landing start --json --repo C`, `machine stop NAME --json --repo C`. Each answers the Land now envelope (outcome, line 1, line 2). A repeat is the verb's own unchanged success (R-129-ui): the verbs already answer "already stopped", "was not paused", "already stopped" that way. A 403 opens the sign-in sheet and the page presses once more, as Land now does.

What the server checks before it acts: POST, allowed host, same site, one origin, a bounded body with no unknown field; a live session; `SessionValidFor` this checkout; `act.SignedIn`'s name and session match. For Stop, the name passes `board.SafeName`, and the server resolves it exactly as the verb does (`discoverHostMachines` then `matchMachine`, `intent_machine.go`: this checkout, the host registry's armed and stopped checkouts, the lane's), through the same reading in `cmd/metasystem/ui.go`. It refuses, before any stop runs: a name that resolves to no machine of this computer, or to one on another computer; and the serving checkout itself (`matched.This`, or a checkout equal to `roots.Checkout`), in words that say the terminal stops that one (S2-01). A board seat is not the test (S2-02): the board's seats are the armed checkouts only (`pipeline.go` `Seats`, `registry.ArmedCheckouts`), so a machine just stopped leaves them, and a second tab's Stop would be refused instead of reaching the verb's own "already stopped". The verb decides the rest: a lane being unset, a lane that cannot run, a machine already stopped.

What is recorded as the actor: Pause writes the pause file with `by` = the session's human, which the panel and `landing status` show ("paused by Wido since 22:40"). Resume removes the pause file and records nobody, at the terminal as from the browser. Stop's fence records the verb and the engine process that ran the stop (`stoptransition/transition.go`, `self()` probes `os.Getpid()`): the ui server from the browser, the `metasystem` process at a terminal, never the shell. The browser is recorded no worse than the terminal; a resume or stop record is on the later list.

### What the person sees

- Lane heading: [Pause] when Running, [Resume] when Paused, beside the state word, never both; neither while the lane is unread (`unreadOf`, the Land now rule). The Needs you "lane paused" item shows [Resume] instead of the terminal sentence.
- Fleet table, opened row: [Stop] for a machine with a board seat on this computer, other than the one serving this page. A stop run inside the server either ends the server mid-act or, since the untracked family skips its own pid (`stoptransition/families.go:893`), leaves it standing while reporting the machine stopped; either way the page cannot show the result. That one machine is stopped at a terminal, its row says so, and the server refuses it as well (above): hiding the button is not the guard. The stuck-seat item of 2a.1 shows [Stop] under the same rule.
- Stop asks once more: the button becomes "Stop m1f? [Yes, stop it]". A stop ends the seat and every job on it, and the steward does not start it again; a stray tap on a phone must not do that. Pause and Resume are reversible and need no second press.
- After each press the board is re-read, so the state word, the row and Needs you follow.

Files: `internal/ui/httpd/laneacts.go` with its test (modeled on `landnow_test.go`), `httpd.go` (route table); `cmd/metasystem/ui.go`, `ui_landnow.go` (one runner that takes session owners); `api.ts`, `panel.ts`, `LandingLane.tsx`, `FleetPane.tsx` and their tests, `fleet.css`, `help/terms.ts` with `partner/vocabulary.json` for the three words, the bundle. m1e-owned files: none; m1e reviews at landing.

Proof, failing first. httpd: each route refuses every hand but a signed-in one and runs nothing; a signed-in Pause runs `landing stop` with `by` = the session's human; Stop refuses a name that is not a safe name, one that resolves to no machine of this computer, and the serving checkout, each before the stop seam is called (the seam counts zero calls); a seam failure is 500 in words; a repeat passes the verb's unchanged through. cmd/metasystem: `TestUIResumeRunsLandingStartWithTheSessionAsThePerson` (a paused fixture lane resumes and its pause file is gone); `TestUIStopClassifiesTheSessionAsAPerson`; `TestUIStopRefusesTheServingCheckout` (the machine bed's "this checkout" named by nickname and by path: no fence changes, no process is touched); `TestUIStopTwiceIsTheVerbsUnchangedWithNoNewFence` (two signed-in stops of one fixture machine: the first is confirmed and its fence completed, the second is the verb's unchanged and the tree digest is the same, as `witnessMachineStopRepeat` checks with `idemSameTree`). Panel: Pause only when Running, Resume only when Paused, neither when unread; Stop only for a seat of this computer that is not this seat; the second press; the sign-in retry once.

## Later, when it hurts

- Fleet-wide questions in Needs you (R2-8); the verdict's scope line stays until then.
- A reason box for Pause; a one-line record of who resumed or stopped.
- The failing gate section as the red reason; needs the proof command to declare its sections.
- The log in the page, tailed, following a running proof; today it opens whole in a new tab.
- Other unknown cards (unprobeable, reused, no owner) as items; the steward's seat-idle alert (a seat with nothing to do, not a stuck one).
- [Stop] for the machine serving the page; Talk and Forget (their own goals); progress bars.

## Open questions for Wido

1. Resume and Stop from the browser need one ruling row extending R-125-m1u and R-128-ui: a signed-in session is admitted for `landing start` and `machine stop` from the fleet panel, recorded as above. Recommendation: yes; it is what R2-1 and your step-2 go already say, and the register's readers can then check it.
2. Stop asks a second press; Pause and Resume do not. Recommendation: keep it.
3. The machine serving the page is neither offered nor admitted for Stop. Recommendation: keep it; the terminal does that one whole.
4. The red reason is the exit sentence only. Recommendation: yes; the section name waits for the proof command to say it.

## Critique round 1 (Astra)

| Id | Material | Slice | Disposition |
|---|---|---|---|
| S2-01 | yes | 2b | folded: the server refuses the serving checkout before any stop runs; a test proves the stop seam is never called |
| S2-02 (R-129-ui) | yes | 2b | folded: Stop's admission is the verb's own local-machine resolution, not "is a board seat"; a two-request test proves the repeat is unchanged with no new fence generation |
| S2-03 | no | 2a | text corrected: a terminal `landing prove --wait` writes its log into the proofs folder and is found |
| S2-04 | no | 2a | text corrected: the exit reason is set only when the command ran and failed; an inherited green keeps its reason (703ecb830), with a test |
| S2-05 | no | 2b | text corrected: the fence records the engine process that ran the stop, never the shell |

## Critique round 2 (Astra, the failsafe)

S2-01 to S2-05 all closed; no new finding. VERDICT: 0 material.
