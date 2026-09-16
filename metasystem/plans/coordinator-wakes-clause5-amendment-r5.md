### 7.2a The observed stamp is taken after the read (revision 5)

Revision 4 settled which publication is a sample's edge. Revision 5 amends two points and nothing else: where the loop samples `observedBootNanos`, and what a `wait-published` under another boot identity does to a sample. Sections 7.1, 7.4, 7.5 and 7.6 stand. The line references below are to the candidate branch `m1b-cw5-ready` at df37dd21c, which is where the code the read looked at lives; trunk does not yet have it.

#### What the candidate does today

`waitLoop` samples the boot clock once per iteration, at the top (`metasystem/internal/run/waiter.go:780`), and puts that sample into `iterationStamps` (`:781`). It then runs `Actionable` (`:797`, bounded by ten seconds or the remaining deadline) and `observe` (`:813`, the same bound). A ready return passes `iterationStamps` into `finishV2` as the observed stamp (`:841`, `:810`); a pending observation persists the same sample into the row as `lastObservedBootNanos` and `lastObservedBootId` (`:852`). Registration has the same shape: `registrationStamps` (`:940`) precedes `initialObservation` (`:941`), and the at-entry finish passes it as the observed stamp. Renewal repeats it: the sample at `:1282` precedes the renewal's read.

The read's claim holds. The observed stamp is meant as the upper edge of the publication: the read that returned ready saw the write, so the write was visible no later than the instant the read looked, and that instant is no later than the read's return. A stamp taken before the read starts is not that bound. A write that lands two seconds into an eight-second read is seen by that read and gets an observed stamp eight seconds before the truth. 7.2 takes `publishedUpper` as the smaller of `observedBootNanos` and the hint's `publishedBootNanos`, so the early stamp wins, `rLowerNanos` grows by the length of the read, and a sample near the bound is `refuted` on a number the engine never measured. The direction is wrong for a measurement that may only refute: it can refute what did not fail.

The read's second claim also holds. The lower edge, `prevObservedBootNanos`, is the previous iteration's top-of-loop sample, taken before the previous read. That read did not see the write, so the write was not visible at the instant it looked, and that instant is at or after the sample. The write landed after the sample. The lower edge is sound; it is the upper edge that is not.

#### The rule

Every read has two samples: one before it starts and one after it returns. The two carry different meanings and are not interchangeable.

- The pre-read sample is the lower edge a later iteration uses. It stays where it is, at the top of the iteration, before `Actionable` and `observe`. When the iteration's read finds the source still pending, the iteration persists the pre-read sample into `lastObservedBootNanos` and `lastObservedBootId`, exactly as `:852` does today. The next ready return reads those row fields as `prevObservedBootNanos` and `prevObservedBootId`. This is the value the read called sound, and the rule does not move it.
- The post-read sample is the observed stamp. The loop takes it immediately after the read that produced the return comes back: after `observe` when the observation was ready, after `Actionable` when that call reported a change, after `initialObservation` and after the renewal's read for an at-entry finish. That sample, not the top-of-loop one, goes into `finishV2` as `ObservedBootNanos` and `ObservedBootID` for every ready return.

Why the next iteration's lower edge stays sound under this rule. The row is written only by an iteration whose read did not see the write, and it is written with that iteration's pre-read sample. The read looked at the source at some instant between its pre-read sample and its post-read sample; not seeing the write proves the write was not yet visible at that instant; so the write became visible after it, and therefore after the pre-read sample. The post-read sample of that same read would not do as a lower edge: the write may have landed between the instant the read looked and the instant it returned, and a lower edge set at the return would then sit after the publication and refute falsely from the other side. The two samples are therefore not symmetric: the one before the read bounds the publication from below when the read misses, the one after the read bounds it from above when the read hits. The rule keeps each on its own side.

Why the pre-read sample stays at the top of the iteration and not just before `observe`. Between the two sits `Actionable`, bounded by ten seconds. Sampling before it makes the lower edge up to ten seconds early, which widens the bracket and can only move a sample from `returned-within-60` to `suspect`, never to `refuted`. Adding a third sample per iteration to win those seconds is not worth a second row field, and the top-of-loop sample already serves the deadline check and `RemainingNanos`.

Two consequences the builder owns:

- `finishV2` today replaces a zero observed stamp with the row's `lastObservedBootNanos` (`:719` to `:720`). That fallback is fine for deadline, failed and interrupted returns, which are never sampled. It must not apply to a ready return: the row holds the previous pre-read sample, which is a lower edge, and using it as the upper edge recreates the defect. A ready return whose post-read sample could not be taken carries a zero observed stamp and the measure marks it `unavailable: clock-not-comparable`, which is 7.2's existing rule for a zero used stamp.
- The post-read sample's identity is compared with `row.DeadlineBootID` like the top-of-loop one. It cannot differ for a live process, and if it does the stamp is carried as sampled and the measure marks the sample `clock-not-comparable`.

`stamp-order` is unchanged and still holds: `registeredBootNanos` is the registration sample, `prevObservedBootNanos` a later pre-read sample, `observedBootNanos` a post-read sample of a later read, `returnedBootNanos` sampled inside `finishV2` after that.

#### Amended rows of the 7.2 stamp table

| Stamp | Sampled where | Carried as |
| --- | --- | --- |
| `prevObservedBootNanos`, `prevObservedBootId` | the row's `lastObservedBootNanos` and `lastObservedBootId` as they stood when `finishV2` was entered. The row holds a pre-read sample: the top-of-iteration sample of the last iteration whose read found the source still pending, persisted by that iteration; at registration and renewal, the registration sample, which precedes the initial read. It is never a post-read sample. | `wait-returned` |
| `observedBootNanos`, `observedBootId` | a sample taken immediately after the read that produced the return came back: after `observe`, after `Actionable` when it reported the change, after `initialObservation` or the renewal's read for an at-entry finish. Passed into `finishV2`; never the top-of-iteration sample and never filled from the row on a ready return. | `wait-returned` |

The other four rows stand. `registeredBootNanos` is still sampled before the initial read; it is a registration stamp, not an edge.

### 7.7a A hint under another boot identity joins nothing (revision 5)

7.3, as superseded by 7.3a, makes boot identity a join term: a `wait-published` matches only when its `kind`, `targetId`, `publicationId` and boot identity equal the return's. 7.7 lists a fixture case, "a `wait-published` under another boot identity", asserting `clock-not-comparable`. The two cannot both hold. 7.3 holds and 7.7 is amended.

The reason is 7.2's own definition. `clock-not-comparable` is a property of the stamps the arithmetic uses: any used stamp zero, or two used stamps under different identities. A hint that does not join contributes no stamp. Its identity therefore cannot make the sample's stamps non-comparable; the return's own four stamps agree with each other and the sample is evaluated on them. That is the `unhinted` outcome of 7.3a with reason `no-matching-publication`: the previous observation did not see the write, the observation that returned did, and the hint is simply not there. Making the sample unavailable instead would discard a sample the waiter's own records can still evaluate, and an `unhinted` sample can still be `refuted` on the waiter's return latency alone. Throwing it away hides refutations and proves nothing in exchange.

Which identity is the join term. A hint carries two: `beganBootId`, sampled by the owner before the write, and `publishedBootId`, sampled inside `NotifyWaiters` after it. The join uses `publishedBootId` against the return's `returnedBootId`, which is what the candidate does (`metasystem/internal/usage/waitmeasure.go:275`). A hint that joins but whose non-zero `beganBootNanos` carries an identity other than the return's is a different case: its began stamp is now a used stamp, the lower edge, and 7.2's definition applies as written, `clock-not-comparable` (`:327`). No owner in the engine can produce that hint, since a reboot between the sample and the notification ends the process; a shell `wait notify` with a mismatched `--boot-id` can, and the sample says so rather than guessing.

`clock-not-comparable` keeps its other producer unchanged: a return whose own four stamps disagree about identity, or carry a zero (`:320` to `:324`).

The candidate's fixture conflates the two. Its "boot mismatch" case (`metasystem/internal/usage/usage_test.go:49` to `:50`) changes the return's `returnedBootId`, which both unjoins the hint and makes the return's own stamps disagree, and then asserts `clock-not-comparable` with `no-matching-publication` beside it. That passes for the wrong reason. It is split into the two cases below.

#### Amended 7.7 fixture rows

| Fixture | Owner file | Sets up | Asserts, and the false rule each case would expose |
| --- | --- | --- | --- |
| TestWaitMeasurementAccounting (amended cases only; the rest of the row stands) | `metasystem/internal/usage/usage_test.go` | a `wait-published` whose `publishedBootId` is not the return's, the return's own four stamps agreeing with each other; a return whose own four stamps carry two boot identities; a joined hint with a non-zero `beganBootNanos` under an identity other than the return's | the first is `unhinted` with `no-matching-publication`, its bracket from `prevObservedBootNanos` and `observedBootNanos`, neither hint stamp printed as an edge, and its verdict computed, which exposes the rule that a foreign hint can void a sample; the second is `clock-not-comparable`; the third is `clock-not-comparable`, which exposes a rule that would subtract a began stamp from another clock |
| TestWaitLifecycleEvents (one added assertion; the rest of the row stands) | `metasystem/internal/run/waiter_test.go` | the fake source advances the fake boot clock while inside `Observe`, on the pending observation and on the ready one, and inside `initialObservation` for the at-entry registration | the ready return's `observedBootNanos` is later than the instant its read started; the row's `lastObservedBootNanos` written by the pending observation is earlier than the instant that read started; the at-entry return's `observedBootNanos` is later than its `registeredBootNanos`; a ready return whose post-read sample fails carries zero, not the row's value. This exposes the pre-read observed stamp of F5 and the `finishV2` fallback |

#### What the builder does

- `metasystem/internal/run/waiter.go`: after `observe` returns and after `Actionable` reports a change, sample the boot clock into the stamps passed to `finishV2`; leave the top-of-iteration sample as what `:852` persists. Do the same after `initialObservation` in `Wait` and after the renewal's read in `renewSavedWait` for their at-entry finishes. Restrict the `:719` fallback to non-ready returns.
- `metasystem/internal/usage/waitmeasure.go`: no logic change; the join at `:275` and the checks at `:320` to `:328` already implement the rule. The reason printed for a foreign-identity hint is `no-matching-publication`, which `:299` already does.
- `metasystem/internal/usage/usage_test.go`: split the "boot mismatch" case into the three cases above.
- `metasystem/internal/run/waiter_test.go`: the added assertion above, with the fake clock advanced from inside the fake source.
