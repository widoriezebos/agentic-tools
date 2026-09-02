# Appendix: How the Metasystem Works

**Four pictures of one system: how work moves, how records hold it together, who may move it and how it learns.**

This appendix draws the paper's functional design. A reader should be able to follow how the metasystem works with people from these four diagrams and their descriptions alone, without the seventeen chapters. The vocabulary is the paper's; the closing section says where the accompanying software's names and reach differ today, because the paper's design goes beyond what the software currently does.

Two conventions hold in every diagram. Solid arrows carry the work forward, and the work travels as records: each stage writes what it did and the next stage reads it, so nothing authoritative depends on a private conversation between actors. Dashed arrows are exchanges with people: questions going to a person, reports written for a person and the decisions a person records.

A few terms carry the whole appendix. A candidate is a version of a change that still has to be judged. An enforced rule is a rule with the power to refuse an action at the moment it happens, when a required fact is missing; its refusal names the fact. A budget is the recorded limit a human attaches before work starts, and it is complete: how long, how many attempts, how much machine time, how much at once. Four risk questions set how much evidence and spending a change deserves: how severe the harm could be, how unfamiliar the approach is, how many users or systems it can reach and how much change has accumulated since the last broad look. And recorded intent holds more than a wish: the outcome, its constraints, the open questions, the freedoms left to construction and the observations that will later count for or against it in production.

## One change, end to end

The first diagram follows one consequential change from a request to live behavior. Read it top to bottom. A sitting is held only when intent or a design must first be shaped; mechanical work with clear intent skips it. The budget is attached by a human when the work is claimed, and reaching any of its limits, before construction or in the middle of it, is a stop, never a nudge: the work keeps its records and waits, and only a recorded human decision extends, narrows or abandons it. Each examination round uses a fresh examiner, one that never saw the builder's path, and whoever helped shape the design in a sitting is disqualified from examining it. A human reviewer, independent of construction, may demand more evidence, may narrow or stop exposure through the releaser and may authorize acceptance; the custodian is still the actor that performs acceptance, and only after every required fact is present. When a fact is missing, the custodian does not send the work anywhere: it leaves the candidate unaccepted, and the refusal names the fact so its owner can supply it, whether that owner is a builder, an examiner or the authority itself. This picture shows the full configuration a consequential change deserves; a small, low-risk change staffs fewer actors while the same protections hold at the boundaries.

```mermaid
flowchart TD
    REQ["A request arrives"] --> SIT["Sitting: a working conversation,<br/>held when intent or a design<br/>must first be shaped"]
    REQ -->|"clear mechanical work<br/>needs no sitting"| INT
    SIT -->|"ends in records"| INT["Recorded intent: outcome,<br/>constraints, open questions and the<br/>observations that will count<br/>for or against it"]
    INT --> CLAIM["A human claims the work<br/>and attaches its complete budget"]
    CLAIM --> DIS["Dispatch delegate chooses the<br/>configuration within the recorded<br/>budget and risk bounds"]
    DIS -->|"admits work<br/>inside the budget"| B["Builder constructs<br/>a candidate"]
    DIS -->|"a budget limit<br/>is already reached"| STOP["Work stops with its records kept"]
    B -->|"a budget limit is<br/>reached mid-work"| STOP
    STOP -.->|"the question, with<br/>its records"| RA
    RA -.->|"recorded decision: extend,<br/>narrow or abandon"| STOP
    B -->|"candidate and evidence<br/>written to the record"| EX["A fresh independent examiner<br/>reads the record and tries<br/>to break the candidate"]
    EX -->|"material finding<br/>written to the record"| B
    EX -->|"results written<br/>to the record"| CU["Custodian checks the chain:<br/>exact candidate, complete evidence,<br/>every enforced rule satisfied"]
    CU -->|"a fact is missing: the candidate<br/>stays unaccepted and the refusal<br/>names the fact's owner"| WAIT["The owner supplies the fact"]
    WAIT -->|"a repair becomes<br/>a new candidate"| B
    WAIT -->|"missing examination<br/>runs"| EX
    WAIT -.->|"a missing ruling<br/>is requested"| RA
    STOP -->|"a resuming decision<br/>re-enters through dispatch"| DIS
    CU -->|"performs acceptance"| ACC["Accepted candidate"]
    ACC --> REL["Releaser exposes it to a small<br/>part of live traffic and compares<br/>observations with the recorded bounds"]
    REL -->|"inside bounds:<br/>expand"| LIVE["Live, under standing watch"]
    REL -->|"a bound is crossed"| BACK["Contain or roll back,<br/>evidence preserved"]
    B -.->|"value question found:<br/>ruling request"| RA["Responsible authority:<br/>a scoped person or body, acting as<br/>intent-holder, legislator, judge<br/>or reviewer by decision"]
    RA -.->|"ruling becomes<br/>recorded intent"| INT
    RA -.->|"review may authorize<br/>acceptance"| CU
    RA -.->|"review may narrow<br/>or stop exposure"| REL
```

## Records hold the roles together

The second diagram shows the working roles around the one authoritative record. This is the system's coordination: each role reads only the view its task needs, writes what it did and never depends on a conversation with another role, while the full history stays preserved for recovery and audit. Any of these roles can be held by a person or by machinery under the same permissions; what the diagram fixes is the role's reach, not the kind of worker in it. The examiner's view is the deliberate one: it receives the intent, the candidate and the evidence, never the builder's path of discarded attempts, because that path is the framing it must challenge. Two standing observers live here as well. The liveness watcher tells a stopped worker from a slow one and may replace it, but may never erase why it stopped. The narrator turns the record into the one output written for people; it changes nothing, and every claim in its report stays traceable to the record it came from.

```mermaid
flowchart TD
    R[("The one authoritative record:<br/>intent and rulings, work state,<br/>candidates and evidence,<br/>decisions and history")]
    D2["Dispatch delegate"] -->|"writes the configuration<br/>and each admission"| R
    R -->|"budget, risk bounds<br/>and priorities"| D2
    B2["Builder"] -->|"writes candidates, progress<br/>and discoveries"| R
    R -->|"intent, rulings and the<br/>parts it may change"| B2
    EX2["Independent examiner"] -->|"writes findings bound<br/>to the exact candidate"| R
    R -->|"intent, candidate and evidence;<br/>never the builder's path"| EX2
    CU2["Custodian"] -->|"writes the acceptance"| R
    R -->|"the complete chain"| CU2
    REL2["Releaser"] -->|"writes observations,<br/>containment and rollback"| R
    R -->|"the accepted candidate<br/>and its release bounds"| REL2
    LW["Liveness watcher"] -->|"writes timeouts and replacements;<br/>may never erase history"| R
    R -->|"progress marks<br/>and heartbeats"| LW
    R -->|"what the report needs"| N["Narrator"]
    N -.->|"a report a person can act on,<br/>every claim traceable"| RA2["Responsible authorities,<br/>each scoped to a domain"]
    RA2 -.->|"rulings, budgets and<br/>intent revisions"| R
```

## The states of a change and who may move it

The third diagram is the paper's conceptual lifecycle of one change. It is not a single machine inside the software; the closing section maps the software's own state records onto it. Every transition is a recorded act, bound to the actor and evidence behind it. Four boundaries carry more than that: acceptance, release, expansion and the budget stop each stand under an enforced rule that refuses or halts when a required fact is missing or a limit is reached, and the refusal names what happened. The separations show in the labels: the actor that builds never appears on an accepting arrow, the actor that accepts never builds or examines, and an implementation defect found after containment returns through construction and a fresh examination, never around them. Containment can also reveal a fault that is not the implementation at all: a check that proved too weak, a release bound set wrong or an intent that harms the people it governs, and each of those returns to its own owner, the last one to the authority as an intent revision. There is no final state after Live on purpose: the watch stands for as long as the intent's conditions do, and care continues.

```mermaid
stateDiagram-v2
    [*] --> IntentRecorded : the authority records intent from a sitting or a request
    IntentRecorded --> Queued : a delegate of the authority takes it up as one deployable piece
    Queued --> Claimed : a human attaches the complete budget
    Claimed --> UnderConstruction : the dispatch delegate admits work inside the budget
    Claimed --> StoppedAtBudget : an enforced rule closes admission at a budget limit
    UnderConstruction --> StoppedAtBudget : an enforced rule stops work at a budget limit
    StoppedAtBudget --> Claimed : a recorded human decision resumes or narrows
    StoppedAtBudget --> [*] : the authority abandons the outcome, records kept
    UnderConstruction --> UnderExamination : the builder records the candidate and evidence
    UnderExamination --> UnderConstruction : the examiner records a material finding
    UnderExamination --> Accepted : the custodian verifies the chain and accepts
    Accepted --> Releasing : the releaser exposes a small part
    Releasing --> Live : the releaser expands while observations stay inside the bounds
    Releasing --> Contained : the releaser acts when a bound is crossed
    Live --> Contained : the releaser's standing watch detects harm or drift
    Contained --> UnderConstruction : an implementation defect returns to a builder
    Contained --> IntentRecorded : a harmful intent returns to the authority for revision
```

## How the system learns

The fourth diagram shows the return paths that make one failure change future behavior. An observation that no rule can interpret goes to a person through the narrator's report, and the person decides where the fault sits: the implementation, a weak check, a wrong bound or the intent itself, each with its own owner. After containment, a builder assembles the incident record and investigates, keeping what was observed apart from what is suspected, and proposes a candidate lesson. The lesson is challenged before anyone acts on it. The authority then has three honest outcomes: decline with a recorded reason, keep the lesson as a ruling when no reliable check can hold it, or adopt it. An adopted lesson earns its power slowly. It enters the backlog and is built, examined and accepted like any other change. It runs first in marking mode, where it refuses nothing and the builder who proposed it reviews what it marked. An examiner then tests it with changes it must block and changes it must pass, and its owner advances it through staged activation, one step at a time, only while it behaves within its recorded bounds. Full power comes with a governance record naming the rule's owner, review date, known-bad case and appeal route, and the two ways a rule degrades part ways: a passed review date leaves a working rule enforcing until a recorded decision renews or retires it, while a known-bad case that stops failing means the check itself is broken, so an ordinary rule falls back to marking, a severe rule closes its recorded scope and either way a dated, budgeted repair record opens. One rule governs all of this when the change is to a rule itself: the current rules, the retained known-bad cases and the current authority judge the proposed replacement. A rule change never judges its own adoption. And when live evidence shows the intent itself is wrong, the return is not a rule at all: the intent-holder revises the intent, and a new version controls from then on.

```mermaid
flowchart TD
    OBS["Live observation, compared<br/>with the recorded bounds"] -->|"inside bounds"| GO["Release continues"]
    OBS -->|"a bound is crossed"| CTN["Releaser contains or rolls back,<br/>evidence preserved"]
    OBS -->|"no rule says<br/>what it means"| NR["Narrator report to a person, who<br/>decides where the fault sits and<br/>whose work it becomes"]
    CTN --> INV["A builder assembles the incident<br/>record and investigates, then<br/>proposes a candidate lesson"]
    INV --> CH["A fresh examiner challenges it:<br/>wrong cause, too broad, too narrow?"]
    CH --> AUTH{"The responsible<br/>authority decides"}
    AUTH -->|"declined, with a<br/>recorded reason"| NOCH["No change"]
    AUTH -->|"no reliable check<br/>can hold it"| RUL["A ruling, recorded with reasons,<br/>scope and a reconsideration date"]
    AUTH -->|"adopted"| BL["The lesson enters the backlog and is<br/>built, examined and accepted<br/>like any other change"]
    BL --> MARK["Marking mode: the rule refuses nothing;<br/>the builder who proposed it<br/>reviews what it marked"]
    MARK --> STAGE["An examiner tests must-block and<br/>must-pass cases; the owner advances<br/>each stage while the rule stays<br/>within its recorded bounds"]
    STAGE --> GATE["Refusal power at the boundary, under<br/>a governance record: owner, review date,<br/>known-bad case, appeal route"]
    GATE -->|"review date passes"| RENEW["The rule keeps enforcing while<br/>the authority renews it, assigns<br/>a new owner or retires it"]
    GATE -->|"the known-bad case<br/>stops failing"| BROKEN["The check is broken: an ordinary<br/>rule falls back to marking, a severe<br/>rule closes its recorded scope"]
    BROKEN --> REPAIR["A dated, budgeted repair: a builder<br/>restores a discriminating check and<br/>a fresh examiner proves it"]
    REPAIR -->|"a proven check returns<br/>the rule to power"| GATE
    OBS -->|"evidence against the<br/>intent itself"| REV["The intent-holder revises the intent;<br/>a new version controls from here"]
```

## Where the software stands today

The diagrams draw the design; the software holds part of it, in its own vocabulary. The goal ledger is the recorded intent, the backlog and the budget in one place: a goal is queued, claimed with the complete budget, parked or done, and the budget's four limits are elapsed time, attempts, reserved machine minutes and concurrent jobs. Reaching most limits closes further admission while running work finishes; an elapsed-time breach stops live work. Resuming needs a fresh complete budget from a human. Delegate jobs are the machine workers, with their own setup, running and terminal states, and a critic chain is the independent examination. Dispatch fixes the minimum examination duty from a declared hazard class. The three classes name what kind of wrongness the work could carry: purely mechanical work, work that carries design and work that can reach live data destructively. The class sets the required roles, while the four risk questions remain the judgment behind budgets and depth. The chain's closure gate refuses to close work whose required examination is missing, stale or performed by a session that saw the builder's path; it binds that evidence to the final round of work, not yet to an exact candidate tree. Closure is custody of evidence; it is not the custodian's acceptance and not a release, and the ordinary landing of a change does not yet consume a closed chain, a gap with queued backlog work against it.

The other gaps matter as much. Bounded release, production observation against intent conditions and the care machinery are the design's direction; the software does not hold them yet. The coordinator seat today combines dispatch, custody of landing and reporting in one actor, exactly the configuration Chapter 7 warns about, so until the queued guards land, that separation holds by conduct rather than at the actions. Liveness watching exists as supervision, and the narrator exists as reports and digests. Where a diagram and the software disagree, the diagram shows the design the software is being built toward.
