# g1-s38 What a chip is for, and how long it lives

- Kind: design
- Id: 01M39Y32TV3GJYNQ41QT8W3HRM
- Status: accepted
- Cites: 01M34HS374KF1RSS3EWKBGD2WE

Revision 1, 2026-09-24, Claude on Fable, on Wido's finding that a draft chip outlived its sheet, and his ruling: "we need to have proper management around the life cycle of chips ... a proper mechanism for this to make sure that we do this right all over the place." Frontend only; the Partner's routes are unchanged.

## The principle

Everything that stands above the composer is an **attachment**: a piece of context the human put there by one act, which will travel with the next question. Every attachment has one source, one owner, one lifetime declared where it is made, and one way to take it back. No attachment outlives the thing it stands for, and none is cleared by a rule written somewhere else.

## The attachments

| Attachment | Made by | Stands for | Lives | Taken back by |
| --- | --- | --- | --- | --- |
| Chosen subject | "Ask about this" on a goal, record, lane or Overview item | the thing chosen | until cleared or replaced; survives navigation and expansion (g1-s29, contract 3) | its ×, or another Ask |
| Passage | "Ask" on a selection | the quoted text and its source | until the next question is sent, then consumed; or cleared | its × |
| Draft | "Ask about this" in a sheet's head | the sheet's fields as they stand | while its sheet is open; refreshed from the sheet on each send; goes when the sheet closes, cancelled or opened | its ×, or the sheet's closing |
| Page (implicit) | being on a page | where the human is | follows the page; never a chip | nothing; it is the Seeing line |

Suggested questions are not attachments: they are offers, shown while the field is empty. Message chips in the transcript are history: they never change and are never taken back.

## The mechanism

1. One store owns all attachments as one list, each with `kind`, `source`, `label`, `lifetime` (`until-cleared` | `until-sent` | `with-sheet:<name>`) and its content; the acts that make them declare the lifetime at the point of making, and nothing else decides it.
2. Two events retire attachments: **sent** retires every `until-sent` one after the question carried it; **sheet closed** retires every `with-sheet:<name>` one for that sheet. The chosen subject is retired only by its × or its replacement. A navigation retires nothing.
3. Every chip shows its × and, on hover or focus, one line saying its life: "stays until you clear it", "goes with the next question", "goes with the New goal sheet". The human never has to guess why a chip is there or when it will leave.
4. The capture composes from the list, so what the Partner is told is exactly the chips the human sees, in that order.

## Not in this slice

New kinds of attachment; attachments on the focused page differing from the drawer's (they are one list).
