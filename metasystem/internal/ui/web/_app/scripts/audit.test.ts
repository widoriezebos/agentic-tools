import { type ChildProcess, spawn } from "node:child_process";
import { createServer, type ServerResponse } from "node:http";
import { mkdtempSync, writeFileSync } from "node:fs";
import type { AddressInfo } from "node:net";
import { tmpdir } from "node:os";
import path from "node:path";
import { gunzipSync } from "node:zlib";
import { afterEach, describe, expect, it } from "vitest";

import { auditArgs, DECLARED_REGISTRY, runAudit } from "./bundle.mjs";

/**
 * The audit gate cannot be made green by ambient configuration.
 *
 * Every case drives the production argument vector against a pinned loopback
 * registry that answers 500, so the run ends in a refusal through npm's
 * endpoint-error path, and asserts three things: the refusal, that the pinned
 * registry received exactly one bulk request naming every package in the
 * lockfile, and that a second loopback registry — the one the ambient setting
 * points at, answering 200 with the empty advisory object that would make a
 * report clean — received nothing.
 *
 * Each case is paired with a control that removes one flag and shows what
 * fails without it. The cases are what must hold; the controls are what the
 * flags are for.
 *
 * The recorders answer on this worker's event loop, and runAudit waits for npm
 * synchronously, so every networked case runs the real runAudit in a child
 * Node process and awaits its result; run here, it would block the loop npm is
 * waiting on.
 */

const TIMEOUT = 120_000;
/** Ends a child audit, and the npm it is waiting on, before the case's own timeout. */
const CHILD_DEADLINE = 100_000;
const BULK = "/-/npm/v1/security/advisories/bulk";

type Recorded = { method: string; url: string; body: string };

type Recorder = { origin: string; seen: Recorded[]; stop: () => Promise<void> };

type Audit = ReturnType<typeof runAudit>;

const running: Recorder[] = [];
const audits: ChildProcess[] = [];

/** Kills a child audit's whole process group, npm included, if it has not exited. */
function end(child: ChildProcess): void {
  if (child.pid !== undefined && child.exitCode === null && child.signalCode === null) {
    try {
      process.kill(-child.pid, "SIGKILL");
    } catch {
      // The group is already gone.
    }
  }
}

afterEach(async () => {
  audits.splice(0).forEach(end);
  await Promise.all(running.splice(0).map((recorder) => recorder.stop()));
});

/** Imports the actual bundle script and prints what its runAudit returns. */
const CHILD = `const { runAudit } = await import(process.argv[1]);
process.stdout.write(JSON.stringify(runAudit(process.env, JSON.parse(process.argv[2]), process.argv[3])));`;

/**
 * Runs the actual runAudit in a child Node process that leads its own process
 * group, with the fixture environment as the child's environment. A deadline
 * or a failed case kills the group, so no npm outlives the case.
 */
function audit(env: NodeJS.ProcessEnv, args: string[], cwd: string): Promise<Audit> {
  const bundle = new URL("./bundle.mjs", import.meta.url).href;
  const child = spawn(
    process.execPath,
    ["--input-type=module", "--eval", CHILD, bundle, JSON.stringify(args), cwd],
    { cwd, env, detached: true, stdio: ["ignore", "pipe", "pipe"] },
  );
  audits.push(child);
  const stdout: Buffer[] = [];
  const stderr: Buffer[] = [];
  child.stdout?.on("data", (chunk: Buffer) => stdout.push(chunk));
  child.stderr?.on("data", (chunk: Buffer) => stderr.push(chunk));
  return new Promise<Audit>((resolve, reject) => {
    const deadline = setTimeout(() => {
      end(child);
      reject(new Error(`the child audit did not finish within ${String(CHILD_DEADLINE)} ms`));
    }, CHILD_DEADLINE);
    child.on("error", (error) => {
      clearTimeout(deadline);
      end(child);
      reject(error);
    });
    child.on("close", (code, signal) => {
      clearTimeout(deadline);
      if (code !== 0) {
        reject(
          new Error(
            `the child audit exited ${String(code ?? signal)}: ${Buffer.concat(stderr).toString("utf8")}`,
          ),
        );
        return;
      }
      resolve(JSON.parse(Buffer.concat(stdout).toString("utf8")) as Audit);
    });
  });
}

async function recorder(respond: (response: ServerResponse) => void): Promise<Recorder> {
  const seen: Recorded[] = [];
  const server = createServer((request, response) => {
    const chunks: Buffer[] = [];
    request.on("data", (chunk: Buffer) => chunks.push(chunk));
    request.on("end", () => {
      const raw = Buffer.concat(chunks);
      const gzipped = request.headers["content-encoding"] === "gzip" && raw.length > 0;
      seen.push({
        method: request.method ?? "",
        url: request.url ?? "",
        body: (gzipped ? gunzipSync(raw) : raw).toString("utf8"),
      });
      respond(response);
    });
  });
  await new Promise<void>((resolve) => {
    server.listen(0, "127.0.0.1", resolve);
  });
  const { port } = server.address() as AddressInfo;
  const stop = async (): Promise<void> => {
    server.closeAllConnections();
    await new Promise<void>((resolve) => {
      server.close(() => {
        resolve();
      });
    });
  };
  const started: Recorder = { origin: `http://127.0.0.1:${String(port)}/`, seen, stop };
  running.push(started);
  return started;
}

/** Refuses every request, so no case can end in a clean report. */
function pinnedRegistry(): Promise<Recorder> {
  return recorder((response) => {
    response.writeHead(500, { "content-type": "application/json" });
    response.end('{"error":"the pinned fixture registry refuses"}');
  });
}

/** Answers the empty advisory object, which would make a report clean. */
function ambientRegistry(): Promise<Recorder> {
  return recorder((response) => {
    response.writeHead(200, { "content-type": "application/json" });
    response.end("{}");
  });
}

/**
 * A synthetic project: one production, one development, one optional, and one
 * peer-only package, so every --include flag has a node that is dropped without
 * it. npm loads the lockfile as the virtual tree with no node_modules on disk
 * and takes each node's dev, optional, and peer flag from its entry.
 */
function fixtureProject(): string {
  const dir = mkdtempSync(path.join(tmpdir(), "metasystem-audit-"));
  const entry = (name: string, extra: Record<string, unknown>) => ({
    version: "1.0.0",
    resolved: `${DECLARED_REGISTRY}${name}/-/${name}-1.0.0.tgz`,
    ...extra,
  });
  writeFileSync(
    path.join(dir, "package.json"),
    `${JSON.stringify(
      {
        name: "audit-fixture",
        version: "1.0.0",
        private: true,
        dependencies: { a: "1.0.0" },
        devDependencies: { d: "1.0.0" },
        optionalDependencies: { o: "1.0.0" },
      },
      null,
      2,
    )}\n`,
  );
  writeFileSync(
    path.join(dir, "package-lock.json"),
    `${JSON.stringify(
      {
        name: "audit-fixture",
        version: "1.0.0",
        lockfileVersion: 3,
        requires: true,
        packages: {
          "": {
            name: "audit-fixture",
            version: "1.0.0",
            dependencies: { a: "1.0.0" },
            devDependencies: { d: "1.0.0" },
            optionalDependencies: { o: "1.0.0" },
          },
          "node_modules/a": entry("a", { peerDependencies: { p: "1.0.0" } }),
          "node_modules/d": entry("d", { dev: true }),
          "node_modules/o": entry("o", { optional: true }),
          "node_modules/p": entry("p", { peer: true }),
        },
      },
      null,
      2,
    )}\n`,
  );
  return dir;
}

/**
 * npm's update notifier asks the registry for npm's own latest version, which
 * is not the audit; off here, so the registries see only what the audit sends.
 */
function environment(cwd: string, ambient: Record<string, string>): NodeJS.ProcessEnv {
  return {
    ...process.env,
    NPM_CONFIG_CACHE: path.join(cwd, "npm-cache"),
    NPM_CONFIG_UPDATE_NOTIFIER: "false",
    ...ambient,
  };
}

function userConfig(cwd: string, lines: string[]): string {
  const file = path.join(cwd, ".npmrc");
  writeFileSync(file, `${lines.join("\n")}\n`);
  return file;
}

/** The names and versions npm asked the registry about. */
function bulkBody(recorded: Recorded): Record<string, string[]> {
  return JSON.parse(recorded.body) as Record<string, string[]>;
}

function expectOneBulkCall(pinned: Recorder): Record<string, string[]> {
  expect(pinned.seen).toHaveLength(1);
  expect(pinned.seen[0].method).toBe("POST");
  expect(pinned.seen[0].url).toBe(BULK);
  return bulkBody(pinned.seen[0]);
}

describe("the audit's argument vector", () => {
  it("is what the bundle script runs", () => {
    expect(auditArgs(DECLARED_REGISTRY)).toEqual([
      "audit",
      "--json",
      "--registry=https://registry.npmjs.org/",
      "--no-offline",
      "--include=dev",
      "--include=optional",
      "--include=peer",
    ]);
  });
});

describe("ambient configuration cannot make the audit green", () => {
  const cases: { name: string; ambient: (ambient: Recorder, cwd: string) => Record<string, string> }[] = [
    { name: "an offline environment variable", ambient: () => ({ NPM_CONFIG_OFFLINE: "true" }) },
    {
      name: "an npmrc taking it offline, narrowing it, and redirecting it",
      ambient: (ambient, cwd) => ({
        NPM_CONFIG_USERCONFIG: userConfig(cwd, ["offline=true", "omit=optional", `registry=${ambient.origin}`]),
      }),
    },
    { name: "an omitted optional type", ambient: () => ({ NPM_CONFIG_OMIT: "optional" }) },
    { name: "a production NODE_ENV with an omitted dev type", ambient: () => ({ NPM_CONFIG_OMIT: "dev", NODE_ENV: "production" }) },
    { name: "an omitted peer type", ambient: () => ({ NPM_CONFIG_OMIT: "peer" }) },
    { name: "a disabled audit", ambient: () => ({ NPM_CONFIG_AUDIT: "false" }) },
    { name: "another registry", ambient: (ambient) => ({ NPM_CONFIG_REGISTRY: ambient.origin }) },
    { name: "nothing at all", ambient: () => ({}) },
  ];

  for (const testCase of cases) {
    it(
      `refuses under ${testCase.name}`,
      async () => {
        const cwd = fixtureProject();
        const pinned = await pinnedRegistry();
        const ambient = await ambientRegistry();

        const { verdict } = await audit(environment(cwd, testCase.ambient(ambient, cwd)), auditArgs(pinned.origin), cwd);

        expect(verdict.green).toBe(false);
        expect(expectOneBulkCall(pinned)).toEqual({ a: ["1.0.0"], d: ["1.0.0"], o: ["1.0.0"], p: ["1.0.0"] });
        expect(ambient.seen).toEqual([]);
      },
      TIMEOUT,
    );
  }
});

describe("each flag has a control that fails without it", () => {
  const without = (flag: string, origin: string): string[] => auditArgs(origin).filter((argument) => argument !== flag);

  it(
    "without --no-offline an offline environment audits nothing and reports green",
    async () => {
      const cwd = fixtureProject();
      const pinned = await pinnedRegistry();
      const ambient = await ambientRegistry();

      const { verdict } = await audit(
        environment(cwd, { NPM_CONFIG_OFFLINE: "true" }),
        without("--no-offline", pinned.origin),
        cwd,
      );

      expect(verdict.green).toBe(true);
      expect(pinned.seen).toEqual([]);
      expect(ambient.seen).toEqual([]);
    },
    TIMEOUT,
  );

  it(
    "without --include=optional the optional package is not audited",
    async () => {
      const cwd = fixtureProject();
      const pinned = await pinnedRegistry();

      await audit(environment(cwd, { NPM_CONFIG_OMIT: "optional" }), without("--include=optional", pinned.origin), cwd);

      expect(Object.keys(expectOneBulkCall(pinned)).sort()).toEqual(["a", "d", "p"]);
    },
    TIMEOUT,
  );

  it(
    "without --include=dev a production NODE_ENV drops the development package",
    async () => {
      const cwd = fixtureProject();
      const pinned = await pinnedRegistry();

      await audit(
        environment(cwd, { NPM_CONFIG_OMIT: "dev", NODE_ENV: "production" }),
        without("--include=dev", pinned.origin),
        cwd,
      );

      expect(Object.keys(expectOneBulkCall(pinned)).sort()).toEqual(["a", "o", "p"]);
    },
    TIMEOUT,
  );

  it(
    "without --include=peer the peer-only package is not audited",
    async () => {
      const cwd = fixtureProject();
      const pinned = await pinnedRegistry();

      await audit(environment(cwd, { NPM_CONFIG_OMIT: "peer" }), without("--include=peer", pinned.origin), cwd);

      expect(Object.keys(expectOneBulkCall(pinned)).sort()).toEqual(["a", "d", "o"]);
    },
    TIMEOUT,
  );

  it(
    "without a pinned registry an ambient one answers, and its empty report is green",
    async () => {
      const cwd = fixtureProject();
      const pinned = await pinnedRegistry();
      const ambient = await ambientRegistry();

      const { verdict } = await audit(
        environment(cwd, { NPM_CONFIG_REGISTRY: ambient.origin }),
        without(`--registry=${pinned.origin}`, pinned.origin),
        cwd,
      );

      expect(verdict.green).toBe(true);
      expect(pinned.seen).toEqual([]);
      expect(ambient.seen.map((seen) => seen.url)).toEqual([BULK]);
    },
    TIMEOUT,
  );
});
