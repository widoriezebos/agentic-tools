import { PanelLeft } from "lucide-react";
import { NavLink, useLocation } from "react-router";

import { Hint, IconButton } from "./controls";
import { ThemeControl } from "./ThemeControl";
import { activeSection, projectSections, sections, type Section } from "../routes";
import type { ThemePreference } from "../theme";

/**
 * The rail: the six project sections, Settings at the foot. The Project
 * Partner has no row here: its drawer is on every page, and the drawer's
 * expand button is the way to its whole conversation (Wido, 2026-09-25),
 * and the theme control beneath it.
 *
 * Collapsed, a row keeps its label in the DOM and hides it from the screen, so
 * the accessible name never depends on the width, and a tooltip repeats it for
 * the eye.
 */

const settings = sections[sections.length - 1];

export function Rail({
  expanded,
  toggleable,
  onToggle,
  theme,
  onTheme,
  onNavigate,
  inSheet = false,
}: {
  expanded: boolean;
  /** The toggle exists only where an expanded rail would fit. */
  toggleable: boolean;
  onToggle: () => void;
  theme: ThemePreference;
  onTheme: (preference: ThemePreference) => void;
  /** The phone's sheet closes itself when a row is followed. */
  onNavigate?: () => void;
  inSheet?: boolean;
}) {
  const classes = ["ms-rail"];
  if (!expanded) {
    classes.push("ms-rail--collapsed");
  }
  if (inSheet) {
    classes.push("ms-rail--sheet");
  }
  return (
    <div className={classes.join(" ")}>
      {toggleable && (
        <div className="ms-rail-toggle-row">
          <IconButton
            label={expanded ? "Collapse the rail" : "Expand the rail"}
            aria-expanded={expanded}
            aria-controls="sections"
            onClick={onToggle}
          >
            <PanelLeft size={16} strokeWidth={1.75} aria-hidden="true" />
          </IconButton>
        </div>
      )}
      <nav id="sections" aria-label="Sections" className="ms-rail-sections">
        <div className="ms-rail-nav">
          {projectSections.map((section) => (
            <RailItem key={section.id} section={section} expanded={expanded} onNavigate={onNavigate} />
          ))}
        </div>
        <div className="ms-rail-spacer" />
        <div className="ms-rail-nav">
          <RailItem section={settings} expanded={expanded} onNavigate={onNavigate} />
        </div>
      </nav>
      <div className="ms-rail-footer">
        <ThemeControl preference={theme} onChange={onTheme} />
      </div>
    </div>
  );
}

function RailItem({
  section,
  expanded,
  onNavigate,
}: {
  section: Section;
  expanded: boolean;
  onNavigate?: () => void;
}) {
  const Icon = section.icon;
  const location = useLocation();
  // The rail marks the section the router is in, which is the section the
  // header names: a view nested beneath a section — a document under Project,
  // a goal under Backlog — lights its own row rather than none. Everywhere
  // else the match is exact, so an address beneath a section that matches no
  // route lights nothing, which is what the header says of it too.
  const here = activeSection(location.pathname)?.id === section.id;
  const row = (
    <NavLink className="ms-rail-item" to={section.path} end={!here} onClick={onNavigate}>
      <Icon size={18} strokeWidth={1.75} aria-hidden="true" />
      <span className={expanded ? undefined : "ms-visually-hidden"}>{section.title}</span>
    </NavLink>
  );
  if (expanded) {
    return row;
  }
  return <Hint label={section.title}>{row}</Hint>;
}
