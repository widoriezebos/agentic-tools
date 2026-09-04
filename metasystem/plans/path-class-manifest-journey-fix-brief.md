Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal path-class-manifest)
Date: 2026-09-04

# Correction round on chain pcm-journey (the journey chapter)

The review (pcm-journey-cc1; dispositions in
metasystem/records/misc/path-class-manifest-journey-critique-cc1.md)
found three material slips and one weak gloss. Fix them in the
chapter only, keeping its length between 150 and 300 words:

- PCMJ-01: a carriage landing does not move work between machines.
  Gloss it as a seat landing its own records directly, without a
  review; the handover between machines was the pushed branch the
  fourth paragraph already describes.
- PCMJ-02: explain "behaviour" (the code, scripts, instructions,
  documents and configuration that make the system act) and "a
  reviewed chain" (a build examined by an independent critic until
  nothing material remains) at their first use.
- PCMJ-03: two machines, one after the other, made the three landings;
  the opening must not say one machine.
- PCMJ-04: gloss the floor as the set of paths that may never change
  without a review.

Gate: `git diff --check` clean; no token matching [A-Z]{2,}-[0-9]+ or a
7-or-more hex string in the chapter. Boundary: metasystem/docs/journey.md
only. Wall-clock budget: 10 minutes. Gap rule: stop and report a gap.
