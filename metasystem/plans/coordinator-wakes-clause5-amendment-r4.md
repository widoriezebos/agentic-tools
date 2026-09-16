### 7.3a Which publication is the sample's edge (revision 4)

Section 7.3 joins a `wait-published` event to a return by `publicationId` equal to the return's `sourceEvidence` (jobs, attempts) or `ledgerTip` (goals, landings), and forbids a time window. The implementer stopped on a real gap: for jobs the evidence is not unique per durable write, so "the matching hint" is not one hint. This section closes the gap. It changes no observer, adds no event, and never picks among candidates.

#### What the tree does today

The job observer builds one evidence string per incarnation, not per write. `ObserveJob` (`metasystem/internal/dispatch/watch.go:82`) returns `job:<jobId>:<operationId>:pending-setup` for a record in setup (`:100`) and otherwise `job:<jobId>:<operationId>:r<round>:<startedAt>` (`:110`) for pending, running and every terminal status alike; the status goes into `Outcome` (`:111`), and a terminal record also carries `TerminalStamp` from `endedAt` (`:126`). The fields come from the record file `artifacts/agents/jobs/<id>.json` decoded into `jobStatus` (`:57` to `:64`, `readJobStatus` at `:66`). The waiter stores that string as the result's `SourceEvidence` and the outcome as `SourceOutcome` (`metasystem/internal/run/waiter.go:97` to `:98`, set at `:462`); 7.2 copies both into `wait-returned`.

Three job write paths notify, all through `notifyJobWaiters` (`metasystem/internal/dispatch/record.go:37` to `:39`), which today sends `WaitHint{Kind: "job", TargetID: job}` and nothing that names the write:

| Path | What it writes | Guard that prevents a second write of the same status | Hint |
| --- | --- | --- | --- |
| `RecordSetup` (`:334`) | the setup husk swap: the record goes from `pending-setup` to `pending` (`writeRecord` at `:386`) | current must be `pending-setup` and the source must be `pending` (`:345` to `:347`) | `:393` |
| `RecordProtocolError` (`:445`) | status `failed`, `error` `protocol_error`, `endedAt` (`:489`) | a record already failed with the same violation key returns without writing, so `wroteTransition` stays false (`:463` to `:467`); otherwise the current status must equal `expect`, which must be `pending` or `running` (`:468`) | `:496` |
| `RecordCAS` (`:509`) | the general transition to `target`, `endedAt` stamped when `target` is terminal (`:579` to `:582`) | `current != expect` refuses (`:523`); a same-status write is a metadata update and does not notify (`:543`, `:585`); any other edge must be in `statusTransitions` (`:544`) | `:592` |

`statusTransitions` (`:44` to `:48`) is acyclic and terminal states have no outgoing edge (`:51` to `:53`): `pending-setup` to `failed` or `cancelled`; `pending` to `running`, `failed`, `cancelled`; `running` to `completed`, `failed`, `cancelled`, `timeout`. Together with the guards above, one job record is written with notification at most once per status. So for one job the pair (evidence, status) names at most one notifying durable write, while evidence alone names two or three of them: the setup write, the running write when there is one, and the terminal write. Every job sample, not a rare one, faces the ambiguity under 7.3 as written.

The other owners 7.3 names do not share the problem in the engine, but share the need for a duplicate rule:

- Proof attempts. `ObserveAttempt` reports `attempt:<attemptId>:<identityDigest>` (`metasystem/internal/proofrun/attempt.go:328`) for pending and terminal alike, but the only engine hint site follows `FinalizeAttempt` (`metasystem/internal/proofrun/launcher.go:488`, hint at `:497`), and `FinalizeAttemptWithTestResultLocked` refuses a second terminal write (`attempt.go:1092` to `:1094`). One notifying write per attempt, so the evidence is unique per write.
- Goals and landings. The evidence is `ledger:<tip>:operation:<opid>` (`metasystem/internal/goal/attention.go:877`) or `commit:<tip>:<provenance>` (`:841`), and 7.3 joins on `ledgerTip`, which is one commit. Unique per write. The `:557` and `:788` outcomes of 7.3 may re-announce an earlier write's tip with a zero `beganBootNanos`; that case is covered below.
- Any kind. `wait notify` accepts `--job`, `--attempt` and `--goal` from a shell (`metasystem/cmd/metasystem/wait_verb.go:243` to `:252`) and 7.7 gives it `--publication-id`. A shell can therefore emit a hint with any identity, or none, for any kind. The rule below must not let that hint shorten a bracket.

#### The rule

The join key of a ready return is built from `wait-returned` alone, per kind:

| Kind | Join key |
| --- | --- |
| job | `sourceEvidence` + `:` + `sourceOutcome` |
| attempt | `sourceEvidence` |
| goal, landing | `ledgerTip` |

A `wait-published` matches when its `kind`, `targetId` and boot identity equal the return's (7.3) and its `publicationId` equals the join key. An empty `publicationId` matches nothing.

The publication owner writes the same key as `publicationId`. For jobs that is the observer's evidence for the record as written, joined with `:` and the status written: `job:<jobId>:<operationId>:r<round>:<startedAt>:<status>`. The owner computes it inside the locked closure, after `writeRecord` succeeded, from the bytes it wrote: decode them through `jobStatus` and a formatter shared with `ObserveJob`, never from the `map[string]any` it assembled, so that owner and observer agree by construction and not by review. `notifyJobWaiters` takes the key and the pre-write boot sample of 7.3, and its three call sites (`record.go:393`, `:496`, `:592`) pass what their closure captured. `ObserveJob`'s string does not change; it only moves into the shared formatter. Attempts, goals and landings keep 7.3's `publicationId` unchanged.

Among the matching hints, the sample's publication edge is the one hint whose `beganBootNanos` is non-zero. That hint supplies `publishedLower` as in 7.2. `publishedUpper` is the smaller of `observedBootNanos` and the `publishedBootNanos` of every matching hint, including hints with a zero `beganBootNanos`: a zero sample means the owner found the write already made (7.3, `:557` and `:788`), so its publication stamp follows the write and is a sound upper edge, which is what 7.2 already used it for.

Why this and not a changed observer. `sourceOutcome` already sits beside `sourceEvidence` in the same event and the same row, and for a ready job return it is the written status (`watch.go:111`, the cases at `:113` to `:122`). Making the evidence itself unique would change `SourceEvidence` in the v2 result schema (`waiter.go:97`) and the replay staleness check that compares it (`waiter.go:1146`) for the benefit of a measurement, with nothing gained that the outcome does not already give. The rule is per kind because the attempt launcher does not know the terminal result string when `options.CommitTerminal` is supplied (`launcher.go:481` to `:483`), and for goals the tip is already a write identity.

Why never the first, last or nearest. With the key above, two matching hints with non-zero `beganBootNanos` can only come from a duplicate identity: a shell hint that copied the engine's, or an owner defect. The records then witness two publications under one name and do not say which one the return consumed. Any choice would be an estimate, and the wrong one refutes falsely or hides a refutation. So the rule refuses to choose.

#### When the rule cannot be satisfied

The sample keeps its observation edges and loses the hint. That is 7.3's `unhinted` outcome with a named reason, and it is the honest outcome because the observation edges are witnessed by the waiter itself: the previous observation did not see the write, the observation that returned did, so the write became visible between them regardless of what any hint says. Nothing about the hint is estimated; the hint is not used.

| Situation | Outcome | Loose-edge reason printed |
| --- | --- | --- |
| no matching hint (dropped hint, the owner died between write and hint, events append failed, a shell hint with no `--publication-id`) | `unhinted`: `publishedLower` = `prevObservedBootNanos`, `publishedUpper` = `observedBootNanos` | `no-matching-publication` |
| two or more matching hints with non-zero `beganBootNanos` | `unhinted`, every matching hint discarded, including the zero-sample ones, because a colliding identity makes all of them untrusted | `publication-ambiguous`; the report also counts these per owner as a defect line, so a duplicate identity cannot hide inside a suspect |
| the return lacks the field the key needs (`sourceEvidence` or `sourceOutcome` empty on a ready job or attempt return, `ledgerTip` empty on a goal return) | `unhinted` | `join-key-missing`, counted as a record defect |
| any of the above on an at-entry return | `unavailable: no-lower-edge` (7.2), with the same reason beside it, because an at-entry return has no previous observation to supply a lower edge | as above |

An `unhinted` sample is still a sample. It can be `refuted` when the waiter's own return latency after the observation reaches 60 seconds, and `suspect` when the observation bracket reaches 60 seconds; it can never be a pass, because there is no pass. A sample that cannot be joined never becomes `returned-within-60` on the strength of a hint it does not have; it can reach that class only when `observedBootNanos` and `returnedBootNanos` alone put `rUpperNanos` under 60 seconds, which is a fact of the waiter's records.

The measurement stays inside the engine's own records: the key is built from the record the owner wrote and from `wait-returned`, the events are the engine's, and no runtime hook, harness or transcript is consulted. The shell path is plumbing that carries a key it is given; a shell that gives none produces a hint that joins nothing.

#### Effect on the verdicts of 7.2

No verdict vocabulary changes and no arithmetic changes. Three things move:

1. Job samples become joinable. Under 7.3 as written every job sample had two or three same-identity candidates and no rule; this section gives each ready job return exactly one publication witness, the terminal write's, or none. Job at-entry returns that 7.3 would have left `unavailable: no-lower-edge` for want of a defined match now join and are sampled under the `at-entry` class with the owner's `beganBootNanos`, which is the finding-6 case 7.2 requires.
2. Compared with any rule that picks one of several same-identity hints, this rule can only widen a bracket, never narrow it. It cannot turn a `suspect` into a `refuted`. It can leave as `suspect` what a "pick the latest" reading would have `refuted` on a hint that belonged to another write, and that is exactly the false refutation the picking rule would have produced.
3. Hints with a zero `beganBootNanos` keep the role 7.2 gave them, an upper edge only, so no goal sample changes verdict on that account. They are discarded only when the identity is ambiguous, where discarding widens the bracket, as in item 2.

#### What the builder does

- `metasystem/internal/dispatch`: extract the evidence formatter used at `watch.go:110` into a function both `ObserveJob` and the owners call; give `notifyJobWaiters` the key and the boot sample; at `record.go:393`, `:496`, `:592` pass the key computed inside the closure from the written bytes through `jobStatus`.
- `metasystem/internal/usage/waitmeasure.go` (7.7): build the join key per kind from `wait-returned`; select the single non-zero-sample match; take the upper edge over all matches; emit the three reasons and the two defect counts above.
- Fixture row `TestWaitPublishedAtOwners` (7.7), job column: `publicationId` equals the observer's evidence for the same record joined with `:` and the written status; add one case where a job goes `pending-setup`, `pending`, `running`, `completed` and the measure joins the return to the `completed` hint only; add one case with a duplicated engine identity from a shell `wait notify --job ... --publication-id` and assert `publication-ambiguous` with the observation edges.
- `wait notify`: no new flag; `--publication-id` already carries the key for any kind.
