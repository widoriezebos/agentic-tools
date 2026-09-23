import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

/**
 * The section's refresh, in the header, beside the section's name.
 *
 * Most pages carry their own refresh in their own strip, because they have a
 * strip: the board's toolbar, the briefing's tabs, the reader's crumbs. A page
 * that is one screen of blocks has none, and giving it a bar of its own for
 * one icon would be a bar that exists to hold an icon. So the icon goes where
 * the section is already named, and the page says it has one by offering the
 * read.
 *
 * A pane offers it for as long as it is on screen and takes it back when it
 * leaves, exactly as it says what it is about: the header falls back to no
 * icon at all, so a section that does not read anything has nothing to press
 * and no pane can leave a stale refresh behind pointing at a page that is
 * gone.
 *
 * Nothing here is on a timer. The offer is a function the pane already has,
 * and pressing the icon is the only thing that calls it.
 */

/** What the header shows: the read to make, and what the tooltip says. */
export type Offer = { reread: () => void; hint: string };

type Refresh = { offered: Offer | null; offer: (offer: Offer | null) => void };

const RefreshContext = createContext<Refresh>({ offered: null, offer: () => undefined });

export function RefreshProvider({ children }: { children: ReactNode }) {
  const [offered, offer] = useState<Offer | null>(null);
  const value = useMemo(() => ({ offered, offer }), [offered]);
  return <RefreshContext.Provider value={value}>{children}</RefreshContext.Provider>;
}

/**
 * A pane offers its own read, for as long as it is on screen.
 *
 * The read must be stable across renders — a useCallback, or a function the
 * pane does not rebuild — because it is what the effect depends on. The hint
 * may change with every response, which is the point: it says when the page
 * was read.
 */
export function useOffersRefresh(reread: () => void, hint: string): void {
  const { offer } = useContext(RefreshContext);
  useEffect(() => {
    offer({ reread, hint });
    return () => {
      offer(null);
    };
  }, [reread, hint, offer]);
}

/** What the header shows, or null where this section offers no read. */
export function useSectionRefresh(): Offer | null {
  return useContext(RefreshContext).offered;
}
