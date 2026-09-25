import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

import {
  addSticky,
  changeSticky,
  failureMessage,
  loadStickies,
  removeSticky,
  type About,
  type Change,
  type Notepad,
} from "./api";
import { StickiesPanel } from "./Panel";
import { emptyNotepad } from "./stickies";

/**
 * The notepad, held once for the whole page.
 *
 * There is one list, and the header's button, the panel, the goal page's block
 * and the Partner's capture all read it from here — four surfaces onto one
 * fact, rather than four requests and four opinions. It is the shape the
 * notifications provider already has, and for the same reason: the panel is a
 * sheet rendered beside the shell's children, so nothing below has to know it
 * exists in order to be underneath it.
 *
 * The notepad is read once, when the page loads. Everything after that is an
 * act the human made, and every act answers the whole notepad, so there is
 * nothing here to poll for and no timer anywhere.
 */

type Stickies = {
  notepad: Notepad;
  /** What the first read is doing, or why it did not happen. */
  loaded: boolean;
  problem: string;
  /** True while the panel stands open, which the capture reads. */
  panelIsOpen: boolean;
  /**
   * Open the panel, optionally with the composer already about something —
   * which is what "Add a sticky" on a goal page does.
   */
  openPanel: (about?: About) => void;
  closePanel: () => void;
  /** What the composer opened about, taken once by the composer that reads it. */
  opening: About | null;
  /**
   * The three acts. Each answers the whole notepad, or the refusal's words.
   *
   * Jotting is J1's own word, and it is also the one the cut guard leaves
   * free: the other spelling names document.write everywhere under src/, and
   * exactly one file is allowed to say it.
   */
  jot: (text: string, about: About[]) => Promise<string>;
  change: (id: string, changed: Change) => Promise<string>;
  remove: (id: string) => Promise<string>;
};

/**
 * The notepad's context.
 *
 * It is exported, unlike the notifications provider's, because three surfaces
 * read it and none of them is a page: the header's button, the panel, and the
 * block a goal page and the reader carry. Rendering any one of them over a
 * known notepad — which is how they are checked — means standing this above
 * it, and a provider that only ever wrapped the whole application would leave
 * them reachable only through a server.
 */
export const StickiesContext = createContext<Stickies>({
  notepad: emptyNotepad,
  loaded: false,
  problem: "",
  panelIsOpen: false,
  openPanel: () => {},
  closePanel: () => {},
  opening: null,
  jot: async () => "",
  change: async () => "",
  remove: async () => "",
});

export function useStickies(): Stickies {
  return useContext(StickiesContext);
}

export function StickiesProvider({ children }: { children: ReactNode }) {
  const [notepad, setNotepad] = useState<Notepad>(emptyNotepad);
  const [loaded, setLoaded] = useState(false);
  const [problem, setProblem] = useState("");
  const [open, setOpen] = useState(false);
  const [opening, setOpening] = useState<About | null>(null);

  // The notepad, once.
  useEffect(() => {
    const aborter = new AbortController();
    loadStickies(aborter.signal)
      .then((read) => {
        setNotepad(read);
        setLoaded(true);
      })
      .catch((error: unknown) => {
        if (!aborter.signal.aborted) {
          setProblem(failureMessage(error));
          setLoaded(true);
        }
      });
    return () => {
      aborter.abort();
    };
  }, []);

  const openPanel = useCallback((about?: About) => {
    setOpening(about ?? null);
    setOpen(true);
  }, []);

  const closePanel = useCallback(() => {
    setOpen(false);
    setOpening(null);
  }, []);

  /**
   * One act: run it, take the whole notepad it answered with, and hand back
   * the refusal's words where it refused.
   *
   * The refusal is returned rather than held here because it belongs to the
   * control that made the act: a bound refused in the composer is a sentence
   * under the composer, and the same sentence on a card is under that card.
   */
  const act = useCallback(async (run: () => Promise<Notepad>): Promise<string> => {
    try {
      setNotepad(await run());
      setProblem("");
      return "";
    } catch (error: unknown) {
      return failureMessage(error);
    }
  }, []);

  const jot = useCallback(
    async (text: string, about: About[]) => act(async () => addSticky(text, about)),
    [act],
  );
  const change = useCallback(
    async (id: string, changed: Change) => act(async () => changeSticky(id, changed)),
    [act],
  );
  const remove = useCallback(async (id: string) => act(async () => removeSticky(id)), [act]);

  const value = useMemo(
    () => ({
      notepad,
      loaded,
      problem,
      panelIsOpen: open,
      openPanel,
      closePanel,
      opening,
      jot,
      change,
      remove,
    }),
    [notepad, loaded, problem, open, openPanel, closePanel, opening, jot, change, remove],
  );

  return (
    <StickiesContext.Provider value={value}>
      {children}
      <StickiesPanel />
    </StickiesContext.Provider>
  );
}
