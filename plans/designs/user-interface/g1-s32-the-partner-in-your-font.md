# g1-s32 The Partner in your font

- Kind: design
- Id: 01M38YVXPK3DER8T29CGGVZSXE
- Status: accepted
- Cites: 01M34HS374KF1RSS3EWKBGD2WE

Revision 1, 2026-09-24, Claude on Fable, on Wido's ask: "I want a font chooser for the project partner. It should remember the font that was set across sessions. I want the font that is currently selected, but I also want a fixed width font so that it looks more like a terminal ... Is it possible to choose operating system fonts? Currently, in my terminal, I have 'Meslo LG M Regular for Powerline 24' which I like a lot. So a few popular fonts to choose from would be nice, and a font size picker." A presentation slice over the conversation; not sent to Astra: it changes no behaviour.

## Outcome

The conversation, in the drawer and on the focused page, is read in the face and size the human chose, remembered on this browser across sessions. The choice covers the Partner's answers, the human's turns and the composer's field; the rest of the interface is untouched.

## The chooser

An "Aa" control in the drawer's header, beside the help icon, opens a small popover:

1. **Face.** A list, each entry drawn in its own face with a one-line preview: *Interface* (the face the pages use), *Reading* (the reading face the documents use), *Mono* (the bundled monospace face), then the popular ones the operating system may carry: Meslo LG M for Powerline, Menlo, SF Mono, Monaco, Fira Code, Source Code Pro, JetBrains Mono, Consolas, Courier New, Georgia, Helvetica Neue. A face the browser cannot render on this machine is marked "not installed" and, when chosen, falls back to *Mono* or *Reading* by its kind, visibly. Below the list, **Other**: a field for any installed face by its name, with the same live preview and the same mark.
2. **Size.** A stepper from 12 to 28 pixels, one pixel a step, the current value shown, with the line height following the size. The preview line shows both.
3. **Reset** to the interface's own face and size.

The choice is written to this browser's storage the moment it changes and read when the page loads; losing it changes nothing but the look. A face is applied by name and nothing else: a name is letters, digits, spaces and hyphens, and anything else is refused before it reaches a stylesheet.

## Not in this slice

Fonts for the rest of the interface; uploading or bundling a font; a per-human choice kept on the server.
