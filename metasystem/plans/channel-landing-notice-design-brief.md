Working Mode: design
Orchestrator Identity: m1 (lineage main-1788594343-3833-fb64b9, dispatch delegate under goal channel-tells-me-when-something-lands, tier 3 DESIGN-BEARING)
Date: 2026-09-06

# Goal

Author the design document channel-landing-notice-design.md, a NEW file you
create in the metasystem plans directory, revision 1, for this goal: a
landing tells Wido, the moment it lands.

Wido's words, 2026-09-05: "the moment something lands, I want a message of
that." Two landings that day reached origin at 12:43 and 12:52 local and he
heard nothing for over an hour, then asked whether the machinery worked at
all.

# The defect, against the code

The channel was built as a periodic digest and is being asked to be a
notifier. metasystem/internal/channel/phase/phase.go composes a status on
every steward tick and posts only if ShouldPost in
metasystem/internal/channel/report.go agrees, and ShouldPost is an AND:
channel.status.interval-minutes (default 240) must have elapsed AND the
content must have changed. Nothing anywhere posts on a landing: `channel
status --post` in metasystem/cmd/metasystem/channel_verbs.go has no
production caller.

# What the design must decide

- Decision 1, what a landing is. The fleet already has a definition:
  landingLines in metasystem/internal/channel/report.go reads commits with
  a Goal-Item trailer to build the status's Delivered lines. Reuse it or
  say why not.
- Decision 2, where the trigger lives. metasystem/scripts/agents/land.sh
  pushes to origin in its "push origin" step and then syncs the transport;
  a post straight after a successful push is immediate but couples landing
  to the channel (a slow provider, a hung HTTP call, a landing that now
  depends on network); a sweep over commits since a cursor on the steward
  tick is decoupled but arrives a tick late against "the moment". Decide,
  with the bound on the prompt path's time and what land.sh does if a post
  hangs rather than fails. A post that fails must never change land.sh's
  exit status.
- Decision 3, exactly once across machines. A cursor beside the existing
  inbound cursor in the channel's state directory under the agents
  artifacts, advanced under the channel's lock. Attack your own answer: two checkouts of one repository on one
  machine, a push during a tick's sweep, a push whose cursor write fails,
  a rebase that changes commit ids after announcement, and a machine that
  fetches another machine's landings - which machine announces what.
- Decision 4, the message. One line per landing, inside the 1600-rune ask
  bound landed on 2026-09-05 in b52711d3a (metasystem/internal/channel/question.go);
  what a push of twenty commits becomes. Local time per the timestamps
  landed the same day in the report renderer (metasystem/internal/channel/report.go, ComposeReport).
- Decision 5, the four-hourly status is untouched: its interval and digest
  keep their meaning, and a notice at 13:00 followed by a status at 13:44
  saying the same thing is a wall of text by another door - say how the
  two states relate. An off switch, default on.
- Decision 6, proof: a fixture list where each obligation is provable, and
  anything unprovable named as residual risk rather than claimed.

Self-grade and reject condition, as the hook-root design carries them.

# Constraints

Wall-clock budget: 25 minutes. Read metasystem/internal/channel/report.go,
metasystem/internal/channel/phase/phase.go,
metasystem/internal/channel/telegram/telegram.go,
metasystem/internal/channel/inbox.go and metasystem/scripts/agents/land.sh
before writing. Version-2 implementer JSON; diffBoundary lists exactly the
one new design file you created in the metasystem plans directory.

# Gap Rule

Stop and report a gap; never fill it silently.
