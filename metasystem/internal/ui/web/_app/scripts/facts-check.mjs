import { chromium } from "playwright";

import { expectedStages } from "./facts.mjs";

/**
 * Proves in a real browser that the pages say what the server's own API says:
 *
 *   node scripts/facts-check.mjs http://127.0.0.1:7878
 *
 * A count or a state on a page is a fact the engine holds, and a page that
 * shows another number is wrong however good it looks. So each page is opened
 * at 1480 px wide and read beside the resource of the same server that carries
 * the same fact, and every disagreement is listed:
 *
 *   1. Overview's "Needs you", the Decisions page's Inbox and the inbox count
 *      in /api/decisions are one number, and Overview's "Open questions" is
 *      the number of asks from a seat in that inbox.
 *   2. An In Progress card whose goal has a known stage on the host board shows
 *      that stage, and never "not recorded".
 *   3. Fleet does not say that no health verdict was recorded when /api/fleet
 *      carries one, and no machine that is working opens to "running nothing".
 *   4. Application lists no known problem twice, and no title carries a
 *      backtick.
 *   5. An address naming a tab or a goal that does not exist says so.
 *   6. The notifications panel lists the notices /api/notifications returns.
 *
 * The page and the resource are read moments apart, so a fact that changed in
 * between is reported as a disagreement: run it again before believing one.
 *
 * This check is run by hand against a running server. It is never a Go test
 * and never part of the gate: it needs a browser and a running server.
 */

const address = process.argv[2];
if (address === undefined || address === "") {
  process.stderr.write("usage: node scripts/facts-check.mjs <address>, for example http://127.0.0.1:7878\n");
  process.exit(2);
}
const base = address.replace(/\/+$/, "");

/** How long a page is given to show what it read. */
const WAIT = 15000;
/** The notices the panel reads before a human asks for older ones. */
const PANEL_PAGE = 200;

const failures = [];
const browser = await chromium.launch();
try {
  const page = await browser.newPage({ viewport: { width: 1480, height: 900 } });
  page.on("pageerror", (error) => failures.push(`the page threw: ${error.message}`));

  /** One resource of the server, or null with the refusal recorded. */
  const served = async (path) => {
    const response = await page.request.get(base + path, { headers: { Accept: "application/json" } });
    if (!response.ok()) {
      failures.push(`${path} answered ${String(response.status())}`);
      return null;
    }
    return response.json();
  };

  /** Opens a page and waits for the element that says it has been read. */
  const opened = async (path, ready) => {
    await page.goto(base + path, { waitUntil: "load" });
    try {
      await page.locator(ready).first().waitFor({ state: "visible", timeout: WAIT });
      return true;
    } catch {
      failures.push(`${path} never showed ${ready}`);
      return false;
    }
  };

  /** The whole number a piece of text carries, or null where it carries none. */
  const figure = (text) => {
    const found = /\d+/.exec(text ?? "");
    return found === null ? null : Number(found[0]);
  };

  // 1 and 2: Overview, beside the inbox and the host board.
  const decisions = await served("/api/decisions");
  const board = await served("/api/board");
  let overviewNeeds = null;
  if (await opened("/overview", "nav.ms-overview-glance")) {
    const named = (label, words) => ({ has: page.locator(label, { hasText: words }) });
    const tile = page.locator(".ms-overview-tile", named(".ms-overview-figure-label", /^Needs you$/));
    overviewNeeds = figure(await tile.locator(".ms-overview-figure").first().textContent());
    // A kind with nothing in it is left off the page, which is a count of none.
    const questions = page
      .locator(".ms-overview-kind", named(".ms-overview-kind-label", /^Open questions$/))
      .locator(".ms-overview-badge");
    const shownAsks = (await questions.count()) === 0 ? 0 : figure(await questions.first().textContent());
    if (decisions !== null) {
      const asks = decisions.needsYou.filter((need) => need.kind === "ask").length;
      if (shownAsks !== asks) {
        failures.push(`Overview says ${String(shownAsks)} open questions; /api/decisions carries ${String(asks)} asks`);
      }
    }

    const stages = expectedStages(board);
    const cards = await page.locator(".ms-overview-card").evaluateAll((items) =>
      items.map((item) => ({
        id: item.querySelector(".ms-overview-card-id")?.textContent ?? "",
        chips: [...item.querySelectorAll(".ms-chip")].map((chip) => chip.textContent ?? ""),
      })),
    );
    for (const card of cards) {
      const stage = stages.get(card.id);
      if (stage !== undefined && (card.chips.includes("not recorded") || !card.chips.includes(stage))) {
        failures.push(`In Progress card ${card.id} shows ${card.chips.join(", ")}; the host board says ${stage}`);
      }
    }
  }
  if (decisions !== null && (await opened("/decisions", '[role="tab"]'))) {
    const inbox = figure(await page.getByRole("tab", { name: /^Inbox/ }).first().textContent());
    const counted = decisions.counts.needsYou;
    if (overviewNeeds !== counted || inbox !== counted) {
      failures.push(
        `Needs you is ${String(overviewNeeds)} on Overview and ${String(inbox)} on Decisions; /api/decisions counts ${String(counted)}`,
      );
    }
  }

  // 3: Fleet, beside its own resource. An opened row is in the page whether a
  // viewer opened it or not, so it is read where it stands.
  const fleet = await served("/api/fleet");
  if (fleet !== null && (await opened("/fleet", ".ms-fleet-fact"))) {
    const said = (await page.locator(".ms-fleet").first().textContent()) ?? "";
    if (fleet.this.health !== null && said.includes("no health verdict")) {
      failures.push("Fleet says no health verdict was recorded; /api/fleet carries one");
    }
    for (const machine of fleet.machines) {
      if (machine.working.length === 0 && machine.running === null) {
        continue;
      }
      const row = (await page.locator(`[id="ms-fleet-work-${machine.machine}"]`).first().textContent()) ?? "";
      if (row.includes("running nothing")) {
        failures.push(`Fleet says ${machine.machine} is running nothing; /api/fleet says it is working`);
      }
    }
  }

  // 4: Application's known problems, the concluded ones included.
  const titled = ".ms-application-block-title";
  if (await opened("/application", `${titled}:has-text("Known problems")`)) {
    const block = page.locator(".ms-application-block", { has: page.locator(titled, { hasText: "Known problems" }) });
    const concluded = block.getByRole("button", { name: /^Show concluded/ });
    if ((await concluded.count()) > 0) {
      await concluded.first().click();
    }
    const rows = await block.locator(".ms-application-row-item").evaluateAll((items) =>
      items.map((item) => ({
        id: item.querySelector(".ms-application-row-id")?.textContent ?? "",
        said: item.querySelector(".ms-application-row-said")?.textContent ?? "",
      })),
    );
    const seen = new Set();
    for (const row of rows) {
      if (seen.has(row.id)) {
        failures.push(`Application lists known problem ${row.id} twice`);
      }
      seen.add(row.id);
      if (row.said.includes("`")) {
        failures.push(`Application shows a backtick in the title of ${row.id}: ${row.said}`);
      }
    }
  }

  // 5: an address that names nothing. The helper records the failure.
  for (const path of ["/project/no-such-tab", "/backlog/goal/no-such-goal"]) {
    await opened(path, 'h2:has-text("No page at this address")');
  }

  // 6: the notifications panel, beside the history it reads.
  const history = await served(`/api/notifications?limit=${String(PANEL_PAGE)}`);
  if (history !== null && (await opened("/overview", "nav.ms-overview-glance"))) {
    const notices = Math.min(history.notifications.length, PANEL_PAGE);
    await page.getByRole("button", { name: "Notifications", exact: true }).first().click();
    const listed = page.locator("#notifications-panel .ms-notification");
    // The count below is what reports a panel that never listed anything.
    const waited = notices > 0 ? listed.first() : page.locator("#notifications-panel");
    await waited.waitFor({ state: "visible", timeout: WAIT }).catch(() => undefined);
    const shown = await listed.count();
    if (shown !== notices) {
      failures.push(`the notifications panel lists ${String(shown)}; /api/notifications returns ${String(notices)}`);
    }
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
process.stdout.write(`the pages at ${address} say what its API serves\n`);
