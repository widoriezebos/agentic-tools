# Dispositions: coexistence design, round 6

All five accepted; each reproduced by reading the cited mechanism (IL-21 level: read).

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| MM-6-1 | accepted | Reuse could falsely AUTHENTICATE, not merely refuse. | Per-call process-table re-read; pid, start, and command must match together; residual stated as the triple within one read. |
| MM-6-2 | accepted | Verify-then-write left a window. | Record creation inside the lease flock; no window exists. |
| MM-6-3 | accepted | Durable evidence collided across sessions. | Per-checkout evidence subtree in the durable store. |
| MM-6-4 | accepted | Any failure passed the probe. | Two-phase probe: would-block while held, success after release; anything else refuses. |
| MM-6-5 | accepted | The Proof described deleted machinery. | Proof rewritten to the surviving mechanisms. |
