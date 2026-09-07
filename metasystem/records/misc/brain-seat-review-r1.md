# brain seat build code review — round 1 (chain brain-build1b-20260907, round-4 tree)

Chain: build brain-build1b-20260907 rounds 1-4 (Sol) -> reviewed tree 50f6b6e1f0d35c545d8a8ee742c1a3b9912a0c8f -> critic brain-review3-20260907 (Fable, code-critic, read-only; a first attempt died on the account's 429 session limit). 5 material findings, 6 notes. The coordinator carried the return here verbatim because the critic wrote no register file.

## F-1 — high, material=True

CLAIM: The session-start object the hook relays lacks the runtime event name the design requires, so the standing instruction may never enter the model's context. The design's channel decision says the object also carries hookSpecificOutput.hookEventName equal to SessionStart because the runtime requires it, and the runtime's documented output shape lists that member beside additionalContext. The hook builds only the dotted field from the registry declaration and adds nothing else, and no fixture or test asserts the member. If the runtime rejects the object, a declared brain opens uninstructed, which is the design's stated reject condition, and nothing on the screen says so.

EVIDENCE: Reproducing emit_start_payload with the built engine's json object verb yields {"systemMessage":...,"hookSpecificOutput":{"additionalContext":...}} and parsing shows additionalContext as the only key under hookSpecificOutput. grep for hookEventName across the tree finds it only in the three design records. The supervision-hook fixture reads hookSpecificOutput.additionalContext and never checks hookEventName. Design revision 3 section 2, the Channel paragraph, names the member as required.

## F-2 — high, material=True

CLAIM: On a template-layout checkout, which is what this repository's own seats are, the hook boots the brain with the git toplevel as the packet checkout, so the payload reports the role packet missing and the digest section reads a log that does not exist. The hook resolves repo as the git toplevel and state_root as the installation directory, then runs brain boot with --repo set to the toplevel. brain.PhaseOne joins the packet's relative path onto that directory and the digest package joins records/narrator-digest.log onto it. Both files live under the installation directory in this layout. The goal's whole deliverable, the standing instruction loading unprompted with the narrator digest as input, is therefore unmet on the seat it was built for, and the hook fixture cannot see it because its bed is adopted-style with the two roots equal.

EVIDENCE: Ran the built engine: brain boot --root <scratch root with a corrupt record> --repo /Users/wido/LocalStorage/GitHub/agentic-tools printed THE ROLE PACKET IS MISSING at /Users/wido/LocalStorage/GitHub/agentic-tools/records/misc/fleet-coordinator-brain-role-packet.md as its third payload line, while --repo .../agentic-tools/metasystem printed the packet's first heading. ls shows no records directory at the toplevel and the narrator log at metasystem/records/narrator-digest.log. supervision-hook.sh lines 398 to 411 set repo to the toplevel and state_root to the installation in template mode, and the boot call passes --repo "$repo"; brain.go line 375 joins the packet path onto repo; digest.go Path joins records onto the same root.

## F-3 — medium, material=True

CLAIM: When the boot verb fails, exceeds the hook's eight-second bound, prints an invalid object, or the start-context declaration is unreadable, the hook prints its failure notice and exits before the steward-pending notice, the identity check, and the up leg. Every seat runs this verb at every session start, resume, clear, and compaction, so a failure in the brain machinery leaves a node's session unarmed and its pending steward incidents unsurfaced. The design says the start path collects every message and prints one object at exit, and section 6 says a brain that is down changes nothing a node does; the early exit turns a notice into a lost arming.

EVIDENCE: supervision-hook.sh start block: if brain_boot_valid is not true the hook prints the json object with the failure text, removes its temp files, and runs exit 0. The steward pending line, the identity refusal, and the up invocation that calls emit_start_payload all come later in the file. The brain-boot-verb-fails fixture asserts only the one notice and never checks that arming still ran.

## F-4 — medium, material=True

CLAIM: The named fixture brain-absent-node-proceeds is red on this tree in its landing leg. Clone B lands payload.txt with no chain or direct-fix declaration, which the landing driver refuses as missing-declaration on every tree. Addendum 4 decides the leg is a register-carriage landing of one new record file, and that decision is not built here, so the row that proves never-a-bottleneck for landings does not pass.

EVIDENCE: land-fixtures.sh brain-absent-node-proceeds branch: printf 'node landing' > payload.txt followed by land.sh -m <message> --skip-transport payload.txt with no --direct-fix or --chain flag. The builder's round-4 return and the host both report the promoted refusal with code missing-declaration for this row. Design revision 3 addendum 4 names the expected form land.sh -m <message> --direct-fix register-carriage <records path>.

## F-5 — low, material=True

CLAIM: The shell fence in dispatch.sh fails open when the fence verb itself fails. The helper brain_fence_outcome is always invoked as a command substitution inside an if condition, so its die on an engine exit other than 0 or 2, or on a json get failure in the fenced branch, only ends the subshell with status 1, which the caller reads as not fenced, and the guard, dispatch_job, follow_up, cancel_job, close_chain, and reap_jobs all proceed. land.sh and commit.sh handle the same case by exiting 1. The design asks for fail-closed at every guarded act on an unreadable declaration; here an unreadable fence answer is treated as an open gate.

EVIDENCE: Ran dispatch.sh dispatch and dispatch.sh reap --job nothing with METASYSTEM_BIN pointing at a wrapper whose brain fence exits 1: stderr shows 'brain fence failed for dispatch' and the script then prints the legacy REFUSED-REQUEST object rather than stopping; for reap it prints 'brain fence failed for reap' and continues into reap_jobs and the checkout lease checks. dispatch.sh lines 26 to 36 define the helper with die 1 on a non-2 non-0 exit; every call site is of the form if brain_outcome=$(brain_fence_outcome act); then.

## N-1 — low, material=False

CLAIM: On a corrupt declaration the request seam refuses every verb that passes through it, including agent-only verbs with no --by such as goal open, release, park, and set-next. The design's step two and three fire only on a classification error or on a non-empty --by from a non-human class, and its remedy line lists dispatch, landing, claims, cancels, closes, reaps, and a human's word, not drafting. The brain therefore cannot draft a backlog item while its own record is unreadable; the state needs one human command to repair, so this is a conservative deviation rather than a shipped hole.

EVIDENCE: goalsync_mutations.go brainHumanWordClassification: after the classification-error branch, if brainState.State == brain.Corrupt it returns the remedy refusal unconditionally, before the by != "" test.

## N-2 — low, material=False

CLAIM: Two scanner changes reach every seat, not only the brain. openwork.go now counts pending-setup job records as in flight, so a stale setup husk marks a node's turn as STILL WORKING and suppresses its goal clause until the reaper clears it; the design implied it by making declare use the scanner's own read but never named the cross-seat effect. scan.go now projects the ledger on every new-world checkout to find drafts and appends any resolution error to Unreadable, which vetoes the all-clear on a seat with no nickname or a failed projection. The design promised that for an undeclared checkout the two new fields are filled and not displayed so no other seat's verdict changes.

EVIDENCE: openwork.go inFlightStatus gained "pending-setup"; scan.go scanDrafts appends "draft scan: " errors to result.Unreadable for every checkout where goal.NewWorld is true.

## N-3 — low, material=False

CLAIM: The brain-actor-seam-coverage row is a sha256 of ripgrep output rather than the allow-list the design names, and its second pattern also matches equality comparisons, so about forty unrelated 'r.Actor.Human == ""' lines in the goal package sit inside the digest. Any edit to those lines, or a reformat, fails the row without a new actor site appearing, and a reader cannot tell what is allowed without rerunning the search. The row also depends on rg being installed.

EVIDENCE: brain-fixtures.sh lines 317 to 326; rerunning the same search shows the matched lines are dominated by Actor.Human == "" comparisons in verbs.go, approval.go, split.go, recover.go, stop.go, and reconcilepub.go.

## N-4 — low, material=False

CLAIM: Weak rows. brain-delegate-refuses and brain-cancel-close-reap-refuse exercise only the Go fence in runDelegate and the top-of-script legacy guard, because every dispatch.sh call unsets the internal marker; the router-level fences inside dispatch_job, follow_up, cancel_job, close_chain, and reap_jobs, which are the steward and internal path the design added them for, are never reached. The dispatch leg of brain-absent-node-proceeds asserts only that the refusal string is absent from a no-argument dispatch and never starts the fake-runtime delegate the design row describes. brain-boot-bound checks the packet's first heading, not that the packet bytes are intact. The hook rows build only an adopted-style bed and never assert the event-name member.

EVIDENCE: dispatch-fixtures.sh brain branch: every dispatch.sh invocation uses env -u METASYSTEM_DELEGATE_INTERNAL; the absent-node leg checks [[ "$node_dispatch" != *BRAIN_REFUSED* ]] only. brain-fixtures.sh line 186 greps for the heading line. supervision-hook-fixtures.sh comment 'build one declared adopted-style checkout'.

## N-5 — low, material=False

CLAIM: A failure of the boot-inputs child that is not the deadline, for example a section file write failing or the child dying under a signal, makes the whole boot verb exit 1, and the hook then prints its failure notice and injects nothing. The phase-one text that the design says nothing later can cut is discarded with it. Treating such a child failure like the deadline, with every section skipped and the phase-one text kept, would honour the design's guarantee.

EVIDENCE: brain_boot.go composeBrainBoot: else if err != nil { return brainBootOutput{}, fmt.Errorf("optional-input reader failed ...") } after the wait, where err is the child's exit error.

## N-6 — low, material=False

CLAIM: Small failure-path and portability nits in the visibility plumbing. channel status --post on a declared brain that has never booted or ended a turn posts successfully and then exits 1 because MarkStatusPosted cannot read a status file that does not exist yet. In the screen-only fallback for a runtime without a context field, the line LC_ALL=C start_notices=... is two plain assignments, so LC_ALL stays C for the rest of the hook, and the 2,048 check counts characters rather than the bytes the design names.

EVIDENCE: channel_verbs.go runChannelStatus calls brain.MarkStatusPosted after SaveStatusState and returns 1 on its error; brain.go MarkStatusPosted returns the ReadStatus error. supervision-hook.sh screen-only branch uses ${#start_notices} and the LC_ALL prefix on an assignment with no command.

## Gaps the critic named

- No fixture bed script could run in this sandbox: bash 3.2 heredoc temp files are denied and no newer bash exists. The brain, dispatch, land, goal-cli, and supervision-hook rows rest on the host's receipts and on reading; I ran the Go tests and direct engine probes instead.
- The runtime's consumption of the context field was not observed live, which the design itself lists as unprovable in a sandbox. The event-name finding rests on the design's explicit text and the runtime's documented output shape, not on a Claude Code session.
- Pre-existing and outside this change: the Stop hook's human digest also reads under the git toplevel in the template layout (a narrator-digest.flock exists at the toplevel and no cursor file exists anywhere), so the human digest appears never to have delivered on template seats. The brain's digest inherits the same root.
- go test for cmd/metasystem was not rerun here; the host reports it green apart from the process-inspection test.
- The tree hash could not be recomputed because the sandbox denies writes to the shared object database; equality with the dispatcher's diff artifact was checked by content instead.
- Tool names were unobserved by the launcher, so this return is advisory per the runtime notice.

## Coordinator disposition (m1, 2026-09-07)

Every material finding folds in build round 5; the notes fold where
they are cheap and change what a node experiences. Nothing lands before
the fold and a green full battery.

- F-1 (event name): the hook's start object carries
  hookSpecificOutput.hookEventName=SessionStart beside the context
  field, declared per runtime in the registry next to the field name;
  the hook fixture asserts the member. Folded.
- F-2 (template layout): the packet checkout handed to brain boot is
  the installation directory the hook already knows as the state root's
  owner (the directory holding bin/metasystem, metasystem/ in this
  repository), never the git toplevel; the digest and the packet resolve
  under it. A fixture on a template-layout bed proves the packet loads.
  Folded; this is the goal's deliverable and it was broken on our own
  seats.
- F-3 (boot failure skips arming): the boot failure notice is collected
  like every other notice and the start path continues to the steward
  pending line, the identity check and the up leg; one object at exit.
  The brain-boot-verb-fails row asserts arming still ran. Folded.
- F-4 (absent-node row): built per addendum 4. Folded.
- F-5 (shell fence fails open): a fence helper failure is fail-closed at
  every call site in dispatch.sh (exit 1 with the reason), matching
  land.sh and commit.sh; a fixture with a failing fence verb proves it.
  Folded.
- N-1 (corrupt record refuses drafting): accepted as the conservative
  reading; the remedy line is one human command. Not changed.
- N-2 (scanner changes reach every seat): folded. pending-setup is not
  in flight for the goal clause on an undeclared checkout, and a draft
  scan error never vetoes an undeclared seat's all-clear; the two new
  fields are filled and not displayed there, as the design promised.
- N-3 (coverage row is a digest): folded to the allow-list the design
  names, matching only actor-construction sites, without rg.
- N-4 (weak rows): folded where one line does it: the dispatch rows
  also reach the router-level fences through the internal marker; the
  absent-node dispatch leg starts the fake-runtime delegate; the bound
  row checks the packet bytes.
- N-5 (child failure discards phase one): folded; a non-deadline child
  failure is treated like the deadline, sections skipped, phase-one
  text kept.
- N-6 (status-post exit 1 before first boot; LC_ALL assignment; bytes
  versus characters): folded, three small fixes.
- Pre-existing (the human digest reads under the toplevel on template
  seats): outside this chain; opened as its own goal.

Next: round 5 folds the above, host gate, then review round 2 on the
folded tree, then the full battery and the landing.
