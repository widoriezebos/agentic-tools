import * as Tooltip from "@radix-ui/react-tooltip";
import { BrowserRouter } from "react-router";

import { TOOLTIP_DELAY } from "./shell/controls";
import { IdentityProvider } from "./shell/identity";
import { Shell } from "./shell/Shell";

/**
 * The application: a router, the workspace the header reads once, and the
 * shell. Everything a human sees is beneath it, and every destination is a
 * URL that survives a reload and the back button.
 */
export function App() {
  return (
    <BrowserRouter>
      <Tooltip.Provider delayDuration={TOOLTIP_DELAY}>
        <IdentityProvider>
          <Shell />
        </IdentityProvider>
      </Tooltip.Provider>
    </BrowserRouter>
  );
}
