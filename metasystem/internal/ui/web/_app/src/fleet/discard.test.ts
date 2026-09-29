import { describe, expect, it } from "vitest";

import { discardAddress, ResourceError } from "./api";
import { failureCode, failureMessage } from "../shell/workspace";

/**
 * The discard act's address, and the refusal it can come back with.
 *
 * The request itself is the module's one request, which the cut guard keeps
 * to one place in this file; what is proved here is the part only this act
 * can get wrong: which address it posts to, and that a refusal's code travels
 * to the trouble line beside its words.
 */

describe("discarding a launch", () => {
  it("posts to the launch's own address beneath the fleet", () => {
    expect(discardAddress("01M3BQAVYXE2AT6F0JG9YB64PG")).toBe("/api/fleet/launches/01M3BQAVYXE2AT6F0JG9YB64PG/discard");
    // An id is one path segment, whatever a caller hands in.
    expect(discardAddress("a/b")).toBe("/api/fleet/launches/a%2Fb/discard");
  });

  it("carries a refusal's words and code to the trouble line", () => {
    const refused = new ResourceError(
      discardAddress("01M3BQAVYXE2AT6F0JG9YB64PG"),
      409,
      "the launch of m1f is still running",
      "SEAT_LAUNCH_DISCARD_RUNNING",
    );
    expect(failureMessage(refused)).toBe("the launch of m1f is still running");
    expect(failureCode(refused)).toBe("SEAT_LAUNCH_DISCARD_RUNNING");
  });
});
