# metasystem stop: fourth code critique of round 21 (chain stopverb-build1)

Written out of order, on 2026-09-08, to close a hole in this series. Critic stopverb-crit8 (code-critic, Opus 5) read round 21 on reviewed tree 3ca4bd2e709a59a0f59d134cabf66621a224b60b and returned three material findings and two notes. Its findings were disposed at the time by routing them to a design round rather than straight to a fold, which is how sections 16, 17 and 18 of the specification came to exist, and the register entry was never written. The numbering of this file follows the order the registers were written, not the order the reads happened: -r7.md covers the read of round 20, this file covers the read of round 21, and -r8.md and -r9.md cover the reads of rounds 24 and 25.

## SVC8-01 - high - material=True

CLAIM: A human arm can be refused with an unactionable error and told to run arm again, leaving the checkout stopped with no printed way out. When the previous stop recorded a job owned by another machine as a survivor, arm probes that survivor by re-reading the job's record file, and the code that does so tries to treat a missing record as "the job is gone" by testing the error against the standard file-not-found error. That test can never succeed, because the reader wraps every failure with the formatting verb that discards the original error. A missing or unreadable record for a remote-machine survivor therefore reads as a live survivor forever.

EVIDENCE: Proven by execution. In a scratch copy of the reviewed tree the critic published a durable fence record in the stop-incomplete phase whose single survivor was a job with a foreign machine id, then called the arm path over it. The refusal printed was the wrapper text from the record reader in internal/dispatch/record.go, which names no path the person can act on, and arm's remedy pointed back at arm.

## SVC8-02 - medium - material=True

CLAIM: The stop report prints the same surviving thing up to five times while its closing line says one thing was not stopped. After the fence closes, stop acts on and prints its inventory in the main family pass, then in each of three late-arrival passes, then in the final sweep, with no de-duplication of the report lines. The durable record is correct, because survivor entries are de-duplicated, so the count in the closing line and the number of lines above it disagree by construction.

EVIDENCE: Proven by execution. A probe with one family whose inventory always returns the same live item and whose stop always reports it not stopped produced five calls to the family's stop operation, five identical NOT STOPPED lines, and a closing line reporting one survivor.

## SVC8-03 - medium - material=True

CLAIM: status tells a person that a checkout with a failed stop is simply stopped, and points them at a command that will refuse. With the durable record in the stop-incomplete phase and survivors recorded, status prints the same closing sentence it prints after a clean stop, including the arm remedy, and exits 0. Arm over that same record refuses and points at stop, so status and arm send a person back and forth.

EVIDENCE: Proven by execution. With a fence record in state closed, phase stop-incomplete, carrying one survivor on another machine, the status path with an empty inventory printed only the checkout line, "nothing is running", and the clean-stop closing sentence with exit 0, mentioning no survivor.

## SVC8-N1 - low - material=False

CLAIM: Two fixture scripts in the diff replace the hard-coded approval review date of 6 September 2026 with a date computed as tomorrow. That date is now in the past, so the fixtures would refuse without the change. The repair belongs to whatever goal owns those fixtures, but it removes a time bomb rather than weakening a check.

EVIDENCE: The channel and goal-cli fixture scripts each gain a helper computing tomorrow's date and use it at their approval call sites. Both paths are inside the declared diff boundary.

## SVC8-N2 - low - material=False

CLAIM: Every live holder of the transition lock is reported to the caller as a stop, whatever verb actually holds it, so a caller blocked by a concurrent arm or by the mission handover is told a stop is running. Arm is short-lived and section 16.2 only requires that a live holder be recognised as a holder rather than as a timeout.

EVIDENCE: internal/stoptransition/transition.go defines the in-progress error with a fixed sentence naming a stop and returns it for any live holder regardless of the verb label the lock records. The mission handover reaches the same lock through the open-fence path.

## Coordinator's reading (m1b, 2026-09-07, recorded 2026-09-08)

All three material findings accepted; both notes recorded, not worked.
This read is the one that changed how the rest of the chain was run.

Two of the three are the same fault in different clothes: the page and
the durable record disagree about what a stop achieved, and the person
is bounced between two commands that each point at the other. Rather
than fold three patches, the findings went to a design round on the
Codex lane, which produced sections 16, 17 and 18: actionable refusals
that name the file and the repair, one final line per identity, and a
status and up page that distinguish a closed fence from a completed
stop. Rounds 22 to 24 built that text.

The three notes and findings of this read are also why the seat stopped
writing design itself. Wido's instruction that day was explicit: the
coordinator coordinates, and design, design critique, code and code
critique all belong to delegates.
