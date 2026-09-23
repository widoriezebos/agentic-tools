import type { TagStyle } from "@codemirror/language";
import type { KeyBinding } from "@codemirror/view";
import { tags } from "@lezer/highlight";

/**
 * What the editor looks like, as data.
 *
 * The widget is CodeMirror, which paints itself from JavaScript rather than
 * from a stylesheet: its rules are built into a style element at runtime. That
 * is why the ground, the caret, the selection and every Markdown tag are
 * written here and not in reading.css — but they are written under the same
 * rule the stylesheet obeys, which is that a colour is a token and nothing
 * else. Every colour below is a `var(--ms-…)` from src/tokens.css, so light
 * and dark follow the theme without this file knowing which one is on, and
 * styling.test.ts refuses a literal that creeps in.
 *
 * The sizes are the reading view's own type scale, read off reading.css so
 * that a heading being typed is the size it will be once it is read. There is
 * no size token to share: tokens.css is the colour table, and tokens.test.ts
 * asserts it holds the colours and the layers and nothing else.
 *
 * Nothing here touches the DOM, so the guards can read it without a browser.
 */

/** The mono face, as reading.css writes it for .ms-md-pre and .ms-editor. */
const MONO = '"JetBrains Mono Variable", ui-monospace, SFMono-Regular, Menlo, monospace';

/**
 * The reading view's headings, from .ms-md-h1 through .ms-md-h4 in
 * reading.css. The fifth and the sixth level are rendered with the fourth's
 * styles there, and are the fourth's size here for the same reason.
 */
const HEADINGS = [
  { size: "24px", line: "32px" },
  { size: "20px", line: "28px" },
  { size: "16px", line: "24px" },
  { size: "14px", line: "20px" },
  { size: "14px", line: "20px" },
  { size: "14px", line: "20px" },
];

const HEADING_TAGS = [tags.heading1, tags.heading2, tags.heading3, tags.heading4, tags.heading5, tags.heading6];

/**
 * Markdown, styled as it is typed.
 *
 * The order matters where two rules can land on one span: a marker carries
 * both its heading's tag and processingInstruction, and the later rule wins
 * the properties they share. So the markers come last and are muted, while the
 * size the heading rule gave them stays — which is the whole point: a `#` that
 * is the size of its heading and quiet enough to read past.
 *
 * `list` is deliberately unstyled. @lezer/markdown gives that tag to every
 * child of a list, not to the bullet, so muting it would mute the text of
 * every list in the document; the bullet is a ListMark and is muted below with
 * the other markers.
 */
export const HIGHLIGHT: TagStyle[] = [
  ...HEADING_TAGS.map((tag, index) => ({
    tag,
    fontSize: HEADINGS[index].size,
    lineHeight: HEADINGS[index].line,
    fontWeight: "600",
    color: "var(--ms-text)",
  })),
  // A table's header row, which carries the plain heading tag.
  { tag: tags.heading, fontWeight: "600", color: "var(--ms-text)" },
  { tag: tags.strong, fontWeight: "700", color: "var(--ms-text)" },
  { tag: tags.emphasis, fontStyle: "italic" },
  { tag: tags.strikethrough, textDecoration: "line-through", color: "var(--ms-text-3)" },
  // Inline code and the text of a fenced block, on the ground the reader gives
  // them. No padding: the editor measures characters to place the caret, and a
  // box that is wider than its text moves the caret away from the letter it is
  // in front of.
  { tag: tags.monospace, fontFamily: MONO, backgroundColor: "var(--ms-surface-2)", color: "var(--ms-text)" },
  { tag: tags.link, color: "var(--ms-accent)" },
  { tag: tags.url, color: "var(--ms-accent)", textDecoration: "underline" },
  { tag: tags.labelName, color: "var(--ms-text-2)" },
  { tag: tags.string, color: "var(--ms-text-2)" },
  { tag: tags.quote, color: "var(--ms-text-2)", fontStyle: "italic" },
  { tag: tags.atom, color: "var(--ms-accent)" },
  { tag: tags.comment, color: "var(--ms-text-3)", fontStyle: "italic" },
  { tag: tags.contentSeparator, color: "var(--ms-text-3)" },
  { tag: tags.escape, color: "var(--ms-text-3)" },
  { tag: tags.character, color: "var(--ms-text-3)" },
  // The `#`, the `*`, the `>`, the `-`, the brackets and the fence: every mark
  // the Markdown is made of rather than made up of.
  { tag: tags.processingInstruction, color: "var(--ms-text-3)", fontWeight: "400" },
];

/**
 * The editor itself: the ground it is written on, the caret, the selection,
 * the line being typed on, and the focus ring the rest of the shell uses.
 *
 * Every selector carries `&.cm-editor` rather than standing alone. CodeMirror
 * mounts its own base theme first and some of its rules are written against
 * two classes on the wrapper; a bare `.cm-content` here would be out-specified
 * by the base theme's `&light .cm-content`, and the caret would stay black in
 * the dark theme. One class more, and the later rule wins.
 *
 * There are no gutters: line numbers, folding and the active-line gutter are
 * not in the extension list, so there is nothing in the margin to style.
 */
export const EDITOR_THEME: Record<string, Record<string, string>> = {
  "&": {
    color: "var(--ms-text)",
    backgroundColor: "var(--ms-surface)",
    border: "1px solid var(--ms-border-strong)",
    borderRadius: "6px",
    fontFamily: MONO,
    fontSize: "13px",
  },
  "&.cm-editor.cm-focused": {
    outline: "2px solid var(--ms-accent)",
    outlineOffset: "2px",
  },
  "&.cm-editor .cm-scroller": {
    fontFamily: "inherit",
    lineHeight: "20px",
  },
  // No tab-size here: CodeMirror writes one onto this element from the state's
  // own tabSize, and a rule that an inline style always beats is a rule that
  // never runs.
  "&.cm-editor .cm-content": {
    padding: "12px 0",
    caretColor: "var(--ms-text)",
  },
  "&.cm-editor .cm-line": {
    padding: "0 12px",
  },
  "&.cm-editor .cm-activeLine": {
    backgroundColor: "var(--ms-surface-2)",
  },
  "&.cm-editor .cm-content ::selection": {
    backgroundColor: "var(--ms-surface-3)",
  },
};

/**
 * The two shortcuts an editor is expected to have.
 *
 * Both are bound here rather than only on the article around the editor,
 * because a keymap inside CodeMirror is the only thing that can stop the
 * browser from doing its own thing with Mod-s while the text has focus. The
 * article keeps its own pair for the bar and the preview, and stands down when
 * this one has already answered.
 */
export function shortcuts(act: { save: () => void; cancel: () => void }): KeyBinding[] {
  return [
    {
      key: "Mod-s",
      preventDefault: true,
      run: () => {
        act.save();
        return true;
      },
    },
    {
      key: "Escape",
      preventDefault: true,
      run: () => {
        act.cancel();
        return true;
      },
    },
  ];
}
