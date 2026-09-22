import { Button } from "./controls";

/**
 * The composer, disabled until the Project Partner is connected.
 *
 * It is shown rather than hidden because a human looking for where to type
 * should find it and read why it does nothing; the reason is on screen, not in
 * a tooltip, and the field names it through aria-describedby.
 */
export const COMPOSER_REASON = "Your Project Partner is not connected in this build.";

export function Composer() {
  return (
    <div className="ms-composer">
      <label className="ms-visually-hidden" htmlFor="composer">
        Message to your Project Partner
      </label>
      <textarea
        id="composer"
        className="ms-composer-field"
        placeholder="Message to your Project Partner"
        aria-describedby="composer-reason"
        disabled
      />
      <div className="ms-composer-foot">
        <span className="ms-composer-reason" id="composer-reason">
          {COMPOSER_REASON}
        </span>
        <Button primary disabled>
          Send
        </Button>
      </div>
    </div>
  );
}
