import { useNotifications } from "./store";
import { clockTime, groupByDay, type Notification } from "./notifications";
import { Button, Chip } from "../shell/controls";
import { Sheet } from "../shell/Sheet";
import { Trouble } from "../shell/Trouble";

/**
 * The history, as a sheet from the right.
 *
 * A macOS toast is a message that exists for four seconds. This is the same
 * message kept: newest first, under the day it was sent on, with the time, the
 * whole message rather than a truncation of it, and the part of the steward it
 * came from. Nothing is summarised, because a human opening this is looking
 * for the one line they half-read while it was on screen.
 *
 * A row the steward could not deliver says so, and says why. That line is the
 * delivery gate made visible: the steward's contract is that a message counts
 * as delivered only when the notifier accepted it, and until this panel
 * existed, a refusal was something only a log knew about.
 */

/** What the panel says when the steward has never had to say anything. */
const NOTHING_YET = "Nothing yet. The steward's messages will appear here as they are sent.";

export function NotificationsPanel() {
  const { store, panelIsOpen, closePanel, focused, loadOlder } = useNotifications();
  const days = groupByDay(store.history, new Date());

  return (
    <Sheet
      open={panelIsOpen}
      onOpenChange={(next) => {
        if (!next) {
          closePanel();
        }
      }}
      id="notifications-panel"
      side="right"
      label="Notifications"
      title="Notifications"
      closeLabel="Close the notifications"
      bodyClassName="ms-sheet-body--notifications"
    >
      {store.problem !== "" && <Trouble text={store.problem} onAsked={closePanel} />}
      {store.loaded && store.problem === "" && store.history.length === 0 && (
        <p className="ms-notifications-empty">{NOTHING_YET}</p>
      )}
      {days.map((day) => (
        <section key={day.key} className="ms-notifications-day">
          <h3 className="ms-notifications-heading">{day.heading}</h3>
          <ul className="ms-notifications-rows">
            {day.rows.map((notification) => (
              <Row
                key={notification.id}
                notification={notification}
                focused={notification.id === focused}
                onAsked={closePanel}
              />
            ))}
          </ul>
        </section>
      ))}
      {store.hasOlder && (
        <div className="ms-notifications-more">
          <Button disabled={store.loadingOlder} onClick={loadOlder}>
            {store.loadingOlder ? "Loading" : "Load older"}
          </Button>
        </div>
      )}
    </Sheet>
  );
}

/**
 * One row: when, what, and where it came from.
 *
 * A row the panel was opened at brings itself into view, which is what
 * clicking a toast body does — the toast is a glimpse and this is the record,
 * and landing anywhere but on that message would make the click a lie.
 */
function Row({
  notification,
  focused,
  onAsked,
}: {
  notification: Notification;
  focused: boolean;
  onAsked: () => void;
}) {
  return (
    <li
      className={focused ? "ms-notification ms-notification--focused" : "ms-notification"}
      ref={(element) => {
        if (focused && element !== null) {
          element.scrollIntoView({ block: "center" });
        }
      }}
    >
      <div className="ms-notification-line">
        <time className="ms-notification-time" dateTime={notification.at}>
          {clockTime(notification.at)}
        </time>
        <Chip>{notification.source}</Chip>
      </div>
      {/* An alert is the steward saying something is wrong, so its row is a
          trouble line: a toast that flashed by is asked about from here
          (g1-s68 D5). The press closes the panel, which is window-modal, so
          the answer is where the human can read it. */}
      {notification.source === "alert" ? (
        <Trouble text={notification.message} onAsked={onAsked} />
      ) : (
        <p className="ms-notification-message">{notification.message}</p>
      )}
      {!notification.delivered && (
        <Trouble text={`not delivered to macOS: ${notification.error}`} onAsked={onAsked} />
      )}
    </li>
  );
}
