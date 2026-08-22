# The dispatch and mission flow from the agent seat

agent-ease-assessment, slice two (Appetite: 4h). Method as slice
one: meet the surface the way a working agent does, score
intuitiveness / forgiveness / completeness / observability /
embedding, propose fixes with appetite guesses. IN PROGRESS —
findings bank here as probed.

## Intuitiveness

1. The entry discipline is RIGHT, all three probes: a bare call,
   an unknown verb, and --help each print the usage (bare and
   unknown exit 2). My first probe misread an artifact as silence
   — recorded here as method honesty: verify twice before filing.
2. The usage text is good: verbs, flags, and the exit-code table
   an automation author needs.

## Forgiveness and observability (probed, true rcs unpiped)

3. `status --job <nonexistent>` answers rc=6 (unknown-status, per
   the table) but prints NOTHING. Right for automation, blank for
   a human — one stderr line ("no job record for X") costs nothing
   and orients. Appetite: minutes.
4. `follow-up` misuse exits 2 with usage — correct.
5. **`watch --job <nonexistent>` burns the full watch timeout and
   exits 4.** A job that never existed is knowable IMMEDIATELY
   (no record on disk) and should answer 5/vanished fast; timing
   out conflates "never was" with "still silent". Appetite: an
   hour with a fixture leg.
6. `runtime list` is clean and exactly where the usage points.
7. METHOD FINDING, on myself: three probes in this review misread
   return codes because a `| head` pipeline replaced the verb's rc
   with head's. Agents WILL make this exact mistake in automation.
   Worth considering: dispatch verbs always print their terminal
   status line to stderr, so the truth survives a piped stdout.
   Appetite: an hour, one shared exit path.

## Completeness and embedding

8. **`metasystem mission` (family, no verb) says "a verb is
   required" and exits rc=0.** A refusal wearing a success code —
   the same class as finding 5, in the engine's own front door.
   Check every family's bare invocation while fixing. Appetite: an
   hour with a table-driven test.
9. The templates are admirably compact (brief 32 lines, follow-up
   16, host-turn 13) and the host-turn template already speaks
   qualified names post-sweep. No finding.

## Slice summary

The dispatch flow's bones are right: entry discipline, usage
text, exit-code table, runtime discoverability, template economy.
The findings are all one family — REFUSALS WEARING SUCCESS OR
SILENCE: watch times out on a never-existed job instead of
failing fast (5), status answers a human with nothing, the
mission family's bare call refuses with rc=0, and piped stdout
eats the truth (my own probes proved it thrice). Proposed as ONE
backlog item: "dispatch/mission exit honesty" — fast-fail watch,
stderr status lines on every terminal path, rc audit across
family entries. Appetite: 3h with fixture legs. Slice three
(scripts+skills) and slice four (docs) remain tokened separately.
