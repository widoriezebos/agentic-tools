# Codex design critique of clause 5 amendment r2

Job m1b-cw5-crit2-1789528370, against metasystem/plans/coordinator-wakes-clause5-amendment-r2.md (landed 2802028db).

## Finding 1 [critical]

Section 7.2, metasystem/plans/coordinator-wakes-clause5-amendment-r2.md:27-39, does not prove that the echoing inference request consumed the wait result. The token is persisted in the waiter row and emitted in wait-returned before command output, and validation explicitly accepts either copy. A log reader, fixture, or second process descended from the authenticated main can therefore obtain and echo the token before the result reaches any model request; replay also exposes the same token again. The earliest forged echo then wins and can make a request delayed beyond 60 seconds pass. This defeats the amendment's central premise and must be fixed before a builder starts.

Evidence: metasystem/internal/run/waiter.go:647-656 persists the result before returning it; metasystem/internal/events/emit.go:124-133 writes a repository-readable event stream; metasystem/internal/lease/classify.go:377-432 classifies descendants of the main as that main. Amendment line 37 accepts a token from the row or event, contradicting line 31's assertion that it was delivered only inside the result. TestWaitConsumedVerb at amendment line 117 tests session acceptance and refusal but never an authorized sibling echoing before result delivery, so the fixture would pass while this rule is false.

## Finding 2 [critical]

Section 7.4, metasystem/plans/coordinator-wakes-clause5-amendment-r2.md:73-78, can pass true latency above 60 seconds because wait-published is joined only by kind, target identifier, boot identifier, and time window. A dropped hint for the publication actually consumed can be replaced by a later hint for another write to the same goal; using that later beganBootNanos moves publishedLower forward and shortens latencyUpper. This must be fixed before a builder starts.

Evidence: metasystem/internal/goal/txn.go:557,730,788 can notify the same goal repeatedly, including idempotent already-applied outcomes. The amendment's matching rule carries no sourceEvidence, ledger tip, operation identifier, or publication identity. TestWaitMeasurementAccounting at amendment line 118 places one of two publications before the previous observation; it does not cover a dropped true hint followed by a different same-target hint inside the matching window, so it would not expose this false pass.

## Finding 3 [critical]

The pre-write lower-edge instructions cite post-write helpers, so a builder following section 7.4 and builder item 6 can sample beganBootNanos after publication and produce a false pass. metasystem/plans/coordinator-wakes-clause5-amendment-r2.md:68 and :138 cite dispatch line 37, goal line 500, and landing line 65 as the pre-write locations, but those are notification helper declarations; their calls follow the durable writes. A post-write began stamp is not a lower bound on publication. This must be fixed before a builder starts.

Evidence: metasystem/internal/dispatch/record.go:386-393,489-496,582-592 writes before notifying. metasystem/internal/goal/txn.go:557,730,788 notifies only after confirmation or an already-applied decision. metasystem/scripts/agents/land.sh:980,1170,1200 calls the line-65 helper after successful pushes. Only metasystem/internal/proofrun/launcher.go:488 is genuinely before its named terminal write. The builder needs the actual write sites and retry semantics, not the cited helpers.

## Finding 4 [critical]

Sections 7.3 and 7.5, metasystem/plans/coordinator-wakes-clause5-amendment-r2.md:56 and :91-101, can hide a delivered ready result as pending-at-cut. The design guarantees checked append only for wait-consumed, yet asserts that a failed wait-returned append prevents result delivery without specifying checked emission or its failure outcome. With the existing best-effort emitter, persistence can succeed, wait-returned can disappear, the result can print, and a missing echo leaves only wait-registered; measurement then calls it pending-at-cut instead of unavailable no-echo. This can leave clause 5 falsely green and must be fixed before a builder starts.

Evidence: metasystem/internal/events/emit.go:41-45 and :120-133 silently drop registry, size, open, and write failures. Builder item 4 at amendment line 136 merely says to emit wait-returned, whereas line 56 reserves EmitChecked for wait-consumed. TestWaitLifecycleEvents covers failed row persistence, not event failure after successful persistence; TestWaitMeasurementAccounting covers a missing return event only when an echo exists. No fixture covers both a lost return event and no echo.

## Finding 5 [critical]

The event schemas cannot perform the boot-identity validation required before arithmetic. wait-consumed at metasystem/plans/coordinator-wakes-clause5-amendment-r2.md:52 contains copied return stamps and a newly sampled consumedBootNanos but only one bootId; when wait-returned is missing and stampsFrom is row, measurement cannot compare the return boot with the echo boot as line 82 requires. wait-published similarly combines caller-supplied beganBootNanos with a locally sampled publishedBootNanos under one bootId without requiring the two identities to match. Cross-boot elapsed values can therefore be subtracted and can coincidentally produce a passing interval. This must be fixed before a builder starts.

Evidence: Amendment lines 45,50,52 provide one bootId per event, while lines 60 and 82 require an identity for every used stamp and an explicit return-versus-echo comparison. Builder item 3 adds bootId to WaitResult, but the wait-consumed schema does not copy it separately from the echo's boot identity. The cross-host fixture at line 118 covers only a wait-published event with another bootId; it does not cover a missing return event with row stamps across a reboot or a mismatched began/published pair.

## Finding 6 [high]

Section 7.5, metasystem/plans/coordinator-wakes-clause5-amendment-r2.md:93-94, excludes every at-entry result from the 60-second rule even though the parent clause says to measure each event to the first resumed request consuming its result. An event may be durable for more than 60 seconds before wait registration and then return at entry; the polling stream does not derive or enforce that event-to-request latency. The fixture at line 118 expressly asserts already-published exclusion, so it would preserve rather than expose the violation. This must be fixed before a builder starts.

Evidence: metasystem/plans/coordinator-wakes-on-events-not-polls-design.md:228-231 requires each event's upper bound below 60 seconds and says polling proves absence of polling, not prompt wait registration. Amendment line 94 supplies no declared input that proves a late at-entry registration was timely; it merely asserts that the separate polling proof catches it.

## Finding 7 [high]

The token-unknown outcome is not constructible from its declared inputs. metasystem/plans/coordinator-wakes-clause5-amendment-r2.md:37 and :52 require an unknown-token wait-consumed event to carry waitId, nonce, kind, targetId, the row's runtime session, and copied return data, but by definition neither a row nor a return event matched the token. The same line says the event has no stamps, contradicting the required-field table, and best-effort emission cannot guarantee that line 98 counts the loss. A builder must invent a second schema and grouping rule. This must be fixed before a builder starts.

Evidence: The contradiction is within amendment lines 37,52,91,98 and builder item 8 at line 140. TestWaitConsumedVerb at line 117 asserts only that a token-unknown line appears; TestWaitMeasurementAccounting does not include token-unknown, so no fixture proves its fields, runtime grouping, unavailable classification, or loss behavior.

## Finding 8 [high]

The waiter lifecycle does not carry enough provenance to emit the required mode and atEntry fields consistently. wait-returned requires mode register, renew, takeover, or replay at metasystem/plans/coordinator-wakes-clause5-amendment-r2.md:51, but builder item 4 at line 136 adds only AtEntry and ObservedBootNanos to finishStamps. Current finishV2 is shared by fresh registration, renewal, takeover, deadline, interruption, and observation returns, and the separate renewal-at-entry call is not named by the builder rule. Implementers must guess whether to add another argument, infer from mutable row history, or store mode. That choice changes sample inclusion and must be fixed before a builder starts.

Evidence: metasystem/internal/run/waiter.go:643 defines the shared finishV2; fresh at-entry is at :982-983, renewal at-entry is separately at :1354-1355, and takeover re-enters the loop at :1536. Amendment line 136 specifies the first at-entry call but says every other caller passes what it has. TestWaitLifecycleEvents at line 115 includes one at-entry return, one renew, and one takeover but does not require at-entry classification for each lifecycle mode.

## Finding 9 [high]

The mandatory moved-effects gate rejects the amendment. The section at metasystem/plans/coordinator-wakes-clause5-amendment-r2.md:123-125 contains prose saying None rather than the required Effect, From, To, Code inventory, so the validator reports MOVED-EFFECTS-NO-ROWS. Under the role contract, a validator-reported moved-effects problem is material and must be fixed before a builder starts.

Evidence: Running metasystem/bin/metasystem validate moved-effects --file metasystem/plans/coordinator-wakes-clause5-amendment-r2.md exited 1 with: MOVED-EFFECTS-NO-ROWS: line 123: Moved effects section has no inventory rows.

## What this critique did not check

- The proposed fixtures do not yet exist outside the plans, so fixture sensitivity was judged from their declared setup and assertions rather than execution.
- No live Claude, Codex, or other provider-host trace was available. The token attack is grounded in the shipped local storage and caller-classification paths.
- The broad-read launcher classified this job as advisory and did not prove independent context isolation.
- No files were edited, as required by the design-critic role.
