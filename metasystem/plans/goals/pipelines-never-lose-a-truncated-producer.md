# pipelines-never-lose-a-truncated-producer

- State: queued
- Risk: severity=3 novelty=1 exposure=3 accumulation=2 basis="severity 3: the witness gate runs git archive into tar under pipefail on a real landing path, so a tree crossing the pipe-buffer window refuses real landings with a false tar write error; novelty 1: a known SIGPIPE-under-pipefail class; exposure 3: every seat's landings through the witness gate plus the land and supervision beds; accumulation 2: the failure comes and goes with tree size, so it recurs silently as the tree grows"
- Tier: 3
- Intent: A pipeline whose reader may legitimately stop early must never fail, or falsely succeed, because its producer was cut off. On 2026-09-14 m1b measured git archive piped into tar exiting git=141 tar=0 under pipefail at some tree sizes (47,032,320 and 47,769,600 bytes fail; 47,636,480 passes): bsdtar stops at the end-of-archive marker while git still has trailing bytes, so the landing bed goes red on an unmodified tip. The same pipeline exists at eight sites, including scripts/agents/witness-gate.sh:144 and :147, which run it under set -o pipefail on a real landing path, and scripts/agents/supervision-fixtures.sh:2528. The wider shape bit m1c twice the same day: a git apply piped into head truncated a rebase and reported success, and a trailing grep with no match turned a green verification into exit 1. DONE: every git-archive seeding site archives to a file and extracts from it; a gate check refuses a producer piped into tar -x, head, grep -q, sed with q, or awk exit under pipefail unless the site declares why the producer's status is irrelevant; and a fixture reproduces the size-dependent SIGPIPE and proves the file-based form passes at every size.
- Origin: human
- Next step: Replace the eight git archive | tar sites with archive-to-file then extract, witness-gate.sh first because it guards real landings; then add the gate check over scripts/ with a declaration form for the rare pipe whose producer status genuinely does not matter, and the size-sweeping reproduction fixture. m1b's stop-decisions round 24 makes land-fixtures.sh file-based; if it lands first it is this goal's first conforming site.
- OpenedAt: 2026-09-14T17:06:35Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-14T17:06:35Z BZGJWQGTEKXMPXCJXYHQ2VFGQ5-m1e-c6925449 open actor=human:Wido targets=pipelines-never-lose-a-truncated-producer
Integrity: sha256=63d1f3485dede9e163c0b7946baf1b2e96b5a0d8c929be8d44cff1b13cdcf93a
