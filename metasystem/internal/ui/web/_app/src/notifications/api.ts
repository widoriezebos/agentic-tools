/**
 * The notification history, and the fifth place this build talks to the
 * server.
 *
 * It is read once when the page loads, and again only when a human presses
 * "Load older" — never on a timer and never on a window event. What arrives
 * after the load arrives on the stream beside this, which the server pushes;
 * there is nothing here to poll for.
 *
 * src/cuts.test.ts holds the call sites to a written list, so a request added
 * under any other name fails the guard rather than the review.
 */

import type { Notification } from "./notifications";

const HISTORY = "/api/notifications";

/** The page size both readings ask for, and the one the server defaults to. */
export const PAGE = 200;

type HistoryPayload = {
  notifications: Notification[];
  /**
   * What the server thinks is unread. It is always null: what a viewer has
   * seen is kept in that viewer's own browser, and the server has no opinion.
   * It is read here so that the shape is named where it is parsed.
   */
  unreadFrom: string | null;
};

/**
 * One page of the history, newest first. `before` names the oldest row the
 * page already holds, and asks for what lies behind it.
 */
export async function loadNotifications(before: string | null, signal?: AbortSignal): Promise<Notification[]> {
  const query = before === null ? `?limit=${String(PAGE)}` : `?limit=${String(PAGE)}&before=${encodeURIComponent(before)}`;
  const response = await fetch(HISTORY + query, { signal, headers: { Accept: "application/json" } });
  if (!response.ok) {
    throw new Error(`${HISTORY} answered ${String(response.status)}`);
  }
  const payload = (await response.json()) as HistoryPayload;
  return payload.notifications;
}
