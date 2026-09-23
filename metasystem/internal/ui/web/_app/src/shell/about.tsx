import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

/**
 * What the page is about, said once, where the drawer and the Partner can read
 * it.
 *
 * The line is what a human reads. The subject beside it is what the server
 * needs in order to compose the Partner's context: which kind of thing the
 * page is about, which one, and the revision the page is showing. A label
 * cannot carry that — "sign-in goal · Plan" names nothing a reader can look
 * up, and a Partner given only a label would read whatever the working tree
 * happens to hold and explain a different state than the one on screen.
 *
 * A pane says both here as it renders and takes them back when it leaves, so a
 * pane that says nothing still has a context line and no pane can leave a
 * stale one behind. Nothing here reaches the network; the subject travels with
 * a question when a human asks one, and never on its own.
 */

/** What the page is about, as something the server can look up. */
export type Subject = {
  /** goal or document, where the page is about one; empty otherwise. */
  kind?: string;
  /** The goal's ledger id, or the document's checkout-relative id. */
  subject?: string;
  title?: string;
  /** The revision the page is displaying, where the subject carries one. */
  revision?: string;
  /** The tab of the page that is open. */
  tab?: string;
  /** Which reading of the section is open, where it has more than one. */
  view?: string;
  /**
   * The reading this page rendered from: the accepted tip it drew its rows
   * out of, and the moment the server observed it.
   *
   * They travel because the server composes its own reading a moment later,
   * and the two can be minutes apart. A sheet that showed the server's reading
   * beside a card drawn from this one would certify the wrong page, so the
   * capture carries the page's and the block says both where they differ.
   */
  tip?: string;
  observedAt?: string;
  /** How far back the Done lane reached, as the page spells it. */
  window?: string;
  /**
   * The address this page returns to: its path with the view, filters, window
   * and tab it is showing. It is composed here, by the page that owns its own
   * address grammar, and kept with every question asked from it.
   */
  returnTo?: string;
  /** What the page is narrowed to, as the page spells it. */
  filters?: string[];
  /**
   * The board as this page is showing it: one entry per lane on screen, in the
   * board's order, with the goals its filters and its Done window left in it.
   *
   * It travels because the goals are not in any file. The board is built from
   * the accepted ledger commit, which the engine loads out of git, so a
   * Partner told only where the human is would look for them in the checkout,
   * not find them, and say so. Only this page knows what is on screen; only
   * the server knows what each of those goals is. So the page names them and
   * the server reads them.
   */
  lanes?: Lane[];
  /**
   * What the open project tab is listing, by the key the page lists it under:
   * a record's checkout-relative path, or a question's id.
   */
  records?: string[];
};

/** One column of the board as the page is showing it. */
export type Lane = {
  id: string;
  title: string;
  /** How many the lane holds after the filters; goals names the first of them. */
  total: number;
  goals: string[];
};

type About = {
  about: string | null;
  subject: Subject;
  describe: (about: string | null, subject: Subject) => void;
};

const empty: Subject = {};

const AboutContext = createContext<About>({ about: null, subject: empty, describe: () => undefined });

export function AboutProvider({ children }: { children: ReactNode }) {
  const [said, setSaid] = useState<{ about: string | null; subject: Subject }>({ about: null, subject: empty });
  // describe never changes identity, and that is the whole of why this works.
  // A pane says what it is about from an effect, and that effect depends on
  // describe; a describe rebuilt whenever the value changed made the effect
  // run again, and its own cleanup then said the page was about nothing. The
  // page ended up describing itself and immediately taking it back, so the
  // context a question carried was empty — which is exactly what a live
  // Partner reported: it was given a lane's count and no goals at all.
  const describe = useCallback((about: string | null, subject: Subject) => {
    setSaid({ about, subject });
  }, []);
  const value = useMemo(
    () => ({ about: said.about, subject: said.subject, describe }),
    [said, describe],
  );
  return <AboutContext.Provider value={value}>{children}</AboutContext.Provider>;
}

/**
 * What the page is about, said once: the page, and the part of it being read.
 *
 * A document's outline opens with the document's own title, so the part being
 * read is the whole of it until the reader has scrolled past the first
 * heading. Naming both would say the same thing twice, so a part that is the
 * page — and a page no part has been read of yet — is the page alone.
 */
export function aboutLine(page: string, part: string): string {
  const read = part.trim();
  if (read === "" || read === page.trim()) {
    return page;
  }
  return `${page} · ${read}`;
}

/**
 * A pane says what it is about, for as long as it is on screen. The subject is
 * optional because not every page is about one thing: the Overview is about
 * the workspace, and the board is about a filtered set rather than a record.
 */
export function useAbout(about: string, subject: Subject = empty): void {
  const { describe } = useContext(AboutContext);
  // The subject is an object literal at nearly every call site, so it is
  // compared by what it says rather than by identity; otherwise every render
  // of the pane would describe the page again and the drawer would re-render
  // with it.
  const said = JSON.stringify(subject);
  useEffect(() => {
    describe(about, JSON.parse(said) as Subject);
    return () => {
      describe(null, empty);
    };
  }, [about, said, describe]);
}

/** What the drawer shows: the pane's own words, or the section's name. */
export function useAboutLine(fallback: string): string {
  const { about } = useContext(AboutContext);
  return about ?? fallback;
}

/** What the page is about, as the Partner's question carries it. */
export function useSubject(): Subject {
  const { subject } = useContext(AboutContext);
  return subject;
}
