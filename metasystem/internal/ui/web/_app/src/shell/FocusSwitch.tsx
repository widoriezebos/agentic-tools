import { Button } from "./controls";

/**
 * Below 960 the focused view is one column, so the conversation and the
 * subject take turns. The choice is not remembered: a human who opens the
 * conversation wants the conversation.
 */
export type FocusedView = "conversation" | "subject";

export function FocusSwitch({ view, onChange }: { view: FocusedView; onChange: (view: FocusedView) => void }) {
  return (
    <div className="ms-switch ms-focus-switch" role="group" aria-label="View">
      <Button
        aria-pressed={view === "conversation"}
        onClick={() => {
          onChange("conversation");
        }}
      >
        Conversation
      </Button>
      <Button
        aria-pressed={view === "subject"}
        onClick={() => {
          onChange("subject");
        }}
      >
        Subject
      </Button>
    </div>
  );
}
