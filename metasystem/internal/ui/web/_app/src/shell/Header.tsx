import { Menu, PanelBottom } from "lucide-react";

import { IconButton } from "./controls";
import { useWorkspaceState } from "./identity";
import { SignInControl } from "./SignInControl";
import { WorkspaceIdentity } from "./WorkspaceIdentity";
import { Help } from "../help/Help";
import type { HelpId } from "../help/terms";
import { NotificationsBell } from "../notifications/Bell";

/**
 * The header: who this workspace is on the left, where you are in the middle,
 * and, on the right, what the steward has said, who the server is acting as,
 * and the Project Partner's toggle. The drawer along the bottom carries the
 * same toggle; this one is where a keyboard reaches it without crossing the
 * whole page.
 *
 * The bell comes first in that cluster because it is the only control there
 * that changes on its own: the other two say what is already true, and this
 * one is where something new arrives.
 *
 * The section's name carries the help for the section: it is the one place
 * every page names what it is, so it is where "what is this for" belongs.
 * Settings and the focused conversation have none — a workspace's own
 * configuration is not one of the project's structures, and the conversation
 * explains itself on its own drawer.
 *
 * On the focused view the toggle stays in the sequential order and says why it
 * does nothing, rather than disappearing and taking its explanation with it.
 */
export function Header({
  sectionTitle,
  help,
  onMenu,
  dockOpen,
  onDockToggle,
  focused,
}: {
  sectionTitle: string;
  /** The term that explains this section, or null where it has none. */
  help: HelpId | null;
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
        <span className="ms-header-name">
          <span className="ms-header-title">{sectionTitle}</span>
          {help !== null && <Help id={help} />}
        </span>
      </div>
      <div className="ms-header-right">
        <NotificationsBell />
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
