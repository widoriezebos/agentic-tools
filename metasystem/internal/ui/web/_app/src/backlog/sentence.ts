/**
 * A record's first statement, without the rest of it.
 *
 * A row is one line, and a conclusion, a known problem or a ruling is often
 * three sentences. The first one is what tells this row from its neighbours;
 * the whole of it is in the open row.
 *
 * The statement is the first line, up to the first stop that ends a sentence.
 * A stop ends one only where white space follows it or the line ends there:
 * the stop in a file name, a path, a version or a percentage is inside a word.
 * That is the rule the server cuts its own titles by, so a row titled here and
 * a row titled there end at the same word. The full stop itself is left off,
 * and a question keeps its mark, because without it the line asks nothing.
 *
 * The line is plain text, so the backticks that set code apart in the record
 * are not part of it.
 */
export function firstSentence(text: string): string {
  const line = text.trim().split("\n", 1)[0].replaceAll("`", "").trim();
  const stop = line.search(/[.!?](?=\s|$)/);
  if (stop <= 0) {
    return line;
  }
  return line.slice(0, line[stop] === "." ? stop : stop + 1).trim();
}
