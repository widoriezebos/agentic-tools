import { Maximize2, X } from "lucide-react";
import { useNavigate } from "react-router";

import { Composer } from "./Composer";
import { Chip, IconButton } from "./controls";
import { BrainStatement } from "../panes/Brain";

/**
 * The Project Partner dock: the conversation's home beside the work, with the
 * way into the focused view and the way to close it.
 *
 * The element id and the class names are the kit's own, and stay; what a human
 * reads says Project Partner.
 */
export function Dock({ onClose }: { onClose: () => void }) {
  const navigate = useNavigate();
  return (
    <aside id="brain-dock" className="ms-dock" aria-label="Project Partner dock">
      <div className="ms-dock-header">
        <span className="ms-dock-title">Project Partner</span>
        <Chip>unavailable until gate 3</Chip>
        <div className="ms-dock-actions">
          <IconButton
            label="Expand the Project Partner view"
            onClick={() => {
              void navigate("/brain");
            }}
          >
            <Maximize2 size={16} strokeWidth={1.75} aria-hidden="true" />
          </IconButton>
          <IconButton label="Close the Project Partner dock" onClick={onClose}>
            <X size={16} strokeWidth={1.75} aria-hidden="true" />
          </IconButton>
        </div>
      </div>
      <div className="ms-dock-body">
        <div className="ms-dock-statement">
          <BrainStatement />
        </div>
        <Composer />
      </div>
    </aside>
  );
}

/** The dock's body, as the compact and phone sheet shows it. */
export function DockSheetBody() {
  return (
    <>
      <div className="ms-dock-statement">
        <BrainStatement />
      </div>
      <Composer />
    </>
  );
}
