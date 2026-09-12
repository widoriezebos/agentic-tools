Working Mode: build
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal stop-refusal-fits-on-one-screen, reopened)
Date: 2026-09-09

# Goal, second slice

Goal stop-refusal-fits-on-one-screen (tier 3, approved; reopened
2026-09-09 after its first slice landed as d533caf17). Its record,
metasystem/plans/goals/stop-refusal-fits-on-one-screen.md, is the
contract. The first slice bounded the BLOCKING refusal and its system
message. The first Stop message after it landed showed what it missed:
on the ALLOW path (the verdict was STILL WORKING) the hook appended its
check-in tail unbounded, and that tail was the whole health line, about
2,400 runes, sixteen checks each with a remedy, in one line. Worse, the
line was wrong: it reported steward-runner, supervision-owner,
repo-watcher and session-main dead two lines after the re-arm notice
had proved them alive, because health was read against the wrong root.
Wido read it and asked, fairly, in what way that was not a wall of
text. DONE for this slice: every Stop message, block or allow, fits the
same 4,000-rune bound; the health line on the hook path reports the
seat's real state on a template-layout seat; and its hook-preview
rendering is the aggregate plus only the checks that are not alive,
with the full health text in the stop-verdicts file, not the message.

# Where it lives (traced)

- metasystem/scripts/agents/supervision-hook.sh: line 833 calls
  `health --hook-preview --repo "$repo" --metasystem-root "$harness_root"`;
  lines 855 to 859 compose checkin_tail as health line, re-arm notice
  and narrator digest; line 935 puts it into the refusal system message
  (bounded by the first slice through report stop-block); lines 954 and
  955 append it to allowed_message on the ALLOW path, which is then
  emitted through `json object systemMessage=` with no bound;
  surface_json (line 606) emits other allow-path notices the same way.
- metasystem/internal/steward/health.go: PreviewHealthAt (line 272)
  takes repoRoot and metasystemRoot and calls evaluateHealthRoles with
  both; on this seat (repository root /Users/wido/LocalStorage/GitHub/agentic-tools,
  installation under its metasystem directory) the supervision roles
  answer dead with remedies naming `--repo <repository root>`, while
  `health --repo <metasystem directory>` answers alive. So at least the
  steward-runner, supervision-owner, repo-watcher, census-freshness,
  narrator-freshness and session-main roles read state under repoRoot
  where they must read under the installation root.
- metasystem/internal/report/stopblock.go: BoundSystemMessage, the
  first slice's bound, currently reachable only through
  `report stop-block --system-message`.
- metasystem/cmd/metasystem/report.go: the report verbs.
- metasystem/scripts/agents/supervision-hook-fixtures.sh: the suite,
  including the first slice's 200-run scenario near line 510.

# The change

1. Health reads the installation root. In metasystem/internal/steward/health.go,
   every role that reads supervision state (runner, owner, watcher,
   census, narrator, session main, hook freshness, and any other that
   opens artifacts under a root) reads it under metasystemRoot when the
   caller gives one, and the remedies it prints name that root. The
   ordinary `health` path (ObserveHealth) gets the same root argument so
   the two never disagree. A Go test drives PreviewHealthAt with a
   repository root and a distinct installation root holding a live
   runner record and asserts alive, not dead.
2. The hook-preview line is short. PreviewHealthAt's rendering for the
   hook (`--hook-preview`) prints the aggregate, then only the roles
   that are not alive, each with its remedy, then "N checks alive". The
   full line as it is today goes into the turn's stop-verdicts file
   (the first slice writes that file inside the verdict's flock; append
   the health text to it, or write a sibling named
   <session>-health.txt beside it, and say which). A Go test asserts a
   verdict with one dead role renders to under 400 runes and names the
   dead role and its remedy.
3. The allow path is bounded. Add a report verb that returns the bounded
   text for a message, using BoundSystemMessage and the same constant
   (`report bound-message --text <text>` is a fine name), and route the
   hook's allow-path emissions through it: the allowed_message at line
   954 and surface_json at line 606. The hook stays plumbing; the bound
   is Go's. The refusal path already goes through report stop-block and
   does not change.
4. Fixtures: in metasystem/scripts/agents/supervision-hook-fixtures.sh,
   one scenario on a template-layout bed (repository root distinct from
   the installation root) in which the verdict allows (a running job of
   the caller main) and the health text is longer than the bound:
   assert the emitted systemMessage is within 4,000 runes, contains the
   health aggregate, does not contain the alive roles' remedies, and
   that the health preview says the runner is alive when the bed's
   runner record is alive. The 200-run scenario from the first slice
   stays as it is.

# Not in scope

What is judged. The retro-due notice is a real obligation and stays as
a line. The narrator digest line stays; if it is long, the bound trims
it with the notice like any other part.

# Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l . (empty)`;
`go test ./internal/steward/ ./internal/report/ ./cmd/metasystem/ -count=1`
green; `bash scripts/agents/supervision-hook-fixtures.sh` green (say so
if the sandbox cannot run it; the orchestrator replays it outside).
Coverage must not drop below the tree's figure on 2026-09-09:
internal/report 87.6 percent; measure internal/steward before and after
and report both.

# Constraints

Wall-clock budget: 90 minutes; return before it ends even if something
is red, naming it. MECHANICAL reach (tier 3). Declare the boundary as
every file that differs from main. Stop adding at a gap that needs a
decision no page has made; keep what is built and green; report the
gap with the resolution you propose. Never delete built work. Plain
English in comments and messages; no finding or round references in
source comments.
