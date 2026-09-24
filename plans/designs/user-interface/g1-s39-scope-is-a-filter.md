# g1-s39 Scope is a filter

- Kind: design
- Id: 01M39Z1P1MG9ZWF0CRMREC8D4D
- Status: accepted
- Cites: 01M34HS374KF1RSS3EWKBGD2WE 01M348YTJ2EY11F37Y5KBVNSER

Revision 1, 2026-09-24, Claude on Fable, on Wido's finding that a design made on a goal page also appears, unmarked, on the Project page: "We will have too many designs at project level then ... I don't want to be overwhelmed with designs at project level. I do want to see the designs that are applicable to the goal that I'm looking at." Ruled "I agree. Go." Frontend, the Overview's composition and the Partner's index; the record grammar is unchanged: a record's scope is its `Goals:` line, a question's its `goals` column, and none means the project as a whole.

## The principle

Scope is a filter with a sensible default, not two lists. The project's own records are the small set that shapes everything and are what the Project page opens on; the records under goals are one control away and always found by search.

## The design

1. **Project tabs default to the project's own records**: decisions, designs and open questions with no goal on them. The tab's count says the rest exists: "Designs 12 · 43 under goals".
2. **One scope control at the tab strip's end**: Project · Goals · All. Goals shows the goal-scoped records grouped under their goal, each group collapsed to the goal's title and count, its title linking to the goal page; a record naming several goals stands under each. All is the flat list with a goal chip on every row that has one. The choice is remembered per viewer; the default stays Project.
3. **Find crosses every scope.** The Find box the board has joins the Project strip: typing filters the tab's records whatever the scope control says, each match carrying its scope chip; clearing it restores the control's view.
4. **A goal page shows its own records only**, as today, and each tab ends with one muted line, "12 project-wide designs →", to the Project tab with the control on Project.
5. **Scope is a fact the human can change.** A record's page shows "About: this project" or "About: g1-s23 · its title" in the facts row with an Edit that opens the goal picker (several goals allowed) and writes the head's Goals line through the existing goals route; intent and doctrine records show no such edit, since they never name goals.
6. **The same rule wherever counts appear**: the Overview's memory tiles count the project's own records with "+43 under goals" small beneath; the Partner's memory index lists the project's own records first and then the goal-scoped ones grouped by goal, trimmed by whole groups from the end.

## Not in this slice

A new kind or home; scope on intent and doctrine; a goal page listing project-wide records inline.
