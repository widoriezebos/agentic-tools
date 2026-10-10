# Task: write the design for fleet-survives-its-providers (plan goal 3)

Working Mode: Design
Write plans/designs/fleet-survives-its-providers.md from the brief plans/fleet-survives-its-providers-design-brief.md (read it in full, including its 10-07 additions: load-aware work in progress, the unloaded host comparison and integration observations moved here from machinery-measures-its-own-process). Read plans/machinery-open-findings-plan-2026-10-06.md findings 4, 7, 8, 16 and "Learned 2026-10-07", plans/designs/machinery-mechanisms.md (host state, the boundary event), and plans/designs/machinery-measures-its-own-process.md (its cost reader consumes this goal's unloaded comparison).

SIZE (Wido 2026-10-07): each unit at most 250 production lines, at most 5 units; if the scope does not fit, split it into a follow-up brief BEFORE the units table and name what moved. Write each Decision so a builder cannot read more scope into it than the unit's row: one paragraph per unit, the exact behaviour, its sites. Production callers only. Each unit's check is its new and changed tests and their mutations; the full suite runs once when the goal lands.

Shape as the accepted designs: header (Kind: design, Id: a new ULID, Status: draft, Goals: fleet-survives-its-providers), Wido's binding words, what it delivers, Decisions with code sites read on the current tree (cite file:line), Units table (unit, intent, production lines, areas), Estimates, one public-verb test per unit failing under its mutation, readers; the five questions for every new function, record and act; the four defect classes.

Do not touch code, memory/, records/ or the ledger. Never open any metasystem.conf.local. Return: page path, units with production-line estimates, anything split off.
