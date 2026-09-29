import { Component, type ErrorInfo, type ReactNode } from "react";

import { Button } from "./controls";
import { Trouble } from "./Trouble";
import { BROKEN_DRAWER, BROKEN_ROOM, boundaryText } from "./troubling";

/**
 * A pane that throws takes down the pane, not the shell: the rail, the header
 * and the dock keep working, and what failed is named rather than left as a
 * blank rectangle. A class component because that is the only way React lets a
 * render error be caught.
 *
 * What it names is a trouble line (g1-s68 D1), so the throw can be asked
 * about: the Partner is told what the page was showing and the error by name.
 * Where the boundary stands around a conversation's own renderer — the room,
 * or the drawer — the answer could not be read until a reload, so the line
 * says so and offers the reload instead of the Ask (g1-s68 D2, S68-08).
 */
type State = { error: Error | null };

/** Which conversation this boundary would take down with its pane, if any. */
type Holds = "room" | "drawer" | undefined;

export class ErrorBoundary extends Component<{ children: ReactNode; conversation?: Holds }, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: unknown): State {
    return { error: error instanceof Error ? error : new Error(String(error)) };
  }

  componentDidCatch(error: unknown, info: ErrorInfo): void {
    // The console is the only place this can go: the page has no server to
    // report to, and a swallowed error is a mystery for the next reader.
    console.error(error, info.componentStack);
  }

  render(): ReactNode {
    const { error } = this.state;
    if (error === null) {
      return this.props.children;
    }
    return <Caught error={error} conversation={this.props.conversation} />;
  }
}

/** What a caught pane shows: the trouble, and the reload. */
export function Caught({ error, conversation }: { error: Error; conversation?: Holds }) {
  const broken = conversation === "room" ? BROKEN_ROOM : conversation === "drawer" ? BROKEN_DRAWER : undefined;
  return (
    <div className="ms-caught">
      <Trouble text={boundaryText(error)} role="alert" as="div" variant="pane" broken={broken}>
        <h2 className="ms-trouble-heading">This pane could not be rendered</h2>
        <pre className="ms-trouble-detail">
          {error.name}: {error.message}
        </pre>
      </Trouble>
      {broken === undefined && (
        <Button
          onClick={() => {
            location.reload();
          }}
        >
          Reload page
        </Button>
      )}
    </div>
  );
}
