import { describe, expect, it } from "vitest";

import { sharedInTheMoment } from "./api";

/**
 * The shell's rail and the Fleet page read the fleet in the same moment — on
 * mount, and on the same `fleet` beat — and ask the server once between them.
 */
describe("a read shared in the moment", () => {
  it("asks once for the callers of one run, and again for a caller after it", async () => {
    let asked = 0;
    const read = sharedInTheMoment(() => {
      asked += 1;
      return Promise.resolve(asked);
    });

    const together = await Promise.all([read(), read(new AbortController().signal)]);
    expect(asked).toBe(1);
    expect(together).toEqual([1, 1]);
    // A read asked for after that run, as after an act, is a read of its own.
    expect(await read()).toBe(2);
    expect(asked).toBe(2);
  });

  it("ends one caller's wait on its own signal and leaves the other's answer", async () => {
    let answer: (value: string) => void = () => undefined;
    const read = sharedInTheMoment(
      () =>
        new Promise<string>((resolve) => {
          answer = resolve;
        }),
    );
    const aborter = new AbortController();

    const gone = read(aborter.signal);
    const kept = read();
    aborter.abort();
    answer("the fleet");

    await expect(gone).rejects.toMatchObject({ name: "AbortError" });
    await expect(kept).resolves.toBe("the fleet");
  });
});
