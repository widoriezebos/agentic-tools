import { describe, expect, it } from "vitest";

import { activeSection, HOME_PATH, isReserved, pathFor, projectSections, routeFor, sectionFor, sections } from "./routes";

describe("sections", () => {
  it("are the eight destinations, in rail order", () => {
    expect(sections.map((section) => section.id)).toEqual([
      "brain",
      "overview",
      "project",
      "backlog",
      "fleet",
      "decisions",
      "application",
      "settings",
    ]);
    expect(sections.map((section) => section.path)).toEqual([
      "/brain",
      "/overview",
      "/project",
      "/backlog",
      "/fleet",
      "/decisions",
      "/application",
      "/settings",
    ]);
  });

  it("put the six project sections between the Brain and Settings", () => {
    expect(projectSections.map((section) => section.id)).toEqual([
      "overview",
      "project",
      "backlog",
      "fleet",
      "decisions",
      "application",
    ]);
  });

  it("name a path and a path names them", () => {
    for (const section of sections) {
      expect(pathFor(section.id)).toBe(section.path);
      expect(sectionFor(section.path)?.id).toBe(section.id);
    }
    expect(pathFor("goals")).toBeNull();
  });

  it("emit no trailing slash, and match a pasted one", () => {
    for (const section of sections) {
      expect(section.path.endsWith("/")).toBe(false);
      expect(sectionFor(`${section.path}/`)?.id).toBe(section.id);
    }
  });

  it("own everything beneath their prefix", () => {
    expect(sectionFor("/backlog/goals/g1")?.id).toBe("backlog");
    expect(sectionFor("/settings/appearance")?.id).toBe("settings");
    expect(sectionFor("/backlogged")).toBeNull();
    expect(sectionFor("/nothing/here")).toBeNull();
  });

  it("send the root to Overview", () => {
    expect(HOME_PATH).toBe("/overview");
    expect(sectionFor("/")?.id).toBe("overview");
    expect(activeSection("/")?.id).toBe("overview");
  });
});

describe("activeSection", () => {
  // This build registers one route per section and nothing beneath, so an
  // address beneath a section matches no route and is the not-found pane. The
  // header and the rail have to agree with the router about that.
  it("matches a section exactly, and nothing beneath it", () => {
    expect(activeSection("/backlog")?.id).toBe("backlog");
    expect(activeSection("/backlog/")?.id).toBe("backlog");
    expect(activeSection("/backlog/goals/g1")).toBeNull();
    expect(activeSection("/nothing/here")).toBeNull();
  });
});

describe("reserved prefixes", () => {
  it("belong to the server, exactly and beneath", () => {
    for (const path of ["/-", "/-/anything", "/api", "/api/workspace", "/assets", "/assets/app.js"]) {
      expect({ path, reserved: isReserved(path) }).toEqual({ path, reserved: true });
    }
  });

  it("do not catch a path that merely begins with the same letters", () => {
    for (const path of ["/", "/overview", "/apiary", "/assetsmith", "/-view"]) {
      expect({ path, reserved: isReserved(path) }).toEqual({ path, reserved: false });
    }
  });

  it("are never a section's path", () => {
    for (const section of sections) {
      expect({ path: section.path, reserved: isReserved(section.path) }).toEqual({ path: section.path, reserved: false });
    }
  });
});

describe("routeFor", () => {
  it("resolves a section reference", () => {
    expect(routeFor({ kind: "section", id: "fleet" })).toBe("/fleet");
  });

  it("answers null for a kind this build has no view for", () => {
    expect(routeFor({ kind: "goal", id: "g1-s9" })).toBeNull();
    expect(routeFor({ kind: "seat", id: "implementer" })).toBeNull();
    expect(routeFor({ kind: "question", id: "d28" })).toBeNull();
    expect(routeFor({ kind: "section" })).toBeNull();
    expect(routeFor({ kind: "section", id: "nowhere" })).toBeNull();
  });

  it("never answers with a reserved prefix", () => {
    for (const section of sections) {
      const resolved = routeFor({ kind: "section", id: section.id });
      expect(resolved).not.toBeNull();
      expect(isReserved(resolved as string)).toBe(false);
    }
  });
});
