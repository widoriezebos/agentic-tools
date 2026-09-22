import { BookOpen, CircleHelp, LayoutDashboard, MessageSquare, Package, Server, Settings, SquareKanban } from "lucide-react";
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
};

/**
 * Rail order: the Project Partner, then the six project sections, then
 * Settings.
 *
 * The id and the path stay `brain`: they are the engine's names for the seat's
 * process, and the kit keeps them until a deliberate rename. The title is what
 * a human reads, and the interface stopped calling the collaborator a brain.
 */
export const sections: readonly Section[] = [
  { id: "brain", path: "/brain", title: "Project Partner", icon: MessageSquare },
  { id: "overview", path: "/overview", title: "Overview", icon: LayoutDashboard },
  { id: "project", path: "/project", title: "Project", icon: BookOpen },
  { id: "backlog", path: "/backlog", title: "Backlog", icon: SquareKanban },
  { id: "fleet", path: "/fleet", title: "Fleet", icon: Server },
  { id: "decisions", path: "/decisions", title: "Decisions", icon: CircleHelp },
  { id: "application", path: "/application", title: "Application", icon: Package },
  { id: "settings", path: "/settings", title: "Settings", icon: Settings },
];

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
 */
export const nestedRoutes: readonly { path: string; sectionId: string }[] = [
  { path: "/project/doc", sectionId: "project" },
  { path: "/project/goal", sectionId: "project" },
];

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
  const nested = nestedRoutes.find(
    (route) => normalized === route.path || normalized.startsWith(`${route.path}/`),
  );
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

/** Everything beneath this prefix is one goal of the ledger. */
export const GOAL_PREFIX = "/project/goal/";

/** Where one goal is shown. The id is one segment, encoded as one. */
export function goalPath(id: string): string {
  return GOAL_PREFIX + encodeURIComponent(id);
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
