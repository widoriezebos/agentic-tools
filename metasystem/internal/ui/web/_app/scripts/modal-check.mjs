import { chromium } from "playwright";

/**
 * Proves the nonce path in a real browser against the embedded build:
 *
 *   node scripts/modal-check.mjs http://127.0.0.1:7878
 *
 * Opening the dialog mounts Radix's overlay, whose scroll lock appends a
 * <style> to <head>. Under this server's policy that element is allowed only
 * because it carries the response's nonce, so three things must hold together:
 * the body is scroll-locked, the appended style carries the page's nonce, and
 * the browser reported no policy violation.
 *
 * This check is run by hand during the walkthrough. It is never a Go test and
 * never part of the gate: it needs a browser and a running server.
 *
 * g1-s35 made every sheet but sign-in modal for the work area, and a sheet of
 * that mode is told modal={false}: Radix locks no scroll for it, appends no
 * style, and there is nothing here for it to check. Sign-in is the one sheet
 * that still takes Radix's own modality, so it is the dialog this check must
 * be pointed at — a page whose "Open a dialog" opens anything else will fail
 * on the overflow, and rightly: it would be proving nothing.
 */

const address = process.argv[2];
if (address === undefined || address === "") {
  process.stderr.write("usage: node scripts/modal-check.mjs <address>, for example http://127.0.0.1:7878\n");
  process.exit(2);
}

const failures = [];
const browser = await chromium.launch();
try {
  const page = await browser.newPage();
  await page.addInitScript(() => {
    window.__policyViolations = [];
    document.addEventListener("securitypolicyviolation", (event) => {
      window.__policyViolations.push(`${event.violatedDirective} blocked ${event.blockedURI}`);
    });
  });
  page.on("pageerror", (error) => failures.push(`the page threw: ${error.message}`));

  const response = await page.goto(address, { waitUntil: "load" });
  if (response === null || !response.ok()) {
    failures.push(`${address} answered ${response === null ? "nothing" : String(response.status())}`);
  }

  await page.getByRole("button", { name: "Open a dialog" }).click();
  await page.getByRole("dialog").waitFor({ state: "visible" });

  const observed = await page.evaluate(() => ({
    overflow: getComputedStyle(document.body).overflow,
    metaNonce: document.querySelector("meta[property='csp-nonce']")?.nonce ?? "",
    styleNonces: [...document.head.querySelectorAll("style")].map((style) => style.nonce),
    violations: window.__policyViolations,
  }));

  if (observed.overflow !== "hidden") {
    failures.push(`the body's overflow is ${observed.overflow}, not hidden: the scroll lock did not apply`);
  }
  if (observed.metaNonce === "") {
    failures.push("the page carries no nonce: the server did not substitute one");
  }
  const carrying = observed.styleNonces.filter((nonce) => nonce !== "");
  if (carrying.length !== 1) {
    failures.push(`<head> holds ${String(carrying.length)} style elements with a nonce, not 1`);
  } else if (carrying[0] !== observed.metaNonce) {
    failures.push("the appended style's nonce is not the page's nonce");
  }
  for (const violation of observed.violations) {
    failures.push(`content security policy violation: ${violation}`);
  }
} finally {
  await browser.close();
}

if (failures.length > 0) {
  for (const failure of failures) {
    process.stderr.write(`${failure}\n`);
  }
  process.exit(1);
}
process.stdout.write(`the dialog's scroll lock is allowed by the page nonce at ${address}\n`);
