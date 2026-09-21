/**
 * The theme: a preference the human sets, and the light or dark it resolves to.
 *
 * The stylesheet has one light block and one dark block and no media query, so
 * the resolution happens here and in public/theme.js, which runs before the
 * first paint. Both read the same key and set the same attribute, which is why
 * this module exports their names.
 */

export const THEME_KEY = "ms.ui.theme";
export const THEME_ATTRIBUTE = "data-theme";
export const DARK_QUERY = "(prefers-color-scheme: dark)";

export type ThemePreference = "system" | "light" | "dark";
export type Theme = "light" | "dark";

export const THEME_PREFERENCES: readonly ThemePreference[] = ["system", "light", "dark"];

/** Anything that is not one of the three preferences reads as "system". */
export function normalizeTheme(value: string | null | undefined): ThemePreference {
  return value === "system" || value === "light" || value === "dark" ? value : "system";
}

export function effectiveTheme(preference: ThemePreference, systemIsDark: boolean): Theme {
  if (preference === "system") {
    return systemIsDark ? "dark" : "light";
  }
  return preference;
}

/** The media query list, or null where there is no browser to ask. */
function darkQuery(): MediaQueryList | null {
  try {
    if (typeof matchMedia !== "function") {
      return null;
    }
    return matchMedia(DARK_QUERY);
  } catch {
    return null;
  }
}

export function systemIsDark(): boolean {
  return darkQuery()?.matches ?? false;
}

/** Follows the system preference while the page is open; returns its undo. */
export function watchSystemTheme(onChange: (dark: boolean) => void): () => void {
  const media = darkQuery();
  if (media === null) {
    return () => {};
  }
  const listener = (event: MediaQueryListEvent) => {
    onChange(event.matches);
  };
  media.addEventListener("change", listener);
  return () => {
    media.removeEventListener("change", listener);
  };
}

export function applyTheme(root: Element, theme: Theme): void {
  root.setAttribute(THEME_ATTRIBUTE, theme);
}
