import { ChevronDown, ChevronUp, Maximize2 } from "lucide-react";
import { useNavigate } from "react-router";

import { useAboutLine } from "./about";
import { COMPOSER_REASON } from "./Composer";
import { Chip, IconButton } from "./controls";
import { BrainStatement } from "../panes/Brain";

/**
 * The Project Partner, as a drawer along the bottom of the work area.
 *
 * It was a column on the right, and it took a third of the width from the one
 * thing the work area is for. Along the bottom it takes forty-eight pixels and
 * names the page above it: who it is, where to type, what it is about, and the
 * way to open it. Opening lifts a panel over the foot of the page; the page
 * above keeps its width and keeps scrolling.
 *
 * It arrives closed. The bar is the whole of what an unasked-for collaborator
 * owes the page; the panel is what a human opens, and what this build then
 * remembers for them.
 *
 * The element id and the class names are the kit's own, and stay; what a human
 * reads says Project Partner.
 */
export function Drawer({
  open,
  about,
  onToggle,
}: {
  open: boolean;
  /** The section's own name, where the pane on screen says nothing. */
  about: string;
  onToggle: () => void;
}) {
  const navigate = useNavigate();
  const line = useAboutLine(about);
  return (
    <aside className="ms-drawer" aria-label="Project Partner" data-open={open ? "true" : "false"}>
      <div className="ms-drawer-bar">
        <span className="ms-drawer-who">
          <span className="ms-drawer-title">Project Partner</span>
          <Chip>unavailable until gate 3</Chip>
        </span>
        <label className="ms-visually-hidden" htmlFor="drawer-composer">
          Message to your Project Partner
        </label>
        <input
          id="drawer-composer"
          className="ms-drawer-field"
          type="text"
          placeholder="Message to your Project Partner"
          aria-describedby="drawer-reason"
          disabled
        />
        {/* The chip says it on screen; the field names the whole sentence,
            which is what a human who cannot see the chip needs to hear. */}
        <span className="ms-visually-hidden" id="drawer-reason">
          {COMPOSER_REASON}
        </span>
        <span className="ms-drawer-about">
          about: <b>{line}</b>
        </span>
        <span className="ms-drawer-actions">
          <IconButton
            label="Expand the Project Partner view"
            onClick={() => {
              void navigate("/brain");
            }}
          >
            <Maximize2 size={16} strokeWidth={1.75} aria-hidden="true" />
          </IconButton>
          <button
            type="button"
            className="ms-drawer-toggle"
            aria-expanded={open}
            aria-controls="brain-dock"
            onClick={onToggle}
          >
            {open ? "Close" : "Open"}
            {open ? (
              <ChevronDown size={14} strokeWidth={1.75} aria-hidden="true" />
            ) : (
              <ChevronUp size={14} strokeWidth={1.75} aria-hidden="true" />
            )}
          </button>
        </span>
      </div>
      <div className="ms-drawer-panel" id="brain-dock" hidden={!open}>
        <BrainStatement />
      </div>
    </aside>
  );
}
