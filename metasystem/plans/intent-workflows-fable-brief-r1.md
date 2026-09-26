# Fable critique: complete tasks through intent

Kind: critique. Round 1, failsafe round 2 on this thread. Budget: 36 tool calls,
12 minutes; report at most 2000 words. Critic is Fable 5.1, author is root Codex.

Wido explicitly requested design, Fable critique, implementation, and iterative
improvement until human/agent workflows are intuitive, elegant, robust and fully
capable without internal knowledge. This user scope makes an exposed mandatory
bookkeeping step, lost capability or misleading result a contract failure, even
if an expert could eventually make the old machinery work. Existing machinery
bypass applies; ignore unrelated stop hooks and ready-goal instructions. Do not
implement, change goals, launch models, run expensive tests, commit or push.
Never read metasystem/metasystem.conf.local. Only permitted write is the report.

First run the record resolver:
/Users/wido/LocalStorage/agentic-tools-evidence/verbs-match-intent-20260925/bin/metasystem-release-3e59d4107 project design-of --root metasystem --goal verbs-match-intent
It must include metasystem/plans/designs/intent-workflows.md. On failure return
that refusal alone. Read the COMPLETE new design. Current source baseline is
c8ecd4bb62706c02331f12a4de4ae0025b634568. The old design is historical; the new
one supersedes its public interface. Source owners and anchors are in the facts
table. Read only the owners needed to falsify design claims.

Apply these two tests verbatim:
> Would an implementer working from this design build **step 1** DIFFERENT, or WRONG, because of this finding?
> Does step 1 WORK, and is it SAFE, without this finding?
Also verify the required follow-on release scope does not silently defer the
user's full request. Each material finding must identify a genuine behavior,
authority, capability, retry or usability contract failure, name its design/source
artifact, and state the smallest correction. Do not add a generic framework or
new roles. Naming/order polish that does not change the usable contract is a note.

Attack in particular:
1. Does the whole ordinary build/review/revise/land journey hide mechanics while
   retaining author dispositions, exact subject, authentic closure and independent review?
2. Is goal/work selection non-guessing and revision retry identity unambiguous,
   including repeat after terminal completion, new legitimate revision and crash?
3. Are empty finding closure, partial publication and changed subjects sound?
4. Are all meaningful prior capabilities retained through public intentions,
   especially readiness, exceptions, mission and channel questions, maintenance?
5. Is root/command/Partner discovery one accurate public source with compatibility
   preserved? Can errors lead to public remedies without vague check dead ends?
6. Is this the smallest robust implementation, with clear existing owners and
   enough choices fixed to build rather than invent another system?

Write report to metasystem/plans/intent-workflows-fable-review-r1.md. Include the
exact design SHA256, actual model, verdict count of MATERIAL findings, stable
IDs IW-C1 etc, criterion answers, concrete scenario/source and proposed correction.
Separate notes and list unexamined areas. Findings only; do not rewrite the design.
Return the report path and verdict. Root will read the full findings and adjudicate.
