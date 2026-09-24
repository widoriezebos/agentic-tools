/**
 * The face and the size the conversation is read in.
 *
 * Wido reads his terminal in Meslo LG M for Powerline and wanted the Project
 * Partner to look the same way, so this file is the whole of what a chosen
 * face is allowed to be. Three faces are this build's own — the one the pages
 * use, the one the documents are read in, and the bundled monospace — and
 * everything else is a name that may or may not exist on the computer the
 * browser is running on.
 *
 * A name is letters, digits, spaces and hyphens, and nothing else ever reaches
 * a stylesheet: `familyOf` refuses a name that is not one of those before it
 * builds a font-family value, so a quote, a semicolon or a colon typed into
 * the "Other" field can never close the declaration it would be written into.
 * The value it builds is set on two elements through `element.style
 * .setProperty`, which is how this build has always carried a computed value
 * into a stylesheet (src/backlog/CardMenu.tsx, src/partner/AskSelection.tsx),
 * and the two custom properties below are read by exactly two rules: the
 * conversation's root and the composer's field.
 *
 * Nothing here touches the interface's own face. A human who chooses a
 * terminal face for their Partner has said nothing about the board.
 */

/** The two custom properties the conversation and the composer's field read. */
export const FACE_PROPERTY = "--ms-partner-face";
export const SIZE_PROPERTY = "--ms-partner-size";

/**
 * The face the pages use. It is written here as well as in fonts.css, which is
 * a second copy of one fact, so typeface.test.ts holds the two to each other.
 */
export const INTERFACE_STACK = '"Inter Variable", system-ui, sans-serif';

/**
 * The face the documents are read in.
 *
 * It is the interface's own face today: the reader sets a measure, a size and a
 * rhythm for a document and inherits the family from the root like everything
 * else. The token is here all the same, because the design offers it as a
 * choice and because the day the documents take a face of their own is the day
 * this one value changes and the chooser follows it.
 */
export const READING_STACK = INTERFACE_STACK;

/** The bundled monospace, and the fallbacks the rest of the build names with it. */
export const MONO_STACK = '"JetBrains Mono Variable", ui-monospace, SFMono-Regular, Menlo, monospace';

/** The three faces this build carries, which are chosen by these three words. */
export const INTERFACE = "interface";
export const READING = "reading";
export const MONO = "mono";

/**
 * How big the conversation may be read, in pixels, and what it is read at
 * where nobody has said. Twelve is the smallest a paragraph stays a paragraph
 * at; twenty-eight is a terminal pulled right up to the eyes.
 */
export const SMALLEST_SIZE = 12;
export const LARGEST_SIZE = 28;
export const DEFAULT_SIZE = 16;

/** The rhythm the chosen size is read at, which follows it rather than a count of pixels. */
export const LEADING = 1.5;

/** What a human chose: a face by name or by token, and a size in pixels. */
export type Typeface = { face: string; size: number };

export const DEFAULT_TYPEFACE: Typeface = { face: INTERFACE, size: DEFAULT_SIZE };

/** What an unrecognised name falls back to, visibly, when it is not installed. */
export type FallbackKind = "mono" | "reading";

/** One entry of the offered list: a name, and what it falls back to. */
export type Offered = { name: string; kind: FallbackKind };

/**
 * The faces the operating system may carry, in the order the chooser offers
 * them: the monospaces a terminal is read in first, then the two proportional
 * ones nearly every machine has.
 *
 * Meslo LG M for Powerline leads because it is the one Wido asked for. The
 * family is what is named here — the machine this was built against installs
 * it as "Meslo LG M Regular for Powerline" and its relatives — because a
 * family name is what a font-family declaration takes.
 */
export const OFFERED: readonly Offered[] = [
  { name: "Meslo LG M for Powerline", kind: "mono" },
  { name: "Menlo", kind: "mono" },
  { name: "SF Mono", kind: "mono" },
  { name: "Monaco", kind: "mono" },
  { name: "Fira Code", kind: "mono" },
  { name: "Source Code Pro", kind: "mono" },
  { name: "JetBrains Mono", kind: "mono" },
  { name: "Consolas", kind: "mono" },
  { name: "Courier New", kind: "mono" },
  { name: "Georgia", kind: "reading" },
  { name: "Helvetica Neue", kind: "reading" },
];

/**
 * A face name: letters, digits, spaces and hyphens, opening with a letter or a
 * digit. Everything else is refused — a quote would close the name, a
 * semicolon would open a second declaration, and a colon would open a second
 * property — and refused here rather than escaped, because a name this build
 * cannot write plainly is not a name it needs to carry.
 */
const NAME = /^[A-Za-z0-9][A-Za-z0-9 -]*$/;

/** The longest name this build will carry. No family is named at this length. */
const LONGEST_NAME = 64;

export function isFaceName(value: string): boolean {
  const name = value.trim();
  return name !== "" && name.length <= LONGEST_NAME && NAME.test(name);
}

/** True where the word names one of the three faces this build carries. */
export function isToken(face: string): boolean {
  return face === INTERFACE || face === READING || face === MONO;
}

/**
 * What a name falls back to where the computer has not got it.
 *
 * A name the chooser offers is answered from the list, however it was typed; a
 * name it does not offer falls back to the reading face, because this build has
 * no way to know whether a family it has never heard of is monospaced.
 */
export function fallbackKind(name: string): FallbackKind {
  const wanted = name.trim().toLowerCase();
  return OFFERED.find((face) => face.name.toLowerCase() === wanted)?.kind ?? "reading";
}

/** The stack a fallback kind names. */
export function stackOf(kind: FallbackKind): string {
  return kind === "mono" ? MONO_STACK : READING_STACK;
}

/**
 * The font-family value a chosen face becomes.
 *
 * A token is its stack. A name is the name, quoted, with the stack of its kind
 * behind it — so a face the computer has not got falls through to Mono or to
 * Reading where a human can see that it did, without anything having to
 * measure whether it is installed. A name this build refuses never gets here
 * at all: it reads as the interface's own face.
 */
export function familyOf(face: string): string {
  if (face === READING) {
    return READING_STACK;
  }
  if (face === MONO) {
    return MONO_STACK;
  }
  if (face === INTERFACE || !isFaceName(face)) {
    return INTERFACE_STACK;
  }
  const name = face.trim();
  return `"${name}", ${stackOf(fallbackKind(name))}`;
}

/** The size as a stylesheet takes it. */
export function sizeOf(size: number): string {
  return `${String(clampSize(size))}px`;
}

/** A size held inside what the stepper offers, whatever it was given. */
export function clampSize(size: number): number {
  if (!Number.isFinite(size)) {
    return DEFAULT_SIZE;
  }
  return Math.min(Math.max(Math.round(size), SMALLEST_SIZE), LARGEST_SIZE);
}

/**
 * A stored face, or the interface's own where what is stored is not a face
 * this build would ever have written.
 */
export function normalizeFace(value: string | null | undefined): string {
  if (typeof value !== "string") {
    return INTERFACE;
  }
  if (isToken(value)) {
    return value;
  }
  return isFaceName(value) ? value.trim() : INTERFACE;
}

/**
 * A stored size, or the default where what is stored is not a whole number of
 * pixels. A number outside the stepper's range is a size a human once had and
 * is brought back inside it rather than thrown away.
 */
export function normalizeSize(value: string | null | undefined): number {
  if (typeof value !== "string" || !/^\d+$/.test(value.trim())) {
    return DEFAULT_SIZE;
  }
  return clampSize(Number.parseInt(value.trim(), 10));
}

/**
 * Sets the chosen face and size on one element, and on nothing else.
 *
 * Two custom properties rather than two declarations: what inherits from here
 * is a value, and the two rules that read it are the conversation's root and
 * the composer's field. Everything else under this element is declared in `em`
 * and in a unitless line height, so one property moves the whole column.
 */
export function apply(element: HTMLElement | null, typeface: Typeface): void {
  if (element === null) {
    return;
  }
  element.style.setProperty(FACE_PROPERTY, familyOf(typeface.face));
  element.style.setProperty(SIZE_PROPERTY, sizeOf(typeface.size));
}

/**
 * Whether the computer has a face, measured rather than asked for.
 *
 * There is no way to ask a browser what is installed, so the oldest trick is
 * the one used here: a string is measured in the wanted family with a bare
 * fallback behind it, and again in the bare fallback alone. A family the
 * browser has got renders the string at its own width; one it has not got
 * renders it at the fallback's, and the two measurements agree. Two fallbacks
 * are tried because a face whose metrics happen to match one of them would
 * otherwise read as absent.
 *
 * The measuring is a parameter so the rule can be tested without a canvas.
 */
export const PROBE = "mmmmmmmmmmlliWWWW0Oo";

const BARE: readonly string[] = ["monospace", "serif"];

export function measuredInstalled(measure: (family: string) => number, name: string): boolean {
  if (!isFaceName(name)) {
    return false;
  }
  const quoted = `"${name.trim()}"`;
  return BARE.some((bare) => measure(`${quoted}, ${bare}`) !== measure(bare));
}

/** The one canvas every measurement is made on, made the first time one is. */
let measurer: ((family: string) => number) | null = null;
let measurerAsked = false;

function canvasMeasure(): ((family: string) => number) | null {
  if (measurerAsked) {
    return measurer;
  }
  measurerAsked = true;
  try {
    const context = globalThis.document.createElement("canvas").getContext("2d");
    if (context !== null) {
      measurer = (family) => {
        context.font = `72px ${family}`;
        return context.measureText(PROBE).width;
      };
    }
  } catch {
    // A browser that will not give a canvas marks nothing, which is better
    // than marking everything absent.
  }
  return measurer;
}

/**
 * True where this computer has the face. A browser that cannot be asked
 * answers yes: an unmarked list is a smaller lie than a list that says every
 * face is missing.
 */
export function isInstalled(name: string): boolean {
  const measure = canvasMeasure();
  if (measure === null) {
    return true;
  }
  return measuredInstalled(measure, name);
}
