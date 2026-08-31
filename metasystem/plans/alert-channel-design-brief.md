Working Mode: design
Orchestrator Identity: m3 (lineage mac-m3, dispatch delegate under goal alert-escalation-channel, overnight envelope R-38-m3)
Date: 2026-08-31

# Goal

A design document at plans/alert-channel-design.md for the promoted
alert-escalation-channel goal: escalations and blocked-on-human states
reach Wido IMMEDIATELY over an external channel, so he is notified the
moment machinery lawfully needs his judgment — instead of escalations
terminating in a git-landed log he must poll. The driving specimen is
records/misc/idle-loss-2026-08-31.md: three hours lost, nine
escalations written, none delivered.

# Workspace

The dispatch-created job worktree. Produce exactly one new file:
plans/alert-channel-design.md. Design only — no code.

# Inputs

- plans/goals/alert-escalation-channel.md — the goal text as promoted,
  carrying the merged scope (alerts immediate and unmissable; narrator
  digests batched as a second, lower-urgency message class on the same
  channel design).
- Wido's design requirement, verbatim, binding: "it needs to have an
  abstraction/adapter. I want to be able to have email, slack,
  telegram, whatsapp etc underneath by simple configuration."
- records/misc/idle-loss-2026-08-31.md — the specimen and root causes.
- internal/steward/ — where escalations are born (episodes, the
  narrator digest Append, the tick); the alert episode store the goal
  text names as source of truth (L4).
- metasystem.conf / internal/config — the configuration idiom the
  adapter selection must follow (keys resolve flag > env > .local >
  committed; see docs/glossary.md for the config family).
- cmd/metasystem/health verbs — `health acknowledge-alert` is the
  existing acknowledgment seam the goal text names.
- docs/paper/12-learning-systems.md and the law-becomes-software draft
  (plans/goals-drafts/law-becomes-software.md) — the channel is
  transport with per-rule governance discipline, never a second state.

# Requirements the design must satisfy

1. TRANSPORT ABSTRACTION (Wido's word): one channel interface with
   pluggable adapters — email, Slack, Telegram, WhatsApp as the named
   targets — selected and configured through metasystem.conf keys
   alone; adding a configured channel must require zero code changes
   at the call sites. The design names the adapter contract (send one
   message of a given class, report delivery or failure, nothing
   more) and the configuration key shape.
2. TWO MESSAGE CLASSES on one channel design: alerts (immediate,
   unmissable, one per actionable state) and digests (batched
   narrative). Alerts must never drown in narrative — distinct
   channel instance or distinct high-urgency identity per the goal
   text.
3. ALERT CONTENT: every alert names what happened, what is asked, and
   the exact command or act that answers it, in the plain-words
   discipline of docs/seat-communication.md (it governs every channel
   that reaches the human).
4. SOURCE OF TRUTH: the alert episode store is authoritative; the
   channel is transport and keeps no second state. Delivery receipts
   are recorded against episodes; the acknowledgment path is the
   existing health acknowledge-alert seam.
5. ALERT CLASSES carried: the Ruling L escalation classes (ended
   auto-heal, no lawful remedy, flapping) PLUS every blocked-on-human
   state (a claim awaiting approval, a stop awaiting resume, a
   decision-ask with no human at the terminal, an enrollment drift
   awaiting re-arm).
6. FAILURE HONESTY: when the channel is down or a send fails, the
   design says what happens — the recorded fallback (phone/desktop
   path, the stop-message floor) and how failed delivery is surfaced
   without becoming its own unread log.
7. CREDENTIALS: channel credentials live outside the repository
   (local configuration or environment), never committed; the design
   names the key shape and the refusal when credentials are absent
   (the channel reports unconfigured, machinery never blocks on it).
8. SLICE PLAN: ends with independently deployable slices, the first
   at most 4 hours, and names which adapter ships first (choose the
   one with the simplest credential story) with the others as
   configuration-only additions behind the same contract.

# Constraints

- Design only. The design carries its R-24-m1 self-grade: confidence,
  weakest claim, reject condition.
- Wall-clock budget: 35 minutes.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/alert-channel-design.md; evidence cites inputs read.

# Gap Rule

stop and report a gap; never fill it silently.
