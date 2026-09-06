# enroll-terminal-refuses-macos-terminal

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="severity 3: this verb is the only durable source of human authority; while it refuses, every human-only act rides relayed words with a hard horizon (R-32-m1, 2026-09-06), and the fleet's whole authority model degrades to per-act relays; novelty 2: the walk's trust rule must distinguish an OS-withheld argv from a hidden one, which is a new judgement in the ancestry proof; exposure 3: every Mac seat, the machines Wido actually sits at; accumulation 2: the same ancestry walk already produced the auto-mode classifier dead ends on 2026-09-02 and the temporary-word workaround the fleet has been living on since"
- Tier: 3
- Intent: goal enroll-terminal cannot enroll a human from the standard macOS Terminal. Terminal.app spawns every tab through a root-owned login process (login -pf wido), and macOS denies a non-root process the argv of a root-owned one: metasystem proc probe --pid <that login> returns argv null. The enrollment walk in internal/humanauthority/authority.go (stableRead) requires ArgvKnown for every ancestor it reads, so the walk refuses ARGV_UNREADABLE at login on every Terminal.app shell - Wido hit exactly this on 2026-09-06 running the documented command. DONE means a human at a Terminal.app shell can enroll: the walk treats a root-owned ancestor whose argv the OS withholds as what it is (a system process that cannot be an agent, identified by its executable and owner) rather than as unreadable, or the documented enrollment path names a shell shape that works and the verb says so in its refusal; and a fixture pins the Terminal.app ancestry shape
- Origin: main
- Next step: Confirmed 2026-09-06 on m1: proc probe on the root-owned login (pid 92843) returns argv null; on the wido zsh under it argv is readable; Terminal.app itself is readable. Decide the rule for a root-owned ancestor with withheld argv (executable path plus owner root plus a known system login binary is not an agent), pin it with a fixture that builds the Terminal.app chain shape, and make the refusal name the workaround until then. Workaround to verify: a shell inside a tmux server the human started from Terminal has no root-owned ancestor between the shell and launchd
- OpenedAt: 2026-09-06T06:36:15Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-06T06:36:15Z 8Q3WR34BMV3GWVW2MN7H9K111N-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=enroll-terminal-refuses-macos-terminal
Integrity: sha256=ec5100260777a176dc5bc78e7543bac71ba61f61a5c6fdbe97f9958b532ceb6e
