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

8. RETRACTED: the mission family's bare call exits 2 correctly —
   the rc=0 reading was this review's OWN pipe error, a FOURTH
   instance, committed in the same session that documented the
   error class. Finding 7 upgrades from proposal to necessity on
   this evidence: even a reviewer hunting this exact mistake kept
   making it.
9. The templates are admirably compact (brief 32 lines, follow-up
   16, host-turn 13) and the host-turn template already speaks
   qualified names post-sweep. No finding.

## Slice summary

The dispatch flow's bones are right: entry discipline, usage
text, exit-code table, runtime discoverability, template economy.
The findings are all one family — REFUSALS WEARING SUCCESS OR
SILENCE: watch times out on a never-existed job instead of
failing fast (5), status answers a human with nothing (3), and a
piped stdout eats the terminal truth — proved FOUR times by this
review's own probes, finding 8 retracted as the fourth. Proposed
as ONE backlog item: "dispatch-exit-honesty" — fast-fail watch,
a stderr line on silent status, terminal status lines to stderr
on every dispatch exit path. Appetite: 3h with fixture legs.
Slice three (scripts+skills) and slice four (docs) remain
tokened separately.
