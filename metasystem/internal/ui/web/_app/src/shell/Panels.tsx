import { Group, Panel, Separator } from "react-resizable-panels";
import { useRef, useState, type ReactNode } from "react";

import { viewportWidth } from "./media";
import {
  fitDockWidth,
  MAXIMUM_DOCK_WIDTH,
  MINIMUM_DOCK_WIDTH,
  MINIMUM_WORK_WIDTH,
  readDockWidth,
  writeDockWidth,
} from "../storage";

/**
 * The work area, the separator, and the dock.
 *
 * The stored width is a pixel count, clamped to the dock's own limits and then
 * to the room this viewport leaves the work area, before the group mounts.
 * From there the library owns the arithmetic: the dock keeps its pixel width
 * while the window resizes and the work area gives way first, down to its own
 * minimum, so a human dragging the window narrower never watches the dock
 * shrink while 400 pixels still fit.
 *
 * Only a human's own resize is written back. A mount, a window resize, or a
 * programmatic layout carries isUserInteraction false and writes nothing, so
 * the stored width survives a trip through a narrow window.
 */

type DockHandle = { getSize: () => { inPixels: number } };

export function Panels({ railWidth, work, dock }: { railWidth: number; work: ReactNode; dock: ReactNode }) {
  const dockRef = useRef<DockHandle | null>(null);
  const [initialWidth] = useState(() => fitDockWidth(readDockWidth(), viewportWidth(), railWidth));

  const save = (_layout: unknown, meta: { isUserInteraction: boolean }) => {
    if (!meta.isUserInteraction) {
      return;
    }
    const size = dockRef.current?.getSize();
    if (size !== undefined) {
      writeDockWidth(size.inPixels);
    }
  };

  return (
    <Group id="shell" className="ms-group" orientation="horizontal" disableCursor onLayoutChanged={save}>
      <Panel
        id="work"
        className="ms-panel-work"
        minSize={MINIMUM_WORK_WIDTH}
        groupResizeBehavior="preserve-relative-size"
      >
        {work}
      </Panel>
      <Separator className="ms-separator" aria-label="Resize the Brain dock" />
      <Panel
        id="dock"
        className="ms-panel-dock"
        defaultSize={initialWidth}
        minSize={MINIMUM_DOCK_WIDTH}
        maxSize={MAXIMUM_DOCK_WIDTH}
        groupResizeBehavior="preserve-pixel-size"
        panelRef={dockRef}
      >
        {dock}
      </Panel>
    </Group>
  );
}
