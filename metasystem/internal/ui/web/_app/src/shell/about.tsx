import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

/**
 * What the page is about, said once, where the drawer can read it.
 *
 * The Project Partner's drawer names the page above it, and only the page
 * knows what that is: which goal is selected, which record is open, which
 * heading is being read. A pane says it here as it renders and takes it back
 * when it leaves, and the drawer falls back to the section's own name — so a
 * pane that says nothing still has a context line, and no pane can leave a
 * stale one behind.
 *
 * It carries no conversation and reaches no network. At gate 3 this is the
 * context the agent receives; today it is the line a human reads.
 */

type About = { about: string | null; describe: (about: string | null) => void };

const AboutContext = createContext<About>({ about: null, describe: () => undefined });

export function AboutProvider({ children }: { children: ReactNode }) {
  const [about, describe] = useState<string | null>(null);
  const value = useMemo(() => ({ about, describe }), [about]);
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

/** A pane says what it is about, for as long as it is on screen. */
export function useAbout(about: string): void {
  const { describe } = useContext(AboutContext);
  useEffect(() => {
    describe(about);
    return () => {
      describe(null);
    };
  }, [about, describe]);
}

/** What the drawer shows: the pane's own words, or the section's name. */
export function useAboutLine(fallback: string): string {
  const { about } = useContext(AboutContext);
  return about ?? fallback;
}
