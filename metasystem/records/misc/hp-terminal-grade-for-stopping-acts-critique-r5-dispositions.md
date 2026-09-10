# hp-terminal-grade-for-stopping-acts, critique round five: dispositions

Chain hp-terminal-build1-20260909. The fifth critic
hp-terminal-crit5-20260909 (claude-opus-5) reviewed the fold
hp-terminal-build1-20260909-r7 at tree
3257d86a33a707d072bc703fc18b02f4822fd521 and returned two material
findings and three notes. Round eight folds four of them.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| HPT-15 | accepted | The critic ran round seven's live test verbosely on this host: the walk stops at process 1 with ANCESTRY_UNREADABLE because both platform readers report a parent of zero as "parent unknown" and the stable read refuses an unreadable parent before the root-owned-image rule is reached. HPT-13 is not fixed; every human is still refused and enrollment is still broken. | Round eight makes the top of the tree a known fact: the readers report the root's parent as none-and-known, "unknown" is reserved for a kernel refusal, the walk ends at the root after admitting it under the round-seven rule, and a proof's last node must be the root. |
| HPT-16 | accepted | Every fake tree gives process 1 a parent of 0 marked known, a shape the readers cannot produce, and the live test tolerates every refusal but two, so the change certified nothing. | Round eight shapes the fakes exactly as the readers shape a root and makes the live test assert the walk reached the root with no read refusal, ending only in the two honest outcomes of a real machine. |
| HPT-17 | noted | A root-owned protected system image with withheld arguments is exempt from the signature check, so an agent running as root behind a stock image would pass. Out of the threat model: an agent with root can rewrite the enrollment record or the engine; the critic found no route for a non-root user to make an image appear root-owned. | No change. Recorded here so the next design pass on the human proof sees it. |
| HPT-18 | accepted | The terminal walk records the parent's number without pinning its birth identity, where the enrolled walk refuses PROCESS_REUSED; exploiting it needs a pid reused inside the walk's own window and buys nothing new. | Round eight gives the terminal walk the enrolled walk's parent-continuity check. |
| HPT-19 | accepted | The recorded-holder cleanup path reports and sets status after its bounded wait but sends no kill, unlike the other three paths. | Round eight adds the same unconditional kill after the bounded wait. |
