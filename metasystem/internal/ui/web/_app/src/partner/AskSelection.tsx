import { useEffect, useState } from "react";

import { usePartner } from "./store";
import type { Chosen } from "./subject";

/**
 * Selected text is a subject.
 *
 * Select a passage anywhere the interface renders prose and a small Ask
 * appears beside it; choosing it attaches the passage, with the document and
 * revision it was read at, and puts the caret in the composer. It is attached
 * for one question: a quote is said once, and its chip says so. Nothing is
 * sent: the quote is a removable chip above the composer and the question is
 * still the human's to write, which is Astra's seventh finding.
 *
 * It appears over reading surfaces only. A card is a drag target and a
 * selection on it competes with the gesture the board is built on; the editor
 * holds text nobody has saved, and only saved text is ever shared. So a
 * surface opts in by marking itself, and nothing else offers this at all.
 */

/** The attributes a reading surface marks itself with. */
export const ASK_SURFACE = "data-ask-surface";
export const ASK_SOURCE = "data-ask-source";
export const ASK_REVISION = "data-ask-revision";

/** How much of one selection travels. A passage, not a document. */
const MAX_PASSAGE = 4000;

type Standing = { text: string; source: string; revision: string; anchor: string; x: number; y: number };

export function AskSelection() {
  const { askPassage } = usePartner();
  const [standing, setStanding] = useState<Standing | null>(null);

  useEffect(() => {
    const changed = () => {
      setStanding(selected());
    };
    const dismissed = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setStanding(null);
      }
    };
    const document_ = globalThis.document;
    document_.addEventListener("selectionchange", changed);
    document_.addEventListener("keydown", dismissed);
    return () => {
      document_.removeEventListener("selectionchange", changed);
      document_.removeEventListener("keydown", dismissed);
    };
  }, []);

  if (standing === null) {
    return null;
  }
  return (
    <button
      type="button"
      className="ms-ask-selection"
      ref={(element) => {
        element?.style.setProperty("--ms-ask-x", `${String(standing.x)}px`);
        element?.style.setProperty("--ms-ask-y", `${String(standing.y)}px`);
      }}
      onMouseDown={(event) => {
        // The press must not clear the selection before this reads it.
        event.preventDefault();
      }}
      onClick={() => {
        askPassage(passageOf(standing));
        setStanding(null);
      }}
    >
      Ask
    </button>
  );
}

/** The selection as a subject: the passage, whole, and where it came from. */
function passageOf(standing: Standing): Chosen {
  return {
    kind: "passage",
    id: standing.source,
    title: firstWords(standing.text),
    source: standing.revision === "" ? standing.source : `${standing.source}, revision ${standing.revision}`,
    summary: "",
    quote: standing.text,
    revision: standing.revision,
    anchor: standing.anchor,
  };
}

/** What the chip calls a passage: its opening words. */
function firstWords(text: string): string {
  const one = text.replace(/\s+/gu, " ").trim();
  return one.length <= 60 ? one : `${one.slice(0, 60)}…`;
}

/**
 * The selection, where it is one this build offers Ask on: inside a marked
 * reading surface, not empty, and not collapsed to a caret.
 */
function selected(): Standing | null {
  const selection = globalThis.getSelection();
  if (selection === null || selection.isCollapsed || selection.rangeCount === 0) {
    return null;
  }
  const text = selection.toString().trim();
  if (text === "") {
    return null;
  }
  const surface = surfaceOf(selection.anchorNode) ?? surfaceOf(selection.focusNode);
  if (surface === null) {
    return null;
  }
  const box = selection.getRangeAt(0).getBoundingClientRect();
  return {
    text: text.length > MAX_PASSAGE ? `${text.slice(0, MAX_PASSAGE)}…` : text,
    source: surface.getAttribute(ASK_SOURCE) ?? "",
    revision: surface.getAttribute(ASK_REVISION) ?? "",
    anchor: headingOver(selection.anchorNode),
    x: box.left + box.width / 2,
    y: box.top,
  };
}

/** The marked reading surface this node is in, or null. */
function surfaceOf(node: Node | null): Element | null {
  const element = node === null ? null : node.nodeType === Node.ELEMENT_NODE ? (node as Element) : node.parentElement;
  return element?.closest(`[${ASK_SURFACE}]`) ?? null;
}

/**
 * The heading the passage sits under, which is what a message chip returns to
 * when the passage itself has moved. The reader gives every heading an id, so
 * this is the nearest one before the selection.
 */
function headingOver(node: Node | null): string {
  let at = node === null ? null : node.nodeType === Node.ELEMENT_NODE ? (node as Element) : node.parentElement;
  while (at !== null) {
    let before: Element | null = at;
    while (before !== null) {
      if (/^H[1-6]$/.test(before.tagName) && before.id !== "") {
        return before.id;
      }
      before = before.previousElementSibling;
    }
    at = at.parentElement;
  }
  return "";
}
