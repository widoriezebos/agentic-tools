import { Transcript } from "../partner/Transcript";
import { Composer } from "../shell/Composer";

/**
 * The focused conversation at /brain: the whole exchange, in one column, with
 * the composer under it. The drawer is not shown here — the conversation is
 * already in front of the human — and there is no second draft: the store
 * holds one, so expanding into this view carries whatever was half-written in
 * the drawer rather than stranding it there.
 *
 * There is no subject panel yet. Pinning a subject, and sending a selection to
 * the conversation, are the next slice's; a panel that could only say it was
 * empty would be a third of the width spent on a sentence.
 */
export function Focused() {
  return (
    <main id="content" className="ms-focused-conversation" tabIndex={-1}>
      <h1 className="ms-visually-hidden">Project Partner</h1>
      <div className="ms-focused-column">
        <div className="ms-dock-statement">
          <Transcript />
        </div>
        <Composer />
      </div>
    </main>
  );
}
