import { useEffect, useState } from "react";

/**
 * The three widths. Media queries lay the shell out; this hook drives the
 * behaviour that differs rather than the appearance — which panel exists at
 * all, and whether the dock is a panel or a sheet.
 */

export const WIDE_QUERY = "(min-width: 960px)";
export const PHONE_QUERY = "(max-width: 599px)";
/** The rail expands only where 240 + 8 + 480 + 400 still fits. */
export const RAIL_QUERY = "(min-width: 1128px)";

export const RAIL_WIDTH_EXPANDED = 240;
export const RAIL_WIDTH_COLLAPSED = 56;

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

/** The viewport's width, or the wide threshold where there is no window. */
export function viewportWidth(): number {
  try {
    return globalThis.innerWidth || 960;
  } catch {
    return 960;
  }
}
