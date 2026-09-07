Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Design round: decide the fifth read's three findings (goal metasystem-stop-verb)

You are deciding design, not implementing it, and not critiquing it. The
orchestrator is deliberately not deciding these itself: Wido's word today
is that design belongs to a delegate and the coordinator only
coordinates.

The design under amendment is metasystem/plans/metasystem-stop-verb-design.md,
landed on main as commit 43741a4a; its SHA-256 is
bcc2579d7691cae5c2d1b76693580abc3efc6ea0e7489a7e2b87e1d230acbefb. Read it
whole, and in particular sections 2, 6, 9, 13.1, 14.1, 14.6, 15.1, 15.2,
16.2 and 17, which the findings touch. Sections are cumulative: a later
section wins over an earlier one, and your amendment is section 18.

The verb is built and its whole acceptance is green: eight fixture
scenarios across three beds and the package matrix, in the
orchestrator's own environment. Three critique rounds are folded. This
is the fifth independent read, and it found three material items and two
notes, all about what a PERSON is told and one of which can leave a
checkout stopped with no printed way out.

# The findings, verbatim from the read

### SVC8-01 (high, material=True)

CLAIM: A human arm can be refused with an unactionable error and told to run arm again, leaving the checkout stopped with no printed way out. When the previous stop recorded a job owned by another machine as a survivor, arm probes that survivor by re-reading the job's record file. The code that does this, remoteJobStillOpen in the stop-transition package, tries to treat a missing record as 'the job is gone' by testing the error against the standard file-not-found error. That test can never succeed, because the reader it calls wraps every failure with the formatting verb that discards the original error instead of the one that preserves it. So a missing or unreadable record for a remote-machine survivor makes arm return a raw file error. Because that error matches none of arm's typed survivor errors, the refusal's second line falls through to the default, which is the arm command itself. A person who follows arm's own advice loops forever. Only a second metasystem stop clears the state, and nothing on the page says so. This is the wedge the brief names, sitting inside the one act Wido's requirement says must be able to clear a stop, and it also breaks design amendment 15.2, which forbids a refusal from naming the command that just produced it.

EVIDENCE: Proven by execution. In a scratch copy of the reviewed tree I wrote a probe that publishes a durable fence record in phase stop-incomplete whose single survivor is a job with component 'job', id 'remote-1' and a foreign machine id, then calls the arm path over it. The probe printed: arm refusal = cannot read the job record: open <tmp>/artifacts/agents/jobs/remote-1.json: no such file or directory. That string is the wrapper text from ReadRecordObject in metasystem/internal/dispatch/record.go line 628, which proves the not-exist branch at metasystem/internal/stoptransition/families.go line 281 did not fire. Reading confirms why: line 628 uses fmt.Errorf("cannot read the job record: %v", err), so errors.Is cannot see the underlying file error. In metasystem/internal/stoptransition/transition.go the probe error short-circuits at lines 502-505 (return 0, probeErr) before any of the typed survivor errors at lines 512-525 can be chosen, and metasystem/cmd/metasystem/process_verbs.go line 209 then supplies the default second line 'run: metasystem arm --repo <checkout>'. A malformed rather than missing record takes the same path, since remoteJobStillOpen returns the read error unchanged at

### SVC8-02 (medium, material=True)

CLAIM: The stop report prints the same surviving thing up to five times while its closing line says one thing was not stopped. After the fence closes, stop acts on and prints its inventory in the main family pass, then in each of three late-arrival passes, then in a final sweep. Anything still live at each of those points is inventoried again, acted on again, and printed again, with no de-duplication of the report lines. The durable record is correct, because survivor entries are de-duplicated, so the count in the closing line and the number of lines above it disagree by construction. The clearest reachable cases are a job owned by another machine, which is reported without ever being signalled and therefore never leaves the inventory, and a run held under adopted custody, which the design says is deliberately never signalled and so is printed once by the main pass and once more by the final sweep. Design section 6 asks for one line per thing in the order acted on, and design amendment 14.6 exists because the closing line is what a person reads; a page that lists one survivor five times and then says 'one not stopped, listed above' undermines both.

EVIDENCE: Proven by execution. In a scratch copy of the reviewed tree I wrote a probe with one process family whose inventory always returns the same live item and whose stop operation always reports it as not stopped. The run produced five calls to the family's stop operation, five identical lines reading 'NOT STOPPED job remote-1 running machine other: owned by another machine; did: nothing', and the closing line 'stop incomplete for /checkout; 1 not stopped, listed above; run: metasystem stop --repo /checkout', with one entry in the durable record. The mechanism is in metasystem/internal/stoptransition/transition.go: the main family loop at lines 208-230, the three-iteration late-arrival loop at lines 232-265 whose eligibility test at line 239 always admits an item whose recorded fence generation is at or below the one stop just closed, and the final sweep at lines 270-283. Lines are appended by appendOutcomeLines at line 345 with no de-duplication, while survivors go through appendUniqueSurvivor at line 830, which does de-duplicate. The existing regression test TestSurvivorDoesNotStopLaterFamilies at metasystem/internal/stoptransition/transition_test.go line 97 misses this because its sc

### SVC8-03 (medium, material=True)

CLAIM: status tells a person that a checkout with a failed stop is simply stopped, and points them at a command that will refuse. When the durable record is in the stop-incomplete phase with recorded survivors, status prints the same closing sentence it prints after a clean stop, including 'start again: metasystem arm', and exits 0. It reads the record's state but ignores its phase and its list of survivors. Arm over that same record refuses and points at metasystem stop, so status and arm send a person back and forth. This matters most for the survivors that status's own inventory cannot show: a creator whose liveness could not be established and a family whose records could not be read are recorded in the fence but are not live processes status would list. Design amendment 14.6 decided that an incomplete stop must never close by saying the checkout is stopped; that decision was applied to stop's closing line and not to status's.

EVIDENCE: Proven by execution. In a scratch copy of the reviewed tree I published a fence record in state closed, phase stop-incomplete, generation 3, carrying one survivor (component 'job', id 'remote-1', machine 'other'), then called the status path with an empty inventory. It printed exactly three lines: 'checkout /checkout', 'nothing is running', and 'stopped since 2026-09-07T10:00:00Z by stop pid 99; start again: metasystem arm --repo /checkout', with exit code 0 and no mention of the recorded survivor or of the incomplete phase. The code is metasystem/internal/stoptransition/transition.go lines 164-174, where the closed branch reads only the record's changed-at timestamp and actor and never inspects its phase or its survivor list. Compare the stop closing line at lines 307-315, which does split on the survivor count, and design amendment 14.6 at metasystem/plans/metasystem-stop-verb-design.md lines 1097-1104.

### SVC8-N1 (low, material=False)

CLAIM: Two fixture scripts in the diff replace a hard-coded approval review date, 6 September 2026, with a date computed as tomorrow in coordinated universal time. That date is now in the past, so the fixtures would refuse without the change. The repair is unrelated to the stop verb and belongs to whatever goal owns those fixtures, but it removes a time bomb rather than weakening a check, and both files are inside the declared boundary. Recorded, not work.

EVIDENCE: metasystem/scripts/agents/channel-fixtures.sh gains a helper that computes tomorrow's date and uses it at two approval call sites; metasystem/scripts/agents/goal-cli-fixtures.sh takes the same treatment. Both paths appear in the 66-entry declared diffBoundary, and the comparison of declared paths against changed paths found no undeclared change.

### SVC8-N2 (low, material=False)

CLAIM: Every live holder of the transition lock is reported to the caller as a stop, whatever verb actually holds it. The error type carries only a process id and renders as 'a stop by pid <n> holds the checkout', so a caller blocked by a concurrent arm, or by the human mission start and resume handover that also takes this lock, is told a stop is running. Arm is short-lived and design amendment 16.2 only requires that a live holder be recognised as a holder rather than as a timeout, which this does, so nothing misleads anyone for long. Recorded, not work.

EVIDENCE: metasystem/internal/stoptransition/transition.go lines 543-547 define StopInProgressError with a fixed sentence naming a stop, and the acquire helper at lines 665-672 returns it for any live holder regardless of the verb label the lock records. The mission handover reaches the same lock through OpenFence at line 460, called from missionFenceBeforeArm in metasystem/cmd/metasystem/process_verbs.go line 348.

# What to decide

Write section 18 of the design, deciding each of SVC8-01, SVC8-02 and
SVC8-03, in the page's voice and at the page's level: what the system
does and why, not how to code it. For each, say plainly what a person
sees and what the durable record says, so a builder has no room to
invent. Where a finding exposes a rule the page already states but does
not hold to, say which rule and make it hold everywhere rather than in
the branch the read happened to find.

Two notes the read marked immaterial are yours to rule on in the same
section, briefly: whether every holder of the transition lock should be
reported as a stop when it may be an arm or a mission handover, and
whether the fixture review-date repair inside this diff belongs here.

# Constraints, binding

- Do NOT enlarge the slice. The fleet form and the five escalation
  scenarios are deferred to other goals and stay deferred.
- Do NOT introduce a new mechanism if an existing rule of the page,
  applied consistently, settles the finding. The last mechanism this
  page gained for correctness (the completeness barrier of 13.1) became
  its most defect-dense area and section 17 simplified it away; that
  history is a warning about your own additions.
- The acceptance surface stays as it is: no scenario is renamed or
  removed, and any new proof you require must be namable as a scenario
  or a package test in the beds section 10 assigns.
- Say what you decided and why in the page. Do not touch any other file.

# Constraints

Wall-clock budget: 60 minutes. Return per the schema. Gap rule: stop and
report a gap; never fill it silently.
