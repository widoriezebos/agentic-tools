import { Minus, Plus } from "lucide-react";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
  type RefObject,
} from "react";

import {
  apply,
  DEFAULT_TYPEFACE,
  FACE_PROPERTY,
  familyOf,
  fallbackKind,
  INTERFACE,
  isFaceName,
  isInstalled,
  isToken,
  LARGEST_LEADING,
  LARGEST_SIZE,
  LEADING_STEP,
  leadingText,
  MONO,
  OFFERED,
  SMALLEST_LEADING,
  SMALLEST_SIZE,
  clampLeading,
  clampSize,
  type Typeface,
} from "./typeface";
import "./partner.css";
import { Button, IconButton } from "../shell/controls";
import { Sheet } from "../shell/Sheet";
import { readPartnerTypeface, writePartnerTypeface } from "../storage";

/**
 * The font of the conversation: the control, and the choice it carries.
 *
 * Wido reads his terminal in Meslo LG M for Powerline and asked for the
 * Project Partner in the same face, at a size he sets, remembered; then, of a
 * long answer, "the distance between the lines, can I also change that?". So
 * this is an "Aa" in the drawer's header and on the focused page, a sheet with
 * the faces in their own faces, and two steppers — the size, and the rhythm it
 * is read at; what it chooses is written to this browser's storage the moment
 * it changes and read back when the page loads.
 *
 * The choice is held above the drawer and the focused page, because they are
 * two views of one conversation and a face chosen in one is the face of the
 * other. It reaches the page as three custom properties set on two elements —
 * the conversation's root and the composer's card — and the two rules that
 * read them are the only places in this build a chosen face is applied.
 *
 * A face this computer has not got is marked rather than hidden: the list says
 * "not installed" beside it, and choosing it anyway shows what it falls back
 * to, which is the point of marking it rather than refusing it.
 */

/** The accessible name of the control, said once and used in three places. */
export const FONT_LABEL = "Font of the conversation";

/** What every face is previewed with: a question a human would actually ask. */
export const PREVIEW = "Which goals are in Ready for Work?";

/**
 * What the one preview under the steppers shows. It is the question and an
 * answer to it, because line spacing is the distance between two lines and a
 * sample that fits on one line shows none of it.
 */
export const SAMPLE =
  `${PREVIEW} Two carry no approval, so no seat may claim them yet; ` +
  "the board reads the accepted ledger rather than the working tree.";

type Chooser = {
  typeface: Typeface;
  choose: (typeface: Typeface) => void;
};

const TypefaceContext = createContext<Chooser>({ typeface: DEFAULT_TYPEFACE, choose: () => {} });

export function useTypeface(): Chooser {
  return useContext(TypefaceContext);
}

export function TypefaceProvider({ children }: { children: ReactNode }) {
  const [typeface, setTypeface] = useState<Typeface>(() => readPartnerTypeface());
  const choose = useCallback((next: Typeface) => {
    setTypeface(next);
    writePartnerTypeface(next);
  }, []);
  const chooser = useMemo(() => ({ typeface, choose }), [typeface, choose]);
  return <TypefaceContext.Provider value={chooser}>{children}</TypefaceContext.Provider>;
}

/**
 * Puts the chosen face, size and rhythm on one element. The conversation's
 * root and the composer's card are the two that ask for it, and nothing else
 * does.
 */
export function useTypefaceOn<Element extends HTMLElement>(element: RefObject<Element | null>): void {
  const { typeface } = useTypeface();
  useEffect(() => {
    apply(element.current, typeface);
  }, [element, typeface]);
}

export function FontControl() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <IconButton
        className="ms-font-control"
        label={FONT_LABEL}
        aria-expanded={open}
        onClick={() => {
          setOpen(true);
        }}
      >
        <span className="ms-font-mark" aria-hidden="true">
          Aa
        </span>
      </IconButton>
      <Sheet
        open={open}
        onOpenChange={setOpen}
        side="right"
        label={FONT_LABEL}
        title={FONT_LABEL}
        closeLabel="Close the font chooser"
        bodyClassName="ms-sheet-body--font"
        sheetName="Font"
      >
        <FontForm />
      </Sheet>
    </>
  );
}

/**
 * The faces this build carries, and what each of them is.
 *
 * There were three, and one of them was the other drawn twice: the documents
 * are read in the interface's own face today, so "Reading" offered a second
 * entry that changed nothing a human could see. Two identical entries in a
 * list of faces teach a human that the list is not to be trusted, so the
 * entry is gone; the day the documents take a face of their own is the day it
 * comes back, with something behind it.
 */
const TOKENS: readonly { face: string; title: string; note: string }[] = [
  { face: INTERFACE, title: "Interface", note: "the face the pages use" },
  { face: MONO, title: "Mono", note: "the monospace this build carries with it" },
];

function FontForm() {
  const { typeface, choose } = useTypeface();
  // The typed name, seeded with whatever is chosen when it is neither of the
  // two. It is the field's own state and not the choice: a half-typed name is
  // not a face, and nothing half-typed reaches the conversation.
  const [typed, setTyped] = useState(() => (isToken(typeface.face) ? "" : typeface.face));
  // Whether each offered face is on this computer, measured once rather than
  // on every keystroke in the field below the list.
  const present = useMemo(() => new Map(OFFERED.map((face) => [face.name, isInstalled(face.name)])), []);

  const named = typed.trim();
  const typedIsName = isFaceName(named);
  const typedInstalled = typedIsName && isInstalled(named);

  return (
    <div className="ms-font-form">
      <div className="ms-font-group" role="group" aria-label="Face">
        <p className="ms-font-legend">Face</p>
        {TOKENS.map((token) => (
          <FaceEntry
            key={token.face}
            face={token.face}
            title={token.title}
            note={token.note}
            chosen={typeface.face === token.face}
            onChoose={() => {
              choose({ ...typeface, face: token.face });
            }}
          />
        ))}
        {OFFERED.map((offer) => (
          <FaceEntry
            key={offer.name}
            face={offer.name}
            title={offer.name}
            note={present.get(offer.name) === false ? absence(offer.name) : ""}
            absent={present.get(offer.name) === false}
            chosen={typeface.face === offer.name}
            onChoose={() => {
              choose({ ...typeface, face: offer.name });
            }}
          />
        ))}
      </div>

      <div className="ms-font-group">
        <label className="ms-font-legend" htmlFor="ms-font-other">
          Other
        </label>
        <p className="ms-font-help">
          Any face installed on this computer, by its family name. Letters, digits, spaces and hyphens.
        </p>
        <div className="ms-font-other">
          <input
            id="ms-font-other"
            type="text"
            className="ms-font-field"
            placeholder="Meslo LG M for Powerline"
            value={typed}
            onChange={(event) => {
              setTyped(event.target.value);
            }}
            onKeyDown={(event) => {
              if (event.key === "Enter" && typedIsName) {
                event.preventDefault();
                choose({ ...typeface, face: named });
              }
            }}
          />
          <Button
            disabled={!typedIsName || typeface.face === named}
            onClick={() => {
              choose({ ...typeface, face: named });
            }}
          >
            Use
          </Button>
        </div>
        {named !== "" && !typedIsName && (
          <p className="ms-font-refusal">A face is named in letters, digits, spaces and hyphens, and nothing else.</p>
        )}
        {typedIsName && (
          <FacePreview face={named} absent={!typedInstalled} note={typedInstalled ? "" : absence(named)} />
        )}
      </div>

      <div className="ms-font-group" role="group" aria-label="Size">
        <p className="ms-font-legend">Size</p>
        <div className="ms-font-stepper">
          <IconButton
            label="Smaller"
            disabled={typeface.size <= SMALLEST_SIZE}
            onClick={() => {
              choose({ ...typeface, size: clampSize(typeface.size - 1) });
            }}
          >
            <Minus size={16} strokeWidth={1.75} aria-hidden="true" />
          </IconButton>
          <output className="ms-font-size">{typeface.size} px</output>
          <IconButton
            label="Bigger"
            disabled={typeface.size >= LARGEST_SIZE}
            onClick={() => {
              choose({ ...typeface, size: clampSize(typeface.size + 1) });
            }}
          >
            <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
          </IconButton>
        </div>
      </div>

      {/* The rhythm, which is the third thing a human reading a long answer
          asks for and the one nothing here offered (Wido, 2026-09-24: "the
          distance between the lines, can I also change that?"). It is a
          multiple of the size rather than a count of pixels, so it survives
          every size the stepper above it reaches. */}
      <div className="ms-font-group" role="group" aria-label="Line spacing">
        <p className="ms-font-legend">Line spacing</p>
        <div className="ms-font-stepper">
          <IconButton
            label="Tighter"
            disabled={typeface.leading <= SMALLEST_LEADING}
            onClick={() => {
              choose({ ...typeface, leading: clampLeading(typeface.leading - LEADING_STEP) });
            }}
          >
            <Minus size={16} strokeWidth={1.75} aria-hidden="true" />
          </IconButton>
          <output className="ms-font-size">{leadingText(typeface.leading)}</output>
          <IconButton
            label="Looser"
            disabled={typeface.leading >= LARGEST_LEADING}
            onClick={() => {
              choose({ ...typeface, leading: clampLeading(typeface.leading + LEADING_STEP) });
            }}
          >
            <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
          </IconButton>
        </div>
        {/* The one preview that shows all three: the face that is chosen, at
            the size that is chosen, in the rhythm that is chosen. It is more
            than one sentence because the distance between two lines cannot be
            shown on one of them. */}
        <p
          className="ms-font-sample"
          ref={(element) => {
            apply(element, typeface);
          }}
        >
          {SAMPLE}
        </p>
      </div>

      <div className="ms-font-foot">
        <Button
          onClick={() => {
            setTyped("");
            choose(DEFAULT_TYPEFACE);
          }}
        >
          Reset
        </Button>
      </div>
    </div>
  );
}

/**
 * What a face that is not on this computer is read in instead, named by the
 * entry in the list above that a human can go and choose.
 */
function absence(name: string): string {
  return `not installed — read in ${fallbackKind(name) === "mono" ? "Mono" : "Interface"}`;
}

function FaceEntry({
  face,
  title,
  note,
  absent = false,
  chosen,
  onChoose,
}: {
  face: string;
  title: string;
  note: string;
  absent?: boolean;
  chosen: boolean;
  onChoose: () => void;
}) {
  return (
    <button type="button" className="ms-font-entry" aria-pressed={chosen} onClick={onChoose}>
      <span className="ms-font-entry-head">
        <span className="ms-font-entry-name">{title}</span>
        {note !== "" && (
          <span className={absent ? "ms-font-entry-absent" : "ms-font-entry-note"}>{note}</span>
        )}
      </span>
      {/* The sample is what the face looks like, which is nothing a screen
          reader can convey: the name and the mark beside it carry the whole
          of what this entry says. */}
      <span
        className="ms-font-entry-preview"
        aria-hidden="true"
        ref={(element) => {
          element?.style.setProperty(FACE_PROPERTY, familyOf(face));
        }}
      >
        {PREVIEW}
      </span>
    </button>
  );
}

/** The typed name, drawn in the face it asks for, with the same mark. */
function FacePreview({ face, absent, note }: { face: string; absent: boolean; note: string }) {
  return (
    <div className="ms-font-entry ms-font-entry--still">
      <span className="ms-font-entry-head">
        <span className="ms-font-entry-name">{face}</span>
        {absent && <span className="ms-font-entry-absent">{note}</span>}
      </span>
      {/* The sample is what the face looks like, which is nothing a screen
          reader can convey: the name and the mark beside it carry the whole
          of what this entry says. */}
      <span
        className="ms-font-entry-preview"
        aria-hidden="true"
        ref={(element) => {
          element?.style.setProperty(FACE_PROPERTY, familyOf(face));
        }}
      >
        {PREVIEW}
      </span>
    </div>
  );
}
