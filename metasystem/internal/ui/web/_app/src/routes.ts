import { BookOpen, Gavel, LayoutDashboard, MessageSquare, Package, Server, Settings, SquareKanban } from "lucide-react";
import type { ComponentType, SVGProps } from "react";

/**
 * Where the application can be. Every destination has one path, the path is
 * what the address bar shows, and a reserved prefix is never one of them: /-,
 * /api and /assets belong to the server, which answers them itself.
 *
 * routeFor is the master's resolver at its smallest. It answers one kind,
 * `section`; later slices register `goal`, `seat` and `question`, and null
 * still means "this build has no view for that", which a caller shows as text
 * rather than as a link that would refuse.
 */

export type SectionIcon = ComponentType<SVGProps<SVGSVGElement> & { size?: number | string; strokeWidth?: number | string }>;

export type Section = {
  id: string;
  path: string;
  title: string;
  icon: SectionIcon;
  /**
   * What this section's page puts on the screen, in one sentence.
   *
   * It is not what the section is for — the help register says that — and it
   * is not whether this build has it: `projected` says that. Three statements
   * about one section that a reader must be able to tell apart, so each has
   * its own field and its own owner.
   */
  shows: string;
  /**
   * Whether this build renders this section's own view.
   *
   * It is the one answer to that question. The shell routes the sections that
   * project to their panes and the rest to the pane that says which gate
   * brings them; the empty register covers exactly the ones that do not; and
   * the interface manifest tells the Project Partner the same thing, so a
   * human asking where to look is never sent to a placeholder.
   */
  projected: boolean;
};

/**
 * Rail order: the Project Partner, then the six project sections, then
 * Settings.
 *
 * The id and the path stay `brain`: they are the engine's names for the seat's
 * process, and the kit keeps them until a deliberate rename. The title is what
 * a human reads, and the interface stopped calling the collaborator a brain.
 *
 * Decisions is a gavel. It wore the circled question mark until the help
 * control took that mark for its own — one mark cannot mean both "ask me what
 * this is" and "a section of this workspace" — and the gavel is what the
 * section actually is: the place a human rules on what the machinery asks.
 */
export const sections: readonly Section[] = [
  {
    id: "brain",
    path: "/brain",
    title: "Project Partner",
    icon: MessageSquare,
    shows: "The conversation with the Partner in full, with the subject under discussion pinned beside it.",
    projected: true,
  },
  {
    id: "overview",
    path: "/overview",
    title: "Overview",
    icon: LayoutDashboard,
    shows: "What needs you, what changed since your last visit, what the fleet is working on now, and one line of health.",
    projected: true,
  },
  {
    id: "project",
    path: "/project",
    title: "Project",
    icon: BookOpen,
    shows: "The project's records a kind at a time — intent, doctrine, decisions, designs, open questions and the checkout's other documents — and any one of them opened for reading.",
    projected: true,
  },
  {
    id: "backlog",
    path: "/backlog",
    title: "Backlog",
    icon: SquareKanban,
    shows: "The goals as a board, lane by lane, with the closed items behind a disclosure and one goal opened on its own page.",
    projected: true,
  },
  {
    id: "fleet",
    path: "/fleet",
    title: "Fleet",
    icon: Server,
    shows: "A row per machine: whether it has been heard from and when, what it is running, and which goals it holds, with this seat's own publishing and health above them.",
    projected: true,
  },
  {
    id: "decisions",
    path: "/decisions",
    title: "Decisions",
    icon: Gavel,
    shows: "Everything waiting on a human, each row saying what is asked, who asks and what happens if you do nothing, and what you have already decided: your rulings whole, the decisions recorded, the questions answered and the goals approved.",
    projected: true,
  },
  {
    id: "application",
    path: "/application",
    title: "Application",
    icon: Package,
    shows: "The known problems, open ones first; what this workspace has concluded, week by week, in the ledger's own words; and the documents that say what it is.",
    projected: true,
  },
  {
    id: "settings",
    path: "/settings",
    title: "Settings",
    icon: Settings,
    shows: "This workspace's own identity, the appearance control, and the settings pages themselves.",
    projected: false,
  },
];

/** The sections this build renders a view of, and the sections it does not. */
export const projectedSections: readonly Section[] = sections.filter((section) => section.projected);
export const unprojectedSections: readonly Section[] = sections.filter((section) => !section.projected);

/** The six project sections: the rail's middle group, between two rules. */
export const projectSections: readonly Section[] = sections.filter(
  (section) => section.id !== "brain" && section.id !== "settings",
);

export const HOME_PATH = "/overview";

/** The prefixes the server owns, exactly or with anything beneath them. */
export const reservedPrefixes: readonly string[] = ["/-", "/api", "/assets"];

export function isReserved(pathname: string): boolean {
  const normalized = normalize(pathname);
  return reservedPrefixes.some((prefix) => normalized === prefix || normalized.startsWith(`${prefix}/`));
}

/**
 * The section that owns a path: its own, or anything beneath it, so that a
 * nested view a later slice adds still lights its rail row. "/" is the home
 * section, because "/" redirects there.
 */
export function sectionFor(pathname: string): Section | null {
  const normalized = normalize(pathname);
  if (normalized === "/" || normalized === "") {
    return sectionFor(HOME_PATH);
  }
  return (
    sections.find((section) => normalized === section.path || normalized.startsWith(`${section.path}/`)) ?? null
  );
}

/**
 * The nested routes this build registers, and the section each belongs to.
 * The list is here rather than inferred from the section's prefix because the
 * router registers exactly these: an address beneath a section that is not one
 * of them matches no route and is the not-found pane, and the header has to
 * agree with the router about that.
 *
 * A route naming a segment with a colon takes exactly one segment there and
 * ends: /project/designs is the Project page opened on its designs, and
 * /project/designs/anything is not a route at all. A route written without
 * one owns everything beneath it, which is what the document reader and the
 * goal page are.
 */
export const nestedRoutes: readonly { path: string; sectionId: string }[] = [
  { path: "/project/doc", sectionId: "project" },
  { path: "/project/:tab", sectionId: "project" },
  { path: "/backlog/goal", sectionId: "backlog" },
];

/** True when one of the nested routes above matches this address. */
function matchesNested(route: string, pathname: string): boolean {
  if (!route.includes(":")) {
    return pathname === route || pathname.startsWith(`${route}/`);
  }
  const wanted = route.split("/");
  const given = pathname.split("/");
  return (
    wanted.length === given.length &&
    wanted.every((segment, at) => (segment.startsWith(":") ? given[at] !== "" : segment === given[at]))
  );
}

/**
 * The section the router itself matches: its own path, or a nested route this
 * build registers beneath it. When a slice nests a view, its route and this
 * match change together.
 */
export function activeSection(pathname: string): Section | null {
  const normalized = normalize(pathname);
  if (normalized === "/" || normalized === "") {
    return activeSection(HOME_PATH);
  }
  const exact = sections.find((section) => section.path === normalized);
  if (exact !== undefined) {
    return exact;
  }
  const nested = nestedRoutes.find((route) => matchesNested(route.path, normalized));
  return nested === undefined ? null : (sections.find((section) => section.id === nested.sectionId) ?? null);
}

export function pathFor(sectionId: string): string | null {
  return sections.find((section) => section.id === sectionId)?.path ?? null;
}

export type Reference = { kind: string; id?: string };

export function routeFor(reference: Reference): string | null {
  if (reference.id === undefined || reference.id === "") {
    return null;
  }
  if (reference.kind === "section") {
    return pathFor(reference.id);
  }
  if (reference.kind === "document") {
    return documentPath(reference.id);
  }
  return null;
}

/** Everything beneath this prefix is a document, named by the rest of it. */
export const DOCUMENT_PREFIX = "/project/doc/";

/**
 * Everything beneath this prefix is one goal of the ledger.
 *
 * A goal belongs to the Backlog, which is where the ledger is read and worked:
 * Project is the repository's own records, and a goal page under it made the
 * ledger look like a second subdivision of the project's documents. The page
 * is the same page; the address says which section owns it.
 */
export const GOAL_PREFIX = "/backlog/goal/";

/**
 * Where one goal is shown, and on which of its tabs. The id is one segment,
 * encoded as one; the tab is the segment after it.
 */
export function goalPath(id: string, tab?: string): string {
  return withTab(GOAL_PREFIX + encodeURIComponent(id), tab);
}

/**
 * The query the Backlog lands on a goal from.
 *
 * A goal is not a place on the Backlog, so it is not a segment of the
 * address: the Backlog is the place, and this says which goal this one
 * arrival is about. The page shows that goal and then drops the query, so a
 * reload is the Backlog rather than the landing again — which is also why the
 * query never has to be in the section table above.
 */
export const SHOWN_GOAL = "goal";

/** Where the Backlog is, and the goal it should land on where one is named. */
export function backlogPath(goal?: string): string {
  if (goal === undefined || goal === "") {
    return "/backlog";
  }
  return `/backlog?${SHOWN_GOAL}=${encodeURIComponent(goal)}`;
}

/**
 * Where the Project page is, and which of its tabs is open.
 *
 * The tab is in the address because a section of the project is a place: a
 * human sends a colleague the doctrine, or reloads on the designs they were
 * reading, and both need the address to say which. An address naming no tab
 * has asked for nothing, and the page opens on the tab that browser was last
 * left on.
 */
export function projectPath(tab?: string): string {
  return withTab("/project", tab);
}

function withTab(at: string, tab: string | undefined): string {
  return tab === undefined || tab === "" ? at : `${at}/${encodeURIComponent(tab)}`;
}

/**
 * Where a document is read. The id is a checkout-relative path with "/"
 * separators, and each segment is encoded on its own, so a separator stays a
 * separator and everything else survives the address bar.
 */
export function documentPath(id: string): string {
  return (
    DOCUMENT_PREFIX +
    id
      .split("/")
      .map((segment) => encodeURIComponent(segment))
      .join("/")
  );
}

/**
 * The id an address carries back, decoded a segment at a time.
 *
 * It reads the address itself rather than a router parameter, because the
 * router decodes a parameter for its own purposes and a name holding a per-cent
 * sign would then be decoded twice; here one encoder and one decoder face each
 * other, and the round trip is a test.
 */
export function documentIdFromPath(pathname: string): string {
  if (!pathname.startsWith(DOCUMENT_PREFIX)) {
    return "";
  }
  return pathname
    .slice(DOCUMENT_PREFIX.length)
    .split("/")
    .map((segment) => decodeSegment(segment))
    .join("/");
}

function decodeSegment(segment: string): string {
  try {
    return decodeURIComponent(segment);
  } catch {
    return segment;
  }
}

/** A pasted path with a trailing slash names the same place as one without. */
function normalize(pathname: string): string {
  if (pathname.length > 1 && pathname.endsWith("/")) {
    return normalize(pathname.slice(0, -1));
  }
  return pathname;
}
