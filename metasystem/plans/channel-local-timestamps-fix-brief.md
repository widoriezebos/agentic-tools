Working Mode: implement
Orchestrator Identity: m1 (lineage main-1788594343-3833-fb64b9, follow-up round on chain implementer-d8ff4e56e9453860f3e03154, goal channel-local-timestamps)
Date: 2026-09-05

# What this round is

The fold of code-critic-0286b4148df0ffc8cbe70d83's review. It found nothing
material: the display/record split holds, the offsets are right, and the
tests pin their locations. Two low findings, one of which is worth closing
now. Change nothing else.

# F-1: the both-forms recognition is only half proven

You named it yourself as the riskiest part: the digest must keep ignoring
headline-only time changes, and it now has to recognise both the new
offset-bearing headline and the legacy Z form. The round-1 diff rewrote the
existing Z-form digest test into a +0200 form instead of keeping it, so the
Z branch of the recogniser has no test. The critic confirms the branch
works by reading it, and that production never feeds old-form text to the
new binary because each machine digests only text it composed itself, so
nothing ships broken — but the riskiest part should not be the untested
one.

Add the check, keeping every existing test: the digest of a Z-form headline
and the digest of the equivalent offset-bearing headline must be equal, for
otherwise identical body text. The critic's own example is
'm status 2026-09-04 08:00Z\nNext up: first' against
'm status 2026-09-04 13:00 +0200\nNext up: first'.

# F-2: say what the configured location is for

The location defaults to the posting machine's zone, which is what the
brief asked for and what the goal's DONE says. The critic notes what that
means in practice: a seat whose system zone is UTC still posts a +0000
headline, and the fleet's Lima guests are exactly that. Whether the fleet
should render one human's zone everywhere is Wido's decision and is NOT
part of this round.

What this round does: make the configuration explicit where it is declared,
so the next reader can set it without rediscovering the mechanism. Name the
key, say it takes an IANA location, say the default is the posting
machine's zone, and say that setting it makes every machine render that one
location. Do not add a default value, do not read any new environment
variable, and do not change behaviour for a machine that sets nothing.

# Boundary

Only metasystem/internal/channel/report.go and
metasystem/internal/channel/channel_test.go, unchanged from round 1. Report
`go test ./internal/channel/...` green.
