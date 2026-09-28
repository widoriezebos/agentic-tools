import { createContext, useContext, type ReactNode } from "react";

import { anchorsIn, type DeskItem } from "./room";

/**
 * Anchors put things on the desk (g1-s65 D5).
 *
 * In the review room, a file and its lines named anywhere in the conversation
 * is a chip, and pressing it opens that on the desk. Outside the room there is
 * no desk, so there is no chip: the context is null and the words stay words.
 */
export const DeskAnchors = createContext<((item: DeskItem) => void) | null>(null);

/** Whether the words are being read in a room, which has a desk to put things on. */
export function useDeskAnchors(): ((item: DeskItem) => void) | null {
  return useContext(DeskAnchors);
}

/**
 * One run of words with every anchor in it a chip. `rest` renders the words
 * between the anchors, so the room's chips and the conversation's own links
 * compose rather than one replacing the other.
 */
export function Anchored({
  words,
  put,
  rest,
}: {
  words: string;
  put: (item: DeskItem) => void;
  rest: (words: string) => ReactNode;
}) {
  const found = anchorsIn(words);
  if (found.length === 0) {
    return <>{rest(words)}</>;
  }
  const parts: ReactNode[] = [];
  let at = 0;
  found.forEach((anchor, index) => {
    const start = words.indexOf(anchor.text, at);
    if (start > at) {
      parts.push(<span key={`w${String(index)}`}>{rest(words.slice(at, start))}</span>);
    }
    parts.push(
      <button
        key={`a${String(index)}`}
        type="button"
        className="ms-anchor-chip"
        title={`Put ${anchor.text} on the desk`}
        onClick={() => {
          put(anchor.item);
        }}
      >
        {anchor.text}
      </button>,
    );
    at = start + anchor.text.length;
  });
  if (at < words.length) {
    parts.push(<span key="tail">{rest(words.slice(at))}</span>);
  }
  return <>{parts}</>;
}
