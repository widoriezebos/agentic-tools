import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { markdown, markdownLanguage } from "@codemirror/lang-markdown";
import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { EditorState } from "@codemirror/state";
import { EditorView, highlightActiveLine, keymap } from "@codemirror/view";
import { useEffect, useRef } from "react";

import { EDITOR_THEME, HIGHLIGHT, shortcuts } from "./styling";
import { readNonce } from "../nonce";

/**
 * The document, as Markdown, styled while it is typed.
 *
 * This is one CodeMirror view, owned by one React component: it is built once
 * when the editor opens, torn down when it closes, and never rebuilt in
 * between. What changes between renders are the four things the page does with
 * it — what the text now says, saving, cancelling — and those are read through
 * a reference at the moment they are needed rather than baked into the view,
 * so a new handler does not mean a new editor and the caret never jumps.
 *
 * The text is the document and the marks are part of it: a `#` is shown, not
 * hidden, and is quiet rather than absent. This step sizes the headings and
 * colours the emphasis, the code, the links and the quotes; hiding a mark on a
 * line the caret has left is a later question.
 *
 * There is no toolbar, no autocomplete, no search and no linting, and no timer
 * anywhere: CodeMirror answers an event when there is one and sleeps otherwise.
 */

const ourHighlight = HighlightStyle.define(HIGHLIGHT);

export function Editor({
  value,
  onChange,
  onSave,
  onCancel,
  autoFocus = false,
}: {
  value: string;
  onChange: (source: string) => void;
  onSave: () => void;
  onCancel: () => void;
  autoFocus?: boolean;
}) {
  const host = useRef<HTMLDivElement | null>(null);
  const view = useRef<EditorView | null>(null);
  // The page as it stands now. The view is built from it once and asks it for
  // every handler afterwards, which is why this is kept current before the
  // effect that builds the view and before every effect that follows.
  const latest = useRef({ value, onChange, onSave, onCancel });
  useEffect(() => {
    latest.current = { value, onChange, onSave, onCancel };
  });

  useEffect(() => {
    const parent = host.current;
    if (parent === null) {
      return;
    }
    const state = EditorState.create({
      doc: latest.current.value,
      extensions: [
        // The shortcuts first: the default keymap binds Escape to simplifying
        // the selection, and a binding that runs before it is the only way to
        // leave the editor with the key that leaves everything else.
        keymap.of([
          ...shortcuts({
            save: () => {
              latest.current.onSave();
            },
            cancel: () => {
              latest.current.onCancel();
            },
          }),
          ...historyKeymap,
          ...defaultKeymap,
        ]),
        history(),
        // GFM — tables, strikethrough and task lists — through the language
        // the package configures for it, so nothing reaches past this
        // dependency into the grammar underneath it. No code languages are
        // registered: a fenced block is code, and which code it is is a
        // question for the reader rather than for the editor.
        markdown({ base: markdownLanguage }),
        syntaxHighlighting(ourHighlight),
        EditorView.lineWrapping,
        highlightActiveLine(),
        EditorView.theme(EDITOR_THEME),
        // Tab is not bound: it moves focus out of the editor, as it did out of
        // the text area, because a text box that swallows Tab is a trap for
        // anyone who does not use a mouse.
        EditorView.contentAttributes.of({ spellcheck: "true", "aria-label": "The document, as Markdown" }),
        // CodeMirror writes its own stylesheet at runtime, and the policy this
        // page is served under allows an inline style only with this
        // response's nonce. Without it the editor renders, silently, unstyled.
        EditorView.cspNonce.of(readNonce(globalThis.document)),
        EditorView.domEventHandlers({ beforeinput: typeOverASelection }),
        EditorView.updateListener.of((update) => {
          if (update.docChanged) {
            latest.current.onChange(update.state.doc.toString());
          }
        }),
      ],
    });
    const created = new EditorView({ state, parent });
    view.current = created;
    if (autoFocus) {
      created.focus();
    }
    return () => {
      view.current = null;
      created.destroy();
    };
    // Built once. Everything that changes is read through `latest`.
  }, [autoFocus]);

  // The text the page holds, when it is not the text the view holds. This is
  // the page refusing a keystroke — typing while a save is in flight — and
  // putting back what it kept, which is what the text area did before it.
  useEffect(() => {
    const created = view.current;
    if (created === null) {
      return;
    }
    const held = created.state.doc.toString();
    if (held === value) {
      return;
    }
    created.dispatch({ changes: { from: 0, to: held.length, insert: value } });
  }, [value]);

  return <div className="ms-editor" ref={host} />;
}

/**
 * Replacing a selection by typing over it, done here rather than by the browser.
 *
 * CodeMirror normally lets the browser edit its contenteditable and reads the
 * result back, which is what keeps input methods and autocorrect working. But
 * when the text being replaced carries more than one style — a heading and its
 * `#`, a word and the emphasis around it — Chromium tries to carry that style
 * over to what is typed, and it carries it by writing a `style` attribute onto
 * the line. This page is served under a policy with no `style-src-attr`, so the
 * attribute is refused, and the console says so every time a human selects
 * across two styles and types. Nothing is lost when it happens — the attribute
 * never applies and the editor repaints from its own state — but a console that
 * cries wolf on an ordinary edit is a console nobody reads, and the answer is
 * not to let a style attribute through: it is to not make one.
 *
 * So the one case that provokes it is taken here: plain typed text, over a
 * selection that is not empty. The change is dispatched, the default is
 * prevented, and the browser never edits the DOM, so there is no style to
 * preserve and nothing to refuse. Everything else — every other input type,
 * every empty selection, and every keystroke of a composition, where the
 * browser's own editing is the point — is left exactly as it was.
 */
function typeOverASelection(event: InputEvent, held: EditorView): boolean {
  if (event.inputType !== "insertText" || event.data === null || held.composing) {
    return false;
  }
  if (held.state.readOnly || held.state.selection.ranges.every((range) => range.empty)) {
    return false;
  }
  held.dispatch(held.state.replaceSelection(event.data), { scrollIntoView: true, userEvent: "input.type" });
  return true;
}
