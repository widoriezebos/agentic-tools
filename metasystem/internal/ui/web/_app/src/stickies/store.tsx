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
import { emptyNotepad } from "./stickies";

/**
 * The notepad, held once for the whole page.
 *
 * There is one list, and the header's button, the panel, the goal page's block
 * and the Partner's capture all read it from here — four surfaces onto one
 * fact, rather than four requests and four opinions.
 *
 * The list stands above the shell, because a goal page and the document reader
 * each show the stickies about them. The PANEL does not: it is rendered inside
 * the shell, beside the rest of what stands over the work area, because it
 * needs three things this provider is above and therefore cannot give it — the
 * page's own subject, so the composer's chip is the goal being looked at; the
 * conversation, so an open sheet is named in the capture; and the work area,
 * so the sheet is modal for the work area alone and the Partner drawer stays
 * live underneath. A panel rendered here had none of the three, and "open the
 * panel and ask" — J4's own gesture — could not be performed at all.
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
   * True while the "Done (n)" disclosure stands open. It is held here rather
   * than in the element because the capture depends on it: what the Partner is
   * given is what the human can SEE, and struck-off stickies folded away
   * behind the disclosure are not that.
   */
  doneIsOpen: boolean;
  showDone: (open: boolean) => void;
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
  doneIsOpen: false,
  showDone: () => {},
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
  const [doneOpen, setDoneOpen] = useState(false);
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

  // The panel opens with the struck-off ones folded away, every time, and the
  // state here is put back to that on both edges. The disclosure itself goes
  // with the sheet when the sheet closes, so a remembered true would be this
  // store claiming the human can see something no longer on their screen —
  // and the capture believes this store.
  const openPanel = useCallback((about?: About) => {
    setOpening(about ?? null);
    setDoneOpen(false);
    setOpen(true);
  }, []);

  const closePanel = useCallback(() => {
    setOpen(false);
    setDoneOpen(false);
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
      doneIsOpen: doneOpen,
      showDone: setDoneOpen,
      openPanel,
      closePanel,
      opening,
      jot,
      change,
      remove,
    }),
    [notepad, loaded, problem, open, doneOpen, openPanel, closePanel, opening, jot, change, remove],
  );

  return <StickiesContext.Provider value={value}>{children}</StickiesContext.Provider>;
}
