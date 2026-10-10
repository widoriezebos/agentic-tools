# Task: revise the design briefs-carry-their-rules after critique round 1

Working Mode: Design
Revise plans/designs/briefs-carry-their-rules.md in place (keep Status: draft). Round 1 (Opus) found 4 material findings, each with a concrete change; apply them and add a "Round 1 findings and changes" table. Cap: unit <=250 production lines, <=5 units; cite code on origin/main.

1. B3 effort: `high` below 150 production lines, `xhigh` at 150 or more and for design-level units (R-90-m1; mechanisms page §11; the brief); when the composer has no size-derived value pass an empty effort so configured settings decide (launch.build.effort default xhigh, internal/config/defaults.go:387; UnitOptions.apply lets the unit option win, internal/launch/unit_named.go:285). Fix B3's test and mutation.
2. Size premise: no production-line reader exists on main; its owner is review-drops-and-design-convergence (its Decision at line 96); name it as B1's dependency. Add a changed-lines column to this page's Units table ahead of Production lines (sizesFromTable, internal/launch/admit.go:204, takes the first header containing "line").
3. B5: state the post-B5 meaning of `work build --brief` and `work revise --brief` for design-backed units (ignored, appended as supplementary text, required to match the `work brief` preview, or refused: choose the smallest that keeps a person's own brief usable) and what the B5 test asserts for each form.
4. B4: keep the hand-written defect classes (plans/lane-policies-and-helm-u1-settings.md:12) as the stated fallback until the window holds at least two units with recorded reads, then switch to record-derived advice; add a B4 test case for the fallback.
Non-material, fold if cheap: B2's `cat-file -e` needs exit-status handling for absent vs error; the template citation is :23.
Return: the material count you believe remains and the unit table with lines.
