import { X } from "lucide-react";
import { useEffect, useRef, useState, type KeyboardEvent } from "react";

/**
 * A field of free words that shows which words already exist.
 *
 * A label is not a record and never has to be one: a human may invent one,
 * and inventing one is how the first of any label is made. What a human may
 * not do without noticing is invent one by accident — "boad" for "board" is a
 * label nobody meant, nothing refuses it, and it is found months later by
 * somebody wondering why a filter is empty. So the field keeps the words free
 * and makes the new ones visible: what is typed is suggested from what the
 * board already carries, each accepted word becomes a chip, and a chip the
 * board has never seen wears a small "new".
 *
 * The value is the one string the act already sends — the words separated by
 * spaces — so nothing about the request changes because the field that fills
 * it changed. The separators a human may type are the ones the field always
 * took: Enter, a comma, or a space ends a word.
 */

/** The words one line means, however they were separated. */
export function tokensOf(value: string): string[] {
  return value
    .split(/[\s,]+/)
    .map((token) => token.trim())
    .filter((token) => token !== "");
}

/** The keys that end a word. A space is one, which is why none is ever typed. */
export function accepts(key: string): boolean {
  return key === "Enter" || key === "," || key === " ";
}

/**
 * The existing words a half-typed one could become: what the board carries,
 * by prefix, without what has already been accepted. Nothing is suggested
 * for an empty field — a list of every label the board has ever used is not
 * an answer to a human who has not asked anything yet.
 */
export function suggesting(known: readonly string[], typed: string, taken: readonly string[]): string[] {
  const prefix = typed.trim().toLowerCase();
  if (prefix === "") {
    return [];
  }
  return known.filter((label) => label.toLowerCase().startsWith(prefix) && !taken.includes(label));
}

/** True for a word the board has never carried, which is what "new" marks. */
export function isNew(token: string, known: readonly string[]): boolean {
  return !known.includes(token);
}

/** One suggestion's own id, which is what aria-activedescendant names. */
function optionId(field: string, at: number): string {
  return `${field}-suggestion-${String(at)}`;
}

export function TokenField({
  id,
  value,
  known,
  placeholder = "",
  onChange,
}: {
  /** The field's id, which the label points at and the list is named from. */
  id: string;
  /** The accepted words, as the one string the act sends: separated by spaces. */
  value: string;
  /** Every word this board already carries, sorted, offered as they are typed. */
  known: readonly string[];
  placeholder?: string;
  onChange: (value: string) => void;
}) {
  const [typed, setTyped] = useState("");
  const [active, setActive] = useState(0);
  const field = useRef<HTMLInputElement | null>(null);
  const list = useRef<HTMLUListElement | null>(null);

  const tokens = tokensOf(value);
  const suggestions = suggesting(known, typed, tokens);
  const at = Math.min(active, Math.max(suggestions.length - 1, 0));
  const listed = suggestions.length > 0;
  const listId = `${id}-list`;

  // The list brings itself into view, so that one opened near the foot of a
  // sheet whose body scrolls is not a list below the fold; the suggestion the
  // arrows are on brings itself into view inside the list.
  useEffect(() => {
    if (!listed) {
      return;
    }
    list.current?.scrollIntoView({ block: "nearest" });
    const option = list.current?.children.item(at);
    if (option instanceof HTMLElement) {
      option.scrollIntoView({ block: "nearest" });
    }
  }, [listed, at, suggestions.length]);

  const add = (word: string) => {
    const wanted = word.trim();
    setTyped("");
    setActive(0);
    if (wanted === "" || tokens.includes(wanted)) {
      return;
    }
    onChange([...tokens, wanted].join(" "));
  };

  const remove = (word: string) => {
    onChange(tokens.filter((token) => token !== word).join(" "));
    field.current?.focus();
  };

  const keys = (event: KeyboardEvent<HTMLInputElement>) => {
    if (accepts(event.key)) {
      event.preventDefault();
      add(listed ? suggestions[at] : typed);
      return;
    }
    if (event.key === "Backspace" && typed === "") {
      const last = tokens.at(-1);
      if (last === undefined) {
        return;
      }
      event.preventDefault();
      onChange(tokens.slice(0, -1).join(" "));
      return;
    }
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      if (!listed) {
        return;
      }
      event.preventDefault();
      const step = event.key === "ArrowDown" ? 1 : -1;
      setActive((at + step + suggestions.length) % suggestions.length);
    }
  };

  return (
    <div className="ms-tokens">
      {tokens.length > 0 && (
        <div className="ms-tokens-chosen">
          {tokens.map((token) => (
            <span className="ms-token" key={token}>
              <span>{token}</span>
              {isNew(token, known) && <span className="ms-token-new">new</span>}
              <button
                type="button"
                className="ms-token-clear"
                aria-label={`Remove ${token}`}
                onClick={() => {
                  remove(token);
                }}
              >
                <X size={12} strokeWidth={2} aria-hidden="true" />
              </button>
            </span>
          ))}
        </div>
      )}
      <input
        id={id}
        type="text"
        role="combobox"
        value={typed}
        placeholder={placeholder}
        autoComplete="off"
        aria-expanded={listed}
        aria-controls={listId}
        aria-autocomplete="list"
        aria-activedescendant={listed ? optionId(id, at) : undefined}
        aria-describedby={`${id}-hint`}
        ref={field}
        onChange={(event) => {
          setTyped(event.target.value);
          setActive(0);
        }}
        onKeyDown={keys}
        // A word left half-typed is still a word the human meant, so leaving
        // the field accepts it rather than dropping it.
        onBlur={() => {
          add(typed);
        }}
      />
      {listed && (
        <ul
          className="ms-tokens-list"
          id={listId}
          role="listbox"
          aria-label="Labels this board already carries"
          ref={list}
          onMouseDown={(event) => {
            event.preventDefault();
          }}
        >
          {suggestions.map((label, index) => (
            <li
              key={label}
              id={optionId(id, index)}
              role="option"
              className={index === at ? "ms-tokens-option ms-tokens-option--active" : "ms-tokens-option"}
              aria-selected={index === at}
              onClick={() => {
                add(label);
                field.current?.focus();
              }}
            >
              {label}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
