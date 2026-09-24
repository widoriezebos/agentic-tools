# g1-s36 Pick a goal, never type one

- Kind: design
- Id: 01M399JG8XXXNZDA73K88172SW
- Status: done
- Cites: 01M34HS374KF1RSS3EWKBGD2WE

Revision 1, 2026-09-24, Claude on Fable, on Wido's review of the New goal sheet: "the 'Unblocks' is a free text field; so we can enter anything there. I think we need a bit of help here to avoid mistakes." Ruled "design for this and get it implemented". Frontend only; the act's request is unchanged.

## The principle

A field that must hold an existing thing is never free text. The human recognises the thing from a list, sees what they chose, and learns the consequence before they commit. A field that holds free words stays free but shows what already exists, so a new word is a choice and not a typo.

## The design

1. **Unblocks is a picker.** A combobox (the ARIA pattern: , a listbox of options, arrow keys, Enter to choose, Escape to close) over the goals the board has already loaded, live and closed alike. Typing filters by id and by the intent's words; each option shows the id in mono, the intent's first words on one line, and the lane as a chip. The choice becomes a chip in the field, the id and the intent's first words with an × to clear. Empty stays allowed.
2. **Only a real goal gets in.** Text that matches no goal shows "no goal named <text>" under the field and the form cannot be sent; a goal that is done or abandoned is listed but greyed and, if chosen, refused with "already done, nothing to unblock" (or "abandoned"); the goal being opened cannot unblock itself.
3. **The consequence is said.** The hint under the picker: "This goal parks with the chosen one recorded as its blocker, and returns when that one is done."
4. **Labels suggest, and mark the new.** The field stays free words, separated by space or comma as today, but as the human types it suggests the labels already on the board (from the loaded rows), each accepted label becomes a chip, and a label the board has never seen is chipped with a small "new" mark so a typo is visible before it is a label nobody meant. Backspace on an empty field removes the last chip.
5. **The pattern is shared.** The picker and the token field are two small components under `src/shell/` so the priority sheet, the record sheets' goal references and later forms use the same ones. Tokens only; keyboard complete; announced to a screen reader through the ARIA pattern.

## Not in this slice

Goals not yet loaded by the board (the picker sees what the page sees; the board loads every live and closed goal already); server-side search.
