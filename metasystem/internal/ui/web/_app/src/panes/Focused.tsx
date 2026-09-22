import { useState } from "react";

import { BrainStatement, SubjectStatement } from "./Brain";
import { Composer } from "../shell/Composer";
import { FocusSwitch, type FocusedView } from "../shell/FocusSwitch";

/**
 * The focused conversation at /brain.
 *
 * Wide: the conversation beside a fixed subject panel, the dock hidden because
 * the conversation is already in front of the human. Narrower: one column, and
 * a switch, which always opens on the conversation.
 */
export function Focused({ wide }: { wide: boolean }) {
  const [view, setView] = useState<FocusedView>("conversation");

  if (wide) {
    return (
      <div className="ms-focused">
        <main id="content" className="ms-focused-conversation" tabIndex={-1}>
          <h1 className="ms-visually-hidden">Project Partner</h1>
          <Conversation />
        </main>
        <aside className="ms-subject-panel" aria-label="Subject">
          <SubjectStatement />
        </aside>
      </div>
    );
  }

  return (
    <main id="content" className="ms-focused-conversation ms-focused--stacked" tabIndex={-1}>
      <h1 className="ms-visually-hidden">Project Partner</h1>
      <FocusSwitch view={view} onChange={setView} />
      {view === "conversation" ? <Conversation /> : <SubjectStatement />}
    </main>
  );
}

function Conversation() {
  return (
    <div className="ms-focused-column">
      <div className="ms-dock-statement">
        <BrainStatement />
      </div>
      <Composer />
    </div>
  );
}
