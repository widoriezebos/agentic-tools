/**
 * The one stream this build holds open, and the one deliberate exception to
 * the cut guard's "no event stream" rule.
 *
 * Everything else in this interface is read because a human asked for it: a
 * view mounts, or somebody presses Refresh. A notification is the opposite
 * kind of thing. It is the steward speaking, at a moment nobody on this side
 * chose, and the alternative to being pushed is polling — which is the thing
 * the guard exists to prevent. So there is exactly one stream, opened once for
 * the life of the page, and src/cuts.test.ts names this file and this resource
 * as the only place it may be opened.
 *
 * Nothing here retries. EventSource reconnects on its own when the connection
 * drops, and sends back the id of the last event it received; the server reads
 * that as Last-Event-ID and resumes from it. A retry loop of our own would be
 * a second reconnection policy fighting the browser's, and a timer in a file
 * that is not allowed one.
 */

import type { Notification } from "./notifications";

const STREAM = "/api/notifications/stream";

/** The event the server names each notification with. */
const EVENT = "notification";

/**
 * Open the stream. `arrived` is called once per notification, in the order
 * the steward wrote them. The returned function closes it, which is what a
 * page teardown calls; nothing else closes it, because the page holds one for
 * as long as it exists.
 */
export function openNotificationStream(arrived: (notification: Notification) => void): () => void {
  const source = new EventSource(STREAM);
  const listener = (event: MessageEvent<string>) => {
    let notification: Notification;
    try {
      notification = JSON.parse(event.data) as Notification;
    } catch {
      // A line this build cannot parse is a line from a newer engine or a
      // truncated frame. Dropping it costs one row; throwing here would end
      // the listener and cost every row after it.
      return;
    }
    if (typeof notification.id !== "string" || notification.id === "") {
      return;
    }
    arrived(notification);
  };
  source.addEventListener(EVENT, listener as EventListener);
  return () => {
    source.removeEventListener(EVENT, listener as EventListener);
    source.close();
  };
}
