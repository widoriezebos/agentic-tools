import * as Tooltip from "@radix-ui/react-tooltip";
import { BrowserRouter } from "react-router";

import { NotificationsProvider } from "./notifications/store";
import { TOOLTIP_DELAY } from "./shell/controls";
import { IdentityProvider } from "./shell/identity";
import { Shell } from "./shell/Shell";

/**
 * The application: a router, the workspace the header reads once, what the
 * steward has said, and the shell. Everything a human sees is beneath it, and
 * every destination is a URL that survives a reload and the back button.
 *
 * The notifications provider is outside the shell because two of the three
 * surfaces it feeds are not in the shell's layout at all: the panel is a sheet
 * and the toasts stand over everything, and only the bell is a control in the
 * header.
 */
export function App() {
  return (
    <BrowserRouter>
      <Tooltip.Provider delayDuration={TOOLTIP_DELAY}>
        <IdentityProvider>
          <NotificationsProvider>
            <Shell />
          </NotificationsProvider>
        </IdentityProvider>
      </Tooltip.Provider>
    </BrowserRouter>
  );
}
