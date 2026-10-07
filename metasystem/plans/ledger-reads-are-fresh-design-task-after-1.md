# Task: revise plans/designs/ledger-reads-are-fresh.md after critique round 1

Working Mode: Design
Round 1 found 3 material. Fold them as below; keep everything else; Status stays draft. Never open any metasystem.conf.local. Edit only this page.

Decided by m1e for Wido (record it under that heading, reversible): the session-start hook keeps today's offline ledger read; only the explicit commands (`goal claim`, `metasystem internal goal next`, `session start`, `metasystem up`) read the ledger fresh. Reason: the stale-view friction of 2026-10-06 came through the explicit commands; the hook read's only beneficiary on main is remote reservations, which person-claims builds; and the hook's 15 s budget is shared with brain preparation (up to 8 s), supervision arming and wait recovery (up.go:833-845, runtime_hook_start.go:599-605, :645), so a fresh read there risks the hook's main job. This removes round-1 finding 2.
1. (finding 1) Build against main's restamp callback (cmd/metasystem/up.go:113-165) and claim as they are; move the post-preparation reread, per-claim pending preservation and their test assertions to person-claims, which will consume FreshProjection (say so under "Not in this goal").
2. (finding 3) Drop the explicit-command retry: one bounded attempt (4 s, the existing defaultFreshProjectionTimeout, project.go:46), report the cause, the remedy is to rerun the same command; no stop record needed (no loop).
3. Notes: the hook path needs no budget constant now; say so. "real Git available" for the cleanup proof; a named claim with the transport down fails at publication's CaptureTip as an environment failure, and the message says so.
Re-estimate (the publication-deadline plumbing goes). Return the units table and one line per item.
