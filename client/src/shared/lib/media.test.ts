import { describe, expect, it } from "vitest";
import { resolveMediaUrl } from "./media";

describe("resolveMediaUrl", () => {
  it("returns same-origin path for localhost absolute URLs", () => {
    expect(resolveMediaUrl("http://localhost:8081/uploads/abc.webp")).toBe("/uploads/abc.webp");
  });

  it("returns same-origin path for stale LAN-IP absolute URLs", () => {
    expect(resolveMediaUrl("http://192.168.68.112:8081/uploads/abc.webp")).toBe("/uploads/abc.webp");
  });

  it("passes through already-relative uploads paths", () => {
    expect(resolveMediaUrl("/uploads/abc.webp")).toBe("/uploads/abc.webp");
  });

  it("keeps query and hash on uploads paths", () => {
    expect(resolveMediaUrl("/uploads/a.webp?v=2#h")).toBe("/uploads/a.webp?v=2#h");
    expect(resolveMediaUrl("http://192.168.68.112:8081/uploads/a.webp?v=2#h")).toBe(
      "/uploads/a.webp?v=2#h"
    );
  });

  it("handles bare uploads paths without a leading slash", () => {
    expect(resolveMediaUrl("uploads/a.webp")).toBe("/uploads/a.webp");
  });

  it("trims whitespace", () => {
    expect(resolveMediaUrl("  /uploads/a.webp  ")).toBe("/uploads/a.webp");
    expect(resolveMediaUrl("   ")).toBe("");
  });

  it("rejects javascript: URLs", () => {
    expect(resolveMediaUrl("javascript:alert(1)")).toBe("");
  });

  it("rejects protocol-relative URLs", () => {
    expect(resolveMediaUrl("//evil.com/uploads/x.webp")).toBe("");
  });

  it("allows blob previews explicitly", () => {
    const blob = "blob:http://localhost:3000/550e8400-e29b-41d4-a716-446655440000";
    expect(resolveMediaUrl(blob)).toBe(blob);
  });

  it("allows data:image URLs but rejects other data: types", () => {
    const png = "data:image/png;base64,iVBORw0KGgo=";
    expect(resolveMediaUrl(png)).toBe(png);
    expect(resolveMediaUrl("data:text/html,<h1>hi</h1>")).toBe("");
    expect(resolveMediaUrl("data:application/javascript,alert(1)")).toBe("");
  });

  it("returns empty for missing values", () => {
    expect(resolveMediaUrl(null)).toBe("");
    expect(resolveMediaUrl(undefined)).toBe("");
    expect(resolveMediaUrl("")).toBe("");
  });

  it("passes through non-uploads absolute URLs unchanged", () => {
    expect(resolveMediaUrl("https://cdn.example/img.png")).toBe("https://cdn.example/img.png");
  });
});
