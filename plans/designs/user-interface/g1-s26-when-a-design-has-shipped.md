# g1-s26 When a design has shipped

- Kind: design
- Id: 01M36T7VYK7NPNX4EPRDK8MXSB
- Status: done
- Goals: browser-interface
- Cites: 01M348YTJ2EY11F37Y5KBVNSER

Revision 1, 2026-09-23, Claude on Fable, proposed on Wido's question: "At project level designs, how do we know that the design has shipped? ... there is a loose connection between this project level design and the goals that come from it. I don't think that's a hard association." Accepted by Wido the same day: "ok, approved. go".

Built as one slice, 2026-09-23.

## What is true today

A design's `Status: done` is set by hand, and nothing checks it. The only link runs one way: a design's `Goals:` line names ledger goals. A goal has no field naming its design, and a design's `## Slices` are prose, not goal ids. In this checkout it is looser still: the interface work bypasses the ledger, so the interface slice designs name no goals at all.

Two kinds of design want different answers. A **standing design**, such as the interface master design, never becomes done; it is amended or superseded, and the pane must not nag about it. A design with no goals and `Status: accepted` is read as standing. A **design of a piece of work** ships when its work does, which is knowable only if the association is hard.

## Step 1

1. **Derive, show, confirm.** For every design that names goals, the pane reads their ledger states and shows the count: "2 of 3 goals done" on the Designs tab, and on the design's page a "Work" section listing the goals with their states. When every named goal is concluded, the design shows "all work done" and one control, "Mark done", which uses the status route that exists. The status stays a human word; the evidence is in front of them and the act is one click.
2. **The association is made by the machine, not typed.** On a design's page, "New goal for this design" opens a goal through the existing open route, its intent pre-filled from the design's title, and then appends the new goal's id to the design's `Goals:` line itself. Nobody types an id.

Both are interface and resolver work; no engine floor is touched.

## Later, when it hurts

- A `Design:` line in the goal file, so a goal names its governing design and the resolver reads both directions. That touches the goal package on every seat; step 1 shows whether the one-way link plus the machine-made association is already enough.
- Slices as goals: a design's slice list naming the goal that carries each slice, so "shipped" can be read per slice.
