import { describe, expect, it } from "vitest";
import { formatTime } from "./format";

const SHAPE =
  /^(\d{1,2}) (Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sept|Sep|Oct|Nov|Dec) \d{4}, \d{2}:\d{2}$/;

describe("formatTime", () => {
  it("formats a timestamp consistently regardless of environment locale", () => {
    const out = formatTime("2026-09-16T10:30:00Z");
    expect(out).toMatch(SHAPE);
    expect(out).toContain("2026");
  });

  it("returns an empty string for an invalid date", () => {
    expect(formatTime("not-a-date")).toBe("");
  });
});