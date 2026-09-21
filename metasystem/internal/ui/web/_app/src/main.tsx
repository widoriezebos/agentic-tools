import { setNonce } from "get-nonce";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App } from "./App";
import { readNonce } from "./nonce";
import "./fonts.css";
import "./index.css";

// Before the first render: react-style-singleton, which the dialog's scroll
// lock mounts, reads this when it appends its <style>, and the policy allows an
// inline style only with this response's nonce.
setNonce(readNonce(document));

const container = document.getElementById("root");
if (container === null) {
  throw new Error("the interface page carries no root element");
}

createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
