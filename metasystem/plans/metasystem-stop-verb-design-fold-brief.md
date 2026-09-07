Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Design round 2: fold the critique of your section 18 (goal metasystem-stop-verb)

You wrote section 18. A design critic on a different model read it and
found three material holes, listed verbatim below. Decide them in the
page, in the same voice, as amendments to section 18 itself.

THIS IS THE LAST DESIGN ROUND for this slice. The implementation-first
ruling applies: after this the decisions go straight into a build brief
and any further finding is carried as a build finding, not a fourth
prose budget. So leave nothing that an implementer must choose.

# The critique's findings, verbatim

### S18-01 (high)

CLAIM: Section 18 turns missing evidence about a remote job into success after one extra stop. It first says that disappearance of the local job record is not evidence that the remote job is terminal, so arm correctly refuses. It then requires a fresh stop to erase the survivor when the jobs directory can be read but the file is absent. That stop has learned nothing about the process on the other machine, yet it writes a completed stop and permits arm. An implementer following this design would therefore build a false-success path that contradicts the earlier rule that a remote job remains an objection until its record is terminal. The artifact that must change is metasystem/plans/metasystem-stop-verb-design.md: it must name terminal evidence or an explicit human risk-acceptance path instead of treating absence as proof.

EVIDENCE: Section 18.1 lines 1357-1360 calls the missing file missing evidence and explicitly refuses to declare the remote process dead. Lines 1371-1378 then say a successful family read that finds no job clears the entry and may write closed/stopped. Section 2 lines 267-270 says the remote job counts as alive until its record is terminal. Section 18.6 lines 1514-1524 requires tests that encode the unsafe absence-clears behavior.

### S18-02 (high)

CLAIM: The new equality invariant cannot hold when publication of the final fence state fails. Sections 14.1 and 17.5 require every failure after the fence closes to become a printed failure line, and section 18 requires every final NOT STOPPED line to have a durable survivor entry and contribute exactly once to the closing count. If writing that final record fails, the system can print and count the failure but cannot durably add its matching entry through the failed write. The design does not choose a fallback, an earlier persistence point or a qualified invariant. The artifact that must change is metasystem/plans/metasystem-stop-verb-design.md so the builder does not have to invent the transaction's most important persistence-failure behavior.

EVIDENCE: Section 14.1 lines 1112-1116 makes every post-close failure a printed line; section 15.1 lines 1186-1195 requires a matching durable entry; section 17.5 lines 1320-1323 makes the rule absolute; section 18.2 lines 1410-1413 asserts exact equality. The built slice's final-write branch in internal/stoptransition/transition.go already demonstrates the contradiction: after stopfence.Write fails it appends a NOT STOPPED line and increments finalNotStopped, but the record containing that entry was not written.

### S18-03 (medium)

CLAIM: The status read-failure contract leaves the implementer a user-visible choice. Section 18 says a failed inventory read preserves a partial report, but it never decides whether status stops at the first unreadable family or continues inventorying later independent families. Those choices show different running processes and require different control flow. The proof section also omits this failure case even though the current unreadable-fence test asserts the opposite exit code. The artifact that must change is metasystem/plans/metasystem-stop-verb-design.md: it must choose the status continuation rule and name its exit-code and partial-output fixture in the existing stop-transition package bed.

EVIDENCE: Section 18.3 lines 1485-1489 requires exit 1 and a preserved partial report on inventory or fence-read failure. Section 18.6 lines 1532-1539 names only successful-read status assertions. The current Status implementation calls an inventory operation that returns no partial collection after the first family error, while the existing malformed-fence status test expects exit 0. Both implementation behavior and test truth therefore depend on an unstated choice.

# What to decide

For each finding, amend section 18 so the rule is decided rather than
described. Where the critic shows an invariant that cannot hold in a
failure case, either bound the invariant to the cases where it does hold
and say what happens in the others, or change the rule so it holds
always: say which you chose and why.

# Constraints, binding

- Do NOT enlarge the slice, and do NOT introduce a record, verb,
  barrier or pass. Section 18's own claim to add no machinery is what
  the critic checked hardest; keep it true.
- The acceptance surface stays as it is: proofs must be nameable in the
  beds section 10 already assigns.
- Touch only the design page.
- Keep the parts of section 18 the critique did not fault; the critic
  accepted the rest, including its identity rule and its treatment of
  adopted and untracked processes.

# Constraints

Wall-clock budget: 45 minutes. Return per the schema. Gap rule: stop and
report a gap; never fill it silently.
