/**
 * What a suggestion chip does to what is already written.
 *
 * Astra's seventh finding is the reason this is a decision rather than a
 * shortcut. The send path clears the shared draft when the server accepts, so
 * a chip that always sent would throw away a half-written question the moment
 * somebody pressed one. With an empty composer a chip is the question; with a
 * draft present it is words, and they go in where the caret is.
 */

/** What a chip does: send its question, or put its words in the draft. */
export type Chipped = "send" | "insert";

export function chipped(draft: string): Chipped {
  return draft.trim() === "" ? "send" : "insert";
}

/** A draft with words put in at the cursor, and where the cursor then is. */
export type Inserted = { text: string; caret: number };

/**
 * Insert words into a draft at the selection.
 *
 * A selection is replaced, which is what every text field does; a space is put
 * in where the words would otherwise run into what is before them, and never
 * where one is already there.
 */
export function insertAt(value: string, start: number, end: number, words: string): Inserted {
  const before = value.slice(0, start);
  const after = value.slice(end);
  const gap = before === "" || before.endsWith(" ") || before.endsWith("\n") ? "" : " ";
  const written = `${before}${gap}${words}`;
  return { text: `${written}${after}`, caret: written.length };
}
