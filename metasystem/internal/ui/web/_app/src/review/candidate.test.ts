import { describe, expect, it } from "vitest";

import { candidateRequest, candidateRoute } from "./candidate";

/**
 * Try it names the version the page shows (fix round 3, F-1): the commit the
 * page read, never the review record, whose version another room can move.
 */
describe("Try it's requests", () => {
  it("name the commit the page shows on the read, the start and the stop", () => {
    const shown = "a".repeat(40);
    expect(candidateRequest(candidateRoute("g", "status"), false, shown))
      .toEqual({ url: `/api/app/g/status?commit=${shown}`, method: "GET", body: undefined });
    expect(candidateRequest(candidateRoute("g", "start"), true, shown))
      .toEqual({ url: "/api/app/g/start", method: "POST", body: JSON.stringify({ commit: shown }) });
    expect(candidateRequest(candidateRoute("g", "stop"), true, shown))
      .toEqual({ url: "/api/app/g/stop", method: "POST", body: JSON.stringify({ commit: shown }) });
    expect(candidateRequest(candidateRoute("g", "start"), true, "")).toEqual({ url: "/api/app/g/start", method: "POST", body: "{}" });
  });
});
