import { useEffect, useState } from "react";

/**
 * The widths. Media queries lay the shell out; this hook drives the behaviour
 * that differs rather than the appearance — whether the rail is beside the
 * work or in a sheet behind the menu button, and whether the focused
 * conversation has room for its subject panel.
 *
 * There is no width at which the Project Partner is somewhere else: it is a
 * drawer along the bottom at every one of them.
 */

export const WIDE_QUERY = "(min-width: 960px)";
export const PHONE_QUERY = "(max-width: 599px)";
/** The rail expands only where 240 beside a full work area still fits. */
export const RAIL_QUERY = "(min-width: 1128px)";

function query(text: string): MediaQueryList | null {
  try {
    if (typeof matchMedia !== "function") {
      return null;
    }
    return matchMedia(text);
  } catch {
    return null;
  }
}

export function matches(text: string): boolean {
  return query(text)?.matches ?? false;
}

export function useMediaQuery(text: string): boolean {
  const [matched, setMatched] = useState(() => matches(text));
  useEffect(() => {
    const list = query(text);
    if (list === null) {
      return;
    }
    setMatched(list.matches);
    const listener = (event: MediaQueryListEvent) => {
      setMatched(event.matches);
    };
    list.addEventListener("change", listener);
    return () => {
      list.removeEventListener("change", listener);
    };
  }, [text]);
  return matched;
}
