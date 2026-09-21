import { describe, expect, it } from "vitest";

import { ageBetween, clockTime, dateAndTime, shortTip, UNKNOWN } from "./format";

/**
 * Times a human reads.
 *
 * The assertions below hold in any time zone, because a test that only passed
 * where it was written would say nothing about the machine a human runs this
 * on: one instant written two ways renders the same, and an age is a
 * difference rather than a wall clock.
 */

describe("times on the page", () => {
  it("render one instant the same however it was written", () => {
    expect(dateAndTime("2026-09-21T18:39:42Z")).toBe(dateAndTime("2026-09-21T20:39:42+02:00"));
    expect(clockTime("2026-09-21T18:39:42Z")).toBe(clockTime("2026-09-21T13:39:42-05:00"));
  });

  it("carry the calendar day and the clock to the minute, and the clock to the second", () => {
    expect(dateAndTime("2026-09-21T18:39:42Z")).toMatch(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}$/);
    expect(clockTime("2026-09-21T18:39:42Z")).toMatch(/^\d{2}:\d{2}:\d{2}$/);
  });

  it("move with the instant", () => {
    const [earlyDate, earlyClock] = dateAndTime("2026-09-21T18:00:00Z").split(" ");
    const [lateDate, lateClock] = dateAndTime("2026-09-21T18:01:00Z").split(" ");
    expect(earlyDate).toBe(lateDate);
    expect(Number(lateClock.slice(3)) - Number(earlyClock.slice(3))).toBe(1);
  });

  it("say so when the record carries no instant", () => {
    for (const absent of ["", "not a time", "2026-13-45T99:99:99Z"]) {
      expect({ absent, rendered: dateAndTime(absent) }).toEqual({ absent, rendered: UNKNOWN });
      expect({ absent, rendered: clockTime(absent) }).toEqual({ absent, rendered: UNKNOWN });
    }
  });
});

describe("ages", () => {
  it("use the largest unit that still says something", () => {
    const cases: [string, string, string][] = [
      ["2026-09-21T19:05:00Z", "2026-09-21T19:05:30Z", "less than a minute"],
      ["2026-09-21T19:00:00Z", "2026-09-21T19:31:00Z", "31 min"],
      ["2026-09-21T16:05:12Z", "2026-09-21T19:05:12Z", "3 h"],
      ["2026-09-20T19:05:12Z", "2026-09-21T19:05:12Z", "24 h"],
      ["2026-09-18T19:05:12Z", "2026-09-21T19:05:12Z", "3 d"],
    ];
    for (const [from, to, age] of cases) {
      expect({ from, to, age: ageBetween(from, to) }).toEqual({ from, to, age });
    }
  });

  it("read an instant in the future as no age rather than a negative one", () => {
    expect(ageBetween("2026-09-21T20:00:00Z", "2026-09-21T19:00:00Z")).toBe("less than a minute");
  });

  it("say so when either end is missing", () => {
    expect(ageBetween("", "2026-09-21T19:05:12Z")).toBe(UNKNOWN);
    expect(ageBetween("2026-09-21T19:05:12Z", "")).toBe(UNKNOWN);
  });
});

describe("the short tip", () => {
  it("is the seven characters a human compares by eye", () => {
    expect(shortTip("c5d517f427e35e19c2944ab9aefee4d9c9cd9e6c")).toBe("c5d517f");
    expect(shortTip("")).toBe("");
  });
});
