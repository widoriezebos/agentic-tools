# Draft: dispatch-cap-necessity (Wido's question, 2026-08-31)

Wido, while ordering the m3 Sol cap key removed: "why do we need these
at all now we have streaming and can see/judge the progress. And in the
end we have a fail stop of 120 minutes; but we should work not to ever
need that."

The question, unpacked: per-job reservation caps (cap.min.* keys, the
120-minute built-in) predate streaming observability. Now that a
dispatch's progress is visible live and judgeable, a tight
per-job cap may be redundant ceremony — the working control should be
progress observation, with the 120-minute fail-stop as the never-hit
backstop. Today the cap also feeds the reserved-job-minutes budget
projection at admission, so caps and budget tuples are coupled: a lean
tuple forces a low cap key (m3 hit this twice on 2026-08-31 — a
120-minute default cap against 60- and 90-minute tuples refused
admission before any work ran).

Design question for Wido's word before any goal opens: should the
budget projection consume observed/estimated run minutes rather than
reservation caps, demoting the cap to pure fail-stop? Interacts with
R-13 (structured limits are the only budget law), R-17 (slice-norm
governs capMin), and the stop-loss machinery.

Status: draft awaiting Wido's word (R-2: design-bearing, touches budget
law). Not opened as a goal.
