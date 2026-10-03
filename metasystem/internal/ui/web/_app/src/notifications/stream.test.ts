import { afterEach, describe, expect, it } from "vitest";

import { followReopens, onceStreamOpens, onStreamReopen } from "./stream";

/**
 * The Partner and the fleet's readers each read once when they mount, and read
 * again when the stream comes back after it was down. The connection's first
 * open is the page's own load, so a page load reads each of them once.
 */
describe("a reopen of the stream", () => {
  function follow(): { source: EventTarget; reads: () => number; stop: () => void } {
    const source = new EventTarget();
    let reads = 0;
    const stopListening = onStreamReopen(() => {
      reads += 1;
    });
    const stopFollowing = followReopens(source);
    return {
      source,
      reads: () => reads,
      stop: () => {
        stopFollowing();
        stopListening();
      },
    };
  }

  it("is not the first open on page load, so nothing reads a second time", () => {
    const { source, reads, stop } = follow();

    source.dispatchEvent(new Event("open"));
    expect(reads()).toBe(0);
    stop();
  });

  it("is an open after the connection dropped, and each such open reads once", () => {
    const { source, reads, stop } = follow();

    source.dispatchEvent(new Event("open"));
    source.dispatchEvent(new Event("error"));
    source.dispatchEvent(new Event("open"));
    expect(reads()).toBe(1);
    // An open with no failure before it says nothing new.
    source.dispatchEvent(new Event("open"));
    expect(reads()).toBe(1);
    source.dispatchEvent(new Event("error"));
    source.dispatchEvent(new Event("open"));
    expect(reads()).toBe(2);
    stop();
  });

  it("is an open after a first attempt that failed, which may have missed something", () => {
    const { source, reads, stop } = follow();

    source.dispatchEvent(new Event("error"));
    source.dispatchEvent(new Event("open"));
    expect(reads()).toBe(1);
    stop();
  });
});

/**
 * The server sends a beat only to a stream it already holds, and keeps none
 * for later. The Partner reads its conversation on page load once the stream
 * has opened, so a turn is either in what it read or reaches it as beats.
 */
describe("the Partner's read on page load", () => {
  const leaving: (() => void)[] = [];
  afterEach(() => {
    for (const leave of leaving.splice(0)) {
      leave();
    }
  });

  // A page: the Partner mounts and waits as the store does, and the stream is
  // followed after it, as a parent's effects run after a child's.
  function load() {
    const source = new EventTarget();
    const server: string[] = [];
    let holding = false;
    let shown: string[] = [];
    let reads = 0;
    const read = () => {
      reads += 1;
      shown = [...server];
    };
    leaving.push(onceStreamOpens(read), onStreamReopen(read), followReopens(source));
    return {
      reads: () => reads,
      shown: () => shown,
      turn: (id: string) => {
        server.push(id);
        if (holding) {
          shown = [...shown, id];
        }
      },
      open: () => {
        holding = true;
        source.dispatchEvent(new Event("open"));
      },
      fail: () => source.dispatchEvent(new Event("error")),
    };
  }

  it("shows a turn that started and ended before the stream opened", () => {
    const page = load();
    page.turn("t1");
    page.open();
    page.turn("t2");
    expect(page.shown()).toEqual(["t1", "t2"]);
  });

  it("reads once on a normal load, and only after the stream opened", () => {
    const page = load();
    expect(page.reads()).toBe(0);
    page.open();
    expect(page.reads()).toBe(1);
    page.open();
    expect(page.reads()).toBe(1);
  });

  it("reads when the stream fails before it opens, and again when it comes back", () => {
    const page = load();
    page.turn("t1");
    page.fail();
    expect(page.shown()).toEqual(["t1"]);
    page.open();
    expect(page.reads()).toBe(2);
  });
});
