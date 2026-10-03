# parked-goals-return-on-their-trigger

- State: approved
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="Changes when goals re-enter the queue for every seat; a trigger that fires wrongly floods the queue or resumes work a person parked on purpose, one that never fires hides work forever"
- Tier: 2
- Intent: A parked goal comes back by itself when what it waits for has happened, and nothing can be parked without saying what that is (Wido 2026-10-03, on 32 goals waiting: 'the trigger is interesting, do we already have a trigger mechanism? How can these be automatically resumed if they were blocked by a goal that was just completed?'; 'create a goal and a proper design. This needs to follow machinery: we need to do this with the care it deserves'; and his rule of 2026-09-16: a park needs a trigger, else conclude). Today only one trigger is mechanical: goal block G --on G2 lifts the park when every goal it waits on is done (internal/goal/file.go:416-418). A park 'until X happens' is free text nobody evaluates, so on 2026-10-03 the parked list held 32 goals: 8 with no trigger at all, several whose condition had long happened, 3 real dependencies. Wanted: a park records its trigger as data, of a known kind (another goal done; a date; an event the machinery records, such as a refusal code or a steward pattern; a person's review by a date); the machinery evaluates it and resumes the goal or puts it in front of the person with the reason; a park with no trigger is refused with the plain alternative (conclude it); the person sees parked goals grouped by what they wait for, separate from goals blocked by a dependency.
- Origin: main
- Next step: Proper design first. Part 1 intent and use cases, drawn from the real park reasons audited on 2026-10-03 (agentic-tools-evidence/parked-audit-20261003/part-1..3.md): classify each real trigger by kind. Threat model and the rabbit-hole risks named up front (for example a general event language; triggers that fire wrongly and flood the queue; resuming work a person parked on purpose, against R-142-m1e). Part 2: the trigger record and its kinds, who evaluates (the steward, as one pattern under steward-acts-on-behaviour-patterns' rule contract), what resume means for approval, priority and claims, the verbs (goal pause with a trigger; every UI act is a verb), the UI grouping (input to the backlog page and fleet-page-redesign), migration of existing parks. Estimates per slice in the design (goal-budget-follows-its-plan). Astra critique at the goal's tier; then build in slices through the lane.
- OpenedAt: 2026-10-03T09:48:22Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-03T09:48:38Z revision=2 opid=HQX5R2Q2Q250HVPEAD2WBB92HJ-m1e-718ba0eb authority=proven digest=66e5f2e894bf654ab2ebc8a2feabc856589bf16500cde3407c06ce29d4481559 episode=2

History:
- 2026-10-03T09:48:22Z 3ESZ70BNBDCKJ9EG6VGWYEP3KC-m1e-718ba0eb open actor=human:Wido targets=parked-goals-return-on-their-trigger
- 2026-10-03T09:48:38Z HQX5R2Q2Q250HVPEAD2WBB92HJ-m1e-718ba0eb approve actor=human:Wido targets=parked-goals-return-on-their-trigger
Integrity: sha256=1989db958d20d5dbba67532d8f024b37e351694d7879cacf5089575e289bbcd2
