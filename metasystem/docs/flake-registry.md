# The flake protocol

When a fixture leg fails:

1. **Check the registry** (memory/flake-registry.md). A listed leg
   earns exactly ONE solo rerun of its suite. Green rerun → record
   the sighting (count and date) and continue; the failure gates
   nothing further. Red rerun → it is real: stop and fix.
2. **An unlisted leg gets no rerun benefit of the doubt** on its
   first failure — diagnose it. If diagnosis lands on "transient,
   unreproducible, standalone-green", add it to the registry with
   its first sighting and continue.
3. **Three sightings inside thirty days** promote the leg to a fix
   goal on the backlog. Its complete structured budget is supplied
   if and when the goal is claimed. The registry entry links the goal
   until it closes.
4. Sightings are recorded in the SAME landing as the work that hit
   them — a flake nobody wrote down is a flake the next agent pays
   for again.

The registry is data, the protocol is this page, and neither lives
in any agent's memory.
