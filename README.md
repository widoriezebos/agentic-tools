# agentic-tools

This repository builds the **metasystem**: a portable set of working rules,
scripts, and skills for repositories where agents (Claude Code, Codex, Devin)
and humans build software together. One question organizes everything here:
*what ships to other repositories, and what merely helped build it?*

## The layout answers that question

The heart of the metasystem is a team of **named roles** — coordinator,
builder, critics, investigator, verifier, judge — where every role can be
filled by a different agent on a different model, so the one reviewing is
never the one who wrote. The roles and the loop they play are explained in
plain English in [`metasystem/README.md`](metasystem/README.md).

| Folder | What it is | Ships? |
| --- | --- | --- |
| [`metasystem/`](metasystem/) | **The product.** The template that gets installed into other repositories. Its payload is an explicit allowlist; nothing else leaves. | The allowlisted parts, via `metasystem/scripts/adopt.sh` |
| [`benchmark/`](benchmark/) | **The measuring kit.** Specs, held-out graders, the scorecard extractor, rubrics, and the provisioner that stage benchmark runs to measure how well the metasystem makes agents build. It measures the product from outside, so it lives outside. | Never |
| [`development/`](development/) | **The building of it.** Design docs, analysis, finished reports, this repository's own local rules, the evidence index. | Never |
| `evidence/` | Gitignored symlink to the durable evidence store, where full run transcripts and archived mission repositories live. Hundreds of megabytes a day of raw evidence would sink git; the tracked index of it is [`development/evidence-index.md`](development/evidence-index.md). | Never |

## There is no "meta-meta system"

It is tempting to think this repository needs a third concept: the metasystem,
plus some meta-meta thing that builds it. It does not, and the vocabulary
stays two-level on purpose:

- **The metasystem** is equipment. It plays the *same role here as in any
  repository that adopts it*: hooks fire at turn ends, supervision watches
  processes, dispatch runs delegates, receipts accumulate, retros tune rules.
- **This repository is simply an adopted repository whose product happens to
  be the metasystem's own source.** Its `metasystem/plans/` holds this
  project's plans, like any project's. Its `metasystem/artifacts/` holds this
  project's live runtime state, like any project's — and transfers to no one.

What people mistake for a third layer is the ordinary software boundary
between a **source tree** and a **distribution**. That boundary is enforced
here, not narrated: the payload is an allowlist (a forgotten file stays home),
the shipped files are swept for any trace of this repository, and one rule is
audited mechanically — *the kit may call into the metasystem; nothing shipped
may reference outward.*

Every path in `metasystem/` carries one of three verdicts — SHIPS,
PROJECT-STATE, or RUNTIME — in
[`development/metasystem-inventory.md`](development/metasystem-inventory.md),
each with the deciding rule.

## I want to…

- **Install the metasystem in a fresh repository** →
  `metasystem/scripts/adopt.sh <target> --runtimes claude,codex` and follow
  [`metasystem/docs/project-adaptation.md`](metasystem/docs/project-adaptation.md).
- **Install it in a repository that already has agent rules** → follow
  [`metasystem/docs/metasystem-reconciliation.md`](metasystem/docs/metasystem-reconciliation.md),
  written for an agent to execute with human review points.
- **Upgrade an adopted repository to a newer template** → the upgrade
  procedure in the same reconciliation manual (the recorded adoption SHA and
  the three-bucket rule). Currently a documented manual procedure; a script
  is on the backlog.
- **Run a benchmark** → [`benchmark/README.md`](benchmark/README.md).
- **Change the metasystem itself** → work in this repository under its own
  rules; the gates of record are `metasystem/scripts/validate-metasystem.sh`
  and `benchmark/validate-kit.sh`, and nothing is pushed on red.
