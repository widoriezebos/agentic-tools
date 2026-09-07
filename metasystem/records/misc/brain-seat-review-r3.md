# brain seat build code review — round 3 and last (chain brain-build1b-20260907, round-8 tree)

Chain: round 7 folded the round-2 review, round 8 re-declared the tree after round 7's return was voided for a malformed entry -> reviewed tree 3919cf73f5dfbc3bb95fcab5c8b14fed5d856c29 -> critic brain-review6-20260907 (Opus 5, code-critic, read-only). ZERO material findings; 8 notes, each stated by the critic as not material. This is the review that covers the landing tree.

## N-1 — low, material=False

CLAIM: The new row for the BRAIN_REFUSED refusal code in the refusal register points at a source line that renders no refusal. The row records the emitting site as brain.go line 420, but line 420 in metasystem/internal/brain/brain.go is the closing brace of the loop that finds the packet's standing-instruction heading. The two functions that actually render the refusal text, RemedialRefusal and Fence, begin at lines 425 and 429. The code word itself is emitted in cmd/metasystem/delegate.go and in dispatch.sh, not in the brain package at all. The design's third addendum asked for 'the line that renders the refusal in brain.go'.

EVIDENCE: Read of metasystem/internal/refusal/register.go line 41 and of metasystem/internal/brain/brain.go lines 415 to 432. Not material: I read both tests in metasystem/internal/refusal/register_test.go and neither checks the Site field. One test checks that every refusal token found in the tree has a row, the other checks that a row's named override is a real goal verb, which skips this row because its override is a brain verb. The register's own opening comment already says the list is kept by hand and no test proves it complete. Nothing behaves differently and no gate is weakened; it is a stale pointer in documentation.

## N-2 — low, material=False

CLAIM: A running validation gate is not one of the obstacles that stop a brain declaration. The quiescence check reads the scanner's busy list and handles only the three kinds job, run and mission, so a busy item of kind 'gate' is silently ignored. Declaring the brain while a gate run is live in that checkout succeeds, and every turn end on the new brain then prints STILL WORKING naming that gate until the gate finishes.

EVIDENCE: Read of brainDeclarationObstacles in metasystem/cmd/metasystem/brain.go lines 122 to 135, whose switch covers job, run and mission, against metasystem/internal/report/scan.go lines 60 to 69, which adds busy items of kind 'gate' from the gate-run survey and one more under the METASYSTEM_GATES_RUNNING fixture variable. Not material: the design lists exactly four quiescence obstacles, a held claim, an in-flight job, a live run and an active mission, and the build implements exactly those four. A gate is also self-clearing and is not one of the acts the fence blocks, so nothing is stranded.

## N-3 — low, material=False

CLAIM: One refusal does not end with a node command. Asking the fence about an act name it does not know returns the sentence 'this checkout is declared the brain; act <name> is fenced' with no command for a node to run.

EVIDENCE: Read of the default branch of Fence in metasystem/internal/brain/brain.go lines 450 to 452, and ran `metasystem brain fence --root <scratch> --act totally-made-up` on a declared-then-corrupted scratch checkout. Not material: no guarded act reaches that branch. Every caller passes one of the six named acts, and the only way to reach the default is by typing an invented act into the diagnostic verb by hand. On a corrupt record the branch is unreachable at all, because the remedy line returns first.

## N-4 — low, material=False

CLAIM: Posting the fleet status from a declared brain that has never booted records the status as published even though that post did not carry the brain's visibility line. The status composer only adds the line when the status file already exists and its stored line matches the current declaration, but the posting verb writes and then marks that file regardless. The first publication of the line can therefore be delayed by up to the four-hour cadence window.

EVIDENCE: Read of runChannelStatus in metasystem/cmd/metasystem/channel_verbs.go lines 87 to 100, which calls WriteStatus and then MarkStatusPosted after the post, against ComposeStatusReport in metasystem/internal/channel/report.go lines 95 to 102, which requires an already-matching status file to include the line. Not material: the behaviour is deliberate and pinned by the test named TestChannelStatusPostSeedsAnUnbootedBrainStatus, and the case is nearly unreachable in the wired path, because both brain boot and every brain turn end write the status file before the hook ever posts.

## N-5 — low, material=False

CLAIM: The supervision hook's own eight-second bound on the boot verb signals only the boot process, not the process group that boot put its input-reading child into. If the hook ever has to kill the boot parent, that child is orphaned and, if it is blocked on an unreadable input, stays alive.

EVIDENCE: Read of metasystem/scripts/agents/supervision-hook.sh, where the timeout path sends TERM and then KILL to the single recorded boot process id, against composeBrainBoot in metasystem/cmd/metasystem/brain_boot.go line 106, which starts the child with its own process group and signals the negated group id at its own deadline. Not material: the verb's own five-second deadline plus a two-hundred-millisecond grace returns well before the hook's eight seconds, so the hook's kill is a second line of defence that should not fire; and the child is a read-only reader of local files. The stalled fixture proves the verb's own kill leaves no surviving child.

## N-6 — low, material=False

CLAIM: In one corner the narrator digest section can be trimmed away to nothing and still count as cut, which advances the brain's digest cursor to the end of the log while the session receives neither a digest line nor any sign that a digest existed.

EVIDENCE: Read of fitBrainSection in metasystem/cmd/metasystem/brain_boot.go lines 283 to 285, which returns empty text with the cut flag set when even the section heading plus its 'older lines cut' notice will not fit, and of line 211, where digestEmitted is true for a cut section. I bounded the reach arithmetically: the digest always receives at least thirty-five percent of the budget left after the fixed opening text, so reaching this needs a bound near the 2048-byte minimum together with a corrupt record and a checkout path long enough to make the twice-repeated remedy line dominate. Not material: at the only declared bound in the tree, 10000 bytes for the Claude runtime, the digest gets thousands of bytes; and the design states outright that a cut digest still advances the cursor because the log is a durable record.

## N-7 — low, material=False

CLAIM: Two fixture rows are weaker than their siblings. The breach-stop exemption row asserts only that the command fails for its own reasons and that the output contains no brain refusal, so it would also pass on the tree before this change and can only ever catch a future over-fence. The actor-seam coverage row compares a text listing of every actor and caller-classification site against a literal allow-list, so any unrelated future change that adds such a site fails it until someone edits the list by hand.

EVIDENCE: Read of the brain-breach-stop-exempt block in metasystem/scripts/agents/dispatch-fixtures.sh and of the brain-actor-seam-coverage block in metasystem/scripts/agents/brain-fixtures.sh. Not material: both shapes are what the design asked for. The exemption row exists precisely to be a negative guard, and the design names the seam row as the way 'inherits by construction' stays checkable, accepting its allow-list. Every other new row fails on the pre-change tree and observes designed behaviour rather than the presence of the feature.

## N-8 — low, material=False

CLAIM: The boot deadline line is printed whenever a section file is missing, including when the input-reading child died at once rather than running out of time, so its wording, 'not read within N milliseconds', can misdescribe the cause.

EVIDENCE: Read of composeBrainBoot in metasystem/cmd/metasystem/brain_boot.go lines 153 to 156, which composes the line from the missing-section list without consulting whether the deadline actually fired. Not material: the behaviour is deliberate and asserted by TestBrainBootKeepsPhaseOneWhenOptionalInputChildFails, which makes the child exit immediately and requires the same line. The design's own table of one line per failing input has no separate row for a child that fails to start, and the remedy the line offers, running boot by hand, is the right one either way.

## Gaps the critic named

- The bash fixture beds could not run here. The sandbox has no writable location that bash 3.2 can use for the heredoc temporary files these scripts need, and several rows launch background channel servers and fake runtimes. The brain, dispatch, land, goal-cli and supervision-hook rows therefore rest on reading their code against the design's fixture table and on the host's own gate receipt reported in the brief. I substituted direct engine probes on scratch checkouts for the boot, fence, show and withdraw behaviour.
- The reviewed tree hash could not be recomputed, because writing a tree object requires write access to the shared git object database, which the sandbox denies. I proved equality a different way: every one of the 41 changed paths in the dispatcher's diff has an explicit post-image blob hash, and all 41 match the worktree file exactly.
- The runtime's own consumption of the session-start context field and of the compact lifecycle source was not observed. The design itself lists this as unprovable inside a sandbox and names a human opening and compacting a declared seat as the check.
- I did not reproduce the two host reds the brief describes (the pre-existing dispatch scenario's budgetless refusal and the lease text when a fixture commits from an unauthenticated caller), so my agreement that they are the host's own caller classification and reproduce on main rests on the brief's account, not on my own run.
- Tool names were unobserved by this launcher, and the runtime notice classifies this context as advisory: it cannot prove context isolation or independent examination.

## Coordinator disposition (m1, 2026-09-07)

The tree lands. No finding is material and none is a defect the critic
asks to be fixed before landing; each note names behaviour the design
chose, a fixture shape the design asked for, or a cosmetic imprecision.
Two are worth carrying forward rather than dropping, and both go to the
backlog rather than into this landing:

- N-1, the refusal register's Site field for BRAIN_REFUSED points a few
  lines off the function that renders the refusal, and nothing tests
  that field. A register whose sites drift is a register that stops
  helping a reader; the row is corrected when the next chain touches it.
- N-2, a live validation gate is not one of declare's quiescence
  obstacles, so declaring the brain during a gate run leaves the seat
  printing STILL WORKING until the gate ends. Harmless and self-clearing,
  and the remedy is one case in the obstacle switch.

The rest are recorded here as read and left as built: the deliberate
status-post cadence, the hook's own bound on the boot verb, the digest
corner where a section is cut to nothing, the two negative fixture
shapes, and the deadline line's wording.
