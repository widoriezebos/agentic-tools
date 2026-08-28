# The goal-verb system from the agent seat

Part one of agent-ease-assessment, written for the 2026-08-22 retro
after one full day of real use: the conversion, eleven mutations,
four goals claimed and three concluded through the live system.
Each finding carries today's evidence and a proposed fix with an
appetite guess. Nothing here is implemented; it is retro input.

## Intuitiveness — good bones, two traps

1. **The verb names and refusals read well.** "goal claim refused:
   machine widos-m5-pro claims backlog-git-sync, backlog-mechanism:
   the quota is one claim per machine" told me the law, the holder,
   and the fix in one line. Keep this bar.
2. **TRAP: the verbs and the checkout live in different worlds.**
   Mutations publish to origin's canonical branch; the local
   checkout does not advance. Every landing today then failed its
   first push (non-fast-forward) until a manual rebase — three
   times. An agent who does not know the design will read this as
   breakage. Fix candidates: `commit.sh --push` does one
   `pull --rebase` on refusal and retries; or goal verbs print
   "origin advanced; rebase before your next landing". Appetite:
   an hour.
3. **TRAP: --root is easy to hold wrong.** I probed the wrong root
   once and misread an empty answer as a routing bug. The empty
   legacy shape ({"baselineMatches":false...}) says nothing about
   WHERE it looked. Fix: every read answer carries "root" and
   "world" fields. Appetite: an hour.

## Forgiveness — one sharp edge

4. **METASYSTEM_OWNER_LINEAGE defaults to "session" silently.** A
   forgotten export mints claims under a generic lineage — lawful
   but wrong identity, and steal/succession semantics then bite.
   Fix: refuse mutations when the variable is unset unless
   --lineage is passed; reads stay lenient. Appetite: an hour.
5. Idempotent replays, quota refusals, and journaled REJECTED
   results all behaved forgivingly today — a failed claim cost
   nothing and explained itself.

## Completeness — the matrix has holes

6. **Wired:** open(+--claim), claim/release(+--arc), steal, done,
   park/unpark (single goal), reopen, edit, set-arc, detach, prune,
   reconcile(+--refresh-only), recover, fetch, list, next, migrate,
   source-digest.
7. **Unwired engine verbs:** ParkArc/UnparkArc (park --arc silently
   parks ONE member today — worse than missing: it half-does the
   cascade the engine supports); DeclareFree (CLI stub says "raise
   it if you need it"); repair --accept-remote (exists read-side
   only). Fix: wire the three, refuse --arc on park/unpark until
   then. Appetite: two hours.
8. **Missing read verb: `goal show --id X`.** Today the only ways
   to see one goal are the whole list JSON or reading the file at
   the tip. History, claim holder, park reason — all in the file,
   none addressable. Appetite: an hour.

## Observability — machines yes, humans partly

9. **list is one JSON line.** Right for tooling, hostile to eyes.
   `goal list --pretty` (or a short table mode) is the single
   cheapest ergonomics win. Appetite: an hour.
10. **next is exactly right** — one orientation line, claimed
    first. Keep.
11. **The journal is invisible.** Recovery reports print at recover
    time only; there is no `goal journal` to inspect stranded or
    terminal entries when debugging. Appetite: an hour, read-only.

## Embedding — the biggest gap is the turn boundary

12. **AGENTS.md carries goal open/next doctrine and the audit
    enforces it — good.** But the RUNNER never speaks goal-verb:
    the turn-boundary integration (the old BGS-16: `goal next` at
    host-turn start, claim/conclude hooks in the mission flow) was
    displaced by the review loop and never built. Until it lands,
    the backlog steers humans and coordinators but not the machinery
    that spends most of the tokens. This is the one item here that
    is plausibly ABOVE the eight-hour line — it needs its own
    appetite discussion tonight.
13. **The stop hook and steward still read legacy shapes** in their
    prompts (goal next is safe for both worlds now, but nothing
    FEEDS it to them). Same discussion.

## Retro summary

Small fixes (2-9, 11): about a day of accumulated hours-items;
propose bundling as one "verb ergonomics" backlog item, appetite
one day. The turn-boundary embedding (12-13) is its own item with
its own appetite conversation. Everything else observed today —
naming, refusal quality, quota behavior, idempotence — is in good
shape and needs no work.
