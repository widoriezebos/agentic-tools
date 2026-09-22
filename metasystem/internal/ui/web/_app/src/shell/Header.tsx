import { Menu, PanelBottom } from "lucide-react";

import { IconButton } from "./controls";
import { useWorkspaceState } from "./identity";
import { SignInControl } from "./SignInControl";
import { WorkspaceIdentity } from "./WorkspaceIdentity";

/**
 * The header: who this workspace is on the left, where you are in the middle,
 * and who the server is acting as beside the Project Partner's toggle on the
 * right. The drawer along the bottom carries the same toggle; this one is
 * where a keyboard reaches it without crossing the whole page.
 *
 * On the focused view the toggle stays in the sequential order and says why it
 * does nothing, rather than disappearing and taking its explanation with it.
 */
export function Header({
  sectionTitle,
  onMenu,
  dockOpen,
  onDockToggle,
  focused,
}: {
  sectionTitle: string;
  /** Present on the phone, where the rail lives in a sheet. */
  onMenu?: () => void;
  dockOpen: boolean;
  onDockToggle: () => void;
  focused: boolean;
}) {
  const { workspace } = useWorkspaceState();
  const known = workspace.state === "known" ? workspace.workspace : null;
  const classes = ["ms-header"];
  if (known?.conflict === true) {
    classes.push("ms-header--conflict");
  } else if (known?.mode === "self-hosted") {
    classes.push("ms-header--marked");
  }

  return (
    <header className={classes.join(" ")}>
      <div className="ms-header-left">
        {onMenu !== undefined && (
          <IconButton label="Sections" onClick={onMenu}>
            <Menu size={16} strokeWidth={1.75} aria-hidden="true" />
          </IconButton>
        )}
        <WorkspaceIdentity />
        <span className="ms-header-rule" aria-hidden="true" />
        <span className="ms-header-title">{sectionTitle}</span>
      </div>
      <div className="ms-header-right">
        <SignInControl />
        <IconButton
          label="Project Partner drawer"
          hint={focused ? "The conversation is already in focus" : "Project Partner drawer"}
          aria-expanded={dockOpen}
          aria-controls="brain-dock"
          aria-disabled={focused ? true : undefined}
          onClick={() => {
            if (focused) {
              return;
            }
            onDockToggle();
          }}
        >
          <PanelBottom size={16} strokeWidth={1.75} aria-hidden="true" />
        </IconButton>
      </div>
    </header>
  );
}
