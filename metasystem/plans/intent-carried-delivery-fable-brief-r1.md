# Fable: exceptional delivery implementation correction

Root authors; Fable critiques, no product edits. Read intent-carried-delivery-design.md
and relevant parent intent-workflows.md IW-4 plus actual cited source. Full public
intent redesign is user-authorized, as is machinery bypass; stale stop hook ignored.

Criterion verbatim:
> Would an implementer working from this design build **step 1** DIFFERENT, or WRONG, because of this finding?
> Does step 1 WORK, and is it SAFE, without this finding?

Challenge the smallest correction: projected endpoint -> projected candidate patch,
immutable subject before carry, real main staging under existing lock then existing
carried script. Does this preserve actual authority, shared state and crash replay?
Is some stated helper semantics false, or some mechanism unnecessary? Identify
actual first-use blockers only, not optional general import/recovery frameworks.

Report stable IC-C1 onward, severity/material, answers to both tests, exact source
and smallest amendment, reviewed SHA, unexamined scope. Maximum two rounds, round1;
<=24tools/10minutes, <=1500words. Do not spawn agents, build/test, change code/goals/
receipts, commit/push or read conf.local. Only output allowed:
plans/intent-carried-delivery-fable-report-r1.md.
