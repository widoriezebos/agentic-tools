import { CircleHelp } from "lucide-react";

import { EmptyState, Pane } from "./Pane";
import type { Section } from "../routes";

/**
 * A section this build does not project yet. Every one of them says so, names
 * the slice or gate that brings it, and offers nothing that would mislead.
 */
export function SectionPane({ section }: { section: Section }) {
  return (
    <Pane title={section.title}>
      <EmptyState id={section.id} icon={section.icon} />
    </Pane>
  );
}

/** An address that matches nothing. Not a section, and the one pane with an action. */
export function NotFoundPane() {
  return (
    <Pane title="Not found">
      <EmptyState id="not-found" icon={CircleHelp} />
    </Pane>
  );
}
