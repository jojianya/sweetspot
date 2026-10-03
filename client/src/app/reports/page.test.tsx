// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { reportEmptyCopy } from "./page";

describe("reportEmptyCopy", () => {
  it("keeps the per-status wording", () => {
    expect(reportEmptyCopy("pending")).toEqual({
      title: "No pending reports",
      description: "You're all caught up.",
    });
    expect(reportEmptyCopy("reviewed")).toEqual({
      title: "No reviewed reports",
      description: "Try another filter.",
    });
    expect(reportEmptyCopy("actioned")).toEqual({
      title: "No actioned reports",
      description: "Try another filter.",
    });
  });

  it("does not tell All-tab viewers to try another filter", () => {
    expect(reportEmptyCopy("all")).toEqual({
      title: "No reports",
      description: "No reports have been filed yet.",
    });
  });
});
