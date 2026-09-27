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

/**
 * What the header shows: the read to make, and what the tooltip says.
 *
 * `reread` is under a contract now, and it is the one this slice added: it KEEPS
 * THE PAGE MOUNTED. It sets what it read in place and never passes through the
 * loading state, which is for the first read and for Retry after a failure.
 * Anything else may be called while a human is typing — the store asks for it
 * after a confirmed act, and the act may have been made in the Partner's drawer
 * over a page whose inline form is half filled in — and a re-read that unmounted
 * the page would throw those keystrokes away (Astra S58-03, S58-10).
 *
 * That holds when the read FAILS as well, which was the half the contract left
 * unsaid: a refusal keeps the last reading and its mounted children and shows the
 * refusal's words beside them, and only a first read's failure — where there is
 * nothing on screen to keep — draws the page's error view (Astra C-05). Both
 * halves are asserted over the source of every pane that offers a read, in
 * `refresh.test.ts`, because neither can be mounted here.
 *
 * `inStrip` says the page already carries a refresh of its own, in its own
 * toolbar or crumbs. Such a page offers its read all the same, so that a
 * confirmed act elsewhere moves it; what it does not want is a second icon
 * beside the one it already has, so the header shows none.
 */
export type Offer = { reread: () => void; hint: string; inStrip?: boolean };

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
 * was read. And the read must keep the page mounted, which is the Offer's own
 * contract above: a pane whose only read blanks it passes an in-place one here
 * and keeps the blanking one for its own Retry.
 *
 * `inStrip` is for a page that already shows a refresh in its own strip: it
 * offers its read so that a confirmed act moves it, and the header shows no
 * second icon.
 */
export function useOffersRefresh(reread: () => void, hint: string, inStrip = false): void {
  const { offer } = useContext(RefreshContext);
  useEffect(() => {
    offer({ reread, hint, inStrip });
    return () => {
      offer(null);
    };
  }, [reread, hint, inStrip, offer]);
}

/** What the header shows, or null where this section offers no read. */
export function useSectionRefresh(): Offer | null {
  return useContext(RefreshContext).offered;
}

/**
 * Whether the header shows an icon for this offer.
 *
 * A page with a refresh in its own strip offers its read all the same — that is
 * how a confirmed act made elsewhere moves it — and does not want a second icon
 * beside the one the strip already has. So the offer stands and the header stays
 * quiet.
 */
export function showsInHeader(offered: Offer | null): boolean {
  return offered !== null && offered.inStrip !== true;
}
