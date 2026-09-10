Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal hp-terminal-grade-for-stopping-acts)
Date: 2026-09-10

# Fold round thirteen: a journal entry is never an authority credential

Follow-up round on chain hp-terminal-build1-20260909; your worktree
carries round twelve. The tenth critic (hp-terminal-crit10-20260909)
showed that round twelve's fix for HPT-28 went the wrong way, and the
seat accepts that: this round reverses it. Two findings fold; nothing
else.

## HPT-31 (high): round twelve made journal text mint human attribution

Recovery now stamps the stored "by" name into the replayed actor with
no grade attached, so a journal entry alone produces a
human-attributed ledger record (Parked.By "human:<name>", a history
line attributed to that human), after which only a terminal-grade
human proof can lift the park. The critic proved it with the round's
own test, which fabricates the entry through CreateEntry with
by="Wido" against a goal never parked live. The grant is wider than the
three stopping verbs: recovery rebuilds done too, and done's three
human-only gates test the human name alone, so a stored done entry
with a "by" now concludes goals it could not conclude before. The
file's own doctrine comment says a journal remains evidence of intent
and never an authority credential. That doctrine wins.

The rule, decided: recovery never carries a human name into a replayed
actor. A journal entry whose intent carries a "by" name is a
proof-bearing act; recovery refuses to replay it, for every verb
(park, unpark, release, done, and any other verb whose intent stores a
"by"), closing the entry rejected with the explicit cannot-replay
sentence and the typed condition where one exists, exactly as round
eleven does for the verbs' own conditions. It writes nothing. An entry
without a "by" name replays through the real verb as an agent act,
with round eleven's conditional refusals unchanged. So a human's
stranded park is neither downgraded to a machine park (HPT-28's harm)
nor minted from text (HPT-31's harm): it is re-run at the human
boundary. Tests: (a) a stranded entry with by="Wido" for park is
refused, nothing written, the entry closed rejected with the wording;
(b) the same for done; (c) an entry without a name for a machine's own
release still replays and heals; (d) round eleven's conditional
refusals unchanged. Replace round twelve's test that asserted the
recovered park reads human:Wido.

## HPT-33 (note): the doctrine comment and the code agree again

internal/goal/recover.go's comment at the policy-aware entry point
("a journal remains evidence of intent and never an authority
credential") is now true again; make sure the code beside it says the
same and no comment still describes round twelve's behaviour.

## HPT-32 (note, no change)

Three command-package tests fail on this tree and identically on
unmodified main (TestLandingTestReceiptCanonicalCLIUsesSectionSelector,
TestProcessClassifierDataFailureRepairsThenRetriesTheRequestedVerb,
TestFrozenPublicVersionOneCorpusRunsAllSixCasesThroughFirstTransitionWorker);
they are 1b12f534's own and not this chain's. Do not touch them.

## Mandate

1. HPT-31 and HPT-33 as above.
2. Nothing else changes.

## Proof

internal/goal recover, park, unpark, release and done tests;
internal/humanauthority; gofmt, vet. Report the round as your own.

## Constraints

Wall-clock budget: 15 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
