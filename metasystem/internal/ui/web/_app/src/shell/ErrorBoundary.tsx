import { Component, type ErrorInfo, type ReactNode } from "react";

import { Button } from "./controls";

/**
 * A pane that throws takes down the pane, not the shell: the rail, the header
 * and the dock keep working, and what failed is named rather than left as a
 * blank rectangle. A class component because that is the only way React lets a
 * render error be caught.
 */
type State = { error: Error | null };

export class ErrorBoundary extends Component<{ children: ReactNode }, State> {
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
    return (
      <div className="ms-error">
        <h2 className="ms-error-heading">This pane could not be rendered</h2>
        <pre className="ms-error-detail">{error.message}</pre>
        <Button
          onClick={() => {
            location.reload();
          }}
        >
          Reload page
        </Button>
      </div>
    );
  }
}
