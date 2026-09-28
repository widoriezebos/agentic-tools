# Dispositions: multi-main coexistence, round 1

Chain design-critic-20260807t081739z-df4e. All thirteen accepted; folded by full rewrite of the Changes section.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| MM-1-1 | accepted | The harness cannot see editor writes; "one writer" was false. | M-2 narrows to mediated mutations, fail-closed, plus detection of foreign edits; the residual is named, not hidden. |
| MM-1-2 | accepted | Session UUIDs die at restart; ownership would orphan daily. | Ownership is a renewed lease with TTL; restart costs at most one TTL. |
| MM-1-3 | accepted | No atomic acquisition. | Claims and renewals ride dispatch's existing compare-and-swap. |
| MM-1-4 | accepted | No caller authentication. | M-2a: process-ancestry resolution to an announced main via the census's find-ancestor. |
| MM-1-5 | accepted | Lease record underspecified. | Full field set: holder, pids, claimedAt, renewedAt, ttlSec, takeover history. |
| MM-1-6 | accepted | Circular recovery when holder and supervision both dead. | M-2b: claim first, then arm as holder; arming checks the lease. |
| MM-1-7 | accepted | Surviving delegates race the next owner. | M-2c: takeover reaps and adopts, recorded. |
| MM-1-8 | accepted | Jobs are not linked to streams. | dispatch --stream; per-stream in-flight from records. |
| MM-1-9 | accepted | Read-only peer still churns shared CLI config. | M-6 absorbs KI-19: filtered identity hash per adapter declaration. |
| MM-1-10 | accepted | Adapter observation is not universal. | M-4 states provenance per runtime; `unobserved` where absent; never pretend. |
| MM-1-11 | accepted | `claimed` breaks additionalProperties:false schemas. | Versioned return-schema bump with a migration window. |
| MM-1-12 | accepted | Counter at follow-up misses terminal errors. | Increment at detection in assert-return-complete. |
| MM-1-13 | accepted | "Turn-end report" was unnamed. | The stop-hook status line, with a per-session growth cursor. |
