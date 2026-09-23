import { Chips } from "../partner/Chips";
import { PinnedSubject } from "../partner/Pinned";
import { Transcript } from "../partner/Transcript";
import { Composer } from "../shell/Composer";

/**
 * The focused conversation at /brain: the whole exchange, with the subject
 * pinned beside it and the composer under it.
 *
 * The drawer is for a quick question from anywhere; this is for a long
 * discussion, and a long discussion is exactly where the subject scrolls away.
 * So the thing under discussion stands beside the conversation with its
 * identity, the reading it was taken from, and its summary as the pages show
 * it — and it is the same subject the drawer's chip shows, because expanding
 * the drawer must not change what "this" means.
 *
 * There is no second draft: the store holds one, so expanding into this view
 * carries whatever was half-written in the drawer rather than stranding it.
 */
export function Focused() {
  return (
    <main id="content" className="ms-focused-conversation" tabIndex={-1}>
      <h1 className="ms-visually-hidden">Project Partner</h1>
      <div className="ms-focused-frame">
        <PinnedSubject />
        <div className="ms-focused-column">
          <div className="ms-dock-statement">
            <Transcript />
          </div>
          <Chips />
          <Composer />
        </div>
      </div>
    </main>
  );
}
