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
 * The Project Partner's beats ride the same connection, under their own event
 * type. They are the same kind of thing for the same reason — the Partner
 * speaks while it composes an answer, and the alternative is polling for the
 * rest of a sentence — and a second stream would be a second reconnection
 * policy over the same page. They carry no id of their own, deliberately: the
 * browser sends the last id it saw back as Last-Event-ID and the server
 * resumes the notification journal from it, so a Partner id there would name
 * something that journal cannot find and the reconnect would lose every
 * notification in between. The Partner joins its beats to the conversation's
 * snapshot instead, by turn and sequence, which is what the open listener
 * below is for.
 *
 * Nothing here retries. EventSource reconnects on its own when the connection
 * drops, and sends back the id of the last event it received; the server reads
 * that as Last-Event-ID and resumes from it. A retry loop of our own would be
 * a second reconnection policy fighting the browser's, and a timer in a file
 * that is not allowed one.
 */

import type { Notification } from "./notifications";
import type { PartnerEvent } from "../partner/api";

const STREAM = "/api/notifications/stream";

/** The three events the server names on this stream. */
const EVENT = "notification";
const PARTNER = "partner";
/**
 * The presence fetch owner finished an attempt — a success, a failure, or a
 * look that found nothing new.
 *
 * It rides here for the same reason the Partner's beats do. Presence changes
 * because another machine ticked, at a moment nobody on this side chose, and
 * a Fleet page that is already mounted would otherwise show the reading it
 * loaded with until somebody pressed Refresh. The event carries no payload
 * and no id: the page re-reads /api/fleet, which is the one place the answer
 * is composed, and an id of its own would name something the notification
 * journal cannot find on a reconnect.
 */
const FLEET = "fleet";

/**
 * The listeners that are not the notification store's.
 *
 * They are held here, beside the connection, rather than passed in, because
 * there is one connection and more than one thing on the page that reads from
 * it: the Partner's conversation is held in its own store, above the drawer
 * and the focused page, and it must not depend on which provider happens to
 * open the connection.
 */
const partnerListeners = new Set<(event: PartnerEvent) => void>();
const openListeners = new Set<() => void>();
const fleetListeners = new Set<() => void>();

/** Listen for the Partner's beats. The returned function stops listening. */
export function onPartnerEvent(listener: (event: PartnerEvent) => void): () => void {
  partnerListeners.add(listener);
  return () => {
    partnerListeners.delete(listener);
  };
}

/**
 * Listen for a finished presence attempt. The returned function stops
 * listening, which is what a Fleet page that has left the screen calls.
 */
export function onFleetEvent(listener: () => void): () => void {
  fleetListeners.add(listener);
  return () => {
    fleetListeners.delete(listener);
  };
}

/**
 * Listen for the connection opening, which happens once on load and again on
 * every reconnect. It is what the Partner re-reads its conversation on: a
 * reconnect means beats may have been missed, and the snapshot is the truth.
 */
export function onStreamOpen(listener: () => void): () => void {
  openListeners.add(listener);
  return () => {
    openListeners.delete(listener);
  };
}

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
  const partner = (event: MessageEvent<string>) => {
    let beat: PartnerEvent;
    try {
      beat = JSON.parse(event.data) as PartnerEvent;
    } catch {
      return;
    }
    if (typeof beat.turn !== "string" || beat.turn === "" || typeof beat.seq !== "number") {
      return;
    }
    for (const held of partnerListeners) {
      held(beat);
    }
  };
  // A presence attempt finished. Nothing is parsed, because nothing is sent:
  // the event says "there is something to read again", and the page does the
  // reading.
  const fleet = () => {
    for (const held of fleetListeners) {
      held();
    }
  };
  const opened = () => {
    for (const held of openListeners) {
      held();
    }
  };
  source.addEventListener(EVENT, listener as EventListener);
  source.addEventListener(PARTNER, partner as EventListener);
  source.addEventListener(FLEET, fleet);
  source.addEventListener("open", opened);
  return () => {
    source.removeEventListener(EVENT, listener as EventListener);
    source.removeEventListener(PARTNER, partner as EventListener);
    source.removeEventListener(FLEET, fleet);
    source.removeEventListener("open", opened);
    source.close();
  };
}
