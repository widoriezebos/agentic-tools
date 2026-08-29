# 11. The Economy of Machine Engineering

**Spend where expected value and risk justify it and stop where they do not.**

Chapter 1's session change is cheap to construct and expensive to decide. Production, comparison, independent examination, human judgment, gradual release, observation and recovery all consume finite resources. Cheap construction does not make dependable delivery free.

Machine engineering has an economy. More attempts can improve a result or multiply the alternatives to compare. More machinery can prevent harm or cost more than the work it protects. The governing rule is proportionality: spend where expected value and risk justify the spend and stop where they do not.

## Value before process

Suppose every session change waits for a weekly meeting. The meeting may once have served a real need: the security authority, support representative and release owner needed one place to discover conflicting concerns. If the same concerns now become visible in the recorded intent and reach the named authority when a ruling is required, the meeting's recurrence no longer justifies its cost.

Every continuing process must name the outcome it protects. It must also produce evidence that its protection is worth the time, delay, attention and machinery it consumes. A human discussion may remain the least costly way to resolve a value conflict. An automatic refusal may be cheaper and more dependable for a repeated condition.

Some protected values are hard to price exactly (avoided disclosure, preserved trust, faster recovery, clearer accountability) but they are still concrete. The discipline is to name the protected value and check whether the process actually contributes to it. When that connection disappears, the process is reduced, redesigned or stopped, and the reason is recorded so that a later incident can challenge the decision.

## Four risk questions govern verification and spending

Before the session change begins, someone must decide where one more unit of effort is most valuable. A second builder could produce another design. A fresh independent examiner could search for a different failure. More release observation could expose rare lockouts.

The same four questions that set evidence depth in Chapter 6 govern the spending side here: how severe the harm would be if the work were wrong, how unfamiliar the approach is, how many users or systems it can affect and how much change has accumulated since the last broad check. The answers stay separate rather than collapsing into an unexplained score.

Severity changes what the budget must cover. A change that can expose accounts needs room for independent judgment, controlled release and recovery even if the candidate is cheap to produce. Novelty changes the value of exploration. An unfamiliar approach may justify separate attempts or outside expertise because the first plausible design carries more uncertainty. Exposure changes the cost of learning from live use: broad reach makes a mistake expensive, so money and time move toward smaller stages, observation and response capacity. Accumulated change changes the value of a wider look. Many individually ordinary changes can move the application far enough that a broad examination becomes worth its cost.

They direct marginal spending, meaning the next unit of time or attention. The system asks whether that next unit is more valuable in construction, examination, human judgment, gradual release, care or not being spent at all. Chapter 7 gave that choice its form: picking the configuration for a change is this spending decision.

## Budgets are enforced stop rules

Suppose a builder makes two unsuccessful attempts to prevent delayed responses from reviving an expired session. A third attempt begins to repeat the first with different wording. Without a prior limit, every earlier expense becomes an argument for spending a little more: stopping now would seem to waste what has already been invested.

Work begins instead with explicit limits on time, attempts and total spend. It names the person or authority that may approve an exception, and the evidence that an exception request must contain. The limits are not forecasts of how long work ought to take. They are stop rules that prevent an uncertain loop from consuming resources without a new decision. A live release expands only while enough budget remains to observe and contain harm.

This inverts the estimate: instead of asking how long the work will take, a budget states how much the outcome is worth spending. Worth is a value question, so the budget is set by the responsible authority and recorded. Some teams call that limit an appetite. The need Chapter 4 found behind estimates (a decision with a visible statement of expected cost and uncertainty) is met by the recorded decision that sets the budget, and again at the stop rule, where an exception request must carry its evidence.

When a limit is reached, an enforced rule stops or narrows the work. The record may ask the authority to accept more cost, reduce scope, choose a known alternative, gather missing information or abandon the outcome. It does not authorize another attempt on its own. Previous spending is already gone; it cannot establish the value of future spending. Only the expected benefit and risk of the next action can do that. No new role appeared in this loop: worth is the responsible authority's ruling, the stop is an enforced rule, the exception belongs to its named judge. Chapter 7's test explains why: a hazard that a rule and a record already control does not create a role.

This boundary also makes failure visible. A stopped attempt is not disguised as ongoing progress. Repeated budget exceptions become evidence that the design, intent or cost model is wrong and needs examination of its own.

## Parallel attempts include the cost of judging

Two builders propose different session designs. One records a fixed expiry time and grants uploads a narrow continuation. The other keeps a separate activity history and derives expiry when each request arrives. Producing both may be cheap. Deciding between them requires each to be understood, challenged, compared against the ruling and examined for reversal and live behavior.

Chapter 3's production-versus-judging gap returns as a price: every additional attempt creates a judging obligation. Someone or something must identify the meaningful differences, test claims that are not shared, resolve conflicting evidence and explain why one result should displace another. If human judgment is required, attention may be the scarcest part of the exercise.

Parallel attempts are worth their cost when disagreement itself has value. They can reveal that the chosen design depends on an assumption no single builder noticed. They can explore truly different responses to a high-consequence problem. They can also increase confidence when independent paths reach the same bounded result. They are not worth it when several versions differ only in surface form or when the acceptance criteria already make one routine construction obvious.

The decision comes before broad generation. Estimate what another attempt could teach or solve; compare that with the cost of producing and judging it. If no one has the budget or authority to decide among five candidates, producing five only piles up unfinished work.

## Small changes can be risky; large changes can be routine

The final repair to session expiry may change a single comparison: a response received after expiry no longer counts as activity. That line sits on an authorization boundary used by every signed-in account. Reversing the comparison can keep unattended sessions open or eject active users. Its small size says little about its economic importance.

A correction to thousands of low-impact help pages can be much larger and remain routine: visible, reversible, separated from money, identity and essential work, and checkable by machinery at far lower cost than a person reading every changed line. Chapter 6 drew the evidence conclusion: line count says little about what a change deserves. The economic conclusion is the same: the four questions, possible harm, unfamiliarity, reach and accumulated change, set the budget, the examination depth and the starting release bounds. Treating size as risk would lavish attention on bulky harmless work while letting a compact authorization mistake pass cheaply.

## When the machinery is not worth its cost

A person writes a disposable program to compare two local data files once, examines the result and throws the program away. Building durable role separation, continuous observation, an acceptance custodian and a learning system around that act could cost more than repeating the comparison by hand. Little reuse exists, the possible harm is low, and any mistake is easy to notice and reverse.

This is the break-even question. On one side sit the costs of building and changing the delivery machinery, operating it, examining its own results, preserving records and supplying human judgment. On the other sit expected reuse, avoided harm, faster recovery and learning that future work can retain. The machinery is worth building when its expected protection and repeated value exceed its total life cost. The possibility of automation alone does not justify it.

One-off prototypes, disposable explorations, small low-risk tools and short-lived software often fall below that point. Manual construction and judgment can be cheaper and clearer. A prototype that begins handling identity, money, private data or a decision people depend on can cross the point even before it is large. Repetition also changes the answer. A small release performed hundreds of times can justify durable controls that no single release would repay.

The argument allows a rational decision not to build the system, without exempting small work from thought: even the disposable comparison needs care to match its consequence.

## Use no more machinery than the work needs

A team facing the first low-risk version of a tool may need only recorded intent, a discriminating check and a reversible release. Later, the tool gains users and begins storing private information. The cost balance changes, and independent examination, narrower authority, durable custody and production care become justified.

The right amount of machinery is the smallest that still protects the relevant outcome at the expected level of risk and reuse. Structure is added when evidence reveals a new hazard, a repeated cost or a valuable learning opportunity. It is removed when the need it served disappears or a cheaper protection proves dependable. This keeps the delivery system proportional instead of allowing it to become a product whose maintenance overwhelms the application it serves.

The proposed shift in engineering ownership is directional; it does not carry the same weight everywhere. Repeated or consequential software makes the system that builds and cares for it a durable asset. A script used once may need almost none of that asset. Both choices can follow the same economic discipline.

## Measuring the system, not activity

Suppose the closing report says that three candidates were produced, eleven checks ran, and hundreds of messages passed between workers. Those numbers can explain cost, but they cannot show that users are safer, that active work survives or that the delivery system deserves to continue in its present form.

Useful measures connect total spend and delay to delivered intent, harm that escaped the controls, recovery time and learning that stays available for later work. They show how much construction and examination cost, how long required judgment waited, how often harmful behavior escaped, how quickly protected outcomes returned and whether a repeated change becomes cheaper without becoming less safe.

Activity measures still help diagnose waste. Rising attempts with unchanged outcomes may expose a weak builder or unclear intent. Growing examination time may reveal accumulated complexity. But counts of generated code, messages or attempts measure activity; they do not measure value. The economic account closes only when activity connects to an outcome the system exists to protect. The narrator's report is where that connection becomes visible, and the responsible authority reads it to decide whether the machinery still deserves its cost.
