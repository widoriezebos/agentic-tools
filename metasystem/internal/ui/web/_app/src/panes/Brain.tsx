import { BookOpen, MessageSquare } from "lucide-react";

import { EmptyState } from "./Pane";

/**
 * What the dock and the focused conversation both say, in one place. The file
 * and the function keep the kit's name; the words they render do not.
 */
export function BrainStatement() {
  return <EmptyState id="brain" icon={MessageSquare} />;
}

export function SubjectStatement() {
  return <EmptyState id="subject" icon={BookOpen} />;
}
