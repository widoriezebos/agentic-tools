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

/** Rail order: the Brain, then the six project sections, then Settings. */
export const sections: readonly Section[] = [
  { id: "brain", path: "/brain", title: "Brain", icon: MessageSquare },
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
 * The section the router itself matches: this build registers one route per
 * section and nothing beneath, so an address beneath a section matches nothing
 * and is the not-found pane. When a slice nests a view, its route and this
 * match change together.
 */
export function activeSection(pathname: string): Section | null {
  const normalized = normalize(pathname);
  if (normalized === "/" || normalized === "") {
    return activeSection(HOME_PATH);
  }
  return sections.find((section) => section.path === normalized) ?? null;
}

export function pathFor(sectionId: string): string | null {
  return sections.find((section) => section.id === sectionId)?.path ?? null;
}

export type Reference = { kind: string; id?: string };

export function routeFor(reference: Reference): string | null {
  if (reference.kind !== "section" || reference.id === undefined) {
    return null;
  }
  return pathFor(reference.id);
}

/** A pasted path with a trailing slash names the same place as one without. */
function normalize(pathname: string): string {
  if (pathname.length > 1 && pathname.endsWith("/")) {
    return normalize(pathname.slice(0, -1));
  }
  return pathname;
}
