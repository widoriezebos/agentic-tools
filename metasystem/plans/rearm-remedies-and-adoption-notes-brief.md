Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal rearm-remedies-and-adoption-notes, tier 2, hazard DESIGN-BEARING)
Date: 2026-09-06

# Goal

The engine re-arm landing (ea8c3ead) left four notes from its third code
critique, none material, and the field added a fifth. When you are done,
all five are fixed with a test per remedy:

1. Adoption of a detached target. metasystem/scripts/adopt.sh dies with
   "adoption refused: the target checkout is detached" after it has
   already written the payload, the engine and the nickname, leaving
   the target half adopted. The Go verbs already handle the same case
   by continuing and saying automatic machine re-arm stays off
   (seedStewardLandingRef in metasystem/cmd/metasystem/steward_verbs.go).
2. Adoption overwrites an existing landing ref. The same script writes
   the git config key metasystem.steward.landing-ref unconditionally,
   while the human steward verbs read it first (with --local
   --no-includes) and keep an existing value.
3. The not-landed remedy names only the terminal. In
   metasystem/internal/up/up.go the enrollment-drift remedy has
   branches for a ref that does not resolve and for no ref at all, but
   a rebuilt engine "not landed on" or "not proven landed on" the ref
   (the four messages in metasystem/internal/steward/rearm_resolver.go,
   resolveLandedBuild) falls to the default remedy, which names only a
   terminal restart. A merely stale remote-tracking ref is repaired by
   one fetch of the configured remote, so the remedy must name both.
4. The before-mint catch-all. In up.go, a machine re-arm that fails
   before any stage transition with a non-drift error gets "repair the
   named enrollment publication failure, then rerun metasystem up"
   whatever the cause. The causes that reach it come from arm in
   metasystem/internal/steward/runner.go before the mint: the missing
   notification channel, the runner directory that cannot be created,
   the lock file that cannot be opened or taken, the identity
   re-publication with durability pending. The remedy must name the
   actual class: set the notify command; make the runner directory
   writable; find and end the holder of the arm lock; repair the
   publication.
5. A human's arm must leave a human-witnessed generation. On two
   machines today Wido ran steward arm at an agent-free terminal with a
   rebuilt engine at the enrolled path while a runner was live; the
   verb seeded the landing ref, hit the live-runner short circuit in
   arm (runner.go, "already armed"), and minted nothing. The next
   session's metasystem up then saw the changed bytes and minted
   generation two as machine-rebuild with no human witness, although
   the human had stood at the terminal with exactly those bytes.
   Decision: when the human arm verb (human-terminal, not the
   temporary-word form) finds a live runner AND the enrolled engine's
   bytes differ from the recorded identity, it takes the replace path
   restart takes (stop the runner, re-arm, mint a human-terminal
   generation whose human witness is itself) and says so in its
   message. When the bytes are unchanged, the existing behaviour stays:
   already armed, and the message names restart for a witness. The
   machine rule is unchanged.

# The design

- Item 1: in adopt.sh, replace the die with a note on stderr in the
  voice of the script's two existing "landing ref was not seeded"
  lines, saying the target is detached and automatic machine re-arm
  remains disabled until the key is configured; skip the upstream and
  landing-ref steps for a detached target; the rest of adoption
  continues and the script exits zero.
- Item 2: before writing the key, read it with the same scope the Go
  side uses (git config --local --no-includes --get); when it is set,
  keep it and print a note saying it was kept.
- Item 3: a new remedy branch keyed on the resolver's not-landed
  messages, naming one fetch of the configured remote (git -C <root>
  fetch <remote>) followed by rerunning metasystem up, or the terminal
  restart.
- Item 4: classify the before-mint error by the substrings of the arm
  errors named above and emit the matching remedy; keep the current
  sentence as the fallback for an unrecognised error.
- Item 5: in runner.go's arm, at the live-runner check, when replace is
  false and the plan is the human-terminal mint and the enrolled bytes
  differ from the recorded identity, continue as if replace were true.
  Decide the byte comparison from what arm already reads (the recorded
  identity's engine digest against the binary at the enrolled path);
  name the field you compare.

# Tests

- Items 1 and 2: two legs in metasystem/scripts/adopt-fixtures.sh: a
  detached target adopts with the note and exits zero; a target with a
  preset key keeps it. If the fixture bed's template mode cannot host
  them, say so and put them where adoption is otherwise proven.
- Items 3 and 4: one test each in metasystem/internal/up/up_test.go,
  in the shape of the existing landing-ref remedy tests: the not-landed
  error yields a remedy naming both the fetch and the terminal; each of
  the four before-mint classes yields its own remedy sentence.
- Item 5: one test in metasystem/internal/steward/rearm_test.go beside
  the existing human-arm-beside-a-live-runner test: with changed
  enrolled bytes, human Arm mints generation two as human-terminal with
  human witness two and replaces the runner; the existing test's
  scenario (unchanged bytes) keeps its assertions.
- metasystem/scripts/agents/supervision-fixtures.sh asserts the
  resolver's error text and the two landing-ref remedies; touch it only
  if a remedy assertion there names the default sentence you change.

# Workspace

The job worktree the dispatcher creates for you, branched from main.
May touch: metasystem/scripts/adopt.sh
May touch: metasystem/scripts/adopt-fixtures.sh
May touch: metasystem/internal/up/up.go
May touch: metasystem/internal/up/up_test.go
May touch: metasystem/internal/steward/runner.go
May touch: metasystem/internal/steward/rearm_test.go
May touch: metasystem/scripts/agents/supervision-fixtures.sh (assertions only)
Must not touch: rearm_resolver.go's messages, steward_verbs.go, anything under plans.

# Constraints

- Bash 3.2 clean for the shell changes. Never weaken a test. Go tests
  first; run the adopt fixtures only if they run in your sandbox, else
  say so and the orchestrator runs them seat-side.
- One round, at most 120 minutes of wall clock. Hazard DESIGN-BEARING
  for item 5 alone: who mints a witnessed generation changes; an
  independent critique follows.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `go test -count=1 ./internal/up ./internal/steward` (expected: ok)
- `go vet ./internal/up ./internal/steward` and `gofmt -l ./internal/up ./internal/steward` (expected: clean, no output)
- `bash -n ./scripts/adopt.sh` and `bash -n ./scripts/adopt-fixtures.sh` (expected: clean)
- `bash scripts/adopt-fixtures.sh` (expected: passes, the two new legs named; or the sandbox reason)
- `git diff --stat` (expected: only files under May touch)

# Acceptance Criteria

1. Adopting a detached target exits zero, prints the note, and leaves
   the landing ref unset; adopting a target with a preset landing ref
   keeps the value.
2. The not-landed remedy names the fetch and the terminal; each
   before-mint error class has its own remedy; a test pins each.
3. Human Arm beside a live runner with changed enrolled bytes mints a
   human-terminal generation with its own witness; with unchanged bytes
   it still reports already armed and names restart.
4. All named packages and fixture beds green.

# Gap Rule

stop and report a gap; never fill it silently.
