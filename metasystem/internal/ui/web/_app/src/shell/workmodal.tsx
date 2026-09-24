import { createContext, useContext, useEffect, useMemo, useRef, type ReactNode } from "react";

/**
 * How much of the window a sheet is modal for.
 *
 * A sheet blocks the page it sits on so that a stray click cannot drag a card
 * while a form is being filled. It has no reason to block the Project Partner.
 * So "work" — every sheet's default — covers the work area and nothing else:
 * the scrim and the pointer-blocking stop at the work area's own box, and the
 * drawer beneath it stays live, opening, resizing and taking questions while
 * the sheet is open. Focus is not trapped: Tab leaves the sheet for the drawer
 * and Escape acts on whichever of the two has focus.
 *
 * "window" is sign-in's, and sign-in's alone: a one-time code is typed once
 * and nothing else should be reachable while it is.
 *
 * A work-area sheet falls back to the window where there is no work area. The
 * focused conversation at /brain is the one such page — it has no drawer, so
 * there is nothing for a sheet there to leave free — and a sheet opened there
 * is modal for the page exactly as it was before this slice.
 */
export type Modality = "work" | "window";

/** Every sheet's mode unless it says otherwise. */
export const DEFAULT_MODALITY: Modality = "work";

/** Sign-in's, and nothing else's. */
export const SIGN_IN_MODALITY: Modality = "window";

/**
 * The two elements the work area lends a sheet: the layer a sheet renders
 * into, which is an absolutely positioned cover over the work area's own box,
 * and the content under it, which is made inert while a sheet is open so that
 * neither a pointer nor the Tab key reaches it.
 *
 * Both are null where there is no work area, which is the focused page and
 * every render outside the shell.
 */
type WorkArea = { layer: HTMLElement | null; under: HTMLElement | null };

const WorkAreaContext = createContext<WorkArea>({ layer: null, under: null });

/** The shell says where its work area is. Nothing else may. */
export function WorkAreaProvider({
  layer,
  under,
  children,
}: {
  layer: HTMLElement | null;
  under: HTMLElement | null;
  children: ReactNode;
}) {
  const value = useMemo(() => ({ layer, under }), [layer, under]);
  return <WorkAreaContext.Provider value={value}>{children}</WorkAreaContext.Provider>;
}

/**
 * Where a sheet of this mode renders, or null for the whole window.
 *
 * While it returns a layer, the work area under that layer is inert: the scrim
 * takes the clicks and the content behind it holds no tab stop, so Tab runs
 * out of the sheet into the drawer and back into the sheet rather than into a
 * board nobody can see.
 *
 * The inert is applied from here and not from a render, and this effect is the
 * first the sheet declares, so its cleanup is the first to run: the work area
 * is live again before the sheet hands the caret back to whatever opened it.
 * An element that is still inert cannot take focus, and a sheet that returned
 * the caret to one would be returning it to nowhere.
 */
export function useWorkModal(modality: Modality, open = true): HTMLElement | null {
  const { layer, under } = useContext(WorkAreaContext);
  const host = modality === "work" ? layer : null;
  // Where the sheet is mounted whether or not it is open — the font chooser
  // is, because the control and its sheet are one component — only an open one
  // covers anything. The layer itself is returned either way, so that what a
  // sheet tells Radix about its own modality does not change as it opens.
  const covered = host !== null && open;
  useEffect(() => {
    if (!covered || under === null) {
      return;
    }
    cover(under);
    return () => {
      uncover(under);
    };
  }, [covered, under]);
  return host;
}

/**
 * Whatever held the caret when this sheet mounted, read while it still holds
 * it.
 *
 * It is read during the render and not from an effect, because covering the
 * work area takes the caret off whatever had it: an element inside an inert
 * subtree is not focusable, and the browser moves focus to the document. By
 * the time any effect runs there is nothing left to remember.
 */
export function useOpener(): { current: HTMLElement | null } {
  const opener = useRef<HTMLElement | null>(null);
  const read = useRef(false);
  if (!read.current) {
    read.current = true;
    // Rendered on a server there is no document at all, and no caret to keep;
    // there is no HTMLElement to ask about either, so what is wanted of it is
    // asked of the value itself.
    opener.current = takesFocus(globalThis.document?.activeElement);
  }
  return opener;
}

/**
 * The node, where it is one a caret can be given back to. It asks the value
 * whether it can take the caret rather than naming a class the server render
 * has never heard of.
 */
function takesFocus(node: unknown): HTMLElement | null {
  const candidate = node as HTMLElement | null | undefined;
  return typeof candidate?.focus === "function" ? candidate : null;
}

/**
 * How many sheets are covering one work area, so that two of them cannot
 * uncover it once. Nothing in this build opens two at a time today; the count
 * is here so that the day one does, the second to close is the one that gives
 * the work area back.
 */
const covering = new WeakMap<HTMLElement, number>();

function cover(element: HTMLElement): void {
  covering.set(element, (covering.get(element) ?? 0) + 1);
  element.inert = true;
}

function uncover(element: HTMLElement): void {
  const held = (covering.get(element) ?? 1) - 1;
  covering.set(element, held);
  if (held <= 0) {
    element.inert = false;
  }
}
