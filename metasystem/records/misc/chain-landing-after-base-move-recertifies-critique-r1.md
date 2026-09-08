# Chain landing after a base move: design critique round 1

Critic clbm-crit1 (design-critic, codex gpt-5.6-sol, xhigh, read-only) on revision 1 of plans/chain-landing-after-base-move-recertifies-design.md, written by Astra (codex gpt-6-astra, xhigh) the same hour. Five material findings, four high, and two notes. Every material finding is a place where two implementers would build different things, which is the standard this critique was sent to apply.

## CLB-01 - high - material=True

CLAIM: The headless failure path has no named state owner. Revision 1 says an unattended node uses an existing stream-park mechanism and none exists in the tree, so an implementer must guess between goal park, mission parking, a scheduler record and a diagnostic exit. Those differ materially: goal park clears the claim, mission parking is unavailable to this mission-free chain, and an exit alone is indistinguishable from a stall. So the unattended node does not yet have exactly one lawful durable move, and the transport canary has no park record to observe.

EVIDENCE: A literal search for the term returns only the design page itself. internal/goal/verbs.go:1204-1270 shows goal park as a ledger transition that records a reason and clears claim binding. scripts/agents/land.sh has no parking transition: it reports a failed step and exits.

## CLB-02 - high - material=True

CLAIM: The operational content rule does not exclude every both-sided binary change the page claims to exclude. All three blobs need only lack a NUL byte, bytes are expressly allowed to be invalid UTF-8, and Git is forced through --text. A newline-delimited binary blob with high-bit bytes and no zero byte is therefore assigned line hunks and can pass when the two sides touch distant records, so the implementer would admit a file shape the page calls unsupported.

EVIDENCE: Design line 79 requires only regular modes and the absence of NUL and says bytes need not be UTF-8, then says binary blobs are unsupported; lines 81-88 force --text, which treats every file as text. A both-sided blob containing bytes FF and FE with newline separators and no 00 satisfies every stated check.

## CLB-03 - high - material=True

CLAIM: The required execution evidence for an area-width recertification has no defined command. "The same required checks for that chain's gate width" identifies the fixed full-width battery, and the system defines no area-width receipt command, so one implementation can accept any successful command including `true` while another tries to recover the original chain's focused checks. Those enforce different safety contracts.

EVIDENCE: docs/orchestration.md:76-78 fixes only the full-width battery. internal/landing/receipt.go:55-57 accepts any non-empty command and lines 290-310 validate success and tree posture without identifying an area check. Design line 153 nevertheless says execution evidence remains necessary, and its canary at line 190 picks a meaningful tiny test without defining how production selects or binds that command.

## CLB-04 - medium - material=True

CLAIM: The canonical diff grammar omits Git's legal optional hunk-header suffix, so a strict parser following the shown shape can reject ordinary middle-of-file insertions while a tolerant parser accepts them. The design requires two implementations to agree and its DONE case includes ordinary well-separated edits, so this cannot stay an implicit parser choice.

EVIDENCE: With the exact option set the page specifies, Git emitted a hunk header carrying a trailing section suffix for an insertion after a line. Design line 92 specifies only the unsuffixed shape and line 100 makes malformed output a refusal. The page also relies on Git's no-final-newline marker without defining its parser treatment.

## CLB-05 - high - material=True

CLAIM: The refusal canaries do not prove the forged-record and stale-target defences for the right reasons. The candidate-tamper subtest is properly isolated with an otherwise valid receipt, but the changed-target and changed-proof checks require only "not bar a". An implementation that falls back to the old chain path, rejects a stale receipt, or checks only the record's self-hash satisfies that observation while omitting the hard refusal and the full proof recomputation the page requires.

EVIDENCE: Design line 151 requires an invalid explicitly named record to hard-refuse without fallback; line 190 merely says a changed target and changed proof payload must not return bar a. A byte-flipped payload tests corruption, not a forged record whose attacker recomputed the input and record digests, and a changed target can be rejected by an unrelated receipt or worktree-posture check.

## CLB-06 - low - material=False

CLAIM: A semantically coupled but textually disjoint pair can pass although no critic read the merged tree, for example main changing a constant near the top of a file while the chain changes a distant use of it. The goal explicitly permits a mechanical proof, and the page already states that it preserves text rather than semantics.

## CLB-07 - low - material=False

CLAIM: All three named canary observations are unreachable in the current tree because their test functions do not exist yet. Revision 1 identifies them as future implementation tests.

## Coordinator's reading (m1b, 2026-09-08)

All five material findings accepted; revision 2 folds them and closes
the page. Both notes are recorded and not actioned: CLB-06 is the
residual risk the goal knowingly accepts by asking for a mechanical
proof, and the page already says the proof preserves text rather than
semantics, and CLB-07 is what "specified, not written" means.

Two of the five are the same failure mode as the design's own subject.
CLB-02 and CLB-04 are places where the page describes a mechanism in
prose that the real tool does not implement the way the prose assumes:
--text makes every file textual, and Git's hunk headers legally carry a
section suffix the stated grammar rejects. A design that turns on exact
diff bytes has to be written against the tool's actual output, and Sol
proved both by running Git rather than by reading about it.

CLB-05 is the third design in this program whose canary could not fail
for the right reason. The first was caught by a read, the second was
caught by a read, and the third is caught here. That is now a pattern
worth naming rather than a coincidence: a refusal canary written as
"must not pass" is nearly always too weak, because many wrong
implementations also do not pass. Every refusal observation this program
writes from now on names the reason the refusal must carry and keeps
every unrelated prerequisite valid, so the only thing that can produce
it is the defence under test.

This is design cycle two and the last for this page. Revision 2 goes to
the build behind the fixtures it names, with no second critique, per
D81: two prose budgets exhausted means build behind fixtures rather than
open a third. If Astra reports that something cannot be settled without
another revision, that decision comes to Wido rather than being taken
here.
