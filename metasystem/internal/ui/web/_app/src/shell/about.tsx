import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

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
  /** What the page is narrowed to, as the page spells it. */
  filters?: string[];
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
  const value = useMemo(
    () => ({
      about: said.about,
      subject: said.subject,
      describe: (about: string | null, subject: Subject) => {
        setSaid({ about, subject });
      },
    }),
    [said],
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
